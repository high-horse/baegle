<template>
    <div class="h-full w-full min-w-0 min-h-0 flex flex-col overflow-hidden text-gray-300">
        <div class="flex shrink-0 min-w-0 items-center justify-between gap-2 mb-3">
            <span class="min-w-0 truncate text-xs font-mono" :title="table ? [table.schema, table.name].filter(Boolean).join('.') : 'DDL'">{{ table ? [table.schema, table.name].filter(Boolean).join('.') : 'DDL' }}</span>
            <q-btn v-if="table" flat dense no-caps icon="refresh" label="Refresh" size="sm" class="shrink-0" :disable="loading" @click="loadDDL" />
        </div>
        <div v-if="loading" class="flex items-center gap-2 text-xs"><q-spinner color="amber" />Loading DDL…</div>
        <div v-else-if="error" role="alert" class="text-xs text-red-300">{{ error }}</div>
        <SqlPreview v-else-if="ddl" :text="ddl" :driver="connectionStore.activeConnection?.driver" class="flex-1 min-w-0 min-h-0" />
        <div v-else class="text-xs text-gray-500">Open a table from the sidebar to view its DDL.</div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount } from 'vue';
import { DbService } from '@bindings/db-lens/internal/app';
import { useTableEditsStore } from '@/stores/tableEditsStore';
import { useConnectionStore } from '@/stores/connectionStore';
import SqlPreview from '@/components/workspace/SqlPreview.vue';

const props = defineProps<{ tableTabId?: string }>();
const tableEdits = useTableEditsStore();
const connectionStore = useConnectionStore();
const table = computed(() => props.tableTabId ? tableEdits.states[props.tableTabId]?.table : undefined);
const ddl = ref('');
const error = ref('');
const loading = ref(false);
let request = 0;
async function loadDDL() {
    const current = ++request;
    ddl.value = '';
    error.value = '';
    loading.value = !!table.value;
    if (!table.value) return;
    try {
        const value = await DbService.GetDDL(table.value);
        if (current === request) {
            ddl.value = value;
            if (!value.trim()) error.value = 'No DDL was returned for this table.';
        }
    } catch (failure) {
        if (current === request) error.value = failure instanceof Error ? failure.message : String(failure);
    } finally {
        if (current === request) loading.value = false;
    }
}
watch(table, loadDDL, { immediate: true });
onBeforeUnmount(() => { request++; });
</script>
