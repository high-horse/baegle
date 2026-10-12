<template>
    <q-layout view="hHh Lpr fFf" class="connection-portal">
        <q-header bordered class="portal-header">
            <q-toolbar class="q-px-lg" style="min-height: 64px">
                <q-btn flat round dense icon="menu" aria-label="Toggle saved connections" :aria-expanded="drawerOpen" color="grey-4" class="q-mr-sm" @click="drawerOpen = !drawerOpen" />
                <q-icon name="storage" size="24px" color="amber" />
                <q-toolbar-title>
                    <div class="text-subtitle1 text-weight-medium">DB Viewer</div>
                    <div class="text-caption text-grey-5">Database workspace</div>
                </q-toolbar-title>
                <q-btn unelevated no-caps color="amber" text-color="black" icon="add" label="New connection" @click="$router.push({ name: 'Welcome', query: {} })" />
            </q-toolbar>
        </q-header>
        <q-drawer v-model="drawerOpen" show-if-above :breakpoint="1023" :width="280" bordered class="portal-drawer">
            <q-scroll-area class="fit" :content-style="{ width: '100%' }">
                <q-list padding class="q-pa-md">
                    <q-item dense class="q-mb-sm">
                        <q-item-section class="text-caption text-grey-5">Saved connections</q-item-section>
                        <q-item-section side>
                            <q-btn flat round dense icon="close" color="grey-5" aria-label="Close saved connections" @click="drawerOpen = false" />
                        </q-item-section>
                    </q-item>
                    <q-item v-if="!connections.length">
                        <q-item-section>
                            <q-item-label class="text-grey-4">No saved connections</q-item-label>
                            <q-item-label caption>Save a connection to find it here.</q-item-label>
                        </q-item-section>
                    </q-item>
                    <q-item v-for="connection in connections" :key="connection.id" clickable :active="String(connection.id) === $route.query.edit" active-class="text-amber" class="saved-connection q-mb-sm" @click="selectConnection(connection)" @keydown.enter.prevent="selectConnection(connection)" @contextmenu.prevent.stop="actionMenuId = connection.id">
                        <q-item-section avatar>
                            <q-spinner v-if="connectingConnectionId === connection.id" color="amber" size="20px" />
                            <q-icon v-else name="lan" color="amber" size="20px" />
                        </q-item-section>
                        <q-item-section>
                            <q-item-label class="ellipsis">{{ connection.name }}</q-item-label>
                            <q-item-label v-if="connectingConnectionId === connection.id" caption class="text-amber" role="status">{{ connectionStage }}</q-item-label>
                            <q-item-label caption class="ellipsis">{{ connection.driver }} · {{ connection.host ? `${connection.host}:${connection.port.Int64}` : connection.dbname?.split('/').pop() }}</q-item-label>

                        </q-item-section>
                        <q-item-section side class="connection-actions">
                            <q-btn flat round dense icon="more_vert" color="amber" :aria-label="`Actions for ${connection.name}`" @click.stop="actionMenuId = actionMenuId === connection.id ? null : connection.id" @dblclick.stop @keydown.enter.stop @keydown.space.stop>
                                <q-menu :model-value="actionMenuId === connection.id" no-parent-event class="connection-menu" anchor="bottom right" self="top right" :offset="[0, 4]" @update:model-value="open => { if (!open && actionMenuId === connection.id) actionMenuId = null; }">
                                    <q-list dense style="min-width: 180px">
                                        <q-item clickable v-close-popup @click="actionMenuId = null; editConnection(connection.id)">
                                            <q-item-section avatar><q-icon name="edit" size="18px" /></q-item-section>
                                            <q-item-section>Edit connection</q-item-section>
                                        </q-item>
                                        <q-item clickable v-close-popup class="text-red-4" @click="actionMenuId = null; connectionToDelete = connection">
                                            <q-item-section avatar><q-icon name="delete_outline" size="18px" /></q-item-section>
                                            <q-item-section>Delete connection</q-item-section>
                                        </q-item>
                                    </q-list>
                                </q-menu>
                            </q-btn>
                        </q-item-section>
                    </q-item>
                </q-list>
            </q-scroll-area>
        </q-drawer>
        <q-page-container><router-view /></q-page-container>
        <q-dialog :model-value="!!connectionError" @update:model-value="open => { if (!open) store.connectionError = null; }">
            <q-card flat bordered class="connection-card" style="width: 520px">
                <q-card-section class="row items-center no-wrap q-pa-lg">
                    <q-icon name="error_outline" color="negative" size="28px" class="q-mr-md" />
                    <div class="col">
                        <div class="text-h6">Unable to connect</div>
                        <div class="text-caption text-grey-5">{{ connectionError?.name }}</div>
                    </div>
                </q-card-section>
                <q-card-section class="q-px-lg q-pt-none">
                    <q-banner rounded class="connection-note connection-error-message">{{ connectionError?.message }}</q-banner>
                    <div class="text-caption text-grey-5 q-mt-md">Check the saved host, port, credentials, and SSH settings, then try again.</div>
                </q-card-section>
                <q-separator dark />
                <q-card-actions align="right" class="q-pa-lg">
                    <q-btn flat no-caps label="Close" color="grey-5" @click="store.connectionError = null" />
                    <q-btn outline no-caps label="Edit connection" color="amber" icon="edit" @click="editFailedConnection" />
                    <q-btn unelevated no-caps label="Try again" color="amber" text-color="black" icon="refresh" @click="retryConnection" />
                </q-card-actions>
            </q-card>
        </q-dialog>
        <q-dialog :model-value="!!connectionToDelete" :persistent="deletingConnection" @update:model-value="value => { if (!value) connectionToDelete = null; }">
            <q-card flat bordered class="connection-card" style="width: 420px">
                <q-card-section class="text-h6 q-pa-lg">Delete connection</q-card-section>
                <q-card-section class="q-px-lg q-pt-none">Delete “{{ connectionToDelete?.name }}” from saved connections? Your database and its data will remain unchanged.</q-card-section>
                <q-separator dark />
                <q-card-actions align="right" class="q-pa-lg">
                    <q-btn flat no-caps label="Cancel" color="grey-5" :disable="deletingConnection" @click="connectionToDelete = null" />
                    <q-btn unelevated no-caps color="negative" label="Delete" :loading="deletingConnection" @click="deleteConnection" />
                </q-card-actions>
            </q-card>
        </q-dialog>
    </q-layout>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useRouter } from "vue-router";
import { storeToRefs } from "pinia";
import { useConnectionStore } from "@/stores/connectionStore";
import { useActiveConnection } from "@/stores/activeConnection";
import type { Connection } from "@bindings/db-lens/internal/types";

const $router = useRouter();
const drawerOpen = ref(false);
const actionMenuId = ref<number | null>(null);
const activeConnectionStore = useActiveConnection();
const store = useConnectionStore();
const connectionToDelete = ref<Connection | null>(null);
const deletingConnection = ref(false);

async function deleteConnection() {
    const connection = connectionToDelete.value;
    if (!connection || deletingConnection.value) return;
    deletingConnection.value = true;
    try {
        if (await store.deleteConnection(connection.id)) {
            connectionToDelete.value = null;
            if ($router.currentRoute.value.query.edit === String(connection.id)) {
                await $router.replace({ name: "Welcome", query: {} });
            }
        }
    } finally {
        deletingConnection.value = false;
    }
}

const { connections, activeConnection, connectingConnectionId, connectionStage, connectionError } =
    storeToRefs(store);

function editConnection(id: number) {
    $router.push({ name: "Welcome", query: { edit: String(id) } });
}

async function selectConnection(connection: Connection) {
    if (store.loadingStates.connecting) return;
    actionMenuId.value = null;
    store.setSelectedSession(connection);

    const connected = await store.connectToSession(connection);
    if (connected) {
        await activeConnectionStore.setActiveConnection();
        $router.push({ name: "WorkSpace" });
    }
}

function editFailedConnection() {
    const id = connectionError.value?.connectionId;
    store.connectionError = null;
    if (id !== undefined) editConnection(id);
}

async function retryConnection() {
    const id = connectionError.value?.connectionId;
    store.connectionError = null;
    const connection = connections.value.find(item => item.id === id);
    if (connection) await selectConnection(connection);
}

watch(activeConnection, (newConnection) => {
    if (newConnection) {
        console.log("connection dd", newConnection);
    }
});

</script>

<style>
.connection-portal { background: #282828; color: #eeeae4; }
.portal-header, .connection-card, .connection-menu { background: #353535; color: #eeeae4; }
.portal-drawer { background: #2d2d2d; color: #eeeae4; }
.connection-card { width: 100%; max-width: 600px; border-color: #505050; border-radius: 12px; }
.ssh-card { width: 560px; max-width: calc(100vw - 32px); }
.connection-options, .saved-connection { border: 1px solid #505050; border-radius: 8px; }
.saved-connection { background: #353535; width: 100%; min-width: 0; }
.saved-connection.q-item--active { border-color: #c99334; background: #423c32; }
.saved-connection .q-item__section--avatar { min-width: 32px; }
.saved-connection > .q-item__section--main { min-width: 0; overflow: hidden; }
.connection-actions { flex-shrink: 0; padding-left: 8px; }
.connection-error-message { white-space: pre-wrap; overflow-wrap: anywhere; }
.connection-note { background: #3c3933; color: #d5cec3; }
.connection-card .q-card__actions { gap: 8px; }
.connection-card .q-card__actions > .q-btn { margin-left: 0; }
</style>
