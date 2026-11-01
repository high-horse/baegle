import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';
function load(path, overrides = {}) {
    const compiled = ts.transpileModule(readFileSync(new URL(path, import.meta.url), 'utf8'), {
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
    }).outputText;
    const module = {exports: {}};
    new Function('module', 'exports', 'require', compiled)(module, module.exports, name => overrides[name]);
    return module.exports;
}
const clipboard = load('../src/utils/resultClipboard.ts');
const {inlineColumnEditable, stageInlineValue} = load('../src/utils/inlineTableEdit.ts', {'./resultClipboard': clipboard});
const name = {name: 'name', databaseType: 'TEXT', nullable: true};
const target = {key: 'one', keys: {id: 1}, original: {id: 1, name: 'before', note: null}};
test('inline edits coalesce and reverting a cell removes just that column change', () => {
    let draft = stageInlineValue(name, ['id'], target, 'value', 'after');
    assert.deepEqual(draft.values, {name: 'after'});
    draft = stageInlineValue({...name, name: 'note'}, ['id'], {...target, draft}, 'value', '');
    assert.deepEqual(draft.values, {name: 'after', note: ''});
    draft = stageInlineValue(name, ['id'], {...target, draft}, 'value', 'before');
    assert.deepEqual(draft.values, {note: ''});
    draft = stageInlineValue({...name, name: 'note'}, ['id'], {...target, draft}, 'null', '');
    assert.deepEqual(draft.values, {});
    assert.deepEqual(target.original, {id: 1, name: 'before', note: null});
});
test('new rows distinguish default, empty string, and NULL while retaining other cells', () => {
    const insert = {...target, original: {}, keys: {}, draft: {key: 'new', operation: 'insert', keys: {}, values: {note: 'keep'}}};
    let draft = stageInlineValue(name, ['id'], insert, 'value', '');
    assert.deepEqual(draft.values, {note: 'keep', name: ''});
    draft = stageInlineValue(name, ['id'], {...insert, draft}, 'null', '');
    assert.deepEqual(draft.values, {note: 'keep', name: null});
    draft = stageInlineValue(name, ['id'], {...insert, draft}, 'default', '');
    assert.deepEqual(draft.values, {note: 'keep'});
    assert.equal(draft.operation, 'insert');
    assert.throws(() => stageInlineValue(name, ['id'], target, 'default', ''), /new rows/);
});
test('locked columns, deleted rows and imprecise keys reject inline writes', () => {
    for (const column of [{...name, name: 'id'}, {...name, primaryKey: true}, {...name, generated: true}]) {
        assert.equal(inlineColumnEditable(column, ['id'], false), false);
        assert.throws(() => stageInlineValue(column, ['id'], target, 'value', 'bad'), /read-only/);
    }
    assert.equal(inlineColumnEditable({...name, autoIncrement: true}, [], true), false);
    assert.throws(() => stageInlineValue(name, [], {...target, keys: {id: Number.MAX_SAFE_INTEGER + 1}}, 'value', 'a'), /precisely/);
    assert.throws(() => stageInlineValue(name, [], {...target, draft: {operation: 'delete', values: {}}}, 'value', 'a'), /deletion/);
});
test('numeric, boolean, nullable validation retains large integer precision', () => {
    assert.equal(stageInlineValue({...name, databaseType: 'BIGINT'}, [], target, 'value', '9007199254740993').values.name, '9007199254740993');
    assert.equal(stageInlineValue({...name, databaseType: 'BOOL'}, [], target, 'value', 'true').values.name, true);
    assert.throws(() => stageInlineValue({...name, databaseType: 'INTEGER'}, [], target, 'value', 'bad'), /number/);
    assert.throws(() => stageInlineValue({...name, databaseType: 'BOOL'}, [], target, 'value', 'yes'), /true/);
    assert.throws(() => stageInlineValue({...name, nullable: false}, [], target, 'null', ''), /NULL/);
});

const gridSource = readFileSync(new URL('../src/components/workspace/resultGrid/ResultTab.vue', import.meta.url), 'utf8').split('<script setup lang="ts">')[1].split('</script>')[0];
const gridAst = ts.createSourceFile('grid.ts', gridSource, ts.ScriptTarget.Latest, true);
const functions = ['inlineTarget', 'inlineMetadata', 'canEditInline', 'focusInline', 'editInlineCell', 'commitInline', 'cancelInline', 'editInlineRow', 'addInlineRow', 'inlineKeydown', 'handleResultKeydown', 'refreshResult'];
const gridFunctions = ts.transpileModule(gridAst.statements.filter(node => ts.isFunctionDeclaration(node) && functions.includes(node.name?.text)).map(node => node.getText(gridAst)).join('\n'), {compilerOptions: {target: ts.ScriptTarget.ES2022}}).outputText;
function grid() {
    const state = {info: {driver: 'sqlite', keys: ['id'], canInsert: true, canModify: true, columns: [
        {name: 'id', databaseType: 'INTEGER', primaryKey: true, autoIncrement: true},
        {name: 'name', databaseType: 'TEXT', nullable: true},
        {name: 'age', databaseType: 'INTEGER', nullable: false},
    ]}, drafts: []};
    const columns = [{Name: 'id'}, {Name: 'name'}, {Name: 'age'}];
    const props = {tableTabId: 'tab', result: {Columns: columns, Rows: [[1, 'original', 20]]}};
    const edits = {
        keysFor: () => ({id: 1}),
        stage(_id, draft) {state.drafts.push(draft);},
        stageMany(_id, drafts) {
            const keys = new Set(drafts.map(draft => draft.key));
            state.drafts = [...state.drafts.filter(draft => !keys.has(draft.key)), ...drafts.filter(draft => draft.operation !== 'update' || Object.keys(draft.values).length)];
        },
    };
    const inline = {value: null};
    const active = {value: null};
    const selected = {value: ''};
    const events = [];
    const globals = {
        emit: (...args) => events.push(args),
        props, editState: {value: state}, displayedColumns: {value: columns}, sourceRowIndexes: {value: new Map([['one', 0]])},
        inlineEdit: inline, activeCell: active, selectedRowKey: selected, tableEdits: edits,
        mappedRows: {get value() { return [{id: 'one'}, ...state.drafts.filter(draft => draft.operation === 'insert').map(draft => ({id: draft.key}))]; }},
        dataTable: {value: null}, rowEditor: {value: null},
        selectCell(key, column) {active.value = {rowKey: key, column: Number(column.slice(7))};},
        scrollToRow() {}, nextTick: async () => {}, inlineColumnEditable, stageInlineValue,
        inlineValueText: value => value == null ? '' : String(value), HTMLSelectElement: class {}, handleGridKeydown() {},
        async saveChanges() { if (api.commitInline()) state.saved = true; },
    };
    const api = new Function(...Object.keys(globals), gridFunctions + ';return {editInlineCell,commitInline,cancelInline,addInlineRow,inlineKeydown,handleResultKeydown,refreshResult};')(...Object.values(globals));
    return {api, inline, state, props, active, events};
}
test('grid editing stages on commit, skips protected cells with Tab, and Escape preserves prior drafts', async () => {
    const {api, inline, state, props} = grid();
    await api.editInlineCell('one', 'column-0');
    assert.equal(inline.value, null);
    await api.editInlineCell('one', 'column-1');
    assert.equal(state.editing, true);
    inline.value.text = 'changed';
    await api.inlineKeydown({key: 'Tab', preventDefault() {}});
    assert.deepEqual(state.drafts[0].values, {name: 'changed'});
    assert.equal(inline.value.column, 'column-2');
    inline.value.text = '25';
    await api.inlineKeydown({key: 'Escape', preventDefault() {}});
    assert.equal(inline.value, null);
    assert.equal(state.editing, false);
    assert.deepEqual(state.drafts[0].values, {name: 'changed'});
    assert.equal(props.result.Rows[0][1], 'original');
});
test('invalid values keep the editor open and prevent Save from proceeding', async () => {
    const {api, inline, state} = grid();
    await api.editInlineCell('one', 'column-2');
    inline.value.text = 'invalid';
    assert.equal(api.commitInline(), false);
    assert.match(inline.value.error, /number/);
    assert.equal(state.editing, true);
    assert.deepEqual(state.drafts, []);
    inline.value.text = '25';
    assert.equal(api.commitInline(), true);
    assert.equal(state.drafts[0].values.age, '25');
    assert.equal(state.editing, false);
});
test('Add row creates one inline draft and untouched Default fields remain omitted', async () => {
    const {api, inline, state} = grid();
    await api.addInlineRow();
    assert.equal(state.drafts.length, 1);
    assert.equal(state.drafts[0].operation, 'insert');
    assert.equal(inline.value.column, 'column-1');
    assert.equal(inline.value.mode, 'default');
    assert.equal(api.commitInline(), true);
    assert.deepEqual(state.drafts[0].values, {});
    await api.editInlineCell(state.drafts[0].key, 'column-1');
    inline.value.mode = 'value';
    inline.value.text = '';
    assert.equal(api.commitInline(), true);
    assert.deepEqual(state.drafts[0].values, {name: ''});
});


test('typing on a selected new cell replaces DEFAULT and stages the insert value', async () => {
    const {api, inline, state, active} = grid();
    await api.addInlineRow();
    const key = state.drafts[0].key;
    api.cancelInline();
    active.value = {rowKey: key, column: 1};
    const root = {};
    let prevented = false;
    api.handleResultKeydown({key: 'A', target: root, currentTarget: root, preventDefault() {prevented = true;}, stopPropagation() {}});
    assert.equal(prevented, true);
    assert.equal(inline.value.mode, 'value');
    assert.equal(inline.value.text, 'A');
    assert.equal(api.commitInline(), true);
    assert.deepEqual(state.drafts[0].values, {name: 'A'});
});

test('new row values survive Tab between cells and generated keys remain omitted', async () => {
    const {api, inline, state} = grid();
    await api.addInlineRow();
    inline.value.mode = 'value';
    inline.value.text = 'New person';
    await api.inlineKeydown({key: 'Tab', preventDefault() {}});
    assert.equal(inline.value.column, 'column-2');
    inline.value.mode = 'value';
    inline.value.text = '42';
    assert.equal(api.commitInline(), true);
    assert.deepEqual(state.drafts[0].values, {name: 'New person', age: '42'});
    assert.equal(Object.hasOwn(state.drafts[0].values, 'id'), false);
});


test('Ctrl+S saves the value still being typed in a new row', async () => {
    const {api, inline, state} = grid();
    await api.addInlineRow();
    inline.value.mode = 'value';
    inline.value.text = 'Ready to insert';
    let prevented = false;
    await api.inlineKeydown({key: 's', ctrlKey: true, preventDefault() {prevented = true;}});
    assert.equal(prevented, true);
    assert.equal(state.saved, true);
    assert.deepEqual(state.drafts[0].values, {name: 'Ready to insert'});
    assert.equal(inline.value, null);
});


test('refresh cancels an invalid active cell before emitting instead of committing it', async () => {
    const {api, inline, state, events} = grid();
    await api.editInlineCell('one', 'column-2');
    inline.value.text = 'invalid number';
    assert.equal(api.commitInline(), false);
    api.refreshResult();
    assert.equal(inline.value, null);
    assert.equal(state.editing, false);
    assert.deepEqual(state.drafts, []);
    assert.deepEqual(events, [['refresh']]);
});
