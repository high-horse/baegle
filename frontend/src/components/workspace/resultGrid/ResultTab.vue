<template>
    <div ref="gridElement" class="relative h-full w-full overflow-hidden bg-[#2d2d2d]" :class="{ 'striped-rows': resultSettings.settings.stripedRows }" :style="{ '--grid-font-size': `${resultSettings.settings.fontSize}px` }">
        <q-form v-if="result.IsQuery" class="result-filter-bar" @submit.prevent="applyExpressions">
            <q-icon name="filter_alt" size="15px" :class="appliedFilter ? 'text-amber-400' : 'text-gray-500'" />
            <div class="result-filter-input" :class="{ 'has-error': filterError }">
                <span class="text-gray-500 select-none">WHERE</span>
                <ResultFilterInput v-model="filterDraft" :columns="result.Columns" :error="filterError" :applied="appliedFilter" @apply="applyExpressions" @clear="clearFilter" />
                <q-icon v-if="filterError" name="error_outline" size="14px" class="text-red-400"><q-tooltip>{{ filterError }}</q-tooltip></q-icon>
            </div>
            <div class="result-filter-input result-order-input" :class="{ 'has-error': orderError }">
                <span class="text-gray-500 select-none whitespace-nowrap">ORDER BY</span>
                <ResultFilterInput v-model="orderDraft" mode="order" :columns="result.Columns" :error="orderError" :applied="appliedOrder" @apply="applyExpressions" @clear="clearOrder" />
                <q-icon v-if="orderError" name="error_outline" size="14px" class="text-red-400"><q-tooltip>{{ orderError }}</q-tooltip></q-icon>
                <q-btn v-if="orderDraft || appliedOrder" flat dense type="button" class="filter-action" :disable="loading || editState?.saving" title="Reset ORDER BY (Esc)" aria-label="Reset order by" @click="clearOrder"><q-icon name="close" size="14px" /></q-btn>
            </div>
            <q-btn flat dense no-caps type="submit" class="filter-action" :disable="loading || editState?.saving" title="Apply WHERE and ORDER BY (Enter)" aria-label="Apply filter and order"><q-icon name="play_arrow" size="16px" /></q-btn>
            <q-btn flat dense no-caps type="button" class="filter-action" :disable="!filterDraft && !appliedFilter" title="Reset filter (Esc)" aria-label="Reset filter" @click="clearFilter"><q-icon name="close" size="15px" /></q-btn>
            <q-btn flat dense no-caps type="button" class="filter-action" title="Filter syntax" aria-label="Filter syntax">
                <q-icon name="help_outline" size="15px" />
                <q-menu class="border border-[#484848] bg-[#353535] text-gray-300">
                    <div class="max-w-sm p-3 font-mono text-[11px] leading-6">
                        <div class="text-amber-400">Combine conditions with AND / OR</div>
                        <div>age &gt;= 18 AND status = 'active'</div>
                        <div>(name LIKE 'A%' OR name LIKE 'B%')</div>
                        <div>status IN ('active', 'pending')</div>
                        <div>deleted_at IS NULL</div>
                        <div class="mt-2 text-amber-400">ORDER BY: name ASC, created_at DESC</div>
                        <div class="mt-2 text-gray-500">Use double quotes for column names with spaces, or column numbers in ORDER BY.<br />Enter to apply · Esc to reset the focused field<br />WHERE and ORDER BY apply to the current page.</div>
                        <div class="mt-2 text-gray-500">Columns: {{ result.Columns.map(column => column.Name).join(', ') }}</div>
                    </div>
                </q-menu>
            </q-btn>
            <span class="filter-count">{{ filteredRows.length }}/{{ workingRows.length }} <span class="filter-scope">on this page</span></span>
        </q-form>

        <TableDataEditor v-if="tableTabId" ref="rowEditor" :tab-id="tableTabId" :result="result" :selected-key="selectedRowKey" :loading="loading" />

        <!-- =========================================================
             TABLE
             ========================================================= -->

        <q-table
            flat
            square
            dense
            dark
            :rows="mappedRows"
            :columns="mappedColumns"
            row-key="id"
            :pagination="{
                rowsPerPage: 0,
            }"
            :virtual-scroll="mappedRows.length > 100"
            :virtual-scroll-item-size="28"
            :virtual-scroll-sticky-size-start="28"
            class="data-explorer-grid bg-transparent"
            :class="{ 'has-filter-toolbar': result.IsQuery, 'has-edit-toolbar': !!tableTabId }"
            table-class="table-fixed"
            :style="{ '--result-table-width': `${tableWidth}px`, '--row-number-width': `${snColumnWidth}px` }"
            hide-bottom
        >
            <!-- =====================================================
                 HEADER
                 ===================================================== -->

            <template #header="props">
                <q-tr :props="props" class="bg-[#353535]">
                    <q-th
                        v-for="col in props.cols"
                        :key="col.name"
                        :props="props"
                        class="relative box-border h-[28px] overflow-hidden whitespace-nowrap border-b-2 border-[#484848] px-2 py-0 align-middle font-mono text-[11px] font-bold text-amber-400"
                        :class="{
                            'cursor-pointer': col.name !== 'sn' && (result.CanSort ?? result.CanNavigate),
                            'sticky-col-header pl-2 pr-1 text-left font-normal text-[#909090]':
                                col.name === 'sn',
                        }"
                        :style="getColumnStyle(col.name)"
                        :tabindex="col.name !== 'sn' && (result.CanSort ?? result.CanNavigate) ? 0 : undefined"
                        :aria-sort="activeSortColumn === col.name ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'"
                        @click="toggleSort(col.name)"
                        @keydown.enter.prevent="toggleSort(col.name)"
                    >
                        <div
                            class="flex w-full min-w-0 items-center overflow-hidden whitespace-nowrap"
                        >
                            <div
                                class="flex min-w-0 flex-1 items-center overflow-hidden pr-1 whitespace-nowrap"
                            >
                                <!-- Column name -->
                                <span
                                    class="min-w-0 shrink overflow-hidden text-ellipsis whitespace-nowrap"
                                >
                                    {{ col.label }}
                                </span>

                                <!-- Column type -->
                                <span
                                    v-if="col.name !== 'sn'"
                                    class="ml-1 min-w-0 shrink overflow-hidden text-ellipsis whitespace-nowrap text-[8px] font-normal text-gray-500"
                                >
                                    {{ col.Type }}
                                </span>
                            </div>

                            <!-- Sort indicator -->
                            <q-icon
                                v-if="col.name !== 'sn' && (result.CanSort ?? result.CanNavigate)"
                                :name="
                                    activeSortColumn !== col.name ? 'unfold_more' : sortDirection === 'asc'
                                        ? 'arrow_upward'
                                        : 'arrow_downward'
                                "
                                size="12px"
                                class="ml-0.5 shrink-0 text-amber-400"
                            />
                        </div>

                        <!-- Resize handle -->
                        <span
                            v-if="col.name !== 'sn'"
                            class="column-resizer"
                            role="separator"
                            aria-orientation="vertical"
                            :aria-label="`Resize ${col.label}`"
                            @pointerdown.stop.prevent="startResize($event, col.name)"
                            @click.stop
                        />

                        <!-- Column tooltip -->
                        <q-tooltip
                            v-if="col.name !== 'sn'"
                            anchor="bottom middle"
                            self="top middle"
                            :offset="[0, 6]"
                            class="bg-[#383838] text-gray-300 shadow-lg"
                        >
                            <div class="font-mono text-[10px]">
                                {{ col.Type }}

                                |

                                {{ col.Nullable ? "Nullable" : "Not Nullable" }}

                                {{
                                    col.DefaultValue
                                        ? ` | Default: ${col.DefaultValue}`
                                        : ""
                                }}
                            </div>
                        </q-tooltip>

                        <!-- Context menu -->
                        <q-menu
                            v-if="col.name !== 'sn' && (result.CanSort ?? result.CanNavigate)"
                            context-menu
                            class="min-w-[150px] border border-[#484848] bg-[#383838] text-gray-300 shadow-xl"
                        >
                            <!-- Sort Ascending -->
                            <q-item
                                clickable
                                v-close-popup
                                dense
                                class="min-h-[28px] px-2 hover:bg-[#484848]"
                                :disable="loading"
                                @click="sortColumn(col.name, 'asc')"
                            >
                                <q-item-section avatar class="min-w-[24px]">
                                    <q-icon
                                        name="arrow_upward"
                                        size="14px"
                                        class="text-gray-400"
                                    />
                                </q-item-section>

                                <q-item-section>
                                    <q-item-label class="font-mono text-[11px]">
                                        Sort ascending
                                    </q-item-label>
                                </q-item-section>
                            </q-item>

                            <!-- Sort Descending -->
                            <q-item
                                clickable
                                v-close-popup
                                dense
                                class="min-h-[28px] px-2 hover:bg-[#484848]"
                                :disable="loading"
                                @click="sortColumn(col.name, 'desc')"
                            >
                                <q-item-section avatar class="min-w-[24px]">
                                    <q-icon
                                        name="arrow_downward"
                                        size="14px"
                                        class="text-gray-400"
                                    />
                                </q-item-section>

                                <q-item-section>
                                    <q-item-label class="font-mono text-[11px]">
                                        Sort descending
                                    </q-item-label>
                                </q-item-section>
                            </q-item>

                            <q-separator class="my-1 bg-[#484848]" />

                            <!-- Clear Sort -->
                            <q-item
                                v-if="activeSortColumn === col.name"
                                clickable
                                v-close-popup
                                dense
                                class="min-h-[28px] px-2 hover:bg-[#484848]"
                                :disable="loading"
                                @click="clearSort"
                            >
                                <q-item-section avatar class="min-w-[24px]">
                                    <q-icon
                                        name="close"
                                        size="14px"
                                        class="text-gray-500"
                                    />
                                </q-item-section>

                                <q-item-section>
                                    <q-item-label class="font-mono text-[11px]">
                                        Clear Sort
                                    </q-item-label>
                                </q-item-section>
                            </q-item>
                        </q-menu>
                    </q-th>
                </q-tr>
            </template>

            <!-- =====================================================
                 BODY
                 ===================================================== -->

            <template #body="props">
                <q-tr
                    :props="props"
                    class="bg-[#2d2d2d] hover:!bg-[#404040]"
                    :class="{ 'striped-row': resultSettings.settings.stripedRows && props.rowIndex % 2 === 1, 'selected-edit-row': selectedRowKey === props.row._editKey, 'pending-insert': props.row._status === 'insert', 'pending-update': props.row._status === 'update', 'pending-delete': props.row._status === 'delete' }"
                    @click="selectedRowKey = props.row._editKey || ''"
                >
                    <q-td
                        v-for="col in props.cols"
                        :key="col.name"
                        :props="props"
                        class="box-border overflow-hidden whitespace-nowrap border-b border-[#484848] px-2 py-1.5 font-mono text-[11px] text-gray-300"
                        :class="{
                            'sticky-col-body pl-2 pr-1 text-left text-[#909090]':
                                col.name === 'sn',
                        }"
                        :style="getColumnStyle(col.name)"
                        @click="col.name !== 'sn' && ($event.detail === 3 && (isMongoResult || isJSONColumn(col.Type)) ? inspectDocument(props.row[col.field], col.Type) : $event.detail === 4 && tableTabId && editState?.info?.canInsert ? rowEditor?.openRow(props.row._editKey) : undefined)"
                    >
                        <!-- Row number -->
                        <template v-if="col.name === 'sn'">
                            <span class="row-number">
                                {{
                                    props.row._rowNumber
                                }}
                            </span>
                        </template>

                        <!-- Cell -->
                        <template v-else>
                            <div
                                class="block w-full min-w-0 overflow-hidden text-ellipsis whitespace-nowrap"
                                :title="getCellTitle(props.row[col.field])"
                            >
                                <span v-if="props.row[col.field] == null && resultSettings.settings.showNulls" class="text-gray-500 italic">NULL</span>
                                <template v-else>{{ props.row[col.field] }}</template>
                            </div>
                        </template>
                    </q-td>
                </q-tr>
            </template>
        </q-table>

        <q-dialog v-model="documentDialog">
            <q-card class="bg-[#353535] text-gray-300" style="width: 800px; max-width: 90vw">
                <q-card-section class="flex items-center justify-between">
                    <span>{{ documentTitle }}</span>
                    <q-btn flat round dense icon="close" aria-label="Close JSON details" v-close-popup />
                </q-card-section>
                <q-card-section style="max-height: 70vh; overflow: auto">
                    <pre class="text-xs font-mono whitespace-pre-wrap break-words select-text">{{ selectedDocument }}</pre>
                </q-card-section>
            </q-card>
        </q-dialog>

        <!-- =========================================================
             FIXED BOTTOM TOOLBAR
             ========================================================= -->

        <div
            class="data-grid-footer absolute bottom-0 left-0 right-0 z-[100] flex h-[36px] items-center justify-between border-t border-[#484848] bg-[#353535] px-2"
        >
            <!-- Left side -->
            <div class="flex items-center gap-2">
                <q-btn flat dense no-caps type="button" class="pagination-button" :disable="loading || navigationBlocked"
                    :title="tableTabId ? 'Refresh table (Ctrl+R)' : 'Run this query again'" aria-label="Run this query again" @click="emit('refresh')">
                    <q-icon name="refresh" size="16px" />
                </q-btn>
                <DataTransfer v-if="result.IsQuery" action="export" :result="result" :table-tab-id="tableTabId"
                    :disabled="loading || navigationBlocked" />
                <span class="font-mono text-[10px] text-gray-500">
                    <template v-if="result.Rows.length">Rows {{ result.StartRow }}–{{ result.StartRow + result.Rows.length - 1 }}</template>
                    <template v-else>{{ result.IsQuery ? 'No rows' : 'Statement completed' }}</template>
                    <span v-if="result.IsQuery && !result.CanSort && totalRows != null"> · {{ totalRows.toLocaleString() }} rows total</span>
                </span>
                <template v-if="result.IsQuery && result.CanSort">
                    <q-btn v-if="countingRows" flat dense no-caps type="button" size="sm" color="amber"
                        :disable="stoppingCount" :label="stoppingCount ? 'Cancelling…' : 'Counting… Cancel'"
                        aria-label="Cancel row count" @click="emit('cancelCount')">
                        <q-spinner class="q-ml-xs" size="12px" />
                    </q-btn>
                    <q-btn v-else flat dense no-caps type="button" size="sm"
                        :disable="loading || navigationBlocked"
                        :label="totalRows == null ? 'Count rows' : `${totalRows.toLocaleString()} rows total`"
                        :title="totalRows == null ? 'Calculate total rows' : 'Recalculate total rows'"
                        aria-label="Count total rows" @click="emit('count')" />
                    <span v-if="countError" class="text-red-300 text-xs max-w-40 truncate" :title="countError" role="alert">{{ countError }}</span>
                </template>
            </div>
            <div v-if="result.IsQuery" class="flex items-center gap-1">
                <q-btn flat dense no-caps type="button" class="pagination-button" :disable="loading || navigationBlocked || !canFirst"
                    title="Go to first page" aria-label="Go to first page" @click="emit('first')">
                    <q-icon name="first_page" size="16px" />
                </q-btn>
                <!-- Previous -->
                <q-btn flat dense no-caps
                    type="button"
                    class="pagination-button"
                    :disable="loading || navigationBlocked || !canPrevious"
                    :title="result.CanNavigate ? 'Previous page' : 'Previous cached page'"
                    @click="emit('previous')"
                >
                    <q-icon name="chevron_left" size="16px" />
                </q-btn>

                <!-- Page -->
                <div
                    class="mx-1 flex h-[24px] min-w-[80px] items-center justify-center border border-[#484848] bg-[#2d2d2d] px-2 font-mono text-[10px] text-gray-400"
                >
                    Page {{ currentPage }}
                </div>

                <!-- Next -->
                <q-btn flat dense no-caps
                    type="button"
                    class="pagination-button"
                    :disable="loading || navigationBlocked || !canNext"
                    title="Next page"
                    @click="emit('next')"
                >
                    <q-icon name="chevron_right" size="16px" />
                </q-btn>

                <q-btn flat dense no-caps type="button" class="pagination-button"
                    :disable="loading || countingRows || navigationBlocked || !canLast"
                    :title="result.CanNavigate ? (totalRows == null ? 'Count rows and go to last page' : 'Go directly to last page') : 'Go to last page (fetches remaining rows)'" aria-label="Go to last page" @click="emit('last')">
                    <q-icon name="last_page" size="16px" />
                </q-btn>
                <q-btn flat dense no-caps v-if="fetchingLast" type="button" class="text-xs text-amber-400 px-2"
                    @click="emit('stop')">Stop fetching</q-btn>
                <span class="ml-2 font-mono text-[10px] text-gray-500">{{ result.PageSize }} rows/page</span>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import {
    computed,
    nextTick,
    onBeforeUnmount,
    onMounted,
    reactive,
    ref,
    watch,
} from "vue";

import type { QTableColumn } from "quasar";

import TableDataEditor from "./TableDataEditor.vue";
import ResultFilterInput from "./ResultFilterInput.vue";
import DataTransfer from "../DataTransfer.vue";
import { useResultSettingsStore } from "@/stores/resultSettingsStore";
const resultSettings = useResultSettingsStore();
import { useTableEditsStore } from "@/stores/tableEditsStore";
import type { QueryResult } from "@/types/queryTab";
import { compileResultFilter } from "@/utils/resultFilter";
import { sortResultRows, orderResultRows, parseResultOrder } from "@/utils/resultSort";

const documentDialog = ref(false);
const selectedDocument = ref("");
const documentTitle = ref("JSON value");
function isJSONColumn(type?: string): boolean {
    return /(?:^|[.\s_])jsonb?(?:\[\])?$/i.test(type?.trim() ?? "");
}
function inspectDocument(value: unknown, type?: string) {
    documentTitle.value = isMongoResult.value ? "MongoDB document" : "JSON value";
    if (typeof value === "string") {
        if (type === "string") selectedDocument.value = value;
        else {
            try { selectedDocument.value = JSON.stringify(JSON.parse(value), null, 2); }
            catch { selectedDocument.value = value; }
        }
    } else {
        selectedDocument.value = value === undefined ? "undefined" : JSON.stringify(value, null, 2);
    }
    documentDialog.value = true;
}

const tableEdits = useTableEditsStore();
const editState = computed(() => props.tableTabId ? tableEdits.states[props.tableTabId] : undefined);
const isMongoResult = computed(() => props.result.Documents !== undefined
    || editState.value?.info?.driver === "mongodb"
    || props.result.Columns.some(column => column.Type === "Extended JSON"));
const navigationBlocked = computed(() => !!editState.value?.drafts.length || !!editState.value?.saving);
const selectedRowKey = ref("");
const rowEditor = ref<InstanceType<typeof TableDataEditor> | null>(null);

const MIN_COLUMN_WIDTH = 60;
const snColumnWidth = computed(() => Math.max(56, String(props.totalRows ?? (props.result.StartRow + props.result.Rows.length - 1)).length * 8 + 24));
const HEADER_EXTRA_WIDTH = 24;

const props = defineProps<{
    result: QueryResult;
    tableTabId?: string;
    loading?: boolean;
    canFirst?: boolean;
    canPrevious?: boolean;
    canNext?: boolean;
    canLast?: boolean;
    totalRows?: number;
    countingRows?: boolean;
    stoppingCount?: boolean;
    countError?: string;
    fetchingLast?: boolean;
    sortColumn?: number;
    sortDirection?: "asc" | "desc";
}>();

const emit = defineEmits<{
    first: [];
    previous: [];
    next: [];
    last: [];
    stop: [];
    refresh: [];
    count: [];
    cancelCount: [];
    sort: [column: number, direction?: "asc" | "desc"];
}>();

/* =========================================================
   COLUMN WIDTHS
   ========================================================= */

const columnWidths = reactive<Record<string, number>>({});
const gridElement = ref<HTMLElement | null>(null);
const tableWidth = computed(() => snColumnWidth.value + props.result.Columns.reduce(
    (width, _column, index) => width + getColumnWidth(`column-${index}`), 0,
));

const getColumnWidth = (columnName: string): number => {
    if (columnName === "sn") {
        return snColumnWidth.value;
    }

    return columnWidths[columnName] ?? MIN_COLUMN_WIDTH;
};

const getColumnStyle = (columnName: string) => {
    const width = getColumnWidth(columnName);

    return {
        width: `${width}px`,
        minWidth: `${width}px`,
        maxWidth: `${width}px`,
    };
};

/* =========================================================
   SAFE CELL TITLE
   ========================================================= */

const getCellTitle = (value: unknown): string => {
    if (value === null || value === undefined) {
        return "";
    }

    return String(value);
};

/* =========================================================
   HEADER WIDTH MEASUREMENT
   ========================================================= */

const measureHeaderWidths = async () => {
    await nextTick();

    await new Promise<void>((resolve) => {
        requestAnimationFrame(() => resolve());
    });

    const table = gridElement.value?.querySelector(
        ".data-explorer-grid table",
    );

    if (!table) {
        return;
    }

    const headers = Array.from(table.querySelectorAll<HTMLElement>("thead th"))
        .slice(1)
        .map((header, index) => ({
            header, index, content: header.querySelector<HTMLElement>("div"),
            width: header.style.width, minWidth: header.style.minWidth, maxWidth: header.style.maxWidth,
        }))
        .filter(item => item.content && props.result.Columns[item.index] && columnWidths[`column-${item.index}`] === undefined);
    if (!headers.length) return;

    // Batch writes, reads, then restoration: one table layout instead of one
    // forced layout per column. Preserve any widths the user has resized.
    const tableElement = table as HTMLElement;
    const oldLayout = tableElement.style.tableLayout;
    tableElement.style.tableLayout = "auto";
    for (const { header } of headers) {
        header.style.width = "max-content";
        header.style.minWidth = "max-content";
        header.style.maxWidth = "none";
    }
    const widths = headers.map(({ header, content }) =>
        Math.max(MIN_COLUMN_WIDTH, Math.max(header.scrollWidth, content!.scrollWidth) + HEADER_EXTRA_WIDTH));
    tableElement.style.tableLayout = oldLayout;
    headers.forEach(({ header, index, width, minWidth, maxWidth }, position) => {
        header.style.width = width;
        header.style.minWidth = minWidth;
        header.style.maxWidth = maxWidth;
        columnWidths[`column-${index}`] = widths[position];
    });
};

onMounted(() => {
    measureHeaderWidths();
});

/* =========================================================
   COLUMN WATCH
   ========================================================= */

const columnSignature = computed(() =>
    props.result.Columns
        .map(
            (column) =>
                `${column.Name}:${column.Type}`,
        )
        .join("|"),
);

watch(columnSignature, async () => {
    const validColumns = new Set(
        props.result.Columns.map(
            (_column, index) => `column-${index}`,
        ),
    );

    Object.keys(columnWidths).forEach(
        (columnName) => {
            if (!validColumns.has(columnName)) {
                delete columnWidths[columnName];
            }
        },
    );



    await nextTick();

    await measureHeaderWidths();
});

/* =========================================================
   COLUMN RESIZING
   ========================================================= */

let resizingColumn: string | null = null;

let resizeStartX = 0;

let resizeStartWidth = 0;
let resizeHandle: HTMLElement | null = null;
let resizePointerId: number | null = null;

const startResize = (
    event: PointerEvent,
    columnName: string,
) => {
    if (event.button !== 0) return;
    stopResize();
    event.preventDefault();
    event.stopPropagation();

    resizingColumn = columnName;

    resizeStartX = event.clientX;

    const target =
        event.currentTarget as HTMLElement;
    resizeHandle = target;
    resizePointerId = event.pointerId;
    target.setPointerCapture(event.pointerId);

    const header =
        target.closest("th") as HTMLElement | null;

    resizeStartWidth =
        header?.getBoundingClientRect().width ??
        getColumnWidth(columnName);

    columnWidths[columnName] =
        resizeStartWidth;

    document.body.classList.add(
        "resizing-column",
    );

    document.addEventListener(
        "pointermove",
        handleResize,
    );

    document.addEventListener(
        "pointerup",
        stopResize,
    );
    document.addEventListener("pointercancel", stopResize);
    target.addEventListener("lostpointercapture", stopResize);
    window.addEventListener("blur", stopResize);
};

const handleResize = (event: PointerEvent) => {
    if (!resizingColumn || event.pointerId !== resizePointerId) {
        return;
    }

    const delta =
        event.clientX - resizeStartX;

    const newWidth = Math.max(
        MIN_COLUMN_WIDTH,
        resizeStartWidth + delta,
    );

    columnWidths[resizingColumn] =
        newWidth;
};

const stopResize = () => {
    if (resizeHandle && resizePointerId !== null) {
        resizeHandle.removeEventListener("lostpointercapture", stopResize);
        if (resizeHandle.hasPointerCapture(resizePointerId)) {
            resizeHandle.releasePointerCapture(resizePointerId);
        }
    }
    resizeHandle = null;
    resizePointerId = null;
    resizingColumn = null;

    document.body.classList.remove(
        "resizing-column",
    );

    document.removeEventListener(
        "pointermove",
        handleResize,
    );

    document.removeEventListener(
        "pointerup",
        stopResize,
    );
    document.removeEventListener("pointercancel", stopResize);
    window.removeEventListener("blur", stopResize);
};

onBeforeUnmount(() => {
    stopResize();
});

/* =========================================================
   SORTING
   ========================================================= */

const pageSort = ref<{ column: number; direction?: "asc" | "desc" } | null>(null);
const effectiveSort = computed(() => orderTerms.value.length ? orderTerms.value[0] : (appliedFilter.value || navigationBlocked.value) && pageSort.value
    ? pageSort.value : { column: props.sortColumn ?? 0, direction: props.sortDirection });
const activeSortColumn = computed(() => effectiveSort.value.column ? `column-${effectiveSort.value.column - 1}` : null);
const sortDirection = computed(() => effectiveSort.value.direction ?? null);

const sortColumn = (columnName: string, direction: "asc" | "desc") => {
    if (props.loading || !(props.result.CanSort ?? props.result.CanNavigate) || columnName === "sn") return;
    const column = Number(columnName.slice(7)) + 1;
    clearOrder();
    if (appliedFilter.value || navigationBlocked.value) {
        pageSort.value = { column, direction };
        return;
    }
    emit("sort", column, direction);
};
const clearSort = () => {
    if (props.loading || !(props.result.CanSort ?? props.result.CanNavigate)) return;
    clearOrder();
    if (appliedFilter.value || navigationBlocked.value) pageSort.value = { column: 0 };
    else emit("sort", 0);
};
const toggleSort = (columnName: string) => {
    if (columnName === "sn" || props.loading || !(props.result.CanSort ?? props.result.CanNavigate)) return;
    if (activeSortColumn.value !== columnName) sortColumn(columnName, "asc");
    else if (sortDirection.value === "asc") sortColumn(columnName, "desc");
    else clearSort();
};

const filterDraft = ref("");
const appliedFilter = ref("");
const filterError = ref("");
const orderDraft = ref("");
const appliedOrder = ref("");
const orderError = ref("");
const orderTerms = computed(() => {
    try { return parseResultOrder(appliedOrder.value, props.result.Columns.map(column => column.Name)); }
    catch { return []; }
});
function clearOrder() {
    orderDraft.value = "";
    appliedOrder.value = "";
    orderError.value = "";
}
const filterPredicate = computed(() => {
    try { return compileResultFilter(appliedFilter.value, props.result.Columns.map(column => column.Name)); }
    catch { return null; }
});
function applyExpressions() {
    if (props.loading || editState.value?.saving) return;
    filterError.value = "";
    orderError.value = "";
    try {
        const predicate = compileResultFilter(filterDraft.value, props.result.Columns.map(column => column.Name));
        props.result.Rows.forEach(row => predicate(row));
    } catch (error) {
        filterError.value = error instanceof Error ? error.message : String(error);
    }
    try {
        parseResultOrder(orderDraft.value, props.result.Columns.map(column => column.Name));
    } catch (error) {
        orderError.value = error instanceof Error ? error.message : String(error);
    }
    if (filterError.value || orderError.value) return;
    appliedFilter.value = filterDraft.value.trim();
    appliedOrder.value = orderDraft.value.trim();
    if (!appliedFilter.value) pageSort.value = null;
}
function clearFilter() {
    filterDraft.value = "";
    appliedFilter.value = "";
    pageSort.value = null;
    filterError.value = "";
}
watch(() => props.result, () => {
    try {
        parseResultOrder(appliedOrder.value, props.result.Columns.map(column => column.Name));
        orderError.value = "";
    } catch (error) {
        orderError.value = error instanceof Error ? error.message : String(error);
    }
    try {
        const predicate = compileResultFilter(appliedFilter.value, props.result.Columns.map(column => column.Name));
        props.result.Rows.forEach(row => predicate(row));
        filterError.value = "";
    } catch (error) {
        filterError.value = error instanceof Error ? error.message : String(error);
    }
});
const workingRows = computed(() => {
    const drafts = editState.value?.drafts ?? [];
    const rows = props.result.Rows.map((original, index) => {
        const key = props.tableTabId ? tableEdits.rowKey(props.tableTabId, props.result, index) : String(props.result.StartRow + index);
        const draft = drafts.find(draft => draft.key === key);
        const row = props.result.Columns.map((column, col) => {
            const value = draft && Object.hasOwn(draft.values, column.Name) ? draft.values[column.Name] : original[col];
            return value != null && typeof value === 'object' ? JSON.stringify(value) : value;
        });
        return { row, index, key, status: draft?.operation ?? '', number: String(props.result.StartRow + index) };
    });
    for (const draft of drafts.filter(draft => !rows.some(row => row.key === draft.key))) {
        rows.push({ row: props.result.Columns.map(column => {
            const value = Object.hasOwn(draft.values, column.Name) ? draft.values[column.Name] : draft.original?.[column.Name];
            return value != null && typeof value === 'object' ? JSON.stringify(value) : value;
        }), index: rows.length, key: draft.key, status: draft.operation, number: draft.operation === 'insert' ? 'New' : 'Pending' });
    }
    return rows;
});
const filteredRows = computed(() => {
    const rows = workingRows.value.filter(({ row, status }) => {
        // Pending changes remain visible until saved or canceled.
        if (status) return true;
        try { return filterPredicate.value?.(row) ?? true; }
        catch { return true; }
    });
    if (orderTerms.value.length) return orderResultRows(rows, orderTerms.value);
    return (appliedFilter.value || navigationBlocked.value) && pageSort.value
        ? sortResultRows(rows, pageSort.value.column, pageSort.value.direction)
        : rows;
});

// Stable row identities preserve selection while values are staged or sorted.
const mappedRows = computed(() => filteredRows.value.map(({ row, key, status, number }) => {
    const mapped: Record<string, unknown> = { id: key, _editKey: key, _status: status, _rowNumber: number };
    row.forEach((value, column) => { mapped[`column-${column}`] = value; });
    return mapped;
}));

/* =========================================================
   PAGINATION
   ========================================================= */

const rowOffset = computed(() => Math.max(0, props.result.StartRow - 1));
const currentPage = computed(() => Math.floor(rowOffset.value / props.result.PageSize) + 1);

/* =========================================================
   COLUMNS
   ========================================================= */

const mappedColumns =
    computed<QTableColumn[]>(() => {
        return [
            {
                name: "sn",

                label: "#",

                field: "sn",

                align: "left",

                sortable: false,

                style: `
                    width: ${snColumnWidth.value}px;
                    min-width: ${snColumnWidth.value}px;
                    max-width: ${snColumnWidth.value}px;
                `,

                headerStyle: `
                    width: ${snColumnWidth.value}px;
                    min-width: ${snColumnWidth.value}px;
                    max-width: ${snColumnWidth.value}px;
                `,
            },

            ...props.result.Columns.map(
                (column, index) => {
                    const key = `column-${index}`;
                    const width =
                        getColumnWidth(
                            key,
                        );

                    return {
                        name: key,

                        label: column.Name,

                        field: key,

                        align: "left" as const,

                        sortable: false,

                        Type: column.Type,

                        Nullable:
                            column.Nullable,

                        DefaultValue:
                            column.DefaultValue,

                        style: `
                            width: ${width}px;
                            min-width: ${width}px;
                            max-width: ${width}px;
                        `,

                        headerStyle: `
                            width: ${width}px;
                            min-width: ${width}px;
                            max-width: ${width}px;
                        `,
                    };
                },
            ),
        ];
    });
</script>

<style scoped>
.result-filter-bar {
    position: absolute;
    inset: 0 0 auto;
    height: 36px;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 8px;
    background: #353535;
    border-bottom: 1px solid #484848;
    font-family: monospace;
    font-size: 11px;
}
.result-filter-input {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
    min-width: 0;
    height: 25px;
    padding: 0 8px;
    border: 1px solid #484848;
    background: #2d2d2d;
}
.result-filter-input:focus-within { border-color: #a97724; }
.result-filter-input.has-error { border-color: #b45353; }
.filter-expression { flex: 1; min-width: 0; }
.filter-expression :deep(.q-field__control), .filter-expression :deep(.q-field__marginal) { min-height: 23px; height: 23px; }
.filter-expression :deep(.q-field__native) { padding: 0; color: #d1d5db; }
.filter-expression :deep(input::placeholder) { color: #a0a0a0; }
.filter-action { min-height: 25px; padding: 0; display: flex; align-items: center; justify-content: center; width: 25px; height: 25px; flex-shrink: 0; color: #9ca3af; border: 1px solid transparent; }
.filter-action:hover:not(.disabled), .filter-action:focus-visible { color: #fbbf24; background: #484848; border-color: #555555; }
.filter-action.disabled { opacity: .3; cursor: default; }
.filter-count { color: #a0a0a0; white-space: nowrap; padding-left: 6px; }
@media (max-width: 600px) { .filter-scope { display: none; } }

.row-number {
    display: block;
    color: #94a3b8;
    font-variant-numeric: tabular-nums;
    text-align: right;
    white-space: nowrap;
    font-size: var(--grid-font-size, 11px);
}

/* =========================================================
   RESIZE HANDLE
   ========================================================= */

.column-resizer {
    position: absolute;

    top: 0;
    right: 0;
    bottom: 0;

    width: 8px;

    cursor: col-resize;
    touch-action: none;
    user-select: none;

    z-index: 70;
}

.column-resizer::after {
    content: "";

    position: absolute;

    top: 3px;
    bottom: 3px;
    left: 3px;

    width: 1px;

    background: #5a5a5a;

    transition:
        background-color 80ms ease,
        width 80ms ease;
}

.column-resizer:hover::after {
    width: 2px;

    background: #f59e0b;
}

/* =========================================================
   RESIZING STATE
   ========================================================= */

:global(body.resizing-column) {
    cursor: col-resize !important;

    user-select: none !important;
}

:global(body.resizing-column *) {
    cursor: col-resize !important;

    user-select: none !important;
}

/* =========================================================
   QTABLE CONTAINER
   ========================================================= */

.data-explorer-grid {
    position: absolute;
    inset: 0 0 36px;
    min-width: 0;
    min-height: 0;
}

.data-explorer-grid.has-edit-toolbar { top: 70px !important; }
.selected-edit-row :deep(td) { box-shadow: inset 0 1px #70572d, inset 0 -1px #70572d; }
.pending-insert :deep(td) { background: #12281f !important; }
.pending-update :deep(td) { background: #2b2415 !important; }
.pending-delete :deep(td) { background: #321c1c !important; text-decoration: line-through; opacity: .65; }

.data-explorer-grid.has-filter-toolbar {
    top: 36px;
}

.data-explorer-grid :deep(.q-table__card),
.data-explorer-grid :deep(.q-table__container) {
    background: transparent !important;

    box-shadow: none !important;
}

.data-explorer-grid :deep(.q-table__middle) {
    background: #2d2d2d;

    min-width: 0;
    min-height: 0;
    max-width: 100%;

    overflow: auto;

    scrollbar-width: thin;

    scrollbar-color:
        #484848
        #2d2d2d;
}

/* =========================================================
   TABLE LAYOUT
   ========================================================= */

.data-explorer-grid :deep(table) {
    table-layout: fixed;
    width: var(--result-table-width);
    min-width: var(--result-table-width);
    max-width: var(--result-table-width);

}

/* =========================================================
   ALL CELLS
   ========================================================= */

.data-explorer-grid :deep(th),
.data-explorer-grid :deep(td) {
    font-size: var(--grid-font-size, 11px);
    box-sizing: border-box;

    min-width: 0;

    overflow: hidden;

    white-space: nowrap;

    text-overflow: ellipsis;

    user-select: text;
}

/* =========================================================
   STICKY HEADER
   ========================================================= */

.data-explorer-grid :deep(thead) {
    position: sticky;

    top: 0;

    z-index: 40;
}

.data-explorer-grid :deep(thead th) {
    position: sticky;

    top: 0;

    z-index: 40;

    background: #353535;

    overflow: hidden;

    white-space: nowrap;

    text-overflow: ellipsis;
}

/* =========================================================
   STICKY # HEADER
   ========================================================= */

.data-explorer-grid :deep(thead th:first-child) {
    padding-left: 8px;
    padding-right: 8px;
    position: sticky;

    left: 0;

    top: 0;

    z-index: 60;

    width: var(--row-number-width);

    min-width: var(--row-number-width);

    max-width: var(--row-number-width);

    background: #353535;

    border-right: 1px solid #505050;
}

/* =========================================================
   STICKY # BODY
   ========================================================= */

.data-explorer-grid :deep(tbody td:first-child) {
    padding-left: 8px;
    padding-right: 8px;
    position: sticky;

    left: 0;

    z-index: 20;

    width: var(--row-number-width);

    min-width: var(--row-number-width);

    max-width: var(--row-number-width);

    overflow: visible;
    text-overflow: clip;

    background: #2d2d2d;

    border-right: 1px solid #484848;
}

/* =========================================================
   ALTERNATING ROWS
   ========================================================= */

.striped-rows .data-explorer-grid
    :deep(tbody tr.striped-row:not(:hover):not(.pending-insert):not(.pending-update):not(.pending-delete) td) {
    background: #353535;
}

/* =========================================================
   HOVER
   ========================================================= */

.data-explorer-grid
    :deep(tbody tr:hover td:first-child) {
    background: #404040;
}

/* =========================================================
   BODY CELLS
   ========================================================= */

.data-explorer-grid :deep(tbody td) {
    overflow: hidden;

    white-space: nowrap;

    text-overflow: ellipsis;
}

.data-explorer-grid
    :deep(tbody td > div) {
    display: block;

    width: 100%;

    min-width: 0;

    max-width: 100%;

    overflow: hidden;

    white-space: nowrap;

    text-overflow: ellipsis;
}

/* =========================================================
   COLUMN BORDERS
   ========================================================= */

.data-explorer-grid :deep(th + th),
.data-explorer-grid :deep(td + td) {
    border-left: 1px solid #484848;
}

.data-explorer-grid
    :deep(thead th + th) {
    border-left-color: #505050;
}

/* =========================================================
   STICKY COLUMN SHADOW
   ========================================================= */

.data-explorer-grid
    :deep(thead th:first-child),
.data-explorer-grid
    :deep(tbody td:first-child) {
    box-shadow:
        2px 0 3px
        rgba(0, 0, 0, 0.18);
}

/* =========================================================
   SCROLLBAR
   ========================================================= */

.data-explorer-grid
    :deep(.q-table__middle::-webkit-scrollbar) {
    width: 8px;

    height: 8px;
}

.data-explorer-grid
    :deep(.q-table__middle::-webkit-scrollbar-track) {
    background: #2d2d2d;
}

.data-explorer-grid
    :deep(.q-table__middle::-webkit-scrollbar-thumb) {
    background: #484848;

    border-radius: 4px;
}

.data-explorer-grid
    :deep(.q-table__middle::-webkit-scrollbar-thumb:hover) {
    background: #5a5a5a;
}

/* =========================================================
   PAGINATION FOOTER
   ========================================================= */

.data-grid-footer {
    box-shadow:
        0 -2px 8px
        rgba(0, 0, 0, 0.25);
}

/* =========================================================
   PAGINATION BUTTONS
   ========================================================= */

.pagination-button {
    min-height: 24px;
    padding: 0;
    display: flex;

    height: 24px;
    width: 24px;

    align-items: center;
    justify-content: center;

    border: 1px solid #484848;

    background: #2d2d2d;

    color: #a0a0a0;

    cursor: pointer;

    transition:
        background-color 80ms ease,
        color 80ms ease,
        border-color 80ms ease;
}

.pagination-button:hover:not(
        .disabled
    ) {
    background: #484848;

    border-color: #5a5a5a;

    color: #f59e0b;
}

.pagination-button.disabled {
    cursor: not-allowed;

    opacity: 0.3;
}

/* =========================================================
   SELECT
   ========================================================= */

.data-grid-footer select {
    cursor: pointer;
}

.data-grid-footer select:focus {
    border-color: #92400e;
}

/* =========================================================
   REFRESH ANIMATION
   ========================================================= */

.animate-spin {
    animation: spin 0.8s linear infinite;
}

@keyframes spin {
    from {
        transform: rotate(0deg);
    }

    to {
        transform: rotate(360deg);
    }
}
/* Virtual padding rows reserve scroll height; they are not sticky data cells. */
.data-explorer-grid :deep(.q-virtual-scroll__padding td) {
    position: static;
    padding: 0 !important;
    border: 0;
    min-width: 0;
    max-width: none;
    width: auto;
}
</style>
