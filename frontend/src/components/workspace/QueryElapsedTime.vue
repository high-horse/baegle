<template>
    <span class="query-elapsed">{{ seconds }} sec</span>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";

const props = defineProps<{ startedAt: number }>();
const now = ref(performance.now());
const seconds = computed(() => Math.floor(Math.max(0, now.value - props.startedAt) / 1000));
let timer: ReturnType<typeof setInterval> | undefined;

watch(() => props.startedAt, () => {
    clearInterval(timer);
    now.value = performance.now();
    timer = setInterval(() => { now.value = performance.now(); }, 250);
}, { immediate: true });

onBeforeUnmount(() => clearInterval(timer));
</script>

<style scoped>
.query-elapsed { white-space: nowrap; font-variant-numeric: tabular-nums; }
</style>
