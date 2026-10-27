import * as databaseDialect from "../src/utils/databaseDialect.ts";
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import ts from 'typescript';
import { createPinia, setActivePinia } from 'pinia';

const require = createRequire(import.meta.url);
const source = readFileSync(new URL('../src/stores/queryTabsStore.ts', import.meta.url), 'utf8');
function setup({ saved = new Map(), storage = new Map() } = {}) {
    const notifications = [];
    let executions = 0;
    const service = {
        SaveConsole: async console => { saved.set(`${console.connectionId}:${console.id}`, { ...console }); },
        GetSavedConsoles: async connectionId => [...saved.values()].filter(console => console.connectionId === connectionId).sort((a,b) => a.position - b.position),
        DeleteSavedConsole: async (connectionId, id) => { saved.delete(`${connectionId}:${id}`); },
    };
    const connection = { activeConnection: { id: 1, driver: 'pgx' } };
    const overrides = {
        "@/utils/databaseDialect": databaseDialect,
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
    let scheduled;
    const localStorage = {
        getItem: key => storage.get(key) ?? null,
        setItem: (key, value) => storage.set(key, value),
        removeItem: key => storage.delete(key),
    };
    new Function('module', 'exports', 'require', 'localStorage', 'setTimeout', 'clearTimeout', compiled)(
        module, module.exports, name => overrides[name] ?? require(name), localStorage,
        callback => { scheduled = callback; return 1; }, () => { scheduled = undefined; });
    setActivePinia(createPinia());
    return { store: module.exports.useQueryTabsStore(), saved, storage, service, connection, notifications,
        autosave: async () => { const callback = scheduled; scheduled = undefined; if (callback) await callback(); },
        executions: () => executions };
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

test('edits made during saving remain modified until the next autosave', async () => {
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

test('typing autosaves without Ctrl+S and restores after restarting the store', async () => {
    const first = setup();
    await first.store.restoreConsoles('1');
    const id = first.store.tabs[0].id;
    first.store.updateSql(id, 'SELECT 1;\nSELECT 2;');
    assert.equal(first.saved.size, 0, 'database writes are debounced');
    assert.ok(first.storage.get('baegle.workspace.1').includes('SELECT 2;'), 'latest typing has immediate recovery');
    await first.autosave();
    assert.equal(first.saved.get(`1:${id}`).sql, 'SELECT 1;\nSELECT 2;');
    assert.equal(first.store.tabs[0].dirty, false);
    assert.equal(first.storage.has('baegle.workspace.1'), false, 'durable saves release the recovery snapshot');
    const restarted = setup(first);
    await restarted.store.restoreConsoles('1');
    assert.equal(restarted.store.tabs[0].sql, 'SELECT 1;\nSELECT 2;');
    assert.equal(restarted.executions(), 0);
});

test('recovery retains latest typing when the window exits before the database save', async () => {
    const first = setup();
    await first.store.restoreConsoles('1');
    const id = first.store.tabs[0].id;
    first.store.updateSql(id, 'SELECT 1');
    await first.autosave();
    first.store.updateSql(id, 'SELECT 123');
    const restarted = setup(first);
    await restarted.store.restoreConsoles('1');
    assert.equal(restarted.store.tabs[0].sql, 'SELECT 123');
    await restarted.autosave();
    assert.equal(first.saved.get(`1:${id}`).sql, 'SELECT 123');
});

test('table tabs, empty consoles, tab order and active selection survive restart', async () => {
    const first = setup();
    await first.store.restoreConsoles('1');
    const blank = first.store.tabs[0];
    const table = first.store.createResultTab('users', undefined, { name: 'users', schema: 'public', database: 'app' });
    first.store.updateSql(table.id, 'SELECT * FROM "public"."users";');
    table.result = { Rows: [[1, 'private row data']] };
    first.store.moveTab(table.id, blank.id, 'before');
    first.store.selectTab(table.id);
    await first.autosave();
    const restarted = setup(first);
    await restarted.store.restoreConsoles('1');
    assert.deepEqual(restarted.store.tabs.map(tab => tab.id), [table.id, blank.id]);
    assert.equal(restarted.store.activeTabId, table.id);
    assert.equal(restarted.store.tabs[0].type, 'result');
    assert.deepEqual(restarted.store.tabs[0].tableRef, table.tableRef);
    assert.equal(restarted.store.tabs[0].needsTableRestore, true);
    assert.equal(restarted.store.tabs[0].result, null);
    assert.equal(restarted.store.tabs[1].sql, '');
    assert.equal(restarted.executions(), 0, 'store restore does not run saved SQL');
    assert.ok(!JSON.stringify([...first.saved.values()]).includes('private row data'));
});

test('closing before autosave does not resurrect a tab from local recovery', async () => {
    const first = setup();
    await first.store.restoreConsoles('1');
    const tab = first.store.tabs[0];
    first.store.updateSql(tab.id, 'SELECT 1');
    await first.store.closeTab(tab.id);
    await first.autosave();
    const restarted = setup(first);
    await restarted.store.restoreConsoles('1');
    assert.ok(!restarted.store.tabs.some(item => item.id === tab.id));
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
