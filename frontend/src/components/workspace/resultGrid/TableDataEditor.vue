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
                ><q-icon name="add" size="15px" /> Add roww</q-btn
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
                @click="deleteRow"
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
            <span class="ml-auto text-gray-500">{{
                state.drafts.length
                    ? `${state.drafts.length} pending`
                    : "Double-click a row to edit"
            }}</span>
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
    <q-dialog v-model="dialog" persistent>
        <q-card class="row-editor-card">
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
            <q-form @submit="stageRow">
                <div class="row-editor-body">
                    <template v-if="state?.info?.driver === 'mongodb'">
                        <div class="row-editor-field">
                            <label for="mongo-row-id"
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
                                color="amber"
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
                            color="amber"
                            spellcheck="false"
                            aria-label="MongoDB document fields"
                        />
                    </template>
                    <div v-else class="row-editor-fields">
                        <div
                            v-for="(field, index) in fields"
                            :key="field.name"
                            class="row-editor-field"
                        >
                            <label :for="`row-field-${index}`"
                                >{{ field.name }}
                                <span class="field-type">{{
                                    field.type
                                }}</span></label
                            >
                            <q-input
                                :for="`row-field-${index}`"
                                v-model="field.text"
                                outlined
                                dense
                                dark
                                hide-bottom-space
                                color="amber"
                                class="editor-input"
                                :class="{
                                    'locked-input':
                                        field.locked || field.generated,
                                }"
                                :readonly="field.locked || field.generated"
                                :disable="
                                    !field.locked &&
                                    !field.generated &&
                                    field.mode !== 'value'
                                "
                                :placeholder="
                                    field.mode === 'default'
                                        ? 'Database default'
                                        : field.mode === 'null'
                                          ? 'NULL'
                                          : ''
                                "
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
                            <q-select
                                v-else
                                v-model="field.mode"
                                :aria-label="`${field.name} value mode`"
                                class="value-mode"
                                outlined
                                dense
                                dark
                                hide-bottom-space
                                color="amber"
                                emit-value
                                map-options
                                :options="valueModes(field)"
                                popup-content-class="bg-[#353535] text-gray-300 font-mono text-xs"
                            />
                        </div>
                    </div>
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
import { Notify } from "quasar";
import { useTableEditsStore } from "@/stores/tableEditsStore";
import { useQueryTabsStore } from "@/stores/queryTabsStore";
import type { QueryResult } from "@/types/queryTab";

const props = defineProps<{
    tabId: string;
    result: QueryResult;
    selectedKey: string;
    loading?: boolean;
}>();
const edits = useTableEditsStore();
const queries = useQueryTabsStore();
const state = computed(() => edits.states[props.tabId]);
const busy = computed(
    () => props.loading || state.value?.saving || state.value?.loading,
);
const selectedDraft = computed(() =>
    state.value?.drafts.find((row) => row.key === props.selectedKey),
);
const canEditSelected = computed(
    () =>
        !!props.selectedKey &&
        (selectedDraft.value?.operation === "insert" ||
            (state.value?.info?.canModify &&
                (selectedDraft.value ||
                    props.result.Rows.some(
                        (_row, index) =>
                            edits.rowKey(props.tabId, props.result, index) ===
                            props.selectedKey,
                    )))),
);
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
let original: Record<string, unknown> = {};
const valueModes = (field: { nullable: boolean }) => [
    { label: "Value", value: "value" },
    ...(field.nullable ? [{ label: "NULL", value: "null" }] : []),
    ...(isInsert.value ? [{ label: "Default", value: "default" }] : []),
];
const asText = (value: unknown) =>
    value == null
        ? ""
        : typeof value === "object"
          ? JSON.stringify(value)
          : String(value);
function openRow(key = "") {
    if (busy.value || !state.value?.info?.canInsert) return;
    const draft = state.value.drafts.find((row) => row.key === key);
    if (key && draft?.operation !== "insert" && !state.value.info.canModify)
        return;
    if (draft?.operation === "delete") {
        edits.remove(props.tabId, key);
        return;
    }
    editingKey.value = key;
    isInsert.value = !key || draft?.operation === "insert";
    const index = props.result.Rows.findIndex(
        (_row, index) => edits.rowKey(props.tabId, props.result, index) === key,
    );
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
        const index = props.result.Rows.findIndex(
            (_row, index) =>
                edits.rowKey(props.tabId, props.result, index) ===
                editingKey.value,
        );
        const existing = state.value?.drafts.find(
            (row) => row.key === editingKey.value,
        );
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
function deleteRow() {
    if (busy.value || !props.selectedKey || !canEditSelected.value) return;
    const draft = selectedDraft.value;
    if (draft?.operation === "insert" || draft?.operation === "delete") {
        edits.remove(props.tabId, props.selectedKey);
        return;
    }
    const index = props.result.Rows.findIndex(
        (_row, index) =>
            edits.rowKey(props.tabId, props.result, index) ===
            props.selectedKey,
    );
    if (index < 0 && !draft) return;
    edits.stage(props.tabId, {
        key: props.selectedKey,
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
defineExpose({ openRow });
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
    background: #353535;
    border-bottom: 1px solid #484848;
    font-family: monospace;
    font-size: 11px;
}
.table-edit-bar .q-btn {
    min-height: 24px;
    font: inherit;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 3px 6px;
    color: #9ca3af;
    white-space: nowrap;
}
.table-edit-bar .q-btn:hover:not(.disabled) {
    background: #484848;
    color: #fbbf24;
}
.table-edit-bar .q-btn.disabled {
    opacity: 0.35;
    cursor: default;
}
.table-edit-bar .q-btn.save {
    color: #fbbf24;
}
.divider {
    height: 16px;
    border-left: 1px solid #555555;
    margin: 0 3px;
}
.row-editor-card {
    width: 720px;
    max-width: 95vw;
    background: #353535;
    color: #d1d5db;
    border: 1px solid #555555;
    border-radius: 4px;
    font-family: monospace;
    font-size: 11px;
}
.row-editor-header,
.row-editor-footer {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
    background: #353535;
}
.row-editor-header {
    justify-content: space-between;
    border-bottom: 1px solid #484848;
}
.row-editor-heading {
    display: flex;
    align-items: center;
    gap: 8px;
    color: #fbbf24;
    font-weight: 600;
}
.row-editor-table {
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
    background: #484848;
    color: #fbbf24;
}
.row-editor-body {
    max-height: 65vh;
    overflow: auto;
    padding: 12px;
    background: #2d2d2d;
}
.row-editor-fields {
    display: flex;
    flex-direction: column;
    gap: 8px;
}
.row-editor-field {
    display: grid;
    grid-template-columns: minmax(100px, 150px) minmax(0, 1fr) 100px;
    align-items: center;
    gap: 10px;
}
.row-editor-field label {
    overflow-wrap: anywhere;
    color: #d1d5db;
}
.field-type {
    display: block;
    font-size: 10px;
    color: #a0a0a0;
    margin-top: 2px;
    font-weight: 400;
}
.editor-input,
.value-mode {
    width: 100%;
    min-width: 0;
    font: inherit;
}
.editor-input :deep(.q-field__control),
.value-mode :deep(.q-field__control) {
    min-height: 28px;
    height: 28px;
    border-radius: 2px;
    background: #353535;
}
.editor-input :deep(.q-field__control::before),
.value-mode :deep(.q-field__control::before) {
    border-color: #555555;
}
.editor-input :deep(.q-field__native),
.value-mode :deep(.q-field__native) {
    font: inherit;
    padding: 3px 0;
}
.editor-input :deep(.q-field__marginal),
.value-mode :deep(.q-field__marginal) {
    height: 28px;
}
.locked-input :deep(.q-field__native) {
    color: #a0a0a0;
}
.locked-input :deep(.q-field__control) {
    background: #303030;
}
.field-lock {
    display: flex;
    align-items: center;
    gap: 5px;
    color: #a0a0a0;
    font-size: 10px;
    white-space: nowrap;
}
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
    border-top: 1px solid #484848;
}
.row-editor-footer > span {
    flex: 1;
    color: #a0a0a0;
    font-size: 10px;
}
.editor-button {
    min-height: 28px;
    font: inherit;
    padding: 6px 10px;
    border: 1px solid #555555;
    border-radius: 2px;
    color: #9ca3af;
    background: #353535;
    white-space: nowrap;
}
.editor-button:hover {
    background: #484848;
    color: #fbbf24;
}
.editor-button-primary {
    border-color: #a97724;
    color: #fbbf24;
}
@media (max-width: 520px) {
    .row-editor-field {
        grid-template-columns: minmax(75px, 110px) minmax(0, 1fr) 80px;
        gap: 6px;
    }
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
    font-size: 12px;
}
</style>
