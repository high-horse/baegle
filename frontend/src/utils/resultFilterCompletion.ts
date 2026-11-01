import { syntaxTree } from "@codemirror/language";
import type { CompletionContext } from "@codemirror/autocomplete";

export function completeResultExpression(context: CompletionContext, columns: readonly { Name: string; Type?: string }[], mode: "where" | "order" = "where") {
    const node = syntaxTree(context.state).resolveInner(context.pos, -1);
    if (/String|Comment/.test(node.name)) return null;
    const quoted = node.name === "QuotedIdentifier"
        ? { from: node.from, text: context.state.sliceDoc(node.from, context.pos) } : null;
    const word = quoted || context.matchBefore(/[\p{L}\p{N}_$.]+/u);
    const before = context.state.sliceDoc(0, context.pos);
    const expectingColumn = /(?:^|\b(?:AND|OR|NOT)\s+|\(\s*|,\s*)$/i.test(before);
    if (!word && !context.explicit && !expectingColumn) return null;
    const quote = quoted?.text[0] || '"';
    const closing = quoted && context.state.sliceDoc(context.pos, context.pos + 1) === quote;
    return {
        from: word?.from ?? context.pos,
        to: closing ? context.pos + 1 : context.pos,
        options: [
            ...columns.map(column => ({
                label: quoted ? quote + column.Name : column.Name,
                displayLabel: column.Name,
                type: "property",
                detail: column.Type,
                apply: quote + column.Name.split(quote).join(quote + quote) + quote,
            })),
            ...(!quoted ? (mode === "order" ? "ASC DESC" : "AND OR NOT LIKE IN IS NULL TRUE FALSE")
                .split(" ").map(label => ({ label, type: "keyword" })) : []),
        ],
    };
}

