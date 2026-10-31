<template>
    <q-page class="welcome-page">
        <div class="welcome-page-content">
            <div class="welcome-intro">
                <h1>Connect to your data.</h1>
                <p>Choose a saved connection or set up a new one.</p>
            </div>
            <WelcomeIndex />
        </div>
    </q-page>
</template>

<script setup lang="ts">
import WelcomeIndex from '../components/welcome/WelcomeIndex.vue';
</script>

<style scoped>
.welcome-page { padding: 24px clamp(20px, 3vw, 44px) 32px; background: var(--surface-base); }
.welcome-page-content { width: 100%; max-width: 1050px; margin: 0 auto; }
.welcome-intro { margin-bottom: 20px; }
h1 { margin: 0 0 6px; color: var(--text-primary); font-size: calc(var(--ui-font-size) + 17px); line-height: 1.25; font-weight: 600; letter-spacing: -.8px; }
p { margin: 0; color: var(--text-muted); font-size: calc(var(--ui-font-size) + 2px); line-height: 1.6; }
@media (max-width: 600px) { .welcome-page { padding: 24px 16px; } .welcome-intro { margin-bottom: 20px; } h1 { font-size: calc(var(--ui-font-size) + 13px); } }
</style>
