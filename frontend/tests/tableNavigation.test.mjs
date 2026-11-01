import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(new URL('../src/layouts/WorkspaceLayout.vue', import.meta.url), 'utf8').split('<script setup lang="ts">')[1].split('</script>')[0];
const ast = ts.createSourceFile('workspace.ts', source, ts.ScriptTarget.Latest, true);
const handler = ast.statements.find(node => ts.isFunctionDeclaration(node) && node.name?.text === 'handleTableSelect');
const compiled = ts.transpileModule(handler.getText(ast), { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;
const ref = { name: 'users', schema: 'public', database: 'app' };
const original = { id: 'existing', type: 'result', connectionId: '1', tableRef: ref };
function setup(tabs) {
    const calls = [];
    const store = {
        tabs,
        selectTab: id => calls.push(['select', id]),
        createResultTab(title, connectionId, tableRef) {
            calls.push(['create', title, connectionId, tableRef]);
            const tab = { id: 'new', type: 'result', connectionId, tableRef };
            tabs.push(tab);
            return tab;
        },
    };
    const handle = new Function('queryTabsStore', 'activeConnection', 'executeTableResult', 'tableEdits', 'DbService', `${compiled}; return handleTableSelect;`)(
        store, {value: {id: 1, dbname: 'app'}}, async id => calls.push(['execute', id]),
        {load: async id => calls.push(['load', id])}, {GetActiveConnection: async () => 1});
    return { calls, handle: (node = {}) => handle({ label: 'users', type: 'table', schema: 'public', ...node }) };
}
test('existing table navigation selects its tab without executing or reloading edits', async () => {
    const {calls, handle} = setup([original]);
    await handle();
    assert.deepEqual(calls, [['select', 'existing']]);
});
test('different schemas, databases, connections, consoles, and closing tabs are not reused', async () => {
    for (const tab of [
        {...original, tableRef: {...ref, schema: 'audit'}},
        {...original, tableRef: {...ref, database: 'other'}},
        {...original, connectionId: '2'},
        {...original, type: 'query'},
        {...original, closing: true},
    ]) {
        const {calls, handle} = setup([tab]);
        await handle();
        assert.equal(calls[0][0], 'create');
        assert.deepEqual(calls.slice(1), [['execute', 'new'], ['load', 'new']]);
    }
});
test('repeated navigation reuses a newly opened table tab', async () => {
    const {calls, handle} = setup([]);
    await handle();
    await handle();
    assert.equal(calls.filter(call => call[0] === 'create').length, 1);
    assert.deepEqual(calls.at(-1), ['select', 'new']);
});
