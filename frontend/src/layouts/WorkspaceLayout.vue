<template>
    <div
        class="workspace-layout fixed inset-0 flex flex-col flex-nowrap bg-[#282828] text-[#94a3b8] overflow-hidden select-text"
    >
        <!-- select-none -->
        <WorkspaceHeader @disconnect="handleDisconnect(false)" />
        <TableQuickOpen @selectTable="handleTableSelect" />

        <div class="flex-1 flex flex-nowrap relative min-w-0 min-h-0 overflow-hidden">
            <q-splitter
                v-model="sidebarWidth"
                :limits="[15, 35]"
                class="absolute-full min-w-0 min-h-0"
            >
                <!-- LEFT SIDEBAR -->
                <template #before>
                    <div class="h-full min-w-0 overflow-hidden">
                        <SchemaSidebar @selectTable="handleTableSelect" />
                    </div>
                </template>

                <!-- MAIN -->
                <template #after>
                    <div
                        class="h-full min-w-0 min-h-0 overflow-hidden flex flex-col flex-nowrap"
                    >
                        <WorkspaceTabs
                            :tabs="queryTabsStore.tabs"
                            :active-tab-id="queryTabsStore.activeTabId"
                            @create-tab="queryTabsStore.createTab()"
                            @select-tab="queryTabsStore.selectTab"
                            @close-tab="queryTabsStore.closeTab"
                            :closing-tabs="queryTabsStore.closingTabs"
                            @close-tabs="queryTabsStore.closeTabs"
                            @move-tab="queryTabsStore.moveTab"
                        />

                        <!-- ACTIVE TAB CONTENT -->

                        <div ref="tabContentHost" tabindex="-1" class="flex-grow min-h-0 min-w-0 overflow-hidden outline-none">
                            <KeepAlive :max="8">
                                <WorkspaceTabContent
                                    v-if="queryTabsStore.activeTab"
                                    :key="queryTabsStore.activeTab.id"
                                    :tab="queryTabsStore.activeTab"
                                    :show-results="showResults"
                                    @execute="executeQuery"
                                    @hide-results="hideResults"
                                    @open-table="handleTableSelect"
                                />
                            </KeepAlive>

                            <!-- ================= NO TAB ================= -->

                            <template v-if="!queryTabsStore.activeTab">
                                <div
                                    class="h-full flex flex-col items-center justify-center gap-3"
                                >
                                    <q-icon
                                        name="tab"
                                        size="36px"
                                        class="text-[#909090]"
                                    />

                                    <span class="text-xs text-[#a0a0a0]">
                                        No tab selected
                                    </span>

                                    <q-btn
                                        unelevated
                                        color="amber"
                                        text-color="black"
                                        size="sm"
                                        label="New Query"
                                        icon="add"
                                        dense
                                        @click="queryTabsStore.createTab()"
                                    />
                                </div>
                            </template>
                        </div>
                    </div>
                </template>
            </q-splitter>
        </div>
    </div>
</template>

<script setup lang="ts">
import { consoleExtension } from "@/utils/databaseDialect";
import { nextTick, ref, watch, onMounted, onBeforeMount, onBeforeUnmount } from "vue";

import { storeToRefs } from "pinia";
import { useRouter } from "vue-router";

import { Dialog, Notify } from "quasar";
import { useTableEditsStore } from "@/stores/tableEditsStore";
import { DbService } from "@bindings/db-lens/internal/app";



import WorkspaceHeader from "@/components/workspace/WorkspaceHeader.vue";
import SchemaSidebar from "@/components/workspace/SchemaSidebar.vue";
import TableQuickOpen from "@/components/workspace/TableQuickOpen.vue";
import WorkspaceTabContent from "@/components/workspace/WorkspaceTabContent.vue";

import WorkspaceTabs from "@/components/workspace/WorkspaceTabs.vue";

import { useQueryTabsStore } from "@/stores/queryTabsStore";
import { useConnectionStore } from "@/stores/connectionStore";


const $router = useRouter();

const queryTabsStore = useQueryTabsStore();
const tabContentHost = ref<HTMLElement | null>(null);
const connectionStore = useConnectionStore();
const tableEdits = useTableEditsStore();

const { activeConnection, activeConnectionMetadata } =
    storeToRefs(connectionStore);

watch(() => activeConnection.value?.id, id => {
    if (id !== undefined) void queryTabsStore.restoreConsoles(String(id));
}, { immediate: true });

watch(() => activeConnection.value?.driver, (driver) => {
    for (const tab of queryTabsStore.tabs) {
        if (tab.type === "query" && !tab.sql.trim()) {
            tab.title = tab.title.replace(/\.(sql|json|cql)$/, `.${consoleExtension(driver)}`);
        }
    }
}, { immediate: true });

const sidebarWidth = ref(20);

const showResults = ref(true);

function hideResults() {
    showResults.value = false;
}

async function executeQuery(id: string, sql: string) {
    showResults.value = true;
    await queryTabsStore.execute(id, sql);
}


async function handleTableSelect(node: {
    id: string;
    label: string;
    type?: string;
    schema?: string;
    database?: string;
}) {
    if (node.type !== "table" && node.type !== "view" && node.type !== "collection") {
        return;
    }

    const tab = queryTabsStore.createResultTab(node.label, undefined, {
        name: node.label, schema: node.schema ?? "", database: node.database ?? activeConnection.value?.dbname ?? "",
    });

    await executeTableResult(tab.id, node);
    if (activeConnection.value) {
        await tableEdits.load(tab.id, { connectionId: await DbService.GetActiveConnection(), name: node.label,
            schema: node.schema ?? "", database: node.database ?? activeConnection.value.dbname });
    }
}

async function executeTableResult(
    id: string,
    node: {
        id: string;
        label: string;
        type?: string;
        schema?: string;
    },
) {
    if (["couchdb", "redis"].includes(activeConnection.value?.driver ?? "")) {
        const command = JSON.stringify(activeConnection.value?.driver === "redis" ? ["SCAN", "0", "MATCH", "*"] : { method: "GET", path: "/_all_docs?include_docs=true" }, null, 2);
        queryTabsStore.updateSql(id, command);
        await executeQuery(id, command);
        return;
    }
    if (activeConnection.value?.driver === "mongodb") {
        const command = JSON.stringify({ find: node.label, filter: {} }, null, 2);
        queryTabsStore.updateSql(id, command);
        await executeQuery(id, command);
        return;
    }
    const tableName = [node.schema, node.label].filter((part): part is string => !!part)
        .map(part => quoteIdentifier(part, activeConnection.value?.driver)).join(".");
    const sql = `SELECT * FROM ${tableName};`;
    queryTabsStore.updateSql(id, sql);
    await executeQuery(id, sql);
}

function quoteIdentifier(identifier: string, driver: string = "pgx"): string {

  switch (driver) {
      case "sqlserver":
        return `[${identifier.replace(/]/g, ']]')}]`;
      case "pgx":
        return `"${identifier.replace(/"/g, '""')}"`;
      case "mysql":
          return `\`${identifier.replace(/`/g, "``")}\``;
    default:
      return `"${identifier.replace(/"/g, '""')}"`;
  }

  
}

let disconnecting = false;

watch(() => queryTabsStore.activeTab, tab => {
    if (!tab?.needsTableRestore || !tab.tableRef) return;
    tab.needsTableRestore = false;
    const table = tab.tableRef;
    void executeTableResult(tab.id, { id: tab.id, label: table.name, schema: table.schema });
    const connection = activeConnection.value?.id;
    void DbService.GetActiveConnection().then(connectionId => {
        if (activeConnection.value?.id === connection && queryTabsStore.tabs.includes(tab)) {
            return tableEdits.load(tab.id, { ...table, connectionId });
        }
    }).catch(error => Notify.create({ type: "negative", message: `Could not restore table details: ${String(error)}` }));
}, { immediate: true });

function flushWorkspace() { void queryTabsStore.flushAutosave(); }

async function handleDisconnect(discard = false) {
    if (disconnecting) return;
    if (Object.values(tableEdits.states).some(state => state.saving)) return;
    if (tableEdits.anyPending && !discard) {
        Dialog.create({ title: "Unsaved changes", message: "Discard staged row changes and disconnect?", cancel: true, ok: "Discard" }).onOk(() => handleDisconnect(true));
        return;
    }
    const session = connectionStore.getActiveSession();

    if (session) {
        disconnecting = true;
        try {
            await queryTabsStore.flushAutosave();
            await DbService.Disconnect(String(session.id));
            connectionStore.clearActiveSession();
            await $router.push({ name: "Welcome" });
        } catch (error) {
            Notify.create({ type: "negative", message: `Failed to disconnect: ${String(error)}` });
        } finally {
            disconnecting = false;
        }
    }
}

let lastEscapeAt: number | undefined;

function resetEscapeShortcut() {
    lastEscapeAt = undefined;
}

function handleDisconnectShortcut(event: KeyboardEvent) {
    if (event.key !== "Escape" || event.ctrlKey || event.metaKey || event.altKey || event.shiftKey || event.isComposing) {
        resetEscapeShortcut();
        return;
    }
    // Escape should dismiss dialogs, menus and editor completions normally.
    if (event.repeat || document.querySelector(".q-dialog, .q-menu, .cm-tooltip-autocomplete")) {
        resetEscapeShortcut();
        return;
    }
    const now = performance.now();
    if (lastEscapeAt !== undefined && now - lastEscapeAt <= 500) {
        resetEscapeShortcut();
        event.preventDefault();
        event.stopPropagation();
        void handleDisconnect();
    } else {
        lastEscapeAt = now;
    }
}

async function setActiveSession() {
    try {
        const [session, exist] = await DbService.GetActiveConnectionObject();

        if (exist && session) {
            connectionStore.setActiveSession(session);

            if (!activeConnectionMetadata.value) {
                connectionStore.setActiveConnectionMetadata();
            }
        }
    } catch (error) {
        console.error("failed to set active session", error);
    }
}

function handleCreateTabShortcut(event: KeyboardEvent) {
    if (event.ctrlKey && event.key.toLowerCase() === "t") {
        event.preventDefault();

        queryTabsStore.createTab();
    }
}

function handleSaveConsoleShortcut(event: KeyboardEvent) {
    if ((event.ctrlKey || event.metaKey) && !event.shiftKey && !event.altKey && !event.isComposing && event.key.toLowerCase() === "s") {
        event.preventDefault();
        event.stopPropagation();
        if (!event.repeat && queryTabsStore.activeTab?.type === "query") {
            void queryTabsStore.saveConsole(queryTabsStore.activeTab.id);
        }
    }
}

function handleSwitchTabShortcut(event: KeyboardEvent) {
    if (!(event.code === "Tab" || event.key === "Tab" || event.key === "ISO_Left_Tab") || !event.ctrlKey || event.altKey || event.metaKey || event.isComposing) return;
    // Only an open modal blocks workspace navigation; menus may be transient.
    if (document.querySelector('.q-dialog[aria-modal="true"]')) return;
    const tabs = queryTabsStore.tabs;
    if (!tabs.length) return;
    event.preventDefault();
    event.stopPropagation();
    const current = tabs.findIndex(tab => tab.id === queryTabsStore.activeTabId);
    const next = current < 0
        ? (event.shiftKey ? tabs.length - 1 : 0)
        : (current + (event.shiftKey ? -1 : 1) + tabs.length) % tabs.length;
    // Move focus out of the outgoing cached view before Vue detaches it.
    // The host survives switches, keeping keyboard events inside the webview.
    tabContentHost.value?.focus({ preventScroll: true });
    queryTabsStore.selectTab(tabs[next].id);
    void nextTick(() => {
        const host = tabContentHost.value;
        if (!host || document.activeElement !== host) return;
        const editor = host.querySelector<HTMLElement>(".cm-content[contenteditable='true']");
        editor?.focus({ preventScroll: true });
    });
}

function handleRefreshTableShortcut(event: KeyboardEvent) {
    if (!event.ctrlKey || event.shiftKey || event.altKey || event.metaKey || event.isComposing || event.key.toLowerCase() !== "r") return;
    const tab = queryTabsStore.activeTab;
    if (tab?.type !== "result") return;
    // Consume reload even when refresh is blocked, preserving the workspace.
    event.preventDefault();
    event.stopPropagation();
    if (event.repeat || tab.loading || tab.closing || tableEdits.hasPending(tab.id) || tableEdits.isSaving(tab.id)
        || document.querySelector('.q-dialog[aria-modal="true"]')) return;
    void executeQuery(tab.id, tab.executedSql ?? tab.sql);
}

function handleCloseTabShortcut(event: KeyboardEvent) {
    if (event.ctrlKey && event.key.toLowerCase() === "w") {
        event.preventDefault();

        if (queryTabsStore.activeTabId) {
            queryTabsStore.closeTab(queryTabsStore.activeTabId);
        }
    }
}

function handleToggleResultTabShortcut(event: KeyboardEvent) {
    if (event.ctrlKey && event.key.toLowerCase() === "j") {
        event.preventDefault();

        if (queryTabsStore.activeTab?.type === "query") {
            showResults.value = !showResults.value;
        }
    }
}

onBeforeMount(() => {
    window.addEventListener("blur", flushWorkspace);
    window.addEventListener("pagehide", flushWorkspace);
    window.addEventListener("keydown", handleDisconnectShortcut, true);
    window.addEventListener("blur", resetEscapeShortcut);
    window.addEventListener("keydown", handleSaveConsoleShortcut, true);
    window.addEventListener("keydown", handleSwitchTabShortcut, true);
    window.addEventListener("keydown", handleRefreshTableShortcut, true);
    window.addEventListener("keydown", handleCreateTabShortcut);
    window.addEventListener("keydown", handleCloseTabShortcut);
    window.addEventListener("keydown", handleToggleResultTabShortcut);
});

onBeforeUnmount(() => {
    window.removeEventListener("blur", flushWorkspace);
    window.removeEventListener("pagehide", flushWorkspace);
    window.removeEventListener("keydown", handleDisconnectShortcut, true);
    window.removeEventListener("blur", resetEscapeShortcut);
    resetEscapeShortcut();
    window.removeEventListener("keydown", handleSaveConsoleShortcut, true);
    window.removeEventListener("keydown", handleSwitchTabShortcut, true);
    window.removeEventListener("keydown", handleRefreshTableShortcut, true);
    window.removeEventListener("keydown", handleCreateTabShortcut);
    window.removeEventListener("keydown", handleCloseTabShortcut);
    window.removeEventListener("keydown", handleToggleResultTabShortcut);
  
    queryTabsStore.clearResults();
    connectionStore.resetStore();
});

onMounted(() => {
    if (!activeConnection.value) {
        setActiveSession();
    }
});
</script>

<style scoped>
.workspace-layout :deep(.cm-scroller),
.workspace-layout :deep(.q-scrollarea__container),
.workspace-layout :deep(.q-table__middle) {
    overscroll-behavior: contain;
}

.results-collapsed > :deep(.q-splitter__before) {
    height: 100% !important;
}

.results-collapsed > :deep(.q-splitter__after),
.results-collapsed > :deep(.q-splitter__separator) {
    display: none;
}

:deep(.q-splitter) {
    min-width: 0;
    min-height: 0;
}

:deep(.q-splitter__panel) {
    min-width: 0;
    min-height: 0;
    overflow: hidden;
}
</style>
