<template>
    <div class="h-full min-h-0 min-w-0 flex flex-col overflow-hidden bg-[var(--surface-data)]">
        <div class="shrink-0 flex items-center gap-3 px-3 py-2 border-b border-[var(--border-default)] text-[length:var(--ui-font-size)]">
            <q-select v-model="selectedStatement" :options="statementOptions" emit-value map-options dense outlined dark
                class="min-w-0 flex-1" label="Statement results" aria-label="Select SQL statement result" />
            <span class="shrink-0" :class="result.error ? 'text-red-300' : 'text-teal-300'">{{ result.completed }} / {{ result.total }} completed · {{ formatDuration(result.duration) }}</span>
        </div>
        <q-tabs v-if="queryStatements.length" v-model="selectedStatement" dense no-caps align="left" active-color="primary" indicator-color="primary" class="shrink-0 border-b border-[var(--border-default)]">
            <q-tab v-for="entry in queryStatements" :key="entry.statement" :name="entry.statement" :label="`Statement ${entry.statement}`" />
        </q-tabs>
        <template v-if="statement">
            <div class="shrink-0 px-3 py-2 text-[length:var(--ui-font-size)] border-b border-[var(--border-default)]">
                <div class="flex items-center gap-3 text-[var(--text-muted)]">
                    <span>Line {{ statement.line }} · {{ formatDuration(statement.duration) }}</span>
                    <span v-if="statement.rowsAffected != null">{{ statement.rowsAffected }} rows affected</span>
                    <q-btn flat dense no-caps size="sm" label="Go to statement" @click="emit('focusLine', statement.line)" />
                </div>
                <div class="font-mono truncate" :title="statement.preview">{{ statement.preview }}</div>
                <div v-if="statement.error" role="alert" class="text-red-300">{{ statement.error }}</div>
                <div v-else-if="!statement.results.length" class="text-teal-300">Statement completed successfully.</div>
            </div>
            <q-tabs v-if="statement.results.length > 1" v-model="selectedSet" dense no-caps align="left" class="shrink-0" active-color="primary">
                <q-tab v-for="(_, index) in statement.results" :key="index" :name="index" :label="`Result ${index + 1}`" />
            </q-tabs>
            <div v-if="currentSet?.truncated" role="status" class="shrink-0 px-3 py-2 text-amber-300 text-[length:var(--ui-font-size)]">
                Result preview limited to 1,000 rows per result and 10,000 rows per script. All statements still execute in full. Run this query separately to browse more rows.
            </div>
            <div v-if="gridResult" class="flex-1 min-h-0 min-w-0 overflow-hidden">
                <ResultGrid :key="`${selectedStatement}:${selectedSet}`" :result="gridResult" :loading="false" :error="null" />
            </div>
        </template>
    </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import type { QueryResult, QueryTab } from '@/types/queryTab';
import { formatDuration } from '@/utils/formatDuration';
import ResultGrid from './resultGrid/ResultGrid.vue';

const props = defineProps<{ result: NonNullable<QueryTab['scriptResult']> }>();
const emit = defineEmits<{ focusLine: [line: number] }>();
const selectedStatement = ref(1);
const selectedSet = ref(0);
const statements = computed(() => props.result.statements ?? []);
const queryStatements = computed(() => statements.value.filter(entry => entry.results.length > 0));
const statementOptions = computed(() => statements.value.map(entry => ({
    value: entry.statement,
    label: `Statement ${entry.statement} · line ${entry.line}${entry.error ? ' · failed' : entry.rowsAffected != null ? ` · ${entry.rowsAffected} rows affected` : ''}`,
})));
const statement = computed(() => statements.value.find(entry => entry.statement === selectedStatement.value));
const currentSet = computed(() => statement.value?.results[selectedSet.value]);
const gridResult = computed<QueryResult | null>(() => currentSet.value ? {
    Cursor: '', HasMore: false, StartRow: 1, PageSize: Math.max(1, currentSet.value.rows.length),
    IsQuery: true, CanNavigate: false, CanSort: false,
    Duration: statement.value?.duration ?? 0,
    Columns: currentSet.value.columns.map(Name => ({ Name })), Rows: currentSet.value.rows,
} : null);
watch(() => props.result, () => {
    selectedStatement.value = statements.value.find(entry => entry.error)?.statement ?? queryStatements.value[0]?.statement ?? statements.value[0]?.statement ?? 1;
    selectedSet.value = 0;
}, { immediate: true });
watch(selectedStatement, () => { selectedSet.value = 0; });
</script>
