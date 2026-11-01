<template>
    <q-dialog :model-value="modelValue" :persistent="copying" :transition-duration="0" @update:model-value="emit('update:modelValue', $event)">
        <q-card class="advanced-copy-card app-dialog">
            <header class="app-dialog-header"><div><h2 class="app-dialog-title">Advanced copy</h2><p>{{ selection.rows.length }} rows · {{ selection.columns.length }} columns · selected cells</p></div>
                <q-btn flat dense class="app-close-button" icon="close" aria-label="Close advanced copy" :disable="copying" v-close-popup /></header>
            <div class="copy-options">
                <q-select v-model="format" :disable="copying" :options="formats" emit-value map-options dark dense outlined label="Format" />
                <q-input v-if="format === 'custom'" v-model="delimiter" :disable="copying" dark dense outlined label="Delimiter" />
                <q-checkbox v-if="delimited" v-model="headers" :disable="copying" dark dense label="Include column headers" />
                <q-input v-if="delimited" v-model="nullText" :disable="copying" dark dense outlined label="NULL text" />
            </div>
            <div class="copy-preview"><label for="advanced-copy-preview">Preview</label>
                <textarea id="advanced-copy-preview" :value="preview.text" readonly spellcheck="false" aria-label="Advanced copy preview" />
                <p v-if="preview.truncated" class="preview-note">Preview shortened for performance. Copy includes all {{ selection.rows.length }} selected rows.</p>
                <p v-if="copyError" class="copy-error" role="alert">{{ copyError }}</p>
                <p v-if="preview.error" class="copy-error" role="alert">{{ preview.error }}</p>
            </div>
            <footer class="app-dialog-actions"><q-btn flat dense no-caps label="Cancel" :disable="copying" v-close-popup />
                <q-btn unelevated dense no-caps color="primary" label="Copy" icon="content_copy" :disable="!!preview.error || !preview.text" :loading="copying" @click="copy" /></footer>
        </q-card>
    </q-dialog>
</template>
<script setup lang="ts">
import {computed, nextTick, ref, watch} from 'vue';
import {copyToClipboard, Notify} from 'quasar';
import {formatCopySelection} from '@/utils/copyWorkerClient';
import {advancedCopyPreview, type CopyFormat, type CopySelection, type CopyTable} from '@/utils/gridConvenience';
const props = defineProps<{modelValue: boolean; selection: CopySelection; driver: string; table?: CopyTable}>();
const emit = defineEmits<{'update:modelValue': [value: boolean]}>();
const format = ref<CopyFormat>('tsv');
const headers = ref(true);
const delimiter = ref('|');
const nullText = ref('NULL');
const copying = ref(false);
const copyError = ref('');
const delimited = computed(() => ['tsv', 'csv', 'custom'].includes(format.value));
const formats = computed(() => [
    {label: 'Tab-separated', value: 'tsv'}, {label: 'CSV', value: 'csv'},
    {label: 'Custom delimiter', value: 'custom'}, {label: 'JSON', value: 'json'},
    ...(props.table ? [{label: 'SQL INSERT', value: 'insert'}, {label: 'SQL UPDATE', value: 'update'}] : []),
]);
const options = computed(() => ({format: format.value, headers: headers.value, delimiter: delimiter.value, nullText: nullText.value, driver: props.driver, table: props.table ? {name: props.table.name, schema: props.table.schema, database: props.table.database} : undefined}));
watch(options, () => {copyError.value = '';});
watch(() => props.modelValue, () => {copyError.value = '';});
const preview = computed(() => {
    if (!props.modelValue) return {text: '', error: '', truncated: false};
    try {return {...advancedCopyPreview(props.selection, options.value), error: ''};}
    catch (error) {return {text: '', error: error instanceof Error ? error.message : String(error), truncated: false};}
});
async function copy() {
    if (copying.value || preview.value.error || !preview.value.text) return;
    copying.value = true;
    copyError.value = '';
    const selection = props.selection, settings = options.value;
    try {
        // Let the loading state render before formatting a large selection.
        await nextTick();
        await new Promise<void>(resolve => setTimeout(resolve, 0));
        await copyToClipboard(await formatCopySelection(selection, settings));
        emit('update:modelValue', false);
        Notify.create({color: 'positive', message: `${selection.rows.length} selected rows copied`});
    } catch (error) {
        copyError.value = error instanceof Error ? error.message : String(error);
    } finally {copying.value = false;}
}
</script>
<style scoped>
.advanced-copy-card.q-card { width: 660px; display: flex; flex-direction: column; overflow: hidden; }
header, footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 16px; flex-shrink: 0; }
header { border-bottom: 1px solid var(--border-default); }
h2 { margin: 0; font-size: 13px; font-weight: 600; color: var(--accent-color); }
p { margin: 5px 0 0; color: var(--text-muted); }
.copy-options { padding: 20px; display: grid; grid-template-columns: 1fr 1fr; gap: 12px; flex-shrink: 0; }
.copy-preview { flex: 1 1 auto; display: flex; flex-direction: column; min-height: 0; padding: 0 20px 20px; }
label { color: var(--text-muted); margin-bottom: 6px; }
textarea { width: 100%; height: 260px; min-height: 80px; resize: vertical; background: var(--surface-data); color: var(--text-primary); border: 1px solid var(--border-default); border-radius: 4px; padding: 10px; font: inherit; font-family: var(--font-mono); white-space: pre; overflow: auto; }
.copy-error { color: var(--status-error); }
footer { justify-content: flex-end; border-top: 1px solid var(--border-default); }
</style>

<style scoped>
@media (max-width: 480px) { .copy-options { grid-template-columns: 1fr; } }
</style>
