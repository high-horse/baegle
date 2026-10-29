<template>
    <div
        class="h-full w-full min-w-0 min-h-0 flex flex-col flex-nowrap bg-[var(--surface-data)] overflow-hidden"
    >

        <!-- Query info -->
        <div
            class="h-7 shrink-0 bg-[var(--surface-panel)] border-b border-[var(--border-default)] flex flex-nowrap items-center justify-between gap-2 min-w-0 overflow-hidden px-3"
        >
            <div class="flex flex-nowrap items-center gap-3 min-w-0">
                <span
                    v-if="activeTab"
                    class="text-[length:var(--ui-font-size)] font-mono text-[var(--text-muted)] truncate"
                >
                    {{ activeTab.title }}
                </span>

                <span
                    v-if="activeTab?.dirty"
                    class="text-[length:var(--ui-font-size)] text-secondary"
                >
                    Modified
                </span>
                <span v-else-if="activeTab?.savingConsole" class="text-[length:var(--ui-font-size)] text-[var(--text-muted)]">Saving…</span>
                <span v-else-if="activeTab?.savedSql !== undefined" class="text-[length:var(--ui-font-size)] text-teal-400">Autosaved</span>
            </div>

            <div
                v-if="activeTab"
                class="flex flex-nowrap shrink-0 items-center gap-2 text-[length:var(--ui-font-size)] text-[var(--text-muted)]"
            >
                <QueryHistory />
                <span v-if="isNoSQL(dbDriver)">{{ dbDriver === 'cassandra' ? 'CQL' : dbDriver === 'redis' ? 'Redis command array' : dbDriver === 'couchdb' ? 'CouchDB JSON request' : 'MongoDB JSON command' }}</span>
                <q-btn flat dense no-caps size="sm" class="query-action" icon="save" label="Save" color="primary" :loading="activeTab.savingConsole" :disable="activeTab.closing" title="Save console (Ctrl+S)" aria-label="Save console" @click="queryTabsStore.saveConsole(activeTab.id)" />
                <span>{{ activeTab.stopping ? 'Stopping…' : activeTab.runningScript ? 'Script running…' : activeTab.runningQuery ? 'Query running…' : 'Ctrl + Enter' }}</span>
                <QueryElapsedTime v-if="activeTab.runningQuery && activeTab.queryStartedAt != null" :started-at="activeTab.queryStartedAt" />

                <q-btn v-if="activeTab.runningQuery" unelevated dense no-caps color="negative" size="sm" class="query-action" icon="stop" :label="activeTab.stopping ? 'Stopping…' : 'Stop'" :disable="activeTab.stopping" aria-label="Cancel running query" @click="queryTabsStore.cancelQuery(activeTab.id)" />

                <q-btn v-if="!isNoSQL(dbDriver)" flat dense no-caps size="sm" class="query-action" icon="playlist_play" label="Run script"
                    title="Execute all statements in this console (Alt+X)" aria-label="Run SQL script"
                    :disable="activeTab.loading || !activeTab.sql.trim() || connectionStore.activeConnection?.read_only"
                    @click="executeScript" />

                <q-btn
                    unelevated
                    dense
                    no-caps
                    color="primary"
                    text-color="black"
                    size="sm"
                    class="query-action"
                    icon="play_arrow"
                    label="Run"
                    :disable="!activeTab || activeTab.loading || !activeTab.sql.trim()"
                    :loading="activeTab?.loading"
                    @click.prevent="sqlEditorRef?.execute()"
                />
            </div>
        </div>

        <div v-if="dbDriver === 'mongodb'" class="shrink-0 px-3 py-2 text-[length:var(--ui-font-size)] font-mono text-[#94a3b8] border-b border-[var(--border-default)]">
            Example: {"find":"users","filter":{},"sort":{"_id":1}} · Double-click a result document to inspect it.
        </div>
        <div v-if="dbDriver === 'couchdb'" class="shrink-0 px-3 py-2 text-[length:var(--ui-font-size)] font-mono text-[#94a3b8] border-b border-[var(--border-default)]">
            Example: {"method":"POST","path":"/_find","body":{"selector":{}}} · Paths are relative to the selected database.
        </div>
        <div v-if="dbDriver === 'redis'" class="shrink-0 px-3 py-2 text-[length:var(--ui-font-size)] font-mono text-[#94a3b8] border-b border-[var(--border-default)]">
            Examples: ["SCAN","0","MATCH","*"] · ["GET","key"] · ["HGETALL","key"]
        </div>
        <div v-if="dbDriver === 'cassandra'" class="shrink-0 px-3 py-2 text-[length:var(--ui-font-size)] font-mono text-[#94a3b8] border-b border-[var(--border-default)]">
            Run one CQL statement at a time. Results use native Cassandra paging.
        </div>
        <div v-if="activeTab?.importedScript || activeTab?.scriptResult" class="script-info shrink-0 px-3 py-2 text-[length:var(--ui-font-size)] border-b border-[var(--border-default)]" role="status">
            <template v-if="activeTab.scriptResult">
                <span :class="activeTab.scriptResult.error ? 'text-red-300' : 'text-teal-300'">
                    {{ activeTab.scriptResult.cancelled ? 'Script stopped' : activeTab.scriptResult.error ? 'Script failed' : 'Script finished' }}:
                    {{ activeTab.scriptResult.completed }} / {{ activeTab.scriptResult.total }} statements completed in {{ formatDuration(activeTab.scriptResult.duration) }}.
                </span>
                <div v-if="activeTab.scriptResult.error" class="text-red-300">{{ activeTab.scriptResult.error }}
                    <q-btn v-if="activeTab.scriptResult.errorLine" flat dense no-caps label="Go to line" @click="sqlEditorRef?.focusLine(activeTab!.scriptResult!.errorLine)" />
                </div>
            </template>
            <template v-else>SQL file loaded for review. Edit it, then use <strong>Run script</strong> (Alt+X). Run / Ctrl+Enter executes the current statement or selection.</template>
            <div class="text-[var(--text-muted)]">Script execution stops on the first error. Earlier commits remain; use explicit transactions when needed. This console saves automatically.</div>
        </div>
        <!-- Query Editor -->
        <div
            v-if="activeTab"
            class="flex-1 min-h-0 min-w-0 relative overflow-hidden"
        >
            <SqlEditor
                ref="sqlEditorRef"
                :model-value="activeTab.sql"
                :db-driver="dbDriver"
                :tables="connectionStore.activeConnectionMetadata ?? []"
                @update:model-value="handleSqlUpdate"
                @execute="executeActiveTab"
                @execute-script="executeScript"
                @open-table="emit('openTable', $event)"
            />
        </div>

        <!-- No active query -->
        <div
            v-else
            class="flex-1 min-h-0 min-w-0 flex flex-col flex-nowrap items-center justify-center gap-3 bg-[var(--surface-data)]"
        >
            <q-icon
                name="code"
                size="36px"
                class="text-[var(--text-muted)]"
            />

            <div class="text-[length:var(--ui-font-size)] text-[var(--text-muted)]">
                No query tab open
            </div>

            <q-btn
                unelevated
                color="primary"
                text-color="black"
                size="sm"
                dense
                label="New Query"
                icon="add"
                @click="createTab"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { formatDuration } from "@/utils/formatDuration";
import { useConnectionStore } from "@/stores/connectionStore";
import { useQueryTabsStore } from "@/stores/queryTabsStore";
import { isNoSQL } from '@/utils/databaseDialect';
const queryTabsStore = useQueryTabsStore();
const connectionStore = useConnectionStore();
const dbDriver = computed(() => {
 const driver = connectionStore.activeConnection?.driver;
 return driver === "mysql" || driver === "sqlite" || driver === "mongodb" || driver === "oracle" || driver === "sqlserver" || driver === "db2" || driver === "cassandra" || driver === "couchdb" || driver === "redis" ? driver : "pgx";
});
import type { QueryTab } from "@/types/queryTab";
import type { InspectTableInfo } from "@bindings/db-lens/internal/engine/entities";
import SqlEditor from "./SqlEditor.vue";
import QueryElapsedTime from "./QueryElapsedTime.vue";
import QueryHistory from "./QueryHistory.vue";

const sqlEditorRef =
    ref<InstanceType<typeof SqlEditor> | null>(null);

const props = defineProps<{
    activeTab: QueryTab | null;
    onExecute: (id: string, sql: string) => void;
}>();

const emit = defineEmits<{
    createTab: [];
    updateSql: [id: string, sql: string];
    openTable: [table: InspectTableInfo];
}>();

function createTab() {
    emit("createTab");
}

function handleSqlUpdate(sql: string) {
    if (!props.activeTab) {
        return;
    }

    emit(
        "updateSql",
        props.activeTab.id,
        sql,
    );
}

function executeActiveTab(sql: string) {
    if (!props.activeTab) {
        return;
    }

    if (!sql.trim()) {
        return;
    }

    props.onExecute(
        props.activeTab.id,
        sql,
    );
}

function executeScript() {
    if (!props.activeTab || connectionStore.activeConnection?.read_only || isNoSQL(dbDriver.value)) return;
    void queryTabsStore.executeScript(props.activeTab.id);
}
</script>

<style scoped>
.query-action {
    height: 24px;
    min-height: 24px;
    padding: 0 8px;
    line-height: 1.2;
    font-weight: 600;
}
.query-action :deep(.q-icon) { font-size: 16px; }
.query-action :deep(.q-icon.on-left) { margin-right: 4px; }
.script-info { max-height: 140px; overflow: auto; overflow-wrap: anywhere; }
</style>
