import { computed, ref } from 'vue';
import { defineStore } from 'pinia';
import { DbService } from '@bindings/db-lens/internal/app';
import type { TableEditInfo, TableRef, RowChange } from '@bindings/db-lens/internal/engine/entities';
import type { QueryResult } from '@/types/queryTab';

export interface DraftRow {
    key: string;
    operation: 'insert' | 'update' | 'delete';
    keys: Record<string, unknown>;
    values: Record<string, unknown>;
    original?: Record<string, unknown>;
}
export interface EditState {
    table: TableRef;
    info: TableEditInfo | null;
    drafts: DraftRow[];
    loading: boolean;
    saving: boolean;
    error: string;
}
export const useTableEditsStore = defineStore('tableEdits', () => {
    const states = ref<Record<string, EditState>>({});
    const hasPending = (id?: string) => !!id && !!states.value[id]?.drafts.length;
    const isSaving = (id?: string) => !!id && !!states.value[id]?.saving;
    const anyPending = computed(() => Object.values(states.value).some(state => state.drafts.length || state.saving));
    async function load(id: string, table: TableRef) {
        states.value[id] = { table, info: null, drafts: [], loading: true, saving: false, error: '' };
        try {
            const info = await DbService.DescribeTableEdit(table);
            if (states.value[id]) states.value[id].info = info;
        } catch (error) {
            if (states.value[id]) states.value[id].error = error instanceof Error ? error.message : String(error);
        } finally { if (states.value[id]) states.value[id].loading = false; }
    }
    function keysFor(id: string, result: QueryResult, index: number): Record<string, unknown> {
        const info = states.value[id]?.info;
        if (!info) return {};
        const document = info.driver === 'mongodb' ? JSON.parse(result.Documents?.[index] ?? '{}') : null;
        return Object.fromEntries((info.keys ?? []).map(name => [name,
            document ? document[name] : result.Rows[index]?.[result.Columns.findIndex(column => column.Name === name)],
        ]));
    }
    function rowKey(id: string, result: QueryResult, index: number): string {
        const keys = keysFor(id, result, index);
        return Object.keys(keys).length ? JSON.stringify(keys) : `row-${result.StartRow + index}`;
    }
    function stage(id: string, draft: DraftRow) {
        const state = states.value[id];
        if (!state || state.saving) return;
        const index = state.drafts.findIndex(row => row.key === draft.key);
        if (index >= 0) state.drafts[index] = draft;
        else state.drafts.push(draft);
        state.error = '';
    }
    function remove(id: string, key: string) {
        const state = states.value[id];
        if (state && !state.saving) state.drafts = state.drafts.filter(row => row.key !== key);
    }
    function stageMany(id: string, drafts: DraftRow[]) {
        const state = states.value[id];
        if (!state || state.saving) return;
        const changed = new Map(drafts.map(draft => [draft.key, draft]));
        state.drafts = [...state.drafts.filter(draft => !changed.has(draft.key)),
            ...drafts.filter(draft => draft.operation !== 'update' || Object.keys(draft.values).length > 0)];
        state.error = '';
    }
    function discard(id: string) {
        const state = states.value[id];
        if (state && !state.saving) { state.drafts = []; state.error = ''; }
    }
    async function save(id: string, cursor = ""): Promise<boolean> {
        const state = states.value[id];
        if (!state?.info || state.saving || !state.drafts.length) return false;
        state.saving = true;
        state.error = '';
        try {
            const response = await DbService.SaveTableChanges({ table: state.info.table, cursor,
                changes: state.drafts.map(row => ({ operation: row.operation, keys: row.keys, values: row.values })) as RowChange[],
            });
            if (!response) throw new Error('No save response received');
            state.drafts.splice(0, response.applied);
            state.error = response.error;
            return response.applied > 0;
        } catch (error) {
            state.error = error instanceof Error ? error.message : String(error);
            return false;
        } finally { state.saving = false; }
    }
    function clear(id: string) { delete states.value[id]; }
    function clearAll() { states.value = {}; }
    return { states, anyPending, hasPending, isSaving, load, keysFor, rowKey, stage, stageMany, remove, discard, save, clear, clearAll };
});
