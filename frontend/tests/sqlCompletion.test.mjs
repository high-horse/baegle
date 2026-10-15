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
const { buildSQLCompletionSchema, completeSQLColumns, getSQLTableReferences, tableAlias, sqlTableAtPosition } = module.exports;
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

function aliasColumns(marked, driver = 'mysql') {
    const pos = marked.indexOf('|');
    const doc = marked.replace('|', '');
    const metadata = [
        {name:'users', schema:'ag', type:'TABLE'},
        {name:'events', schema:'ag', type:'TABLE'},
    ];
    const state = EditorState.create({doc, extensions:[sql({dialect:driver === 'mysql' ? MySQL : PostgreSQL})]});
    const context = new CompletionContext(state, pos, false);
    const references = getSQLTableReferences(state, pos);
    const result = completeSQLColumns(context, metadata, driver, table => [{name:table.name === 'users' ? 'user_id' : 'event_id', databaseType:'integer'}]);
    return {result, references, doc};
}

test('pasted and manually typed AS aliases resolve qualified and unqualified tables', () => {
    for (const table of ['users', 'ag.users', '`ag`.`users`']) {
        for (const alias of ['as t', 'AS t', 't', 'AS `t`']) {
            for (const marked of [`select t.| from ${table} ${alias}`, `select * from ${table} ${alias} where t.u|`]) {
                const {result, references} = aliasColumns(marked);
                assert.equal(references[0].alias, 't');
                assert.deepEqual(result?.options.map(option => option.label), ['user_id'], marked);
            }
        }
    }
});

test('aliases survive comments, line breaks, and joins in copied SQL', () => {
    const {result, references} = aliasColumns('SELECT * FROM ag.users /* copied */ AS\n t INNER JOIN ag.events /* join */ AS e ON e.event_id = t.user_id WHERE e.|');
    assert.deepEqual(references.map(ref => ref.alias), ['t','e']);
    assert.deepEqual(result?.options.map(option => option.label), ['event_id']);
});

test('column completion finds aliases beyond the initial parse of long pasted SQL', () => {
    const {result} = aliasColumns('SELECT t.|, ' + Array(2500).fill('1').join(', ') + ' FROM ag.users AS t');
    assert.deepEqual(result?.options.map(option => option.label), ['user_id']);
});

test('column replacement preserves qualifiers and replaces partial quoted names', () => {
    for (const marked of ['SELECT * FROM ag.users AS t WHERE t.`us|`', 'SELECT * FROM ag.users AS t WHERE t.us|']) {
        const {result, doc} = aliasColumns(marked);
        assert.ok(result);
        const after = doc.slice(0,result.from) + result.options[0].apply + doc.slice(result.to);
        assert.equal(after, 'SELECT * FROM ag.users AS t WHERE t.`user_id`');
    }
});

test('PostgreSQL quoted schema and typed alias support column completion', () => {
    const {result} = aliasColumns('SELECT t.| FROM "ag"."users" AS t', 'pgx');
    assert.deepEqual(result?.options.map(option => option.label), ['user_id']);
    assert.equal(result.options[0].apply, '"user_id"');
});

test('alias column completion stays scoped and excludes literals and comments', () => {
    for (const marked of [
        'SELECT * FROM ag.users AS t; SELECT t.|',
        "SELECT 't.|' FROM ag.users AS t",
        'SELECT * FROM ag.users AS t -- t.|',
    ]) assert.equal(aliasColumns(marked).result, null, marked);
});

const rankedPractices = `WITH ranked_practices AS (
    SELECT
        c.id as cus_id,
        c.\`name\` AS customer_name,
        ll.\`name\` AS from_language,
        ll2.\`name\` AS to_language,
        lds.\`name\` AS session_name,
        ldsg.\`title\` AS group_title,
        lds.\`rank\` AS session_rank,
        lsel.name,
        ldsp.*,
        ROW_NUMBER() OVER (
            PARTITION BY ldsp.\`customer_id\`, ldsp.\`syllabus_id\`
            ORDER BY lds.\`rank\` DESC, ldsp.\`created_at\` DESC, ldsp.\`id\` DESC
        ) AS rn
    FROM \`ag\`.\`lang_daily_session_practices\` AS ldsp
    INNER JOIN \`ag\`.\`lang_learning_syllabus\` AS lls
        ON lls.\`id\` = ldsp.\`syllabus_id\`
    INNER JOIN \`ag\`.\`lang_languages\` AS ll
        ON ll.\`id\` = lls.\`from_langauge_id\`
    INNER JOIN \`ag\`.\`lang_languages\` AS ll2
        ON ll2.\`id\` = lls.\`to_langauge_id\`
    INNER JOIN \`ag\`.\`lang_daily_sessions\` AS lds
        ON lds.\`id\` = ldsp.\`daily_session_id\`
    INNER JOIN \`ag\`.\`lang_daily_session_groups\` AS ldsg
        ON ldsg.\`id\` = lds.\`lang_daily_session_group_id\`
    INNER JOIN \`ag\`.\`lang_sub_experience_levels\` as lsel
        ON lsel.id = lds.sub_experience_level_id
    INNER JOIN \`ag\`.\`customers\` AS c
        ON c.\`id\` = ldsp.\`customer_id\`
    WHERE ldsp.\`created_at\` < '2026-10-16 00:00:00'
      AND ldsp.\`optional_type\` = 0 AND ldsp.deleted
)
SELECT *
FROM ranked_practices
WHERE rn = 1;`;

test('ranked_practices CTE loads the ldsp table and suggests deleted_at while editing', () => {
    const pos = rankedPractices.indexOf('ldsp.deleted') + 'ldsp.deleted'.length;
    const state = EditorState.create({doc:rankedPractices, extensions:[sql({dialect:MySQL})]});
    const refs = getSQLTableReferences(state, pos);
    assert.equal(refs.length, 8);
    assert.deepEqual(refs.find(ref => ref.alias === 'ldsp').path, ['ag','lang_daily_session_practices']);
    const metadata = [{name:'lang_daily_session_practices',schema:'ag',type:'TABLE'}];
    const loaded = [];
    const result = completeSQLColumns(new CompletionContext(state,pos,false), metadata, 'mysql', table => {
        loaded.push(table);
        return [{name:'deleted_at',databaseType:'datetime'}, {name:'customer_id',databaseType:'bigint'}];
    });
    assert.deepEqual(loaded, metadata);
    assert.ok(result.options.some(option => option.label === 'deleted_at'));
    const column = result.options.find(option => option.label === 'deleted_at');
    assert.equal(rankedPractices.slice(0,result.from) + column.apply + rankedPractices.slice(result.to),
        rankedPractices.replace('ldsp.deleted', 'ldsp.`deleted_at`'));
    // OVER/PARTITION parentheses belong to the CTE query scope as well.
    const windowPos = rankedPractices.indexOf('ldsp.`customer_id`') + 5;
    assert.deepEqual(getSQLTableReferences(state,windowPos).map(ref => ref.alias), refs.map(ref => ref.alias));
});

test('CTE aliases stay inside their own query block', () => {
    const first = aliasColumns('WITH a AS (SELECT * FROM ag.users AS t WHERE t.|), b AS (SELECT * FROM ag.events AS t) SELECT * FROM b');
    assert.deepEqual(first.result?.options.map(option => option.label), ['user_id']);
    const second = aliasColumns('WITH a AS (SELECT * FROM ag.users AS t), b AS (SELECT * FROM ag.events AS t WHERE t.|) SELECT * FROM b');
    assert.deepEqual(second.result?.options.map(option => option.label), ['event_id']);
    assert.equal(aliasColumns('WITH a AS (SELECT * FROM ag.users AS t) SELECT t.| FROM a').result, null);
});

test('subqueries resolve their own aliases and preserve enclosing aliases outside them', () => {
    assert.deepEqual(aliasColumns('SELECT * FROM ag.users AS t WHERE EXISTS (SELECT * FROM ag.events AS t WHERE t.|)').result?.options.map(option => option.label), ['event_id']);
    assert.deepEqual(aliasColumns('SELECT * FROM ag.users AS t WHERE EXISTS (SELECT * FROM ag.events AS t) AND t.|').result?.options.map(option => option.label), ['user_id']);
    assert.deepEqual(aliasColumns('SELECT * FROM (SELECT * FROM ag.users AS t WHERE (t.|)) AS q').result?.options.map(option => option.label), ['user_id']);
});
