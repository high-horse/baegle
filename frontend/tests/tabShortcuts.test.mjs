import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(new URL('../src/layouts/WorkspaceLayout.vue', import.meta.url), 'utf8').split('<script setup lang="ts">')[1].split('</script>')[0];
const ast = ts.createSourceFile('workspace.ts', source, ts.ScriptTarget.Latest, true);
const handler = ast.statements.find(node => ts.isFunctionDeclaration(node) && node.name?.text === 'handleSwitchTabShortcut');
const compiled = ts.transpileModule(handler.getText(ast), { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;

function setup(ids = ['query', 'table', 'other']) {
    const callbacks = [];
    const host = { focus() { document.activeElement = host; }, querySelector() { return editor; } };
    const editor = { focus() { document.activeElement = editor; } };
    const store = { tabs: ids.map(id => ({ id })), activeTabId: ids[0], selectTab(id) {
        assert.equal(document.activeElement, host, 'focus leaves outgoing view before it is detached');
        this.activeTabId = id;
    } };
    const document = { modal: false, querySelector(selector) {
        // A menu can exist without blocking workspace navigation.
        if (selector.includes('.q-menu')) return {};
        return this.modal ? {} : null;
    } };
    const handle = new Function('queryTabsStore', 'document', 'tabContentHost', 'nextTick', `${compiled}; return handleSwitchTabShortcut;`)(store, document, { value: host }, callback => callbacks.push(callback));
    return { store, document, host, editor, flush() { callbacks.splice(0).forEach(callback => callback()); }, press(overrides = {}) {
        const event = { key: 'Tab', code: 'Tab', ctrlKey: true, shiftKey: false, altKey: false, metaKey: false, isComposing: false,
            prevented: false, stopped: false, preventDefault() { this.prevented = true; }, stopPropagation() { this.stopped = true; }, ...overrides };
        handle(event);
        return event;
    } };
}

test('repeated presses follow tab order and wrap in both directions, even with a menu present', () => {
    const { store, press } = setup();
    for (const expected of ['table', 'other', 'query', 'table']) {
        const event = press();
        assert.equal(store.activeTabId, expected);
        assert.ok(event.prevented && event.stopped);
    }
    press({ shiftKey: true });
    assert.equal(store.activeTabId, 'query');
    press({ shiftKey: true });
    assert.equal(store.activeTabId, 'other');
    store.tabs.reverse();
    press();
    assert.equal(store.activeTabId, 'table');
});

test('keeps a stable focus target during switching and restores editor focus after render', () => {
    const { document, host, editor, press, flush } = setup();
    for (let index = 0; index < 10; index++) {
        press({ shiftKey: index % 2 === 0 });
        assert.equal(document.activeElement, host);
        flush();
        assert.equal(document.activeElement, editor);
    }
    press();
    const otherControl = {};
    document.activeElement = otherControl;
    flush();
    assert.equal(document.activeElement, otherControl, 'does not steal focus moved elsewhere');
});

test('accepts physical Tab and alternate desktop Shift+Tab values', () => {
    const { store, press } = setup();
    press({ key: 'Unidentified' });
    assert.equal(store.activeTabId, 'table');
    press({ key: 'ISO_Left_Tab', code: '', shiftKey: true });
    assert.equal(store.activeTabId, 'query');
});

test('leaves plain Tab, composition, other modifiers and modal dialogs alone', () => {
    const { store, document, press } = setup();
    for (const event of [{ ctrlKey: false }, { isComposing: true }, { altKey: true }, { metaKey: true }, { key: 'a', code: 'KeyA' }]) {
        assert.equal(press(event).prevented, false);
        assert.equal(store.activeTabId, 'query');
    }
    document.modal = true;
    assert.equal(press().prevented, false);
});

test('handles empty, single and missing active tabs safely', () => {
    assert.equal(setup([]).press().prevented, false);
    const single = setup(['only']);
    single.press();
    assert.equal(single.store.activeTabId, 'only');
    const { store, press } = setup();
    store.activeTabId = null;
    press({ shiftKey: true });
    assert.equal(store.activeTabId, 'other');
});
