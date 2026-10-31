<template>
    <q-btn flat dense round icon="help_outline" size="sm" title="Workspace help" aria-label="Workspace help and instructions">
        <q-menu anchor="bottom right" self="top right" class="workspace-help">
            <header><q-icon name="help_outline" size="20px" /><h2>Workspace help</h2></header>
            <section>
                <h3>Edit table data</h3>
                <p>Select a row, then press <kbd>F2</kbd> or choose <strong>Edit row</strong>. You can also click a non-JSON cell four times.</p>
                <p>Use <strong>Add row</strong> for a new record. Choose <strong>Stage changes</strong> in the editor, then <strong>Save</strong> above the grid to write to the database. <strong>Cancel</strong> above the grid discards staged changes.</p>
                <p>Existing IDs and primary keys are read-only. Generated fields are always read-only. Existing rows need a primary key and write permission to be edited.</p>
            </section>
            <section>
                <h3>Navigate & select</h3>
                <p>Use the arrow keys to move between cells. Hold <kbd>Shift</kbd> with arrows or a click to extend a selection, or drag across cells.</p>
                <p><kbd>Ctrl</kbd> / <kbd>⌘</kbd> + <kbd>C</kbd> copies cells; <kbd>V</kbd> with the same modifier pastes into an editable table. <kbd>Ctrl</kbd> / <kbd>⌘</kbd> + <kbd>A</kbd> selects the current page.</p>
                <p>Press <kbd>Tab</kbd> to switch between the grid and row details. Triple-click a JSON or MongoDB cell to inspect its document.</p>
            </section>
            <section>
                <h3>Filter & export</h3>
                <p>Right-click a cell for quick filters and related records. Apply <strong>WHERE</strong> and <strong>ORDER BY</strong> with <kbd>Enter</kbd>; <kbd>Esc</kbd> resets the focused expression.</p>
                <p>In Export, choose the source scope and the columns to include. A current-page export includes the fetched values before local filters and staged edits.</p>
            </section>
        </q-menu>
    </q-btn>
</template>

<style scoped>
.workspace-help { width: 410px; max-width: calc(100vw - 32px); max-height: 75vh; overflow-y: auto; padding: 20px; background: var(--surface-panel); color: var(--text-primary); border: 1px solid var(--border-default); border-radius: 10px; }
header { display: flex; align-items: center; gap: 10px; color: var(--accent-color); }
h2 { margin: 0; font-size: 16px; font-weight: 600; }
h3 { margin: 0 0 10px; font-size: 12px; font-weight: 600; }
section { padding-top: 18px; }
section + section { margin-top: 18px; border-top: 1px solid var(--border-default); }
p { margin: 0 0 10px; color: var(--text-muted); font-size: 12px; line-height: 1.65; }
p:last-child { margin-bottom: 0; }
strong { color: var(--text-primary); font-weight: 500; }
kbd { display: inline-block; padding: 0 5px; border: 1px solid var(--border-default); border-radius: 4px; background: var(--surface-data); color: var(--text-primary); font: inherit; font-size: 11px; }
</style>
