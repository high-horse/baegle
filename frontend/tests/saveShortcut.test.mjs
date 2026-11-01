import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import ts from 'typescript';
const source = readFileSync(new URL('../src/layouts/WorkspaceLayout.vue', import.meta.url), 'utf8').split('<script setup lang="ts">')[1].split('</script>')[0];
const ast = ts.createSourceFile('workspace.ts', source, ts.ScriptTarget.Latest, true);
const handler = ast.statements.find(node => ts.isFunctionDeclaration(node) && node.name?.text === 'handleSaveConsoleShortcut');
const compiled = ts.transpileModule(handler.getText(ast), {compilerOptions: {target: ts.ScriptTarget.ES2022}}).outputText;
function press(type = 'result', overrides = {}, modal = false) {
    const calls = [];
    const handle = new Function('queryTabsStore', 'activeTabContent', 'document', compiled + ';return handleSaveConsoleShortcut;')(
        {activeTab: type ? {type, id: 'tab'} : null, saveConsole: id => calls.push(['console', id])},
        {value: {saveTableChanges: () => calls.push(['table'])}}, {querySelector: () => modal});
    handle({key: 's', ctrlKey: true, preventDefault: () => calls.push(['prevent']), stopPropagation() {}, ...overrides});
    return calls;
}
test('Ctrl+S and Cmd+S route table saving through the active grid', () => {
    assert.deepEqual(press(), [['prevent'], ['table']]);
    assert.deepEqual(press('result', {ctrlKey: false, metaKey: true}), [['prevent'], ['table']]);
});
test('query tabs retain console saving and other shortcuts are ignored', () => {
    assert.deepEqual(press('query'), [['prevent'], ['console', 'tab']]);
    for (const event of [{ctrlKey: false}, {shiftKey: true}, {altKey: true}, {isComposing: true}, {key: 'x'}]) assert.deepEqual(press('result', event), []);
    assert.deepEqual(press('result', {repeat: true}), [['prevent']]);
    assert.deepEqual(press('result', {}, true), [['prevent']]);
    assert.deepEqual(press(null), [['prevent']]);
});
