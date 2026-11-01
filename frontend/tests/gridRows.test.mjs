import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import ts from 'typescript';
const source = ts.transpileModule(readFileSync(new URL('../src/utils/gridRows.ts', import.meta.url), 'utf8'), {compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS}}).outputText;
const module = {exports: {}}; new Function('exports', source)(module.exports);
const {createGridRowCache} = module.exports;
const columns = [{Name: 'id'}, {Name: 'name'}, {Name: 'data'}];
const base = () => Array.from({length: 1000}, (_, i) => ({row: [i, `person-${i}`, '{"value":1}'], key: `row-${i}`, number: String(i + 1), index: i, status: ''}));
test('one inline edit reuses the other 999 rows and their displayed objects', () => {
    const cache = createGridRowCache(), rows = base(), visible = [0, 1, 2];
    const before = cache.map(cache.overlay(rows, new Map(), columns), visible);
    const draft = {key: 'row-5', operation: 'update', values: {name: 'changed'}, keys: {id: 5}};
    const working = cache.overlay(rows, new Map([[draft.key, draft]]), columns);
    const after = cache.map(working, visible);
    assert.equal(after[5]['column-1'], 'changed');
    assert.equal(after.filter((row, i) => row === before[i]).length, 999);
    assert.equal(rows[5].row[1], 'person-5');
    assert.equal(cache.map(cache.overlay(rows, new Map([[draft.key, draft]]), columns), visible)[5], after[5]);
});
test('replacing drafts, cancelling edits, deleting rows, and changing layout stay correct', () => {
    const cache = createGridRowCache(), rows = base();
    const draft = {key: 'row-0', operation: 'update', values: {name: 'first'}, keys: {id: 0}};
    cache.overlay(rows, new Map([[draft.key, draft]]), columns);
    const changed = cache.overlay(rows, new Map([[draft.key, {...draft, values: {name: 'second'}}]]), columns);
    assert.equal(changed[0].row[1], 'second');
    assert.equal(cache.map(changed, [1, 0])[0]['column-0'], 'second');
    assert.equal(cache.overlay(rows, new Map(), columns)[0], rows[0]);
    const deleted = cache.overlay(rows, new Map([[draft.key, {...draft, operation: 'delete', values: {}}]]), columns);
    assert.equal(deleted[0].status, 'delete');
});
test('off-page drafts preserve originals, defaults, JSON and new-row numbering', () => {
    const cache = createGridRowCache(), rows = base();
    const inserted = {key: 'new', operation: 'insert', keys: {}, values: {name: 'new name', data: {nested: [1]}}};
    const offPage = {key: 'off-page', operation: 'update', keys: {id: 2000}, original: {id: 2000, name: 'old'}, values: {name: 'edited'}};
    const drafts = new Map([[inserted.key, inserted], [offPage.key, offPage]]);
    const working = cache.overlay(rows, drafts, columns);
    assert.deepEqual(working[1000].row, [undefined, 'new name', '{"nested":[1]}']);
    assert.equal(working[1000].number, 'New');
    assert.deepEqual(working[1001].row, [2000, 'edited', undefined]);
    const shifted = cache.overlay(rows, new Map([[offPage.key, offPage]]), columns);
    assert.equal(shifted[1000].index, 1000);
});
test('refetching invalidates row caches even when keys match', () => {
    const cache = createGridRowCache(), rows = base(), draft = {key: 'row-0', operation: 'update', keys: {id: 0}, values: {name: 'changed'}};
    cache.overlay(rows, new Map([[draft.key, draft]]), columns);
    const fresh = base(); fresh[0].row[2] = 'new database value';
    assert.equal(cache.overlay(fresh, new Map([[draft.key, draft]]), columns)[0].row[2], 'new database value');
});
