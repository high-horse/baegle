<template>
    <q-btn flat dense round icon="help_outline" size="sm" title="Workspace help" aria-label="Workspace help and instructions">
        <q-menu anchor="bottom right" self="top right" :offset="[0, 10]" max-height="75vh" class="workspace-help-menu">
            <div class="workspace-help-content">
            <header>
                <div class="help-icon"><q-icon name="help_outline" size="22px" /></div>
                <div class="help-heading"><h2>Workspace help</h2><p>Shortcuts and everyday actions</p></div>
                <q-btn flat round dense icon="close" size="sm" aria-label="Close workspace help" v-close-popup />
            </header>
            <div class="help-sections">
            <section>
                <h3><q-icon name="edit_note" size="18px" /> Edit table data</h3>
                <p>Double-click a row to edit it. You can also select a row and press <kbd>F2</kbd> or choose <strong>Edit row</strong>.</p>
                <p>Use <strong>Add row</strong> for a new record. Choose <strong>Stage changes</strong> in the editor, then <strong>Save</strong> above the grid to write to the database. <strong>Cancel</strong> above the grid discards staged changes.</p>
                <p>Existing IDs and primary keys are read-only. Generated fields are always read-only. Existing rows need a primary key and write permission to be edited.</p>
            </section>
            <section>
                <h3><q-icon name="open_with" size="18px" /> Navigate &amp; select</h3>
                <p>Use the arrow keys to move between cells. Hold <kbd>Shift</kbd> with arrows or a click to extend a selection, or drag across cells.</p>
                <p><kbd>Ctrl</kbd> / <kbd>⌘</kbd> + <kbd>C</kbd> copies cells; <kbd>V</kbd> with the same modifier pastes into an editable table. <kbd>Ctrl</kbd> / <kbd>⌘</kbd> + <kbd>A</kbd> selects the current page.</p>
                <p>Press <kbd>Tab</kbd> to switch between the grid and row details. Right-click a JSON or MongoDB cell and choose <strong>Inspect JSON</strong> to view its document.</p>
            </section>
            <section>
                <h3><q-icon name="filter_alt" size="18px" /> Filter &amp; export</h3>
                <p>Right-click a table cell or row number for Edit row and Delete row. Deletions are staged until you choose Save. Cell menus also offer quick filters and related records. Apply <strong>WHERE</strong> and <strong>ORDER BY</strong> with <kbd>Enter</kbd>; <kbd>Esc</kbd> resets the focused expression.</p>
                <p>In Export, choose the source scope and the columns to include. A current-page export includes the fetched values before local filters and staged edits.</p>
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
    border-radius: 14px;
    box-shadow: 0 16px 48px #0005;
    overflow-x: hidden;
    overflow-y: auto;
    scrollbar-width: thin;
}
.workspace-help-content { min-height: 0; }
header { position: sticky; top: 0; z-index: 1; display: flex; align-items: center; gap: 12px; padding: 22px 24px; background: var(--surface-panel); border-bottom: 1px solid var(--border-default); }
.help-icon { display: grid; place-items: center; width: 40px; height: 40px; flex-shrink: 0; border-radius: 11px; background: var(--accent-tint); color: var(--accent-color); }
.help-heading { flex: 1; min-width: 0; }
h2 { margin: 0; font-size: 16px; line-height: 1.4; font-weight: 600; letter-spacing: -.2px; }
.help-heading p { margin: 3px 0 0; font-size: 11px; }
.help-sections { display: grid; gap: 16px; padding: 20px 24px 24px; }
section { padding: 16px; background: var(--surface-data); border: 1px solid var(--border-default); border-radius: 9px; }
h3 { display: flex; align-items: center; gap: 8px; margin: 0 0 12px; font-size: 12px; font-weight: 600; }
h3 .q-icon { color: var(--accent-color); }
p { margin: 0 0 12px; color: var(--text-muted); font-size: 12px; line-height: 1.75; }
p:last-child { margin-bottom: 0; }
strong { color: var(--text-primary); font-weight: 500; }
kbd { display: inline-block; min-width: 20px; padding: 1px 6px; border: 1px solid var(--border-default); border-bottom-width: 2px; border-radius: 5px; background: var(--surface-panel); color: var(--text-primary); text-align: center; font: inherit; font-size: 11px; line-height: 1.5; }
@media (max-width: 480px) {
    header { padding: 18px 16px; }
    .help-sections { padding: 16px; gap: 12px; }
    section { padding: 14px; }
}
</style>
