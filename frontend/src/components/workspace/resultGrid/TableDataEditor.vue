<template>
    <div class="table-edit-bar">
        <template v-if="state?.info?.canInsert">
            <q-btn
                flat
                dense
                no-caps
                type="button"
                :disable="busy"
                @click="openRow()"
                ><q-icon name="add" size="15px" /> Add row</q-btn
            >
            <q-btn
                flat
                dense
                no-caps
                type="button"
                :disable="busy || !selectedKey || !canEditSelected"
                @click="openRow(selectedKey)"
                ><q-icon name="edit" size="14px" /> Edit row</q-btn
            >
            <q-btn
                flat
                dense
                no-caps
                type="button"
                :disable="busy || !selectedKey || !canEditSelected"
                @click="deleteRow()"
                ><q-icon name="delete_outline" size="15px" />
                {{
                    selectedDraft?.operation === "delete"
                        ? "Undo delete"
                        : "Delete"
                }}</q-btn
            >
            <span class="divider" />
            <q-btn
                flat
                dense
                no-caps
                type="button"
                class="save"
                :disable="busy || !state.drafts.length"
                @click="save"
                ><q-icon name="save" size="14px" />
                {{ state.saving ? "Saving…" : "Save" }}</q-btn
            >
            <q-btn
                flat
                dense
                no-caps
                type="button"
                :disable="busy || !state.drafts.length"
                @click="edits.discard(tabId)"
                ><q-icon name="undo" size="14px" /> Cancel</q-btn
            >
            <span v-if="state.drafts.length" class="ml-auto text-gray-500">{{ state.drafts.length }} pending</span>
        </template>
        <span v-else class="text-gray-500">{{
            state?.loading
                ? "Loading table editor…"
                : state?.info?.reason ||
                  state?.error ||
                  "Table editor unavailable"
        }}</span>
        <q-icon
            v-if="state?.info?.reason && state.info.canInsert"
            name="info_outline"
            size="14px"
            ><q-tooltip>{{ state.info.reason }}</q-tooltip></q-icon
        >
        <q-icon
            v-if="state?.error && state.info?.canInsert"
            name="error_outline"
            size="15px"
            class="text-red-400"
            ><q-tooltip>{{ state.error }}</q-tooltip></q-icon
        >
    </div>
    <div
        v-if="state?.error && state.info?.canInsert"
        class="edit-error"
        role="alert"
    >
        {{ state.error }}
    </div>
    <q-dialog v-model="dialog" persistent :transition-duration="0">
        <q-card class="row-editor-card" :style="{ '--editor-control-height': `${fieldRowHeight - 12}px` }">
            <header class="row-editor-header">
                <div class="row-editor-heading">
                    <q-icon
                        :name="isInsert ? 'add' : 'edit'"
                        size="16px"
                    /><span>{{ isInsert ? "Add row" : "Edit row" }}</span
                    ><span class="row-editor-table">{{
                        state?.info?.table.name
                    }}</span>
                </div>
                <q-btn
                    flat
                    dense
                    no-caps
                    type="button"
                    class="editor-icon-button"
                    aria-label="Close row editor"
                    @click="dialog = false"
                    ><q-icon name="close" size="16px"
                /></q-btn>
            </header>
            <q-form class="row-editor-form" @submit="stageRow">
                <div class="row-editor-body" :class="{ 'document-editor-body': state?.info?.driver === 'mongodb' }">
                    <template v-if="state?.info?.driver === 'mongodb'">
                        <div class="row-editor-field">
                            <label class="field-label" for="mongo-row-id"
                                >_id
                                <span class="field-type"
                                    >Document ID</span
                                ></label
                            >
                            <q-input
                                for="mongo-row-id"
                                class="editor-input locked-input"
                                :model-value="
                                    lockedDocumentId == null
                                        ? 'Automatically generated'
                                        : asText(lockedDocumentId)
                                "
                                outlined
                                dense
                                dark
                                hide-bottom-space
                                color="primary"
                                readonly
                            />
                            <span class="field-lock"
                                ><q-icon name="lock" size="12px" />
                                Read-only</span
                            >
                        </div>
                        <label for="mongo-document-body" class="document-label"
                            >Document fields
                            <span class="field-type">Extended JSON</span></label
                        >
                        <q-input
                            for="mongo-document-body"
                            v-model="documentText"
                            type="textarea"
                            class="editor-input document-input"
                            outlined
                            dense
                            dark
                            hide-bottom-space
                            color="primary"
                            spellcheck="false"
                            aria-label="MongoDB document fields"
                        />
                    </template>
                    <q-virtual-scroll v-else ref="fieldScroller" :items="fields" :virtual-scroll-item-size="fieldRowHeight"
                        :virtual-scroll-slice-size="16" :virtual-scroll-slice-ratio-before="0.5" :virtual-scroll-slice-ratio-after="0.5"
                        class="row-editor-fields">
                        <template #default="{ item: field, index }">
                        <div
                            :key="field.name"
                            v-memo="[field, field.text, field.mode]"
                            class="row-editor-field"
                        >
                            <label class="field-label" :for="`row-field-${index}`">
                                <span class="field-name" :title="field.name">{{ field.name }}</span>
                                <span class="field-type" :title="field.type">{{ field.type }}</span>
                            </label>
                            <q-input
                                :for="`row-field-${index}`" v-model="field.text"
                                class="editor-input" :class="{ 'locked-input': field.locked || field.generated }"
                                outlined dense dark hide-bottom-space color="primary"
                                autocomplete="off" spellcheck="false" :autofocus="index === firstEditableField"
                                :readonly="field.locked || field.generated"
                                :disable="!field.locked && !field.generated && field.mode !== 'value'"
                                :placeholder="field.mode === 'default' ? 'Database default' : field.mode === 'null' ? 'NULL' : ''"
                            />
                            <span
                                v-if="field.locked || field.generated"
                                class="field-lock"
                                ><q-icon name="lock" size="12px" />
                                {{
                                    field.generated
                                        ? "Generated"
                                        : field.autoIncrement && isInsert
                                          ? "Auto ID"
                                          : "Read-only"
                                }}</span
                            >
                            <q-select v-else v-model="field.mode" :for="`row-mode-${index}`"
                                :aria-label="`${field.name} value mode`" class="value-mode"
                                outlined dense dark hide-bottom-space color="primary" emit-value map-options
                                :options="valueModes(field)" options-dense behavior="menu"
                                popup-content-class="bg-[var(--surface-panel)] text-gray-300" />
                        </div>
                        </template>
                    </q-virtual-scroll>
                    <p v-if="dialogError" class="dialog-error" role="alert">
                        {{ dialogError }}
                    </p>
                </div>
                <footer class="row-editor-footer">
                    <span>Stage changes, then Save to commit.</span>
                    <q-btn
                        flat
                        dense
                        no-caps
                        type="button"
                        class="editor-button"
                        @click="dialog = false"
                        >Cancel</q-btn
                    >
                    <q-btn
                        flat
                        dense
                        no-caps
                        type="submit"
                        class="editor-button editor-button-primary"
                        >Stage changes</q-btn
                    >
                </footer>
            </q-form>
        </q-card>
    </q-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { Notify, type QVirtualScroll } from "quasar";
import { useTableEditsStore } from "@/stores/tableEditsStore";
import { useQueryTabsStore } from "@/stores/queryTabsStore";
import type { QueryResult } from "@/types/queryTab";
import { useResultSettingsStore } from "@/stores/resultSettingsStore";

const props = defineProps<{
    tabId: string;
    result: QueryResult;
    selectedKey: string;
    rowIndexes?: ReadonlyMap<string, number>;
    loading?: boolean;
}>();
const edits = useTableEditsStore();
const queries = useQueryTabsStore();
const state = computed(() => edits.states[props.tabId]);
const busy = computed(
    () => props.loading || state.value?.saving || state.value?.loading,
);
const selectedDraft = computed(() =>
    draftsByKey.value.get(props.selectedKey),
);
// These indexes change with the page or drafts, not with every selected row.
const draftsByKey = computed(() => new Map(state.value?.drafts.map(row => [row.key, row]) ?? []));
const rowIndexes = computed(() => props.rowIndexes ?? new Map(props.result.Rows.map((_row, index) =>
    [edits.rowKey(props.tabId, props.result, index), index] as const)));
function canEditRow(key: string) {
    if (busy.value || !key || !state.value?.info?.canInsert) return false;
    const draft = draftsByKey.value.get(key);
    return draft?.operation === 'insert' || !!state.value.info.canModify && (!!draft || rowIndexes.value.has(key));
}
const canEditSelected = computed(() => canEditRow(props.selectedKey));
const dialog = ref(false);
const dialogError = ref("");
const editingKey = ref("");
const isInsert = ref(false);
const documentText = ref("{}");
const lockedDocumentId = ref<unknown>(null);
const fields = ref<
    Array<{
        name: string;
        text: string;
        mode: string;
        type: string;
        nullable: boolean;
        generated: boolean;
        autoIncrement: boolean;
        locked: boolean;
    }>
>([]);
const fieldScroller = ref<QVirtualScroll | null>(null);
const resultSettings = useResultSettingsStore();
const fieldRowHeight = computed(() => Math.max(40, resultSettings.settings.fontSize + 24) + 12);
const firstEditableField = computed(() => fields.value.findIndex(field => !field.locked && !field.generated && field.mode === 'value'));
let original: Record<string, unknown> = {};
const valueOption = { label: 'Value', value: 'value' };
const nullOption = { label: 'NULL', value: 'null' };
const defaultOption = { label: 'Default', value: 'default' };
const editModes = [[valueOption], [valueOption, nullOption]];
const insertModes = [[valueOption, defaultOption], [valueOption, nullOption, defaultOption]];
const valueModes = (field: { nullable: boolean }) => (isInsert.value ? insertModes : editModes)[Number(field.nullable)];
const asText = (value: unknown) =>
    value == null
        ? ""
        : typeof value === "object"
          ? JSON.stringify(value)
          : String(value);
function openRow(key = "") {
    if (busy.value || !state.value?.info?.canInsert) return;
    const draft = draftsByKey.value.get(key);
    if (key && draft?.operation !== "insert" && !state.value.info.canModify)
        return;
    if (draft?.operation === "delete") {
        edits.remove(props.tabId, key);
        return;
    }
    editingKey.value = key;
    isInsert.value = !key || draft?.operation === "insert";
    const index = key ? rowIndexes.value.get(key) ?? -1 : -1;
    original =
        draft?.original ??
        (index < 0
            ? {}
            : Object.fromEntries(
                  props.result.Columns.map((column, col) => [
                      column.Name,
                      props.result.Rows[index]?.[col],
                  ]),
              ));
    if (state.value.info.driver === "mongodb") {
        original =
            draft?.original ??
            (index < 0
                ? {}
                : JSON.parse(props.result.Documents?.[index] ?? "{}"));
        const document = draft?.values ?? original;
        lockedDocumentId.value = document._id ?? null;
        const { _id, ...editableFields } = document;
        documentText.value = JSON.stringify(editableFields, null, 2);
    } else {
        fields.value = (state.value.info.columns ?? []).map((column) => {
            const present = draft && Object.hasOwn(draft.values, column.name);
            const value = present
                ? draft.values[column.name]
                : original[column.name];
            const optional =
                column.autoIncrement ||
                column.generated ||
                column.nullable ||
                column.defaultValue != null;
            return {
                name: column.name,
                text: asText(value),
                type: column.databaseType,
                nullable: column.nullable && !column.primaryKey,
                generated: column.generated,
                autoIncrement: column.autoIncrement,
                locked:
                    (!isInsert.value &&
                        (column.primaryKey ||
                            state.value?.info?.keys?.includes(column.name) ||
                            column.name.toLowerCase() === "id")) ||
                    (isInsert.value && column.autoIncrement),
                mode:
                    isInsert.value && !present && optional
                        ? "default"
                        : value === null
                          ? "null"
                          : "value",
            };
        });
    }
    dialogError.value = "";
    dialog.value = true;
}
function stageRow() {
    try {
        const info = state.value?.info;
        if (!info) return;
        let values: Record<string, unknown> = {};
        if (info.driver === "mongodb") {
            values = JSON.parse(documentText.value);
            if (!values || Array.isArray(values) || typeof values !== "object")
                throw new Error("Enter a JSON document object");
            const validateNumbers = (value: unknown): void => {
                if (
                    typeof value === "number" &&
                    Number.isInteger(value) &&
                    !Number.isSafeInteger(value)
                )
                    throw new Error(
                        "Use Extended JSON $numberLong for integers outside the safe numeric range",
                    );
                if (value && typeof value === "object")
                    Object.values(value).forEach(validateNumbers);
            };
            validateNumbers(values);
            if (Object.hasOwn(values, "_id"))
                throw new Error(
                    "_id is read-only; edit the document fields only",
                );
            if (lockedDocumentId.value != null)
                values._id = lockedDocumentId.value;
            if (isInsert.value && !Object.hasOwn(values, "_id")) {
                const bytes = crypto.getRandomValues(new Uint8Array(12));
                values._id = {
                    $oid: Array.from(bytes, (byte) =>
                        byte.toString(16).padStart(2, "0"),
                    ).join(""),
                };
            }
            if (
                !isInsert.value &&
                JSON.stringify(values._id) !== JSON.stringify(original._id)
            )
                throw new Error("The existing _id cannot be changed");
        } else {
            for (const field of fields.value) {
                if (field.locked || field.generated || field.mode === "default")
                    continue;
                let value: unknown = field.mode === "null" ? null : field.text;
                if (
                    field.mode === "value" &&
                    /^(boolean|bool)$/i.test(field.type)
                ) {
                    if (!/^(true|false|0|1)$/i.test(field.text))
                        throw new Error(`${field.name}: enter true or false`);
                    value = /^(true|1)$/i.test(field.text);
                }
                if (
                    field.mode === "value" &&
                    /(int|numeric|decimal|real|double|float)/i.test(
                        field.type,
                    ) &&
                    !/^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/.test(
                        field.text,
                    )
                )
                    throw new Error(
                        `${field.name}: enter a number or select NULL / Default`,
                    );
                if (
                    isInsert.value ||
                    (value === null
                        ? original[field.name] != null
                        : original[field.name] == null ||
                          asText(value) !== asText(original[field.name]))
                )
                    values[field.name] = value;
            }
        }
        if (
            !isInsert.value &&
            info.driver !== "mongodb" &&
            !Object.keys(values).length
        ) {
            edits.remove(props.tabId, editingKey.value);
            dialog.value = false;
            return;
        }
        const index = rowIndexes.value.get(editingKey.value) ?? -1;
        const existing = draftsByKey.value.get(editingKey.value);
        const keys = isInsert.value
            ? {}
            : (existing?.keys ??
              edits.keysFor(props.tabId, props.result, index));
        if (
            Object.values(keys).some(
                (value) =>
                    typeof value === "number" && !Number.isSafeInteger(value),
            )
        )
            throw new Error(
                "This primary key cannot be represented precisely; reload before editing",
            );
        edits.stage(props.tabId, {
            key: editingKey.value || `new-${crypto.randomUUID()}`,
            operation: isInsert.value ? "insert" : "update",
            keys,
            values,
            original,
        });
        dialog.value = false;
    } catch (error) {
        dialogError.value =
            error instanceof Error ? error.message : String(error);
    }
}
function deleteRow(key = props.selectedKey) {
    if (!canEditRow(key)) return;
    const draft = draftsByKey.value.get(key);
    if (draft?.operation === "insert" || draft?.operation === "delete") {
        edits.remove(props.tabId, key);
        return;
    }
    const index = rowIndexes.value.get(key) ?? -1;
    if (index < 0 && !draft) return;
    edits.stage(props.tabId, {
        key,
        operation: "delete",
        keys: draft?.keys ?? edits.keysFor(props.tabId, props.result, index),
        values: {},
        original: draft?.original,
    });
}
async function save() {
    const tab = queries.tabs.find((tab) => tab.id === props.tabId);
    if (!tab || busy.value) return;
    const saved = await edits.save(props.tabId, tab.cursor);
    tab.cursor = "";
    tab.streamNextPage = undefined;
    if (saved) {
        Notify.create({
            message: state.value?.error
                ? "Some changes saved; remaining changes are still staged"
                : "Changes saved to database",
            color: state.value?.error ? "warning" : "positive",
        });
        await queries.execute(props.tabId, tab.executedSql ?? tab.sql, true);
    }
}
defineExpose({ openRow, deleteRow, canEditRow });
</script>

<style scoped>
.table-edit-bar {
    position: absolute;
    inset: 36px 0 auto;
    height: 34px;
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 0 8px;
    background: var(--surface-panel);
    border-bottom: 1px solid var(--border-default);
    font-family: monospace;
    font-size: var(--ui-font-size);
}
.table-edit-bar .q-btn {
    min-height: 24px;
    font: inherit;
    line-height: 1.2;
    padding: 3px 6px;
    color: #9ca3af;
    white-space: nowrap;
}
.table-edit-bar .q-btn:hover:not(.disabled) {
    background: var(--border-default);
    color: var(--accent-color);
}
.table-edit-bar .q-btn.disabled {
    opacity: 0.35;
    cursor: default;
}
.table-edit-bar .q-btn.save {
    color: var(--accent-color);
}
.divider {
    height: 16px;
    border-left: 1px solid #555555;
    margin: 0 3px;
}
.row-editor-card.q-card {
    width: 840px;
    max-width: calc(100vw - 32px);
    max-height: calc(100dvh - 48px);
    overflow: hidden;
    display: flex;
    flex-direction: column;
    background: var(--surface-panel);
    color: var(--text-primary);
    border: 1px solid #555555;
    border-radius: 4px;
    font-family: monospace;
    font-size: var(--ui-font-size);
}
.row-editor-header,
.row-editor-footer {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
    background: var(--surface-panel);
}
.row-editor-header {
    justify-content: space-between;
    border-bottom: 1px solid var(--border-default);
}
.row-editor-heading {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--accent-color);
    font-weight: 600;
}
.row-editor-table {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: #9ca3af;
    font-weight: 400;
    margin-left: 4px;
}
.editor-icon-button {
    min-height: 24px;
    padding: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    color: #9ca3af;
}
.editor-icon-button:hover {
    background: var(--border-default);
    color: var(--accent-color);
}
.row-editor-form { display: flex; flex-direction: column; min-height: 0; }
.row-editor-body { display: flex; flex-direction: column; min-height: 0; background: var(--surface-data); }
.row-editor-fields { min-height: 0; max-height: min(65vh, calc(100dvh - 180px)); overflow: auto; padding: 12px 16px; scrollbar-width: thin; }
.row-editor-field {
    display: grid;
    grid-template-columns: minmax(120px, 180px) minmax(0, 1fr) 144px;
    align-items: center;
    column-gap: 16px;
    min-height: calc(var(--editor-control-height, 40px) + 12px);
    padding: 6px 0;
}
.row-editor-field > .field-label { min-width: 0; color: var(--text-primary); }
.field-name, .field-type { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.field-type { font-size: var(--ui-font-size); color: var(--text-muted); margin-top: 3px; font-weight: 400; }
.editor-input, .value-mode { width: 100%; min-width: 0; font: inherit; }
.editor-input :deep(.q-field__control), .value-mode :deep(.q-field__control) {
    min-height: var(--editor-control-height, 40px);
    height: var(--editor-control-height, 40px);
    border-radius: 5px;
    background: var(--surface-panel);
}
.editor-input :deep(.q-field__control::before), .value-mode :deep(.q-field__control::before) { border-color: var(--border-default); }
.editor-input :deep(.q-field__native), .value-mode :deep(.q-field__native) { font: inherit; }
.editor-input :deep(.q-field__marginal), .value-mode :deep(.q-field__marginal) { height: var(--editor-control-height, 40px); }
.value-mode :deep(.q-field__native) { min-width: 0; white-space: nowrap; }
.value-mode :deep(.q-field__append) { padding-left: 4px; }
.locked-input :deep(.q-field__native) { color: var(--text-muted); }
.locked-input :deep(.q-field__control) { background: var(--surface-base); }
.field-lock { display: flex; align-items: center; gap: 5px; color: var(--text-muted); font-size: var(--ui-font-size); white-space: nowrap; }
.document-editor-body { max-height: min(65vh, calc(100dvh - 180px)); overflow: auto; padding: 12px 16px; }
.dialog-error { padding: 0 16px 12px; }
.document-label {
    display: block;
    margin: 16px 0 8px;
}
.document-input :deep(.q-field__control) {
    height: auto;
    min-height: 260px;
}
.document-input :deep(textarea) {
    min-height: 240px;
    line-height: 1.6;
    resize: vertical;
}
.dialog-error {
    color: #fca5a5;
    margin-top: 12px;
}
.row-editor-footer {
    border-top: 1px solid var(--border-default);
}
.row-editor-footer > span {
    flex: 1;
    color: var(--text-muted);
    font-size: var(--ui-font-size);
}
.editor-button {
    min-height: 28px;
    font: inherit;
    padding: 6px 10px;
    border: 1px solid #555555;
    border-radius: 2px;
    color: #9ca3af;
    background: var(--surface-panel);
    white-space: nowrap;
}
.editor-button:hover {
    background: var(--border-default);
    color: var(--accent-color);
}
.editor-button-primary {
    border-color: var(--accent-color);
    color: var(--accent-color);
}
@media (max-width: 520px) {
    .row-editor-field {
        grid-template-columns: minmax(0, 1fr) 144px;
        column-gap: 10px;
    }
    .row-editor-field > .field-label { grid-column: 1 / -1; margin-bottom: 6px; }
    .row-editor-fields { padding: 10px 12px; }
    .row-editor-footer > span {
        display: none;
    }
}
.edit-error {
    position: absolute;
    z-index: 110;
    bottom: 36px;
    inset-inline: 0;
    padding: 8px 12px;
    background: #351c1c;
    color: #fca5a5;
    font-size: var(--ui-font-size);
}
</style>
