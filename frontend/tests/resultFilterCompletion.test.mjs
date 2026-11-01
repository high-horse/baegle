import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';
import { EditorState } from '@codemirror/state';
import { CompletionContext } from '@codemirror/autocomplete';
import { sql, SQLite } from '@codemirror/lang-sql';

const compiled = ts.transpileModule(readFileSync(new URL('../src/utils/resultFilterCompletion.ts', import.meta.url), 'utf8'), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
}).outputText;
const module = { exports: {} };
new Function('module', 'exports', 'require', compiled)(module, module.exports, createRequire(import.meta.url));
const columns = [{ Name: 'name' }, { Name: 'age' }];
function complete(doc, mode) {
    const state = EditorState.create({ doc, extensions: [sql({ dialect: SQLite })] });
    return module.exports.completeResultExpression(new CompletionContext(state, doc.length, false), columns, mode);
}
test('columns complete after an inserted quoted column and subsequent conditions', () => {
    for (const doc of ['"name" = \'Alice\' AND ag', '"name" = \'Alice\' OR ag', '"name" = \'Alice\' AND "age" > 20 AND ag', '`name` = 1 AND ag']) {
        const result = complete(doc);
        assert.equal(result.from, doc.length - 2);
        const age = result.options.find(option => option.label === 'age');
        assert.equal(doc.slice(0, result.from) + age.apply, doc.slice(0, -2) + '"age"');
    }
});
test('columns appear after boolean separators and order commas', () => {
    for (const [doc, mode] of [['', 'where'], ['"age" > 20 AND ', 'where'], ['"age" > 20 OR ', 'where'], ['NOT (', 'where'], ['"name" ASC, ', 'order']]) {
        assert.ok(complete(doc, mode).options.some(option => option.label === 'age'));
    }
});
test('quoted partial identifiers complete and strings and comments stay excluded', () => {
    for (const quote of ['"', '`']) {
        const doc = '"name" = 1 AND ' + quote + 'ag';
        const result = complete(doc);
        assert.equal(result.from, doc.length - 3);
        assert.equal(result.options.find(option => option.displayLabel === 'age').apply, quote + 'age' + quote);
    }
    assert.equal(complete('"name" = \'ag'), null);
    assert.equal(complete('"name" = 1 -- ag'), null);
});
