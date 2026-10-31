<template>
    <aside class="schema-sidebar">
        <div class="schema-connection">
            <div class="connection-summary">
                <div class="connection-engine"><q-icon name="dns" size="20px" /></div>
                <div class="connection-summary-text">
                    <div class="connection-eyebrow">CONNECTION</div>
                    <div class="active-connection-name" :title="activeConnection?.name">{{ activeConnection?.name }}</div>
                </div>
                <q-btn flat dense round icon="refresh" class="schema-refresh" aria-label="Refresh database objects" :loading="loading" @click="refreshSchema">
                    <q-tooltip>Refresh database objects</q-tooltip>
                </q-btn>
            </div>
            <div class="connection-details">
                <span class="driver-badge">{{ activeConnection?.driver }}</span>
                <span class="connection-endpoint" :title="connectionAddress">{{ connectionAddress }}</span>
            </div>
        </div>
        <div class="schema-search">
            <q-input ref="searchInput" v-model="searchTerm" placeholder="Find tables or collections…" aria-label="Search database objects" aria-keyshortcuts="Control+Shift+F" dense outlined hide-bottom-space @keyup.enter="handleSearch">
                <template #prepend><q-icon name="search" size="18px" /></template>
                <q-tooltip :delay="600">Press Enter to search · Ctrl+Shift+F to focus</q-tooltip>
            </q-input>
        </div>
        <div class="schema-section-heading"><span>Database objects</span><q-badge rounded class="object-count">{{ tableCount + viewCount }}</q-badge></div>
        <q-scroll-area class="schema-tree-scroll">
            <q-tree v-model:expanded="expanded" :nodes="schemaNodes" node-key="id" dark no-connectors dense default-expand-all class="compact-tree">
                <template #default-header="prop">
                    <div class="schema-node" :class="{ 'schema-node-group': prop.node.type === 'schema' }" @dblclick="handleNodeDoubleClick(prop.node)">
                        <q-icon :name="prop.node.icon" :color="prop.node.type === 'view' ? 'secondary' : undefined" size="17px" />
                        <span class="schema-node-label" :title="prop.node.label">{{ prop.node.label }}</span>
                        <span v-if="prop.node.children" class="schema-node-count">{{ prop.node.children.length }}</span>
                        <q-icon v-if="prop.node.type === 'view'" name="visibility" size="13px" class="view-marker"><q-tooltip>View</q-tooltip></q-icon>
                    </div>
                </template>
            </q-tree>
            <div v-if="!schemaNodes.length && !loading" class="schema-empty">
                <q-icon name="search_off" size="28px" />
                <span>No {{ activeConnection?.driver === 'mongodb' ? 'collections' : 'tables' }} found</span>
            </div>
        </q-scroll-area>
        <div class="schema-footer">
            <span>{{ tableCount }} {{ activeConnection?.driver === 'mongodb' ? 'collections' : 'tables' }}</span>
            <span v-if="viewCount">{{ viewCount }} views</span>
        </div>
    </aside>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount, watch } from "vue";
import type { QInput } from "quasar";
import { storeToRefs } from "pinia";
import { useConnectionStore } from "@/stores/connectionStore";

const connectionStore = useConnectionStore();
const expanded = ref<string[]>([]);
const searchQuery = ref("");
const searchInput = ref<QInput | null>(null);

const {
    activeConnectionMetadata,
    activeConnection,
    searchTerm
} = storeToRefs(connectionStore);

const connectionAddress = computed(() => {
    const connection = activeConnection.value;
    if (!connection) return "";
    if (/^mongodb(?:\+srv)?:\/\//.test(connection.host)) {
        try { return new URL(connection.host).host; } catch { return "MongoDB"; }
    }
    // A MongoDB URI returned by the backend has already been reduced to its hosts.
    if (connection.driver === "mongodb" && (connection.host.includes(":") || connection.host.includes(","))) {
        return connection.host;
    }
    return connection.port?.Valid ? `${connection.host}:${connection.port.Int64}` : connection.host;
});

interface SchemaNode {
    id: string;
    label: string;
    icon: string;
    iconColor: string;
    type?: string;
    schema?: string;
    database?: string;
    children?: SchemaNode[];
}

const emit = defineEmits<{
    refresh: [];
    selectTable: [node: SchemaNode];
}>();

const loading = ref(false);

/*
 * Build the tree directly from activeConnectionMetadata.
 *
 * Backend data looks like:
 *
 * {
 *   name: "actor",
 *   type: "TABLE",
 *   database: "dvdrental",
 *   schema: "public",
 *   rows: 200
 * }
 */
 const schemaNodes = computed<SchemaNode[]>(() => {
     const metadata = activeConnectionMetadata.value;
 
     if (!metadata || !Array.isArray(metadata)) {
         return [];
     }
 
     const query = searchQuery.value.trim().toLowerCase();
 
     // Filter table/view names using %query%
     const filteredMetadata = query
         ? metadata.filter((item) =>
               String(item.name ?? "")
                   .toLowerCase()
                   .includes(query)
           )
         : metadata;
 
     // Group tables/views by schema
     const schemas = new Map<string, any[]>();
 
     filteredMetadata.forEach((item) => {
         const schema = item.schema || "public";
 
         if (!schemas.has(schema)) {
             schemas.set(schema, []);
         }
 
         schemas.get(schema)!.push(item);
     });
 
     return Array.from(schemas.entries()).map(([schema, objects]) => ({
         id: `schema-${schema}`,
         label: schema,
         icon: "schema",
         iconColor: "primary",
         type: "schema",
 
         children: objects.map((item) => {
             const isView = item.type === "VIEW";
 
             return {
                 id: `${schema}.${item.name}`,
                 schema,
                 database: item.database,
                 label: item.name,
                 icon: item.type === "COLLECTION" ? "data_object" : isView ? "view_list" : "table_chart",
                 iconColor: isView ? "secondary" : "primary",
                 type: item.type === "COLLECTION" ? "collection" : isView ? "view" : "table",
             };
         }),
     }));
 });

 
const tableCount = computed(() => {
    const metadata = activeConnectionMetadata.value;

    if (!metadata || !Array.isArray(metadata)) {
        return 0;
    }

    return metadata.filter(
        (item) => item.type === "TABLE" || item.type === "COLLECTION"
    ).length;
});

const viewCount = computed(() => {
    const metadata = activeConnectionMetadata.value;

    if (!metadata || !Array.isArray(metadata)) {
        return 0;
    }

    return metadata.filter(
        (item) => item.type === "VIEW"
    ).length;
});

async function refreshSchema() {
    loading.value = true;

    try {
        await connectionStore.setActiveConnectionMetadata();

        emit("refresh");
    } finally {
        loading.value = false;
    }
}

function handleNodeDoubleClick(node: SchemaNode) {
    if (node.type === "table" || node.type === "view" || node.type === "collection") {
        emit("selectTable", node);
    }
}

function handleSearch() {
    searchQuery.value = searchTerm.value;
}

function handleSearchShortcut(event: KeyboardEvent) {
    if (!activeConnection.value || event.isComposing || !event.ctrlKey ||
        !event.shiftKey || event.altKey || event.metaKey || event.key.toLowerCase() !== "f") {
        return;
    }

    event.preventDefault();
    event.stopPropagation();
    searchInput.value?.focus();
    searchInput.value?.select();
}

watch(
    schemaNodes,
    (nodes) => {
        expanded.value = nodes.map((node) => node.id);
    },
    { immediate: true }
);

onMounted(() => {
    // Capture the shortcut before the SQL editor handles search keys.
    window.addEventListener("keydown", handleSearchShortcut, true);
    if (!activeConnectionMetadata.value) {
        connectionStore.setActiveConnectionMetadata();
    }
});

onBeforeUnmount(() => {
    window.removeEventListener("keydown", handleSearchShortcut, true);
});
</script>

<style scoped>
.schema-sidebar { --ui-font-size: var(--sidebar-font-size); height: 100%; min-width: 0; display: flex; flex-direction: column; background: var(--surface-data); color: var(--text-primary); font-size: var(--sidebar-font-size); border-right: 1px solid var(--border-default); }
.schema-connection { margin: 14px 12px 0; padding: 14px 12px; border: 1px solid var(--border-default); border-radius: 10px; background: var(--surface-panel); }
.connection-summary { display: flex; align-items: center; gap: 10px; }
.connection-engine { width: 34px; height: 34px; display: grid; place-items: center; flex-shrink: 0; color: var(--accent-color); background: var(--accent-tint); border-radius: 9px; }
.connection-summary-text { flex: 1; min-width: 0; }
.connection-eyebrow { font-size: max(9px, calc(var(--ui-font-size) - 2px)); letter-spacing: .1em; color: var(--text-muted); margin-bottom: 4px; }
.active-connection-name { font-weight: 600; overflow-wrap: anywhere; line-height: 1.4; }
.schema-refresh { color: var(--text-muted); flex-shrink: 0; }
.connection-details { margin-top: 13px; display: flex; align-items: center; flex-wrap: wrap; gap: 7px; min-width: 0; }
.driver-badge { padding: 3px 6px; border: 1px solid var(--border-default); border-radius: 5px; font-size: max(10px, calc(var(--ui-font-size) - 1px)); color: var(--text-muted); }
.connection-endpoint { min-width: 0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-muted); font-size: max(10px, calc(var(--ui-font-size) - 1px)); }
.schema-search { padding: 16px 12px 14px; }
.schema-search :deep(.q-field__control) { border-radius: 7px; min-height: max(38px, calc(var(--ui-font-size) + 24px)); }
.schema-search :deep(.q-field__native) { font-size: var(--ui-font-size); }
.schema-section-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 0 18px 10px; color: var(--text-muted); font-weight: 500; }
.object-count { color: var(--text-muted); background: var(--surface-panel); font-size: inherit; }
.schema-tree-scroll { flex: 1; min-height: 0; }
.compact-tree { padding: 0 10px 14px; font-size: var(--ui-font-size); }
.compact-tree :deep(.q-tree__node-header) { padding: 0 3px; border-radius: 6px; }
.compact-tree :deep(.q-tree__node-header-content) { min-width: 0; }
.compact-tree :deep(.q-tree__node) { padding-bottom: 2px; }
.compact-tree :deep(.q-tree__arrow) { font-size: 16px; color: var(--text-muted); }
.schema-node { display: flex; align-items: center; gap: 9px; width: 100%; min-width: 0; padding: 8px 4px; cursor: pointer; line-height: 1.4; }
.schema-node > .q-icon { flex-shrink: 0; color: var(--text-muted); }
.schema-node-label { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.schema-node-group { font-weight: 600; }
.schema-node-group > .q-icon { color: var(--accent-color); }
.schema-node-count { margin-left: auto; color: var(--text-muted); font-weight: 400; padding-right: 4px; }
.view-marker { margin-left: auto; }
.schema-empty { display: flex; flex-direction: column; align-items: center; gap: 12px; padding: 32px 16px; color: var(--text-muted); text-align: center; }
.schema-footer { display: flex; flex-wrap: wrap; gap: 6px 16px; padding: 12px 16px; border-top: 1px solid var(--border-default); color: var(--text-muted); font-size: max(10px, calc(var(--ui-font-size) - 1px)); }
</style>
