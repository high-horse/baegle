import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import ts from 'typescript';
import {computed, ref, watch} from 'vue';
const source = readFileSync(new URL('../src/components/welcome/WelcomeIndex.vue', import.meta.url), 'utf8').split('<script setup lang="ts">')[1].split('</script>')[0];
const ast = ts.createSourceFile('welcome.ts', source, ts.ScriptTarget.Latest, true);
const names = ['testEditedConnection', 'validateConnectionDetails', 'parseToConfig', 'parseSshConfig', 'saveConnection', 'cancelEdit'];
const functions = ast.statements.filter(node => ts.isFunctionDeclaration(node) && names.includes(node.name?.text)).map(node => node.getText(ast)).join('\n');
const invalidation = ast.statements.find(node => ts.isExpressionStatement(node) && node.getText(ast).startsWith('watch([form, sshForm, editingId]')).getText(ast);
const compiled = ts.transpileModule(`let connectionTestVersion = 0;\n${functions}\nconst stop = ${invalidation}\nreturn {testEditedConnection,saveConnection,stop};`, {compilerOptions: {target: ts.ScriptTarget.ES2022}}).outputText;
function setup({valid = true, response = true} = {}) {
    const requests = [], updates = [], routes = [];
    const globals = {
        editingId: ref('42'), saving: ref(false), testingConnection: ref(false), loadingSavedConnection: ref(false), connectionTestResult: ref(null),
        form: ref({type: 'pgx', host: 'edited-host', port: 5440, user: 'edited-user', password: 'edited-password', database: 'edited-db', connection_name: 'Edited', ssl: true, readonly: true, use_ssh: false, color: '', group: 'Work'}),
        sshForm: ref({name: 'Tunnel', host: 'ssh-host', port: 22, username: 'ssh-user', auth_method: 'password', private_key: '', passphrase: '', password: 'ssh-password'}),
        sshDialog: ref(false), formRef: ref({validate: async () => valid}), isMongoURI: computed(() => false),
        validPort: port => Number.isInteger(Number(port)) && port > 0 && port <= 65535,
        watch,
        DbService: {async PingConfig(config) {requests.push(config); return typeof response === 'function' ? response() : response;}},
        DatabaseService: {async UpdateConnection(config) {updates.push(config);}},
        store: {async getConnections() {}}, Notify: {create() {}}, $router: {async push(route) {routes.push(route);}},
    };
    const api = new Function(...Object.keys(globals), compiled)(...Object.values(globals));
    return {...globals, api, requests, updates, routes};
}
test('edited connection test sends unsaved settings without saving or navigating', async () => {
    const state = setup();
    await state.api.testEditedConnection();
    assert.equal(state.requests.length, 1);
    assert.equal(state.requests[0].ID, '42');
    assert.equal(state.requests[0].Host, 'edited-host');
    assert.equal(state.requests[0].Password, 'edited-password');
    assert.equal(state.requests[0].Port, 5440);
    assert.equal(state.requests[0].SSL, true);
    assert.equal(state.requests[0].ReadOnly, true);
    assert.equal(state.connectionTestResult.value.ok, true);
    assert.equal(state.testingConnection.value, false);
    assert.deepEqual(state.updates, []);
    assert.deepEqual(state.routes, []);
    state.form.value.host = 'changed-again';
    assert.equal(state.connectionTestResult.value, null);
    state.api.stop();
});
test('tests include unsaved SSH settings and reject incomplete tunnel configuration', async () => {
    const state = setup();
    state.form.value.use_ssh = true;
    state.sshForm.value.host = '';
    await state.api.testEditedConnection();
    assert.equal(state.sshDialog.value, true);
    assert.deepEqual(state.requests, []);
    state.sshForm.value.host = 'new-ssh-host';
    await state.api.testEditedConnection();
    assert.equal(state.requests[0].SSHConfig.Host, 'new-ssh-host');
    assert.equal(state.requests[0].SSHConfig.Password, 'ssh-password');
    state.api.stop();
});
test('invalid forms and in-progress saves prevent connection testing', async () => {
    const invalid = setup({valid: false});
    await invalid.api.testEditedConnection();
    assert.deepEqual(invalid.requests, []);
    invalid.api.stop();
    const saving = setup(); saving.saving.value = true;
    await saving.api.testEditedConnection();
    assert.deepEqual(saving.requests, []);
    saving.api.stop();
});
test('failed tests show the failure and leave saving as a separate action', async () => {
    const state = setup({response: () => {throw new Error('Authentication failed');}});
    await state.api.testEditedConnection();
    assert.deepEqual(state.connectionTestResult.value, {ok: false, message: 'Authentication failed'});
    assert.equal(state.testingConnection.value, false);
    assert.deepEqual(state.updates, []);
    await state.api.saveConnection();
    assert.equal(state.updates.length, 1);
    state.api.stop();
});
test('editing fields during a test prevents a stale success; duplicate tests and Save are blocked', async () => {
    let resolve;
    const response = new Promise(done => resolve = done);
    const state = setup({response: () => response});
    const pending = state.api.testEditedConnection();
    await new Promise(done => setImmediate(done));
    await state.api.testEditedConnection();
    await state.api.saveConnection();
    assert.equal(state.requests.length, 1);
    assert.deepEqual(state.updates, []);
    state.form.value.database = 'another-db';
    resolve(true);
    await pending;
    assert.equal(state.connectionTestResult.value, null);
    assert.equal(state.testingConnection.value, false);
    state.api.stop();
});
