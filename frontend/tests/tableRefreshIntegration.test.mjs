import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createRequire} from 'node:module';
import test from 'node:test';
import ts from 'typescript';
import {createPinia, defineStore, setActivePinia} from 'pinia';
import * as dialect from '../src/utils/databaseDialect.ts';
const require = createRequire(import.meta.url);
function evaluate(path, overrides = {}) {
    const output = ts.transpileModule(readFileSync(new URL(path, import.meta.url), 'utf8'), {compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS}}).outputText;
    const module = {exports: {}};
    new Function('module', 'exports', 'require', output)(module, module.exports, name => overrides[name] ?? require(name));
    return module.exports;
}
const {refreshTable} = evaluate('../src/utils/refreshTable.ts');
test('refresh works with existing Pinia stores and actually refetches after discarding', async () => {
    setActivePinia(createPinia());
    const editsModule = evaluate('../src/stores/tableEditsStore.ts', {'@bindings/db-lens/internal/app': {DbService: {}}});
    const edits = editsModule.useTableEditsStore();
    let requests = 0;
    const backend = {
        async ExecuteConsoleQuery() {
            requests++;
            assert.equal(edits.hasPending('table'), false);
            assert.deepEqual(edits.states.table.drafts, []);
            return {columns: [{name: 'id'}, {name: 'name'}], rows: [[1, 'saved value']], startRow: 1, pageSize: 100, isQuery: true, canNavigate: true, hasMore: false};
        },
        async ExecuteQuery() {},
    };
    const {useQueryTabsStore} = evaluate('../src/stores/queryTabsStore.ts', {
        '@/utils/databaseDialect': dialect,
        '@/stores/tableEditsStore': editsModule,
        '@/stores/connectionStore': {useConnectionStore: () => ({activeConnection: {id: 1, driver: 'sqlite'}})},
        '@/stores/resultSettingsStore': {useResultSettingsStore: () => ({settings: {rowsPerPage: 100}})},
        '@bindings/db-lens/internal/app': {DbService: backend, DatabaseService: {}},
        '@bindings/db-lens/internal/engine/entities': {QueryExecutionType: {QueryExecutionExecute: 0}},
        quasar: {Dialog: {}, Notify: {create() {}}},
    });
    const queries = useQueryTabsStore();
    queries.tabs.push({id: 'table', type: 'result', sql: 'SELECT * FROM items ORDER BY id', executedSql: 'SELECT * FROM items ORDER BY id', loading: false,
        tableRef: {name: 'items', schema: '', database: ''}, result: {Rows: [[1, 'saved value']]}});
    edits.states.table = {drafts: [
        {key: 'one', operation: 'update', keys: {id: 1}, values: {name: 'unsaved'}},
        {key: 'new', operation: 'insert', keys: {}, values: {name: 'unsaved new row'}},
        {key: 'two', operation: 'delete', keys: {id: 2}, values: {}},
    ], editing: true, loading: false, saving: false, error: ''};
    assert.equal(typeof queries.refresh, 'undefined', 'existing stores need no newly added action');
    await refreshTable(queries, edits, 'table');
    assert.equal(requests, 1);
    assert.equal(edits.states.table.editing, false);
    assert.deepEqual(queries.tabs.find(tab => tab.id === 'table').result.Rows, [[1, 'saved value']]);
    assert.equal(queries.tabs.find(tab => tab.id === 'table').error, null);
});
test('redefining a Pinia store leaves new actions unavailable on a live instance', () => {
    setActivePinia(createPinia());
    const old = defineStore('live-refresh-test', () => ({execute() {}}))();
    const updated = defineStore('live-refresh-test', () => ({execute() {}, refresh() {}}))();
    assert.equal(updated, old);
    assert.equal(typeof updated.refresh, 'undefined');
});
