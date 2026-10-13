import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import ts from 'typescript';
import { createPinia, setActivePinia } from 'pinia';

const require = createRequire(import.meta.url);
const source = readFileSync(new URL('../src/stores/queryTabsStore.ts', import.meta.url), 'utf8');
function setup() {
    const saved = new Map();
    const notifications = [];
    let executions = 0;
    const service = {
        SaveConsole: async console => { saved.set(`${console.connectionId}:${console.id}`, { ...console }); },
        GetSavedConsoles: async connectionId => [...saved.values()].filter(console => console.connectionId === connectionId).sort((a,b) => a.position - b.position),
        DeleteSavedConsole: async (connectionId, id) => { saved.delete(`${connectionId}:${id}`); },
    };
    const connection = { activeConnection: { id: 1, driver: 'pgx' } };
    const overrides = {
        quasar: { Dialog: {}, Notify: { create: value => notifications.push(value) } },
        '@/stores/tableEditsStore': { useTableEditsStore: () => ({ hasPending: () => false, isSaving: () => false, clear() {}, clearAll() {} }) },
        '@/stores/connectionStore': { useConnectionStore: () => connection },
        '@/stores/resultSettingsStore': { useResultSettingsStore: () => ({ settings: { rowsPerPage: 100 } }) },
        '@bindings/db-lens/internal/app': { DatabaseService: service, DbService: {
            ExecuteQuery: async () => null,
            ExecuteConsoleQuery: async () => { executions++; return { rows: [], columns: [], pageSize: 100, startRow: 1 }; },
        } },
        '@bindings/db-lens/internal/engine/entities': { QueryExecutionType: { QueryExecutionExecute: 0 } },
    };
    const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } }).outputText;
    const module = { exports: {} };
    new Function('module', 'exports', 'require', compiled)(module, module.exports, name => overrides[name] ?? require(name));
    setActivePinia(createPinia());
    return { store: module.exports.useQueryTabsStore(), saved, service, connection, notifications, executions: () => executions };
}

test('saved SQL restores on reconnect without execution; explicit close deletes it', async () => {
    const { store, saved, executions } = setup();
    const id = store.tabs[0].id;
    const sql = "SELECT 'long query';\n".repeat(1000);
    store.updateSql(id, sql);
    assert.equal(await store.saveConsole(id), true);
    assert.equal(store.tabs[0].dirty, false);
    store.clearResults();
    assert.equal(saved.size, 1);
    await store.restoreConsoles('1');
    assert.equal(store.tabs.length, 1);
    assert.equal(store.tabs[0].sql, sql);
    assert.equal(store.tabs[0].savedSql, sql);
    assert.equal(executions(), 0);
    await store.closeTab(id);
    assert.equal(saved.size, 0);
    await store.restoreConsoles('1');
    assert.equal(store.tabs[0].sql, '');
});

test('queries from one connection do not restore into another', async () => {
    const { store, connection } = setup();
    const id = store.tabs[0].id;
    store.updateSql(id, 'SELECT 1');
    await store.saveConsole(id);
    store.clearResults();
    connection.activeConnection.id = 2;
    await store.restoreConsoles('2');
    assert.equal(store.tabs[0].sql, '');
    assert.equal(store.tabs[0].connectionId, '2');
    store.clearResults();
    connection.activeConnection.id = 1;
    await store.restoreConsoles('1');
    assert.equal(store.tabs[0].sql, 'SELECT 1');
});

test('executing does not save edits; edits made during saving remain modified', async () => {
    const { store, service } = setup();
    const id = store.tabs[0].id;
    store.updateSql(id, 'SELECT 1');
    await store.execute(id, 'SELECT 1');
    assert.equal(store.tabs[0].dirty, true);
    let finish;
    service.SaveConsole = () => new Promise(resolve => { finish = resolve; });
    const saving = store.saveConsole(id);
    await Promise.resolve();
    store.updateSql(id, 'SELECT 2');
    finish();
    await saving;
    assert.equal(store.tabs[0].savedSql, 'SELECT 1');
    assert.equal(store.tabs[0].dirty, true);
});

test('closing waits for in-flight save so a closed tab cannot be resurrected', async () => {
    const { store, service, saved } = setup();
    const id = store.tabs[0].id;
    store.updateSql(id, 'SELECT 1');
    const original = service.SaveConsole;
    let finish;
    service.SaveConsole = console => new Promise(resolve => { finish = async () => { await original(console); resolve(); }; });
    const saving = store.saveConsole(id);
    await Promise.resolve();
    const closing = store.closeTab(id);
    assert.equal(store.tabs.length, 1);
    await finish();
    await Promise.all([saving, closing]);
    assert.equal(saved.size, 0);
    assert.equal(store.tabs.length, 0);
});

test('failed deletion keeps saved tab open, and stale restoration cannot reopen disconnected tabs', async () => {
    const { store, service, notifications } = setup();
    const id = store.tabs[0].id;
    await store.saveConsole(id);
    service.DeleteSavedConsole = async () => { throw new Error('disk error'); };
    await store.closeTab(id);
    assert.equal(store.tabs.length, 1);
    assert.equal(store.tabs[0].closing, false);
    assert.match(notifications[0].message, /Could not close saved console/);
    let resolve;
    service.GetSavedConsoles = () => new Promise(done => { resolve = done; });
    const restore = store.restoreConsoles('1');
    await Promise.resolve();
    store.clearResults();
    resolve([{ id: 'old', title: 'old.sql', sql: 'SELECT 1', createdAt: 1 }]);
    await restore;
    assert.equal(store.tabs.length, 0);
});
