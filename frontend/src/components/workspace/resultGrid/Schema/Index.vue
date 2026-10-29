<template>
  <div class="h-full w-full min-w-0 min-h-0 flex overflow-hidden bg-[var(--surface-data)]">

    <!-- Sidebar -->
    <div class="w-[150px] shrink-0 h-full border-r border-[var(--border-default)]">
      <q-list>
        <q-item
          v-for="tab in tabs"
          :key="tab.name"
          clickable
          v-ripple
          :active="selectedTab === tab.name"
          @click="selectedTab = tab.name"
        >
          <q-item-section avatar>
            <q-icon :name="tab.icon" />
          </q-item-section>

          <q-item-section>
            {{ tab.label }}
          </q-item-section>
        </q-item>
      </q-list>
    </div>

    <!-- Content -->
    <div class="flex-1 min-w-0 min-h-0 h-full overflow-hidden q-pa-lg">
      <component
        :is="currentComponent"
        :result="props.result"
        :table-tab-id="props.tableTabId"
        class="h-full"
      />
    </div>

  </div>
</template>


<script setup lang="ts">
import { ref, computed } from "vue"
import type { QueryResult } from "@/types/queryTab"

import DDLTab from "./DDLTab.vue"
import SchemaTable from "./SchemaTable.vue"

const props = defineProps<{
  result: QueryResult
  tableTabId?: string
}>()

const selectedTab = ref("table")

const tabs = [
  {
    name: "table",
    label: "Table",
    icon: "table_chart",
    component: SchemaTable,
  },
  {
    name: "ddl",
    label: "DDL",
    icon: "code",
    component: DDLTab,
  },
]

const currentComponent = computed(() => {
  return tabs.find(
    tab => tab.name === selectedTab.value
  )?.component
})
</script>
