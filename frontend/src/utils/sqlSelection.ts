import { Decoration, EditorView, drawSelection } from "@codemirror/view";

// Share selection rendering so query editors and read-only DDL stay consistent.
export const sqlSelection = [
    drawSelection({ drawRangeCursor: true, cursorBlinkRate: 1000 }),
    EditorView.decorations.compute(["selection"], state => Decoration.set(
        state.selection.ranges.filter(range => !range.empty).map(range =>
            Decoration.mark({ class: "cm-consoleSelectedText" }).range(range.from, range.to),
        ), true,
    )),
    EditorView.theme({
        "&.cm-focused .cm-selectionLayer .cm-selectionBackground": {
            backgroundColor: "var(--selection-bg) !important",
        },
        "&:not(.cm-focused) .cm-selectionLayer .cm-selectionBackground": {
            backgroundColor: "var(--accent-surface)",
        },
        "&.cm-focused .cm-consoleSelectedText": {
            backgroundColor: "var(--selection-bg) !important",
        },
        "&.cm-focused .cm-consoleSelectedText, &.cm-focused .cm-consoleSelectedText *": {
            color: "#ffffff !important",
        },
    }),
];
