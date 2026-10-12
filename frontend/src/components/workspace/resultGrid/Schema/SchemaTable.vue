<template>
    <div class="h-full relative overflow-hidden bg-[#2d2d2d]">
        <div class="q-ma-lg">
            <q-table
                flat
                square
                dense
                dark
                :rows="rows"
                :columns="columns"
                row-key="name"
                :pagination="{
                    rowsPerPage: 0,
                }"
                class="absolute-full schema-grid"
                hide-bottom
            >
                <!-- Header -->
                <template #header="props">
                    <q-tr :props="props">
                        <q-th
                            v-for="col in props.cols"
                            :key="col.name"
                            :props="props"
                            :class="{
                                'sn-header': col.name === 'sn',
                            }"
                            class="schema-header text-amber-400"
                        >
                            {{ col.label }}
                        </q-th>
                    </q-tr>
                </template>
    
                <!-- Body -->
                <template #body="props">
                    <q-tr :props="props">
                        <q-td
                            v-for="col in props.cols"
                            :key="col.name"
                            :props="props"
                            :class="{
                                'sn-cell': col.name === 'sn',
                            }"
                        >
                            <template v-if="col.name === 'sn'">
                                {{ props.rowIndex + 1 }}
                            </template>
    
                            <template v-else-if="col.name === 'column'">
                                <div class="flex items-center gap-2">
                                    <q-icon
                                        name="view_column"
                                        size="14px"
                                        class="text-amber-500"
                                    />
    
                                    <span class="text-white font-medium">
                                        {{ props.row.name }}
                                    </span>
                                </div>
                            </template>
    
                            <template v-else-if="col.name === 'nullable'">
                                <span
                                    :class="
                                        props.row.nullable
                                            ? 'text-teal-400'
                                            : 'text-amber-400'
                                    "
                                >
                                    {{
                                        props.row.nullable
                                            ? "YES"
                                            : "NO"
                                    }}
                                </span>
                            </template>
    
                            <template v-else-if="col.name === 'default'">
                                <span
                                    v-if="props.row.defaultValue"
                                    class="text-purple-300"
                                >
                                    {{ props.row.defaultValue }}
                                </span>
    
                                <span
                                    v-else
                                    class="text-[#909090]"
                                >
                                    —
                                </span>
                            </template>
    
                            <template v-else>
                                {{ col.value }}
                            </template>
                        </q-td>
                    </q-tr>
                </template>
            </q-table>
        </div>
       
    </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { QTableColumn } from "quasar";
import type { QueryResult } from "@/types/queryTab";

const props = defineProps<{
    result: QueryResult;
}>();

const columns = computed<QTableColumn[]>(() => [
    {
        name: "sn",
        label: "#",
        field: "sn",
        align: "left",
        sortable: false,
    },

    {
        name: "column",
        label: "Column",
        field: "name",
        align: "left",
        sortable: true,
    },

    {
        name: "type",
        label: "Type",
        field: "type",
        align: "left",
        sortable: true,
    },

    {
        name: "nullable",
        label: "Nullable",
        field: "nullable",
        align: "left",
        sortable: true,
    },

    {
        name: "default",
        label: "Default",
        field: "defaultValue",
        align: "left",
        sortable: true,
    },
]);

const rows = computed(() => {
    return props.result.Columns.map((column) => ({
        name: column.Name,
        type: column.Type,
        nullable: column.Nullable,
        defaultValue: column.DefaultValue,
    }));
});
</script>
