<template>
    <q-btn flat dense class="app-icon-button" icon="help_outline" size="sm" title="Workspace help" aria-label="Workspace help and instructions">
        <q-menu anchor="bottom right" self="top right" :offset="[0, 10]" max-height="75vh" class="workspace-help-menu">
            <div class="workspace-help-content">
            <header>
                <div class="help-icon"><q-icon name="help_outline" size="22px" /></div>
                <div class="help-heading"><h2>Workspace help</h2><p>Shortcuts and everyday actions</p></div>
                <q-btn flat dense class="app-close-button" icon="close" aria-label="Close workspace help" v-close-popup />
            </header>
            <div class="help-sections">
            <section>
                <h3><q-icon name="terminal" size="18px" /> Query console</h3>
                <p>Write SQL and choose <strong>Run</strong> or press <kbd>Ctrl</kbd> / <kbd>⌘</kbd> + <kbd>Enter</kbd>. Selected text runs first; without a selection, the statement at the cursor runs. Select multiple statements to run them as a script.</p>
                <p><strong>Run script</strong> or <kbd>Alt</kbd> + <kbd>X</kbd> runs the whole SQL console. Use <strong>Stop</strong> to cancel execution. Scripts stop at the first error; earlier commits remain. Use <strong>Go to line</strong> to jump to a reported script error.</p>
                <p><kbd>Ctrl</kbd> + <kbd>Alt</kbd> + <kbd>F</kbd> formats SQL. <kbd>Ctrl</kbd> / <kbd>⌘</kbd> + <kbd>F</kbd> opens search. Choose an autocomplete suggestion with <kbd>Tab</kbd>. <strong>History</strong> lets you search recent executions and open SQL in a new console without running it.</p>
                <p>Hold <kbd>Ctrl</kbd> / <kbd>⌘</kbd> and click a recognized SQL table name to open it.</p>
                <p>Console text saves automatically; <strong>Save</strong> or <kbd>Ctrl</kbd> / <kbd>⌘</kbd> + <kbd>S</kbd> saves it explicitly. Saving console text does not execute SQL or save table edits in another tab.</p>
            </section>
            <section>
                <h3><q-icon name="tab" size="18px" /> Tabs &amp; session</h3>
                <p><kbd>Ctrl</kbd> + <kbd>T</kbd> opens a console; <kbd>Ctrl</kbd> + <kbd>W</kbd> closes the active tab. Switch tabs with <kbd>Ctrl</kbd> + <kbd>Tab</kbd>, or go backward with <kbd>Ctrl</kbd> + <kbd>Shift</kbd> + <kbd>Tab</kbd>. <kbd>Ctrl</kbd> + <kbd>J</kbd> shows or hides console results, including script results. The result header’s close button hides the pane without deleting its results; run another query or script to show output again.</p>
                <p><kbd>Ctrl</kbd> + <kbd>P</kbd> searches tables, views, and collections. Use the arrow keys and <kbd>Enter</kbd> to open a match. Opening an already open table selects its existing tab.</p>
                <p>Drag tabs to reorder them. Right-click a tab to close it, other tabs, tabs to either side, or all tabs; unsaved table edits still require a discard decision.</p>
                <p>Each console remembers its scroll position and cursor selection while you switch tabs during this connection session. These positions reset after disconnecting; saved console text follows the normal restore behavior.</p>
            </section>
            <section>
                <h3><q-icon name="edit_note" size="18px" /> Edit, add &amp; delete rows</h3>
                <p>Open a table from the sidebar. Select a cell and type, double-click it, press <kbd>Enter</kbd> or <kbd>F2</kbd>, or choose <strong>Edit cell</strong>. <kbd>Tab</kbd> stages the value and moves forward; <kbd>Shift</kbd> + <kbd>Tab</kbd> moves backward. <kbd>Esc</kbd> cancels the active cell edit.</p>
                <p><strong>Add row</strong> creates a draft in the grid. Click a new-row cell or type to replace <strong>DEFAULT</strong>. The cell selector distinguishes <strong>Value</strong>, <strong>NULL</strong>, and <strong>Default</strong>: an empty Value is an empty string; untouched Default fields are supplied by the database.</p>
                <p><strong>Delete</strong> stages a row deletion; <strong>Undo delete</strong> restores it before saving. <strong>Save</strong> or <kbd>Ctrl</kbd> / <kbd>⌘</kbd> + <kbd>S</kbd> writes pending table changes, including the active cell. <strong>Cancel</strong> discards pending changes.</p>
                <p>Existing IDs and primary keys are read-only. Generated fields are database-managed. Existing SQL rows need a primary key and write permission to be edited. MongoDB uses the document editor for nested fields.</p>
            </section>
            <section>
                <h3><q-icon name="content_copy" size="18px" /> Duplicate a row</h3>
                <ol>
                    <li>Select the source row, then choose <strong>Duplicate</strong> in the edit toolbar or <strong>Duplicate row</strong> from its right-click menu.</li>
                    <li>Edit the new draft. Copied values include staged edits; primary keys and generated fields are omitted. Enter any required manual keys, or leave database-generated keys at Default.</li>
                    <li>Choose <strong>Save</strong> to insert it, or <strong>Cancel</strong> to discard pending changes. The source row stays unchanged unless you separately edit it.</li>
                </ol>
            </section>
            <section>
                <h3><q-icon name="refresh" size="18px" /> Refresh &amp; discard changes</h3>
                <p>Use the grid footer’s <strong>Refresh</strong> button, or <kbd>Ctrl</kbd> + <kbd>R</kbd> in a table result tab, to fetch saved data again. With unsaved changes, choose <strong>Keep editing</strong> to preserve them or <strong>Discard and refresh</strong> to remove pending inserts, updates, deletions, and active cell input before reloading.</p>
                <p>Refresh is blocked while saving. The same table’s selected sort is retained. New SQL table tabs use primary-key order when available; editing a sort value can move its row. Tables without a primary key have no guaranteed default order.</p>
            </section>
            <section>
                <h3><q-icon name="open_with" size="18px" /> Select, copy &amp; paste cells</h3>
                <p>Use arrows to move between cells. Hold <kbd>Shift</kbd> with arrows or a click to extend a rectangular selection, or drag across cells. <kbd>Ctrl</kbd> / <kbd>⌘</kbd> + <kbd>A</kbd> selects the loaded grid rows.</p>
                <p><kbd>Ctrl</kbd> / <kbd>⌘</kbd> + <kbd>C</kbd> copies the selection. Paste with the same modifier and <kbd>V</kbd> into an editable table. Select one starting cell for a block, or a matching rectangle; a single copied value fills the selected rectangle. Pasting stages changes until Save.</p>
                <p>With the grid focused, <kbd>Tab</kbd> switches to row details. Press <kbd>F7</kbd> on a cell or choose <strong>View cell value</strong> in its menu to inspect long text, JSON, XML, or binary data. The viewer offers display formats, wrapping, and Copy.</p>
            </section>
            <section>
                <h3><q-icon name="content_paste" size="18px" /> Advanced copy</h3>
                <ol>
                    <li>Select the cells to copy. Choose <strong>Advanced copy</strong> from the footer’s copy button or a cell’s right-click menu.</li>
                    <li>Choose tab-separated text, CSV, a custom delimiter, or JSON. Delimited formats let you include column headers and choose how NULL appears.</li>
                    <li>Review the preview and choose <strong>Copy</strong>. Large previews are shortened to keep the dialog responsive; Copy still includes all selected rows in their displayed order, including staged values.</li>
                </ol>
                <p>Table tabs also offer <strong>SQL INSERT</strong> and <strong>SQL UPDATE</strong>. UPDATE needs original primary keys for every selected row, even when those columns are hidden or unselected. SQL is copied for review; it is not executed. New unsaved rows have no original keys for UPDATE.</p>
            </section>
            <section>
                <h3><q-icon name="functions" size="18px" /> Selection statistics</h3>
                <p>Select cells, then click the <strong>Σ</strong> statistics button in the grid footer. It shows selected-cell count, non-NULL count, numeric count, sum, average, minimum, and maximum.</p>
                <p>Calculations cover the selected loaded rows, including staged values. Numeric calculations use numeric columns and exclude NULLs; numeric-looking text columns are not included. <strong>≈</strong> marks an approximate average. Unsupported numeric values are counted separately.</p>
            </section>
            <section>
                <h3><q-icon name="filter_alt" size="18px" /> Filter, sort &amp; related records</h3>
                <p>Enter expressions in <strong>WHERE</strong> and <strong>ORDER BY</strong>, then press <kbd>Enter</kbd> or the apply button. For example, <code>status = 'active'</code> and <code>created_at DESC</code>. <kbd>Esc</kbd> resets the focused expression; the syntax-help button explains supported expressions and scope.</p>
                <p>Click a column header to sort. Right-click a cell for quick filters such as equal, not equal, or NULL. The menu identifies whether filtering applies to the database result or the current page.</p>
                <p><strong>Related records</strong> in a cell’s menu lists declared foreign-key relationships. Choose a relationship to open its SELECT in a new console. Save or discard pending changes first; NULL foreign keys cannot identify a related record.</p>
            </section>
            <section>
                <h3><q-icon name="view_column" size="18px" /> Columns, schema &amp; paging</h3>
                <p>Open <strong>Columns</strong> above the grid to find, hide, pin left, or reorder columns. Drag a header’s resize handle to change its width. <strong>Fit widths</strong> sizes columns to fetched content; <strong>Reset layout</strong> restores the layout. Column choices are remembered for the table.</p>
                <p>Use the result’s <strong>Schema</strong> tab to inspect its structure. Footer arrows move between pages; <strong>Count rows</strong> calculates the total for supported results. Going to the last page may count or fetch remaining rows; use the available Cancel or Stop control to interrupt it.</p>
            </section>
            <section>
                <h3><q-icon name="import_export" size="18px" /> Import, export &amp; settings</h3>
                <p>Use <strong>Export</strong> in a result footer to choose source scope, columns, and format. A current-page export uses fetched values before local filters and staged edits. Use Advanced copy for selected displayed cells.</p>
                <p>The workspace header’s import button opens data import. Review its target and format before starting. SQL files load into a console for review before execution. After importing rows, refresh the table to see them.</p>
                <p>The header’s settings button or <kbd>Ctrl</kbd> + <kbd>.</kbd> opens workspace settings: accent color, font sizes, rows per page, query timeout, import size limit, and result display. Save changes to apply them; page size, timeout, and import limits apply to subsequent operations.</p>
                <p>Use the header’s disconnect button to leave the connection, or press <kbd>Esc</kbd> twice quickly when no dialog, menu, or cell editor is open. Unsaved table changes require a discard decision before disconnecting.</p>
            </section>
            </div>
            </div>
        </q-menu>
    </q-btn>
</template>

<style scoped>
/* QMenu teleports its root; keep layout padding on our own content element. */
:global(.workspace-help-menu) {
    width: 460px;
    max-width: calc(100vw - 24px);
    background: var(--surface-panel);
    color: var(--text-primary);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-panel);
    box-shadow: var(--shadow-overlay);
    overflow-x: hidden;
    overflow-y: auto;
    scrollbar-width: thin;
}
.workspace-help-content { min-height: 0; }
header { position: sticky; top: 0; z-index: 1; display: flex; align-items: center; gap: 12px; padding: 16px 20px; background: var(--surface-panel); border-bottom: 1px solid var(--border-default); }
.help-icon { display: grid; place-items: center; width: 32px; height: 32px; flex-shrink: 0; border-radius: var(--radius-control); background: var(--accent-tint); color: var(--accent-color); }
.help-heading { flex: 1; min-width: 0; }
h2 { margin: 0; font-size: calc(var(--ui-font-size) + 3px); line-height: 1.4; font-weight: 600; letter-spacing: -.2px; }
.help-heading p { margin: 3px 0 0; font-size: 11px; }
.help-sections { display: grid; gap: 16px; padding: 20px; }
section { padding: 16px; background: var(--surface-data); border: 1px solid var(--border-default); border-radius: var(--radius-panel); }
h3 { display: flex; align-items: center; gap: 8px; margin: 0 0 12px; font-size: max(12px, var(--ui-font-size)); font-weight: 600; }
h3 .q-icon { color: var(--accent-color); }
p { margin: 0 0 12px; color: var(--text-muted); font-size: max(12px, var(--ui-font-size)); line-height: 1.75; }
p:last-child { margin-bottom: 0; }
ol { margin: 0; padding-left: 20px; color: var(--text-muted); font-size: max(12px, var(--ui-font-size)); line-height: 1.75; }
li + li { margin-top: 8px; }
ol + p { margin-top: 12px; }
code { padding: 1px 4px; border-radius: 3px; background: var(--surface-panel); color: var(--text-primary); font-size: 11px; overflow-wrap: anywhere; }
strong { color: var(--text-primary); font-weight: 500; }
kbd { display: inline-block; min-width: 20px; padding: 1px 6px; border: 1px solid var(--border-default); border-bottom-width: 2px; border-radius: 5px; background: var(--surface-panel); color: var(--text-primary); text-align: center; font: inherit; font-size: 11px; line-height: 1.5; }
@media (max-width: 480px) {
    header { padding: 18px 16px; }
    .help-sections { padding: 16px; gap: 12px; }
    section { padding: 14px; }
}
</style>
