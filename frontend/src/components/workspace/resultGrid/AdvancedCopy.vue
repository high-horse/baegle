<template>
    <q-dialog :model-value="modelValue" :transition-duration="0" @update:model-value="emit('update:modelValue', $event)">
        <q-card class="advanced-copy-card">
            <header><div><h2>Advanced copy</h2><p>{{ selection.rows.length }} rows · {{ selection.columns.length }} columns · selected cells</p></div>
                <q-btn flat round dense icon="close" aria-label="Close advanced copy" v-close-popup /></header>
            <div class="copy-options">
                <q-select v-model="format" :options="formats" emit-value map-options dark dense outlined label="Format" />
                <q-input v-if="format === 'custom'" v-model="delimiter" dark dense outlined label="Delimiter" />
                <q-checkbox v-if="delimited" v-model="headers" dark dense label="Include column headers" />
                <q-input v-if="delimited" v-model="nullText" dark dense outlined label="NULL text" />
            </div>
            <div class="copy-preview"><label for="advanced-copy-preview">Preview</label>
                <textarea id="advanced-copy-preview" :value="preview.text" readonly spellcheck="false" aria-label="Advanced copy preview" />
                <p v-if="preview.error" class="copy-error" role="alert">{{ preview.error }}</p>
            </div>
            <footer><q-btn flat dense no-caps label="Cancel" v-close-popup />
                <q-btn unelevated dense no-caps color="primary" label="Copy" icon="content_copy" :disable="!!preview.error || !preview.text" :loading="copying" @click="copy" /></footer>
        </q-card>
    </q-dialog>
</template>
<script setup lang="ts">
import {computed, ref} from 'vue';
import {copyToClipboard, Notify} from 'quasar';
import {advancedCopy, type CopyFormat, type CopySelection, type CopyTable} from '@/utils/gridConvenience';
const props = defineProps<{modelValue: boolean; selection: CopySelection; driver: string; table?: CopyTable}>();
const emit = defineEmits<{'update:modelValue': [value: boolean]}>();
const format = ref<CopyFormat>('tsv');
const headers = ref(true);
const delimiter = ref('|');
const nullText = ref('NULL');
const copying = ref(false);
const delimited = computed(() => ['tsv', 'csv', 'custom'].includes(format.value));
const formats = computed(() => [
    {label: 'Tab-separated', value: 'tsv'}, {label: 'CSV', value: 'csv'},
    {label: 'Custom delimiter', value: 'custom'}, {label: 'JSON', value: 'json'},
    ...(props.table ? [{label: 'SQL INSERT', value: 'insert'}, {label: 'SQL UPDATE', value: 'update'}] : []),
]);
const preview = computed(() => {
    try {return {text: advancedCopy(props.selection, {format: format.value, headers: headers.value, delimiter: delimiter.value, nullText: nullText.value, driver: props.driver, table: props.table}), error: ''};}
    catch (error) {return {text: '', error: error instanceof Error ? error.message : String(error)};}
});
async function copy() {
    if (preview.value.error || !preview.value.text) return;
    copying.value = true;
    try {await copyToClipboard(preview.value.text); emit('update:modelValue', false); Notify.create({color: 'positive', message: 'Selection copied'});}
    catch (error) {Notify.create({type: 'negative', message: `Could not copy selection: ${String(error)}`});}
    finally {copying.value = false;}
}
</script>
<style scoped>
.advanced-copy-card.q-card { width: 660px; max-width: calc(100vw - 32px); max-height: calc(100dvh - 48px); display: flex; flex-direction: column; overflow: hidden; background: var(--surface-panel); color: var(--text-primary); border: 1px solid var(--border-default); border-radius: 4px; font-family: monospace; font-size: var(--ui-font-size); }
header, footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 16px; flex-shrink: 0; }
header { border-bottom: 1px solid var(--border-default); }
h2 { margin: 0; font-size: 13px; font-weight: 600; color: var(--accent-color); }
p { margin: 5px 0 0; color: var(--text-muted); }
.copy-options { padding: 16px; display: grid; grid-template-columns: 1fr 1fr; gap: 12px; flex-shrink: 0; }
.copy-preview { display: flex; flex-direction: column; min-height: 0; padding: 0 16px 16px; }
label { color: var(--text-muted); margin-bottom: 6px; }
textarea { width: 100%; height: 260px; min-height: 80px; resize: vertical; background: var(--surface-data); color: var(--text-primary); border: 1px solid var(--border-default); border-radius: 4px; padding: 10px; font: inherit; white-space: pre; overflow: auto; }
.copy-error { color: #f28b82; }
footer { justify-content: flex-end; border-top: 1px solid var(--border-default); }
</style>
