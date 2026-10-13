<template>
    <div ref="container" class="min-h-0 min-w-0 overflow-hidden" />
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Compartment, EditorState } from "@codemirror/state";
import { EditorView, lineNumbers } from "@codemirror/view";
import { sql, PostgreSQL, MySQL, SQLite } from "@codemirror/lang-sql";
import { sqlTheme } from "@/utils/sqlTheme";

const props = defineProps<{ text: string; driver?: string }>();
const container = ref<HTMLElement | null>(null);
const language = new Compartment();
let view: EditorView | null = null;

function sqlLanguage() {
    return sql({ dialect: props.driver === "mysql" ? MySQL : props.driver === "sqlite" ? SQLite : PostgreSQL });
}

onMounted(() => {
    if (!container.value) return;
    view = new EditorView({
        parent: container.value,
        state: EditorState.create({
            doc: props.text,
            extensions: [
                sqlTheme,
                language.of(sqlLanguage()),
                lineNumbers(),
                EditorState.readOnly.of(true),
                EditorView.editable.of(false),
                EditorView.contentAttributes.of({ tabindex: "0", "aria-label": "Table DDL", "aria-readonly": "true" }),
                EditorView.theme({
                    "&": { height: "100%", backgroundColor: "#282828", color: "#d1d5db" },
                    ".cm-scroller": { overflow: "auto", fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace", fontSize: "12px", lineHeight: "1.7" },
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
