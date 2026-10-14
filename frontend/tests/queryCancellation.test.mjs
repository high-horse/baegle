import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import ts from 'typescript';
import { createPinia, setActivePinia } from 'pinia';

const require = createRequire(import.meta.url);
const source = readFileSync(new URL('../src/stores/queryTabsStore.ts', import.meta.url), 'utf8');
function setup(rowsPerPage = 100) {
    const calls = [];
    const cancellations = [];
    const backend = {
        ExecuteConsoleQuery(id, requestID, input) {
            return new Promise((resolve, reject) => calls.push({ id, requestID, input, resolve, reject }));
        },
        CancelConsoleQuery: async (id, requestID) => { cancellations.push({ id, requestID }); return true; },
        ExecuteQuery: async () => null,
    };
    const edits = { hasPending: () => false, isSaving: () => false, clear() {}, clearAll() {} };
    const overrides = {
        quasar: { Dialog: {} },
        '@/stores/tableEditsStore': { useTableEditsStore: () => edits },
        '@/stores/connectionStore': { useConnectionStore: () => ({ activeConnection: { driver: 'sqlite' } }) },
        '@/stores/resultSettingsStore': { useResultSettingsStore: () => ({ settings: { rowsPerPage } }) },
        '@bindings/db-lens/internal/app': { DbService: backend },
        '@bindings/db-lens/internal/engine/entities': { QueryExecutionType: { QueryExecutionExecute: 0, QueryExecutionClose: 2, QueryExecutionNavigate: 4, QueryExecutionFetchPaged: 1, QueryExecutionCount: 5 } },
    };
    const module = { exports: {} };
    const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } }).outputText;
    new Function('module', 'exports', 'require', compiled)(module, module.exports, name => overrides[name] ?? require(name));
    setActivePinia(createPinia());
    return { store: module.exports.useQueryTabsStore(), calls, cancellations, backend };
}

test('SQL files open without execution; Run script uses edited text and shares the console lock', async () => {
    const { store, calls, cancellations, backend } = setup();
    const tab = store.openSQLScript('dump.sql', 'SELECT 1;', '7');
    assert.equal(calls.length, 0);
    assert.equal(tab.title, 'dump.sql');
    assert.equal(tab.dirty, true);
    assert.equal(tab.importedScript, true);
    store.updateSql(tab.id, 'CREATE TABLE edited(id INTEGER); SELECT 2;');
    let finish;
    let scriptCall;
    backend.ExecuteConsoleScript = (...args) => {
        scriptCall = args;
        return new Promise(resolve => { finish = resolve; });
    };
    const run = store.executeScript(tab.id);
    assert.equal(scriptCall[2], '7');
    assert.equal(scriptCall[3], 'CREATE TABLE edited(id INTEGER); SELECT 2;');
    assert.equal(tab.runningScript, true);
    await store.execute(tab.id, 'SELECT overlap');
    assert.equal(calls.length, 0);
    await store.cancelQuery(tab.id);
    assert.equal(cancellations.length, 1);
    assert.equal(tab.loading, true);
    finish({ completed: 1, total: 2, error: 'Cancelled', errorLine: 2, cancelled: true, duration: 123 });
    await run;
    assert.equal(tab.scriptResult.completed, 1);
    assert.equal(tab.scriptResult.cancelled, true);
    assert.equal(tab.loading, false);
    assert.equal(tab.runningScript, false);
    assert.equal(tab.dirty, true);
});

test('Run and Ctrl+Enter cannot overlap, including while Stop is pending', async () => {
    const { store, calls, cancellations } = setup();
    const id = store.tabs[0].id;
    const execution = store.execute(id, 'SELECT 1');
    await store.execute(id, 'SELECT 2');
    assert.equal(calls.length, 1);
    await store.cancelQuery(id);
    assert.deepEqual(cancellations, [{ id, requestID: calls[0].requestID }]);
    assert.equal(store.tabs[0].loading, true);
    assert.equal(store.tabs[0].stopping, true);
    await store.execute(id, 'SELECT 3');
    assert.equal(calls.length, 1);
    calls[0].reject(new Error('context canceled'));
    await execution;
    assert.equal(store.tabs[0].loading, false);
    assert.equal(store.tabs[0].error, 'Query cancelled');
    const next = store.execute(id, 'SELECT 4');
    assert.equal(calls.length, 2);
    calls[1].reject(new Error('test complete'));
    await next;
});

test('new queries use saved page size while post-save refresh preserves the existing page', async () => {
    const { store, calls } = setup(250);
    const id = store.tabs[0].id;
    const first = store.execute(id, 'SELECT 1');
    assert.equal(calls[0].input.pageSize, 250);
    calls[0].reject(new Error('test complete'));
    await first;
    store.tabs[0].result = { CanNavigate: true, StartRow: 101, PageSize: 100 };
    const second = store.execute(id, 'SELECT 1', true);
    assert.equal(calls[1].input.pageSize, 100);
    assert.equal(calls[1].input.page, 2);
    calls[1].reject(new Error('test complete'));
    await second;
});

test('each console executes independently and closing a running tab cancels it', async () => {
    const { store, calls, cancellations } = setup();
    const first = store.tabs[0].id;
    const second = store.createTab().id;
    const a = store.execute(first, 'SELECT 1');
    const b = store.execute(second, 'SELECT 2');
    assert.equal(calls.length, 2);
    store.closeTab(first);
    assert.equal(cancellations.length, 1);
    assert.equal(cancellations[0].id, first);
    assert.equal(store.tabs[0].loading, true);
    calls[0].reject(new Error('cancelled'));
    calls[1].reject(new Error('test complete'));
    await Promise.all([a, b]);
});

test('Stop retries if it arrives before execution registration', async () => {
    const { store, calls, backend } = setup();
    let attempts = 0;
    backend.CancelConsoleQuery = async () => ++attempts > 1;
    const id = store.tabs[0].id;
    const execution = store.execute(id, 'SELECT 1');
    await store.cancelQuery(id);
    assert.equal(attempts, 2);
    assert.equal(store.tabs[0].loading, true);
    calls[0].reject(new Error('cancelled'));
    await execution;
});

test('a failed Stop remains retryable and does not unlock a running console', async () => {
    const { store, calls, backend } = setup();
    backend.CancelConsoleQuery = async () => { throw new Error('transport unavailable'); };
    const id = store.tabs[0].id;
    const execution = store.execute(id, 'SELECT 1');
    await store.cancelQuery(id);
    assert.equal(store.tabs[0].loading, true);
    assert.equal(store.tabs[0].stopping, false);
    assert.match(store.tabs[0].error, /Could not cancel query/);
    await store.execute(id, 'SELECT 2');
    assert.equal(calls.length, 1);
    backend.CancelConsoleQuery = async () => true;
    await store.cancelQuery(id);
    calls[0].reject(new Error('cancelled'));
    await execution;
});

test('a result racing with Stop is discarded and its cursor is released', async () => {
    const { store, calls, backend } = setup();
    const released = [];
    backend.ExecuteQuery = async input => { released.push(input.cursor); return null; };
    const id = store.tabs[0].id;
    const execution = store.execute(id, 'SELECT 1');
    await store.cancelQuery(id);
    calls[0].resolve({ cursor: 'late-result' });
    await execution;
    assert.deepEqual(released, ['late-result']);
    assert.equal(store.tabs[0].result, null);
    assert.equal(store.tabs[0].error, 'Query cancelled');
});


test('Next and final-page totals do not require counting', async () => {
    const { store, backend } = setup();
    const tab = store.tabs[0];
    tab.executedSql = 'SELECT id FROM items';
    tab.result = { CanNavigate: true, StartRow: 1, PageSize: 100, HasMore: true, Rows: [[1]], Columns: [] };
    tab.pages = [tab.result];
    const requests = [];
    backend.ExecuteQuery = async input => {
        requests.push(input);
        return { columns: [], rows: [[101]], startRow: 101, pageSize: 100, hasMore: false, canNavigate: true, totalRows: 101, cursor: '', isQuery: true };
    };
    await store.navigate(tab.id, 'next');
    assert.equal(requests[0].page, 2);
    assert.equal(requests[0].type, 4);
    assert.equal(tab.totalRows, 101);
    await store.navigate(tab.id, 'previous');
    assert.equal(tab.result.StartRow, 1);
    assert.equal(requests.length, 1, 'Previous uses the cached first page');
    await store.navigate(tab.id, 'last');
    assert.equal(tab.result.StartRow, 101);
    await store.navigate(tab.id, 'next');
    assert.equal(requests.length, 1, 'Next stops at the final page');
});


function countableTab(store) {
    const tab = store.tabs[0];
    tab.executedSql = 'SELECT id FROM items';
    tab.result = { CanNavigate: true, CanSort: true, StartRow: 1, PageSize: 100, HasMore: true, Rows: [[1]], Columns: [] };
    tab.pages = [tab.result];
    return tab;
}

test('on-demand count retains visible data and Last jumps directly after counting', async () => {
    const { store, calls, backend } = setup();
    const tab = countableTab(store);
    const original = tab.result;
    const pages = [];
    backend.ExecuteQuery = async input => {
        pages.push(input);
        return { columns: [], rows: Array.from({length: 50}, (_, i) => [201 + i]), startRow: 201, pageSize: 100, hasMore: false, canNavigate: true, totalRows: 250, cursor: '', isQuery: true };
    };
    const last = store.navigate(tab.id, 'last');
    assert.equal(tab.countingRows, true);
    assert.equal(tab.loading, false);
    assert.equal(tab.result, original);
    assert.equal(calls[0].input.type, 5);
    assert.equal(calls[0].input.query, tab.executedSql);
    assert.equal(calls[0].id, `${tab.id}:count`);
    assert.equal(pages.length, 0);
    calls[0].resolve({ totalRows: 250 });
    await last;
    assert.equal(tab.countingRows, false);
    assert.equal(tab.totalRows, 250);
    assert.deepEqual(pages.map(input => input.page), [3]);
    assert.equal(tab.result.StartRow, 201);
});

test('cancelled count cannot update totals or trigger a delayed Last jump', async () => {
    const { store, calls, cancellations, backend } = setup();
    const tab = countableTab(store);
    const original = tab.result;
    let pageRequests = 0;
    backend.ExecuteQuery = async () => { pageRequests++; return null; };
    const last = store.navigate(tab.id, 'last');
    await store.cancelRowCount(tab.id);
    assert.equal(cancellations[0].id, `${tab.id}:count`);
    assert.equal(tab.stoppingCount, true);
    calls[0].resolve({ totalRows: 999 });
    await last;
    assert.equal(tab.totalRows, undefined);
    assert.equal(pageRequests, 0);
    assert.equal(tab.result, original);
    assert.equal(tab.countingRows, false);
});

test('count failure leaves rows usable and allows retry, including zero totals', async () => {
    const { store, calls } = setup();
    const tab = countableTab(store);
    const count = store.countRows(tab.id);
    calls[0].reject(new Error('query timed out'));
    assert.equal(await count, false);
    assert.match(tab.countError, /timed out/);
    assert.equal(tab.result.Rows.length, 1);
    const retry = store.countRows(tab.id);
    calls[1].resolve({ totalRows: 0 });
    assert.equal(await retry, true);
    assert.equal(tab.totalRows, 0);
    assert.equal(tab.countError, undefined);
});

test('refresh cancels a pending count and ignores its stale total', async () => {
    const { store, calls, cancellations } = setup();
    const tab = countableTab(store);
    const count = store.countRows(tab.id);
    const refresh = store.execute(tab.id, 'SELECT id FROM other_items');
    assert.equal(cancellations[0].id, `${tab.id}:count`);
    calls[0].resolve({ totalRows: 999 });
    await count;
    calls[1].resolve({ columns: [], rows: [[1]], startRow: 1, pageSize: 100, hasMore: false, canNavigate: true, totalRows: 1 });
    await refresh;
    assert.equal(tab.totalRows, 1);
});
