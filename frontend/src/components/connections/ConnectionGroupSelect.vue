<template>
    <div>
        <q-select :model-value="modelValue" :options="options" :label="label" clearable outlined dense color="primary"
            :disable="disable" hint="Clear the selection to leave this connection ungrouped."
            @update:model-value="$emit('update:modelValue', $event)" />
        <q-btn v-if="!creating" flat dense no-caps color="primary" icon="create_new_folder" label="Create group"
            class="q-mt-sm" :disable="disable" @click="startCreating" />
        <div v-else class="q-mt-sm">
            <q-input ref="nameInput" v-model="newName" label="New group name" outlined dense color="primary" :disable="disable"
                @keydown.enter.stop.prevent="createGroup" @keydown.esc.stop.prevent="creating = false" />
            <div class="row justify-end q-gutter-sm q-mt-sm">
                <q-btn flat dense no-caps label="Cancel" :disable="disable" @click="creating = false" />
                <q-btn unelevated dense no-caps color="primary" text-color="black" label="Create group"
                    :disable="disable || !newName.trim()" @click="createGroup" />
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from 'vue';
import type { QInput } from 'quasar';

const props = withDefaults(defineProps<{
    modelValue: string | null;
    groups: string[];
    label?: string;
    disable?: boolean;
}>(), { label: 'Group (optional)', disable: false });
const emit = defineEmits<{ 'update:modelValue': [value: string | null] }>();
const creating = ref(false);
const newName = ref('');
const nameInput = ref<QInput | null>(null);
const options = computed(() => Array.from(new Set([
    ...props.groups, ...(props.modelValue ? [props.modelValue] : []),
])).sort((a, b) => a.localeCompare(b)));
async function startCreating() {
    newName.value = '';
    creating.value = true;
    await nextTick();
    nameInput.value?.focus();
}
function createGroup() {
    const name = newName.value.trim();
    if (!name || props.disable) return;
    emit('update:modelValue', name);
    creating.value = false;
}
</script>
