import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import ts from 'typescript';
const compiled = ts.transpileModule(readFileSync(new URL('../src/utils/gridConvenience.ts', import.meta.url), 'utf8'), {compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS}}).outputText;
const module = {exports: {}};
new Function('exports', compiled)(module.exports);
const {duplicateRowValues, advancedCopy, selectionStatistics} = module.exports;
const table = {name: 'people', schema: 'public', database: 'test'};
const options = {format: 'csv', headers: true, nullText: 'NULL', driver: 'pgx', table};
test('duplicate omits composite keys, identities, generated values, and missing defaults', () => {
    const source = {tenant: 2, id: 1, serial: 9, computed: 'x', name: 'Ada', data: {nested: [1]}, missing: undefined};
    const values = duplicateRowValues(['tenant', 'id', 'serial', 'computed', 'name', 'data', 'missing'].map(name => ({name, generated: name === 'computed', autoIncrement: name === 'serial'})), ['tenant', 'id'], source);
    assert.deepEqual(values, {name: 'Ada', data: {nested: [1]}});
    values.data.nested.push(2);
    assert.deepEqual(source.data.nested, [1]);
});
test('advanced CSV escapes delimiters, quotes, newlines, NULL and empty strings', () => {
    const selection = {columns: ['name', 'note'], rows: [['A,"B', 'line1\nline2'], [null, '']]};
    assert.equal(advancedCopy(selection, options), 'name,note\n"A,""B","line1\nline2"\nNULL,');
    assert.equal(advancedCopy(selection, {...options, format: 'tsv', headers: false, nullText: '\\N'}), '"A,""B"\t"line1\nline2"\n\\N\t');
    assert.equal(advancedCopy({columns: ['x'], rows: [['a|b']]}, {...options, format: 'custom', delimiter: '|'}), 'x\n"a|b"');
    assert.throws(() => advancedCopy(selection, {...options, format: 'custom', delimiter: ''}), /delimiter/);
});
test('JSON copy retains primitive types and rejects ambiguous duplicate column names', () => {
    assert.deepEqual(JSON.parse(advancedCopy({columns: ['id', 'active', 'note'], rows: [[1, true, null]]}, {...options, format: 'json'})), [{id: 1, active: true, note: null}]);
    assert.throws(() => advancedCopy({columns: ['id', 'id'], rows: [[1, 2]]}, {...options, format: 'json'}), /unique/);
});
test('SQL INSERT escapes names and values without executing statements', () => {
    const text = advancedCopy({columns: ['na"me', 'note', 'flag'], rows: [["O'Reilly\\path", null, true]]}, {...options, format: 'insert'});
    assert.equal(text, `INSERT INTO "public"."people" ("na""me", "note", "flag") VALUES (E'O''Reilly\\\\path', NULL, TRUE);`);
    assert.equal(advancedCopy({columns: ['name'], rows: [['value']]}, {...options, driver: 'mysql', format: 'insert'}), "INSERT INTO `test`.`people` (`name`) VALUES (CONVERT(X'76616c7565' USING utf8mb4));");
});
test('SQL UPDATE uses original composite keys even when key columns are not selected', () => {
    const text = advancedCopy({columns: ['name'], rows: [['Ada']], keys: [{tenant: 2, id: 5}]}, {...options, format: 'update'});
    assert.equal(text, `UPDATE "public"."people" SET "name" = E'Ada' WHERE "tenant" = 2 AND "id" = 5;`);
    assert.throws(() => advancedCopy({columns: ['name'], rows: [['Ada']]}, {...options, format: 'update'}), /primary keys/);
    assert.throws(() => advancedCopy({columns: ['name'], rows: [['Ada']], keys: [{id: Number.MAX_SAFE_INTEGER + 1}]}, {...options, format: 'update'}), /precisely/);
});
test('statistics ignore text columns and NULL and preserve exact decimal and bigint arithmetic', () => {
    const stats = selectionStatistics([['0.1', '99', null], ['0.2', '10', 2]], [true, false, true]);
    assert.equal(stats.cells, 6); assert.equal(stats.filled, 5); assert.equal(stats.count, 3);
    assert.equal(stats.sum, '2.3'); assert.equal(stats.min, '0.1'); assert.equal(stats.max, '2');
    assert.equal(stats.average, '≈0.7666666666666');
    assert.equal(selectionStatistics([['9007199254740993'], ['2']], [true]).sum, '9007199254740995');
    assert.equal(selectionStatistics([['1.25e2'], ['-0.5']], [true]).sum, '124.5');
    assert.equal(selectionStatistics([['.5'], ['-.25']], [true]).sum, '0.25');
});
test('statistics handle empty selections and reject already imprecise JS numbers', () => {
    assert.deepEqual(selectionStatistics([], []), {cells: 0, filled: 0, count: 0, excluded: 0, sum: '0', min: '', max: '', average: ''});
    const stats = selectionStatistics([[Number.MAX_SAFE_INTEGER + 1], ['NaN'], [null]], [true]);
    assert.equal(stats.count, 0); assert.equal(stats.excluded, 2);
});
