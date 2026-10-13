import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorView } from "@codemirror/view";
import { tags } from "@lezer/highlight";

// Shared DBeaver-inspired SQL colors for query editing and DDL previews.
export const sqlTheme = [
    EditorView.theme({
        "&": { backgroundColor: "#282828", color: "#d1d5db" },
        ".cm-content": { caretColor: "#d1d5db" },
        ".cm-cursor, .cm-dropCursor": { borderLeftColor: "#d1d5db" },
        "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection": { backgroundColor: "#3d5066" },
        ".cm-activeLine": { backgroundColor: "#ffffff08" },
        ".cm-gutters": { backgroundColor: "#2d2d2d", color: "#909090", border: "none" },
        ".cm-activeLineGutter": { backgroundColor: "#353535" },
        ".cm-panels, .cm-tooltip": { backgroundColor: "#353535", color: "#d1d5db", borderColor: "#505050" },
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
