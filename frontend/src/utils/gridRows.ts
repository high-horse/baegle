import type { DraftRow } from '@/stores/tableEditsStore';

export interface GridRow {
    row: unknown[];
    index: number;
    key: string;
    status: string;
    number: string;
}
export const gridValue = (value: unknown) => value != null && typeof value === 'object' ? JSON.stringify(value) : value;

// Fetched rows and staged drafts are replaced rather than mutated. Reuse every
// unaffected row, including its displayed object, across an inline edit.
export function createGridRowCache() {
    let previousBase: readonly GridRow[] | undefined;
    let previousColumns: readonly {Name: string}[] | undefined;
    let edits = new WeakMap<DraftRow, {base?: GridRow; index: number; result: GridRow}>();
    let visible: readonly number[] | undefined;
    let displayed = new WeakMap<GridRow, Record<string, unknown>>();
    function overlay(base: readonly GridRow[], drafts: ReadonlyMap<string, DraftRow>, columns: readonly {Name: string}[]) {
        if (base !== previousBase || columns !== previousColumns) {
            previousBase = base; previousColumns = columns;
            edits = new WeakMap();
        }
        const pageKeys = new Set<string>();
        function apply(draft: DraftRow, original: GridRow | undefined, index: number) {
            const cached = edits.get(draft);
            if (cached && cached.base === original && cached.index === index) return cached.result;
            const row = columns.map((column, i) => Object.hasOwn(draft.values, column.Name)
                ? gridValue(draft.values[column.Name]) : original ? original.row[i] : gridValue(draft.original?.[column.Name]));
            const result = {row, index, key: draft.key, status: draft.operation, number: original?.number ?? (draft.operation === 'insert' ? 'New' : 'Pending')};
            edits.set(draft, {base: original, index, result});
            return result;
        }
        const rows = base.map(row => {
            pageKeys.add(row.key);
            const draft = drafts.get(row.key);
            return draft ? apply(draft, row, row.index) : row;
        });
        for (const draft of drafts.values()) if (!pageKeys.has(draft.key)) rows.push(apply(draft, undefined, rows.length));
        return rows;
    }
    function map(rows: readonly GridRow[], columns: readonly number[]) {
        if (visible !== columns) {visible = columns; displayed = new WeakMap();}
        return rows.map(row => {
            const cached = displayed.get(row);
            if (cached) return cached;
            const value: Record<string, unknown> = {id: row.key, _editKey: row.key, _status: row.status, _rowNumber: row.number};
            columns.forEach((source, index) => {value[`column-${index}`] = row.row[source];});
            displayed.set(row, value);
            return value;
        });
    }
    return {overlay, map};
}
