export interface CopySelection {
    columns: string[];
    rows: unknown[][];
    keys?: Record<string, unknown>[];
}
export type CopyFormat = 'tsv' | 'csv' | 'custom' | 'json' | 'insert' | 'update';
export interface CopyTable { name: string; schema: string; database: string }
export function duplicateRowValues(columns: {name: string; generated?: boolean; autoIncrement?: boolean; primaryKey?: boolean}[], keys: string[], source: Record<string, unknown>) {
    const editable = columns.filter(column => !column.generated && !column.autoIncrement && !column.primaryKey
        && !keys.includes(column.name) && Object.hasOwn(source, column.name) && source[column.name] !== undefined);
    for (const column of editable) {
        const value = source[column.name];
        if (typeof value === 'number' && (!Number.isFinite(value) || (Number.isInteger(value) && !Number.isSafeInteger(value)))) {
            throw new Error(`${column.name} cannot be duplicated precisely; refetch it as text first.`);
        }
    }
    return Object.fromEntries(editable.map(column => [column.name, structuredClone(source[column.name])]));
}
function literal(value: unknown, driver: string): string {
    if (value === undefined) return 'DEFAULT';
    if (value === null) return 'NULL';
    if (typeof value === 'number') {
        if (!Number.isFinite(value) || (Number.isInteger(value) && !Number.isSafeInteger(value))) throw new Error('A number cannot be represented precisely.');
        return String(value);
    }
    if (typeof value === 'boolean') return ['oracle', 'sqlserver'].includes(driver) ? (value ? '1' : '0') : value ? 'TRUE' : 'FALSE';
    const text = typeof value === 'object' ? JSON.stringify(value) : String(value);
    if (text.includes('\0')) throw new Error('SQL copy does not support text containing a zero byte.');
    if (driver === 'mysql') return `CONVERT(X'${Array.from(new TextEncoder().encode(text), byte => byte.toString(16).padStart(2, '0')).join('')}' USING utf8mb4)`;
    const escaped = text.replaceAll("'", "''");
    return driver === 'pgx' ? `E'${escaped.replaceAll('\\', '\\\\')}'` : `${driver === 'sqlserver' ? 'N' : ''}'${escaped}'`;
}
export interface AdvancedCopyOptions {format: CopyFormat; headers: boolean; delimiter?: string; nullText: string; driver: string; table?: CopyTable}
export function advancedCopy(selection: CopySelection, options: AdvancedCopyOptions) {
    const {columns, rows} = selection;
    if (!columns.length || !rows.length) return '';
    if (options.format === 'json') {
        if (new Set(columns).size !== columns.length) throw new Error('JSON objects need unique column names.');
        return JSON.stringify(rows.map(row => Object.fromEntries(columns.map((column, i) => [column, row[i] ?? null]))), null, 2);
    }
    if (options.format === 'insert' || options.format === 'update') {
        if (!options.table) throw new Error('Open a table to copy SQL statements.');
        if (new Set(columns).size !== columns.length) throw new Error('SQL copy needs unique column names.');
        const driver = options.driver;
        const quote = (name: string) => driver === 'mysql' ? '`' + name.replaceAll('`', '``') + '`'
            : driver === 'sqlserver' ? '[' + name.replaceAll(']', ']]') + ']' : '"' + name.replaceAll('"', '""') + '"';
        const table = [driver === 'mysql' ? options.table.database : options.table.schema, options.table.name].filter(Boolean).map(quote).join('.');
        return rows.map((row, index) => {
            if (options.format === 'insert') return `INSERT INTO ${table} (${columns.map(quote).join(', ')}) VALUES (${row.map(value => literal(value, driver)).join(', ')});`;
            const keys = Object.entries(selection.keys?.[index] ?? {});
            if (!keys.length || keys.some(([, value]) => value == null)) throw new Error('UPDATE copy requires primary keys for every selected row.');
            const changes = columns.map((column, i) => `${quote(column)} = ${literal(row[i], driver)}`);
            return `UPDATE ${table} SET ${changes.join(', ')} WHERE ${keys.map(([key, value]) => `${quote(key)} = ${literal(value, driver)}`).join(' AND ')};`;
        }).join('\n');
    }
    const delimiter = options.format === 'csv' ? ',' : options.format === 'tsv' ? '\t' : options.delimiter ?? '';
    if (!delimiter || /[\r\n"]/.test(delimiter)) throw new Error('Enter a delimiter without quotes or line breaks.');
    const encode = (value: unknown) => {
        const text = value == null ? options.nullText : typeof value === 'object' ? JSON.stringify(value) : String(value);
        return text.includes(delimiter) || /[\r\n"]/.test(text) ? '"' + text.replaceAll('"', '""') + '"' : text;
    };
    return [...(options.headers ? [columns] : []), ...rows].map(row => row.map(encode).join(delimiter)).join('\n');
}
export function advancedCopyPreview(selection: CopySelection, options: AdvancedCopyOptions) {
    const rowLimit = 50, characterLimit = 20000;
    const text = advancedCopy({...selection, rows: selection.rows.slice(0, rowLimit), keys: selection.keys?.slice(0, rowLimit)}, options);
    return {text: text.slice(0, characterLimit), truncated: selection.rows.length > rowLimit || text.length > characterLimit};
}
// Accumulate decimal strings exactly, including BIGINT values returned as text.
function decimal(value: unknown): {number: bigint; scale: number} | undefined {
    if (typeof value !== 'string' && typeof value !== 'number') return;
    if (typeof value === 'number' && (!Number.isFinite(value) || (Number.isInteger(value) && !Number.isSafeInteger(value)))) return;
    const match = /^([+-]?)(\d*)(?:\.(\d*))?(?:e([+-]?\d+))?$/i.exec(String(value).trim());
    if (!match || (!match[2] && !match[3])) return;
    const exponent = Number(match[4] ?? 0);
    if (Math.abs(exponent) > 1000 || match[2].length + (match[3]?.length ?? 0) > 1000) return;
    const scale = (match[3]?.length ?? 0) - exponent;
    const number = BigInt((match[1] === '-' ? '-' : '') + match[2] + (match[3] ?? ''));
    return scale < 0 ? {number: number * 10n ** BigInt(-scale), scale: 0} : {number, scale};
}
function decimalText(number: bigint, scale: number) {
    const negative = number < 0n;
    const digits = (negative ? -number : number).toString().padStart(scale + 1, '0');
    const text = scale ? (digits.slice(0, -scale) + '.' + digits.slice(-scale)).replace(/\.?0+$/, '') : digits;
    return (negative && number !== 0n ? '-' : '') + text;
}
export function selectionStatistics(rows: Iterable<readonly unknown[]>, numericColumns: boolean[]) {
    let cells = 0, filled = 0, count = 0, excluded = 0, scale = 0;
    let sum = 0n, min: bigint | undefined, max: bigint | undefined;
    const powers = new Map<number, bigint>([[0, 1n]]);
    const power = (exponent: number) => {
        let value = powers.get(exponent);
        if (value === undefined) {value = 10n ** BigInt(exponent); powers.set(exponent, value);}
        return value;
    };
    for (const row of rows) for (let i = 0; i < row.length; i++) {
        const value = row[i]; cells++;
        if (value == null) continue;
        filled++;
        if (!numericColumns[i]) continue;
        const parsed = decimal(value);
        if (!parsed) {excluded++; continue;}
        if (parsed.scale > scale) {
            const factor = power(parsed.scale - scale);
            sum *= factor;
            if (min !== undefined) min *= factor;
            if (max !== undefined) max *= factor;
            scale = parsed.scale;
        }
        const number = parsed.number * power(scale - parsed.scale);
        sum += number; count++;
        min = min === undefined || number < min ? number : min;
        max = max === undefined || number > max ? number : max;
    }
    const expandedSum = sum * 1000000000000n;
    const divisor = count ? BigInt(count) : 1n;
    const average = expandedSum / divisor;
    return {cells, filled, count, excluded, sum: decimalText(sum, scale),
        min: min === undefined ? '' : decimalText(min, scale), max: max === undefined ? '' : decimalText(max, scale),
        average: count ? (expandedSum % divisor ? '≈' : '') + decimalText(average, scale + 12) : ''};
}
