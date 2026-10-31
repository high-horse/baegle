<template>
    <img :src="appIcon" alt="" aria-hidden="true" :width="size" :height="size" class="app-icon" />
</template>

<script setup lang="ts">
import appIcon from "../../../build/appicon.png";

withDefaults(defineProps<{ size?: number }>(), { size: 32 });
</script>

<style scoped>
.app-icon {
    display: block;
    flex-shrink: 0;
    object-fit: contain;
    border-radius: 6px;
}
</style>
