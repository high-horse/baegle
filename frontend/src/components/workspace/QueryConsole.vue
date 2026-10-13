<template>
    <div
        class="h-full w-full min-w-0 min-h-0 flex flex-col flex-nowrap bg-[#343638] overflow-hidden"
    >

        <!-- Query info -->
        <div
            class="h-7 shrink-0 bg-[#303234] border-b border-[#484848] flex flex-nowrap items-center justify-between gap-2 min-w-0 overflow-hidden px-3"
        >
            <div class="flex flex-nowrap items-center gap-3 min-w-0">
                <span
                    v-if="activeTab"
                    class="text-[10px] font-mono text-[#a0a0a0] truncate"
                >
                    {{ activeTab.title }}
                </span>

                <span
                    v-if="activeTab?.dirty"
                    class="text-[10px] text-amber-500"
                >
                    Modified
                </span>
            </div>

            <div
                v-if="activeTab"
                class="flex flex-nowrap shrink-0 items-center gap-2 text-[10px] text-[#909090]"
            >
                <span v-if="dbDriver === 'mongodb'">MongoDB JSON command</span>
                <span>{{ activeTab.stopping ? 'Stopping query…' : activeTab.runningQuery ? 'Query running…' : 'Ctrl + Enter' }}</span>
                <QueryElapsedTime v-if="activeTab.runningQuery && activeTab.queryStartedAt != null" :started-at="activeTab.queryStartedAt" />

                <q-btn v-if="activeTab.runningQuery" unelevated dense no-caps color="negative" size="sm" class="query-action" icon="stop" :label="activeTab.stopping ? 'Stopping…' : 'Stop'" :disable="activeTab.stopping" aria-label="Cancel running query" @click="queryTabsStore.cancelQuery(activeTab.id)" />

                <q-btn
                    unelevated
                    dense
                    no-caps
                    color="amber"
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

        <div v-if="dbDriver === 'mongodb'" class="shrink-0 px-3 py-2 text-[10px] font-mono text-[#94a3b8] border-b border-[#484848]">
            Example: {"find":"users","filter":{},"sort":{"_id":1}} · Double-click a result document to inspect it.
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
                @open-table="emit('openTable', $event)"
            />
        </div>

        <!-- No active query -->
        <div
            v-else
            class="flex-1 min-h-0 min-w-0 flex flex-col flex-nowrap items-center justify-center gap-3 bg-[#343638]"
        >
            <q-icon
                name="code"
                size="36px"
                class="text-[#909090]"
            />

            <div class="text-xs text-[#a0a0a0]">
                No query tab open
            </div>

            <q-btn
                unelevated
                color="amber"
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
import { useConnectionStore } from "@/stores/connectionStore";
import { useQueryTabsStore } from "@/stores/queryTabsStore";
const queryTabsStore = useQueryTabsStore();
const connectionStore = useConnectionStore();
const dbDriver = computed(() => {
 const driver = connectionStore.activeConnection?.driver;
 return driver === "mysql" || driver === "sqlite" || driver === "mongodb" ? driver : "pgx";
});
import type { QueryTab } from "@/types/queryTab";
import type { InspectTableInfo } from "@bindings/db-lens/internal/engine/entities";
import SqlEditor from "./SqlEditor.vue";
import QueryElapsedTime from "./QueryElapsedTime.vue";

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
</style>
