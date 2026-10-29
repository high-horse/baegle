export type ClipboardCell = string | number | boolean | null;
export const GRID_CLIPBOARD_TYPE = 'application/x-baegle-grid+json';

export function copyResultSelection(event: ClipboardEvent, rows: Record<string, unknown>[], range: {
    fromRow: number; toRow: number; fromColumn: number; toColumn: number;
} | null) {
    if (event.target !== event.currentTarget || !event.clipboardData || !range) return;
    const cells = rows.slice(range.fromRow, range.toRow + 1).map(row =>
        Array.from({ length: range.toColumn - range.fromColumn + 1 }, (_, offset) => row[`column-${range.fromColumn + offset}`] ?? null));
    event.clipboardData.setData('text/plain', encodeGridText(cells));
    event.clipboardData.setData(GRID_CLIPBOARD_TYPE, JSON.stringify(cells));
    event.preventDefault();
}

export function encodeGridText(rows: unknown[][]): string {
    return rows.map(row => row.map(value => {
        const text = value == null ? '' : String(value);
        return /[\t\r\n"]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
    }).join('\t')).join('\n');
}

export function parseGridText(text: string): string[][] {
    const rows: string[][] = [];
    let row: string[] = [], field = '', quoted = false, closedQuote = false;
    for (let i = 0; i < text.length; i++) {
        const char = text[i];
        if (quoted) {
            if (char === '"') {
                if (text[i + 1] === '"') { field += '"'; i++; }
                else { quoted = false; closedQuote = true; }
            } else field += char;
        } else if (char === '\t' || char === '\r' || char === '\n') {
            row.push(field); field = ''; closedQuote = false;
            if (char !== '\t') {
                rows.push(row); row = [];
                if (char === '\r' && text[i + 1] === '\n') i++;
            }
        } else if (closedQuote) {
            throw new Error('Unexpected text after a quoted clipboard value.');
        } else if (char === '"' && field === '') quoted = true;
        else field += char;
    }
    if (quoted) throw new Error('The clipboard contains an unclosed quoted value.');
    if (field !== '' || row.length || !/[\r\n]$/.test(text)) { row.push(field); rows.push(row); }
    validateGridCells(rows);
    return rows;
}

export function validateGridCells(value: unknown): asserts value is ClipboardCell[][] {
    if (!Array.isArray(value) || !value.length || !Array.isArray(value[0]) || !value[0].length
        || !value.every(row => Array.isArray(row) && row.length === value[0].length && row.every(cell =>
            cell === null || typeof cell === 'string' || typeof cell === 'boolean' || (typeof cell === 'number' && Number.isFinite(cell))))) {
        throw new Error('Paste a rectangular range of cells with the same number of columns in every row.');
    }
}

type Column = { name: string; databaseType: string; nullable?: boolean; generated?: boolean; autoIncrement?: boolean; primaryKey?: boolean };
type Draft = { key: string; operation: 'insert' | 'update' | 'delete'; keys: Record<string, unknown>; values: Record<string, unknown>; original?: Record<string, unknown> };
export type PasteTarget = { key: string; keys: Record<string, unknown>; original: Record<string, unknown>; draft?: Draft };

export function prepareGridPaste(columns: Column[], keyNames: string[], targets: PasteTarget[], cells: ClipboardCell[][]): Draft[] {
    if (targets.length !== cells.length || cells.some(row => row.length !== columns.length)) throw new Error('The pasted range does not fit the target cells.');
    return targets.map((target, rowIndex) => {
        if (target.draft?.operation === 'delete') throw new Error('Undo the pending row deletion before pasting into it.');
        if (Object.values(target.keys).some(value => typeof value === 'number' && !Number.isSafeInteger(value))) {
            throw new Error('This row key cannot be represented precisely; reload before editing.');
        }
        const insert = target.draft?.operation === 'insert';
        const values = { ...target.draft?.values };
        columns.forEach((column, index) => {
            if (column.generated || (insert ? column.autoIncrement : column.primaryKey || keyNames.includes(column.name) || column.name.toLowerCase() === 'id')) {
                throw new Error(`${column.name} is read-only. Select editable columns before pasting.`);
            }
            let value: unknown = cells[rowIndex][index];
            if (value === null) {
                if (!column.nullable) throw new Error(`${column.name} does not allow NULL.`);
            } else {
                const text = String(value);
                if (/^(boolean|bool)$/i.test(column.databaseType)) {
                    if (!/^(true|false|0|1)$/i.test(text)) throw new Error(`${column.name}: enter true, false, 0, or 1.`);
                    value = /^(true|1)$/i.test(text);
                } else {
                    if (/(int|numeric|decimal|real|double|float)/i.test(column.databaseType)
                        && !/^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/.test(text)) throw new Error(`${column.name}: enter a number.`);
                    // Preserve decimal/bigint precision; the driver handles conversion.
                    value = text;
                }
            }
            const original = target.original[column.name];
            if (!insert && (value === null ? original == null : original != null && String(original) === String(value))) delete values[column.name];
            else values[column.name] = value;
        });
        return { key: target.key, operation: insert ? 'insert' : 'update', keys: target.keys, original: target.original, values };
    });
}
