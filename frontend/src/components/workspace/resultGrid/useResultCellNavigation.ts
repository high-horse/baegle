import { computed, nextTick, ref, watch, type ComputedRef, type Ref } from "vue";
import type { QTable } from "quasar";

export function useResultCellNavigation(
    table: Ref<QTable | null>,
    rows: ComputedRef<Record<string, unknown>[]>,
    columns: ComputedRef<string[]>,
    selectedRowKey: Ref<string>,
) {
    const gridId = `result-grid-${crypto.randomUUID()}`;
    const activeCell = ref<{ rowKey: string; column: number } | null>(null);
    const activeRow = computed(() => rows.value.findIndex(row => String(row.id) === activeCell.value?.rowKey));
    const cellId = (row: number, column: number) => `${gridId}-${row}-${column}`;
    const activeCellId = computed(() => activeCell.value && activeRow.value >= 0
        ? cellId(activeRow.value, activeCell.value.column) : undefined);
    let awaitingVirtualRow = false;

    async function revealActiveCell(allowVirtualScroll = true) {
        await nextTick();
        const root = table.value?.$el as HTMLElement | undefined;
        if (!root || !activeCellId.value) return;
        const cell = root.querySelector<HTMLElement>(`#${activeCellId.value}`);
        if (!cell) {
            if (allowVirtualScroll && activeRow.value >= 0) {
                awaitingVirtualRow = true;
                table.value?.scrollTo(activeRow.value, "center");
            }
            return;
        }
        awaitingVirtualRow = false;
        const scroller = root.querySelector<HTMLElement>(".q-table__middle");
        if (!scroller) return;
        const viewport = scroller.getBoundingClientRect();
        const rect = cell.getBoundingClientRect();
        const headerHeight = root.querySelector("thead")?.getBoundingClientRect().height ?? 0;
        const rowHeaderWidth = root.querySelector("thead th")?.getBoundingClientRect().width ?? 0;
        const top = viewport.top + headerHeight;
        const left = viewport.left + rowHeaderWidth;
        const bottom = viewport.top + scroller.clientHeight;
        const right = viewport.left + scroller.clientWidth;
        if (rect.top < top) scroller.scrollTop -= top - rect.top;
        else if (rect.bottom > bottom) scroller.scrollTop += rect.bottom - bottom;
        if (rect.left < left) scroller.scrollLeft -= left - rect.left;
        else if (rect.right > right) scroller.scrollLeft += rect.right - right;
    }

    function selectCell(rowKey: string, columnName: string, focus = true) {
        if (!columnName.startsWith("column-")) return;
        const column = Number(columnName.slice(7));
        activeCell.value = { rowKey, column };
        selectedRowKey.value = rowKey;
        // Keep focus on a stable element when virtual rows are recycled.
        if (focus) (table.value?.$el as HTMLElement | undefined)?.focus({ preventScroll: true });
    }

    function handleGridKeydown(event: KeyboardEvent) {
        // Leave header buttons, text fields, and modified shortcuts alone.
        if (event.target !== event.currentTarget || event.isComposing
            || event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) return;
        const moves: Record<string, [number, number]> = {
            ArrowLeft: [0, -1], ArrowRight: [0, 1], ArrowUp: [-1, 0], ArrowDown: [1, 0],
        };
        const move = moves[event.key];
        if (!move || !rows.value.length || !columns.value.length) return;
        event.preventDefault();
        event.stopPropagation();
        const row = activeRow.value < 0 ? 0 : Math.max(0, Math.min(rows.value.length - 1, activeRow.value + move[0]));
        const column = activeCell.value === null ? 0
            : Math.max(0, Math.min(columns.value.length - 1, activeCell.value.column + move[1]));
        activeCell.value = { rowKey: String(rows.value[row].id), column };
        selectedRowKey.value = activeCell.value.rowKey;
        window.getSelection()?.removeAllRanges();
        void revealActiveCell();
    }

    function onVirtualScroll() {
        if (awaitingVirtualRow) void revealActiveCell(false);
    }

    watch(() => [rows.value.map(row => String(row.id)), columns.value] as const, ([keys, names], previous) => {
        if (!activeCell.value) return;
        if (!keys.includes(activeCell.value.rowKey) || JSON.stringify(names) !== JSON.stringify(previous?.[1])) {
            activeCell.value = null;
            selectedRowKey.value = "";
            awaitingVirtualRow = false;
        }
    }, { flush: "sync" });

    return { activeCell, activeCellId, cellId, selectCell, handleGridKeydown, onVirtualScroll, revealActiveCell };
}
