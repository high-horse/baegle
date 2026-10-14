<template>
    <q-dialog v-model="visible" position="top" @show="searchInput?.focus()">
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
            <div ref="resultsElement" id="table-picker-results" role="listbox" aria-label="Database objects" class="table-picker-results">
                <div
                    v-for="(item, index) in matches"
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
                <div v-if="!query.trim()" class="q-pa-lg text-center text-grey-5" role="status">Start typing to search tables, views, and collections.</div>
                <div v-else-if="searchPending || loadingStates.setActiveConnectionMetadata" class="q-pa-lg text-center text-grey-5" role="status"><q-spinner class="q-mr-sm" />Searching database objects…</div>
                <div v-else-if="!matches.length" class="q-pa-lg text-center text-grey-5" role="status">No matching tables, views, or collections.</div>
                <div v-else-if="matches.length === 100" class="q-pa-sm text-center text-grey-5" role="status">Showing the first 100 matches. Keep typing to narrow results.</div>
            </div>
            <q-separator dark />
            <div class="q-px-md q-py-sm text-caption text-grey-5">↑ ↓ to select · Enter to open · Esc to close</div>
        </q-card>
    </q-dialog>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import type { QInput } from "quasar";
import { storeToRefs } from "pinia";
import { useConnectionStore } from "@/stores/connectionStore";
import { filterDatabaseObjects } from "@/utils/tableQuickOpen";

const emit = defineEmits<{
    selectTable: [node: { id: string; label: string; type: string; schema: string; database: string }];
}>();
const connectionStore = useConnectionStore();
const { activeConnection, activeConnectionMetadata, loadingStates } = storeToRefs(connectionStore);
const visible = ref(false);
const query = ref("");
const settledQuery = ref("");
const searchPending = ref(false);
let searchTimer: ReturnType<typeof setTimeout> | undefined;
const selectedIndex = ref(0);
const searchInput = ref<QInput | null>(null);
const resultsElement = ref<HTMLElement | null>(null);
const matches = computed(() => visible.value && settledQuery.value
    ? filterDatabaseObjects(activeConnectionMetadata.value ?? [], settledQuery.value)
    : []);

watch([query, visible], () => {
    clearTimeout(searchTimer);
    settledQuery.value = "";
    searchPending.value = false;
    if (!visible.value || !query.value.trim()) return;
    searchPending.value = true;
    searchTimer = setTimeout(() => {
        settledQuery.value = query.value.trim();
        searchPending.value = false;
        if (!activeConnectionMetadata.value && !loadingStates.value.setActiveConnectionMetadata) {
            void connectionStore.setActiveConnectionMetadata();
        }
    }, 500);
}, { flush: "sync" });

watch(activeConnection, () => { visible.value = false; });

watch(matches, () => {
    selectedIndex.value = 0;
    if (resultsElement.value) resultsElement.value.scrollTop = 0;
});

function openItem(index: number) {
    const item = matches.value[index];
    if (!item || !visible.value) return;
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
        const count = matches.value.length;
        if (!count) return;
        selectedIndex.value = (selectedIndex.value + (event.key === "ArrowDown" ? 1 : -1) + count) % count;
        nextTick(() => resultsElement.value?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" }));
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
    clearTimeout(searchTimer);
    window.removeEventListener("keydown", handleShortcut, true);
});
</script>

<style scoped>
.table-picker { width: 600px; max-width: calc(100vw - 32px); margin-top: 10vh; background: #353535; color: #d1d5db; border: 1px solid #505050; }
.table-picker-results { max-height: 55vh; overflow-y: auto; }
.table-picker-option { display: flex; align-items: center; gap: 12px; padding: 8px 16px; cursor: pointer; }
.table-picker-selected { background: #494236; box-shadow: inset 3px 0 #f59e0b; }
.table-picker-name { overflow-wrap: anywhere; }
</style>
