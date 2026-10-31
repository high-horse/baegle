<template>
    <q-card flat bordered class="connection-card connection-setup">
        <div class="connection-setup-grid">
        <aside class="database-panel" aria-label="Choose a database">
            <div class="panel-eyebrow">01 / DATABASE</div>
            <h2>Choose your engine</h2>
            <p class="panel-description">Start with the database you want to explore.</p>
        <q-tabs v-model="databaseGroup" dense active-color="primary" indicator-color="primary" align="justify" no-caps class="database-category-tabs" aria-label="Database category">
            <q-tab name="sql" label="SQL" />
            <q-tab name="nosql" label="NoSQL" />
        </q-tabs>
        <div class="database-options" role="group" :aria-label="databaseGroup === 'sql' ? 'SQL databases' : 'NoSQL databases'">
            <q-btn v-for="database in availableDatabases" :key="database.value" flat no-caps
                :aria-label="database.label" :aria-pressed="form.type === database.value"
                :color="form.type === database.value ? 'primary' : 'grey-5'"
                :class="{ 'database-option-selected': form.type === database.value }"
                class="database-option" :disable="saving || loadingSavedConnection"
                @click="form.type = database.value">
                <span class="database-option-content">
                    <span class="database-option-icon" aria-hidden="true">{{ database.emoji }}</span>
                    <span class="database-option-name">{{ database.label }}</span>
                    <q-icon v-if="form.type === database.value" name="check_circle" size="14px" class="database-selected-check" />
                </span>
            </q-btn>
        </div>
            <div class="database-panel-note"><q-icon name="info_outline" size="16px" /><span>Switch engines anytime before connecting.</span></div>
        </aside>
        <div class="connection-details-panel">
            <div class="connection-details-heading">
                <div class="connection-heading-icon"><q-icon :name="editingId ? 'edit' : 'add_link'" size="22px" /></div>
                <div class="connection-heading-copy">
                    <div class="panel-eyebrow">02 / CONNECTION</div>
                    <h2>{{ editingId ? 'Edit connection' : 'Connection details' }}</h2>
                    <p class="panel-description">{{ editingId ? 'Update your saved connection settings.' : 'Enter the details for your database.' }}</p>
                </div>
                <q-btn v-if="editingId" flat round dense icon="close" aria-label="Cancel editing" color="grey-5" @click="cancelEdit" />
            </div>
        <q-form ref="formRef" @submit="editingId ? saveConnection() : connect()">
            <q-card-section class="connection-form-body">
                <div class="form-section-title"><q-icon name="badge" size="16px" /><h3>Connection identity</h3></div>
                <div class="row q-col-gutter-md">
                    <div class="col-12 col-sm-7">
                        <q-input v-model="form.connection_name" label="Connection name" :rules="[required]" outlined dense color="primary" hide-bottom-space />
                    </div>
                    <div class="col-12 col-sm-5">
                        <ConnectionGroupSelect compact v-model="form.group" :groups="store.connectionGroups" :disable="saving || loadingSavedConnection" />
                    </div>
                    <div class="col-12"><div class="form-section-title details-section-title"><q-icon :name="form.type === 'sqlite' ? 'folder_open' : 'dns'" size="16px" /><h3>{{ form.type === 'sqlite' ? 'Database file' : 'Server & credentials' }}</h3></div></div>
                    <template v-if="form.type !== 'sqlite'">
                        <div class="col-12 col-sm-8">
                            <q-input v-model="form.host" :label="form.type === 'mongodb' ? 'Host or MongoDB URI' : 'Database host'" :rules="[required]" outlined dense color="primary" hide-bottom-space />
                        </div>
                        <div class="col-12 col-sm-4">
                            <q-input v-model.number="form.port" label="Port" type="number" :disable="isMongoURI" :rules="isMongoURI ? [] : [validPort]" outlined dense color="primary" hide-bottom-space />
                        </div>
                        <div v-if="form.use_ssh || form.type === 'mongodb'" class="col-12 text-caption text-grey-5">
                            {{ form.use_ssh ? 'Use the database host reachable from your SSH server.' : 'Enter a host or mongodb:// / mongodb+srv:// URI.' }}
                        </div>
                        <div class="col-12 col-sm-6">
                            <q-input v-model="form.user" :label="isNoSQL(form.type) ? 'User (optional)' : 'User'" :disable="isMongoURI" :rules="isNoSQL(form.type) ? [] : [required]" outlined dense color="primary" hide-bottom-space />
                        </div>
                        <div class="col-12 col-sm-6">
                            <q-input v-model="form.password" label="Password" type="password" :disable="isMongoURI" outlined dense color="primary" hide-bottom-space />
                        </div>
                        <div class="col-12">
                            <q-input v-model="form.database" :label="form.type === 'oracle' ? 'Service name' : form.type === 'cassandra' ? 'Keyspace' : form.type === 'redis' ? 'Database index' : 'Database'" :rules="form.type === 'redis' ? [validRedisDatabase] : [required]" outlined dense color="primary" hide-bottom-space />
                        </div>
                        <div v-if="form.type === 'oracle'" class="col-12 text-caption text-grey-5">Enter the Oracle service name, for example FREEPDB1. Connects using a database username and password.</div>
                        <div v-if="form.type === 'sqlserver'" class="col-12 text-caption text-grey-5">Use SQL Server authentication and the instance’s TCP port (usually 1433).</div>
                        <div v-if="form.type === 'db2'" class="col-12 text-caption text-grey-5">Db2 LUW requires a Db2-enabled app build and the IBM CLI client libraries.</div>
                        <div v-if="form.type === 'cassandra'" class="col-12 text-caption text-grey-5">Connects to this Cassandra node. Enter an existing keyspace and run CQL statements.</div>
                        <div v-if="form.type === 'couchdb'" class="col-12 text-caption text-grey-5">Enter the host without http:// and an existing database name. Enable TLS for HTTPS.</div>
                        <div v-if="form.type === 'redis'" class="col-12 text-caption text-grey-5">Standalone Redis. Database 0 is the default. Leave User empty for password-only authentication.</div>
                    </template>
                    <div v-else class="col-12">
                        <q-input v-model="form.database" label="Database file" hint="/path/to/database.db" :rules="[required]" outlined dense color="primary">
                            <template #append><q-icon name="folder_open" /></template>
                        </q-input>
                    </div>
                </div>
                <div class="form-section-title options-section-title"><q-icon name="tune" size="16px" /><h3>Connection options</h3></div>
                <div class="connection-preferences">
                    <q-checkbox v-if="!editingId" v-model="form.save_connection" label="Save connection" dense color="primary" />
                    <q-checkbox v-model="form.readonly" label="Read only" dense color="primary" />
                    <q-checkbox v-if="['oracle', 'sqlserver', 'db2'].includes(form.type) || isNoSQL(form.type)" v-model="form.ssl" label="Require TLS encryption" dense color="primary" />
                </div>
                <q-list v-if="form.type !== 'sqlite'" bordered class="connection-options q-mt-md">
                    <q-item>
                        <q-item-section avatar><q-icon name="vpn_key" color="primary" /></q-item-section>
                        <q-item-section>
                            <q-item-label>SSH tunnel</q-item-label>
                            <q-item-label caption>{{ form.use_ssh && sshForm.host ? `${sshForm.username}@${sshForm.host}:${sshForm.port}` : 'Connect through an SSH server' }}</q-item-label>
                        </q-item-section>
                        <q-item-section side><q-toggle v-model="form.use_ssh" :disable="isMongoURI" color="primary" aria-label="Connect through SSH tunnel" /></q-item-section>
                    </q-item>
                    <q-item v-if="form.use_ssh" dense>
                        <q-item-section><q-btn flat no-caps color="primary" label="Configure SSH" icon="settings" @click="sshDialog = true" /></q-item-section>
                    </q-item>
                </q-list>
            </q-card-section>
            <q-separator dark />
            <q-card-actions align="right" class="connection-form-actions">
                <span class="connection-test-note"><q-icon name="network_check" size="16px" />{{ editingId ? 'Changes apply the next time you connect.' : 'Test your connection before opening the workspace.' }}</span>
                <q-btn v-if="editingId" flat no-caps label="Cancel" color="grey-5" :disable="saving" @click="cancelEdit" />
                <q-btn unelevated no-caps color="primary" text-color="black" :label="editingId ? 'Save changes' : 'Test connection'" :icon="editingId ? 'save' : 'network_check'" type="submit" :loading="saving" :disable="loadingSavedConnection" />
            </q-card-actions>
        </q-form>
        </div>
        </div>
    </q-card>

    <q-dialog v-model="sshDialog" persistent>
        <q-card flat bordered class="connection-card ssh-card">
            <q-card-section class="row items-center no-wrap q-pa-lg">
                <q-avatar rounded color="dark" text-color="primary" icon="vpn_key" size="44px" />
                <div class="col q-ml-md">
                    <div class="text-h6">SSH configuration</div>
                    <div class="text-caption text-grey-5">Set up the tunnel for this database connection.</div>
                </div>
                <q-btn flat round dense icon="close" color="grey-5" aria-label="Close SSH configuration" :disable="testingSsh" @click="cancelSsh" />
            </q-card-section>
            <q-separator dark />
            <q-form ref="sshFormRef" @submit="saveSshConfig">
                <q-card-section class="q-pa-lg">
                    <div class="row q-col-gutter-md">
                        <div class="col-12"><q-input v-model="sshForm.name" label="Configuration name" :rules="[required]" outlined dense color="primary" hide-bottom-space :disable="testingSsh" /></div>
                        <div class="col-12 col-sm-8"><q-input v-model="sshForm.host" label="SSH host" :rules="[required]" outlined dense color="primary" hide-bottom-space :disable="testingSsh" /></div>
                        <div class="col-12 col-sm-4"><q-input v-model.number="sshForm.port" label="Port" type="number" :rules="[validPort]" outlined dense color="primary" hide-bottom-space :disable="testingSsh" /></div>
                        <div class="col-12"><q-input v-model="sshForm.username" label="Username" :rules="[required]" outlined dense color="primary" hide-bottom-space :disable="testingSsh" /></div>
                        <div class="col-12">
                            <q-select v-model="sshForm.auth_method" label="Authentication method" outlined dense color="primary" emit-value map-options :disable="testingSsh" :options="[{ label: 'Local keys / agent', value: 'password' }, { label: 'Private key', value: 'private_key' }]" />
                        </div>
                        <div v-if="sshForm.auth_method === 'password'" class="col-12">
                            <q-input v-model="sshForm.password" label="SSH password (optional)" hint="Leave blank to use your SSH agent or local keys." type="password" outlined dense color="primary" :disable="testingSsh" />
                        </div>
                        <template v-if="sshForm.auth_method === 'private_key'">
                            <div class="col-12"><q-input v-model="sshForm.private_key" label="Private key or file path" :rules="[required]" type="textarea" outlined dense color="primary" autogrow hint="Paste your key or enter a path such as ~/.ssh/id_ed25519" :disable="testingSsh" /></div>
                            <div class="col-12"><q-input v-model="sshForm.passphrase" label="Key passphrase (optional)" type="password" outlined dense color="primary" hide-bottom-space :disable="testingSsh" /></div>
                        </template>
                    </div>
                    <q-banner dense rounded class="connection-note q-mt-lg">
                        <template #avatar><q-icon name="info_outline" color="primary" /></template>
                        Test SSH checks your SSH login. Test connection checks database access through the tunnel.
                    </q-banner>
                    <div class="text-caption text-grey-5 q-mt-md">Connect with OpenSSH first to trust the server in ~/.ssh/known_hosts.</div>
                </q-card-section>
                <q-separator dark />
                <q-card-actions align="right" class="q-pa-lg">
                    <q-btn flat no-caps label="Cancel" color="grey-5" :disable="testingSsh" @click="cancelSsh" />
                    <q-btn outline no-caps label="Test SSH" icon="network_check" color="primary" :loading="testingSsh" @click="testSshConnection" />
                    <q-btn unelevated no-caps label="Save SSH" icon="save" color="primary" text-color="black" :disable="testingSsh" type="submit" />
                </q-card-actions>
            </q-form>
        </q-card>
    </q-dialog>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from "vue";
import { isNoSQL } from '@/utils/databaseDialect';
import ConnectionGroupSelect from "@/components/connections/ConnectionGroupSelect.vue";
import type { QForm } from "quasar";
import { useConnectionStore } from "@/stores/connectionStore";
import { DbService, DatabaseService } from "@bindings/db-lens/internal/app";
import type { ConnectionConfig } from "@bindings/db-lens/internal/engine/entities";
import { Dialog, Notify } from "quasar";
import { useActiveConnection } from "@/stores/activeConnection";
import { useRouter, useRoute } from "vue-router";

const $router = useRouter();
const route = useRoute();
const editingId = computed(() => typeof route.query.edit === "string" ? route.query.edit : "");
const loadingSavedConnection = ref(false);
const saving = ref(false);
const store = useConnectionStore();
const activeConnectionStore = useActiveConnection();

const formRef = ref<QForm | null>(null);
const sshFormRef = ref<QForm | null>(null);

const form = ref({
    type: "pgx",
    ssl: false,
    host: "",
    port: 5432,
    user: "",
    password: "",
    database: "",
    connection_name: "",
    color: "",
    group: "" as string | null,
    save_connection: true,
    readonly: false,
    use_ssh: false,
});
const sqlDatabases = [
    { value: 'pgx', label: 'PostgreSQL', emoji: '🐘' },
    { value: 'mysql', label: 'MySQL', emoji: '🐬' },
    { value: 'sqlite', label: 'SQLite', emoji: '🪶' },
    { value: 'oracle', label: 'Oracle', emoji: '🔴' },
    { value: 'sqlserver', label: 'SQL Server', emoji: '🗄️' },
    { value: 'db2', label: 'IBM Db2', emoji: '🟦' },
];
const lastSQLDriver = ref('pgx');
const lastNoSQLDriver = ref('mongodb');
const nosqlDatabases = [
    { value: 'mongodb', label: 'MongoDB', emoji: '🍃' },
    { value: 'cassandra', label: 'Cassandra', emoji: '👁️' },
    { value: 'couchdb', label: 'CouchDB', emoji: '🛋️' },
    { value: 'redis', label: 'Redis', emoji: '⚡' },
];
const databaseGroup = computed({
    get: () => isNoSQL(form.value.type) ? 'nosql' : 'sql',
    set: (group: string) => {
        if (saving.value || loadingSavedConnection.value || group === databaseGroup.value) return;
        form.value.type = group === 'nosql' ? lastNoSQLDriver.value : lastSQLDriver.value;
    },
});
const availableDatabases = computed(() => databaseGroup.value === 'sql'
    ? sqlDatabases : nosqlDatabases);
const sshForm = ref({
    name: "",
    host: "",
    port: 22,
    username: "",
    auth_method: "password",
    private_key: "",
    passphrase: "",
    password: "",
});

const sshDialog = ref(false);
const testingSsh = ref(false);
let sshSnapshot = { ...sshForm.value };
watch(sshDialog, (open) => {
    if (open) sshSnapshot = { ...sshForm.value };
});
const isMongoURI = computed(() => form.value.type === "mongodb" && /^mongodb(?:\+srv)?:\/\//.test(form.value.host));
watch(isMongoURI, (enabled) => { if (enabled) form.value.use_ssh = false; });

const required = (value: string | number | null) =>
    !!value || "This field is required";

const validPort = (value: string | number | null) =>
    (Number.isInteger(Number(value)) && Number(value) > 0 && Number(value) <= 65535) || "Enter a port between 1 and 65535";
const validRedisDatabase = (value: string) => /^\d+$/.test(value) || 'Enter a non-negative database index, usually 0';

async function connect() {
    const valid = await formRef.value?.validate();

    if (!valid) {
        return;
    }

    try {
        const response = await DbService.PingConfig(parseToConfig());
        if (response) {
            Dialog.create({
                title: "Ping Successful",
                message: "Connect to the database?",
                ok: "OK",
                cancel: "Cancel",
            }).onOk(async () => {
                try {
                    const config = parseToConfig();
                    if (form.value.save_connection) {
                        await DbService.SaveAndConnect(config);
                    } else {
                        await DbService.Connect(config);
                    }
                } catch (err: any) {
                    Dialog.create({
                        message: err?.message || "Failed to save & connect",
                        color: "negative",
                    });
                    console.error(err);
                    return;
                }
                activeConnectionStore.setActiveConnection();
              // $router.push({ name: "workspace" });
                $router.push({ name: "WorkSpace" });
            });
        }
    } catch (error: any) {
        Dialog.create({
            message: error.message || "Failed to ping config",
            color: "negative",
        });
        console.error(error);
    }
}

function parseToConfig() {
    return <ConnectionConfig>{
        ID: editingId.value || undefined,
        Name: form.value.connection_name,
        Type: form.value.type,

        Host: form.value.host,
        Port: form.value.port,

        User: isMongoURI.value ? "" : form.value.user,
        Password: isMongoURI.value ? "" : form.value.password,
        Database: form.value.database,

        SSL: form.value.ssl,

        SSHConfigID: null,

        SSHConfig: form.value.type !== "sqlite" && form.value.use_ssh
            ? parseSshConfig()
            : null,

        InMemory: false,
        ReadOnly: form.value.readonly,
        Color: form.value.color,
        Group: (form.value.group || "").trim(),
    };
}

function parseSshConfig() {
    return {
        ID: 0,
        Name: sshForm.value.name,
        Host: sshForm.value.host.trim(),
        Port: Number(sshForm.value.port),
        Username: sshForm.value.username.trim(),
        AuthMethod: sshForm.value.auth_method,
        PrivateKey: sshForm.value.private_key,
        Passphrase: sshForm.value.passphrase,
        Password: sshForm.value.password,
    };
}

async function testSshConnection() {
    if (testingSsh.value || !(await sshFormRef.value?.validate())) return;
    testingSsh.value = true;
    try {
        const ok = await DbService.TestSSHConnection(parseSshConfig());
        if (!ok) throw new Error("SSH connection test failed");
        Notify.create({ message: "SSH connection successful", color: "positive" });
    } catch (error: any) {
        Dialog.create({ title: "SSH connection failed", message: error?.message || String(error), color: "negative" });
    } finally {
        testingSsh.value = false;
    }
}

async function saveSshConfig() {
    if (!(await sshFormRef.value?.validate())) return;
    sshDialog.value = false;
}

function cancelSsh() {
    sshDialog.value = false;
    sshForm.value = { ...sshSnapshot };
    if (!sshSnapshot.host || !sshSnapshot.username) form.value.use_ssh = false;
}

function resetSshForm() {
    sshForm.value = {
        name: "",
        host: "",
        port: 22,
        username: "",
        auth_method: "password",
        private_key: "",
        passphrase: "",
        password: "",
    };
}
watch(
    () => form.value.type,
    (newVal) => {
        if (sqlDatabases.some(database => database.value === newVal)) lastSQLDriver.value = newVal;
        if (isNoSQL(newVal)) lastNoSQLDriver.value = newVal;
        if (loadingSavedConnection.value) return;
        if (newVal === "pgx") {
            form.value.port = 5432;
        } else if (newVal === "mysql") {
            form.value.port = 3306;
        } else if (newVal === "mongodb") {
            form.value.port = 27017;
        } else if (newVal === "cassandra") {
            form.value.port = 9042;
        } else if (newVal === "couchdb") {
            form.value.port = 5984;
        } else if (newVal === "redis") {
            form.value.port = 6379;
            if (!/^\d+$/.test(form.value.database)) form.value.database = '0';
        } else if (newVal === "oracle") {
            form.value.port = 1521;
        } else if (newVal === "sqlserver") {
            form.value.port = 1433;
        } else if (newVal === "db2") {
            form.value.port = 50000;
        } else if (newVal === "sqlite") {
            form.value.port = 0;
            form.value.use_ssh = false;
        }
    },
);

watch(
    () => form.value.use_ssh,
    (enabled) => {
        if (loadingSavedConnection.value) return;
        if (enabled) {
            sshDialog.value = true;
        } else {
            resetSshForm();
        }
    },
);

async function loadSavedConnection() {
    loadingSavedConnection.value = true;
    try {
        if (editingId.value) {
            await store.getConnections();
            const connection = store.connections.find(item => String(item.id) === editingId.value);
            if (!connection) throw new Error("Saved connection not found");
            const ssh = connection.ssh_config;
            form.value = {
                type: connection.driver, host: connection.host, port: Number(connection.port.Int64),
                ssl: connection.ssl ?? false,
                user: connection.user, password: connection.password, database: connection.dbname,
                group: connection.group || "", connection_name: connection.name, color: connection.color.Valid ? connection.color.String : "",
                save_connection: true, readonly: connection.read_only, use_ssh: !!connection.ssh_config_id.Valid,
            };
            sshForm.value = {
                name: ssh.name, host: ssh.host, port: ssh.port || 22, username: ssh.username,
                auth_method: ssh.auth_method || "password", private_key: ssh.private_key.Valid ? ssh.private_key.String : "",
                passphrase: ssh.passphrase.Valid ? ssh.passphrase.String : "", password: ssh.password.Valid ? ssh.password.String : "",
            };
        } else {
            form.value = { type: "pgx", ssl: false, host: "", port: 5432, user: "", password: "", database: "", connection_name: "", color: "", group: "", save_connection: true, readonly: false, use_ssh: false };
            resetSshForm();
        }
        sshDialog.value = false;
        await nextTick();
        formRef.value?.resetValidation();
    } catch (error: any) {
        Notify.create({ message: error?.message || "Failed to load connection", color: "negative" });
        await cancelEdit();
    } finally { loadingSavedConnection.value = false; }
}
async function cancelEdit() {
    await $router.push({ name: "Welcome", query: {} });
}
async function saveConnection() {
    if (saving.value || !(await formRef.value?.validate())) return;
    if (form.value.use_ssh && (!sshForm.value.name || !sshForm.value.host || !sshForm.value.username || validPort(sshForm.value.port) !== true || (sshForm.value.auth_method === "private_key" && !sshForm.value.private_key))) {
        sshDialog.value = true;
        return;
    }
    saving.value = true;
    try {
        await DatabaseService.UpdateConnection(parseToConfig());
        await store.getConnections();
        Notify.create({ message: "Connection updated", color: "positive" });
        await cancelEdit();
    } catch (error: any) {
        Notify.create({ message: error?.message || "Failed to update connection", color: "negative" });
    } finally { saving.value = false; }
}
watch(editingId, loadSavedConnection);
onMounted(async () => {
    if (editingId.value) await loadSavedConnection();
    else await store.getConnections();
});
</script>

<style scoped>
.connection-setup.q-card { font-size: var(--ui-font-size); width: 100%; max-width: none; overflow: hidden; border: 1px solid var(--border-default); border-radius: 14px; background: var(--surface-panel); box-shadow: 0 10px 36px #0001; }
.connection-setup-grid { display: grid; grid-template-columns: calc(204px + 4 * var(--ui-font-size)) minmax(0, 1fr); }
.database-panel { display: flex; flex-direction: column; padding: 24px 20px; background: var(--surface-data); border-right: 1px solid var(--border-default); }
.panel-eyebrow { font-size: max(9px, calc(var(--ui-font-size) - 2px)); font-weight: 600; letter-spacing: 1.5px; color: var(--text-muted); }
h2 { margin: 7px 0 0; color: var(--text-primary); font-size: calc(var(--ui-font-size) + 7px); line-height: 1.4; font-weight: 600; letter-spacing: -.3px; }
.panel-description { margin: 7px 0 0; font-size: var(--ui-font-size); line-height: 1.65; color: var(--text-muted); }
.database-category-tabs { margin-top: 22px; border: 1px solid var(--border-default); border-radius: 7px; background: var(--surface-base); }
.database-category-tabs :deep(.q-tab) { min-height: 34px; }
.database-category-tabs :deep(.q-tab--active) { background: var(--accent-tint); }
.database-category-tabs :deep(.q-tab__indicator) { display: none; }
.database-options { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; margin-top: 16px; }
.database-option { position: relative; min-width: 0; min-height: 78px; padding: 12px 4px; border: 1px solid var(--border-default); border-radius: 9px; background: var(--surface-panel); transition: border-color 120ms, background 120ms; }
.database-option:hover { border-color: var(--accent-color); }
.database-option-selected { border-color: var(--accent-color); background: var(--accent-tint); }
.database-option-content { display: flex; flex-direction: column; align-items: center; gap: 8px; width: 100%; min-width: 0; text-align: center; }
.database-option-icon { font-family: "Apple Color Emoji", "Segoe UI Emoji", "Noto Color Emoji", sans-serif; font-size: calc(var(--ui-font-size) + 12px); line-height: 1.1; }
.database-option-name { max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: var(--ui-font-size); font-weight: 500; }
.database-selected-check { position: absolute; top: 6px; right: 6px; }
.database-option:focus-visible { outline: 2px solid var(--selection-border); outline-offset: 2px; }
.database-panel-note { display: flex; align-items: flex-start; gap: 8px; margin-top: auto; padding-top: 28px; font-size: max(10px, calc(var(--ui-font-size) - 1px)); color: var(--text-muted); line-height: 1.6; }
.database-panel-note .q-icon { flex-shrink: 0; margin-top: 1px; }
.connection-details-panel { min-width: 0; }
.connection-details-heading { display: flex; align-items: center; gap: 14px; padding: 20px 24px; border-bottom: 1px solid var(--border-default); }
.connection-heading-icon { display: grid; place-items: center; width: 44px; height: 44px; flex-shrink: 0; border-radius: 12px; color: var(--accent-color); background: var(--accent-tint); }
.connection-heading-copy { flex: 1; min-width: 0; }
.connection-heading-copy h2 { margin-top: 3px; font-size: calc(var(--ui-font-size) + 10px); }
.connection-heading-copy .panel-description { margin-top: 3px; }
.connection-form-body { padding: 18px 24px; }
.form-section-title { display: flex; align-items: center; gap: 8px; color: var(--text-muted); margin-bottom: 12px; }
.form-section-title h3 { margin: 0; font-size: var(--ui-font-size); font-weight: 600; color: var(--text-primary); }
.details-section-title { padding-top: 0; margin-bottom: 0; }
.options-section-title { margin-top: 18px; margin-bottom: 12px; }
.connection-preferences { display: flex; flex-wrap: wrap; align-items: center; gap: 12px 22px; }
.connection-options { border-color: var(--border-default); border-radius: 8px; background: var(--surface-data); }
.connection-options :deep(.q-item__section--avatar) { min-width: 32px; }
.connection-form-body :deep(.q-field--outlined .q-field__control) { min-height: max(42px, calc(var(--ui-font-size) + 24px)); border-radius: 7px; background: var(--surface-data); }
.connection-setup :deep(.q-field__native), .connection-setup :deep(.q-field__input), .connection-setup :deep(.q-field__label),
.connection-setup :deep(.q-field__bottom), .connection-setup :deep(.q-btn__content), .connection-setup :deep(.q-item__label), .connection-setup :deep(.q-checkbox__label) { font-size: var(--ui-font-size); }
.connection-form-body :deep(.q-field--outlined .q-field__control::before) { border-color: var(--border-default); }
.connection-form-actions { display: flex; flex-wrap: wrap; gap: 10px; padding: 14px 24px; }
.connection-form-actions > .q-btn { margin-left: 0; min-height: 38px; border-radius: 7px; padding: 0 16px; }
.connection-test-note { display: flex; align-items: center; gap: 7px; flex: 1; font-size: max(10px, calc(var(--ui-font-size) - 1px)); line-height: 1.5; color: var(--text-muted); }
.ssh-card.q-card { width: 600px; max-width: calc(100vw - 32px); border-radius: 12px; background: var(--surface-panel); }
.connection-note { background: var(--accent-tint); color: var(--text-muted); }
@media (max-width: 800px) {
    .connection-setup-grid { grid-template-columns: 1fr; }
    .database-panel { padding: 20px; border-right: 0; border-bottom: 1px solid var(--border-default); }
    .database-panel h2 { font-size: calc(var(--ui-font-size) + 6px); }
    .database-panel > .panel-description, .database-panel-note { display: none; }
    .database-category-tabs { margin-top: 14px; }
    .database-options { grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; margin-top: 12px; }
    .database-option { min-height: 56px; padding: 10px 6px; }
    .database-option-content { flex-direction: row; gap: 8px; text-align: left; }
    .database-option-icon { font-size: calc(var(--ui-font-size) + 9px); }
    .database-selected-check { display: none; }
    .connection-details-heading { padding: 20px; }
    .connection-form-body { padding: 20px; }
    .connection-form-actions { padding: 16px 20px; }
}
@media (max-width: 480px) {
    .database-options { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .connection-heading-icon { display: none; }
    .connection-heading-copy h2 { font-size: calc(var(--ui-font-size) + 8px); }
    .connection-test-note { flex-basis: 100%; }
}
</style>
