<template>
    <div
        class="h-9 shrink-0 bg-[#353535] border-b border-[#484848] flex items-center min-w-0"
    >
        <!-- Horizontally scrollable tabs -->
        <q-scroll-area
            horizontal
            class="h-full flex-1 min-w-0 tabs-scroll-area"
        >
            <div class="tabs-content h-full flex items-center" @dragover="dragOverStrip" @drop.prevent="dropTab" @dragleave="leaveStrip">
                <!-- Tabs -->
                <div
                    v-for="tab in tabs"
                    :key="tab.id"
                    class="workspace-tab h-full shrink-0 flex items-center border-r border-[#484848]"
                    :data-tab-id="tab.id"
                    draggable="true"
                    @dragstart="startDrag($event, tab.id)"
                    @dragend="endDrag"
                    @dragover.prevent="dragOverTab($event, tab.id)"
                    :style="{ opacity: draggedTabId === tab.id ? 0.45 : undefined }"
                    :data-drop-side="dropTarget?.id === tab.id ? dropTarget.side : undefined"
                    :class="
                        tab.id === activeTabId
                            ? 'bg-[#282828]'
                            : 'bg-[#353535] hover:bg-[#404040]'
                    "
                >
                    <!-- Tab button -->
                    <q-btn
                        flat
                        dense
                        no-caps
                        unelevated
                        class="tab-button"
                        :class="
                            tab.id === activeTabId
                                ? 'text-amber-400 active-tab'
                                : 'text-[#a0a0a0] hover:text-[#d1d5db]'
                        "
                        @click="selectTab(tab.id)"
                    >
                        <div
                            class="flex items-center gap-2 min-w-0 whitespace-nowrap"
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
                                class="text-amber-500 text-[9px]"
                            >
                                ●
                            </span>

                            <!-- Loading -->
                            <q-spinner
                                v-if="tab.loading"
                                color="amber"
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
                        @pointerdown.stop
                        @dragstart.stop.prevent
                        :class="
                            tab.id === activeTabId
                                ? 'text-[#a0a0a0]'
                                : 'text-[#909090]'
                        "
                        @click.stop="closeTab(tab.id)"
                    >
                        <q-tooltip>
                            Close {{ tab.title }}
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
import { ref } from "vue";
import { useTableEditsStore } from "@/stores/tableEditsStore";
const edits = useTableEditsStore();
import type { QueryTab } from "@/types/queryTab";

const props = defineProps<{
    tabs: QueryTab[];
    activeTabId: string | null;
}>();

const emit = defineEmits<{
    createTab: [];
    selectTab: [id: string];
    closeTab: [id: string];
    moveTab: [id: string, targetId: string, side: "before" | "after"];
}>();

const draggedTabId = ref<string | null>(null);
const dropTarget = ref<{ id: string; side: "before" | "after" } | null>(null);

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
</script>

<style scoped>
.workspace-tab {
    position: relative;
    cursor: grab;
    user-select: none;
}
.workspace-tab[data-drop-side]::after {
    content: "";
    position: absolute;
    top: 3px;
    bottom: 3px;
    width: 2px;
    background: #f59e0b;
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
}


.tab-button {
    height: 36px !important;
    min-height: 36px !important;

    padding: 0 10px !important;

    border-radius: 0 !important;

    font-size: 11px;
    font-family: monospace;

    position: relative;
}

/*
 * Active tab bottom border
 */
.tab-button.active-tab::after {
    content: "";

    position: absolute;

    left: 0;
    right: 0;
    bottom: 0;

    height: 2px;

    background: #f59e0b;
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
}

.tab-close:hover {
    color: #ffffff !important;
    background: #484848 !important;
}


.new-tab-button {
    width: 36px;
    height: 36px;
    min-height: 36px;

    border-radius: 0 !important;
}

.new-tab-button:hover {
    color: #f59e0b !important;
    background: #404040 !important;
}
</style>
