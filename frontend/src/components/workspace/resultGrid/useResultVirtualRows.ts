import { computed, nextTick, onScopeDispose, ref, shallowRef, watch, type ComputedRef, type Ref } from 'vue';

type TableHandle = { $el: HTMLElement };

// Fixed-height result rows update on animation frames, without a scroll debounce.
// Keep a viewport of spare rows on each side and retain that window until its
// buffer is half consumed, so small scrolls do not rebuild the table body.
export function useResultVirtualRows<T>(table: Ref<TableHandle | null>, rows: ComputedRef<T[]>, rowHeight: ComputedRef<number>) {
    const range = shallowRef({ from: 0, to: Math.min(rows.value.length, 80) });
    const viewportHeight = ref(600);
    let scroller: HTMLElement | null = null;
    let observer: ResizeObserver | undefined;
    let frame = 0;
    let disposed = false;

    function updateRange(force = false) {
        const count = rows.value.length;
        const visible = Math.max(1, Math.ceil(viewportHeight.value / rowHeight.value));
        const buffer = Math.max(12, visible);
        const first = Math.min(Math.max(0, count - 1), Math.floor((scroller?.scrollTop ?? 0) / rowHeight.value));
        const current = range.value;
        if (!force && first >= current.from + buffer / 2 && first + visible <= current.to - buffer / 2) return;
        const from = Math.max(0, first - buffer);
        const to = Math.min(count, first + visible + buffer);
        if (from !== current.from || to !== current.to) range.value = { from, to };
    }

    function scheduleScroll() {
        if (frame) return;
        frame = requestAnimationFrame(() => {
            frame = 0;
            updateRange();
        });
    }

    function detach() {
        scroller?.removeEventListener('scroll', scheduleScroll);
        observer?.disconnect();
        if (frame) cancelAnimationFrame(frame);
        frame = 0;
        scroller = null;
    }

    watch(table, async handle => {
        detach();
        await nextTick();
        if (disposed || table.value !== handle) return;
        scroller = handle?.$el.querySelector<HTMLElement>('.q-table__middle') ?? null;
        if (!scroller) return;
        const measure = () => {
            if (scroller && scroller.clientHeight > 0) viewportHeight.value = scroller.clientHeight;
            updateRange(true);
        };
        measure();
        scroller.addEventListener('scroll', scheduleScroll, { passive: true });
        observer = new ResizeObserver(measure);
        observer.observe(scroller);
    }, { immediate: true, flush: 'post' });

    watch([rows, rowHeight], () => updateRange(true), { flush: 'sync' });
    onScopeDispose(() => { disposed = true; detach(); });

    function scrollToRow(index: number) {
        if (!scroller) return;
        const row = Math.max(0, Math.min(rows.value.length - 1, index));
        scroller.scrollTop = Math.max(0, row * rowHeight.value - (viewportHeight.value - rowHeight.value) / 2);
        updateRange(true);
    }

    return {
        visibleRows: computed(() => rows.value.slice(range.value.from, range.value.to)),
        firstRow: computed(() => range.value.from),
        paddingTop: computed(() => range.value.from * rowHeight.value),
        paddingBottom: computed(() => Math.max(0, rows.value.length - range.value.to) * rowHeight.value),
        scrollToRow,
    };
}
