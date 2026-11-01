<template>
    <q-dialog :model-value="modelValue" :transition-duration="0" @update:model-value="emit('update:modelValue', $event)">
        <q-card class="cell-value-viewer bg-[var(--surface-panel)] text-[var(--text-primary)]">
            <q-card-section class="flex items-center gap-3">
                <div class="flex-1 min-w-0">
                    <div class="text-base break-words">Cell value<span v-if="column"> · {{ column }}</span></div>
                    <div class="text-caption text-[var(--text-muted)]">{{ content.kind }}<span v-if="type"> · {{ type }}</span> · {{ content.size }}</div>
                </div>
                <q-btn flat round dense icon="close" aria-label="Close cell value viewer" v-close-popup />
            </q-card-section>
            <div class="flex items-center gap-3 px-4 pb-3">
                <q-btn-toggle v-model="mode" dense flat no-caps :options="modes" aria-label="Value display format" />
                <q-space />
                <q-checkbox v-model="wrap" dense label="Wrap" />
                <q-btn flat dense no-caps icon="content_copy" label="Copy" @click="copyValue" />
            </div>
            <q-separator />
            <q-card-section class="value-content">
                <pre class="value-text" :class="{ 'value-wrap': wrap }" tabindex="0" aria-label="Cell value" aria-readonly="true">{{ displayed }}</pre>
            </q-card-section>
        </q-card>
    </q-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { copyToClipboard, Notify } from 'quasar';
import { cellValue } from '@/utils/cellValue';
const props = defineProps<{ modelValue: boolean; value: unknown; type?: string; column?: string }>();
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>();
const content = computed(() => cellValue(props.value, props.type));
const mode = ref('formatted');
const wrap = ref(true);
const modes = computed(() => content.value.kind === 'Binary'
    ? [{ label: 'Hex', value: 'formatted' }, { label: 'Text (UTF-8)', value: 'text' }, { label: 'Raw', value: 'raw' }]
    : content.value.kind === 'JSON' ? [{ label: 'Formatted', value: 'formatted' }, { label: 'Raw', value: 'raw' }]
    : [{ label: 'Text', value: 'raw' }]);
const displayed = computed(() => mode.value === 'formatted' ? content.value.formatted : mode.value === 'text' ? content.value.text : content.value.raw);
watch(() => props.modelValue, open => {
    if (open) { mode.value = ['JSON', 'Binary'].includes(content.value.kind) ? 'formatted' : 'raw'; wrap.value = content.value.kind !== 'Binary'; }
});
async function copyValue() {
    try { await copyToClipboard(displayed.value); }
    catch (error) { Notify.create({ type: 'negative', message: `Could not copy cell value: ${String(error)}` }); }
}
</script>

<style scoped>
.cell-value-viewer { width: 900px; max-width: 92vw; max-height: 85vh; display: flex; flex-direction: column; }
.value-content { overflow: auto; min-height: 160px; flex: 1; }
.value-text { margin: 0; white-space: pre; font-family: monospace; font-size: var(--ui-font-size); user-select: text; }
.value-wrap { white-space: pre-wrap; overflow-wrap: anywhere; }
</style>
