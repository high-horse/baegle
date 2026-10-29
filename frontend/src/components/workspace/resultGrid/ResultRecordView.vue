<template>
    <section ref="container" class="result-record" tabindex="0" aria-label="Row details" :aria-activedescendant="activeCellId" @keydown="handleKeydown" @copy="copyResultSelection($event, fieldRows, selectionRange)">
        <header class="record-header">
            <div class="record-summary" role="status">
                <strong>Row {{ row._rowNumber }}</strong>
                <span>{{ position + 1 }} of {{ total }} shown · {{ columns.length }} columns</span>
                <span v-if="row._status" class="text-amber-400">Pending {{ row._status }}</span>
            </div>
            <div class="record-actions">
                <q-btn flat dense round icon="chevron_left" aria-label="Previous row" title="Previous row" :disable="position <= 0" @click="emit('move', -1)" />
                <q-btn flat dense round icon="chevron_right" aria-label="Next row" title="Next row" :disable="position >= total - 1" @click="emit('move', 1)" />
                <q-btn flat dense no-caps icon="table_rows" label="Grid" aria-label="Back to result grid" title="Back to grid (Tab or Esc)" @click="emit('close')" />
            </div>
        </header>
        <div class="record-fields" @click="focus">
            <table role="grid" aria-multiselectable="true" aria-label="Selected row fields">
                <thead><tr><th scope="col">Column</th><th scope="col">Value</th></tr></thead>
                <tbody>
                    <tr v-for="(column, index) in columns" :key="index" :data-column="index">
                        <th scope="row" role="gridcell" :id="cellId(index, 0)" :data-grid-row="fieldRows[index].id" data-grid-column="column-0"
                            :aria-selected="isCellSelected(index, 'column-0')" :class="cellClass(index, 0)"
                            @pointerdown="startCellDrag($event, String(fieldRows[index].id), 'column-0')" @contextmenu="selectContextCell(String(fieldRows[index].id), 'column-0')">
                            <div>{{ column.Name }}</div>
                            <q-tooltip v-if="column.Type || column.Nullable !== undefined || column.DefaultValue != null" :delay="200" class="record-field-tooltip" max-width="360px">
                                <div v-if="column.Type">{{ column.Type }}</div>
                                <div v-if="column.Nullable !== undefined">{{ column.Nullable ? 'Nullable' : 'Not null' }}</div>
                                <div v-if="column.DefaultValue != null">Default: {{ column.DefaultValue }}</div>
                            </q-tooltip>
                        </th>
                        <td role="gridcell" :id="cellId(index, 1)" :data-grid-row="fieldRows[index].id" data-grid-column="column-1"
                            :aria-selected="isCellSelected(index, 'column-1')" :class="cellClass(index, 1)"
                            @pointerdown="startCellDrag($event, String(fieldRows[index].id), 'column-1')" @contextmenu="selectContextCell(String(fieldRows[index].id), 'column-1')">
                            <span v-if="row[`column-${index}`] == null" class="record-empty">NULL</span>
                            <span v-else-if="row[`column-${index}`] === ''" class="record-empty">(empty string)</span>
                            <span v-else>{{ row[`column-${index}`] }}</span>
                        </td>
                    </tr>
                </tbody>
            </table>
        </div>
    </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import type { QueryColumn } from '@/types/queryTab';
import { useResultCellNavigation } from './useResultCellNavigation';
import { copyResultSelection } from '@/utils/resultClipboard';

const props = defineProps<{
    row: Record<string, unknown>;
    columns: QueryColumn[];
    position: number;
    total: number;
    selectedColumn: number;
}>();
const emit = defineEmits<{ close: []; move: [offset: number]; selectColumn: [column: number] }>();
const container = ref<HTMLElement | null>(null);
const fieldRows = computed(() => props.columns.map((column, index) => ({
    id: `${props.row.id}:${index}`, 'column-0': column.Name, 'column-1': props.row[`column-${index}`],
})));
const table = computed(() => container.value ? { $el: container.value } : null);
const { activeCell, activeCellId, cellId, selectCell, selectContextCell, startCellDrag, handleGridKeydown, selectionRange, isCellSelected } = useResultCellNavigation(
    table, fieldRows, computed(() => ['Column', 'Value']), ref(''), { scroller: '.record-fields', rowHeader: false },
);
function cellClass(index: number, column: number) {
    return { 'selected-result-cell': isCellSelected(index, `column-${column}`),
        'active-result-cell': activeCell.value?.rowKey === fieldRows.value[index].id && activeCell.value.column === column };
}
watch(activeCell, cell => {
    const index = fieldRows.value.findIndex(row => row.id === cell?.rowKey);
    if (index >= 0) emit('selectColumn', index);
});
watch(() => props.row.id, async () => {
    await nextTick();
    const row = fieldRows.value[props.selectedColumn];
    if (row) selectCell(row.id, 'column-1', false);
});
function focus() { container.value?.focus({ preventScroll: true }); }
function handleKeydown(event: KeyboardEvent) {
    handleGridKeydown(event);
    if (event.isComposing || event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) return;
    if (event.key === 'Escape' || (event.key === 'Tab' && event.target === event.currentTarget)) {
        event.preventDefault();
        event.stopPropagation();
        emit('close');
    }
}
onMounted(async () => {
    await nextTick();
    focus();
    const selected = fieldRows.value[props.selectedColumn];
    if (selected) selectCell(selected.id, 'column-1', false);
    const scroller = container.value?.querySelector<HTMLElement>('.record-fields');
    const field = scroller?.querySelector<HTMLElement>(`[data-column="${props.selectedColumn}"]`);
    if (scroller && field) scroller.scrollTop = Math.max(0, field.offsetTop - 30);
});
</script>

<style scoped>
.result-record { display: flex; flex-direction: column; min-width: 0; min-height: 0; background: var(--surface-data); color: var(--text-primary); font-family: monospace; font-size: var(--grid-font-size, 11px); outline: none; }
.record-header { display: flex; flex-shrink: 0; align-items: center; justify-content: space-between; gap: 12px; padding: 6px 10px; background: var(--surface-panel); border-bottom: 1px solid var(--border-default); }
.record-summary { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 14px; min-width: 0; }
.record-summary strong { color: #fbbf24; }
.record-summary > span { color: var(--text-muted); }
.record-actions { display: flex; flex-shrink: 0; align-items: center; gap: 2px; }
.record-fields { position: relative; flex: 1; min-height: 0; overflow: auto; overscroll-behavior: contain; user-select: none; -webkit-user-select: none; }
table { border-collapse: separate; border-spacing: 0; table-layout: fixed; width: 100%; }
th, td { text-align: left; vertical-align: top; padding: 8px 12px; border-bottom: 1px solid var(--border-default); overflow-wrap: anywhere; }
thead th { position: sticky; top: 0; z-index: 1; background: var(--surface-panel); color: #fbbf24; }
th:first-child { width: 32%; border-right: 1px solid var(--border-default); }
tbody th { font-weight: 500; }
td { white-space: pre-wrap; }
.record-field-tooltip { font-family: monospace; font-size: var(--ui-font-size); line-height: 1.5; overflow-wrap: anywhere; }
.record-empty { color: var(--text-muted); font-style: italic; }
.selected-result-cell { background: var(--selection-bg); color: #fff; }
.active-result-cell { box-shadow: inset 0 0 0 2px var(--selection-border); }
</style>
