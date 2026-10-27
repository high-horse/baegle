import { computed, ref, watch, type ComputedRef } from 'vue';
import type { QueryColumn } from '@/types/queryTab';

export function useColumnLayout(columns: ComputedRef<QueryColumn[]>, storageKey: ComputedRef<string>) {
  const order = ref<number[]>([]), hidden = ref<number[]>([]), pinned = ref<number[]>([]);
  const search = ref('');
  watch(() => [storageKey.value, columns.value.map(c => c.Name)] as const, () => {
    const all = columns.value.map((_, i) => i);
    let saved: { order?: number[]; hidden?: number[]; pinned?: number[] } = {};
    try { saved = JSON.parse(localStorage.getItem(storageKey.value) || '{}'); } catch { /* Defaults when unavailable. */ }
    const valid = (v: unknown): number[] => Array.isArray(v) ? [...new Set(v.filter(i => Number.isInteger(i) && all.includes(i)))] : [];
    const stored = valid(saved?.order);
    order.value = [...stored, ...all.filter(i => !stored.includes(i))];
    hidden.value = valid(saved?.hidden);
    if (hidden.value.length === all.length) hidden.value = [];
    pinned.value = valid(saved?.pinned);
  }, { immediate: true });
  const visible = computed(() => {
    const shown = order.value.filter(i => !hidden.value.includes(i));
    return [...shown.filter(i => pinned.value.includes(i)), ...shown.filter(i => !pinned.value.includes(i))];
  });
  const displayedColumns = computed(() => visible.value.map(i => columns.value[i]));
  function save() {
    try { localStorage.setItem(storageKey.value, JSON.stringify({ order: order.value, hidden: hidden.value, pinned: pinned.value })); } catch { /* Layout still works in memory. */ }
  }
  function toggleHidden(index: number) {
    if (hidden.value.includes(index)) hidden.value = hidden.value.filter(i => i !== index);
    else if (visible.value.length > 1) hidden.value.push(index);
    save();
  }
  function togglePinned(index: number) {
    pinned.value = pinned.value.includes(index) ? pinned.value.filter(i => i !== index) : [...pinned.value, index];
    save();
  }
  function move(index: number, offset: number) {
    const position = order.value.indexOf(index), target = position + offset;
    if (target < 0 || target >= order.value.length) return;
    [order.value[position], order.value[target]] = [order.value[target], order.value[position]];
    save();
  }
  function reset() {
    order.value = columns.value.map((_, i) => i); hidden.value = []; pinned.value = []; save();
  }
  return { order, hidden, pinned, search, visible, displayedColumns, toggleHidden, togglePinned, move, reset };
}
