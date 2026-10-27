import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(new URL('../src/utils/resultClipboard.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } }).outputText;
const module = { exports: {} };
new Function('module', 'exports', compiled)(module, module.exports);
const { encodeGridText, parseGridText, validateGridCells, prepareGridPaste } = module.exports;

test('spreadsheet text round-trips tabs, quoted/multiline strings and Unicode', () => {
    const rows = [['plain', 'a\tb', 'line 1\nline 2'], ['नमस्ते', '"quoted"', '']];
    assert.deepEqual(parseGridText(encodeGridText(rows)), rows);
    assert.deepEqual(parseGridText('one\ttwo\r\nthree\tfour\r\n'), [['one', 'two'], ['three', 'four']]);
    assert.equal(encodeGridText([[null, '', false, 0]]), '\t\tfalse\t0');
    assert.deepEqual(parseGridText(''), [['']]);
});

test('malformed and ragged clipboard data is rejected instead of shifting cells', () => {
    for (const text of ['a\tb\nc', '"unclosed', '"quoted"extra']) assert.throws(() => parseGridText(text));
    for (const value of [[], [[]], [[{}]], [[NaN]], [[1], [1, 2]], 'text']) assert.throws(() => validateGridCells(value));
    validateGridCells([[null, '', 'NULL', false, 123]]);
});

const columns = [{ name: 'name', databaseType: 'TEXT', nullable: false }, { name: 'count', databaseType: 'BIGINT', nullable: true }, { name: 'enabled', databaseType: 'BOOLEAN' }];
const target = { key: 'row-1', keys: { id: 1 }, original: { name: 'old', count: '5', enabled: false } };

test('pastes validate every row, preserve large integers, and merge staged changes', () => {
    const draft = { ...target, operation: 'update', values: { note: 'keep me' } };
    const pasted = prepareGridPaste(columns, ['id'], [{ ...target, draft }], [['new', '9007199254740993', 'true']]);
    assert.deepEqual(pasted[0].values, { note: 'keep me', name: 'new', count: '9007199254740993', enabled: true });
    assert.deepEqual(pasted[0].keys, { id: 1 });
    assert.equal(target.original.name, 'old');
    assert.deepEqual(draft.values, { note: 'keep me' });
    assert.throws(() => prepareGridPaste(columns, ['id'], [target, target], [['new', '6', 'true'], ['other', 'invalid', 'false']]));
    assert.equal(target.original.name, 'old', 'validation cannot partially stage a paste');
});

test('NULL and empty strings stay distinct and restoring original values removes staged changes', () => {
    const draft = { ...target, operation: 'update', values: { name: 'changed' } };
    assert.deepEqual(prepareGridPaste(columns, ['id'], [{ ...target, draft }], [['old', '5', false]])[0].values, {});
    assert.deepEqual(prepareGridPaste(columns, ['id'], [target], [['', null, false]])[0].values, { name: '', count: null });
    assert.throws(() => prepareGridPaste(columns, ['id'], [target], [[null, '5', false]]), /does not allow NULL/);
});

test('identity/generated columns, deleted rows and imprecise keys cannot be pasted over', () => {
    for (const column of [{ name: 'id', databaseType: 'INT' }, { name: 'key', databaseType: 'TEXT', primaryKey: true }, { name: 'generated', databaseType: 'INT', generated: true }]) {
        assert.throws(() => prepareGridPaste([column], ['id'], [target], [['123']]), /read-only/);
    }
    assert.throws(() => prepareGridPaste(columns, ['id'], [{ ...target, draft: { operation: 'delete' } }], [['x', '1', true]]), /deletion/);
    assert.throws(() => prepareGridPaste(columns, ['id'], [{ ...target, keys: { id: 9007199254740992 } }], [['x', '1', true]]), /precisely/);
});
