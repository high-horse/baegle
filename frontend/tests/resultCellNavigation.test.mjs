import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import { computed, effectScope, nextTick, ref } from 'vue';
import ts from 'typescript';

const source = readFileSync(new URL('../src/components/workspace/resultGrid/useResultCellNavigation.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } }).outputText;
const module = { exports: {} };
new Function('module', 'exports', 'require', 'window', compiled)(module, module.exports,
    createRequire(import.meta.url), { getSelection: () => null });

function setup(t) {
    const scope = effectScope();
    t.after(() => scope.stop());
    const rows = ref([{ id: 'a' }, { id: 'b' }, { id: 'c' }]);
    const columns = ref(['id', 'name', 'value']);
    const selectedRow = ref('');
    const scrolls = [];
    const root = { focus() {}, querySelector: () => null };
    const table = ref({ $el: root, scrollTo: row => scrolls.push(row) });
    const nav = scope.run(() => module.exports.useResultCellNavigation(table, computed(() => rows.value), computed(() => columns.value), selectedRow));
    const press = (key, modifiers = {}) => {
        let prevented = false;
        nav.handleGridKeydown({ key, target: root, currentTarget: root, preventDefault() { prevented = true; }, stopPropagation() {}, ...modifiers });
        return prevented;
    };
    return { ...nav, rows, columns, selectedRow, press, scrolls };
}

test('clicking selects a cell and arrow keys move through rows and columns without wrapping', t => {
    const grid = setup(t);
    grid.selectCell('b', 'column-1');
    grid.press('ArrowRight');
    assert.deepEqual(grid.activeCell.value, { rowKey: 'b', column: 2 });
    grid.press('ArrowRight');
    assert.equal(grid.activeCell.value.column, 2);
    grid.press('ArrowDown');
    assert.equal(grid.selectedRow.value, 'c');
    grid.press('ArrowDown');
    assert.equal(grid.selectedRow.value, 'c');
    grid.press('ArrowLeft');
    grid.press('ArrowUp');
    assert.deepEqual(grid.activeCell.value, { rowKey: 'b', column: 1 });
});

test('navigation follows displayed order while selection stays with a row after sorting', t => {
    const grid = setup(t);
    grid.selectCell('b', 'column-1');
    grid.rows.value = [{ id: 'c' }, { id: 'b' }, { id: 'a' }];
    assert.equal(grid.activeCell.value.rowKey, 'b');
    grid.press('ArrowDown');
    assert.equal(grid.activeCell.value.rowKey, 'a');
});

test('filtering out the active row or replacing columns clears stale cell selection', t => {
    const grid = setup(t);
    grid.selectCell('b', 'column-1');
    grid.rows.value = [{ id: 'a' }];
    assert.equal(grid.activeCell.value, null);
    grid.press('ArrowDown');
    assert.deepEqual(grid.activeCell.value, { rowKey: 'a', column: 0 });
    grid.columns.value = ['new'];
    assert.equal(grid.activeCell.value, null);
    grid.rows.value = [];
    assert.equal(grid.press('ArrowDown'), false);
});

test('fields, modified shortcuts and composition retain their own keyboard behavior', t => {
    const grid = setup(t);
    grid.selectCell('b', 'column-1');
    for (const modifiers of [{ ctrlKey: true }, { altKey: true }, { metaKey: true }, { isComposing: true }, { target: {} }]) {
        assert.equal(grid.press('ArrowDown', modifiers), false);
    }
    assert.equal(grid.press('Enter'), false);
    assert.deepEqual(grid.activeCell.value, { rowKey: 'b', column: 1 });
});

test('Shift+arrows and Shift+click extend a rectangular selection from its original anchor', t => {
    const grid = setup(t);
    grid.selectCell('b', 'column-1');
    grid.press('ArrowRight', { shiftKey: true });
    grid.press('ArrowUp', { shiftKey: true });
    assert.deepEqual(grid.selectionRange.value, { fromRow: 0, toRow: 1, fromColumn: 1, toColumn: 2 });
    assert.equal(grid.isCellSelected(0, 'column-2'), true);
    assert.equal(grid.isCellSelected(0, 'column-0'), false);
    grid.selectCell('c', 'column-0', true, true);
    assert.deepEqual(grid.selectionRange.value, { fromRow: 1, toRow: 2, fromColumn: 0, toColumn: 1 });
    grid.press('ArrowRight');
    assert.deepEqual(grid.selectionRange.value, { fromRow: 2, toRow: 2, fromColumn: 1, toColumn: 1 });
});

test('Ctrl+A selects only the loaded grid and a re-sort collapses ranges to the active record', t => {
    const grid = setup(t);
    assert.equal(grid.press('a', { ctrlKey: true }), true);
    assert.deepEqual(grid.selectionRange.value, { fromRow: 0, toRow: 2, fromColumn: 0, toColumn: 2 });
    grid.rows.value = [{ id: 'c' }, { id: 'a' }, { id: 'b' }];
    assert.equal(grid.activeCell.value.rowKey, 'c');
    assert.deepEqual(grid.selectionRange.value, { fromRow: 0, toRow: 0, fromColumn: 2, toColumn: 2 });
});

test('moving to an unrendered row asks the virtual table to reveal its full-list index', async t => {
    const grid = setup(t);
    grid.rows.value = Array.from({ length: 500 }, (_, index) => ({ id: String(index) }));
    grid.selectCell('150', 'column-1');
    grid.press('ArrowDown');
    await nextTick();
    assert.equal(grid.scrolls.at(-1), 151);
    assert.deepEqual(grid.activeCell.value, { rowKey: '151', column: 1 });
});
