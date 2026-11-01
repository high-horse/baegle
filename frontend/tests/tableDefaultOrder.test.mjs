import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';
const source = readFileSync(new URL('../src/layouts/WorkspaceLayout.vue', import.meta.url), 'utf8').split('<script setup lang="ts">')[1].split('</script>')[0];
const ast = ts.createSourceFile('workspace.ts', source, ts.ScriptTarget.Latest, true);
const functions = ast.statements.filter(node => ts.isFunctionDeclaration(node) && ['executeTableResult', 'quoteIdentifier'].includes(node.name?.text));
const compiled = ts.transpileModule(functions.map(node => node.getText(ast)).join('\n'), { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;
async function browse(keys) {
    const queries = [];
    const edits = { states: {}, async load(id) { this.states[id] = { info: { keys } }; } };
    const execute = new Function('activeConnection', 'tableEdits', 'DbService', 'queryTabsStore', 'executeQuery', `${compiled}; return executeTableResult;`)(
        { value: { driver: 'pgx', dbname: 'test' } }, edits, { GetActiveConnection: async () => 'connection' },
        { tabs: [], updateSql() {} }, async (id, sql) => queries.push(sql));
    await execute('tab', { label: 'items', schema: 'public' });
    return queries[0];
}
test('browse uses all primary keys in stable order and quotes identifiers', async () => {
    assert.equal(await browse(['tenant', 'item"id']), 'SELECT * FROM "public"."items" ORDER BY "tenant", "item""id";');
    assert.equal(await browse([]), 'SELECT * FROM "public"."items";');
});
