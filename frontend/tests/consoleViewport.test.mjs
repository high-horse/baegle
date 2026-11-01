import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import ts from 'typescript';
const output = ts.transpileModule(readFileSync(new URL('../src/utils/consoleViewport.ts', import.meta.url), 'utf8'), {compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS}}).outputText;
const module = {exports: {}};
new Function('exports', output)(module.exports);
const {trackConsoleViewport} = module.exports;
function editor() {
    const events = new EventTarget();
    const measures = [];
    const view = {dom: {isConnected: true}, scrollDOM: Object.assign(events, {scrollTop: 0, scrollLeft: 0}),
        state: {doc: {length: 10000}, selection: {main: {anchor: 0, head: 0}}},
        dispatch({selection}) {this.state.selection.main = selection;}, requestMeasure(request) {measures.push(request);}};
    return {view, measure() {for (const item of measures.splice(0)) item.write(item.read());}, scroll(top, left = 0) {view.scrollDOM.scrollTop = top; view.scrollDOM.scrollLeft = left; events.dispatchEvent(new Event('scroll'));}};
}
test('tab activation restores the viewport and cursor despite hidden scroll resets', () => {
    const {view, scroll, measure} = editor();
    let saved;
    const tracker = trackConsoleViewport(view, undefined, value => saved = value);
    view.state.selection.main = {anchor: 700, head: 720};
    scroll(2500, 80);
    tracker.deactivate();
    scroll(0);
    assert.equal(saved.top, 2500);
    tracker.restore();
    measure();
    assert.equal(view.scrollDOM.scrollTop, 2500);
    assert.equal(view.scrollDOM.scrollLeft, 80);
    assert.deepEqual(view.state.selection.main, {anchor: 700, head: 720});
    tracker.destroy();
});
test('recreated editors restore their own session snapshot after cache eviction', () => {
    const first = editor(), second = editor();
    let saved;
    const tracker = trackConsoleViewport(first.view, undefined, value => saved = value);
    first.view.state.selection.main = {anchor: 900, head: 900};
    first.scroll(4200);
    tracker.destroy();
    const restored = trackConsoleViewport(second.view, saved, () => {});
    restored.restore();
    second.measure();
    assert.equal(second.view.scrollDOM.scrollTop, 4200);
    assert.equal(second.view.state.selection.main.head, 900);
    restored.destroy();
});
test('detached editors cannot overwrite a saved viewport; new sessions start fresh', () => {
    const {view, scroll, measure} = editor();
    let saved;
    const tracker = trackConsoleViewport(view, undefined, value => saved = value);
    scroll(1800);
    view.dom.isConnected = false;
    scroll(0);
    assert.equal(saved.top, 1800);
    tracker.destroy();
    const fresh = editor();
    const session = trackConsoleViewport(fresh.view, undefined, () => {});
    session.restore();
    fresh.measure();
    assert.equal(fresh.view.scrollDOM.scrollTop, 0);
    session.destroy();
});
