<template>
    <div
        class="h-screen w-screen flex flex-col flex-nowrap bg-[#282828] text-[#94a3b8] overflow-hidden select-text"
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
                            @move-tab="queryTabsStore.moveTab"
                        />

                        <!-- ACTIVE TAB CONTENT -->

                        <div class="flex-grow min-h-0 min-w-0 overflow-hidden">
                            <template
                                v-if="
                                    queryTabsStore.activeTab?.type === 'query'
                                "
                            >
                                <q-splitter
                                    v-model="editorHeight"
                                    horizontal
                                    :limits="[20, 80]"
                                    class="h-full min-w-0 min-h-0"
                                    :class="{ 'results-collapsed': !showResults }"
                                    :disable="!showResults"
                                >
                                    <!-- QUERY EDITOR -->
                                    <template #before>
                                        <div
                                            class="h-full min-w-0 min-h-0 overflow-hidden"
                                        >
                                            <QueryConsole
                                                :active-tab="
                                                    queryTabsStore.activeTab
                                                "
                                                :on-execute="executeQuery"
                                                @create-tab="
                                                    queryTabsStore.createTab()
                                                "
                                                @update-sql="
                                                    queryTabsStore.updateSql
                                                "
                                            />
                                        </div>
                                    </template>

                                    <!-- QUERY RESULT -->
                                    <template #after>
                                        <div
                                            class="h-full min-w-0 min-h-0 overflow-hidden"
                                        >
                                            <ResultGrid
                                        :key="queryTabsStore.activeTab.id"
                                        :can-first="!!queryTabsStore.activeTab.result && queryTabsStore.activeTab.result.StartRow > 1 && (queryTabsStore.activeTab.result.CanNavigate || !!queryTabsStore.activeTab.pages?.some(page => page.StartRow === 1))"
                                        @first="queryTabsStore.navigate(queryTabsStore.activeTab.id, 'first')"
                                        :can-last="queryTabsStore.activeTab.result?.CanNavigate ? queryTabsStore.activeTab.result.StartRow + queryTabsStore.activeTab.result.Rows.length - 1 < (queryTabsStore.activeTab.totalRows ?? 0) : (queryTabsStore.activeTab.pageIndex ?? 0) < (queryTabsStore.activeTab.pages?.length ?? 0) - 1 || !!queryTabsStore.activeTab.cursor"
                                        :total-rows="queryTabsStore.activeTab.totalRows"
                                        :sort-column="queryTabsStore.activeTab.sortColumn"
                                        :sort-direction="queryTabsStore.activeTab.sortDirection"
                                        @sort="(column, direction) => queryTabsStore.sortResult(queryTabsStore.activeTab?.id ?? '', column, direction)"
                                        :fetching-last="queryTabsStore.activeTab.fetchingLast"
                                        @stop="queryTabsStore.stopFetching(queryTabsStore.activeTab.id)"
                                        @last="queryTabsStore.navigate(queryTabsStore.activeTab.id, 'last')"
                                        :can-previous="queryTabsStore.activeTab.result?.CanNavigate ? queryTabsStore.activeTab.result.StartRow > 1 : (queryTabsStore.activeTab.pageIndex ?? 0) > 0"
                                        :can-next="queryTabsStore.activeTab.result?.CanNavigate ? queryTabsStore.activeTab.result.StartRow + queryTabsStore.activeTab.result.Rows.length - 1 < (queryTabsStore.activeTab.totalRows ?? 0) : (queryTabsStore.activeTab.pageIndex ?? 0) < (queryTabsStore.activeTab.pages?.length ?? 0) - 1 || !!queryTabsStore.activeTab.cursor"
                                        :page-error="queryTabsStore.activeTab.pageError"
                                        @refresh="executeQuery(queryTabsStore.activeTab.id, queryTabsStore.activeTab.executedSql ?? '')"
                                        @previous="queryTabsStore.navigate(queryTabsStore.activeTab.id, 'previous')"
                                        @next="queryTabsStore.navigate(queryTabsStore.activeTab.id, 'next')"
                                                :result="
                                                    queryTabsStore.activeTab.result ?? null"
                                                :loading="
                                                    queryTabsStore.activeTab
                                                        .loading
                                                "
                                                :error="
                                                    queryTabsStore.activeTab
                                                        .error
                                                "
                                                @close="hideResults"
                                            />
                                        </div>
                                    </template>
                                </q-splitter>

                            </template>

                            <!-- ================= RESULT TAB ================= -->

                            <template
                                v-else-if="
                                    queryTabsStore.activeTab?.type === 'result'
                                "
                            >
                                <div
                                    class="h-full w-full min-w-0 min-h-0 overflow-hidden"
                                >
                                    <ResultGrid
                                        :table-tab-id="queryTabsStore.activeTab.id"
                                        :key="queryTabsStore.activeTab.id"
                                        :can-first="!!queryTabsStore.activeTab.result && queryTabsStore.activeTab.result.StartRow > 1 && (queryTabsStore.activeTab.result.CanNavigate || !!queryTabsStore.activeTab.pages?.some(page => page.StartRow === 1))"
                                        @first="queryTabsStore.navigate(queryTabsStore.activeTab.id, 'first')"
                                        :can-last="queryTabsStore.activeTab.result?.CanNavigate ? queryTabsStore.activeTab.result.StartRow + queryTabsStore.activeTab.result.Rows.length - 1 < (queryTabsStore.activeTab.totalRows ?? 0) : (queryTabsStore.activeTab.pageIndex ?? 0) < (queryTabsStore.activeTab.pages?.length ?? 0) - 1 || !!queryTabsStore.activeTab.cursor"
                                        :total-rows="queryTabsStore.activeTab.totalRows"
                                        :sort-column="queryTabsStore.activeTab.sortColumn"
                                        :sort-direction="queryTabsStore.activeTab.sortDirection"
                                        @sort="(column, direction) => queryTabsStore.sortResult(queryTabsStore.activeTab?.id ?? '', column, direction)"
                                        :fetching-last="queryTabsStore.activeTab.fetchingLast"
                                        @stop="queryTabsStore.stopFetching(queryTabsStore.activeTab.id)"
                                        @last="queryTabsStore.navigate(queryTabsStore.activeTab.id, 'last')"
                                        :can-previous="queryTabsStore.activeTab.result?.CanNavigate ? queryTabsStore.activeTab.result.StartRow > 1 : (queryTabsStore.activeTab.pageIndex ?? 0) > 0"
                                        :can-next="queryTabsStore.activeTab.result?.CanNavigate ? queryTabsStore.activeTab.result.StartRow + queryTabsStore.activeTab.result.Rows.length - 1 < (queryTabsStore.activeTab.totalRows ?? 0) : (queryTabsStore.activeTab.pageIndex ?? 0) < (queryTabsStore.activeTab.pages?.length ?? 0) - 1 || !!queryTabsStore.activeTab.cursor"
                                        :page-error="queryTabsStore.activeTab.pageError"
                                        @refresh="executeQuery(queryTabsStore.activeTab.id, queryTabsStore.activeTab.executedSql ?? '')"
                                        @previous="queryTabsStore.navigate(queryTabsStore.activeTab.id, 'previous')"
                                        @next="queryTabsStore.navigate(queryTabsStore.activeTab.id, 'next')"
                                        :result="
                                            queryTabsStore.activeTab.result ??
                                            null
                                        "
                                        :loading="
                                            queryTabsStore.activeTab.loading
                                        "
                                        :error="queryTabsStore.activeTab.error"
                                        @close="
                                            queryTabsStore.closeTab(
                                                queryTabsStore.activeTab.id,
                                            )
                                        "
                                    />
                                </div>
                            </template>

                            <!-- ================= NO TAB ================= -->

                            <template v-else>
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
import { ref, watch, onMounted, onBeforeMount, onBeforeUnmount } from "vue";

import { storeToRefs } from "pinia";
import { useRouter } from "vue-router";

import { Dialog } from "quasar";
import { useTableEditsStore } from "@/stores/tableEditsStore";
import { DbService } from "@bindings/db-lens/internal/app";



import WorkspaceHeader from "@/components/workspace/WorkspaceHeader.vue";
import SchemaSidebar from "@/components/workspace/SchemaSidebar.vue";
import TableQuickOpen from "@/components/workspace/TableQuickOpen.vue";
import QueryConsole from "@/components/workspace/QueryConsole.vue";
// import ResultGrid from "@/components/workspace/ResultGrid.vue";
import ResultGrid from "@/components/workspace/resultGrid/ResultGrid.vue";

import WorkspaceTabs from "@/components/workspace/WorkspaceTabs.vue";

import { useQueryTabsStore } from "@/stores/queryTabsStore";
import { useConnectionStore } from "@/stores/connectionStore";


const $router = useRouter();

const queryTabsStore = useQueryTabsStore();
const connectionStore = useConnectionStore();
const tableEdits = useTableEditsStore();

const { activeConnection, activeConnectionMetadata } =
    storeToRefs(connectionStore);

watch(() => activeConnection.value?.driver, (driver) => {
    for (const tab of queryTabsStore.tabs) {
        if (tab.type === "query" && !tab.sql.trim()) {
            tab.title = tab.title.replace(/\.(sql|json)$/, driver === "mongodb" ? ".json" : ".sql");
        }
    }
}, { immediate: true });

const sidebarWidth = ref(20);
const editorHeight = ref(45);

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

    const tab = queryTabsStore.createResultTab(node.label);

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
      case "pgx":
        return `"${identifier.replace(/"/g, '""')}"`;
      case "mysql":
          return `\`${identifier.replace(/`/g, "``")}\``;
    default:
      return `"${identifier.replace(/"/g, '""')}"`;
  }

  
}

async function handleDisconnect(discard = false) {
    if (Object.values(tableEdits.states).some(state => state.saving)) return;
    if (tableEdits.anyPending && !discard) {
        Dialog.create({ title: "Unsaved changes", message: "Discard staged row changes and disconnect?", cancel: true, ok: "Discard" }).onOk(() => handleDisconnect(true));
        return;
    }
    const session = connectionStore.getActiveSession();

    if (session) {
        await DbService.Disconnect(String(session.id));

        connectionStore.clearActiveSession();

        $router.push({
            name: "Welcome",
        });
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
    window.addEventListener("keydown", handleCreateTabShortcut);
    window.addEventListener("keydown", handleCloseTabShortcut);
    window.addEventListener("keydown", handleToggleResultTabShortcut);
});

onBeforeUnmount(() => {
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
