import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createRequire} from 'node:module';
import test from 'node:test';
import ts from 'typescript';
import {parse, compileScript} from '@vue/compiler-sfc';
import {createRenderer} from 'vue';
const require = createRequire(import.meta.url);
function evaluate(source, overrides = {}) {
    const output = ts.transpileModule(source, {compilerOptions: {module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022}}).outputText;
    const module = {exports: {}};
    new Function('module', 'exports', 'require', output)(module, module.exports, name => overrides[name] ?? require(name));
    return module.exports;
}
const helpers = evaluate(readFileSync(new URL('../src/utils/gridConvenience.ts', import.meta.url), 'utf8'));
const {descriptor} = parse(readFileSync(new URL('../src/components/workspace/resultGrid/AdvancedCopy.vue', import.meta.url), 'utf8'));
const source = compileScript(descriptor, {id: 'copy-dialog-test'}).content;
const renderer = createRenderer({createComment: text => ({text}), createText: text => ({text}), createElement: tag => ({tag}), insert() {}, remove() {}, setText() {}, setElementText() {}, patchProp() {}, parentNode() {return null;}, nextSibling() {return null;}});
function mount(extra = {}) {
    const copied = [], events = [];
    const component = evaluate(source, {'@/utils/gridConvenience': helpers, '@/utils/copyWorkerClient': {formatCopySelection: async (...args) => helpers.advancedCopy(...args)}, quasar: {copyToClipboard: async text => copied.push(text), Notify: {create() {}}}}).default;
    component.render = () => null;
    const app = renderer.createApp(component, {modelValue: true, driver: 'pgx', selection: {columns: ['name'], rows: Array.from({length: 100}, (_, i) => [`row-${i}`])}, 'onUpdate:modelValue': value => events.push(value), ...extra});
    const vm = app.mount({});
    return {app, state: vm.$.setupState, copied, events};
}
test('Copy uses the entire selection despite a bounded preview and prevents duplicate submissions', async () => {
    const {app, state, copied, events} = mount();
    assert.equal(state.preview.truncated, true);
    assert.doesNotMatch(state.preview.text, /row-99/);
    await Promise.all([state.copy(), state.copy()]);
    assert.equal(copied.length, 1);
    assert.match(copied[0], /row-99$/);
    assert.deepEqual(events, [false]);
    app.unmount();
});
test('invalid SQL outside the preview keeps the dialog open and does not change the clipboard', async () => {
    const {app, state, copied, events} = mount({table: {name: 'items', schema: 'public', database: ''}, selection: {
        columns: ['name'], rows: Array.from({length: 100}, () => ['value']), keys: Array.from({length: 100}, (_, i) => i === 99 ? {} : {id: i}),
    }});
    state.format = 'update';
    assert.equal(state.preview.error, '');
    await state.copy();
    assert.match(state.copyError, /primary keys/);
    assert.deepEqual(copied, []);
    assert.deepEqual(events, []);
    assert.equal(state.copying, false);
    app.unmount();
});
test('a closed dialog does not format or render its retained selection', () => {
    let visited = 0;
    const row = [];
    Object.defineProperty(row, 0, {get() {visited++; return 'large data';}, enumerable: true});
    const {app, state} = mount({modelValue: false, selection: {columns: ['name'], rows: [row]}});
    assert.equal(state.preview.text, '');
    assert.equal(visited, 0);
    app.unmount();
});
