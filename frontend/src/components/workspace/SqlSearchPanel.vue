<template>
    <section class="border-b border-[#484848] bg-[#303234] px-2 py-1 text-[13px] text-[#d1d5db]" aria-label="Find and replace">
        <div class="flex flex-nowrap! items-center gap-2">
            <div class="flex flex-nowrap! h-7 w-[360px] max-w-full min-w-0 items-center rounded border bg-[#2d2d2d] pl-2 pr-0.5 focus-within:border-amber-500"
                :class="invalidPattern ? 'border-red-400' : 'border-[#505050]'">
                <q-input v-model="findText" borderless dense dark hide-bottom-space placeholder="Search…" aria-label="Find"
                    class="h-7 w-0! min-w-0! flex-1! [&_.q-field\_\_control]:h-7! [&_.q-field\_\_control]:min-h-0! [&_.q-field\_\_control-container]:h-full!"
                    input-class="text-[13px]! leading-normal! h-full! min-h-0! py-0!" @keydown.enter.prevent.stop="find($event.shiftKey)" />
                <q-btn flat dense no-caps size="12px" label="Aa" class="h-6 min-h-6! w-6 min-w-6! shrink-0! p-0!" :color="matchCase ? 'amber' : undefined"
                    :class="matchCase ? 'bg-amber-500/15' : 'text-[#a0a0a0]'" :aria-pressed="matchCase" title="Match case" aria-label="Match case" @click="matchCase = !matchCase" />
                <q-btn flat dense no-caps size="12px" label="ab" class="h-6 min-h-6! w-6 min-w-6! shrink-0! p-0! underline underline-offset-2" :color="wholeWord ? 'amber' : undefined"
                    :class="wholeWord ? 'bg-amber-500/15' : 'text-[#a0a0a0]'" :aria-pressed="wholeWord" title="Match whole word" aria-label="Match whole word" @click="wholeWord = !wholeWord" />
                <q-btn flat dense no-caps size="12px" label=".*" class="h-6 min-h-6! w-6 min-w-6! shrink-0! p-0! font-mono" :color="regexp ? 'amber' : undefined"
                    :class="regexp ? 'bg-amber-500/15' : 'text-[#a0a0a0]'" :aria-pressed="regexp" title="Regular expression" aria-label="Regular expression" @click="regexp = !regexp" />
            </div>
            <div class="flex flex-nowrap! h-7 shrink-0 items-center gap-1 text-[#a0a0a0]">
                <q-btn v-if="!readOnly.value" flat dense size="12px" icon="find_replace" class="h-6 min-h-6! w-6 min-w-6! shrink-0! p-0!" :color="replaceExpanded ? 'amber' : undefined"
                    :class="replaceExpanded ? 'bg-amber-500/15' : ''" :aria-expanded="replaceExpanded" title="Toggle replace" aria-label="Toggle replace" @click="replaceExpanded = !replaceExpanded" />
                <q-btn flat dense size="12px" icon="select_all" class="h-6 min-h-6! w-6 min-w-6! shrink-0! p-0!" title="Select all matches" aria-label="Select all matches" :disable="!hasMatches" @click="selectMatches(view)" />
                <span class="mx-1 h-4 border-l border-[#505050]" aria-hidden="true" />
                <q-btn flat dense size="12px" icon="chevron_left" class="h-6 min-h-6! w-6 min-w-6! shrink-0! p-0!" title="Previous match (Shift+Enter)" aria-label="Previous match" :disable="!hasMatches" @click="find(true)" />
                <q-btn flat dense size="12px" icon="chevron_right" class="h-6 min-h-6! w-6 min-w-6! shrink-0! p-0!" title="Next match (Enter)" aria-label="Next match" :disable="!hasMatches" @click="find(false)" />
                <span class="min-w-10 px-1 text-xs tabular-nums" :class="findText && !hasMatches ? 'text-red-300' : ''" role="status" aria-label="Search matches">{{ matchIndex }}/{{ matches.length }}</span>
                <q-btn flat dense size="12px" icon="close" class="h-6 min-h-6! w-6 min-w-6! shrink-0! p-0!" title="Close search (Esc)" aria-label="Close find and replace" @click="closeSearchPanel(view)" />
            </div>
        </div>
        <div v-if="replaceExpanded && !readOnly.value" class="mt-1 flex flex-wrap items-center gap-2">
            <q-input v-model="replaceText" borderless dense dark hide-bottom-space placeholder="Replace with…" aria-label="Replace with"
                class="h-7 w-[360px] max-w-full min-w-0 rounded border border-[#505050] bg-[#2d2d2d] px-2 focus-within:border-amber-500 [&_.q-field\_\_control]:h-7! [&_.q-field\_\_control]:min-h-0! [&_.q-field\_\_control-container]:h-full!"
                input-class="text-[13px]! leading-normal! h-full! min-h-0! py-0!" @keydown.enter.prevent.stop="replaceNext(view)" />
            <div class="flex items-center gap-1 text-[#a0a0a0]">
                <q-btn flat dense size="12px" icon="find_replace" class="h-6 min-h-6! w-6 min-w-6! shrink-0! p-0!" title="Replace next match" aria-label="Replace next match" :disable="!hasMatches" @click="replaceNext(view)" />
                <q-btn flat dense size="12px" icon="done_all" class="h-6 min-h-6! w-6 min-w-6! shrink-0! p-0!" title="Replace all matches" aria-label="Replace all matches" :disable="!hasMatches" @click="replaceAll(view)" />
            </div>
        </div>
        <div v-if="invalidPattern" role="alert" class="mt-1 text-xs text-red-300">Invalid regular expression</div>
    </section>
</template>

<script setup lang="ts">
import { computed, ref, type ShallowRef } from "vue";
import { QBtn, QInput } from "quasar";
import type { EditorSelection, Text } from "@codemirror/state";
import type { EditorView } from "@codemirror/view";
import { SearchQuery, closeSearchPanel, findNext, findPrevious, replaceAll, replaceNext, selectMatches, setSearchQuery } from "@codemirror/search";

const props = defineProps<{ view: EditorView; query: ShallowRef<SearchQuery>; readOnly: ShallowRef<boolean>; doc: ShallowRef<Text>; selection: ShallowRef<EditorSelection> }>();
const replaceExpanded = ref(false);
function queryField<K extends 'search' | 'replace' | 'caseSensitive' | 'wholeWord' | 'regexp'>(field: K) {
    return computed({
        get: () => props.query.value[field],
        set: (value: SearchQuery[K]) => {
            props.view.dispatch({ effects: setSearchQuery.of(new SearchQuery({ ...props.query.value, [field]: value })) });
        },
    });
}
const findText = queryField('search');
const replaceText = queryField('replace');
const matchCase = queryField('caseSensitive');
const wholeWord = queryField('wholeWord');
const regexp = queryField('regexp');
const invalidPattern = computed(() => !!findText.value && regexp.value && !props.query.value.valid);
const matches = computed(() => {
    const found: { from: number; to: number }[] = [];
    if (!props.query.value.valid) return found;
    const cursor = props.query.value.getCursor(props.doc.value);
    for (let match = cursor.next(); !match.done; match = cursor.next()) found.push(match.value);
    return found;
});
const hasMatches = computed(() => matches.value.length > 0);
const matchIndex = computed(() => {
    const { from, to } = props.selection.value.main;
    return matches.value.findIndex(match => match.from === from && match.to === to) + 1;
});
function find(backwards: boolean) {
    (backwards ? findPrevious : findNext)(props.view);
}
</script>
