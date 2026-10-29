<template>
    <div class="h-full relative overflow-hidden bg-[var(--surface-data)]">
        <div class="q-ma-lg">
            <q-table
                ref="dataTable"
                role="grid"
                tabindex="0"
                aria-label="Schema columns"
                aria-multiselectable="true"
                :aria-activedescendant="activeCellId"
                @keydown="handleGridKeydown"
                @copy="copyResultSelection($event, displayedRows, selectionRange)"
                flat
                square
                dense
                dark
                :rows="rows"
                :columns="columns"
                row-key="id"
                :pagination="{
                    rowsPerPage: 0,
                }"
                class="absolute-full schema-grid"
                hide-bottom
            >
                <!-- Header -->
                <template #header="props">
                    <q-tr :props="props">
                        <q-th
                            v-for="col in props.cols"
                            :key="col.name"
                            :props="props"
                            :class="{
                                'sn-header': col.name === 'sn',
                            }"
                            class="schema-header text-primary"
                        >
                            {{ col.label }}
                        </q-th>
                    </q-tr>
                </template>
    
                <!-- Body -->
                <template #body="props">
                    <q-tr :props="props">
                        <q-td
                            v-for="col in props.cols"
                            :key="col.name"
                            :props="props"
                            :class="{
                                'sn-cell': col.name === 'sn',
                                'selected-result-cell': isCellSelected(props.rowIndex, gridColumn(col.name)),
                                'active-result-cell': col.name !== 'sn' && activeCell?.rowKey === props.row.id && activeCell?.column === columnIndex(col.name),
                            }"
                            :role="col.name === 'sn' ? 'rowheader' : 'gridcell'"
                            :id="col.name !== 'sn' ? cellId(props.rowIndex, columnIndex(col.name)) : undefined"
                            :aria-selected="col.name !== 'sn' ? isCellSelected(props.rowIndex, gridColumn(col.name)) : undefined"
                            :data-grid-row="col.name !== 'sn' ? props.row.id : undefined"
                            :data-grid-column="col.name !== 'sn' ? gridColumn(col.name) : undefined"
                            @pointerdown="startCellDrag($event, props.row.id, gridColumn(col.name))"
                            @contextmenu="selectContextCell(props.row.id, gridColumn(col.name))"
                        >
                            <template v-if="col.name === 'sn'">
                                {{ props.rowIndex + 1 }}
                            </template>
    
                            <template v-else-if="col.name === 'column'">
                                <div class="flex items-center gap-2">
                                    <q-icon
                                        name="view_column"
                                        size="14px"
                                        class="text-primary"
                                    />
    
                                    <span class="text-white font-medium">
                                        {{ props.row.name }}
                                    </span>
                                </div>
                            </template>
    
                            <template v-else-if="col.name === 'nullable'">
                                <span
                                    :class="
                                        props.row.nullable
                                            ? 'text-teal-400'
                                            : 'text-primary'
                                    "
                                >
                                    {{
                                        props.row.nullable
                                            ? "YES"
                                            : "NO"
                                    }}
                                </span>
                            </template>
    
                            <template v-else-if="col.name === 'default'">
                                <span
                                    v-if="props.row.defaultValue != null && props.row.defaultValue !== ''"
                                    class="text-purple-300"
                                >
                                    {{ props.row.defaultValue }}
                                </span>
    
                                <span
                                    v-else
                                    class="text-[var(--text-muted)]"
                                >
                                    —
                                </span>
                            </template>
    
                            <template v-else>
                                {{ col.value }}
                            </template>
                        </q-td>
                    </q-tr>
                </template>
            </q-table>
        </div>
       
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import type { QTable, QTableColumn } from "quasar";
import type { QueryResult } from "@/types/queryTab";
import { useResultCellNavigation } from '../useResultCellNavigation';
import { copyResultSelection } from '@/utils/resultClipboard';

const props = defineProps<{
    result: QueryResult;
}>();

const columns = computed<QTableColumn[]>(() => [
    {
        name: "sn",
        label: "#",
        field: "sn",
        align: "left",
        sortable: false,
    },

    {
        name: "column",
        label: "Column",
        field: "name",
        align: "left",
        sortable: true,
    },

    {
        name: "type",
        label: "Type",
        field: "type",
        align: "left",
        sortable: true,
    },

    {
        name: "nullable",
        label: "Nullable",
        field: "nullable",
        align: "left",
        sortable: true,
    },

    {
        name: "default",
        label: "Default",
        field: "defaultValue",
        align: "left",
        sortable: true,
    },
]);

const rows = computed(() => {
    return props.result.Columns.map((column, index) => ({
        id: String(index),
        name: column.Name,
        type: column.Type,
        nullable: column.Nullable,
        defaultValue: column.DefaultValue,
        'column-0': column.Name,
        'column-1': column.Type,
        'column-2': column.Nullable ? 'YES' : 'NO',
        'column-3': column.DefaultValue,
    }));
});
const dataTable = ref<QTable | null>(null);
const displayedRows = computed(() => [...(dataTable.value?.filteredSortedRows ?? rows.value)]);
const dataColumns = computed(() => columns.value.filter(column => column.name !== 'sn').map(column => column.name));
const columnIndex = (name: string) => dataColumns.value.indexOf(name);
const gridColumn = (name: string) => name === 'sn' ? 'sn' : `column-${columnIndex(name)}`;
const { activeCell, activeCellId, cellId, selectContextCell, startCellDrag, handleGridKeydown, selectionRange, isCellSelected } = useResultCellNavigation(
    dataTable, displayedRows, dataColumns, ref(''),
);
</script>

<style scoped>
.schema-grid:focus { outline: none; }
.schema-grid :deep(.q-table__middle) { overflow: auto; max-height: 100%; }
.schema-grid :deep(th), .schema-grid :deep(td) { user-select: none; -webkit-user-select: none; }
.schema-grid :deep(thead th) { position: sticky; top: 0; z-index: 1; background: var(--surface-panel); }
.schema-grid :deep(td.selected-result-cell) { background: var(--selection-bg) !important; color: #fff; }
.schema-grid :deep(td.active-result-cell) { box-shadow: inset 0 0 0 2px var(--selection-border); }
</style>
