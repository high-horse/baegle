import { Dialog } from 'quasar';
import type { useQueryTabsStore } from '@/stores/queryTabsStore';
import type { useTableEditsStore } from '@/stores/tableEditsStore';

// Use the existing actions: a live Pinia store may predate newly added actions
// when Vue components are replaced during development.
export async function refreshTable(
    queries: Pick<ReturnType<typeof useQueryTabsStore>, 'tabs' | 'execute'>,
    edits: Pick<ReturnType<typeof useTableEditsStore>, 'discard' | 'isSaving' | 'hasPending' | 'states'>,
    id: string,
) {
    const tab = queries.tabs.find(tab => tab.id === id);
    const sql = tab?.executedSql ?? tab?.sql ?? '';
    if (!tab || tab.loading || tab.runningQuery || tab.closing || edits.isSaving(id) || !sql.trim()) return;
    const state = edits.states[id];
    if (state?.confirmingRefresh) return;
    if (edits.hasPending(id)) {
        if (state) state.confirmingRefresh = true;
        let confirmed = false;
        try {
            confirmed = await new Promise<boolean>(resolve => {
                Dialog.create({
                    dark: true,
                    class: 'table-refresh-confirmation',
                    title: 'Discard unsaved changes?',
                    message: 'Refreshing will discard your unsaved changes and reload the table from the database.',
                    cancel: { label: 'Keep editing', flat: true, noCaps: true, color: 'grey-5' },
                    ok: { label: 'Discard and refresh', color: 'negative', unelevated: true, noCaps: true },
                    focus: 'cancel',
                }).onOk(() => resolve(true)).onDismiss(() => resolve(false));
            });
            if (!confirmed || !queries.tabs.includes(tab) || tab.loading || tab.runningQuery || tab.closing || edits.isSaving(id)) return;
            edits.discard(id);
        } finally {
            if (state) state.confirmingRefresh = false;
        }
    } else {
        edits.discard(id);
    }
    await queries.execute(id, sql);
}
