<template>
    <div ref="container" class="sql-preview min-h-0 min-w-0 overflow-hidden" />
</template>

<script setup lang="ts">
import { getCurrentInstance, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Compartment, EditorState } from "@codemirror/state";
import { EditorView, keymap, lineNumbers } from "@codemirror/view";
import { selectAll } from "@codemirror/commands";
import { search, searchKeymap } from "@codemirror/search";
import { sql } from "@codemirror/lang-sql";
import { json } from "@codemirror/lang-json";
import { editorDialect, isJSONDriver } from '@/utils/databaseDialect';
import { sqlTheme } from "@/utils/sqlTheme";
import { sqlSelection } from "@/utils/sqlSelection";
import { createSQLSearchPanel } from "@/utils/sqlSearchPanel";

const props = defineProps<{ text: string; driver?: string }>();
const container = ref<HTMLElement | null>(null);
const appContext = getCurrentInstance()?.appContext ?? null;
const language = new Compartment();
let view: EditorView | null = null;

function sqlLanguage() {
    return isJSONDriver(props.driver) ? json() : sql({ dialect: editorDialect(props.driver) });
}

onMounted(() => {
    if (!container.value) return;
    view = new EditorView({
        parent: container.value,
        state: EditorState.create({
            doc: props.text,
            extensions: [
                sqlTheme,
                sqlSelection,
                search({ top: true, createPanel: view => createSQLSearchPanel(view, appContext) }),
                keymap.of([{ key: "Mod-a", run: selectAll }, ...searchKeymap]),
                language.of(sqlLanguage()),
                lineNumbers(),
                EditorState.readOnly.of(true),
                EditorView.editable.of(false),
                EditorView.contentAttributes.of({ tabindex: "0", "aria-label": "Table DDL", "aria-readonly": "true" }),
                EditorView.theme({
                    "&": { position: "absolute", inset: "0", height: "100%", width: "100%", minWidth: "0", minHeight: "0", backgroundColor: "#282828", color: "#d1d5db" },
                    ".cm-scroller": { overflow: "auto", minWidth: "0", minHeight: "0", fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace", fontSize: "12px", lineHeight: "1.7" },
                    ".cm-content": { padding: "12px 0" },
                    ".cm-gutters": { backgroundColor: "#2d2d2d", color: "#909090", border: "none" },
                }),
            ],
        }),
    });
});

watch(() => props.text, text => {
    if (view && text !== view.state.doc.toString()) {
        view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: text } });
    }
});
watch(() => props.driver, () => view?.dispatch({ effects: language.reconfigure(sqlLanguage()) }));
onBeforeUnmount(() => { view?.destroy(); view = null; });
</script>

<style scoped>
.sql-preview {
    position: relative;
    flex-basis: 0;
    max-width: 100%;
}
</style>
