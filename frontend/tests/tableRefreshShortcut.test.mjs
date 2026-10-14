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
    const handle = new Function('queryTabsStore', 'tableEdits', 'document', 'executeQuery', `${compiled}; return handleRefreshTableShortcut;`)(
        { activeTab: tab }, { hasPending: () => pending, isSaving: () => saving },
        { querySelector: () => modal ? {} : null }, (...args) => calls.push(args));
    handle(key);
    return { calls, prevented: key.prevented };
}

test('Ctrl+R refreshes the active result query and suppresses page reload', () => {
    assert.deepEqual(press(), { calls: [['table', 'SELECT * FROM items']], prevented: true });
    assert.deepEqual(press({ tab: { id: 'collection', type: 'result', sql: '{"find":"items"}' } }).calls,
        [['collection', '{"find":"items"}']]);
});

test('blocked refreshes preserve unsaved edits and suppress page reload', () => {
    for (const options of [{ pending: true }, { saving: true }, { modal: true }, { event: { repeat: true } },
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
