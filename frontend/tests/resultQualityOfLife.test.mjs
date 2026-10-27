import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import ts from 'typescript';
import { computed, ref, effectScope, nextTick } from 'vue';
function load(path, globals = {}) {
  const compiled = ts.transpileModule(readFileSync(new URL(path, import.meta.url), 'utf8'), { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } }).outputText;
  const m = { exports: {} };
  new Function('module', 'exports', 'require', ...Object.keys(globals), compiled)(m, m.exports, createRequire(import.meta.url), ...Object.values(globals));
  return m.exports;
}
const { quickFilter } = load('../src/utils/quickFilter.ts');
const { compileResultFilter } = load('../src/utils/resultFilter.ts');
const { relatedQuery } = load('../src/utils/relatedQuery.ts');
test('quick filters preserve quoted identifiers, apostrophes, booleans and NULL semantics', () => {
  for (const value of ["O'Reilly", true, 42, null, 'a\nb']) {
    const predicate = compileResultFilter(quickFilter('odd"name', value, '='), ['odd"name']);
    assert.equal(predicate([value]), true);
    assert.equal(predicate(['something else']), false);
  }
  assert.equal(compileResultFilter(quickFilter('a', null, '<>'), ['a'])([null]), false);
  assert.equal(compileResultFilter(quickFilter('a', 'foo', 'LIKE'), ['a'])(['afoob']), true);
});
test('related queries pair composite keys and safely encode dialect-specific literals', () => {
  const relation = { target: { name: 'odd"table', schema: 'public', database: 'demo' }, columns: ['a','b'], targetColumns: ['x','y'] };
  assert.equal(relatedQuery(relation, { a: 12, b: "O'Reilly" }, 'sqlite'), `SELECT * FROM "public"."odd""table" WHERE "x" = 12 AND "y" = 'O''Reilly';`);
  assert.match(relatedQuery(relation, { a: 12, b: "\\'; DROP TABLE x;--" }, 'pgx'), /E'\\\\''; DROP TABLE x;--'/);
  assert.match(relatedQuery(relation, { a: 12, b: "'\\" }, 'mysql'), /CONVERT\(X'275c' USING utf8mb4\)/);
  assert.throws(() => relatedQuery(relation, { a: null, b: 1 }, 'pgx'), /NULL/);
  assert.throws(() => relatedQuery(relation, { a: Number.MAX_SAFE_INTEGER + 1, b: 1 }, 'pgx'), /precisely/);
});
test('column layouts retain one visible column, pin and reorder, and restore per-table settings', async () => {
  const storage = new Map();
  const { useColumnLayout } = load('../src/components/workspace/resultGrid/useColumnLayout.ts', { localStorage: { getItem: key => storage.get(key), setItem: (key, value) => storage.set(key, value) } });
  const scope = effectScope();
  try {
    const columns = ref([{Name:'id'}, {Name:'name'}, {Name:'email'}]);
    const key = ref('table-a');
    const layout = scope.run(() => useColumnLayout(computed(() => columns.value), computed(() => key.value)));
    layout.move(2, -1); assert.deepEqual(layout.visible.value, [0,2,1]);
    layout.togglePinned(1); assert.deepEqual(layout.visible.value, [1,0,2]);
    layout.toggleHidden(0); layout.toggleHidden(2); layout.toggleHidden(1);
    assert.deepEqual(layout.visible.value, [1]);
    key.value = 'table-b'; await nextTick(); assert.deepEqual(layout.visible.value, [0,1,2]);
    key.value = 'table-a'; await nextTick(); assert.deepEqual(layout.visible.value, [1]);
    layout.reset(); assert.deepEqual(layout.visible.value, [0,1,2]);
  } finally { scope.stop(); }
});
