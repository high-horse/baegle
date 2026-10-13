export type IndexedResultRow = { row: unknown[]; index: number };
export type ResultOrder = { column: number; direction: 'asc' | 'desc' };

export function parseResultOrder(expression: string, columns: string[]): ResultOrder[] {
    const orders: ResultOrder[] = [];
    let remaining = expression.trim().replace(/^ORDER\s+BY\s+/i, '');
    if (!remaining) return orders;
    while (remaining) {
        const match = /^("(?:""|[^"])+"|`(?:``|[^`])+`|[\p{L}_$][\p{L}\p{N}_$.]*|\d+)(?:\s+(ASC|DESC))?\s*(,|$)/iu.exec(remaining);
        if (!match) throw new Error('Use column ASC or column DESC, separated by commas');
        const token = match[1];
        const quoted = token.startsWith('"') || token.startsWith('`');
        const name = quoted ? token.slice(1, -1).replaceAll(token[0].repeat(2), token[0]) : token;
        let index: number;
        if (!quoted && /^\d+$/.test(name)) {
            index = Number(name) - 1;
        } else {
            index = columns.indexOf(name);
            if (index < 0) {
                const candidates = columns.map((column, i) => column.toLowerCase() === name.toLowerCase() ? i : -1).filter(i => i >= 0);
                if (candidates.length > 1) throw new Error(`Ambiguous column: ${name}. Use its column number`);
                index = candidates[0] ?? -1;
            } else if (columns.lastIndexOf(name) !== index) {
                throw new Error(`Ambiguous column: ${name}. Use its column number`);
            }
        }
        if (index < 0 || index >= columns.length) throw new Error(`Unknown column: ${name}`);
        orders.push({ column: index + 1, direction: match[2]?.toLowerCase() === 'desc' ? 'desc' : 'asc' });
        remaining = remaining.slice(match[0].length).trim();
        if (match[3] && !remaining) throw new Error('Expected a column after the comma');
    }
    return orders;
}

function sortableValue(value: unknown): unknown {
    if (typeof value !== 'string') return value;
    try {
        const parsed = JSON.parse(value);
        if (parsed && typeof parsed === 'object') {
            if ('$numberLong' in parsed) return BigInt(parsed.$numberLong);
            if ('$numberInt' in parsed) return Number(parsed.$numberInt);
            if ('$numberDouble' in parsed) return Number(parsed.$numberDouble);
            if ('$oid' in parsed) return parsed.$oid;
        }
    } catch { /* Ordinary text keeps its original value. */ }
    return value;
}

// Preserve original row indexes for the grid's row keys and row numbers.
export function sortResultRows<T extends IndexedResultRow>(rows: T[], column: number, direction?: 'asc' | 'desc'): T[] {
    if (column < 1 || !direction) return rows;
    return orderResultRows(rows, [{ column, direction }]);
}

export function orderResultRows<T extends IndexedResultRow>(rows: T[], orders: ResultOrder[]): T[] {
    if (!orders.length) return rows;
    return [...rows].sort((a, b) => {
      for (const { column, direction } of orders) {
        const multiplier = direction === 'desc' ? -1 : 1;
        const left = sortableValue(a.row[column - 1]);
        const right = sortableValue(b.row[column - 1]);
        let comparison = 0;
        if (left == null || right == null) comparison = left == null ? (right == null ? 0 : -1) : 1;
        else if ((typeof left === 'number' || typeof left === 'bigint') && (typeof right === 'number' || typeof right === 'bigint')) {
            comparison = left < right ? -1 : left > right ? 1 : 0;
        } else {
            const text = (value: unknown) => typeof value === 'object' ? JSON.stringify(value) : String(value);
            comparison = text(left).localeCompare(text(right));
        }
        if (comparison) return comparison * multiplier;
      }
      return a.index - b.index;
    });
}
