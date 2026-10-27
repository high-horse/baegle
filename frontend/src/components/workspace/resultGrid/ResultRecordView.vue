<template>
    <section ref="container" class="result-record" tabindex="0" aria-label="Row details" @keydown="handleKeydown">
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
            <table aria-label="Selected row fields">
                <thead><tr><th scope="col">Column</th><th scope="col">Value</th></tr></thead>
                <tbody>
                    <tr v-for="(column, index) in columns" :key="index" :class="{ 'record-active-field': index === selectedColumn }" :data-column="index" @click="emit('selectColumn', index)">
                        <th scope="row">
                            <div>{{ column.Name }}</div>
                            <q-tooltip v-if="column.Type || column.Nullable !== undefined || column.DefaultValue != null" :delay="200" class="record-field-tooltip" max-width="360px">
                                <div v-if="column.Type">{{ column.Type }}</div>
                                <div v-if="column.Nullable !== undefined">{{ column.Nullable ? 'Nullable' : 'Not null' }}</div>
                                <div v-if="column.DefaultValue != null">Default: {{ column.DefaultValue }}</div>
                            </q-tooltip>
                        </th>
                        <td>
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
import { nextTick, onMounted, ref } from 'vue';
import type { QueryColumn } from '@/types/queryTab';

const props = defineProps<{
    row: Record<string, unknown>;
    columns: QueryColumn[];
    position: number;
    total: number;
    selectedColumn: number;
}>();
const emit = defineEmits<{ close: []; move: [offset: number]; selectColumn: [column: number] }>();
const container = ref<HTMLElement | null>(null);
function focus() { container.value?.focus({ preventScroll: true }); }
function handleKeydown(event: KeyboardEvent) {
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
    const scroller = container.value?.querySelector<HTMLElement>('.record-fields');
    const field = scroller?.querySelector<HTMLElement>(`[data-column="${props.selectedColumn}"]`);
    if (scroller && field) scroller.scrollTop = Math.max(0, field.offsetTop - 30);
});
</script>

<style scoped>
.result-record { display: flex; flex-direction: column; min-width: 0; min-height: 0; background: #2d2d2d; color: #d1d5db; font-family: monospace; font-size: var(--grid-font-size, 11px); outline: none; }
.record-header { display: flex; flex-shrink: 0; align-items: center; justify-content: space-between; gap: 12px; padding: 6px 10px; background: #353535; border-bottom: 1px solid #484848; }
.record-summary { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 14px; min-width: 0; }
.record-summary strong { color: #fbbf24; }
.record-summary > span { color: #a0a0a0; }
.record-actions { display: flex; flex-shrink: 0; align-items: center; gap: 2px; }
.record-fields { position: relative; flex: 1; min-height: 0; overflow: auto; overscroll-behavior: contain; user-select: text; }
table { border-collapse: separate; border-spacing: 0; table-layout: fixed; width: 100%; }
th, td { text-align: left; vertical-align: top; padding: 8px 12px; border-bottom: 1px solid #484848; overflow-wrap: anywhere; }
thead th { position: sticky; top: 0; z-index: 1; background: #353535; color: #fbbf24; }
th:first-child { width: 32%; border-right: 1px solid #484848; }
tbody th { font-weight: 500; }
td { white-space: pre-wrap; }
.record-field-tooltip { font-family: monospace; font-size: 11px; line-height: 1.5; overflow-wrap: anywhere; }
.record-empty { color: #a0a0a0; font-style: italic; }
.record-active-field { background: #3d5066; }
.record-active-field th { box-shadow: inset 3px 0 #ffca28; }
</style>
