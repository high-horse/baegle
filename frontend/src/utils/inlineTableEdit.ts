import { prepareGridPaste, type PasteTarget } from './resultClipboard';

type Column = Parameters<typeof prepareGridPaste>[0][number] & { defaultValue?: unknown };
export function inlineColumnEditable(column: Column, keys: string[], insert: boolean): boolean {
    return !column.generated && !(insert ? column.autoIncrement : column.primaryKey || keys.includes(column.name) || column.name.toLowerCase() === 'id');
}
export function inlineValueText(value: unknown): string {
    return value == null ? '' : typeof value === 'object' ? JSON.stringify(value) : String(value);
}
export function stageInlineValue(column: Column, keys: string[], target: PasteTarget, mode: 'value' | 'null' | 'default', text: string) {
    if (!inlineColumnEditable(column, keys, target.draft?.operation === 'insert')) throw new Error(`${column.name} is read-only.`);
    if (mode === 'default') {
        if (target.draft?.operation !== 'insert') throw new Error('Default is available for new rows only.');
        const values = { ...target.draft.values };
        delete values[column.name];
        return { ...target.draft, values };
    }
    return prepareGridPaste([column], keys, [target], [[mode === 'null' ? null : text]])[0]!;
}
