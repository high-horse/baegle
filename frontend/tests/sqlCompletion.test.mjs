import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';
import { EditorState } from '@codemirror/state';
import * as completionModule from '@codemirror/autocomplete';
import * as languageModule from '@codemirror/language';
import { CompletionContext } from '@codemirror/autocomplete';
import { PostgreSQL, MySQL, SQLite, sql, schemaCompletionSource } from '@codemirror/lang-sql';

const compiled = ts.transpileModule(readFileSync(new URL('../src/utils/sqlCompletion.ts', import.meta.url), 'utf8'), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
}).outputText;
const module = { exports: {} };
new Function('module', 'exports', 'require', compiled)(module, module.exports, name => name === '@codemirror/language' ? languageModule : completionModule);
const { buildSQLCompletionSchema, getSQLTableReferences, tableAlias, sqlTableAtPosition } = module.exports;
function suggestions(doc, tables, driver = 'pgx', columns = () => []) {
    const dialect = driver === 'mysql' ? MySQL : driver === 'sqlite' ? SQLite : PostgreSQL;
    const config = { dialect, schema: buildSQLCompletionSchema(tables, driver, columns) };
    const state = EditorState.create({ doc, extensions: [sql(config)] });
    return schemaCompletionSource(config)(new CompletionContext(state, doc.length, false));
}
const tables = [
    { name: 'table_one', schema: 'public', type: 'TABLE' },
    { name: 'table_view', schema: 'public', type: 'VIEW' },
];

test('table navigation resolves quoted schema names and JOIN targets', () => {
    const doc = 'SELECT t.id FROM "public"."table_one" t JOIN table_view v ON v.id = t.id';
    const state = EditorState.create({ doc, extensions: [sql({ dialect: PostgreSQL })] });
    assert.equal(sqlTableAtPosition(state, doc.indexOf('table_one') + 2, tables), tables[0]);
    assert.equal(sqlTableAtPosition(state, doc.indexOf('table_view') + 2, tables), tables[1]);
    assert.equal(sqlTableAtPosition(state, doc.indexOf('t.id'), tables), undefined);
});

test('table navigation ignores literals, comments, unknown names, and other statements', () => {
    const doc = "SELECT 'table_one' FROM missing; SELECT * FROM table_view -- table_one";
    const state = EditorState.create({ doc, extensions: [sql({ dialect: PostgreSQL })] });
    assert.equal(sqlTableAtPosition(state, doc.indexOf('table_one') + 2, tables), undefined);
    assert.equal(sqlTableAtPosition(state, doc.indexOf('missing') + 2, tables), undefined);
    assert.equal(sqlTableAtPosition(state, doc.lastIndexOf('table_one') + 2, tables), undefined);
    assert.equal(sqlTableAtPosition(state, doc.indexOf('table_view') + 2, tables), tables[1]);
});

function applyTable(doc, label, metadata = tables, driver = 'pgx') {
    const result = suggestions(doc, metadata, driver);
    const option = result.options.find(item => item.label === label);
    let state = EditorState.create({doc, extensions:[sql({dialect:driver==='mysql'?MySQL:PostgreSQL})]});
    const view = {get state() {return state;}, dispatch(spec) {state=state.update(spec).state;}};
    option.apply(view,option,result.from,doc.length);
    return state.doc.toString();
}
test('FROM and JOIN selection inserts table initials', () => {
    assert.equal(applyTable('select * from tab','table_one'), 'select * from "public"."table_one" AS to2');
    assert.equal(applyTable('select * from table_one t join tab','table_view'), 'select * from table_one t join "public"."table_view" AS tv');
});
test('schema-qualified completion inserts only the table identifier and alias', () => {
    assert.equal(applyTable('select * from public.tab','table_one'),'select * from public."table_one" AS to2');
});
test('existing aliases are preserved and generated aliases do not collide', () => {
    const doc='select * from table_one to2 join tab';
    assert.equal(applyTable(doc,'table_one'), 'select * from table_one to2 join "public"."table_one" AS to3');
    const withAlias='select * from tab existing';
    const result=suggestions(withAlias.slice(0,17), tables);
    let state=EditorState.create({doc:withAlias,extensions:[sql({dialect:PostgreSQL})]});
    const view={get state(){return state;},dispatch(spec){state=state.update(spec).state;}};
    const option=result.options.find(item=>item.label==='table_one');
    option.apply(view,option,14,17);
    assert.equal(state.doc.toString(),'select * from "public"."table_one" existing');
    assert.equal(tableAlias('userAccounts',['ua','ua2']), 'ua3');
    assert.equal(tableAlias('123_table'), 't1t');
});
test('duplicate names and unusual identifiers are quoted safely', () => {
    const metadata=[{name:'users',schema:'public',type:'TABLE'},{name:'users',schema:'audit',type:'TABLE'},{name:'a"b',schema:'public',type:'TABLE'}];
    assert.equal(applyTable('select * from us','public.users',metadata),'select * from "public"."users" AS u');
    assert.equal(applyTable('select * from us','audit.users',metadata),'select * from "audit"."users" AS u');
    assert.equal(applyTable('select * from a','a"b',metadata),'select * from "public"."a""b" AS ab');
});
test('MySQL and SQLite quoting excludes MongoDB collections', () => {
    const metadata=[{name:'order items',schema:'',type:'TABLE'},{name:'documents',type:'COLLECTION'}];
    assert.equal(applyTable('select * from ord','order items',metadata,'mysql'),'select * from `order items` AS oi');
    assert.equal(applyTable('select * from ord','order items',metadata,'sqlite'),'select * from "order items" AS oi');
    assert.ok(!suggestions('select * from doc',metadata).options.some(item=>item.label==='documents'));
});
test('aliases suggest the correct columns in WHERE and JOIN', () => {
    const columns = table => table.name==='table_one' ? [{name:'id',databaseType:'integer'},{name:'first name',databaseType:'text'}] : [{name:'view_id',databaseType:'integer'}];
    for (const doc of ['select * from "public"."table_one" AS to2 WHERE to2.', 'select * from table_one t join table_view tv on t.i']) {
        const result=suggestions(doc,tables,'pgx',columns);
        assert.ok(result.options.some(item=>item.label==='id'));
        assert.equal(result.options.find(item=>item.label==='first name').apply, '"first name"');
        assert.ok(!result.options.some(item=>item.label==='view_id'));
    }
    assert.ok(suggestions('select * from table_one t join table_view tv on tv.',tables,'pgx',columns).options.some(item=>item.label==='view_id'));
});
test('references are scoped to the current statement and include manually typed aliases', () => {
    const doc='select * from table_one x; select * from public.table_view AS tv WHERE tv.';
    const state=EditorState.create({doc,extensions:[sql({dialect:PostgreSQL})]});
    assert.deepEqual(getSQLTableReferences(state,doc.length).map(ref=>({path:ref.path,alias:ref.alias})),[{path:['public','table_view'],alias:'tv'}]);
    assert.equal(suggestions(doc,tables,'pgx',()=>[{name:'id',databaseType:'integer'}]).options[0].label,'id');
});
test('INSERT table selection does not insert an inappropriate alias', () => {
    assert.equal(applyTable('insert into tab','table_one'), 'insert into "public"."table_one"');
});

test('unqualified aliases use public columns even when other schemas are listed first', () => {
    const metadata=[{name:'users',schema:'audit',type:'TABLE'},{name:'users',schema:'public',type:'TABLE'}];
    const result=suggestions('select * from users u where u.',metadata,'pgx',table=>[{name:table.schema==='public'?'user_name':'audit_id',databaseType:'text'}]);
    assert.ok(result.options.some(item=>item.label==='user_name'));
    assert.ok(!result.options.some(item=>item.label==='audit_id'));
});
