import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorView } from "@codemirror/view";
import { tags } from "@lezer/highlight";

// Shared DBeaver-inspired SQL colors for query editing and DDL previews.
export const sqlTheme = [
    EditorView.theme({
        "&": { backgroundColor: "var(--surface-base)", color: "var(--text-primary)" },
        ".cm-content": { caretColor: "var(--text-primary)" },
        ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--text-primary)" },
        "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection": { backgroundColor: "var(--selection-bg)" },
        ".cm-activeLine": { backgroundColor: "#ffffff08" },
        ".cm-gutters": { backgroundColor: "var(--surface-data)", color: "var(--text-muted)", border: "none" },
        ".cm-activeLineGutter": { backgroundColor: "var(--surface-panel)" },
        ".cm-panels, .cm-tooltip": { backgroundColor: "var(--surface-panel)", color: "var(--text-primary)", borderColor: "var(--border-default)" },
        ".cm-searchMatch": { backgroundColor: "#66552e", outline: "1px solid #a68a48" },
        ".cm-searchMatch.cm-searchMatch-selected": { backgroundColor: "#806d3f" },
        ".cm-matchingBracket": { backgroundColor: "#454545", outline: "1px solid #808080" },
    }, { dark: true }),
    syntaxHighlighting(HighlightStyle.define([
        { tag: tags.keyword, color: "#c5a5de", fontWeight: "600" },
        { tag: [tags.string, tags.special(tags.string)], color: "#a5c995" },
        { tag: [tags.number, tags.bool, tags.null], color: "#91bce0" },
        { tag: tags.typeName, color: "#d9bf8c" },
        { tag: [tags.name, tags.variableName, tags.propertyName], color: "#d8d8d8" },
        { tag: tags.function(tags.variableName), color: "#8fc9c3" },
        { tag: [tags.operator, tags.punctuation], color: "#b8bcc4" },
        { tag: tags.comment, color: "#929b91", fontStyle: "italic" },
        { tag: tags.invalid, color: "#ef9999" },
    ])),
];
