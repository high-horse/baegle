<template>
    <div
        ref="editorContainer"
        class="h-full w-full min-h-0 min-w-0 max-w-full overflow-hidden"
    />
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from "vue";
import { Prec, Compartment } from "@codemirror/state";
import { linter, type Diagnostic } from "@codemirror/lint";
import { syntaxTree } from "@codemirror/language";
import { EditorView, keymap } from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { search, searchKeymap } from "@codemirror/search";
import { EditorSelection } from "@codemirror/state";
import { sql, PostgreSQL, MySQL, SQLite, keywordCompletionSource, schemaCompletionSource } from "@codemirror/lang-sql";
import { nord } from "@fsegurai/codemirror-theme-nord";
import { materialDark } from "@fsegurai/codemirror-theme-material-dark";
import { oneDark } from "@codemirror/theme-one-dark";
import { buildSQLCompletionSchema, getSQLTableReferences, matchingSQLTable } from "@/utils/sqlCompletion";
import type { InspectTableInfo } from "@bindings/db-lens/internal/engine/entities";
import { useConnectionStore } from "@/stores/connectionStore";
import { acceptCompletion, autocompletion, type CompletionContext } from "@codemirror/autocomplete";

const props = defineProps<{
    modelValue: string;
    dbDriver: "pgx" | "mysql" | "sqlite" | "mongodb";
    tables?: InspectTableInfo[];
}>();

const emit = defineEmits<{
    "update:modelValue": [value: string];
    execute: [sql: string];
}>();

const connectionStore = useConnectionStore();

const editorContainer = ref<HTMLElement | null>(null);

let editorView: EditorView | null = null;
const sqlDialect = new Compartment();

function getDialect(driver: string) {
    switch (driver) {
        case "pgx":
            return PostgreSQL;
        case "mysql":
            return MySQL;
        case "sqlite":
            return SQLite;
        default:
            return PostgreSQL;
    }
}

async function completeSQLSchema(context: CompletionContext) {
    if (props.dbDriver === "mongodb") return null;
    const tables = props.tables ?? [];
    const requested = new Set(getSQLTableReferences(context.state, context.pos)
        .map(reference => matchingSQLTable(tables, reference.path)).filter((table): table is InspectTableInfo => !!table));
    await Promise.all([...requested].map(table => connectionStore.loadTableColumns(table).catch(() => [])));
    if (context.aborted || tables !== props.tables) return null;
    const find = (table: { name: string; schema?: string }) => tables.find(item => item.name === table.name && item.schema === table.schema);
    return schemaCompletionSource({ dialect: getDialect(props.dbDriver),
        schema: buildSQLCompletionSchema(tables, props.dbDriver,
            table => { const cached = find(table); return cached ? connectionStore.getCachedTableColumns(cached) : []; },
            table => { const selected = find(table); if (selected) void connectionStore.loadTableColumns(selected).catch(() => {}); }),
    })(context);
}

function languageExtension(driver: string) {
    return [
        driver === "mongodb" ? [] : sql({ dialect: getDialect(driver), upperCaseKeywords: true }),
        autocompletion({
            activateOnTyping: true,
            defaultKeymap: true,
            closeOnBlur: true,
            override: driver === "mongodb" ? [] : [completeSQLSchema, keywordCompletionSource(getDialect(driver), true)],
        }),
    ];
}

function sqlSyntaxLinter(view: EditorView): Diagnostic[] {
    const diagnostics: Diagnostic[] = [];

    syntaxTree(view.state).iterate({
        enter(node) {
            if (node.type.isError) {
                diagnostics.push({
                    from: node.from,
                    to: Math.max(node.to, node.from + 1),
                    severity: "error",
                    message: "SQL syntax error",
                });
            }
        },
    });

    return diagnostics;
}

function getCurrentQuery(view: EditorView): string | null {
    const { from, to, head } = view.state.selection.main;

    // 1. If user has selected text, execute the selection
    if (from !== to) {
        const selected = view.state.doc.sliceString(from, to).trim();
        return selected || null;
    }

    if (props.dbDriver === "mongodb") return view.state.doc.toString().trim() || null;

    // 2. Use the syntax tree to find the Statement at cursor
    const tree = syntaxTree(view.state);
    const statements = tree.topNode.getChildren("Statement");

    for (const stmt of statements) {
        if (head >= stmt.from && head <= stmt.to) {
            view.dispatch({
                selection: EditorSelection.range(stmt.from, stmt.to),
            });
            return (
                view.state.doc.sliceString(stmt.from, stmt.to).trim() || null
            );
        }
    }

    // 3. Cursor is between statements (e.g. blank line). Optional: pick nearest.
    // Uncomment below if you want "run nearest" instead of "do nothing".
    /*
  let nearest: (typeof statements)[0] | null = null;
  let minDist = Infinity;
  for (const stmt of statements) {
    const dist = Math.min(Math.abs(head - stmt.from), Math.abs(head - stmt.to));
    if (dist < minDist) {
      minDist = dist;
      nearest = stmt;
    }
  }
  if (nearest && minDist <= 2) {
    return view.state.doc.sliceString(nearest.from, nearest.to).trim() || null;
  }
  */

    return null;
}

function executeCurrentQuery() {
    if (!editorView) {
        return;
    }

    const sql = getCurrentQuery(editorView);

    if (!sql) {
        return;
    }

    console.log("Executing SQL:", sql);
    emit("execute", sql);
}
defineExpose({
    execute: executeCurrentQuery,
});

onMounted(() => {
    if (!editorContainer.value) {
        return;
    }

    const startState = EditorState.create({
        doc: props.modelValue,

        extensions: [
            // SQL syntax highlighting
            sqlDialect.of(
                languageExtension(props.dbDriver),
            ),

            // linter(sqlSyntaxLinter, {
            //     delay: 300,
            // }),
            // 
            // Dark editor theme
            materialDark, // oneDark, // nord,
            // Basic editing
            //
            search({ top: true }),
            history(),
            keymap.of([{ key: "Tab", run: acceptCompletion }, ...historyKeymap, ...searchKeymap, ...defaultKeymap, indentWithTab]),
            EditorView.lineWrapping,
            // Custom styling
            EditorView.theme({
                "&": {
                    height: "100%",
                    width: "100%",
                    minWidth: "0",
                    minHeight: "0",
                    maxWidth: "100%",
            
                    backgroundColor: "#282828",
                    color: "#d1d5db",
                },
            
                ".cm-scroller": {
                    overflow: "auto",
                    flex: "1 1 0",
                    maxHeight: "100%",
                    minWidth: "0",
                    minHeight: "0",
                    maxWidth: "100%",
                },
            
                ".cm-content": {
                    padding: "16px",
            
                    fontFamily:
                        "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
            
                    fontSize: "12px",
                    lineHeight: "1.7",
                    caretColor: "#f59e0b",
            
                    minWidth: "0",
                },
            
                ".cm-line": {
                    minWidth: "0",
                },
            
                ".cm-gutters": {
                    backgroundColor: "#2d2d2d",
                    color: "#909090",
                    border: "none",
                },
            
                ".cm-activeLineGutter": {
                    backgroundColor: "#404040",
                    color: "#f59e0b",
                },
            
                ".cm-activeLine": {
                    backgroundColor: "rgba(245, 158, 11, 0.04)",
                },
            
                ".cm-selectionBackground": {
                    backgroundColor:
                        "rgba(245, 158, 11, 0.20) !important",
                },
            
                ".cm-tooltip-autocomplete": {
                    backgroundColor: "#353535",
                    color: "#d1d5db",
                    border: "1px solid #505050",
                    borderRadius: "6px",
                },
                ".cm-tooltip-autocomplete ul li[aria-selected]": {
                    backgroundColor: "#494236",
                    color: "#f59e0b",
                },
                ".cm-completionDetail": {
                    color: "#94a3b8",
                },
                ".cm-cursor": {
                    borderLeftColor: "#f59e0b",
                },
            }),

            // Emit changes back to Pinia
            EditorView.updateListener.of((update) => {
                if (!update.docChanged) {
                    return;
                }

                emit("update:modelValue", update.state.doc.toString());
            }),
            Prec.highest(
                keymap.of([
                    {
                        key: "Mod-Enter",

                        run: () => {
                          executeCurrentQuery();
                          return true;
                        },
                    },
                ]),
            ),
        ],
    });

    editorView = new EditorView({
        state: startState,
        parent: editorContainer.value,
    });
});

watch(
    () => props.modelValue,
    (newValue) => {
        if (!editorView) {
            return;
        }

        const currentValue = editorView.state.doc.toString();

        if (currentValue === newValue) {
            return;
        }

        editorView.dispatch({
            changes: {
                from: 0,
                to: editorView.state.doc.length,
                insert: newValue,
            },
        });
    },
);

watch(
    () => [props.dbDriver, props.tables] as const,
    ([driver]) => {
        if (!editorView) {
            return;
        }

        editorView.dispatch({
            effects: sqlDialect.reconfigure(
                languageExtension(driver),
            ),
        });
    },
    { deep: true },
);

onBeforeUnmount(() => {
    editorView?.destroy();
    editorView = null;
});
</script>
