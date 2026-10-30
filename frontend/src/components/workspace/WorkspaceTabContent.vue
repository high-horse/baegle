<template>
    <div class="h-full min-h-0 min-w-0 overflow-hidden">
                            <template
                                v-if="
                                    tab?.type === 'query'
                                "
                            >
                                <q-splitter
                                    v-model="editorHeight"
                                    horizontal
                                    :limits="[20, 80]"
                                    class="h-full min-w-0 min-h-0"
                                    :class="{ 'results-collapsed': !showResults && !tab.runningScript && !tab.scriptResult }"
                                    :disable="!showResults && !tab.runningScript && !tab.scriptResult"
                                >
                                    <!-- QUERY EDITOR -->
                                    <template #before>
                                        <div
                                            class="h-full min-w-0 min-h-0 overflow-hidden"
                                        >
                                            <QueryConsole
                                                ref="queryConsoleRef"
                                                :active-tab="
                                                    tab
                                                "
                                                :on-execute="executeQuery"
                                                @open-table="table => handleTableSelect({ id: `${table.schema || 'public'}.${table.name}`, label: table.name, type: table.type.toUpperCase() === 'VIEW' ? 'view' : 'table', schema: table.schema || 'public', database: table.database })"
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
                                            <ScriptResults v-if="tab.scriptResult?.statements?.length" :result="tab.scriptResult" @focus-line="line => queryConsoleRef?.focusLine(line)" />
                                            <ResultGrid v-else
                                        :query-started-at="tab.queryStartedAt"
                                        :key="tab.id"
                                        :can-first="!!tab.result && tab.result.StartRow > 1 && (tab.result.CanNavigate || !!tab.pages?.some(page => page.StartRow === 1))"
                                        @first="queryTabsStore.navigate(tab.id, 'first')"
                                        :can-last="tab.result?.CanNavigate ? (tab.totalRows == null || tab.result.StartRow + tab.result.Rows.length - 1 < tab.totalRows) : (tab.pageIndex ?? 0) < (tab.pages?.length ?? 0) - 1 || !!tab.cursor"
                                        :total-rows="tab.totalRows"
                                        :counting-rows="tab.countingRows"
                                        :stopping-count="tab.stoppingCount"
                                        :count-error="tab.countError"
                                        @count="queryTabsStore.countRows(tab.id)"
                                        @cancel-count="queryTabsStore.cancelRowCount(tab.id)"
                                        :sort-column="tab.sortColumn"
                                        :result-where="tab.resultWhere"
                                        :result-order="tab.resultOrder"
                                        @expressions="(where, order, sql) => queryTabsStore.applyResultExpressions(tab?.id ?? '', where, order, sql)"
                                        :sort-direction="tab.sortDirection"
                                        @sort="(column, direction) => queryTabsStore.sortResult(tab?.id ?? '', column, direction)"
                                        :fetching-last="tab.fetchingLast"
                                        @stop="queryTabsStore.stopFetching(tab.id)"
                                        @last="queryTabsStore.navigate(tab.id, 'last')"
                                        :can-previous="tab.result?.CanNavigate ? tab.result.StartRow > 1 : (tab.pageIndex ?? 0) > 0"
                                        :can-next="tab.result?.CanNavigate ? tab.result.HasMore : (tab.pageIndex ?? 0) < (tab.pages?.length ?? 0) - 1 || !!tab.cursor"
                                        :page-error="tab.pageError"
                                        @refresh="executeQuery(tab.id, tab.executedSql ?? '')"
                                        @previous="queryTabsStore.navigate(tab.id, 'previous')"
                                        @next="queryTabsStore.navigate(tab.id, 'next')"
                                                :result="
                                                    tab.result ?? null"
                                                :loading="
                                                    tab
                                                        .loading
                                                "
                                                :error="
                                                    tab
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
                                    tab?.type === 'result'
                                "
                            >
                                <div
                                    class="h-full w-full min-w-0 min-h-0 overflow-hidden"
                                >
                                    <ResultGrid
                                        :query-started-at="tab.queryStartedAt"
                                        :table-tab-id="tab.id"
                                        :key="tab.id"
                                        :can-first="!!tab.result && tab.result.StartRow > 1 && (tab.result.CanNavigate || !!tab.pages?.some(page => page.StartRow === 1))"
                                        @first="queryTabsStore.navigate(tab.id, 'first')"
                                        :can-last="tab.result?.CanNavigate ? (tab.totalRows == null || tab.result.StartRow + tab.result.Rows.length - 1 < tab.totalRows) : (tab.pageIndex ?? 0) < (tab.pages?.length ?? 0) - 1 || !!tab.cursor"
                                        :total-rows="tab.totalRows"
                                        :counting-rows="tab.countingRows"
                                        :stopping-count="tab.stoppingCount"
                                        :count-error="tab.countError"
                                        @count="queryTabsStore.countRows(tab.id)"
                                        @cancel-count="queryTabsStore.cancelRowCount(tab.id)"
                                        :sort-column="tab.sortColumn"
                                        :result-where="tab.resultWhere"
                                        :result-order="tab.resultOrder"
                                        @expressions="(where, order, sql) => queryTabsStore.applyResultExpressions(tab?.id ?? '', where, order, sql)"
                                        :sort-direction="tab.sortDirection"
                                        @sort="(column, direction) => queryTabsStore.sortResult(tab?.id ?? '', column, direction)"
                                        :fetching-last="tab.fetchingLast"
                                        @stop="queryTabsStore.stopFetching(tab.id)"
                                        @last="queryTabsStore.navigate(tab.id, 'last')"
                                        :can-previous="tab.result?.CanNavigate ? tab.result.StartRow > 1 : (tab.pageIndex ?? 0) > 0"
                                        :can-next="tab.result?.CanNavigate ? tab.result.HasMore : (tab.pageIndex ?? 0) < (tab.pages?.length ?? 0) - 1 || !!tab.cursor"
                                        :page-error="tab.pageError"
                                        @refresh="executeQuery(tab.id, tab.executedSql ?? '')"
                                        @previous="queryTabsStore.navigate(tab.id, 'previous')"
                                        @next="queryTabsStore.navigate(tab.id, 'next')"
                                        :result="
                                            tab.result ??
                                            null
                                        "
                                        :loading="
                                            tab.loading
                                        "
                                        :error="tab.error"
                                        @close="
                                            queryTabsStore.closeTab(
                                                tab.id,
                                            )
                                        "
                                    />
                                </div>
                            </template>

    </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import type { QueryTab } from "@/types/queryTab";
import { useQueryTabsStore } from "@/stores/queryTabsStore";
import QueryConsole from "./QueryConsole.vue";
import ResultGrid from "./resultGrid/ResultGrid.vue";
import ScriptResults from "./ScriptResults.vue";

defineProps<{ tab: QueryTab; showResults: boolean }>();
const emit = defineEmits<{
    execute: [id: string, sql: string];
    hideResults: [];
    openTable: [node: { id: string; label: string; type: string; schema: string; database: string }];
}>();
const queryTabsStore = useQueryTabsStore();
const queryConsoleRef = ref<InstanceType<typeof QueryConsole> | null>(null);
const editorHeight = ref(45);
function executeQuery(id: string, sql: string) { emit("execute", id, sql); }
function hideResults() { emit("hideResults"); }
function handleTableSelect(node: { id: string; label: string; type: string; schema: string; database: string }) {
    emit("openTable", node);
}
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
