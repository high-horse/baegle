<template>
  <q-btn flat dense no-caps size="sm" icon="history" label="History" aria-label="Query history" @click="show = true; load()" />
  <q-dialog v-model="show">
    <q-card dark class="bg-[var(--surface-panel)] text-gray-300" style="width: 850px; max-width: 95vw">
      <q-card-section class="flex items-center gap-3">
        <strong>Query history</strong><q-space />
        <q-btn flat dense icon="refresh" :loading="loading" aria-label="Refresh history" @click="load" />
        <q-btn flat dense icon="close" v-close-popup aria-label="Close history" />
      </q-card-section>
      <q-card-section class="q-pt-none">
        <q-input v-model="search" dark dense outlined clearable placeholder="Search query text or database" aria-label="Search query history" />
        <q-checkbox v-model="onlyConnection" dark dense label="Current connection only" class="q-mt-sm" />
        <div class="text-[length:var(--ui-font-size)] text-gray-400 q-mt-sm">Most recent 1,000 executions. Open restores SQL to a new console without running it.</div>
        <div v-if="error" class="text-red-300 q-mt-sm" role="alert">{{ error }}</div>
      </q-card-section>
      <q-list style="max-height: 60vh; overflow: auto">
        <q-item v-for="entry in filtered" :key="entry.id" class="border-t border-[var(--border-default)]">
          <q-item-section>
            <q-item-label caption class="text-gray-400">{{ new Date(entry.executed_at * 1000).toLocaleString() }} · {{ entry.databaseName }} · {{ entry.duration }} ms · <span :class="entry.status === 'ERROR' ? 'text-red-300' : 'text-teal-300'">{{ entry.status }}</span></q-item-label>
            <pre class="text-[length:var(--ui-font-size)] whitespace-pre-wrap break-all q-my-sm" style="max-height: 150px; overflow: auto">{{ entry.query_text }}</pre>
          </q-item-section>
          <q-item-section side><q-btn flat dense no-caps label="Open" :disable="entry.connection_id !== connectionId" @click="restore(entry.query_text)" title="Open in a new console on the current connection" /></q-item-section>
        </q-item>
        <div v-if="!loading && !filtered.length" class="q-pa-md text-gray-400">No matching queries.</div>
      </q-list>
    </q-card>
  </q-dialog>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue';
import { DbService } from '@bindings/db-lens/internal/app';
import type { QueryHistoryEntity } from '@bindings/db-lens/internal/db';
import { useConnectionStore } from '@/stores/connectionStore';
import { useQueryTabsStore } from '@/stores/queryTabsStore';
const connection = useConnectionStore();
const tabs = useQueryTabsStore();
const show = ref(false), loading = ref(false), error = ref(''), search = ref(''), onlyConnection = ref(true);
const entries = ref<QueryHistoryEntity[]>([]);
const connectionId = computed(() => String(connection.activeConnection?.id ?? ''));
const filtered = computed(() => entries.value.filter(entry => (!onlyConnection.value || entry.connection_id === connectionId.value)
  && `${entry.query_text} ${entry.databaseName}`.toLowerCase().includes((search.value || '').toLowerCase())));
async function load() {
  loading.value = true; error.value = '';
  try { entries.value = await DbService.GetQueryHistory(1000, '1970-01-01T00:00:00Z') ?? []; }
  catch (e) { error.value = String(e); }
  finally { loading.value = false; }
}
function restore(sql: string) {
  const tab = tabs.createTab(connectionId.value);
  tabs.updateSql(tab.id, sql);
  show.value = false;
}
</script>
