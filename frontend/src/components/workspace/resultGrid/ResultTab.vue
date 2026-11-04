<template>
    <div ref="gridElement" class="relative h-full w-full overflow-hidden bg-[var(--surface-data)]" :class="{ 'striped-rows': resultSettings.settings.stripedRows }" :style="{ '--grid-font-size': `${resultSettings.settings.fontSize}px` }">
        <q-form v-if="result.IsQuery" class="result-filter-bar" @submit.prevent="applyExpressions">
            <q-icon name="filter_alt" size="15px" :class="appliedFilter ? 'text-primary' : 'text-gray-500'" />
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
                <q-menu class="border border-[var(--border-default)] bg-[var(--surface-panel)] text-gray-300">
                    <div class="max-w-sm p-3 font-mono text-[length:var(--ui-font-size)] leading-6">
                        <div class="text-primary">Combine conditions with AND / OR</div>
                        <div>age &gt;= 18 AND status = 'active'</div>
                        <div>(name LIKE 'A%' OR name LIKE 'B%')</div>
                        <div>status IN ('active', 'pending')</div>
                        <div>deleted_at IS NULL</div>
                        <div class="mt-2 text-primary">ORDER BY: name ASC, created_at DESC</div>
                        <div class="mt-2 text-gray-500">Use your database's identifier quoting for WHERE, or column numbers in ORDER BY.<br />Enter to apply · Esc to reset the focused field<br />{{ serverExpressions ? 'WHERE and ORDER BY apply to the database result before pagination.' : 'WHERE and ORDER BY apply to the current page for this result.' }}</div>
                        <div class="mt-2 text-gray-500">Columns: {{ result.Columns.map(column => column.Name).join(', ') }}</div>
                    </div>
                </q-menu>
            </q-btn>
            <q-btn flat dense icon="view_column" size="sm" aria-label="Manage columns" title="Columns">
                <q-menu dark class="bg-[var(--surface-panel)] text-gray-300" style="width: 360px; max-height: 70vh">
                    <div class="q-pa-sm">
                        <q-input v-model="columnLayout.search.value" dark dense outlined clearable placeholder="Find column" />
                        <div class="flex"><q-btn flat dense no-caps label="Reset layout" @click="columnLayout.reset()" /><q-btn flat dense no-caps label="Fit widths" @click="fitColumnWidths" /></div>
                        <div class="text-[length:var(--ui-font-size)] text-gray-400">Hide, pin left, or reorder columns. Saved for this table.</div>
                    </div>
                    <div v-for="(index, position) in columnLayout.order.value" :key="index" v-show="result.Columns[index]?.Name.toLowerCase().includes((columnLayout.search.value || '').toLowerCase())" class="flex items-center px-2">
                        <q-checkbox dark dense :model-value="!columnLayout.hidden.value.includes(index)" :disable="columnLayout.visible.value.length === 1 && !columnLayout.hidden.value.includes(index)" :label="result.Columns[index]?.Name" class="flex-1 min-w-0" @update:model-value="columnLayout.toggleHidden(index)" />
                        <q-btn flat dense size="sm" icon="push_pin" :color="columnLayout.pinned.value.includes(index) ? 'primary' : 'grey'" :aria-label="`Pin ${result.Columns[index]?.Name}`" @click="columnLayout.togglePinned(index)" />
                        <q-btn flat dense size="sm" icon="arrow_upward" :disable="position === 0" aria-label="Move column earlier" @click="columnLayout.move(index, -1)" />
                        <q-btn flat dense size="sm" icon="arrow_downward" :disable="position === columnLayout.order.value.length - 1" aria-label="Move column later" @click="columnLayout.move(index, 1)" />
                    </div>
                </q-menu>
            </q-btn>
            <span class="filter-count">{{ serverExpressions ? workingRows.length : `${filteredRows.length}/${workingRows.length}` }} <span class="filter-scope">on this page</span></span>
        </q-form>

        <TableDataEditor v-if="tableTabId" ref="rowEditor" :tab-id="tableTabId" :result="result" :selected-key="selectedRowKey" :row-indexes="sourceRowIndexes" :loading="loading" :prepare-save="commitInline" @edit-cell="editInlineRow" @add-row="addInlineRow" @duplicate-row="duplicateInlineRow" @cancel-edit="cancelInline" />

        <!-- =========================================================
             TABLE
             ========================================================= -->

        <q-table
            v-show="!recordViewOpen"
            ref="dataTable"
            role="grid"
            tabindex="0"
            aria-label="Query results. Use arrows to move, Shift with arrows or click to select a range, Ctrl+C to copy, Ctrl+V to paste, and Tab for row details."
            aria-multiselectable="true"
            :aria-activedescendant="activeCellId"
            :aria-rowcount="mappedRows.length + 1"
            :aria-colcount="displayedColumns.length + 1"
            @keydown="handleResultKeydown"
            @copy="copyCells"
            @paste="pasteCells"
            @scroll.capture.passive="cellContextMenu?.hide()"
            flat
            square
            dense
            dark
            :rows="visibleRows"
            :columns="mappedColumns"
            row-key="id"
            :pagination="{
                rowsPerPage: 0,
            }"
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
                <q-tr :props="props" class="bg-[var(--surface-panel)]">
                    <q-th
                        v-for="col in props.cols"
                        :key="col.name"
                        :props="props"
                        class="relative box-border h-[28px] overflow-hidden whitespace-nowrap border-b-2 border-[var(--border-default)] px-2 py-0 align-middle font-mono text-[length:var(--ui-font-size)] font-bold text-primary"
                        :class="{
                            'cursor-pointer': col.name !== 'sn' && (result.CanSort ?? result.CanNavigate),
                            'sticky-col-header pl-2 pr-1 text-left font-normal text-[var(--text-muted)]':
                                col.name === 'sn',
                        }"
                        :style="getColumnStyle(col.name)"
                        :data-pinned="isPinned(col.name) || undefined"
                        :tabindex="col.name !== 'sn' && (result.CanSort ?? result.CanNavigate) ? 0 : undefined"
                        :aria-sort="activeSortColumn === col.name ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'"
                        @click="toggleSort(col.name)"
                        @keydown.enter.prevent="toggleSort(col.name)"
                    >
                        <div
                            class="flex flex-nowrap w-full min-w-0 items-center overflow-hidden whitespace-nowrap"
                        >
                            <div
                                class="flex flex-nowrap min-w-0 flex-1 items-center overflow-hidden pr-1 whitespace-nowrap"
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
                                    class="ml-1 min-w-0 shrink overflow-hidden text-ellipsis whitespace-nowrap text-[length:calc(var(--grid-font-size)*0.8)] font-normal text-gray-500"
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
                                class="ml-0.5 shrink-0 text-primary"
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
                            class="bg-[var(--surface-panel)] text-gray-300 shadow-lg"
                        >
                            <div class="font-mono text-[length:var(--ui-font-size)]">
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
                            v-if="col.name !== 'sn'"
                            context-menu
                            class="min-w-[150px] border border-[var(--border-default)] bg-[var(--surface-panel)] text-gray-300 shadow-xl"
                        >
                            <q-item clickable v-close-popup dense @click="copyColumnName(col.label)">
                                <q-item-section avatar class="min-w-[24px]"><q-icon name="content_copy" size="14px" /></q-item-section>
                                <q-item-section>Copy column name</q-item-section>
                            </q-item>
                            <template v-if="result.CanSort ?? result.CanNavigate">
                            <q-separator class="my-1 bg-[var(--border-default)]" />
                            <!-- Sort Ascending -->
                            <q-item
                                clickable
                                v-close-popup
                                dense
                                class="min-h-[28px] px-2 hover:bg-[var(--border-default)]"
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
                                    <q-item-label class="font-mono text-[length:var(--ui-font-size)]">
                                        Sort ascending
                                    </q-item-label>
                                </q-item-section>
                            </q-item>

                            <!-- Sort Descending -->
                            <q-item
                                clickable
                                v-close-popup
                                dense
                                class="min-h-[28px] px-2 hover:bg-[var(--border-default)]"
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
                                    <q-item-label class="font-mono text-[length:var(--ui-font-size)]">
                                        Sort descending
                                    </q-item-label>
                                </q-item-section>
                            </q-item>

                            <q-separator class="my-1 bg-[var(--border-default)]" />

                            <!-- Clear Sort -->
                            <q-item
                                v-if="activeSortColumn === col.name"
                                clickable
                                v-close-popup
                                dense
                                class="min-h-[28px] px-2 hover:bg-[var(--border-default)]"
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
                                    <q-item-label class="font-mono text-[length:var(--ui-font-size)]">
                                        Clear Sort
                                    </q-item-label>
                                </q-item-section>
                            </q-item>
                            </template>
                        </q-menu>
                    </q-th>
                </q-tr>
            </template>

            <!-- =====================================================
                 BODY
                 ===================================================== -->

            <template #top-row>
                <tr v-if="paddingTop" class="q-virtual-scroll__padding" aria-hidden="true">
                    <td :colspan="mappedColumns.length" :style="{ height: `${paddingTop}px` }" />
                </tr>
            </template>
            <template #bottom-row>
                <tr v-if="paddingBottom" class="q-virtual-scroll__padding" aria-hidden="true">
                    <td :colspan="mappedColumns.length" :style="{ height: `${paddingBottom}px` }" />
                </tr>
            </template>
            <template #body="props">
                <tr :key="props.row.id" :aria-rowindex="firstRow + props.rowIndex + 2"
                    class="q-tr bg-[var(--surface-data)] hover:!bg-[var(--surface-hover)]"
                    :class="{ 'striped-row': resultSettings.settings.stripedRows && (firstRow + props.rowIndex) % 2 === 1, 'selected-edit-row': selectedRowKey === props.row._editKey, 'pending-insert': props.row._status === 'insert', 'pending-update': props.row._status === 'update', 'pending-delete': props.row._status === 'delete' }"
                    @click="selectedRowKey = props.row._editKey || ''"

                >
                    <td
                        v-for="col in props.cols"
                        :key="col.name"
                        class="q-td text-left box-border overflow-hidden whitespace-nowrap border-b border-[var(--border-default)] px-2 py-1.5 font-mono text-[length:var(--ui-font-size)] text-gray-300"
                        :class="{
                            'sticky-col-body pl-2 pr-1 text-left text-[var(--text-muted)]':
                                col.name === 'sn',
                            'active-result-cell': activeCell?.rowKey === String(props.row.id) && col.name === `column-${activeCell.column}`,
                            'selected-result-cell': isCellSelected(firstRow + props.rowIndex, col.name),
                        }"
                        :id="col.name !== 'sn' ? cellId(firstRow + props.rowIndex, Number(col.name.slice(7))) : undefined"
                        :role="col.name === 'sn' ? 'rowheader' : 'gridcell'"
                        :data-grid-row="col.name !== 'sn' ? String(props.row.id) : undefined"
                        :data-grid-column="col.name !== 'sn' ? col.name : undefined"
                        :aria-selected="col.name !== 'sn' ? isCellSelected(firstRow + props.rowIndex, col.name) : undefined"
                        :style="getColumnStyle(col.name)"
                        :data-pinned="isPinned(col.name) || undefined"
                        @pointerdown="startCellDrag($event, String(props.row.id), col.name)"
                        @dblclick.stop="editInlineCell(String(props.row.id), col.name)"
                        @contextmenu="openCellMenu($event, props.row, col)"
                        @click="props.row._status === 'insert' && col.name !== 'sn' ? editInlineCell(String(props.row.id), col.name) : col.name !== 'sn' && $event.detail === 3 && !rowEditor?.canEditRow(String(props.row._editKey)) && inspectDocument(props.row[col.field], col.Type, col.label)"
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
                            <div v-if="inlineEdit?.key === String(props.row.id) && inlineEdit.column === col.name" class="inline-cell-editor"
                                @pointerdown.stop @click.stop @dblclick.stop @contextmenu.stop
                                @keydown.stop="inlineKeydown" @focusout="inlineBlur">
                                <textarea :value="inlineEdit.mode === 'value' ? inlineEdit.text : ''" @input="inlineEdit.text = ($event.target as HTMLTextAreaElement).value; inlineEdit.mode = 'value'" data-inline-cell-input rows="1" :aria-label="`Edit ${col.label}`"
                                    :aria-invalid="!!inlineEdit.error"
                                    :placeholder="inlineEdit.mode === 'default' ? 'DEFAULT' : inlineEdit.mode === 'null' ? 'NULL' : ''" />
                                <select v-model="inlineEdit.mode" :aria-label="`${col.label} value mode`">
                                    <option value="value">Value</option>
                                    <option v-if="inlineEdit.nullable" value="null">NULL</option>
                                    <option v-if="inlineEdit.insert" value="default">Default</option>
                                </select>
                                <q-tooltip v-if="inlineEdit.error" :model-value="true" anchor="bottom left" self="top left">{{ inlineEdit.error }}</q-tooltip>
                            </div>
                            <span v-else-if="props.row._status === 'insert' && props.row[col.field] === undefined" class="text-gray-500 italic" :title="canEditInline(String(props.row.id), col.name) ? 'Click or type to enter a value; leave untouched to use the database default' : 'Generated by the database'">DEFAULT</span>
                            <div v-else
                                class="block w-full min-w-0 overflow-hidden text-ellipsis whitespace-nowrap"
                                :title="getCellTitle(props.row[col.field])"
                            >
                                <span v-if="props.row[col.field] == null && resultSettings.settings.showNulls" class="text-gray-500 italic">NULL</span>
                                <template v-else>{{ props.row[col.field] }}</template>
                            </div>
                        </template>
                    </td>
                </tr>
            </template>
        </q-table>

        <!-- One menu for the grid, created only when a cell is opened. -->
        <q-menu v-if="contextCell" ref="cellContextMenu" :target="contextCell.target" no-parent-event context-menu dark class="bg-[var(--surface-panel)] text-gray-300">
            <q-list dense>
                <template v-if="tableTabId">
                    <q-item-label header>Row actions</q-item-label>
                    <q-item clickable v-close-popup :disable="contextCell.status === 'delete' || !rowEditor?.canEditRow(contextCell.rowKey)" @click="editInlineCell(contextCell.rowKey, contextCell.column)">
                        <q-item-section avatar><q-icon name="edit" size="17px" /></q-item-section><q-item-section>{{ isMongoResult ? 'Edit document' : 'Edit cell' }}</q-item-section>
                    </q-item>
                    <q-item clickable v-close-popup :disable="!canDuplicateRow(contextCell.rowKey)" @click="duplicateInlineRow(contextCell.rowKey)">
                        <q-item-section avatar><q-icon name="content_copy" size="17px" /></q-item-section><q-item-section>Duplicate row</q-item-section>
                    </q-item>
                    <q-item clickable v-close-popup :disable="!rowEditor?.canEditRow(contextCell.rowKey)" @click="rowEditor?.deleteRow(contextCell.rowKey)">
                        <q-item-section avatar><q-icon :name="contextCell.status === 'delete' ? 'undo' : 'delete_outline'" size="17px" /></q-item-section>
                        <q-item-section>{{ contextCell.status === 'delete' ? 'Undo delete' : 'Delete row' }}</q-item-section>
                    </q-item>
                    <q-separator v-if="contextCell.column !== 'sn'" />
                </template>
                <template v-if="contextCell.column !== 'sn'">
                <q-item clickable v-close-popup @click="openAdvancedCopy">
                    <q-item-section avatar><q-icon name="content_copy" size="17px" /></q-item-section><q-item-section>Advanced copy…</q-item-section>
                </q-item>
                <q-item clickable v-close-popup @click="copyColumnName(contextCell.label)">
                    <q-item-section avatar><q-icon name="content_copy" size="17px" /></q-item-section>
                    <q-item-section>Copy column name</q-item-section>
                </q-item>
                <q-item clickable v-close-popup @click="inspectDocument(contextCell.value, contextCell.documentType, contextCell.label)">
                    <q-item-section avatar><q-icon name="data_object" size="17px" /></q-item-section><q-item-section>View cell value</q-item-section>
                </q-item>
                <q-item-label header>Quick filter · {{ serverExpressions ? 'database result' : 'current page' }}</q-item-label>
                <q-item v-for="action in quickFilterActions" :key="action.operator" clickable v-close-popup :disable="loading || editState?.saving"
                    @click="filterByCell(contextCell.column, contextCell.value, action.operator)">
                    <q-item-section>{{ action.label }}</q-item-section>
                </q-item>
                <q-item clickable v-close-popup @click="clearFilter"><q-item-section>Clear filter</q-item-section></q-item>
                <RelatedRecords v-if="tableTabId" :tab-id="tableTabId" :column="contextCell.label" :row="contextCell.namedRow" :disabled="!!editState?.drafts.length" />
                </template>
            </q-list>
        </q-menu>

        <ResultRecordView v-if="recordViewOpen && recordRow" class="result-record-view"
            :class="{ 'has-filter-toolbar': result.IsQuery, 'has-edit-toolbar': !!tableTabId }"
            :row="recordRow" :columns="displayedColumns" :position="recordRowIndex" :total="mappedRows.length"
            :selected-column="activeCell?.column ?? 0" @close="closeRecordView" @move="moveRecordRow"
            @select-column="column => selectCell(String(recordRow!.id), `column-${column}`, false)" />

        <AdvancedCopy v-model="advancedCopyOpen" :selection="copySelection" :driver="copyDriver" :table="copyTable" />
        <CellValueViewer v-model="documentDialog" :value="selectedDocument" :type="documentType" :column="documentColumn" />

        <!-- =========================================================
             FIXED BOTTOM TOOLBAR
             ========================================================= -->

        <div
            class="data-grid-footer absolute bottom-0 left-0 right-0 z-[100] flex h-[36px] items-center justify-between border-t border-[var(--border-default)] bg-[var(--surface-panel)] px-2"
        >
            <!-- Left side -->
            <div class="flex items-center gap-2 min-w-0 overflow-hidden">
                <q-btn v-if="result.IsQuery" flat dense no-caps type="button" class="pagination-button"
                    :disable="!mappedRows.length || !result.Columns.length" :title="recordViewOpen ? 'Back to grid (Tab)' : 'Row details (Tab)'"
                    :aria-label="recordViewOpen ? 'Show result grid' : 'Show row details'" :aria-pressed="recordViewOpen"
                    @click="recordViewOpen ? closeRecordView() : openRecordView()">
                    <q-icon :name="recordViewOpen ? 'table_rows' : 'view_agenda'" size="16px" />
                </q-btn>
                <q-btn flat dense no-caps type="button" class="pagination-button" :disable="loading || editState?.saving"
                    :title="tableTabId ? 'Refresh table and discard unsaved changes (Ctrl+R)' : 'Run this query again'" :aria-label="tableTabId ? 'Refresh table and discard unsaved changes' : 'Run this query again'" @mousedown.prevent @click="refreshResult">
                    <q-icon name="refresh" size="16px" />
                </q-btn>
                <q-btn v-if="result.IsQuery" flat dense no-caps type="button" class="pagination-button" icon="content_copy" title="Advanced copy selected cells" aria-label="Advanced copy selected cells" :disable="!selectionRange" @mousedown.prevent @click="openAdvancedCopy" />
                <DataTransfer v-if="result.IsQuery" action="export" :result="result" :table-tab-id="tableTabId"
                    :disabled="loading || navigationBlocked" />
                <span class="font-mono text-[length:var(--ui-font-size)] text-gray-500">
                    <template v-if="result.Rows.length">Rows {{ result.StartRow }}–{{ result.StartRow + result.Rows.length - 1 }}</template>
                    <template v-else>{{ result.IsQuery ? 'No rows' : 'Statement completed' }}</template>
                    <span v-if="result.IsQuery && !result.CanSort && totalRows != null"> · {{ totalRows.toLocaleString() }} rows total</span>
                </span>
                <q-btn v-if="selectionStats.cells" flat dense no-caps type="button" class="selection-stats" icon="functions"
                    :label="selectionStats.count ? `${selectionStats.cells} cells · Σ ${selectionStats.sum}` : `${selectionStats.cells} cells`"
                    title="Selection statistics · loaded rows" aria-label="Show selection statistics">
                    <q-menu dark class="selection-statistics-menu">
                        <div class="statistics-heading">Selection statistics</div>
                        <dl class="statistics-values">
                            <template v-for="item in [['Selected cells', selectionStats.cells], ['Non-NULL', selectionStats.filled], ['Numeric values', selectionStats.count], ['Sum', selectionStats.count ? selectionStats.sum : '—'], ['Average', selectionStats.average || '—'], ['Minimum', selectionStats.min || '—'], ['Maximum', selectionStats.max || '—']]" :key="String(item[0])">
                                <dt>{{ item[0] }}</dt><dd>{{ item[1] }}</dd>
                            </template>
                        </dl>
                        <p v-if="selectionStats.excluded" class="statistics-note">{{ selectionStats.excluded }} unsupported numeric values excluded.</p>
                        <p class="statistics-note">Selected cells in loaded rows. NULLs are excluded from numeric calculations.</p>
                    </q-menu>
                </q-btn>
                <template v-if="result.IsQuery && result.CanSort">
                    <q-btn v-if="countingRows" flat dense no-caps type="button" size="sm" color="primary"
                        :disable="stoppingCount" :label="stoppingCount ? 'Cancelling…' : 'Counting… Cancel'"
                        aria-label="Cancel row count" @click="emit('cancelCount')">
                        <q-spinner class="q-ml-xs" size="12px" />
                    </q-btn>
                    <q-btn v-else flat dense no-caps type="button" size="sm"
                        :disable="loading || navigationBlocked"
                        :label="totalRows == null ? 'Count rows' : `${totalRows.toLocaleString()} rows total`"
                        :title="totalRows == null ? 'Calculate total rows' : 'Recalculate total rows'"
                        aria-label="Count total rows" @click="emit('count')" />
                    <span v-if="countError" class="text-red-300 text-[length:var(--ui-font-size)] max-w-40 truncate" :title="countError" role="alert">{{ countError }}</span>
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
                    class="mx-1 flex h-[24px] min-w-[80px] items-center justify-center border border-[var(--border-default)] bg-[var(--surface-data)] px-2 font-mono text-[length:var(--ui-font-size)] text-gray-400"
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
                    :title="result.CanNavigate ? (totalRows == null ? 'Count rows and go to last page' : 'Go directly to last page') : (canLast ? 'Go to last cached page' : 'Direct last-page navigation is unavailable for this result')" aria-label="Go to last page" @click="emit('last')">
                    <q-icon name="last_page" size="16px" />
                </q-btn>
                <q-btn flat dense no-caps v-if="fetchingLast" type="button" class="text-[length:var(--ui-font-size)] text-primary px-2"
                    @click="emit('stop')">Stop fetching</q-btn>
                <span class="ml-2 font-mono text-[length:var(--ui-font-size)] text-gray-500">{{ result.PageSize }} rows/page</span>
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
    shallowRef,
    toRaw,
    type CSSProperties,
    watch,
} from "vue";

import { copyToClipboard, Notify, type QMenu, type QTable, type QTableColumn } from "quasar";

import { createGridRowCache, gridValue } from "@/utils/gridRows";
import AdvancedCopy from "./AdvancedCopy.vue";
import { duplicateRowValues, selectionStatistics, type CopySelection } from "@/utils/gridConvenience";
import TableDataEditor from "./TableDataEditor.vue";
import ResultFilterInput from "./ResultFilterInput.vue";
import ResultRecordView from "./ResultRecordView.vue";
import { useResultCellNavigation } from "./useResultCellNavigation";
import { useResultVirtualRows } from "./useResultVirtualRows";
import DataTransfer from "../DataTransfer.vue";
import { useResultSettingsStore } from "@/stores/resultSettingsStore";
const resultSettings = useResultSettingsStore();
import { useTableEditsStore } from "@/stores/tableEditsStore";
import type { QueryResult } from "@/types/queryTab";
import { compileResultFilter } from "@/utils/resultFilter";
import { sortResultRows, orderResultRows, parseResultOrder } from "@/utils/resultSort";
import { encodeGridText, parseGridText, validateGridCells, prepareGridPaste, GRID_CLIPBOARD_TYPE, type ClipboardCell } from "@/utils/resultClipboard";
import { inlineColumnEditable, inlineValueText, stageInlineValue } from "@/utils/inlineTableEdit";
import type { PasteTarget } from "@/utils/resultClipboard";
import { quickFilter } from '@/utils/quickFilter';
import { useConnectionStore } from '@/stores/connectionStore';
import { useColumnLayout } from './useColumnLayout';
import { useQueryTabsStore } from '@/stores/queryTabsStore';
import CellValueViewer from "./CellValueViewer.vue";
import RelatedRecords from './RelatedRecords.vue';

const documentDialog = ref(false);
const selectedDocument = shallowRef<unknown>(null);
const documentType = ref<string>();
const documentColumn = ref("");
function inspectDocument(value: unknown, type?: string, column = "") {
    selectedDocument.value = value;
    documentType.value = type;
    documentColumn.value = column;
    documentDialog.value = true;
}

const tableEdits = useTableEditsStore();
const editState = computed(() => props.tableTabId ? tableEdits.states[props.tableTabId] : undefined);
const editDraftsByKey = computed(() => new Map(editState.value?.drafts.map(draft => [draft.key, draft]) ?? []));
const isMongoResult = computed(() => props.result.Documents !== undefined
    || editState.value?.info?.driver === "mongodb"
    || props.result.Columns.some(column => column.Type === "Extended JSON"));
const navigationBlocked = computed(() => !!editState.value?.drafts.length || !!editState.value?.saving || !!editState.value?.editing);
const selectedRowKey = ref("");
const rowEditor = ref<InstanceType<typeof TableDataEditor> | null>(null);
async function saveChanges() { await rowEditor.value?.save(); }
defineExpose({ saveChanges });

const MIN_COLUMN_WIDTH = 60;
const snColumnWidth = computed(() => Math.max(56, String(props.totalRows ?? (props.result.StartRow + props.result.Rows.length - 1)).length * resultSettings.settings.fontSize * 0.65 + 24));
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
    resultWhere?: string;
    resultOrder?: string;
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
    expressions: [where: string, order: string, sql: string];
}>();

/* =========================================================
   COLUMN WIDTHS
   ========================================================= */

const columnWidths = reactive<Record<string, number>>({});
watch(() => resultSettings.settings.fontSize, (size, previous) => {
    for (const key of Object.keys(columnWidths)) columnWidths[key] = Math.max(MIN_COLUMN_WIDTH, Math.ceil(columnWidths[key] * size / previous));
});
const queryTabs = useQueryTabsStore();
const columnLayout = useColumnLayout(computed(() => props.result.Columns), computed(() => {
    const tab = queryTabs.tabs.find(tab => tab.id === props.tableTabId);
    return `baegle.columns.v1:${JSON.stringify([tab?.connectionId, tab?.tableRef, props.result.Columns.map(c => c.Name)])}`;
}));
const displayedColumns = columnLayout.displayedColumns;
function isPinned(name: string) { return name !== 'sn' && columnLayout.pinned.value.includes(columnLayout.visible.value[Number(name.slice(7))]); }
function fitColumnWidths() {
    displayedColumns.value.forEach((column, i) => {
        const original = columnLayout.visible.value[i];
        const length = Math.max(column.Name.length + (column.Type?.length ?? 0) / 2, ...props.result.Rows.slice(0, 500).map(row => String(row[original] ?? 'NULL').split('\n').reduce((n, line) => Math.max(n, line.length), 0)));
        columnWidths[`column-${i}`] = Math.min(500, Math.max(MIN_COLUMN_WIDTH, length * resultSettings.settings.fontSize * 0.65 + 30));
    });
}
watch(() => JSON.stringify(columnLayout.visible.value), () => { Object.keys(columnWidths).forEach(key => delete columnWidths[key]); void measureHeaderWidths(); });
const gridElement = ref<HTMLElement | null>(null);
const tableWidth = computed(() => snColumnWidth.value + displayedColumns.value.reduce(
    (width, _column, index) => width + getColumnWidth(`column-${index}`), 0,
));

const getColumnWidth = (columnName: string): number => {
    if (columnName === "sn") {
        return snColumnWidth.value;
    }

    return columnWidths[columnName] ?? MIN_COLUMN_WIDTH;
};

// Reuse column geometry across rows instead of rebuilding it for every cell.
const columnStyles = computed(() => {
    const styles: Record<string, CSSProperties> = {};
    let pinnedLeft = snColumnWidth.value;
    for (const name of ['sn', ...displayedColumns.value.map((_column, index) => `column-${index}`)]) {
        const width = getColumnWidth(name);
        const pinned = isPinned(name);
        styles[name] = {
            width: `${width}px`, minWidth: `${width}px`, maxWidth: `${width}px`,
            ...(pinned ? { position: 'sticky' as const, left: `${pinnedLeft}px`, zIndex: 25, background: 'var(--surface-data)' } : {}),
        };
        if (pinned) pinnedLeft += width;
    }
    return styles;
});
const getColumnStyle = (name: string) => columnStyles.value[name];

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
const activeSortColumn = computed(() => effectiveSort.value.column ? `column-${columnLayout.visible.value.indexOf(effectiveSort.value.column - 1)}` : null);
const sortDirection = computed(() => effectiveSort.value.direction ?? null);

const sortColumn = (columnName: string, direction: "asc" | "desc") => {
    if (props.loading || !(props.result.CanSort ?? props.result.CanNavigate) || columnName === "sn") return;
    const column = columnLayout.visible.value[Number(columnName.slice(7))] + 1;
    if (serverExpressions.value) {
        if (!navigationBlocked.value) emit('sort', column, direction);
        return;
    }
    clearOrder();
    if (appliedFilter.value || navigationBlocked.value) {
        pageSort.value = { column, direction };
        return;
    }
    emit("sort", column, direction);
};
const clearSort = () => {
    if (props.loading || !(props.result.CanSort ?? props.result.CanNavigate)) return;
    if (serverExpressions.value) {
        if (!navigationBlocked.value) emit('sort', 0);
        return;
    }
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

const serverExpressions = computed(() => props.result.CanSort ?? props.result.CanNavigate);
const filterDraft = ref(props.resultWhere ?? "");
const quickFilterActions = [
    { operator: '=' as const, label: 'Equals this value' }, { operator: '<>' as const, label: 'Exclude this value' },
    { operator: 'LIKE' as const, label: 'Contains this value (% and _ are wildcards)' },
    { operator: 'IS NULL' as const, label: 'Is NULL' }, { operator: 'IS NOT NULL' as const, label: 'Is not NULL' },
];
function filterByCell(name: string, value: unknown, operator: Parameters<typeof quickFilter>[2]) {
    const column = displayedColumns.value[Number(name.slice(7))];
    if (!column) return;
    const expression = quickFilter(column.Name, value, operator, serverExpressions.value ? useConnectionStore().activeConnection?.driver : undefined);
    filterDraft.value = appliedFilter.value ? `(${appliedFilter.value}) AND (${expression})` : expression;
    applyExpressions();
}
function originalNamedRow(row: Record<string, unknown>) {
    const source = filteredRows.value.find(item => item.key === row.id)?.row;
    return Object.fromEntries(props.result.Columns.map((column, i) => [column.Name, source?.[i]]));
}
const appliedFilter = ref(props.resultWhere ?? "");
const filterError = ref("");
const orderDraft = ref(props.resultOrder ?? "");
const appliedOrder = ref(props.resultOrder ?? "");
watch(() => [props.resultWhere, props.resultOrder], () => {
    if (!serverExpressions.value) return;
    filterDraft.value = appliedFilter.value = props.resultWhere ?? '';
    orderDraft.value = appliedOrder.value = props.resultOrder ?? '';
});
const orderError = ref("");
const orderTerms = computed(() => {
    try { return parseResultOrder(appliedOrder.value, props.result.Columns.map(column => column.Name)); }
    catch { return []; }
});
function clearOrder() {
    if (serverExpressions.value) {
        if (props.loading || navigationBlocked.value) return;
        orderDraft.value = '';
        filterDraft.value = appliedFilter.value;
        applyExpressions();
        return;
    }
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
    if (serverExpressions.value) {
        if (navigationBlocked.value) return;
        try {
            const terms = parseResultOrder(orderDraft.value, props.result.Columns.map(column => column.Name));
            emit('expressions', filterDraft.value.trim(), orderDraft.value.trim(), terms.map(term => `${term.column} ${term.direction.toUpperCase()}`).join(', '));
        } catch (error) {
            orderError.value = error instanceof Error ? error.message : String(error);
        }
        return;
    }
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
    if (serverExpressions.value) {
        if (props.loading || navigationBlocked.value) return;
        filterDraft.value = '';
        orderDraft.value = appliedOrder.value;
        applyExpressions();
        return;
    }
    filterDraft.value = "";
    appliedFilter.value = "";
    pageSort.value = null;
    filterError.value = "";
}
watch(() => props.result, () => {
    if (serverExpressions.value) return;
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
const gridRows = createGridRowCache();
const baseRows = computed(() => props.result.Rows.map((original, index) => ({
    row: original.map(gridValue), index,
    key: props.tableTabId ? tableEdits.rowKey(props.tableTabId, props.result, index) : String(props.result.StartRow + index),
    status: '', number: String(props.result.StartRow + index),
})));
const workingRows = computed(() => gridRows.overlay(baseRows.value, editDraftsByKey.value, props.result.Columns));
// Reuse the keys already prepared for the grid; do not serialize them again on click.
const sourceRowIndexes = computed(() => new Map(workingRows.value
    .filter(row => row.index < props.result.Rows.length)
    .map(row => [row.key, row.index] as const)));
const filteredRows = computed(() => {
    if (serverExpressions.value) return workingRows.value;
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
const mappedRows = computed(() => gridRows.map(filteredRows.value, columnLayout.visible.value));

const dataTable = ref<QTable | null>(null);
const rowHeight = computed(() => Math.max(28, resultSettings.settings.fontSize + 16));
const { visibleRows, firstRow, paddingTop, paddingBottom, scrollToRow } = useResultVirtualRows(dataTable, mappedRows, rowHeight);
const navigationTable = computed(() => dataTable.value ? { $el: dataTable.value.$el as HTMLElement, scrollTo: scrollToRow } : null);
type InlineEdit = { key: string; column: string; name: string; text: string; mode: 'value' | 'null' | 'default'; nullable: boolean; insert: boolean; initialText: string; initialMode: string; error: string };
const inlineEdit = ref<InlineEdit | null>(null);
function inlineTarget(key: string): PasteTarget {
    const tabId = props.tableTabId!;
    const draft = editDraftsByKey.value.get(key);
    const index = sourceRowIndexes.value.get(key) ?? -1;
    if (!draft && index < 0) throw new Error('This row is no longer available.');
    const original = draft?.original ?? Object.fromEntries(props.result.Columns.map((column, i) => [column.Name, props.result.Rows[index]?.[i]]));
    return { key, draft, original, keys: draft?.keys ?? tableEdits.keysFor(tabId, props.result, index) };
}
function inlineMetadata(column: string) {
    if (!column.startsWith('column-')) return undefined;
    const name = displayedColumns.value[Number(column.slice(7))]?.Name;
    return editState.value?.info?.columns?.find(item => item.name === name);
}
function canEditInline(key: string, column: string) {
    const info = editState.value?.info;
    const metadata = inlineMetadata(column);
    const draft = editDraftsByKey.value.get(key);
    return !!props.tableTabId && !!info && !!metadata && !props.loading && !editState.value?.saving
        && !editState.value?.loading && info.driver !== 'mongodb' && draft?.operation !== 'delete'
        && (draft?.operation === 'insert' ? info.canInsert : info.canModify)
        && inlineColumnEditable(metadata, info.keys ?? [], draft?.operation === 'insert');
}
async function focusInline() {
    await nextTick();
    const root = dataTable.value?.$el as HTMLElement | undefined;
    const input = root?.querySelector<HTMLTextAreaElement>('[data-inline-cell-input]');
    if (input && !input.disabled) { input.scrollIntoView({ block: 'nearest', inline: 'nearest' }); input.focus({ preventScroll: true }); input.select(); }
    else root?.querySelector<HTMLSelectElement>('.inline-cell-editor select')?.focus({ preventScroll: true });
}
async function editInlineCell(key: string, column: string, typedText?: string) {
    if (inlineEdit.value?.key === key && inlineEdit.value.column === column) return;
    if (!commitInline()) return;
    if (column === 'sn') { await editInlineRow(key); return; }
    if (editState.value?.info?.driver === 'mongodb') { rowEditor.value?.openRow(key); return; }
    if (!canEditInline(key, column)) return;
    const metadata = inlineMetadata(column)!;
    const target = inlineTarget(key);
    const insert = target.draft?.operation === 'insert';
    const present = !!target.draft && Object.hasOwn(target.draft.values, metadata.name);
    const value = present ? target.draft!.values[metadata.name] : target.original[metadata.name];
    const mode = insert && !present ? 'default' : value === null ? 'null' : 'value';
    selectCell(key, column, false);
    scrollToRow(mappedRows.value.findIndex(row => String(row.id) === key));
    if (editState.value) editState.value.editing = true;
    inlineEdit.value = { key, column, name: metadata.name, text: inlineValueText(value), mode, initialText: inlineValueText(value), initialMode: mode,
        nullable: metadata.nullable && !metadata.primaryKey, insert, error: '' };
    if (typedText !== undefined) { inlineEdit.value.text = typedText; inlineEdit.value.mode = 'value'; }
    await focusInline();
    if (typedText !== undefined) {
        const input = (dataTable.value?.$el as HTMLElement | undefined)?.querySelector<HTMLTextAreaElement>('[data-inline-cell-input]');
        input?.setSelectionRange(typedText.length, typedText.length);
    }
}
function refreshResult() {
    if (props.loading || editState.value?.saving) return;
    emit('refresh');
}
function commitInline(): boolean {
    const editing = inlineEdit.value;
    if (!editing) return true;
    try {
        if (editing.text === editing.initialText && editing.mode === editing.initialMode) { cancelInline(); return true; }
        if (inlineMetadata(editing.column)?.name !== editing.name || !canEditInline(editing.key, editing.column)) throw new Error('The column layout changed; cancel this cell edit and reopen it.');
        const target = inlineTarget(editing.key);
        const draft = stageInlineValue(inlineMetadata(editing.column)!, editState.value?.info?.keys ?? [], target, editing.mode, editing.text);
        tableEdits.stageMany(props.tableTabId!, [draft]);
        cancelInline();
        return true;
    } catch (error) {
        editing.error = error instanceof Error ? error.message : String(error);
        void focusInline();
        return false;
    }
}
function cancelInline() { inlineEdit.value = null; if (editState.value) editState.value.editing = false; }
function inlineBlur(event: FocusEvent) {
    if (editState.value?.confirmingRefresh) return;
    if ((event.currentTarget as HTMLElement).contains(event.relatedTarget as Node | null)) return;
    commitInline();
}
async function editInlineRow(key: string) {
    if (editState.value?.info?.driver === 'mongodb') { rowEditor.value?.openRow(key); return; }
    const active = activeCell.value?.rowKey === key ? `column-${activeCell.value.column}` : '';
    const column = canEditInline(key, active) ? active : displayedColumns.value.map((_item, index) => `column-${index}`).find(name => canEditInline(key, name));
    if (column) await editInlineCell(key, column);
}
async function addInlineRow() {
    if (!commitInline() || !props.tableTabId || !editState.value?.info?.canInsert || props.loading || editState.value.saving) return;
    const key = `new-${crypto.randomUUID()}`;
    tableEdits.stage(props.tableTabId, { key, operation: 'insert', keys: {}, values: {}, original: {} });
    await nextTick();
    selectedRowKey.value = key;
    scrollToRow(mappedRows.value.findIndex(row => String(row.id) === key));
    await editInlineRow(key);
}
function canDuplicateRow(key: string) {
    const state = editState.value;
    return !!props.tableTabId && !!state?.info?.canInsert && !props.loading && !state.loading && !state.saving
        && editDraftsByKey.value.get(key)?.operation !== 'delete'
        && mappedRows.value.some(row => String(row.id) === key);
}
async function duplicateInlineRow(key = selectedRowKey.value) {
    if (!canDuplicateRow(key) || !commitInline()) return;
    try {
        const info = editState.value!.info!;
        const target = inlineTarget(key);
        const original = info.driver === 'mongodb'
            ? JSON.parse(props.result.Documents?.[sourceRowIndexes.value.get(key) ?? -1] ?? '{}') : target.original;
        const source = Object.fromEntries(Object.entries({...original, ...target.draft?.values})
            .map(([name, value]) => [name, value && typeof value === 'object' ? toRaw(value) : value]));
        const values = duplicateRowValues(info.columns ?? [], info.keys ?? [], source);
        // Document fields need not appear in the collection's sampled columns.
        if (info.driver === 'mongodb') {
            for (const [name, value] of Object.entries(source)) if (name !== '_id') values[name] = structuredClone(value);
        }
        const newKey = `new-${crypto.randomUUID()}`;
        tableEdits.stage(props.tableTabId!, {key: newKey, operation: 'insert', keys: {}, values, original: {}});
        await nextTick();
        selectedRowKey.value = newKey;
        scrollToRow(mappedRows.value.findIndex(row => String(row.id) === newKey));
        await editInlineRow(newKey);
    } catch (error) {
        Notify.create({type: 'negative', message: error instanceof Error ? error.message : String(error)});
    }
}
async function inlineKeydown(event: KeyboardEvent) {
    if (event.isComposing || !inlineEdit.value) return;
    if ((event.ctrlKey || event.metaKey) && !event.shiftKey && !event.altKey && event.key.toLowerCase() === 's') {
        event.preventDefault();
        if (!event.repeat) await saveChanges();
        return;
    }
    if (event.ctrlKey || event.metaKey || event.altKey) return;
    if (event.key === 'Enter' && event.target instanceof HTMLSelectElement) return;
    if (event.key === 'Escape') {
        event.preventDefault(); cancelInline();
        (dataTable.value?.$el as HTMLElement | undefined)?.focus({ preventScroll: true });
    } else if (event.key === 'Tab' || (event.key === 'Enter' && !event.shiftKey)) {
        event.preventDefault();
        const editing = inlineEdit.value;
        if (!commitInline()) return;
        if (event.key === 'Tab') {
            const cells = mappedRows.value.flatMap(row => displayedColumns.value.map((_column, index) => ({ key: String(row.id), column: `column-${index}` })))
                .filter(cell => canEditInline(cell.key, cell.column));
            const index = cells.findIndex(cell => cell.key === editing.key && cell.column === editing.column);
            const next = cells[index + (event.shiftKey ? -1 : 1)];
            if (next) { await editInlineCell(next.key, next.column); return; }
        }
        (dataTable.value?.$el as HTMLElement | undefined)?.focus({ preventScroll: true });
    }
}
watch(() => editState.value?.confirmingRefresh, confirming => {
    if (confirming === false && inlineEdit.value) void focusInline();
});
watch(() => editState.value?.editing, editing => {
    if (!editing && inlineEdit.value) cancelInline();
}, { flush: 'sync' });
watch(() => props.result, () => cancelInline());
watch(visibleRows, rows => {
    const editing = inlineEdit.value;
    if (!editing || rows.some(row => String(row.id) === editing.key)) return;
    if (!commitInline()) scrollToRow(mappedRows.value.findIndex(row => String(row.id) === editing.key));
});
const cellContextMenu = ref<QMenu | null>(null);
const contextCell = shallowRef<{ documentType?: string; rowKey: string; status: string; target: HTMLElement; column: string; label: string; value: unknown; namedRow: Record<string, unknown> } | null>(null);
async function openCellMenu(event: MouseEvent, row: Record<string, unknown>, column: QTableColumn) {
    selectContextCell(String(row.id), column.name);
    if (column.name === 'sn' && !props.tableTabId) return;
    event.preventDefault();
    if (column.name === 'sn') selectedRowKey.value = String(row.id);
    cellContextMenu.value?.hide();
    const documentType = props.result.Columns[columnLayout.visible.value[Number(column.name.slice(7))]]?.Type;
    contextCell.value = {
        documentType,
        rowKey: String(row.id), status: String(row._status ?? ''),
        target: event.currentTarget as HTMLElement,
        column: column.name,
        label: column.label,
        value: row[String(column.field)],
        namedRow: originalNamedRow(row),
    };
    await nextTick();
    cellContextMenu.value?.show(event);
}
watch(() => [props.result, displayedColumns.value], () => {
    cellContextMenu.value?.hide();
    contextCell.value = null;
});

const { activeCell, activeCellId, cellId, selectCell, selectContextCell, startCellDrag, handleGridKeydown, onVirtualScroll, revealActiveCell, selectionRange, isCellSelected } = useResultCellNavigation(
    navigationTable, mappedRows, computed(() => displayedColumns.value.map(column => column.Name)), selectedRowKey,
);
watch(visibleRows, async () => { await nextTick(); onVirtualScroll(); });

const advancedCopyOpen = ref(false);
const copySelection = shallowRef<CopySelection>({columns: [], rows: []});
watch(advancedCopyOpen, open => { if (!open) copySelection.value = {columns: [], rows: []}; });
const copyDriver = computed(() => editState.value?.info?.driver ?? useConnectionStore().activeConnection?.driver ?? 'pgx');
const copyTable = computed(() => !['mongodb', 'redis', 'couchdb'].includes(copyDriver.value) ? editState.value?.info?.table : undefined);
const numericColumns = computed(() => displayedColumns.value.map(column => /(?:int|numeric|decimal|real|double|float|number|money)/i.test(column.Type ?? '')));
const selectionStats = computed(() => {
    const range = selectionRange.value;
    function* rows() {
        if (!range) return;
        for (let index = range.fromRow; index <= range.toRow; index++) {
            const row = mappedRows.value[index];
            const values: unknown[] = [];
            for (let column = range.fromColumn; column <= range.toColumn; column++) values.push(row[`column-${column}`]);
            yield values;
        }
    }
    return selectionStatistics(rows(), range ? numericColumns.value.slice(range.fromColumn, range.toColumn + 1) : []);
});
function openAdvancedCopy() {
    if (!commitInline() || !selectionRange.value) return;
    const range = selectionRange.value;
    const columns = displayedColumns.value.slice(range.fromColumn, range.toColumn + 1);
    const rows = mappedRows.value.slice(range.fromRow, range.toRow + 1);
    copySelection.value = {
        columns: columns.map(column => column.Name),
        rows: rows.map(row => columns.map((_column, offset) => row[`column-${range.fromColumn + offset}`])),
        keys: copyTable.value ? rows.map(row => {
            const key = String(row.id);
            const keys = editDraftsByKey.value.get(key)?.keys ?? tableEdits.keysFor(props.tableTabId!, props.result, sourceRowIndexes.value.get(key) ?? -1);
            return structuredClone(toRaw(keys));
        }) : undefined,
    };
    advancedCopyOpen.value = true;
}

async function copyColumnName(name: string) {
    try {
        await copyToClipboard(name);
    } catch (error) {
        Notify.create({ type: 'negative', message: `Could not copy column name: ${String(error)}` });
    }
}

function copyCells(event: ClipboardEvent) {
    if (event.target !== event.currentTarget || !event.clipboardData || !selectionRange.value) return;
    const range = selectionRange.value;
    const cells = mappedRows.value.slice(range.fromRow, range.toRow + 1).map(row =>
        displayedColumns.value.slice(range.fromColumn, range.toColumn + 1).map((_column, offset) => row[`column-${range.fromColumn + offset}`] ?? null));
    event.clipboardData.setData('text/plain', encodeGridText(cells));
    event.clipboardData.setData(GRID_CLIPBOARD_TYPE, JSON.stringify(cells));
    event.preventDefault();
}

function pasteCells(event: ClipboardEvent) {
    if (event.target !== event.currentTarget || !event.clipboardData || !selectionRange.value) return;
    event.preventDefault();
    const info = editState.value?.info;
    if (!props.tableTabId || !info || (!info.canModify && !info.canInsert)) {
        Notify.create({ type: 'warning', message: 'Open an editable table to paste cells.' });
        return;
    }
    if (props.loading || editState.value?.saving) return;
    try {
        if (info.driver === 'mongodb') throw new Error('Use the document editor to paste MongoDB documents.');
        const typed = event.clipboardData.getData(GRID_CLIPBOARD_TYPE);
        if (!typed && !event.clipboardData.types.includes('text/plain')) throw new Error('The clipboard must contain text cells.');
        const parsed: unknown = typed ? JSON.parse(typed) : parseGridText(event.clipboardData.getData('text/plain'));
        validateGridCells(parsed);
        let cells: ClipboardCell[][] = parsed;
        const range = selectionRange.value;
        const selectedHeight = range.toRow - range.fromRow + 1;
        const selectedWidth = range.toColumn - range.fromColumn + 1;
        if (cells.length === 1 && cells[0].length === 1) {
            cells = Array.from({ length: selectedHeight }, () => Array(selectedWidth).fill(cells[0][0]));
        } else if (selectedHeight * selectedWidth > 1 && (selectedHeight !== cells.length || selectedWidth !== cells[0].length)) {
            throw new Error('Select one starting cell, or a range matching the clipboard dimensions.');
        }
        if (range.fromRow + cells.length > mappedRows.value.length || range.fromColumn + cells[0].length > displayedColumns.value.length) {
            throw new Error('The pasted range exceeds the loaded rows or columns. Add rows or load the target page first.');
        }
        const columns = displayedColumns.value.slice(range.fromColumn, range.fromColumn + cells[0].length).map(column => {
            const metadata = info.columns?.find(item => item.name === column.Name);
            if (!metadata) throw new Error(`Column metadata is unavailable for ${column.Name}.`);
            return metadata;
        });
        const tabId = props.tableTabId;
        const targets = mappedRows.value.slice(range.fromRow, range.fromRow + cells.length).map(row => {
            const key = String(row.id);
            const draft = editDraftsByKey.value.get(key);
            if (draft?.operation === 'insert' ? !info.canInsert : !info.canModify) throw new Error('This row is read-only.');
            const index = props.result.Rows.findIndex((_row, i) => tableEdits.rowKey(tabId, props.result, i) === key);
            if (!draft && index < 0) throw new Error('The target row is no longer available.');
            const original = draft?.original ?? Object.fromEntries(props.result.Columns.map((column, i) => [column.Name, props.result.Rows[index]?.[i]]));
            return { key, original, draft, keys: draft?.keys ?? tableEdits.keysFor(tabId, props.result, index) };
        });
        const drafts = prepareGridPaste(columns, info.keys ?? [], targets, cells);
        tableEdits.stageMany(tabId, drafts);
        Notify.create({ type: 'positive', message: 'Pasted changes are staged. Use Save to apply or Cancel to discard.' });
    } catch (error) {
        Notify.create({ type: 'negative', message: error instanceof Error ? error.message : String(error) });
    }
}

const recordViewOpen = ref(false);
const recordRowIndex = computed(() => mappedRows.value.findIndex(row => String(row.id) === activeCell.value?.rowKey));
const recordRow = computed(() => mappedRows.value[recordRowIndex.value]);
function openRecordView() {
    if (!mappedRows.value.length || !props.result.Columns.length) return;
    if (!recordRow.value) selectCell(String(mappedRows.value[0].id), 'column-0', false);
    recordViewOpen.value = true;
}
async function closeRecordView() {
    recordViewOpen.value = false;
    await nextTick();
    (dataTable.value?.$el as HTMLElement | undefined)?.focus({ preventScroll: true });
    void revealActiveCell();
}
function moveRecordRow(offset: number) {
    const row = mappedRows.value[recordRowIndex.value + offset];
    if (row) selectCell(String(row.id), `column-${activeCell.value?.column ?? 0}`, false);
}
function handleResultKeydown(event: KeyboardEvent) {
    if (event.target === event.currentTarget && !event.ctrlKey && !event.metaKey && !event.altKey
        && !event.isComposing && event.key.length === 1 && activeCell.value
        && canEditInline(activeCell.value.rowKey, `column-${activeCell.value.column}`)) {
        event.preventDefault();
        event.stopPropagation();
        void editInlineCell(activeCell.value.rowKey, `column-${activeCell.value.column}`, event.key);
        return;
    }
    if (event.key === 'Enter' && event.target === event.currentTarget && activeCell.value
        && !event.shiftKey && !event.ctrlKey && !event.metaKey && !event.altKey && !event.isComposing) {
        event.preventDefault();
        event.stopPropagation();
        editInlineCell(activeCell.value.rowKey, `column-${activeCell.value.column}`);
        return;
    }
    if (event.key === 'F7' && event.target === event.currentTarget && activeCell.value && recordRow.value
        && !event.shiftKey && !event.ctrlKey && !event.metaKey && !event.altKey && !event.isComposing) {
        event.preventDefault();
        event.stopPropagation();
        const column = displayedColumns.value[activeCell.value.column];
        if (column) inspectDocument(recordRow.value[`column-${activeCell.value.column}`], column.Type, column.Name);
        return;
    }
    if (event.target === event.currentTarget && event.key === 'F2' && !event.ctrlKey && !event.metaKey && !event.altKey && !event.isComposing && props.tableTabId && selectedRowKey.value) {
        event.preventDefault();
        event.stopPropagation();
        if (activeCell.value) editInlineCell(activeCell.value.rowKey, `column-${activeCell.value.column}`);
        else editInlineRow(selectedRowKey.value);
        return;
    }
    if (event.key === 'Tab' && event.target === event.currentTarget && activeCell.value
        && !event.shiftKey && !event.ctrlKey && !event.metaKey && !event.altKey && !event.isComposing) {
        event.preventDefault();
        event.stopPropagation();
        openRecordView();
        return;
    }
    handleGridKeydown(event);
}
watch(recordRow, row => {
    // A filter or page change can remove the selected record.
    if (!row && recordViewOpen.value) recordViewOpen.value = false;
});

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

            ...displayedColumns.value.map(
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
    background: var(--surface-panel);
    border-bottom: 1px solid var(--border-default);
    font-family: monospace;
    font-size: var(--ui-font-size);
}
.result-filter-input {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
    min-width: 0;
    height: 25px;
    padding: 0 8px;
    border: 1px solid var(--border-default);
    background: var(--surface-data);
}
.result-filter-input:focus-within { border-color: var(--accent-color); }
.result-filter-input.has-error { border-color: var(--status-error); }
.filter-expression { flex: 1; min-width: 0; }
.filter-expression :deep(.q-field__control), .filter-expression :deep(.q-field__marginal) { min-height: 23px; height: 23px; }
.filter-expression :deep(.q-field__native) { padding: 0; color: var(--text-primary); }
.filter-expression :deep(input::placeholder) { color: var(--text-muted); }
.filter-action { min-height: 25px; padding: 0; display: flex; align-items: center; justify-content: center; width: 25px; height: 25px; flex-shrink: 0; color: var(--text-muted); border: 1px solid transparent; }
.filter-action:hover:not(.disabled), .filter-action:focus-visible { color: var(--accent-color); background: var(--border-default); border-color: var(--border-default); }
.filter-action.disabled { opacity: .3; cursor: default; }
.filter-count { color: var(--text-muted); white-space: nowrap; padding-left: 6px; }
@media (max-width: 600px) { .filter-scope { display: none; } }

.row-number {
    display: block;
    color: var(--text-muted);
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

.inline-cell-editor { display: flex; align-items: center; gap: 3px; min-width: 0; outline: 1px solid var(--accent-color); background: var(--surface-panel); }
.inline-cell-editor textarea { width: 100%; min-width: 24px; height: calc(var(--grid-font-size) + 4px); padding: 0 4px; resize: none; border: 0; outline: 0; background: transparent; color: var(--text-primary); font: inherit; line-height: calc(var(--grid-font-size) + 4px); }
.inline-cell-editor select { max-width: 76px; min-width: 0; height: calc(var(--grid-font-size) + 4px); background: var(--surface-panel); color: var(--text-muted); border: 0; font: inherit; font-size: 11px; }
.column-resizer::after {
    content: "";

    position: absolute;

    top: 3px;
    bottom: 3px;
    left: 3px;

    width: 1px;

    background: var(--border-default);

    transition:
        background-color 80ms ease,
        width 80ms ease;
}

.column-resizer:hover::after {
    width: 2px;

    background: var(--accent-color);
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
.selected-edit-row :deep(td) { box-shadow: inset 0 1px var(--selection-border), inset 0 -1px var(--selection-border); }
.pending-insert :deep(td) { background: var(--status-success-surface) !important; }
.pending-update :deep(td) { background: var(--accent-surface) !important; }
.pending-delete :deep(td) { background: var(--status-error-surface) !important; text-decoration: line-through; opacity: .65; }

.data-explorer-grid.has-filter-toolbar {
    top: 36px;
}

.data-explorer-grid :deep(.q-table__card),
.data-explorer-grid :deep(.q-table__container) {
    background: transparent !important;

    box-shadow: none !important;
}

.data-explorer-grid :deep(.q-table__middle) {
    background: var(--surface-data);

    min-width: 0;
    min-height: 0;
    max-width: 100%;
    overflow: auto;

    scrollbar-width: thin;

    scrollbar-color:
        var(--border-default)
        var(--surface-data);
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

    user-select: none;
    -webkit-user-select: none;
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
    height: var(--grid-row-height);
    position: sticky;

    top: 0;

    z-index: 40;

    background: var(--surface-panel);
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

    background: var(--surface-panel);

    border-right: 1px solid var(--border-default);
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

    background: var(--surface-data);

    border-right: 1px solid var(--border-default);
}

/* =========================================================
   ALTERNATING ROWS
   ========================================================= */

.striped-rows .data-explorer-grid
    :deep(tbody tr.striped-row:not(:hover):not(.pending-insert):not(.pending-update):not(.pending-delete) td) {
    background: var(--surface-panel);
}

/* =========================================================
   HOVER
   ========================================================= */

.data-explorer-grid
    :deep(tbody tr:hover td:first-child) {
    background: var(--surface-hover);
}

/* =========================================================
   BODY CELLS
   ========================================================= */

.data-explorer-grid :deep(tbody td) {
    height: var(--grid-row-height);
    padding-top: 0;
    padding-bottom: 0;
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
    border-left: 1px solid var(--border-default);
}

.data-explorer-grid
    :deep(thead th + th) {
    border-left-color: var(--border-default);
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
    background: var(--surface-data);
}

.data-explorer-grid
    :deep(.q-table__middle::-webkit-scrollbar-thumb) {
    background: var(--border-default);

    border-radius: var(--radius-tight);
}

.data-explorer-grid
    :deep(.q-table__middle::-webkit-scrollbar-thumb:hover) {
    background: var(--border-default);
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

    border: 1px solid var(--border-default);

    background: var(--surface-data);

    color: var(--text-muted);

    cursor: pointer;

    transition:
        background-color 80ms ease,
        color 80ms ease,
        border-color 80ms ease;
}

.pagination-button:hover:not(
        .disabled
    ) {
    background: var(--border-default);

    border-color: var(--border-default);

    color: var(--accent-color);
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
    border-color: var(--accent-color);
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

.data-explorer-grid:focus { outline: none; }
.result-record-view { position: absolute; inset: 0 0 36px; }
.result-record-view.has-filter-toolbar { top: 36px; }
.result-record-view.has-edit-toolbar { top: 70px; }
.data-explorer-grid :deep(td.active-result-cell) {
    background: var(--selection-bg) !important;
    color: #fff;
    box-shadow: inset 0 0 0 2px var(--selection-border) !important;
}
.data-explorer-grid :deep(td.selected-result-cell) {
    background: var(--selection-bg) !important;
    color: #fff;
}
.data-explorer-grid :deep(thead th[data-pinned]) { z-index: 55 !important; background: var(--surface-panel) !important; }
</style>

<style scoped>
.selection-stats { max-width: 220px; min-width: 0; color: var(--text-muted); font-family: monospace; font-size: var(--ui-font-size); }
.selection-stats :deep(.q-btn__content) { flex-wrap: nowrap; }
.selection-stats :deep(.block) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
:global(.selection-statistics-menu) { max-width: min(420px, calc(100vw - 24px)); background: var(--surface-panel); color: var(--text-primary); border: 1px solid var(--border-default); border-radius: var(--radius-tight); font-family: monospace; }
.statistics-heading { padding: 10px 12px; color: var(--accent-color); border-bottom: 1px solid var(--border-default); font-weight: 600; }
.statistics-values { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 8px 20px; margin: 12px; }
.statistics-values dt { color: var(--text-muted); }
.statistics-values dd { margin: 0; text-align: right; overflow-wrap: anywhere; }
.statistics-note { margin: 10px 12px; max-width: 320px; color: var(--text-muted); font-size: var(--ui-font-size); }
</style>
