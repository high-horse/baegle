import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import ts from 'typescript';
function load(path, overrides = {}) {
    const source = ts.transpileModule(readFileSync(new URL(path, import.meta.url), 'utf8'), {compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS}}).outputText;
    const module = {exports: {}}; new Function('exports', 'require', source)(module.exports, name => overrides[name]); return module.exports;
}
const grid = load('../src/utils/gridConvenience.ts');
const {createCopyExecutor} = load('../src/utils/copyExecution.ts', {'./gridConvenience': grid});
const options = {format: 'csv', headers: true, nullText: 'NULL', driver: 'pgx'};
const large = {columns: ['id'], rows: Array.from({length: 20000}, (_, i) => [i])};
function fakeWorker(action) {
    const handlers = {}; let terminated = 0;
    return {worker: {addEventListener(name, fn) {handlers[name] = fn;}, terminate() {terminated++;}, postMessage(request) {queueMicrotask(() => action(request, handlers));}}, terminated: () => terminated};
}
test('large copies use a worker and release it after the full output arrives', async () => {
    const worker = fakeWorker((request, handlers) => handlers.message({data: {ok: true, text: grid.advancedCopy(request.selection, request.options)}}));
    const text = await createCopyExecutor(() => worker.worker)(large, options);
    assert.equal(text, grid.advancedCopy(large, options));
    assert.equal(worker.terminated(), 1);
});
test('small copies avoid worker startup', async () => {
    let starts = 0;
    assert.equal(await createCopyExecutor(() => {starts++;})( {columns: ['id'], rows: [[1]]}, options), 'id\n1');
    assert.equal(starts, 0);
});
test('unsupported workers, clone failures, and loading errors retain the original copy behavior', async () => {
    for (const factory of [() => undefined, () => {throw new Error('unavailable');}, () => ({...fakeWorker(() => {}).worker, postMessage() {throw new Error('cannot clone');}}), () => fakeWorker((_request, handlers) => handlers.error({})).worker]) {
        assert.equal(await createCopyExecutor(factory)(large, options), grid.advancedCopy(large, options));
    }
});
test('format validation errors from the worker preserve the error and release resources', async () => {
    const worker = fakeWorker((_request, handlers) => handlers.message({data: {ok: false, error: 'Missing original primary keys'}}));
    await assert.rejects(createCopyExecutor(() => worker.worker)(large, options), /Missing original primary keys/);
    assert.equal(worker.terminated(), 1);
});
