import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import ts from 'typescript';
import { format } from 'sql-formatter';
function load(file) {
  const source = readFileSync(new URL(file, import.meta.url), 'utf8');
  const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } }).outputText;
  const m = { exports: {} }; new Function('module','exports','require',compiled)(m,m.exports,createRequire(import.meta.url)); return m.exports;
}
const { enterpriseStatementAt: at } = load('../src/utils/enterpriseStatement.ts');
const { editorDialect, formatterDialect } = load('../src/utils/databaseDialect.ts');
const { relatedQuery } = load('../src/utils/relatedQuery.ts');
test('Oracle current statement preserves procedural bodies, q-quotes, and normal CREATE TABLE boundaries', () => {
  const text = "BEGIN\n NULL;\n NULL;\nEND;\n/\nCREATE TABLE t(type VARCHAR2(20));\nSELECT q'[a;b\'c]' FROM dual;";
  let range = at(text, text.indexOf('NULL'), 'oracle'); assert.equal(text.slice(range.from,range.to).trim(),'BEGIN\n NULL;\n NULL;\nEND;');
  range = at(text,text.indexOf('VARCHAR2'),'oracle'); assert.equal(text.slice(range.from,range.to).trim(),'CREATE TABLE t(type VARCHAR2(20));');
  range = at(text,text.indexOf('dual'),'oracle'); assert.equal(text.slice(range.from,range.to).trim(),"SELECT q'[a;b'c]' FROM dual;");
});
test('SQL Server runs the current GO batch without losing variable scope', () => {
  const text = "DECLARE @n int=1; SELECT @n;\nGO\nSELECT [a;b], 'x\nGO\ny';";
  const first = at(text,text.indexOf('@n;'),'sqlserver'); assert.equal(text.slice(first.from,first.to).trim(),'DECLARE @n int=1; SELECT @n;');
  const last = at(text,text.length,'sqlserver'); assert.equal(text.slice(last.from,last.to).trim(),"SELECT [a;b], 'x\nGO\ny';");
});
test('Db2 custom terminators retain compound statements', () => {
  const text = '--#SET TERMINATOR @\nBEGIN ATOMIC\n INSERT INTO t VALUES(1);\n INSERT INTO t VALUES(2);\nEND@\n--#SET TERMINATOR ;\nSELECT * FROM t;';
  const range = at(text,text.indexOf('VALUES(2)'),'db2'); assert.match(text.slice(range.from,range.to),/^\s*BEGIN ATOMIC[\s\S]*END@$/);
  const last = at(text,text.length,'db2'); assert.equal(text.slice(last.from,last.to).trim(),'SELECT * FROM t;');
});
test('new editor and formatter dialects accept vendor syntax', () => {
  for (const [driver,query] of [['oracle','SELECT NVL(name,\'x\') FROM dual'],['sqlserver','SELECT TOP 5 [name] FROM [dbo].[items]'],['db2','SELECT * FROM items FETCH FIRST 5 ROWS ONLY']]) {
    assert.ok(editorDialect(driver)); assert.ok(format(query,{language:formatterDialect(driver)}).includes('SELECT'));
  }
});
test('related-record predicates use portable strings and SQL Server brackets', () => {
  const relation = { target: { name:'items',schema:'APP',database:'db' }, columns:['id'],targetColumns:['key'] };
  assert.equal(relatedQuery(relation,{id:"O'Reilly"},'oracle'),`SELECT * FROM "APP"."items" WHERE "key" = 'O''Reilly';`);
  assert.equal(relatedQuery(relation,{id:"O'Reilly"},'db2'),`SELECT * FROM "APP"."items" WHERE "key" = 'O''Reilly';`);
  assert.equal(relatedQuery(relation,{id:"O'Reilly"},'sqlserver'),`SELECT * FROM [APP].[items] WHERE [key] = N'O''Reilly';`);
});
