<template>
    <q-btn v-if="action === 'import'" flat dense round icon="file_upload" size="sm" title="Import data" aria-label="Import data"
        :disable="connection.activeConnection?.read_only" @click="open('import')" />
    <q-btn v-else flat dense no-caps class="export-button" title="Export result data" aria-label="Export data"
        :disable="disabled" @click="open('export')">
        <q-icon name="file_download" size="16px" /><span>Export</span>
    </q-btn>
    <q-dialog v-model="visible" :persistent="busy">
        <q-card class="transfer-card">
            <q-card-section>
                <div class="text-h6">{{ mode === 'import' ? 'Import data' : 'Export data — Data transfer' }}</div>
                <div class="text-caption text-grey-5">{{ connection.activeConnection?.name }}<template v-if="mode === 'export'"> · {{ sourceTitle }}</template></div>
            </q-card-section>
            <q-card-section class="q-pt-none transfer-body">
                <q-banner v-if="error" class="bg-red-10 text-red-2 q-mb-md" role="alert">{{ error }}</q-banner>
                <q-banner v-if="success" class="bg-green-10 text-green-2 q-mb-md" role="status">{{ success }}</q-banner>
                <template v-if="mode === 'export'">
                    <ol class="export-steps" aria-label="Export steps">
                        <li v-for="(label, index) in exportSteps" :key="label" :class="{ active: exportStep === index, complete: exportStep > index }"
                            :aria-current="exportStep === index ? 'step' : undefined"><span>{{ index + 1 }}</span>{{ label }}</li>
                    </ol>
                    <div v-if="exportStep === 0" class="export-panel">
                        <h3>Choose an output format</h3>
                        <div class="format-list" role="radiogroup" aria-label="Export format">
                            <label v-for="item in formats" :key="item.value" class="format-option" :class="{ selected: format === item.value }">
                                <input v-model="format" type="radio" name="export-format" :value="item.value" :disabled="busy" />
                                <span><strong>{{ item.label }}</strong><small>{{ formatDescriptions[item.value] }}</small></span>
                            </label>
                        </div>
                    </div>
                    <div v-else-if="exportStep === 1" class="export-panel">
                        <h3>Extraction settings</h3>
                        <q-select v-model="scope" :options="scopes" emit-value map-options label="Rows to export" outlined dense :disable="busy" />
                        <p class="text-caption text-grey-5">{{ scopeHint }}</p>
                        <q-select v-if="format === 'sql' && !sourceTable" v-model="tableKey" :options="tableOptions" emit-value map-options
                            label="Target table name for INSERT statements" outlined dense :disable="busy" class="q-mb-md" />
                        <h3>Format settings</h3>
                        <q-select v-if="format === 'csv'" v-model="delimiter" :options="delimiters" emit-value map-options
                            label="Column delimiter" outlined dense :disable="busy" class="q-mb-md" />
                        <q-input v-if="format === 'csv' || format === 'xlsx'" v-model="nullToken" label="NULL value text" outlined dense :disable="busy"
                            hint="Choose a distinct marker to distinguish NULL from an empty string." />
                        <p class="text-caption text-grey-5">{{ formatNotes }}</p>
                    </div>
                    <div v-else class="export-panel">
                        <h3>Review and save</h3>
                        <dl class="export-summary">
                            <dt>Source</dt><dd>{{ sourceTitle }}</dd>
                            <dt>Rows</dt><dd>{{ scopes.find(item => item.value === scope)?.label }}</dd>
                            <template v-if="scope === 'page'"><dt>Loaded rows</dt><dd>{{ sourceResult?.Rows.length ?? 0 }}</dd></template>
                            <dt>Format</dt><dd>{{ formats.find(item => item.value === format)?.label }}</dd>
                            <template v-if="format === 'csv'"><dt>Delimiter</dt><dd>{{ delimiters.find(item => item.value === delimiter)?.label }}</dd></template>
                            <template v-if="format === 'csv' || format === 'xlsx'"><dt>NULL text</dt><dd>{{ nullToken || '(empty)' }}</dd></template>
                            <template v-if="format === 'sql'"><dt>INSERT target</dt><dd>{{ [table?.schema, table?.name].filter(Boolean).join('.') }}</dd></template>
                            <dt>Destination</dt><dd>Local file — choose the path when you click Finish.</dd>
                        </dl>
                        <p class="text-caption text-grey-5">{{ scopeHint }}</p>
                    </div>
                </template>
                <template v-else>
                <q-select v-model="tableKey"
                    :options="tableOptions" emit-value map-options label="Table / collection" outlined dense :disable="busy" class="q-mb-md" />
                <q-select v-model="format" :options="formats" emit-value map-options label="File format" outlined dense :disable="busy" />
                <p class="text-caption text-grey-5 q-mt-sm">{{ scopeHint }}</p>
                <div v-if="format === 'csv'" class="row q-col-gutter-sm q-mb-md">
                    <q-select class="col" v-model="delimiter" :options="delimiters" emit-value map-options label="Delimiter" outlined dense :disable="busy" />
                </div>
                <q-input v-if="format === 'csv' || format === 'xlsx'" v-model="nullToken" label="NULL value text" outlined dense :disable="busy" class="q-mb-md" />
                <p v-if="format === 'csv' || format === 'xlsx'" class="text-caption text-grey-5">
                    First row contains column names. NULL uses the marker above; other empty cells are empty strings. Binary values use base64.
                    A leading backslash is escaped with another backslash on export. Excel exports cells as text to preserve exact values.
                </p>
                <p v-if="format === 'sql'" class="text-caption text-grey-5">
                    Data-only INSERT INTO … VALUES files for the current database dialect. Import maps their values into the selected table.
                    DDL, functions, COPY, and full database dumps are not supported here.
                </p>
                <template v-if="mode === 'import'">
                    <div class="row items-center q-gutter-sm q-mb-md">
                        <q-btn outline no-caps label="Choose file…" icon="folder_open" :disable="busy" @click="chooseFile" />
                        <span class="file-path">{{ path || 'No file selected' }}</span>
                    </div>
                    <q-select v-if="format === 'xlsx' && sheets.length" v-model="sheet" :options="sheets" clearable
                        label="Worksheet (first sheet by default)" outlined dense :disable="busy" class="q-mb-md" />
                    <q-btn outline no-caps label="Preview file" :disable="busy || !path || !table" @click="previewFile" />
                    <p class="text-caption text-grey-5">Up to 32 MiB and 100,000 rows per import. Inserts new rows; existing rows are retained.</p>
                    <template v-if="preview">
                        <div class="text-subtitle2 q-mt-md">{{ preview.total }} rows · Column mapping</div>
                        <p class="text-caption text-grey-5">Unmapped fields use database defaults. Generated columns are excluded. Map identity columns only to preserve source IDs.</p>
                        <div v-for="column in preview.columns" :key="column" class="mapping-row">
                            <span>{{ column }}</span>
                            <q-select v-model="mapping[column]" :options="targetColumns" emit-value map-options clearable
                                label="Destination (blank to skip)" outlined dense :disable="busy" />
                        </div>
                        <div class="preview-scroll q-mt-md">
                            <table><thead><tr><th v-for="column in preview.columns" :key="column">{{ column }}</th></tr></thead>
                                <tbody><tr v-for="(row, i) in preview.rows" :key="i"><td v-for="(cell, j) in row" :key="j">{{ cell }}</td></tr></tbody>
                            </table>
                        </div>
                        <p class="text-caption text-grey-5">Preview shows the first five source rows.</p>
                        <p v-if="isMongo" class="text-caption text-amber-4">MongoDB imports stop at the first error; preceding documents remain inserted. Review the reported count before retrying.</p>
                        <p v-else class="text-caption text-grey-5">SQL table imports run in a transaction and roll back if a row fails. MySQL tables must use a transactional engine (such as InnoDB).</p>
                    </template>
                </template>
                </template>
                <div v-if="busy" class="row items-center q-gutter-sm q-mt-md" role="status">
                    <q-spinner color="amber" /><span>{{ operation }} <QueryElapsedTime v-if="startedAt !== null" :started-at="startedAt" /></span>
                </div>
            </q-card-section>
            <q-card-actions align="right" class="q-pa-md">
                <q-btn flat no-caps :label="success ? 'Close' : 'Cancel'" :disable="busy" v-close-popup />
                <q-btn v-if="mode === 'import'" unelevated no-caps color="amber" text-color="black" :label="`Import ${preview?.total ?? 0} rows`"
                    :disable="busy || !preview?.total || !hasMapping || !!success" @click="importData" />
                <template v-else-if="!success">
                    <q-btn v-if="exportStep > 0" flat no-caps label="Back" :disable="busy" @click="exportStep--" />
                    <q-btn v-if="exportStep < 2" unelevated no-caps color="amber" text-color="black" label="Next"
                        :disable="busy || !canExport" @click="exportStep++" />
                    <q-btn v-else unelevated no-caps color="amber" text-color="black" label="Finish"
                        :disable="busy || !canExport" @click="exportData" />
                </template>
            </q-card-actions>
        </q-card>
    </q-dialog>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue';
import { DbService } from '@bindings/db-lens/internal/app';
import type { ImportPreview, ExportRequest } from '@bindings/db-lens/internal/app';
import type { TableEditInfo, TableRef } from '@bindings/db-lens/internal/engine/entities';
import type { QueryResult } from '@/types/queryTab';
import { useConnectionStore } from '@/stores/connectionStore';
import { useQueryTabsStore } from '@/stores/queryTabsStore';
import { useTableEditsStore } from '@/stores/tableEditsStore';
import QueryElapsedTime from './QueryElapsedTime.vue';

const props = withDefaults(defineProps<{
    action?: 'import' | 'export';
    result?: QueryResult;
    tableTabId?: string;
    disabled?: boolean;
}>(), { action: 'import' });
const connection = useConnectionStore();
const queries = useQueryTabsStore();
const edits = useTableEditsStore();
const visible = ref(false);
const mode = ref<'import' | 'export'>('export');
const scope = ref('table');
const format = ref('csv');
const delimiter = ref(',');
const nullToken = ref('\\N');
const exportStep = ref(0);
const exportSteps = ['Format', 'Settings', 'Review'];
const sourceTitle = ref('Query result');
const sourceQuery = ref('');
const sourceResult = shallowRef<QueryResult | null>(null);
const sourceTable = shallowRef<TableRef | null>(null);
const sourceConnectionId = ref('');
const formatDescriptions: Record<string, string> = {
    csv: 'Delimited text with column headers', json: 'An array of row objects or MongoDB documents',
    xlsx: 'Excel worksheet with values preserved as text', sql: 'SQL INSERT statements for an existing table',
};
const formatNotes = computed(() => format.value === 'csv' ? 'UTF-8 with a header row. Values containing delimiters or line breaks are quoted. Leading backslashes are escaped.' :
    format.value === 'xlsx' ? 'Includes a header row. Cells are exported as text to preserve exact IDs and prevent formula conversion. Leading backslashes are escaped.' :
    format.value === 'json' ? 'Exports complete row objects. MongoDB documents retain Extended JSON types.' :
    'Exports data-only INSERT statements using the current database dialect. Schema creation is not included.');
const delimiters = [
    { label: 'Comma (,)', value: ',' }, { label: 'Semicolon (;)', value: ';' },
    { label: 'Tab', value: '\t' }, { label: 'Pipe (|)', value: '|' },
];
const sheet = ref<string | null>('');
const sheets = ref<string[]>([]);
const tableKey = ref('');
const path = ref('');
const preview = ref<ImportPreview | null>(null);
const info = ref<TableEditInfo | null>(null);
const mapping = ref<Record<string, string | null>>({});
const busy = ref(false);
const operation = ref('');
const startedAt = ref<number | null>(null);
const error = ref('');
const success = ref('');
const isMongo = computed(() => connection.activeConnection?.driver === 'mongodb');
const options = computed(() => ({ format: format.value, delimiter: delimiter.value, nullToken: nullToken.value, sheet: sheet.value || '' }));
const keyFor = (t: { name: string; schema?: string; database?: string }) => JSON.stringify([t.database || '', t.schema || '', t.name]);
const tables = computed(() => (connection.activeConnectionMetadata ?? []).filter(t => mode.value === 'export' || t.type.toUpperCase() !== 'VIEW'));
const tableOptions = computed(() => tables.value.map(t => ({ label: [t.schema, t.name].filter(Boolean).join('.'), value: keyFor(t) })));
const table = computed(() => {
    if (mode.value === 'export' && sourceTable.value) return sourceTable.value;
    const t = tables.value.find(t => keyFor(t) === tableKey.value);
    return t ? { connectionId: String(connection.activeConnection?.id), name: t.name, schema: t.schema || '', database: t.database || '' } : null;
});
const formats = computed(() => [
    { label: 'CSV (.csv)', value: 'csv' }, { label: 'JSON (.json)', value: 'json' },
    { label: 'Excel (.xlsx)', value: 'xlsx' }, ...(!isMongo.value ? [{ label: 'SQL inserts (.sql)', value: 'sql' }] : []),
]);
const scopes = computed(() => [
    ...(sourceTable.value && (!isMongo.value || format.value === 'json') ? [{ label: isMongo.value ? 'Entire collection' : 'Entire source table / view', value: 'table' }] : []),
    ...(!isMongo.value && sourceQuery.value && sourceResult.value?.IsQuery ? [{ label: 'All query rows (query the database)', value: 'query' }] : []),
    ...(sourceResult.value?.IsQuery ? [{ label: 'Fetched rows (current result page)', value: 'page' }] : []),
]);
const canExport = computed(() => scopes.value.some(item => item.value === scope.value) &&
    (exportStep.value === 0 || !((scope.value === 'table' || format.value === 'sql') && !table.value)));
watch(scopes, items => { if (!items.some(item => item.value === scope.value)) scope.value = items[0]?.value ?? 'page'; });
const scopeHint = computed(() => mode.value === 'import' ? 'Review the destination and mappings before importing.' :
    scope.value === 'page' ? 'Exports the loaded page only, before local WHERE / ORDER BY filters and staged edits.' :
    scope.value === 'query' ? 'Reruns the last executed SELECT and exports all returned rows. Local grid filters and staged edits are excluded.' :
    'Exports every stored row in the selected table or collection, independently of result pagination and grid filters.');
const targetColumns = computed(() => isMongo.value ? (preview.value?.columns ?? []).map(name => ({ label: name, value: name })) :
    (info.value?.columns ?? []).filter(c => !c.generated).map(c => ({ label: `${c.name}${c.autoIncrement ? ' (identity)' : ''}`, value: c.name })));
const hasMapping = computed(() => Object.values(mapping.value).some(Boolean));
watch([path, format, delimiter, nullToken, sheet, tableKey], () => { preview.value = null; error.value = ''; success.value = ''; }, { flush: 'sync' });
watch(formats, value => { if (!value.some(f => f.value === format.value)) format.value = value[0]?.value ?? 'json'; });

async function task(label: string, run: () => Promise<void>) {
    if (busy.value) return;
    busy.value = true; operation.value = label; startedAt.value = performance.now(); error.value = ''; success.value = '';
    try { await run(); } catch (failure) { error.value = String(failure); }
    finally { busy.value = false; startedAt.value = null; }
}
async function open(value: 'import' | 'export') {
    mode.value = value; preview.value = null; path.value = ''; sheets.value = []; sheet.value = ''; error.value = ''; success.value = '';
    const active = props.tableTabId ? queries.tabs.find(tab => tab.id === props.tableTabId) : queries.activeTab;
    const activeTable = edits.states[props.tableTabId ?? active?.id ?? '']?.table ?? null;
    sourceResult.value = props.result ?? active?.result ?? null;
    sourceQuery.value = active?.executedSql ?? '';
    sourceTable.value = activeTable;
    sourceTitle.value = activeTable ? [activeTable.schema, activeTable.name].filter(Boolean).join('.') : active?.title ?? 'Query result';
    sourceConnectionId.value = String(connection.activeConnection?.id);
    exportStep.value = 0;
    if (isMongo.value && activeTable) format.value = 'json';
    tableKey.value = activeTable ? keyFor(activeTable) : tableOptions.value[0]?.value ?? '';
    scope.value = activeTable ? 'table' : !isMongo.value && sourceQuery.value ? 'query' : 'page';
    visible.value = true;
    if (!connection.activeConnectionMetadata) await task('Loading tables…', async () => { await connection.setActiveConnectionMetadata(); tableKey.value = tableOptions.value[0]?.value ?? ''; });
}
async function chooseFile() {
    await task('Choosing file…', async () => {
        const selected = await DbService.ChooseImportFile(); if (!selected) return;
        path.value = selected; sheets.value = []; sheet.value = '';
        const extension = selected.split('.').pop()?.toLowerCase();
        if (extension && formats.value.some(f => f.value === extension)) format.value = extension;
    });
}
function importRequest() {
    if (!table.value) throw new Error('Select a destination table');
    return { path: path.value, sha: preview.value?.sha ?? '', options: options.value, table: table.value,
        mapping: Object.fromEntries(Object.entries(mapping.value).map(([from, to]) => [from, to || ''])) };
}
async function previewFile() {
    await task('Reading preview…', async () => {
        preview.value = null;
        const request = importRequest();
        info.value = await DbService.DescribeTableEdit(request.table);
        if (!info.value?.canInsert) throw new Error(info.value?.reason || 'This table does not allow imports');
        const result = await DbService.PreviewImport(request);
        if (!result) throw new Error('No preview returned');
        sheets.value = result.sheets ?? [];
        mapping.value = Object.fromEntries((result.columns ?? []).map(name => [name, isMongo.value || info.value?.columns?.some(c => c.name === name && !c.generated && !c.autoIncrement) ? name : null]));
        preview.value = result;
    });
}
async function importData() {
    await task('Importing rows…', async () => {
        if (!preview.value) throw new Error('Preview the file first');
        const request = importRequest();
        const affected = Object.entries(edits.states).filter(([, state]) => keyFor(state.table) === keyFor(request.table));
        if (affected.some(([id]) => edits.hasPending(id) || edits.isSaving(id))) throw new Error('Save or discard staged changes for this table before importing');
        const states = affected.map(([,state]) => state); states.forEach(state => state.saving = true);
        try {
            const result = await DbService.ImportData(request);
            if (!result) throw new Error('No import result returned');
            success.value = `${result.applied} rows imported. Refresh the table to see the new data.`;
            if (result.error) error.value = result.error;
            // Avoid resubmitting a partially committed MongoDB import.
            if (result.applied > 0) preview.value = null;
        } finally { states.forEach(state => state.saving = false); }
    });
}
async function exportData() {
    await task('Exporting data…', async () => {
        if (!canExport.value) throw new Error('Choose an export source and target format');
        const result = sourceResult.value;
        const response = await DbService.ExportData({ connectionId: sourceConnectionId.value, scope: scope.value,
            options: options.value, table: table.value ?? { connectionId: sourceConnectionId.value, name: '', schema: '', database: '' },
            query: sourceQuery.value, columns: result?.Columns.map(c => c.Name) ?? [], rows: result?.Rows ?? [], documents: result?.Documents ?? [],
        } as ExportRequest);
        if (response) success.value = `Exported ${response.rows} rows to ${response.path}`;
    });
}
</script>

<style scoped>
.transfer-card { width: 760px; max-width: 95vw; background: #353535; color: #d1d5db; }
.transfer-body { max-height: 72vh; overflow: auto; }
.export-button { height: 24px; min-height: 24px; padding: 0 6px; border: 1px solid #484848; background: #2d2d2d; color: #a0a0a0; font-size: 11px; line-height: 1.2; }
.export-button :deep(.q-btn__content) { display: flex; flex-direction: row; flex-wrap: nowrap; align-items: center; gap: 4px; }
.export-button:hover:not(.disabled) { background: #484848; color: #fbbf24; }
.export-steps { display: flex; list-style: none; padding: 0; margin: 0 0 20px; gap: 20px; border-bottom: 1px solid #505050; }
.export-steps li { display: flex; align-items: center; gap: 8px; padding-bottom: 12px; color: #a0a0a0; }
.export-steps li.active { color: #fbbf24; border-bottom: 2px solid #fbbf24; }
.export-steps li.complete { color: #c9d2c7; }
.export-steps li > span { display: grid; place-items: center; width: 22px; height: 22px; border: 1px solid currentColor; border-radius: 50%; font-size: 12px; }
.export-panel { min-height: 260px; }
.export-panel h3 { font-size: 14px; font-weight: 600; line-height: 1.5; margin: 0 0 14px; }
.format-list { display: grid; gap: 8px; }
.format-option { display: flex; align-items: center; gap: 12px; padding: 12px; border: 1px solid #505050; border-radius: 4px; background: #303030; cursor: pointer; }
.format-option.selected { border-color: #bc913f; background: #403b30; }
.format-option input { accent-color: #fbbf24; }
.format-option strong, .format-option small { display: block; }
.format-option small { margin-top: 3px; color: #aeb1b5; }
.export-summary { display: grid; grid-template-columns: 110px minmax(0, 1fr); gap: 12px; font-size: 13px; }
.export-summary dt { color: #a0a0a0; }
.export-summary dd { margin: 0; overflow-wrap: anywhere; }
.file-path { overflow-wrap: anywhere; flex: 1; min-width: 120px; font-size: 12px; }
.mapping-row { display: grid; grid-template-columns: 1fr 2fr; gap: 12px; align-items: center; margin-top: 8px; }
.mapping-row > span { overflow-wrap: anywhere; }
.preview-scroll { overflow: auto; max-height: 210px; }
table { border-collapse: collapse; font-size: 12px; }
th, td { padding: 6px 10px; border: 1px solid #555; text-align: left; max-width: 280px; overflow-wrap: anywhere; white-space: pre-wrap; }
</style>
