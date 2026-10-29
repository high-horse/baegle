import { computed, nextTick, onScopeDispose, ref, watch, type ComputedRef, type Ref } from "vue";

type GridHandle = { $el: HTMLElement; scrollTo?: (row: number, alignment?: 'center') => void };

export function useResultCellNavigation(
    table: Ref<GridHandle | null>,
    rows: ComputedRef<Record<string, unknown>[]>,
    columns: ComputedRef<string[]>,
    selectedRowKey: Ref<string>,
    options: { scroller?: string; rowHeader?: boolean } = {},
) {
    const scrollerSelector = options.scroller ?? '.q-table__middle';
    const gridId = `result-grid-${crypto.randomUUID()}`;
    const activeCell = ref<{ rowKey: string; column: number } | null>(null);
    const selectionAnchor = ref<{ rowKey: string; column: number } | null>(null);
    const rowIndexes = computed(() => new Map(rows.value.map((row, index) => [String(row.id), index])));
    const activeRow = computed(() => rowIndexes.value.get(activeCell.value?.rowKey ?? '') ?? -1);
    const cellId = (row: number, column: number) => `${gridId}-${row}-${column}`;
    const activeCellId = computed(() => activeCell.value && activeRow.value >= 0
        ? cellId(activeRow.value, activeCell.value.column) : undefined);
    const selectionRange = computed(() => {
        if (!activeCell.value || activeRow.value < 0) return null;
        const anchor = selectionAnchor.value ?? activeCell.value;
        const anchorRow = rowIndexes.value.get(anchor.rowKey) ?? -1;
        if (anchorRow < 0) return null;
        return { fromRow: Math.min(anchorRow, activeRow.value), toRow: Math.max(anchorRow, activeRow.value),
            fromColumn: Math.min(anchor.column, activeCell.value.column), toColumn: Math.max(anchor.column, activeCell.value.column) };
    });
    function isCellSelected(row: number, columnName: string) {
        if (!columnName.startsWith('column-') || !selectionRange.value) return false;
        const column = Number(columnName.slice(7));
        const range = selectionRange.value;
        return row >= range.fromRow && row <= range.toRow && column >= range.fromColumn && column <= range.toColumn;
    }
    let awaitingVirtualRow = false;

    async function revealActiveCell(allowVirtualScroll = true) {
        await nextTick();
        const root = table.value?.$el as HTMLElement | undefined;
        if (!root || !activeCellId.value) return;
        const cell = root.querySelector<HTMLElement>(`#${activeCellId.value}`);
        if (!cell) {
            if (allowVirtualScroll && activeRow.value >= 0) {
                awaitingVirtualRow = true;
                table.value?.scrollTo?.(activeRow.value, "center");
            }
            return;
        }
        awaitingVirtualRow = false;
        const scroller = root.querySelector<HTMLElement>(scrollerSelector);
        if (!scroller) return;
        const viewport = scroller.getBoundingClientRect();
        const rect = cell.getBoundingClientRect();
        const headerHeight = root.querySelector("thead")?.getBoundingClientRect().height ?? 0;
        const rowHeaderWidth = options.rowHeader === false ? 0 : root.querySelector("thead th")?.getBoundingClientRect().width ?? 0;
        const pinnedWidth = Array.from(root.querySelectorAll<HTMLElement>('thead th[data-pinned]')).reduce((width, header) => width + header.getBoundingClientRect().width, 0);
        const top = viewport.top + headerHeight;
        const left = viewport.left + rowHeaderWidth + pinnedWidth;
        const bottom = viewport.top + scroller.clientHeight;
        const right = viewport.left + scroller.clientWidth;
        if (rect.top < top) scroller.scrollTop -= top - rect.top;
        else if (rect.bottom > bottom) scroller.scrollTop += rect.bottom - bottom;
        if (!cell.hasAttribute('data-pinned')) {
            if (rect.left < left) scroller.scrollLeft -= left - rect.left;
            else if (rect.right > right) scroller.scrollLeft += rect.right - right;
        }
    }

    function selectCell(rowKey: string, columnName: string, focus = true, extend = false) {
        if (!columnName.startsWith("column-")) return;
        const column = Number(columnName.slice(7));
        if (extend) selectionAnchor.value ??= activeCell.value ?? { rowKey, column };
        else selectionAnchor.value = { rowKey, column };
        // Pointer events often repeat within one cell. Preserve reference identity
        // so they don't rerender the grid and every cell's context menu.
        if (activeCell.value?.rowKey !== rowKey || activeCell.value.column !== column) activeCell.value = { rowKey, column };
        selectedRowKey.value = rowKey;
        // Keep focus on a stable element when virtual rows are recycled.
        if (focus) (table.value?.$el as HTMLElement | undefined)?.focus({ preventScroll: true });
    }

    let drag: { pointerId: number; x: number; y: number } | null = null;
    let scrollFrame = 0;
    function stopCellDrag() {
        drag = null;
        if (scrollFrame) window.cancelAnimationFrame(scrollFrame);
        scrollFrame = 0;
        window.removeEventListener('pointermove', moveCellDrag);
        window.removeEventListener('pointerup', endCellDrag);
        window.removeEventListener('pointercancel', endCellDrag);
        window.removeEventListener('blur', stopCellDrag);
    }
    function extendDragAt(x: number, y: number) {
        const root = table.value?.$el as HTMLElement | undefined;
        const cell = document.elementFromPoint(x, y)?.closest<HTMLElement>('[data-grid-column]');
        if (!cell || !root?.contains(cell) || cell.dataset.gridRow == null) return;
        selectCell(cell.dataset.gridRow, cell.dataset.gridColumn!, false, true);
    }
    function scrollCellDrag() {
        scrollFrame = 0;
        if (!drag) return;
        const root = table.value?.$el as HTMLElement | undefined;
        const scroller = root?.querySelector<HTMLElement>(scrollerSelector);
        if (scroller) {
            const rect = scroller.getBoundingClientRect();
            const top = rect.top + (root?.querySelector('thead')?.getBoundingClientRect().height ?? 0);
            const left = rect.left + (options.rowHeader === false ? 0 : root?.querySelector('thead th')?.getBoundingClientRect().width ?? 0);
            const right = rect.left + scroller.clientWidth;
            const bottom = rect.top + scroller.clientHeight;
            const speed = (point: number, min: number, max: number) => point < min + 20 ? -14 : point > max - 20 ? 14 : 0;
            const oldTop = scroller.scrollTop, oldLeft = scroller.scrollLeft;
            const nextTop = Math.max(0, Math.min(scroller.scrollHeight - scroller.clientHeight, oldTop + speed(drag.y, top, bottom)));
            const nextLeft = Math.max(0, Math.min(scroller.scrollWidth - scroller.clientWidth, oldLeft + speed(drag.x, left, right)));
            // Finish layout reads and hit-testing before writing scroll offsets.
            extendDragAt(Math.max(left + 1, Math.min(right - 1, drag.x)), Math.max(top + 1, Math.min(bottom - 1, drag.y)));
            if (nextTop !== oldTop) scroller.scrollTop = nextTop;
            if (nextLeft !== oldLeft) scroller.scrollLeft = nextLeft;
            // Only edge scrolling needs another frame without pointer movement.
            if (nextTop !== oldTop || nextLeft !== oldLeft) scheduleDragFrame();
        } else {
            extendDragAt(drag.x, drag.y);
        }
    }
    function scheduleDragFrame() {
        if (!scrollFrame) scrollFrame = window.requestAnimationFrame(scrollCellDrag);
    }
    function moveCellDrag(event: PointerEvent) {
        if (!drag || event.pointerId !== drag.pointerId) return;
        if (!(event.buttons & 1)) { stopCellDrag(); return; }
        event.preventDefault();
        drag.x = event.clientX;
        drag.y = event.clientY;
        scheduleDragFrame();
    }
    function endCellDrag(event: PointerEvent) {
        if (event.pointerId !== drag?.pointerId) return;
        // A quick release can arrive before the next animation frame. Apply the
        // final endpoint now so copying immediately after release is accurate.
        if (event.type !== 'pointercancel' && scrollFrame) {
            window.cancelAnimationFrame(scrollFrame);
            scrollCellDrag();
        }
        stopCellDrag();
    }
    function startCellDrag(event: PointerEvent, rowKey: string, columnName: string) {
        if (event.button !== 0 || event.pointerType === 'touch' || !columnName.startsWith('column-')) return;
        stopCellDrag();
        event.preventDefault();
        selectCell(rowKey, columnName, true, event.shiftKey);
        window.getSelection()?.removeAllRanges();
        drag = { pointerId: event.pointerId, x: event.clientX, y: event.clientY };
        window.addEventListener('pointermove', moveCellDrag);
        window.addEventListener('pointerup', endCellDrag);
        window.addEventListener('pointercancel', endCellDrag);
        window.addEventListener('blur', stopCellDrag);
    }
    function selectContextCell(rowKey: string, columnName: string) {
        const row = rowIndexes.value.get(rowKey) ?? -1;
        // Opening a menu within a range must not collapse that range before Copy.
        if (!isCellSelected(row, columnName)) selectCell(rowKey, columnName);
    }
    onScopeDispose(stopCellDrag);

    function handleGridKeydown(event: KeyboardEvent) {
        // Leave header buttons, text fields, and modified shortcuts alone.
        if (event.target !== event.currentTarget || event.isComposing || event.altKey) return;
        if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'a' && !event.shiftKey) {
            if (!rows.value.length || !columns.value.length) return;
            event.preventDefault();
            event.stopPropagation();
            selectionAnchor.value = { rowKey: String(rows.value[0].id), column: 0 };
            activeCell.value = { rowKey: String(rows.value[rows.value.length - 1].id), column: columns.value.length - 1 };
            selectedRowKey.value = activeCell.value.rowKey;
            window.getSelection()?.removeAllRanges();
            return;
        }
        if (event.ctrlKey || event.metaKey) return;
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
        selectCell(String(rows.value[row].id), `column-${column}`, false, event.shiftKey);
        window.getSelection()?.removeAllRanges();
        void revealActiveCell();
    }

    function onVirtualScroll() {
        if (awaitingVirtualRow) void revealActiveCell(false);
    }

    watch(() => [rows.value.map(row => String(row.id)), columns.value] as const, ([keys, names], previous) => {
        stopCellDrag();
        if (!activeCell.value) return;
        if (!keys.includes(activeCell.value.rowKey) || JSON.stringify(names) !== JSON.stringify(previous?.[1])) {
            activeCell.value = null;
            selectionAnchor.value = null;
            selectedRowKey.value = "";
            awaitingVirtualRow = false;
        } else if (JSON.stringify(keys) !== JSON.stringify(previous?.[0])) {
            // Keep the active record, but don't silently select a different
            // rectangle when filtering or sorting changes the displayed order.
            selectionAnchor.value = activeCell.value;
        }
    }, { flush: "sync" });

    return { activeCell, activeCellId, cellId, selectCell, selectContextCell, startCellDrag, handleGridKeydown, onVirtualScroll, revealActiveCell, selectionRange, isCellSelected };
}
