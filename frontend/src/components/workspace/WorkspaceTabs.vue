<template>
    <div
        ref="tabStrip"
        class="h-9 shrink-0 bg-[var(--surface-panel)] border-b border-[var(--border-default)] flex items-center min-w-0"
    >
        <!-- Horizontally scrollable tabs -->
        <q-scroll-area
            horizontal
            class="h-full flex-1 min-w-0 tabs-scroll-area"
        >
            <div class="tabs-content h-full flex items-center" @dragover="dragOverStrip" @drop.prevent="dropTab" @dragleave="leaveStrip">
                <!-- Tabs -->
                <div
                    v-for="(tab, index) in tabs"
                    :key="tab.id"
                    class="workspace-tab shrink-0 flex items-center"
                    :data-tab-id="tab.id"
                    draggable="true"
                    @dragstart="startDrag($event, tab.id)"
                    @dragend="endDrag"
                    @dragover.prevent="dragOverTab($event, tab.id)"
                    :style="{ opacity: draggedTabId === tab.id ? 0.45 : undefined }"
                    :data-drop-side="dropTarget?.id === tab.id ? dropTarget.side : undefined"
                    :class="{ 'workspace-tab-active': tab.id === activeTabId }"
                >
                    <q-menu context-menu dark class="bg-[var(--surface-panel)] text-gray-300">
                        <q-list dense style="min-width: 210px">
                            <q-item clickable v-close-popup :disable="closingTabs || tab.closing" @click="closeTab(tab.id)"><q-item-section>Close Tab</q-item-section></q-item>
                            <q-item clickable v-close-popup :disable="closingTabs || tabs.length < 2" @click="closeTabs(tabs.filter(item => item.id !== tab.id))"><q-item-section>Close Other Tabs</q-item-section></q-item>
                            <q-separator dark />
                            <q-item clickable v-close-popup :disable="closingTabs || index === 0" @click="closeTabs(tabs.slice(0, index))"><q-item-section>Close Tabs to the Left</q-item-section></q-item>
                            <q-item clickable v-close-popup :disable="closingTabs || index === tabs.length - 1" @click="closeTabs(tabs.slice(index + 1))"><q-item-section>Close Tabs to the Right</q-item-section></q-item>
                            <q-separator dark />
                            <q-item clickable v-close-popup :disable="closingTabs" @click="closeTabs(tabs)"><q-item-section>Close All Tabs</q-item-section></q-item>
                        </q-list>
                    </q-menu>
                    <!-- Tab button -->
                    <q-btn
                        flat
                        dense
                        no-caps
                        unelevated
                        class="tab-button"
                        :class="{ 'active-tab': tab.id === activeTabId }"
                        :aria-current="tab.id === activeTabId ? 'page' : undefined"
                        @click="selectTab(tab.id)"
                    >
                        <div
                            class="flex flex-nowrap items-center gap-2 min-w-0 whitespace-nowrap"
                        >
                            <!-- Query / Result icon -->
                            <q-icon
                                :name="
                                    tab.type === 'result'
                                        ? 'table_view'
                                        : 'code'
                                "
                                size="13px"
                            />

                            <!-- Tab title -->
                            <span class="max-w-32 truncate">
                                {{ tab.title }}
                            </span>

                            <!-- Dirty indicator -->
                            <span
                                v-if="(tab.type === 'query' && tab.dirty) || edits.hasPending(tab.id)"
                                class="text-secondary text-[length:var(--ui-font-size)]"
                            >
                                ●
                            </span>

                            <!-- Loading -->
                            <q-spinner
                                v-if="tab.loading"
                                color="primary"
                                size="11px"
                            />
                        </div>
                        <q-tooltip v-if="tab.type === 'result'" :delay="300">
                            {{ tab.title }}
                        </q-tooltip>
                    </q-btn>

                    <!-- Close -->
                    <q-btn
                        flat
                        dense
                        round
                        size="xs"
                        icon="close"
                        class="tab-close"
                        :disable="tab.closing"
                        @pointerdown.stop
                        @dragstart.stop.prevent
                        @click.stop="closeTab(tab.id)"
                    >
                        <q-tooltip>
                            Close {{ tab.title }}
                            <span v-if="tab.savedSql !== undefined"> and remove its saved console</span>
                        </q-tooltip>
                    </q-btn>
                </div>

                <!-- New query tab -->
                <q-btn
                    flat
                    dense
                    square
                    size="sm"
                    icon="add"
                    class="new-tab-button shrink-0"
                    color="grey-6"
                    title="New Query"
                    @click="createTab"
                >
                    <q-tooltip>New Query</q-tooltip>
                </q-btn>
            </div>
        </q-scroll-area>
    </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from "vue";
import { useTableEditsStore } from "@/stores/tableEditsStore";
const edits = useTableEditsStore();
import type { QueryTab } from "@/types/queryTab";

const props = defineProps<{
    tabs: QueryTab[];
    activeTabId: string | null;
    closingTabs?: boolean;
}>();

const emit = defineEmits<{
    createTab: [];
    selectTab: [id: string];
    closeTab: [id: string];
    closeTabs: [ids: string[]];
    moveTab: [id: string, targetId: string, side: "before" | "after"];
}>();

const draggedTabId = ref<string | null>(null);
const dropTarget = ref<{ id: string; side: "before" | "after" } | null>(null);
const tabStrip = ref<HTMLElement | null>(null);
let scrollFrame = 0;

watch(() => props.activeTabId, () => {
    cancelAnimationFrame(scrollFrame);
    scrollFrame = requestAnimationFrame(() => {
        const selected = tabStrip.value?.querySelector(".active-tab")?.closest<HTMLElement>(".workspace-tab");
        const scroller = tabStrip.value?.querySelector<HTMLElement>(".q-scrollarea__container");
        if (!selected || !scroller) return;
        const bounds = selected.getBoundingClientRect();
        const viewport = scroller.getBoundingClientRect();
        if (bounds.left < viewport.left) scroller.scrollLeft += bounds.left - viewport.left;
        else if (bounds.right > viewport.right) scroller.scrollLeft += bounds.right - viewport.right;
    });
}, { flush: "post" });
onBeforeUnmount(() => cancelAnimationFrame(scrollFrame));

function startDrag(event: DragEvent, id: string) {
    if (!event.dataTransfer) return;
    draggedTabId.value = id;
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", id);
    selectTab(id);
}

function dragOverTab(event: DragEvent, id: string) {
    if (!draggedTabId.value) return;
    const bounds = (event.currentTarget as HTMLElement).getBoundingClientRect();
    dropTarget.value = id === draggedTabId.value ? null : {
        id, side: event.clientX < bounds.left + bounds.width / 2 ? "before" : "after",
    };
}

function dragOverStrip(event: DragEvent) {
    if (!draggedTabId.value) return;
    event.preventDefault();
    if (event.dataTransfer) event.dataTransfer.dropEffect = "move";
    const strip = event.currentTarget as HTMLElement;
    if (!(event.target as HTMLElement).closest("[data-tab-id]")) {
        const last = props.tabs.at(-1);
        dropTarget.value = last && last.id !== draggedTabId.value ? { id: last.id, side: "after" } : null;
    }
    const scroller = strip.closest(".q-scrollarea__container");
    if (scroller) {
        const bounds = scroller.getBoundingClientRect();
        if (event.clientX < bounds.left + 36) scroller.scrollLeft -= 24;
        else if (event.clientX > bounds.right - 36) scroller.scrollLeft += 24;
    }
}

function dropTab() {
    if (draggedTabId.value && dropTarget.value) {
        emit("moveTab", draggedTabId.value, dropTarget.value.id, dropTarget.value.side);
    }
    endDrag();
}

function leaveStrip(event: DragEvent) {
    if (!(event.currentTarget as HTMLElement).contains(event.relatedTarget as Node | null)) {
        dropTarget.value = null;
    }
}

function endDrag() {
    draggedTabId.value = null;
    dropTarget.value = null;
}

function createTab() {
    emit("createTab");
}

function selectTab(id: string) {
    emit("selectTab", id);
}

function closeTab(id: string) {
    emit("closeTab", id);
}

function closeTabs(tabs: QueryTab[]) {
    emit('closeTabs', tabs.map(tab => tab.id));
}
</script>

<style scoped>
.workspace-tab {
    position: relative;
    cursor: grab;
    user-select: none;
    height: 100%;
    border: 1px solid transparent;
    border-bottom: 0;
    border-radius: 6px 6px 0 0;
    color: var(--text-muted);
    transition: background-color 120ms ease, color 120ms ease;
}
.workspace-tab:hover { background: var(--surface-hover); color: var(--text-primary); }
.workspace-tab.workspace-tab-active {
    background: var(--accent-surface);
    border-color: #568b83;
    color: #f1f6fc;
    box-shadow: 0 1px 0 var(--accent-surface);
}
.workspace-tab-active::before {
    content: "";
    position: absolute;
    inset: -1px -1px auto;
    height: 3px;
    border-radius: 6px 6px 0 0;
    background: var(--selection-border);
    pointer-events: none;
}
.workspace-tab-active .tab-button { font-weight: 600; }
.workspace-tab-active .tab-button :deep(.q-icon) { color: var(--selection-border); }
.workspace-tab:focus-within { outline: 1px solid var(--selection-border); outline-offset: -2px; }
.workspace-tab[data-drop-side]::after {
    content: "";
    position: absolute;
    top: 3px;
    bottom: 3px;
    width: 2px;
    background: var(--accent-color);
    z-index: 1;
    pointer-events: none;
}
.workspace-tab[data-drop-side="before"]::after { left: 0; }
.workspace-tab[data-drop-side="after"]::after { right: 0; }

.tabs-scroll-area {
    min-width: 0;
    min-height: 0;
}

/*
 * Keep horizontal scrolling enabled.
 * Disable vertical scrolling.
 */
.tabs-scroll-area :deep(.q-scrollarea__container) {
    overflow-x: auto !important;
    overflow-y: hidden !important;

    /* Firefox */
    scrollbar-width: none;

    /* IE / old Edge */
    -ms-overflow-style: none;
}

/*
 * Chrome / Safari / Edge
 */
.tabs-scroll-area :deep(.q-scrollarea__container::-webkit-scrollbar) {
    width: 0 !important;
    height: 0 !important;
    display: none !important;
}

/*
 * Hide Quasar's custom scrollbar.
 *
 * Scrolling still works because the container remains
 * horizontally scrollable.
 */
.tabs-scroll-area :deep(.q-scrollarea__bar),
.tabs-scroll-area :deep(.q-scrollarea__thumb) {
    display: none !important;
    opacity: 0 !important;
    visibility: hidden !important;
}


.tabs-content {
    width: max-content;
    min-width: 100%;

    /*
     * Very important:
     * Never allow tabs to wrap onto another line.
     */
    flex-wrap: nowrap !important;
    gap: 3px;
    padding: 4px 4px 0;
}


.tab-button {
    height: 100% !important;
    min-height: 0 !important;

    padding: 0 10px !important;

    border-radius: 0 !important;

    font-size: var(--ui-font-size);
    font-family: monospace;

    position: relative;
    color: inherit;
}

/*
 * Remove Quasar focus helper.
 */
.tab-button :deep(.q-focus-helper) {
    display: none;
}

.tab-close {
    width: 22px;
    height: 28px;
    min-height: 28px;

    margin-right: 2px;

    border-radius: 3px !important;
    color: inherit;
    opacity: .65;
}
.workspace-tab-active .tab-close, .workspace-tab:hover .tab-close, .tab-close:focus-visible { opacity: 1; }

.tab-close:hover {
    color: #ffffff !important;
    background: var(--border-default) !important;
}


.new-tab-button {
    width: 36px;
    height: 36px;
    min-height: 36px;

    border-radius: 0 !important;
}

.new-tab-button:hover {
    color: var(--accent-color) !important;
    background: var(--surface-hover) !important;
}
</style>
