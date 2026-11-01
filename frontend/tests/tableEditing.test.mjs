import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import ts from 'typescript';
import { parse, compileScript } from '@vue/compiler-sfc';
import { createRenderer, nextTick } from 'vue';
import { createPinia, setActivePinia } from 'pinia';

const require = createRequire(import.meta.url);
let saveCalls = 0;
let saveResponse = { applied: 0, error: '' };
const backend = { DescribeTableEdit: async () => null, SaveTableChanges: async () => { saveCalls++; return saveResponse; } };
function evaluate(source, overrides = {}) {
    const output = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
    const module = { exports: {} };
    new Function('module', 'exports', 'require', output)(module, module.exports, name => overrides[name] ?? require(name));
    return module.exports;
}
const stores = evaluate(readFileSync(new URL('../src/stores/tableEditsStore.ts', import.meta.url), 'utf8'), { '@bindings/db-lens/internal/app': { DbService: backend } });
const { descriptor } = parse(readFileSync(new URL('../src/components/workspace/resultGrid/TableDataEditor.vue', import.meta.url), 'utf8'));
const componentSource = compileScript(descriptor, { id: 'table-editor-test' }).content;
const component = evaluate(componentSource, {
    '@/stores/tableEditsStore': stores,
    '@/stores/queryTabsStore': { useQueryTabsStore: () => ({ tabs: [{id: 'tab', cursor: 'cursor', sql: 'SELECT * FROM items'}], execute: async () => {} }) },
    quasar: { Notify: { create() {} } },
}).default;
component.render = () => null;
const renderer = createRenderer({
    createComment: text => ({ text }), createText: text => ({ text }), createElement: tag => ({ tag }),
    insert() {}, remove() {}, setText() {}, setElementText() {}, patchProp() {}, parentNode() { return null; }, nextSibling() { return null; },
});
function setup(driver = 'sqlite', extraProps = {}) {
    setActivePinia(createPinia());
    const edits = stores.useTableEditsStore();
    edits.states.tab = { info: { driver, table: { connectionId: 'conn', schema: 'main', database: 'db', name: 'items' }, canInsert: true, canModify: true, keys: driver === 'mongodb' ? ['_id'] : ['id'], reason: '', columns: [
        { name: 'id', databaseType: 'INTEGER', nullable: false, autoIncrement: true, generated: false },
        { name: 'name', databaseType: 'TEXT', nullable: false, autoIncrement: false, generated: false },
        { name: 'note', databaseType: 'TEXT', nullable: true, defaultValue: 'default', autoIncrement: false, generated: false },
        { name: 'generated', databaseType: 'INTEGER', generated: true },
    ] }, drafts: [], saving: false, loading: false, error: '' };
    const result = { Columns: [{ Name: 'id' }, { Name: 'name' }, { Name: 'note' }, { Name: 'generated' }], Rows: [[1, 'before', null, 2]], StartRow: 1, PageSize: 100 };
    const events = [];
    const app = renderer.createApp(component, { tabId: 'tab', result, selectedKey: '', onEditCell: key => events.push(['edit', key]), onAddRow: () => events.push(['add']), ...extraProps });
    const vm = app.mount({});
    const setup = vm.$.setupState;
    return { edits, result, vm, setup, app, events };
}

const setupEditor = setup;

test('SQL row actions request inline editing instead of opening a dialog', () => {
    const {edits, result, vm, setup, app, events} = setupEditor();
    const key = edits.rowKey('tab', result, 0);
    vm.openRow(key);
    vm.openRow();
    assert.equal(setup.dialog, false);
    assert.deepEqual(events, [['edit', key], ['add']]);
    app.unmount();
});

test('Mongo document staging preserves Extended JSON and absent keys', () => {
    const { edits, result, vm, setup, app } = setupEditor('mongodb');
    result.Documents = ['{"_id":{"$oid":"507f1f77bcf86cd799439011"},"large":{"$numberLong":"9007199254740993"}}'];
    const key = edits.rowKey('tab', result, 0);
    vm.openRow(key);
    const doc = JSON.parse(setup.documentText);
    assert.equal(Object.hasOwn(doc, '_id'), false);
    doc.name = 'edited';
    setup.documentText = JSON.stringify(doc);
    setup.stageRow();
    assert.deepEqual(edits.states.tab.drafts[0].values.large, { $numberLong: '9007199254740993' });
    assert.equal(Object.hasOwn(edits.states.tab.drafts[0].values, 'note'), false);
    assert.deepEqual(edits.states.tab.drafts[0].keys._id, JSON.parse(result.Documents[0])._id);
    vm.openRow(key);
    setup.documentText = '{"_id":"changed"}';
    setup.stageRow();
    assert.match(setup.dialogError, /_id is read-only/);
    assert.equal(edits.states.tab.drafts.length, 1);
    app.unmount();
});

test('a partially saved batch keeps only failed/unattempted drafts for retry', async () => {
    const { edits, app } = setupEditor('mongodb');
    edits.stage('tab', { key: 'one', operation: 'insert', keys: {}, values: { name: 'one' } });
    edits.stage('tab', { key: 'two', operation: 'insert', keys: {}, values: { name: 'two' } });
    saveResponse = { applied: 1, error: 'second insert failed' };
    assert.equal(await edits.save('tab'), true);
    assert.deepEqual(edits.states.tab.drafts.map(row => row.key), ['two']);
    assert.equal(edits.states.tab.error, 'second insert failed');
    saveResponse = { applied: 1, error: '' };
    assert.equal(await edits.save('tab'), true);
    assert.equal(edits.hasPending('tab'), false);
    app.unmount();
});

test('an active inline edit participates in unsaved-change guards and discard clears it', () => {
    const {edits, app} = setupEditor();
    assert.equal(edits.hasPending('tab'), false);
    edits.states.tab.editing = true;
    assert.equal(edits.hasPending('tab'), true);
    assert.equal(edits.anyPending, true);
    edits.discard('tab');
    assert.equal(edits.hasPending('tab'), false);
    assert.equal(edits.anyPending, false);
    app.unmount();
});

test('Save validates the active cell before sending any staged rows to the backend', async () => {
    let allowed = false;
    const {edits, setup, app} = setupEditor('sqlite', {prepareSave: () => allowed});
    edits.stage('tab', {key: 'new', operation: 'insert', keys: {}, values: {name: 'new'}});
    const before = saveCalls;
    await setup.save();
    assert.equal(saveCalls, before);
    assert.equal(edits.states.tab.drafts.length, 1);
    allowed = true;
    saveResponse = {applied: 1, error: ''};
    await setup.save();
    assert.equal(saveCalls, before + 1);
    assert.equal(edits.states.tab.drafts.length, 0);
    app.unmount();
});
