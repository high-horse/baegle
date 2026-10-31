<template>
    <q-layout view="hHh Lpr fFf" class="connection-portal">
        <q-header bordered class="portal-header">
            <q-toolbar class="portal-toolbar">
                <q-btn flat round dense icon="menu" aria-label="Toggle saved connections" :aria-expanded="drawerOpen" color="grey-4" class="q-mr-sm" @click="drawerOpen = !drawerOpen" />
                <AppIcon :size="32" />
                <q-toolbar-title class="portal-brand">
                    <div class="portal-brand-name">baegle</div>
                    <div class="portal-brand-caption">Database workspace</div>
                </q-toolbar-title>
                <q-btn unelevated no-caps color="primary" text-color="black" icon="add" label="New connection" class="new-connection-button" @click="$router.push({ name: 'Welcome', query: {} })" />
            </q-toolbar>
        </q-header>
        <q-drawer v-model="drawerOpen" show-if-above :breakpoint="1023" :width="drawerWidth" bordered class="portal-drawer" :class="{ 'portal-drawer-resizing': resizingDrawer }">
            <q-scroll-area class="fit" :content-style="{ width: '100%' }">
                <q-list padding class="connection-list q-pa-md">
                    <div class="saved-connections-heading">
                        <div><span>Saved connections</span><q-badge rounded class="connection-count">{{ connections.length }}</q-badge></div>
                        <q-btn flat round dense icon="close" color="grey-5" aria-label="Close saved connections" @click="drawerOpen = false" />
                    </div>
                    <q-input ref="connectionSearchInput" v-model="connectionSearch" outlined dense clearable color="primary" placeholder="Search connections…" aria-label="Search saved connections" hide-bottom-space aria-keyshortcuts="Control+Shift+F" class="q-mb-md">
                        <template #prepend><q-icon name="search" /></template>
                    </q-input>
                    <q-item v-if="!connections.length" class="connections-empty-state">
                        <q-item-section>
                            <q-icon name="bookmark_border" size="28px" class="q-mb-md" />
                            <q-item-label class="text-grey-4">No saved connections yet</q-item-label>
                            <q-item-label caption>Save a connection to find it here.</q-item-label>
                        </q-item-section>
                    </q-item>
                    <q-item v-else-if="!filteredConnections.length">
                        <q-item-section>
                            <q-item-label class="text-grey-4" role="status">No matching connections</q-item-label>
                            <q-item-label caption>Try a different search.</q-item-label>
                        </q-item-section>
                    </q-item>
                    <q-expansion-item v-for="group in groupedConnections" :key="group.name" default-opened header-class="text-grey-4" class="connection-group">
                        <template #header>
                            <q-item-section avatar class="group-folder"><q-icon name="folder_open" size="18px" /></q-item-section>
                            <q-item-section class="group-name">{{ group.name || 'Ungrouped' }}</q-item-section>
                            <q-item-section side><q-badge rounded class="group-count">{{ group.connections.length }}</q-badge></q-item-section>
                        </template>
                    <q-item v-for="connection in group.connections" :key="connection.id" clickable :active="String(connection.id) === $route.query.edit" active-class="text-primary" class="saved-connection" @click="selectConnection(connection)" @keydown.enter.prevent="selectConnection(connection)" @contextmenu.prevent.stop="actionMenuId = connection.id">
                        <q-item-section avatar class="saved-connection-avatar">
                            <div class="saved-connection-icon">
                                <q-spinner v-if="connectingConnectionId === connection.id" color="primary" size="18px" />
                                <q-icon v-else :name="connection.driver === 'mongodb' ? 'data_object' : 'storage'" size="18px" />
                            </div>
                        </q-item-section>
                        <q-item-section>
                            <q-item-label class="connection-name">{{ connection.name }}</q-item-label>
                            <q-item-label v-if="connectingConnectionId === connection.id" caption class="text-primary" role="status">{{ connectionStage }}</q-item-label>
                            <q-item-label caption class="connection-address"><span class="saved-driver">{{ connection.driver }}</span><span class="saved-endpoint">{{ connection.host ? `${connection.host}${connection.port.Valid ? ':' + connection.port.Int64 : ''}` : connection.dbname?.split('/').pop() }}</span></q-item-label>
                            <q-tooltip :delay="350" max-width="420px" class="connection-tooltip">
                                <div class="text-weight-medium">{{ connection.name }}</div>
                                <div>Driver: {{ connection.driver }}</div>
                                <div v-if="connection.host">Host: {{ connection.host }}</div>
                                <div v-if="connection.host && connection.port.Valid">Port: {{ connection.port.Int64 }}</div>
                                <div v-if="connection.dbname">Database: {{ connection.dbname }}</div>
                            </q-tooltip>
                        </q-item-section>
                        <q-item-section side class="connection-actions">
                            <q-btn flat round dense icon="more_vert" color="primary" :aria-label="`Actions for ${connection.name}`" @click.stop="actionMenuId = actionMenuId === connection.id ? null : connection.id" @dblclick.stop @keydown.enter.stop @keydown.space.stop>
                                <q-menu :model-value="actionMenuId === connection.id" no-parent-event class="connection-menu" anchor="bottom right" self="top right" :offset="[0, 4]" @update:model-value="open => { if (!open && actionMenuId === connection.id) actionMenuId = null; }">
                                    <q-list dense style="min-width: 180px">
                                        <q-item clickable v-close-popup @click="actionMenuId = null; editConnection(connection.id)">
                                            <q-item-section avatar><q-icon name="edit" size="18px" /></q-item-section>
                                            <q-item-section>Edit connection</q-item-section>
                                        </q-item>
                                        <q-item clickable v-close-popup @click="openGroupDialog(connection)">
                                            <q-item-section avatar><q-icon name="drive_file_move" size="18px" /></q-item-section>
                                            <q-item-section>Move to group</q-item-section>
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
                    </q-expansion-item>
                </q-list>
            </q-scroll-area>
            <div class="connection-drawer-resizer" role="separator" tabindex="0" aria-label="Resize saved connections sidebar" aria-orientation="vertical"
                :aria-valuemin="drawerMinWidth" :aria-valuemax="drawerMaxWidth" :aria-valuenow="drawerWidth"
                @pointerdown="startDrawerResize" @pointermove="resizeDrawer" @pointerup="finishDrawerResize"
                @pointercancel="finishDrawerResize" @lostpointercapture="finishDrawerResize" @keydown="resizeDrawerWithKeyboard" />
        </q-drawer>
        <q-dialog :model-value="!!groupConnection" :persistent="savingGroup" @update:model-value="open => { if (!open) groupConnection = null; }">
            <q-card class="connection-card" style="width: 420px">
                <q-form @submit="saveGroup">
                    <q-card-section>
                        <div class="text-h6">Move to group</div>
                        <div class="text-caption q-mb-md">{{ groupConnection?.name }}</div>
                        <ConnectionGroupSelect v-model="groupName" :groups="store.connectionGroups" :disable="savingGroup" />
                    </q-card-section>
                    <q-card-actions align="right">
                        <q-btn flat label="Cancel" :disable="savingGroup" @click="groupConnection = null" />
                        <q-btn type="submit" label="Save" color="primary" text-color="black" :loading="savingGroup" />
                    </q-card-actions>
                </q-form>
            </q-card>
        </q-dialog>
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
                    <q-btn outline no-caps label="Edit connection" color="primary" icon="edit" @click="editFailedConnection" />
                    <q-btn unelevated no-caps label="Try again" color="primary" text-color="black" icon="refresh" @click="retryConnection" />
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
import AppIcon from "@/components/AppIcon.vue";
import ConnectionGroupSelect from "@/components/connections/ConnectionGroupSelect.vue";
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { useQuasar, type QInput } from "quasar";
import { useRouter } from "vue-router";
import { storeToRefs } from "pinia";
import { useConnectionStore } from "@/stores/connectionStore";
import { useActiveConnection } from "@/stores/activeConnection";
import type { Connection } from "@bindings/db-lens/internal/types";

const $router = useRouter();
const drawerOpen = ref(false);
const connectionSearchInput = ref<QInput | null>(null);
async function focusConnectionSearch(event: KeyboardEvent) {
    if (event.isComposing || !event.ctrlKey || !event.shiftKey || event.altKey || event.metaKey || event.key.toLowerCase() !== "f") return;
    event.preventDefault();
    drawerOpen.value = true;
    await nextTick();
    connectionSearchInput.value?.focus();
}
onMounted(() => window.addEventListener("keydown", focusConnectionSearch));
onUnmounted(() => window.removeEventListener("keydown", focusConnectionSearch));
const $q = useQuasar();
const drawerMinWidth = 220;
const drawerMaxWidth = computed(() => Math.max(drawerMinWidth, Math.min(560,
    $q.screen.width > 1023 ? $q.screen.width - 480 : $q.screen.width - 32)));
const drawerWidthKey = "baegle.connection-sidebar-width";
function readDrawerWidth() {
    try {
        const value = Number(localStorage.getItem(drawerWidthKey));
        return Number.isFinite(value) && value >= drawerMinWidth ? value : 280;
    } catch { return 280; }
}
const preferredDrawerWidth = ref(readDrawerWidth());
const drawerWidth = computed(() => Math.round(Math.max(drawerMinWidth, Math.min(drawerMaxWidth.value, preferredDrawerWidth.value))));
const resizingDrawer = ref(false);
let resizePointerId: number | null = null;
let resizeStartX = 0;
let resizeStartWidth = 0;
function persistDrawerWidth() {
    try { localStorage.setItem(drawerWidthKey, String(drawerWidth.value)); } catch { /* Resizing still works without storage. */ }
}
function startDrawerResize(event: PointerEvent) {
    if (event.button !== 0 || resizePointerId !== null) return;
    event.preventDefault();
    const handle = event.currentTarget as HTMLElement;
    handle.focus();
    handle.setPointerCapture(event.pointerId);
    resizePointerId = event.pointerId;
    resizeStartX = event.clientX;
    resizeStartWidth = drawerWidth.value;
    resizingDrawer.value = true;
}
function resizeDrawer(event: PointerEvent) {
    if (resizePointerId !== event.pointerId) return;
    preferredDrawerWidth.value = Math.max(drawerMinWidth, Math.min(drawerMaxWidth.value,
        resizeStartWidth + event.clientX - resizeStartX));
}
function finishDrawerResize(event: PointerEvent) {
    if (resizePointerId !== event.pointerId) return;
    resizePointerId = null;
    resizingDrawer.value = false;
    const handle = event.currentTarget as HTMLElement;
    if (handle.hasPointerCapture(event.pointerId)) handle.releasePointerCapture(event.pointerId);
    persistDrawerWidth();
}
function resizeDrawerWithKeyboard(event: KeyboardEvent) {
    let width = drawerWidth.value;
    const step = event.shiftKey ? 40 : 10;
    if (event.key === "ArrowLeft") width -= step;
    else if (event.key === "ArrowRight") width += step;
    else if (event.key === "Home") width = drawerMinWidth;
    else if (event.key === "End") width = drawerMaxWidth.value;
    else return;
    event.preventDefault();
    preferredDrawerWidth.value = Math.max(drawerMinWidth, Math.min(drawerMaxWidth.value, width));
    persistDrawerWidth();
}
const actionMenuId = ref<number | null>(null);
const activeConnectionStore = useActiveConnection();
const store = useConnectionStore();
const connectionToDelete = ref<Connection | null>(null);
const deletingConnection = ref(false);
const connectionSearch = ref<string | null>("");
const filteredConnections = computed(() => {
    const query = (connectionSearch.value || "").trim().toLowerCase();
    if (!query) return store.connections;
    return store.connections.filter(connection =>
        [connection.name, connection.group, connection.host, connection.dbname, connection.driver]
            .some(value => value?.toLowerCase().includes(query)));
});
const groupedConnections = computed(() => {
    const groups = new Map<string, Connection[]>();
    for (const connection of filteredConnections.value) {
        const name = connection.group || "";
        if (!groups.has(name)) groups.set(name, []);
        groups.get(name)!.push(connection);
    }
    return [...groups].sort(([a], [b]) => a ? (b ? a.localeCompare(b) : -1) : 1)
        .map(([name, connections]) => ({ name, connections }));
});
const groupConnection = ref<Connection | null>(null);
const groupName = ref<string | null>("");
const savingGroup = ref(false);
function openGroupDialog(connection: Connection) {
    actionMenuId.value = null;
    groupConnection.value = connection;
    groupName.value = connection.group || "";
}
async function saveGroup() {
    if (!groupConnection.value || savingGroup.value) return;
    savingGroup.value = true;
    try {
        if (await store.setConnectionGroup(groupConnection.value.id, groupName.value || "")) groupConnection.value = null;
    } finally { savingGroup.value = false; }
}


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
.connection-portal { background: var(--surface-base); color: #eeeae4; }
.portal-header, .connection-card, .connection-menu { background: var(--surface-panel); color: #eeeae4; }
.portal-drawer { background: var(--surface-data); color: #eeeae4; container-type: inline-size; }
.portal-drawer-resizing, .portal-drawer-resizing * { user-select: none; cursor: col-resize !important; }
.connection-drawer-resizer { position: absolute; top: 0; right: 0; bottom: 0; width: 7px; z-index: 5; cursor: col-resize; touch-action: none; }
.connection-drawer-resizer::after { content: ''; position: absolute; top: 0; right: 0; bottom: 0; width: 2px; background: var(--border-default); transition: background 120ms; }
.connection-drawer-resizer:hover::after, .connection-drawer-resizer:focus-visible::after, .portal-drawer-resizing .connection-drawer-resizer::after { background: var(--selection-border); }
.connection-drawer-resizer:focus-visible { outline: 2px solid var(--selection-border); outline-offset: -2px; }
.connection-list { padding-right: 20px; }
.connection-group > .q-expansion-item__container > .q-item > .q-item__section--main { min-width: 0; overflow-wrap: anywhere; }
.connection-name, .connection-address { overflow-wrap: anywhere; white-space: normal; line-height: 1.4; }
@container (max-width: 300px) {
    .connection-list { padding: 12px 14px 12px 8px; }
    .connection-group > .q-expansion-item__container > .q-item { padding-left: 8px; padding-right: 8px; }
    .connection-group > .q-expansion-item__container > .q-item > .q-item__section--avatar { min-width: 28px; padding-right: 8px; }
    .saved-connection { padding: 10px 8px; }
    .connection-actions { padding-left: 4px; }
    .connection-address { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
}
.connection-card { width: 100%; max-width: 600px; border-color: var(--border-default); border-radius: 12px; }
.ssh-card { width: 560px; max-width: calc(100vw - 32px); }
.connection-options, .saved-connection { border: 1px solid var(--border-default); border-radius: 8px; }
.saved-connection { background: var(--surface-panel); width: 100%; min-width: 0; }
.saved-connection.q-item--active { border-color: #c99334; background: #423c32; }
.saved-connection .q-item__section--avatar { min-width: 32px; }
.saved-connection > .q-item__section--main { min-width: 0; overflow: hidden; }
.connection-actions { flex-shrink: 0; padding-left: 8px; }
.connection-error-message { white-space: pre-wrap; overflow-wrap: anywhere; }
.connection-tooltip { white-space: normal; overflow-wrap: anywhere; font-size: var(--sidebar-font-size); line-height: 1.5; }
.connection-note { background: #3c3933; color: #d5cec3; }
.connection-card .q-card__actions { gap: 8px; }
.connection-card .q-card__actions > .q-btn { margin-left: 0; }

.portal-header { background: var(--surface-panel); border-color: var(--border-default); }
.portal-toolbar { min-height: 68px; padding: 0 24px; gap: 10px; }
.portal-brand { padding-left: 3px; }
.portal-brand-name { color: var(--text-primary); font-size: calc(var(--ui-font-size) + 6px); line-height: 1.35; font-weight: 600; letter-spacing: -.3px; }
.portal-brand-caption { color: var(--text-muted); font-size: max(10px, calc(var(--ui-font-size) - 1px)); line-height: 1.5; }
.new-connection-button { min-height: 36px; border-radius: 7px; font-weight: 500; }
.portal-drawer { --ui-font-size: var(--sidebar-font-size); font-size: var(--sidebar-font-size); background: var(--surface-data); }
.saved-connections-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 18px; }
.saved-connections-heading > div { min-width: 0; flex: 1; display: flex; align-items: center; gap: 9px; font-size: var(--ui-font-size); color: var(--text-primary); font-weight: 600; }
.saved-connections-heading > div > span:first-child { min-width: 0; overflow-wrap: anywhere; }
.saved-connections-heading > .q-btn, .connection-count { flex-shrink: 0; }
.portal-drawer .q-field__native, .portal-drawer .q-field__label, .portal-drawer .q-field__input { font-size: var(--ui-font-size); }
.portal-drawer .q-field--dense .q-field__control, .portal-drawer .q-field--dense .q-field__marginal { min-height: max(40px, calc(var(--ui-font-size) + 24px)); height: max(40px, calc(var(--ui-font-size) + 24px)); }
.connection-count { background: var(--surface-base); color: var(--text-muted); min-width: 22px; justify-content: center; font-size: max(10px, calc(var(--ui-font-size) - 1px)); padding: 3px 6px; }
.connection-list { padding-top: 22px; }
.connection-list > .q-field .q-field__control { border-radius: 7px; background: var(--surface-base); }
.connection-list .q-field__prepend { padding-right: 8px; }
.connection-group { margin-top: 8px; }
.connection-group > .q-expansion-item__container > .q-item { padding: 10px 6px; min-height: 50px; }
.connection-group > .q-expansion-item__container > .q-item .q-item__label { font-size: var(--ui-font-size); }
.saved-connection { padding: 12px 10px; background: transparent; border: 1px solid transparent; border-radius: 8px; }
.saved-connection:hover { background: var(--surface-panel); }
.saved-connection.q-item--active { border-color: var(--accent-color); background: var(--accent-tint); }
.saved-connection-avatar { color: var(--accent-color); }
.connection-name { font-size: var(--ui-font-size); font-weight: 500; }
.connection-address { margin-top: 5px; font-size: max(10px, calc(var(--ui-font-size) - 1px)); color: var(--text-muted); }
.connections-empty-state { margin-top: 24px; padding: 24px 12px; text-align: center; color: var(--text-muted); }
.connections-empty-state .q-item__section { align-items: center; }
@media (max-width: 600px) {
    .portal-toolbar { padding: 0 12px; gap: 6px; min-height: 64px; }
    .portal-brand-caption { display: none; }
    .new-connection-button { padding: 0 10px; }
}

/* Shared visual rhythm with the workspace object browser. */
.portal-drawer .connection-list { padding: 20px 12px; }
.portal-drawer .saved-connections-heading { padding: 0 4px; margin-bottom: 14px; }
.portal-drawer .connection-group { margin-top: 12px; }
.portal-drawer .connection-group > .q-expansion-item__container > .q-item { min-height: 38px; padding: 6px 8px; border-radius: 7px; }
.portal-drawer .group-folder { min-width: 26px; padding-right: 8px; color: var(--text-muted); }
.portal-drawer .group-name { font-size: var(--ui-font-size); font-weight: 600; }
.portal-drawer .group-count { background: var(--surface-panel); color: var(--text-muted); font-size: max(10px, calc(var(--ui-font-size) - 1px)); }
.portal-drawer .connection-group .q-expansion-item__content { padding: 5px 0 0 8px; }
.portal-drawer .saved-connection { margin-bottom: 6px; padding: 12px 8px; border: 1px solid var(--border-default); background: var(--surface-panel); border-radius: 9px; }
.portal-drawer .saved-connection:hover { border-color: var(--accent-color); background: var(--surface-hover); }
.portal-drawer .saved-connection.q-item--active { border-color: var(--accent-color); background: var(--accent-tint); }
.portal-drawer .saved-connection-avatar { flex: 0 0 30px; min-width: 30px; width: 30px; padding: 0; margin-right: 10px; align-self: center; }
.portal-drawer .saved-connection-icon { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 7px; background: var(--surface-base); color: var(--accent-color); }
.portal-drawer .connection-address { display: flex; flex-wrap: wrap; gap: 5px 7px; align-items: center; line-height: 1.5; }
.saved-driver { font-size: max(9px, calc(var(--sidebar-font-size) - 2px)); padding: 1px 5px; border-radius: 4px; background: var(--surface-base); }
.saved-endpoint { min-width: 0; overflow-wrap: anywhere; }
.portal-drawer .connection-actions { align-self: center; padding-left: 2px; color: var(--text-muted); }
.portal-drawer .connection-actions > .q-btn { color: var(--text-muted) !important; }
</style>
