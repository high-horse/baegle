<template>
    <div
        ref="editorContainer"
        class="h-full w-full min-h-0 min-w-0 max-w-full overflow-hidden [&_.cm-panels-top]:border-0!"
    />
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, getCurrentInstance } from "vue";
import { Prec, Compartment } from "@codemirror/state";
import { linter, type Diagnostic } from "@codemirror/lint";
import { ensureSyntaxTree, syntaxTree } from "@codemirror/language";
import { format } from "sql-formatter";
import { Notify } from "quasar";
import { EditorView, keymap, tooltips } from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { search, searchKeymap } from "@codemirror/search";
import { EditorSelection } from "@codemirror/state";
import { json } from "@codemirror/lang-json";
import { sql, keywordCompletionSource, schemaCompletionSource } from "@codemirror/lang-sql";
import { editorDialect as getDialect, formatterDialect, isJSONDriver, type DatabaseDriver } from '@/utils/databaseDialect';
import { enterpriseStatementAt } from '@/utils/enterpriseStatement';
import { sqlSelection } from "@/utils/sqlSelection";
import { sqlTheme } from "@/utils/sqlTheme";
import { createSQLSearchPanel } from "@/utils/sqlSearchPanel";
import { buildSQLCompletionSchema, completeSQLColumns, getSQLTableReferences, matchingSQLTable, sqlTableAtPosition } from "@/utils/sqlCompletion";
import type { InspectTableInfo } from "@bindings/db-lens/internal/engine/entities";
import { useConnectionStore } from "@/stores/connectionStore";
import { useResultSettingsStore } from "@/stores/resultSettingsStore";
import { acceptCompletion, autocompletion, type CompletionContext } from "@codemirror/autocomplete";

const props = defineProps<{
    modelValue: string;
    dbDriver: DatabaseDriver;
    tables?: InspectTableInfo[];
}>();

const emit = defineEmits<{
    "update:modelValue": [value: string];
    execute: [sql: string];
    'execute-script': [];
    openTable: [table: InspectTableInfo];
}>();

const appContext = getCurrentInstance()?.appContext ?? null;
const connectionStore = useConnectionStore();
const resultSettings = useResultSettingsStore();

const editorContainer = ref<HTMLElement | null>(null);

let editorView: EditorView | null = null;
const sqlDialect = new Compartment();
const editorFont = new Compartment();

function fontExtension() {
    return EditorView.theme({
        "&": { fontSize: `${resultSettings.settings.consoleFontSize}px` },
    });
}

watch(() => resultSettings.settings.consoleFontSize, () => {
    editorView?.dispatch({ effects: editorFont.reconfigure(fontExtension()) });
});


async function completeSQLSchema(context: CompletionContext) {
    if (isJSONDriver(props.dbDriver)) return null;
    const tables = props.tables ?? [];
    const requested = new Set(getSQLTableReferences(context.state, context.pos)
        .map(reference => matchingSQLTable(tables, reference.path)).filter((table): table is InspectTableInfo => !!table));
    await Promise.all([...requested].map(table => connectionStore.loadTableColumns(table).catch(() => [])));
    if (context.aborted || tables !== props.tables) return null;
    const find = (table: { name: string; schema?: string }) => tables.find(item => item.name === table.name && item.schema === table.schema);
    const getColumns = (table: { name: string; schema?: string }) => {
        const cached = find(table);
        return cached ? connectionStore.getCachedTableColumns(cached) : [];
    };
    const columns = completeSQLColumns(context, tables, props.dbDriver, getColumns);
    if (columns) return columns;
    return schemaCompletionSource({ dialect: getDialect(props.dbDriver),
        schema: buildSQLCompletionSchema(tables, props.dbDriver,
            getColumns,
            table => { const selected = find(table); if (selected) void connectionStore.loadTableColumns(selected).catch(() => {}); }),
    })(context);
}

function languageExtension(driver: string) {
    return [
        isJSONDriver(driver) ? json() : sql({ dialect: getDialect(driver), upperCaseKeywords: true }),
        autocompletion({
            activateOnTyping: true,
            defaultKeymap: true,
            closeOnBlur: true,
            override: isJSONDriver(driver) ? [] : [completeSQLSchema, keywordCompletionSource(getDialect(driver), true)],
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

    if (isJSONDriver(props.dbDriver)) return view.state.doc.toString().trim() || null;
    if (['oracle', 'sqlserver', 'db2'].includes(props.dbDriver)) {
        const range = enterpriseStatementAt(view.state.doc.toString(), head, props.dbDriver);
        if (!range) return null;
        view.dispatch({ selection: EditorSelection.range(range.from, range.to) });
        return view.state.doc.sliceString(range.from, range.to).trim() || null;
    }

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

function formatCurrentQuery(view: EditorView): boolean {
    if (isJSONDriver(props.dbDriver) || view.state.readOnly) return false;
    const selection = view.state.selection.main;
    const tree = ensureSyntaxTree(view.state, selection.head, 100);
    const statement = tree?.topNode.getChildren("Statement")
        .find(node => selection.head >= node.from && selection.head <= node.to);
    const range = selection.empty ? (['oracle', 'sqlserver', 'db2'].includes(props.dbDriver)
        ? enterpriseStatementAt(view.state.doc.toString(), selection.head, props.dbDriver) : statement) : selection;
    if (!range) return true;
    const original = view.state.doc.sliceString(range.from, range.to);
    if (!original.trim()) return true;
    try {
        const formatted = format(original, {
            language: formatterDialect(props.dbDriver),
            tabWidth: 4,
            keywordCase: "upper",
        });
        if (formatted === original) return true;
        view.dispatch({
            changes: { from: range.from, to: range.to, insert: formatted },
            selection: selection.empty
                ? { anchor: range.from + Math.min(selection.head - range.from, formatted.length) }
                : EditorSelection.range(range.from, range.from + formatted.length),
            scrollIntoView: true,
            userEvent: "input.format",
        });
    } catch (error) {
        Notify.create({ color: "negative", message: `Unable to format SQL: ${error instanceof Error ? error.message : String(error)}` });
    }
    return true;
}

function executeCurrentQuery() {
    if (!editorView) {
        return;
    }

    const sql = getCurrentQuery(editorView);

    if (!sql) {
        return;
    }

    emit("execute", sql);
}
defineExpose({
    execute: executeCurrentQuery,
    focusLine(line: number) {
        if (!editorView) return;
        const position = editorView.state.doc.line(Math.min(editorView.state.doc.lines, Math.max(1, line))).from;
        editorView.dispatch({ selection: { anchor: position }, effects: EditorView.scrollIntoView(position, { y: 'center' }) });
        editorView.focus();
    },
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
            sqlTheme,
            editorFont.of(fontExtension()),
            // Splitter panels clip their contents. Mount suggestions outside
            // those panels so they can extend over the result grid.
            tooltips({ parent: document.body }),
            EditorView.domEventHandlers({
                mousedown(event, view) {
                    if (isJSONDriver(props.dbDriver) || event.button !== 0 || !(event.ctrlKey || event.metaKey) || event.altKey || event.shiftKey) return false;
                    const pos = view.posAtCoords({ x: event.clientX, y: event.clientY });
                    const table = pos === null ? undefined : sqlTableAtPosition(view.state, pos, props.tables ?? []);
                    if (!table) return false;
                    event.preventDefault();
                    emit("openTable", table);
                    return true;
                },
                mousemove(event, view) {
                    const pos = !isJSONDriver(props.dbDriver) && (event.ctrlKey || event.metaKey)
                        ? view.posAtCoords({ x: event.clientX, y: event.clientY }) : null;
                    const table = pos === null ? undefined : sqlTableAtPosition(view.state, pos, props.tables ?? []);
                    view.contentDOM.style.cursor = table ? "pointer" : "";
                    view.contentDOM.title = table ? `Open ${[table.schema, table.name].filter(Boolean).join(".")} in a new tab` : "";
                },
                keyup(_event, view) { view.contentDOM.style.cursor = ""; view.contentDOM.title = ""; },
                mouseleave(_event, view) { view.contentDOM.style.cursor = ""; view.contentDOM.title = ""; },
            }),
            // Basic editing
            //
            EditorState.allowMultipleSelections.of(true),
            search({ top: true, createPanel: view => createSQLSearchPanel(view, appContext) }),
            history(),
            sqlSelection,
            keymap.of([{ key: "Ctrl-Alt-f", run: formatCurrentQuery }, { key: "Tab", run: acceptCompletion }, ...historyKeymap, ...searchKeymap, ...defaultKeymap, indentWithTab]),
            EditorView.lineWrapping,
            // Custom styling
            EditorView.theme({
                // The body-mounted tooltip host also receives theme classes.
                // Keep editor sizing off that host to avoid page overflow.
                "&.cm-editor": {
                    height: "100%",
                    width: "100%",
                    minWidth: "0",
                    minHeight: "0",
                    maxWidth: "100%",
            
                    backgroundColor: "var(--surface-data)",
                    color: "var(--text-primary)",
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
            
                    lineHeight: "1.7",
                    caretColor: "var(--accent-color)",
            
                    minWidth: "0",
                },
            
                ".cm-line": {
                    minWidth: "0",
                },
            
                ".cm-gutters": {
                    backgroundColor: "var(--surface-panel)",
                    color: "var(--text-muted)",
                    border: "none",
                },
            
                ".cm-activeLineGutter": {
                    backgroundColor: "var(--surface-hover)",
                    color: "var(--accent-color)",
                },
            
                ".cm-activeLine": {
                    backgroundColor: "var(--accent-tint)",
                },
            
                ".cm-tooltip-autocomplete": {
                    zIndex: "1000",
                    backgroundColor: "var(--surface-panel)",
                    color: "var(--text-primary)",
                    border: "1px solid var(--border-default)",
                    borderRadius: "6px",
                },
                ".cm-tooltip-autocomplete ul li[aria-selected]": {
                    backgroundColor: "var(--accent-surface)",
                    color: "var(--accent-color)",
                },
                ".cm-completionDetail": {
                    color: "#94a3b8",
                },
                ".cm-cursor": {
                    borderLeftColor: "#ffffff !important",
                    borderLeftWidth: "2px",
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
                    { key: 'Alt-x', run: () => { emit('execute-script'); return true; } },
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
