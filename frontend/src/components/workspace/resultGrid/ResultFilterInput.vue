<template>
    <div ref="container" class="result-filter-editor" :title="error || applied" />
</template>

<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch } from "vue";
import { Compartment, EditorState } from "@codemirror/state";
import { EditorView, keymap, placeholder, tooltips } from "@codemirror/view";
import { completeResultExpression } from "@/utils/resultFilterCompletion";
import { sql, SQLite } from "@codemirror/lang-sql";
import { autocompletion, completionKeymap, acceptCompletion, type CompletionContext } from "@codemirror/autocomplete";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { sqlTheme } from "@/utils/sqlTheme";
import { sqlSelection } from "@/utils/sqlSelection";

const props = defineProps<{
    modelValue: string;
    columns: { Name: string; Type?: string }[];
    error: string;
    applied: string;
    mode?: "where" | "order";
}>();
const emit = defineEmits<{
    "update:modelValue": [value: string];
    apply: [];
    clear: [];
}>();
const container = ref<HTMLElement | null>(null);
const accessibility = new Compartment();
let view: EditorView | null = null;
const attributes = () => EditorView.contentAttributes.of({
    "aria-label": props.mode === "order" ? "Result order by expression" : "Result filter expression",
    "aria-invalid": String(!!props.error),
    spellcheck: "false",
});

function complete(context: CompletionContext) {
    return completeResultExpression(context, props.columns, props.mode);
}

onMounted(() => {
    if (!container.value) return;
    view = new EditorView({
        parent: container.value,
        state: EditorState.create({
            doc: props.modelValue,
            extensions: [
                sql({ dialect: SQLite }), sqlTheme, sqlSelection,
                history(),
                placeholder(props.mode === "order" ? "column ASC, other DESC…" : "Enter filter expression…"),
                accessibility.of(attributes()),
                autocompletion({ override: [complete], defaultKeymap: false }),
                tooltips({ parent: document.body }),
                keymap.of([
                    ...completionKeymap,
                    { key: "Tab", run: acceptCompletion },
                    { key: "Enter", run: () => { emit("apply"); return true; } },
                    { key: "Escape", run: () => { emit("clear"); return true; } },
                    ...historyKeymap, ...defaultKeymap,
                ]),
                EditorState.transactionFilter.of(transaction => {
                    if (!transaction.docChanged || transaction.newDoc.lines === 1) return transaction;
                    return [transaction, { changes: { from: 0, to: transaction.newDoc.length,
                        insert: transaction.newDoc.toString().replace(/[\r\n]+/g, " ") }, sequential: true }];
                }),
                EditorView.updateListener.of(update => {
                    if (update.docChanged) emit("update:modelValue", update.state.doc.toString());
                }),
                EditorView.theme({
                    // CodeMirror also applies theme classes to body-mounted tooltip
                    // hosts. Only the editor should occupy space in the layout.
                    "&.cm-editor": { backgroundColor: "transparent", height: "23px", fontSize: "var(--ui-font-size)" },
                    "&.cm-focused": { outline: "none" },
                    ".cm-scroller": { overflow: "hidden", fontFamily: "monospace", lineHeight: "23px" },
                    ".cm-content": { padding: "0", minHeight: "23px" },
                    ".cm-line": { padding: "0" },
                    ".cm-placeholder": { color: "var(--text-muted)" },
                    ".cm-tooltip-autocomplete": { fontSize: "var(--ui-font-size)" },
                }),
                // Escape the result grid's overflow clipping.
                EditorView.theme({ ".cm-tooltip": { zIndex: "1000" } }),
                EditorView.domEventHandlers({ keydown(event) { event.stopPropagation(); } }),
            ],
        }),
    });
});
watch(() => props.modelValue, value => {
    if (view && value !== view.state.doc.toString()) {
        view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } });
    }
});
watch(() => props.error, () => view?.dispatch({ effects: accessibility.reconfigure(attributes()) }));
onBeforeUnmount(() => { view?.destroy(); view = null; });
</script>

<style scoped>
.result-filter-editor { flex: 1; min-width: 0; }
</style>
