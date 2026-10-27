<template>
    <q-card flat bordered class="connection-card">
        <q-card-section class="row items-center no-wrap q-pa-lg">
            <q-avatar rounded color="amber" text-color="black" icon="storage" size="44px" />
            <div class="q-ml-md col">
                <div class="text-h6">{{ editingId ? 'Edit connection' : 'Database connection' }}</div>
                <div class="text-caption text-grey-5">{{ editingId ? 'Update your saved connection settings.' : 'Choose a database and enter your connection details.' }}</div>
            </div>
            <q-btn v-if="editingId" flat round dense icon="close" aria-label="Cancel editing" color="grey-5" @click="cancelEdit" />
        </q-card-section>
        <q-tabs v-model="databaseGroup" dense active-color="amber" indicator-color="amber" align="justify" no-caps class="q-px-md" aria-label="Database category">
            <q-tab name="sql" label="SQL" />
            <q-tab name="nosql" label="NoSQL" />
        </q-tabs>
        <div class="database-options q-pa-md" role="group" :aria-label="databaseGroup === 'sql' ? 'SQL databases' : 'NoSQL databases'">
            <q-btn v-for="database in availableDatabases" :key="database.value" flat no-caps
                :label="database.label" :aria-pressed="form.type === database.value"
                :color="form.type === database.value ? 'amber' : 'grey-5'"
                :class="{ 'database-option-selected': form.type === database.value }"
                class="database-option" :disable="saving || loadingSavedConnection"
                @click="form.type = database.value" />
        </div>
        <q-separator dark />
        <q-form ref="formRef" @submit="editingId ? saveConnection() : connect()">
            <q-card-section class="q-pa-lg">
                <div class="row q-col-gutter-md">
                    <div class="col-12">
                        <q-input v-model="form.connection_name" label="Connection name" :rules="[required]" outlined dense color="amber" hide-bottom-space />
                    </div>
                    <div class="col-12">
                        <ConnectionGroupSelect v-model="form.group" :groups="store.connectionGroups" :disable="saving || loadingSavedConnection" />
                    </div>
                    <template v-if="form.type !== 'sqlite'">
                        <div class="col-12 col-sm-8">
                            <q-input v-model="form.host" :label="form.type === 'mongodb' ? 'Host or MongoDB URI' : 'Database host'" :rules="[required]" outlined dense color="amber" hide-bottom-space />
                        </div>
                        <div class="col-12 col-sm-4">
                            <q-input v-model.number="form.port" label="Port" type="number" :disable="isMongoURI" :rules="isMongoURI ? [] : [validPort]" outlined dense color="amber" hide-bottom-space />
                        </div>
                        <div v-if="form.use_ssh || form.type === 'mongodb'" class="col-12 text-caption text-grey-5">
                            {{ form.use_ssh ? 'Use the database host reachable from your SSH server.' : 'Enter a host or mongodb:// / mongodb+srv:// URI.' }}
                        </div>
                        <div class="col-12 col-sm-6">
                            <q-input v-model="form.user" :label="isNoSQL(form.type) ? 'User (optional)' : 'User'" :disable="isMongoURI" :rules="isNoSQL(form.type) ? [] : [required]" outlined dense color="amber" hide-bottom-space />
                        </div>
                        <div class="col-12 col-sm-6">
                            <q-input v-model="form.password" label="Password" type="password" :disable="isMongoURI" outlined dense color="amber" hide-bottom-space />
                        </div>
                        <div class="col-12">
                            <q-input v-model="form.database" :label="form.type === 'oracle' ? 'Service name' : form.type === 'cassandra' ? 'Keyspace' : form.type === 'redis' ? 'Database index' : 'Database'" :rules="form.type === 'redis' ? [validRedisDatabase] : [required]" outlined dense color="amber" hide-bottom-space />
                        </div>
                        <div v-if="form.type === 'oracle'" class="col-12 text-caption text-grey-5">Enter the Oracle service name, for example FREEPDB1. Connects using a database username and password.</div>
                        <div v-if="form.type === 'sqlserver'" class="col-12 text-caption text-grey-5">Use SQL Server authentication and the instance’s TCP port (usually 1433).</div>
                        <div v-if="form.type === 'db2'" class="col-12 text-caption text-grey-5">Db2 LUW requires a Db2-enabled app build and the IBM CLI client libraries.</div>
                        <div v-if="form.type === 'cassandra'" class="col-12 text-caption text-grey-5">Connects to this Cassandra node. Enter an existing keyspace and run CQL statements.</div>
                        <div v-if="form.type === 'couchdb'" class="col-12 text-caption text-grey-5">Enter the host without http:// and an existing database name. Enable TLS for HTTPS.</div>
                        <div v-if="form.type === 'redis'" class="col-12 text-caption text-grey-5">Standalone Redis. Database 0 is the default. Leave User empty for password-only authentication.</div>
                    </template>
                    <div v-else class="col-12">
                        <q-input v-model="form.database" label="Database file" hint="/path/to/database.db" :rules="[required]" outlined dense color="amber">
                            <template #append><q-icon name="folder_open" /></template>
                        </q-input>
                    </div>
                </div>
                <div class="row items-center q-gutter-x-lg q-mt-lg">
                    <q-checkbox v-if="!editingId" v-model="form.save_connection" label="Save connection" dense color="amber" />
                    <q-checkbox v-model="form.readonly" label="Read only" dense color="amber" />
                    <q-checkbox v-if="['oracle', 'sqlserver', 'db2'].includes(form.type) || isNoSQL(form.type)" v-model="form.ssl" label="Require TLS encryption" dense color="amber" />
                </div>
                <q-list v-if="form.type !== 'sqlite'" bordered class="connection-options q-mt-lg">
                    <q-item>
                        <q-item-section avatar><q-icon name="vpn_key" color="amber" /></q-item-section>
                        <q-item-section>
                            <q-item-label>SSH tunnel</q-item-label>
                            <q-item-label caption>{{ form.use_ssh && sshForm.host ? `${sshForm.username}@${sshForm.host}:${sshForm.port}` : 'Connect through an SSH server' }}</q-item-label>
                        </q-item-section>
                        <q-item-section side><q-toggle v-model="form.use_ssh" :disable="isMongoURI" color="amber" aria-label="Connect through SSH tunnel" /></q-item-section>
                    </q-item>
                    <q-item v-if="form.use_ssh" dense>
                        <q-item-section><q-btn flat no-caps color="amber" label="Configure SSH" icon="settings" @click="sshDialog = true" /></q-item-section>
                    </q-item>
                </q-list>
            </q-card-section>
            <q-separator dark />
            <q-card-actions align="right" class="q-pa-lg">
                <q-btn v-if="editingId" flat no-caps label="Cancel" color="grey-5" :disable="saving" @click="cancelEdit" />
                <q-btn unelevated no-caps color="amber" text-color="black" :label="editingId ? 'Save changes' : 'Test connection'" :icon="editingId ? 'save' : 'network_check'" type="submit" :loading="saving" :disable="loadingSavedConnection" />
            </q-card-actions>
        </q-form>
    </q-card>

    <q-dialog v-model="sshDialog" persistent>
        <q-card flat bordered class="connection-card ssh-card">
            <q-card-section class="row items-center no-wrap q-pa-lg">
                <q-avatar rounded color="amber-10" text-color="amber-3" icon="vpn_key" size="44px" />
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
                        <div class="col-12"><q-input v-model="sshForm.name" label="Configuration name" :rules="[required]" outlined dense color="amber" hide-bottom-space :disable="testingSsh" /></div>
                        <div class="col-12 col-sm-8"><q-input v-model="sshForm.host" label="SSH host" :rules="[required]" outlined dense color="amber" hide-bottom-space :disable="testingSsh" /></div>
                        <div class="col-12 col-sm-4"><q-input v-model.number="sshForm.port" label="Port" type="number" :rules="[validPort]" outlined dense color="amber" hide-bottom-space :disable="testingSsh" /></div>
                        <div class="col-12"><q-input v-model="sshForm.username" label="Username" :rules="[required]" outlined dense color="amber" hide-bottom-space :disable="testingSsh" /></div>
                        <div class="col-12">
                            <q-select v-model="sshForm.auth_method" label="Authentication method" outlined dense color="amber" emit-value map-options :disable="testingSsh" :options="[{ label: 'Local keys / agent', value: 'password' }, { label: 'Private key', value: 'private_key' }]" />
                        </div>
                        <div v-if="sshForm.auth_method === 'password'" class="col-12">
                            <q-input v-model="sshForm.password" label="SSH password (optional)" hint="Leave blank to use your SSH agent or local keys." type="password" outlined dense color="amber" :disable="testingSsh" />
                        </div>
                        <template v-if="sshForm.auth_method === 'private_key'">
                            <div class="col-12"><q-input v-model="sshForm.private_key" label="Private key or file path" :rules="[required]" type="textarea" outlined dense color="amber" autogrow hint="Paste your key or enter a path such as ~/.ssh/id_ed25519" :disable="testingSsh" /></div>
                            <div class="col-12"><q-input v-model="sshForm.passphrase" label="Key passphrase (optional)" type="password" outlined dense color="amber" hide-bottom-space :disable="testingSsh" /></div>
                        </template>
                    </div>
                    <q-banner dense rounded class="connection-note q-mt-lg">
                        <template #avatar><q-icon name="info_outline" color="amber" /></template>
                        Test SSH checks your SSH login. Test connection checks database access through the tunnel.
                    </q-banner>
                    <div class="text-caption text-grey-5 q-mt-md">Connect with OpenSSH first to trust the server in ~/.ssh/known_hosts.</div>
                </q-card-section>
                <q-separator dark />
                <q-card-actions align="right" class="q-pa-lg">
                    <q-btn flat no-caps label="Cancel" color="grey-5" :disable="testingSsh" @click="cancelSsh" />
                    <q-btn outline no-caps label="Test SSH" icon="network_check" color="amber" :loading="testingSsh" @click="testSshConnection" />
                    <q-btn unelevated no-caps label="Save SSH" icon="save" color="amber" text-color="black" :disable="testingSsh" type="submit" />
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
    { value: 'pgx', label: 'PostgreSQL' },
    { value: 'mysql', label: 'MySQL' },
    { value: 'sqlite', label: 'SQLite' },
    { value: 'oracle', label: 'Oracle' },
    { value: 'sqlserver', label: 'SQL Server' },
    { value: 'db2', label: 'IBM Db2' },
];
const lastSQLDriver = ref('pgx');
const lastNoSQLDriver = ref('mongodb');
const nosqlDatabases = [
    { value: 'mongodb', label: 'MongoDB' },
    { value: 'cassandra', label: 'Cassandra' },
    { value: 'couchdb', label: 'CouchDB' },
    { value: 'redis', label: 'Redis' },
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
.database-options {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
}
.database-option {
    min-width: 0;
    min-height: 36px;
    border: 1px solid #484848;
    border-radius: 6px;
}
.database-option-selected {
    border-color: #b88626;
    background: rgba(255, 193, 7, 0.08);
}
@media (max-width: 480px) {
    .database-options { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
