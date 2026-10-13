import type { SyntaxNode } from "@lezer/common";
import { syntaxTree } from "@codemirror/language";
import { insertCompletionText, type Completion } from "@codemirror/autocomplete";
import type { EditorState } from "@codemirror/state";
import type { SQLNamespace } from "@codemirror/lang-sql";

type TableMetadata = { name: string; schema?: string; type: string };
type ColumnMetadata = { name: string; databaseType: string };
export type SQLTableReference = { path: string[]; alias?: string; from: number; to: number; keyword: string };

function quoteIdentifier(name: string, driver: string): string {
    const quote = driver === "mysql" ? "`" : '"';
    return quote + name.split(quote).join(quote + quote) + quote;
}
function identifier(text: string): string {
    const quote = text[0];
    if (quote === '"' || quote === "`") return text.slice(1, -1).split(quote + quote).join(quote);
    return text;
}

// Read only the current statement so aliases from another query never leak in.
export function getSQLTableReferences(state: EditorState, pos: number): SQLTableReference[] {
    const statement = syntaxTree(state).topNode.getChildren("Statement").find(node => pos >= node.from && pos <= node.to);
    if (!statement) return [];
    const nodes: SyntaxNode[] = [];
    for (let node = statement.firstChild; node; node = node.nextSibling) {
        if (!/Comment/.test(node.name)) nodes.push(node);
    }
    const refs: SQLTableReference[] = [];
    let expecting = false, inFrom = false, keyword = "";
    for (let index = 0; index < nodes.length; index++) {
        const node = nodes[index];
        const text = state.sliceDoc(node.from, node.to);
        if (node.name === "Keyword") {
            const word = text.toLowerCase();
            if (["from", "join", "update", "into"].includes(word)) {
                keyword = word;
                expecting = true;
                inFrom = word === "from" || word === "join";
            } else if (["where", "group", "having", "order", "union", "intersect", "except", "limit", "offset", "set", "values", "returning"].includes(word)) {
                expecting = inFrom = false;
            }
            continue;
        }
        if (text === "," && inFrom) { expecting = true; continue; }
        if (!expecting) continue;
        expecting = false;
        if (!["Identifier", "QuotedIdentifier", "CompositeIdentifier"].includes(node.name)) continue;
        const path: string[] = [];
        if (node.name === "CompositeIdentifier") {
            for (let part = node.firstChild; part; part = part.nextSibling) {
                if (part.name === "Identifier" || part.name === "QuotedIdentifier") path.push(identifier(state.sliceDoc(part.from, part.to)));
            }
        } else path.push(identifier(text));
        let next = nodes[index + 1];
        if (next && state.sliceDoc(next.from, next.to).toLowerCase() === "as") next = nodes[index + 2];
        const alias = next && ["Identifier", "QuotedIdentifier"].includes(next.name) ? identifier(state.sliceDoc(next.from, next.to)) : undefined;
        refs.push({ path, alias, from: node.from, to: node.to, keyword });
    }
    return refs;
}

export function matchingSQLTable<T extends TableMetadata>(tables: readonly T[], path: readonly string[]): T | undefined {
    const matches = tables.filter(table => table.name === path[path.length - 1] &&
        (path.length < 2 || table.schema === path[path.length - 2]));
    return matches.find(table => table.schema === "public") || (matches.length === 1 ? matches[0] : undefined);
}

export function sqlTableAtPosition<T extends TableMetadata>(state: EditorState, pos: number, tables: readonly T[]): T | undefined {
    const reference = getSQLTableReferences(state, pos).find(ref => pos >= ref.from && pos < ref.to);
    return reference ? matchingSQLTable(tables, reference.path) : undefined;
}

export function tableAlias(name: string, used: readonly string[] = []): string {
    const words = name.replace(/([a-z0-9])([A-Z])/g, "$1 $2").split(/[^a-zA-Z0-9]+/).filter(Boolean);
    let base = words.map(word => word[0]).join("").toLowerCase() || "t";
    if (!/^[a-z]/.test(base)) base = "t" + base;
    const aliases = new Set([...used.map(alias => alias.toLowerCase()),
        ..."all and as asc at by case desc do else end false for from group having if in inner into is join left like limit not null offset on or order outer right select set then to true union update user using when where with".split(" ")]);
    let alias = base, suffix = 2;
    while (aliases.has(alias)) alias = base + suffix++;
    return alias;
}

function tableCompletion(table: TableMetadata, label: string, insert: string,
    onSelected?: (table: TableMetadata) => void): Completion {
    return {
        label, type: "type", detail: table.schema,
        apply(view, _completion, from, to) {
            const references = getSQLTableReferences(view.state, from);
            const reference = references.find(ref => from >= ref.from && to <= ref.to);
            let text = insert;
            if (reference && !reference.alias && ["from", "join"].includes(reference.keyword)) {
                text += " AS " + tableAlias(table.name, references.flatMap(ref => ref.alias ? [ref.alias] : []));
            }
            view.dispatch(insertCompletionText(view.state, text, from, to));
            onSelected?.(table);
        },
    };
}

// Expose tables at the top level and inside their schema for qualified queries.
export function buildSQLCompletionSchema(tables: readonly TableMetadata[], driver: string,
    getColumns: (table: TableMetadata) => readonly ColumnMetadata[] = () => [],
    onSelected?: (table: TableMetadata) => void): SQLNamespace {
    const schema: Record<string, SQLNamespace> = Object.create(null);
    const scopes: Record<string, Record<string, SQLNamespace>> = Object.create(null);
    const sqlTables = tables.filter(table => table.type !== "COLLECTION");
    const counts = new Map<string, number>();
    for (const table of sqlTables) counts.set(table.name, (counts.get(table.name) || 0) + 1);
    const key = (name: string) => name.replace(/\./g, "\\.");
    for (const table of sqlTables) {
        const qualified = table.schema ? `${table.schema}.${table.name}` : table.name;
        const insert = [table.schema, table.name].filter((part): part is string => !!part)
            .map(part => quoteIdentifier(part, driver)).join(".");
        const label = counts.get(table.name)! > 1 ? qualified : table.name;
        const children: Completion[] = getColumns(table).map(column => ({ label: column.name, type: "property",
            detail: column.databaseType, apply: quoteIdentifier(column.name, driver) }));
        // Also register the unqualified name for manually typed table aliases.
        if (!schema[key(table.name)] || table.schema === "public") {
            schema[key(table.name)] = { self: tableCompletion(table, table.name, insert, onSelected), children };
        }
        schema[key(label)] = { self: tableCompletion(table, label, insert, onSelected), children };
        if (table.schema) {
            const scope = scopes[table.schema] ||= Object.create(null);
            scope[key(table.name)] = {
                self: tableCompletion(table, table.name, quoteIdentifier(table.name, driver), onSelected), children,
            };
        }
    }
    for (const [name, children] of Object.entries(scopes)) {
        const existing = schema[key(name)] as { self?: Completion } | undefined;
        schema[key(name)] = { self: existing?.self || { label: name, type: "namespace", apply: quoteIdentifier(name, driver) }, children };
    }
    return schema;
}
