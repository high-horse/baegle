<template>
  <q-item v-if="connection.activeConnection?.driver !== 'mongodb'" clickable :disable="disabled || loading" @click="load">
    <q-item-section>{{ loading ? 'Loading relations…' : 'Related records…' }}</q-item-section>
  </q-item>
  <q-dialog v-model="show">
    <q-card dark class="bg-[#303234] text-gray-300" style="min-width: 380px; max-width: 90vw">
      <q-card-section class="flex items-center"><strong>Related records · {{ column }}</strong><q-space /><q-btn flat dense icon="close" v-close-popup aria-label="Close related records" /></q-card-section>
      <q-card-section v-if="error" class="text-red-300" role="alert">{{ error }}</q-card-section>
      <q-list v-if="relations.length" style="max-height: 60vh; overflow: auto">
        <q-item v-for="(relation, index) in relations" :key="index" clickable :disable="relation.columns.some(name => row[name] == null)" @click="open(relation)">
          <q-item-section><q-item-label>{{ relation.direction }} → {{ relation.target.schema || relation.target.database }}.{{ relation.target.name }}</q-item-label>
            <q-item-label caption class="text-gray-400">{{ relation.columns.join(', ') }} → {{ relation.targetColumns.join(', ') }}</q-item-label>
          </q-item-section>
        </q-item>
      </q-list>
      <q-card-section v-else-if="!error">No declared foreign keys involve this column.</q-card-section>
      <q-card-section class="text-xs text-gray-400">Opens a SELECT in a new console. NULL foreign keys have no related record.</q-card-section>
    </q-card>
  </q-dialog>
</template>
<script setup lang="ts">
import { ref } from 'vue';
import { DbService } from '@bindings/db-lens/internal/app';
import { useConnectionStore } from '@/stores/connectionStore';
import { useQueryTabsStore } from '@/stores/queryTabsStore';
import { relatedQuery } from '@/utils/relatedQuery';
type BackendRelation = NonNullable<Awaited<ReturnType<typeof DbService.InspectTableRelations>>>[number];
type Relation = Omit<BackendRelation, 'columns' | 'targetColumns'> & { columns: string[]; targetColumns: string[] };
const props = defineProps<{ tabId: string; column: string; row: Record<string, unknown>; disabled: boolean }>();
const connection = useConnectionStore(), tabs = useQueryTabsStore();
const show = ref(false), loading = ref(false), error = ref('');
const relations = ref<Relation[]>([]);
async function load() {
  const tab = tabs.tabs.find(t => t.id === props.tabId);
  show.value = true; error.value = ''; relations.value = [];
  if (!tab?.tableRef || tab.connectionId !== String(connection.activeConnection?.id)) { error.value = 'Reconnect to the table’s connection first.'; return; }
  loading.value = true;
  try { relations.value = (await DbService.InspectTableRelations({ ...tab.tableRef, connectionId: tab.connectionId }) ?? [])
    .map(r => ({ ...r, columns: r.columns ?? [], targetColumns: r.targetColumns ?? [] }))
    .filter(r => r.columns.includes(props.column)); }
  catch (e) { error.value = String(e); }
  finally { loading.value = false; }
}
function open(relation: Relation) {
  try {
    if (relation.target.connectionId !== String(connection.activeConnection?.id)) throw new Error('The active connection changed. Reopen related records.');
    const sql = relatedQuery(relation, props.row, connection.activeConnection!.driver);
    const tab = tabs.createTab(relation.target.connectionId);
    tabs.updateSql(tab.id, sql);
    void tabs.execute(tab.id, sql);
    show.value = false;
  } catch (e) { error.value = String(e); }
}
</script>
