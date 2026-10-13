<template>
    <div
        class="h-full bg-[#2d2d2d] border-r border-[#484848] flex flex-col font-sans"
    >
        <!-- Sidebar Header -->
        <div
            class="p-3 border-b border-[#484848] flex justify-between items-center bg-[#353535]"
        >
            <div class="flex items-center gap-2 min-w-0">
                <q-icon name="storage" size="15px" class="text-amber-400" />

                <div class="min-w-0">
                    <div class="text-xs font-semibold text-white truncate">
                        {{ activeConnection?.name }}
                    </div>

                    <div class="text-[10px] text-[#a0a0a0] truncate">
                        <span class="font-bold">
                            {{ activeConnection?.driver }}
                        </span>
                        ({{ connectionAddress }})
                    </div>
                </div>
            </div>

            <q-btn
                flat
                dense
                round
                icon="refresh"
                size="xs"
                class="text-[#a0a0a0] hover:text-amber-400"
                :loading="loading"
                @click="refreshSchema"
            />
        </div>

        <div class="q-pa-md">
            <q-input
                ref="searchInput"
                v-model="searchTerm"
                placeholder="Search..."
                aria-label="Search database objects"
                aria-keyshortcuts="Control+Shift+F"
                class="w-full"
                dense rounded
                outlined
                @keyup.enter="handleSearch"
            />
        </div>
        <!-- Schema Tree -->
        <q-scroll-area class="flex-grow p-2">
            <q-tree
                v-model:expanded="expanded"
                :nodes="schemaNodes"
                node-key="id"
                dark
                no-connectors
                dense
                default-expand-all
                class="compact-tree"
                :style="{ fontSize: `${resultSettings.settings.sidebarFontSize}px` }"
            >
                <template #default-header="prop">
                    <div
                        class="flex items-center gap-1 py-[4px] px-1 rounded cursor-pointer group w-full hover:bg-[#404040] compact-tree-header"
                        @dblclick="handleNodeDoubleClick(prop.node)"
                    >
                        <q-icon
                            :name="prop.node.icon"
                            :color="prop.node.iconColor"
                            size="15px"
                        />
            
                        <span
                            :class="
                                prop.node.type === 'table'
                                    ? 'text-white font-medium'
                                    : prop.node.type === 'view'
                                      ? 'text-blue-300'
                                      : 'text-amber-300'
                            "
                        >
                            {{ prop.node.label }}
                        </span>
                        <q-icon v-if="prop.node.type === 'view'" name="visibility" size="10px" >
                            <q-tooltip>
                                <span>View</span>
                            </q-tooltip>
                        </q-icon>
                    </div>
                </template>
            </q-tree>

            <!-- Empty state -->
            <div
                v-if="!schemaNodes.length && !loading"
                class="flex flex-col items-center justify-center py-8 gap-2 text-[#909090]"
            >
                <q-icon name="table_rows" size="24px" />

                <span class="text-[10px] font-mono">
                    No {{ activeConnection?.driver === 'mongodb' ? 'collections' : 'tables' }} found
                </span>
            </div>
        </q-scroll-area>

        <!-- Sidebar Footer -->
        <div
            class="h-7 px-3 flex items-center justify-between border-t border-[#484848] bg-[#353535]"
        >
            <div class="flex items-center gap-3">
                <span class="text-[10px] text-[#909090] font-mono">
                    {{ tableCount }} {{ activeConnection?.driver === 'mongodb' ? 'collections' : 'tables' }}
                </span>

                <span class="text-[10px] text-[#909090] font-mono">
                    {{ viewCount }} views
                </span>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount, watch } from "vue";
import type { QInput } from "quasar";
import { storeToRefs } from "pinia";
import { useConnectionStore } from "@/stores/connectionStore";
import { useResultSettingsStore } from "@/stores/resultSettingsStore";

const connectionStore = useConnectionStore();
const resultSettings = useResultSettingsStore();
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
         iconColor: "amber-5",
         type: "schema",
 
         children: objects.map((item) => {
             const isView = item.type === "VIEW";
 
             return {
                 id: `${schema}.${item.name}`,
                 schema,
                 database: item.database,
                 label: item.name,
                 icon: item.type === "COLLECTION" ? "data_object" : isView ? "view_list" : "table_chart",
                 iconColor: isView ? "blue-4" : "amber-4",
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
