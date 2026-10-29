<template>
    <q-dialog v-model="visible" :transition-duration="0" position="top" @show="searchInput?.focus()">
        <q-card class="table-picker" @keydown="handlePickerKey">
            <q-input
                ref="searchInput"
                v-model="query"
                autofocus
                borderless
                dense
                class="q-px-md q-py-sm"
                placeholder="Search tables, views, and collections…"
                aria-label="Open database object"
                role="combobox"
                aria-autocomplete="list"
                :aria-expanded="visible"
                aria-controls="table-picker-results"
                :aria-activedescendant="matches.length ? `table-picker-option-${selectedIndex}` : undefined"
            >
                <template #prepend><q-icon name="search" /></template>
                <template #append><span class="text-caption text-grey-5">Esc</span></template>
            </q-input>
            <q-separator dark />
            <q-virtual-scroll ref="resultsElement" id="table-picker-results" role="listbox" aria-label="Database objects" class="table-picker-results" :items="matches" :virtual-scroll-item-size="56" :aria-busy="searchPending">
                <template #default="{ item, index }">
                <div
                    :id="`table-picker-option-${index}`"
                    :key="JSON.stringify([item.database, item.schema, item.name, item.type])"
                    role="option"
                    :aria-selected="index === selectedIndex"
                    class="table-picker-option"
                    :class="{ 'table-picker-selected': index === selectedIndex }"
                    @mouseenter="selectedIndex = index"
                    @mousedown.prevent
                    @click="openItem(index)"
                >
                    <q-icon :name="item.type === 'VIEW' ? 'visibility' : item.type === 'COLLECTION' ? 'data_object' : 'table_chart'" color="amber-5" size="18px" />
                    <div class="col min-w-0">
                        <div class="table-picker-name">{{ item.name }}</div>
                        <div class="text-caption text-grey-5 table-picker-name">{{ [item.database, item.schema].filter(Boolean).join(' · ') }}</div>
                    </div>
                    <span class="text-caption text-grey-5">{{ item.type.toLowerCase() }}</span>
                </div>
                </template>
                <template #after>
                <div v-if="!query.trim()" class="q-pa-lg text-center text-grey-5" role="status">Start typing to search tables, views, and collections.</div>
                <div v-else-if="searchPending || loadingStates.setActiveConnectionMetadata" class="q-pa-lg text-center text-grey-5" role="status"><q-spinner class="q-mr-sm" />Searching database objects…</div>
                <div v-else-if="!matches.length" class="q-pa-lg text-center text-grey-5" role="status">No matching tables, views, or collections.</div>
                <div v-else-if="matches.length === 100" class="q-pa-sm text-center text-grey-5" role="status">Showing the first 100 matches. Keep typing to narrow results.</div>
                </template>
            </q-virtual-scroll>
            <div class="q-px-md q-py-sm text-caption text-grey-5">↑ ↓ to select · Enter to open · Esc to close</div>
        </q-card>
    </q-dialog>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, shallowRef, toRaw, watch } from "vue";
import type { QInput, QVirtualScroll } from "quasar";
import { storeToRefs } from "pinia";
import { useConnectionStore } from "@/stores/connectionStore";
import { createDatabaseObjectSearch } from "@/utils/tableQuickOpen";

const emit = defineEmits<{
    selectTable: [node: { id: string; label: string; type: string; schema: string; database: string }];
}>();
const connectionStore = useConnectionStore();
const { activeConnection, activeConnectionMetadata, loadingStates } = storeToRefs(connectionStore);
const visible = ref(false);
const query = ref("");
const searchPending = ref(false);
let searchTimer: ReturnType<typeof setTimeout> | undefined;
const selectedIndex = ref(0);
const searchInput = ref<QInput | null>(null);
const resultsElement = ref<QVirtualScroll | null>(null);
type DatabaseObject = NonNullable<typeof activeConnectionMetadata.value>[number];
const matches = shallowRef<DatabaseObject[]>([]);
let indexedMetadata: DatabaseObject[] | null = null;
let searchCatalog: ReturnType<typeof createDatabaseObjectSearch<DatabaseObject>> | undefined;
let searchGeneration = 0;

watch([query, visible, activeConnectionMetadata], () => {
    clearTimeout(searchTimer);
    const generation = ++searchGeneration;
    searchPending.value = false;
    if (!visible.value || !query.value.trim()) {
        matches.value = [];
        return;
    }
    searchPending.value = true;
    searchTimer = setTimeout(async () => {
        if (!activeConnectionMetadata.value) {
            if (!loadingStates.value.setActiveConnectionMetadata) await connectionStore.setActiveConnectionMetadata();
            if (generation !== searchGeneration) return;
        }
        const metadata = toRaw(activeConnectionMetadata.value);
        if (!metadata) { searchPending.value = false; return; }
        if (metadata !== indexedMetadata) {
            indexedMetadata = metadata;
            searchCatalog = createDatabaseObjectSearch(metadata);
        }
        const found = await searchCatalog!(query.value, () => generation !== searchGeneration);
        if (found === null || generation !== searchGeneration) return;
        matches.value = found;
        selectedIndex.value = 0;
        searchPending.value = false;
        await nextTick();
        resultsElement.value?.scrollTo(0);
    }, activeConnectionMetadata.value ? 100 : 300);
}, { flush: "sync" });

watch(activeConnection, () => {
    visible.value = false;
    indexedMetadata = null;
    searchCatalog = undefined;
});

function openItem(index: number) {
    const item = matches.value[index];
    if (!item || !visible.value || searchPending.value) return;
    visible.value = false;
    emit("selectTable", {
        id: `${item.schema || "public"}.${item.name}`,
        label: item.name,
        type: item.type.toLowerCase(),
        schema: item.schema || "public",
        database: item.database,
    });
}

function handlePickerKey(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (event.key === "Enter") {
        event.preventDefault();
        event.stopPropagation();
        openItem(selectedIndex.value);
    } else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        event.stopPropagation();
        if (searchPending.value) return;
        const count = matches.value.length;
        if (!count) return;
        selectedIndex.value = (selectedIndex.value + (event.key === "ArrowDown" ? 1 : -1) + count) % count;
        resultsElement.value?.scrollTo(selectedIndex.value, "start-force");
    }
}

function handleShortcut(event: KeyboardEvent) {
    if (!activeConnection.value || event.isComposing || !event.ctrlKey || event.shiftKey || event.altKey || event.metaKey || event.key.toLowerCase() !== "p") return;
    event.preventDefault();
    event.stopPropagation();
    if (!visible.value) {
        query.value = "";
        selectedIndex.value = 0;
        visible.value = true;
    } else {
        searchInput.value?.focus();
    }
}

onMounted(() => window.addEventListener("keydown", handleShortcut, true));
onBeforeUnmount(() => {
    searchGeneration++;
    clearTimeout(searchTimer);
    window.removeEventListener("keydown", handleShortcut, true);
});
</script>

<style scoped>
.table-picker { width: 600px; max-width: calc(100vw - 32px); margin-top: 10vh; background: var(--surface-panel); color: var(--text-primary); border: 1px solid var(--border-default); }
.table-picker-results { height: 320px; max-height: 55vh; overflow-y: auto; }
.table-picker-option { display: flex; align-items: center; gap: 12px; padding: 8px 16px; height: 56px; box-sizing: border-box; cursor: pointer; }
.table-picker-selected { background: #494236; box-shadow: inset 3px 0 #f59e0b; }
.table-picker-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
