import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import ts from 'typescript';
import {computed, reactive} from 'vue';
import {EditorState} from '@codemirror/state';
import {CompletionContext} from '@codemirror/autocomplete';
import * as language from '@codemirror/language';
import * as autocomplete from '@codemirror/autocomplete';
import {PostgreSQL, MySQL, sql, schemaCompletionSource} from '@codemirror/lang-sql';
const helperSource = ts.transpileModule(readFileSync(new URL('../src/utils/sqlCompletion.ts', import.meta.url), 'utf8'), {compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS}}).outputText;
const helper = {exports: {}}; new Function('exports', 'require', helperSource)(helper.exports, name => name === '@codemirror/language' ? language : autocomplete);
const source = readFileSync(new URL('../src/components/workspace/SqlEditor.vue', import.meta.url), 'utf8').split('<script setup lang="ts">')[1].split('</script>')[0];
const ast = ts.createSourceFile('editor.ts', source, ts.ScriptTarget.Latest, true);
const required = ['completionTableLookup', 'findCompletionTable', 'completionColumns', 'cachedSchemaCompletion', 'completeSQLSchema'];
const selected = ast.statements.filter(node => ts.isFunctionDeclaration(node) ? required.includes(node.name?.text) : ts.isVariableStatement(node) && node.declarationList.declarations.some(decl => required.includes(decl.name.getText(ast)))).map(node => node.getText(ast)).join('\n');
const compiled = ts.transpileModule(selected, {compilerOptions: {target: ts.ScriptTarget.ES2022}}).outputText;
function setup() {
    const props = reactive({dbDriver: 'pgx', tables: [{name: 'users', schema: 'public', type: 'TABLE'}]});
    const cache = reactive({}); let builds = 0;
    const connectionStore = {getCachedTableColumns: table => cache[table.name] ?? [], loadTableColumns: async table => cache[table.name] ?? []};
    const globals = {props, connectionStore, computed, isJSONDriver: driver => driver === 'mongodb', getDialect: driver => driver === 'mysql' ? MySQL : PostgreSQL,
        ...helper.exports, schemaCompletionSource,
        buildSQLCompletionSchema(...args) {builds++; return helper.exports.buildSQLCompletionSchema(...args);}};
    const api = new Function(...Object.keys(globals), compiled + ';return {completeSQLSchema};')(...Object.values(globals));
    const context = () => {const state = EditorState.create({doc: 'SELECT * FROM ', extensions: [sql({dialect: props.dbDriver === 'mysql' ? MySQL : PostgreSQL})]}); return new CompletionContext(state, state.doc.length, true);};
    return {api, props, cache, context, connectionStore, builds: () => builds};
}
test('suggestion schema is reused while typing and rebuilt for columns, tables, and dialect changes', async () => {
    const state = setup();
    const result = await state.api.completeSQLSchema(state.context());
    assert.ok(result.options.some(option => option.label === 'users'));
    await state.api.completeSQLSchema(state.context());
    assert.equal(state.builds(), 1);
    state.cache.users = [{name: 'new_column', databaseType: 'TEXT'}];
    await state.api.completeSQLSchema(state.context());
    assert.equal(state.builds(), 2);
    state.props.tables = [...state.props.tables, {name: 'orders', schema: 'public', type: 'TABLE'}];
    const updated = await state.api.completeSQLSchema(state.context());
    assert.ok(updated.options.some(option => option.label === 'orders'));
    assert.equal(state.builds(), 3);
    state.props.dbDriver = 'mysql';
    await state.api.completeSQLSchema(state.context());
    assert.equal(state.builds(), 4);
});
test('metadata refresh invalidates cached suggestions instead of retaining old columns', async () => {
    const state = setup();
    state.cache.users = [{name: 'old_column', databaseType: 'TEXT'}];
    await state.api.completeSQLSchema(state.context());
    delete state.cache.users;
    await state.api.completeSQLSchema(state.context());
    assert.equal(state.builds(), 2);
});
test('suggestions from an outgoing dialect are discarded when loading is still in progress', async () => {
    const state = setup(); let finish;
    state.connectionStore.loadTableColumns = () => new Promise(resolve => finish = resolve);
    const doc = 'SELECT * FROM public.users';
    const editor = EditorState.create({doc, extensions: [sql({dialect: PostgreSQL})]});
    const pending = state.api.completeSQLSchema(new CompletionContext(editor, editor.doc.length, false));
    state.props.dbDriver = 'mysql';
    finish([]);
    assert.equal(await pending, null);
});
