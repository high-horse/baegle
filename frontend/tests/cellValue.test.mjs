import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';
const compiled = ts.transpileModule(readFileSync(new URL('../src/utils/cellValue.ts', import.meta.url), 'utf8'), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
}).outputText;
const module = {exports: {}};
new Function('module', 'exports', compiled)(module, module.exports);
const {cellValue} = module.exports;
test('long text, XML, empty strings and NULL retain their content', () => {
    for (const text of ['a\n'.repeat(10000), '', '<root> a <b/> b </root>', '<script>alert(1)</script>']) {
        assert.equal(cellValue(text).raw, text);
        assert.equal(cellValue(text).formatted, text);
    }
    assert.equal(cellValue(null, 'BLOB').raw, 'NULL');
    assert.equal(cellValue('<root/>', 'XML').kind, 'XML');
});
test('JSON formats separately from raw values and malformed JSON stays readable', () => {
    const value = cellValue('{"a":1}', 'JSONB');
    assert.equal(value.kind, 'JSON');
    assert.equal(value.raw, '{"a":1}');
    assert.equal(value.formatted, '{\n  "a": 1\n}');
    assert.equal(cellValue('{broken', 'JSON').raw, '{broken');
    assert.equal(cellValue({a: 1}).kind, 'JSON');
});
test('binary hex and typed arrays preserve bytes and display offsets and ASCII', () => {
    for (const input of ['0x0041ff', '\\x0041ff', "X'0041ff'", new Uint8Array([0,65,255]), [0,65,255]]) {
        const result = cellValue(input, 'BLOB');
        assert.equal(result.kind, 'Binary');
        assert.equal(result.size, '3 bytes');
        assert.match(result.formatted, /^00000000  00 41 ff/);
        assert.match(result.formatted, /\|\.A\.\|$/);
    }
    assert.equal(cellValue('0x', 'BYTEA').size, '0 bytes');
    assert.equal(cellValue('0x0041', 'VARCHAR').kind, 'Text');
    assert.match(cellValue('0x' + '41'.repeat(17), 'VARBINARY(20)').formatted, /\n00000010/);
});
