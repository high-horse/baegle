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
            backgroundColor: "#316ac5 !important",
        },
        "&:not(.cm-focused) .cm-selectionLayer .cm-selectionBackground": {
            backgroundColor: "#3d5570",
        },
        "&.cm-focused .cm-consoleSelectedText": {
            backgroundColor: "#316ac5 !important",
        },
        "&.cm-focused .cm-consoleSelectedText, &.cm-focused .cm-consoleSelectedText *": {
            color: "#ffffff !important",
        },
    }),
];
