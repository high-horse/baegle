import type { useQueryTabsStore } from '@/stores/queryTabsStore';
import type { useTableEditsStore } from '@/stores/tableEditsStore';

// Use the existing actions: a live Pinia store may predate newly added actions
// when Vue components are replaced during development.
export async function refreshTable(
    queries: Pick<ReturnType<typeof useQueryTabsStore>, 'tabs' | 'execute'>,
    edits: Pick<ReturnType<typeof useTableEditsStore>, 'discard' | 'isSaving'>,
    id: string,
) {
    const tab = queries.tabs.find(tab => tab.id === id);
    const sql = tab?.executedSql ?? tab?.sql ?? '';
    if (!tab || tab.loading || tab.runningQuery || tab.closing || edits.isSaving(id) || !sql.trim()) return;
    edits.discard(id);
    await queries.execute(id, sql);
}
