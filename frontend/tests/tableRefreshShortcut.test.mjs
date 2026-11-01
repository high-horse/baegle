import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(new URL('../src/layouts/WorkspaceLayout.vue', import.meta.url), 'utf8').split('<script setup lang="ts">')[1].split('</script>')[0];
const ast = ts.createSourceFile('workspace.ts', source, ts.ScriptTarget.Latest, true);
const handler = ast.statements.find(node => ts.isFunctionDeclaration(node) && node.name?.text === 'handleRefreshTableShortcut');
const compiled = ts.transpileModule(handler.getText(ast), { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;

function press({ tab = { id: 'table', type: 'result', executedSql: 'SELECT * FROM items', sql: 'other' }, pending = false, saving = false, modal = false, event = {} } = {}) {
    const calls = [];
    const key = { key: 'r', ctrlKey: true, prevented: false, preventDefault() { this.prevented = true; }, stopPropagation() {}, ...event };
    const handle = new Function('queryTabsStore', 'tableEdits', 'document', 'refreshTable', `${compiled}; return handleRefreshTableShortcut;`)(
        { activeTab: tab, refresh: id => calls.push([id]) }, { hasPending: () => pending, isSaving: () => saving },
        { querySelector: () => modal ? {} : null }, (_queries, _edits, id) => calls.push([id]));
    handle(key);
    return { calls, prevented: key.prevented };
}

test('Ctrl+R refreshes the active result query and suppresses page reload', () => {
    assert.deepEqual(press(), { calls: [['table']], prevented: true });
    assert.deepEqual(press({ tab: { id: 'collection', type: 'result', sql: '{"find":"items"}' } }).calls,
        [['collection']]);
});

test('blocked refreshes preserve in-flight saves and suppress page reload', () => {
    for (const options of [{ saving: true }, { modal: true }, { event: { repeat: true } },
        { tab: { id: 'table', type: 'result', loading: true } }, { tab: { id: 'table', type: 'result', closing: true } }]) {
        assert.deepEqual(press(options), { calls: [], prevented: true });
    }
});

test('does not refresh query consoles or respond to unrelated shortcuts', () => {
    for (const options of [{ tab: null }, { tab: { type: 'query' } }, { event: { ctrlKey: false } },
        { event: { shiftKey: true } }, { event: { altKey: true } }, { event: { metaKey: true } },
        { event: { isComposing: true } }, { event: { key: 't' } }]) {
        assert.deepEqual(press(options), { calls: [], prevented: false });
    }
});

test('pending edits do not block Ctrl+R', () => {
    assert.deepEqual(press({pending: true}), {calls: [['table']], prevented: true});
});

const refreshSource = readFileSync(new URL('../src/utils/refreshTable.ts', import.meta.url), 'utf8');
const refreshCompiled = ts.transpileModule(refreshSource, {compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS}}).outputText;
const refreshModule = {exports: {}};
new Function('exports', refreshCompiled)(refreshModule.exports);
function refreshTable(overrides = {}, saving = false) {
    const calls = [];
    const drafts = [{operation: 'insert'}, {operation: 'update'}, {operation: 'delete'}];
    const state = {drafts, editing: true};
    const queries = {tabs: [{id: 'table', executedSql: 'SELECT * FROM items', ...overrides}],
        async execute(id, sql) {calls.push(['execute', id, sql]); assert.deepEqual(state.drafts, []); assert.equal(state.editing, false);} };
    const edits = {isSaving: () => saving, discard(id) {calls.push(['discard', id]); state.drafts = []; state.editing = false;}};
    const refresh = id => refreshModule.exports.refreshTable(queries, edits, id);
    return {refresh, calls, state};
}
test('refresh discards inserts, updates, deletes and the active editor before fetching', async () => {
    const {refresh, calls} = refreshTable();
    await refresh('table');
    assert.deepEqual(calls, [['discard', 'table'], ['execute', 'table', 'SELECT * FROM items']]);
});
test('refresh cannot discard during saves, loading, or closing', async () => {
    for (const [tab, saving] of [[{}, true], [{loading: true}, false], [{closing: true}, false]]) {
        const {refresh, calls, state} = refreshTable(tab, saving);
        await refresh('table');
        assert.deepEqual(calls, []);
        assert.equal(state.drafts.length, 3);
        assert.equal(state.editing, true);
    }
});
