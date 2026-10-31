<template>
    <q-btn flat dense type="button" class="pagination-button" title="Workspace settings"
        aria-label="Workspace settings" @click="openSettings">
        <q-icon name="settings" size="16px" />
    </q-btn>
    <q-dialog v-model="visible" :persistent="saving">
        <q-card class="result-settings-card">
            <q-form @submit.prevent="save">
                <q-card-section>
                    <div class="text-h6">Workspace settings</div>
                    <div class="text-caption text-grey-5">Shared across all connections and saved for next time.</div>
                </q-card-section>
                <q-card-section class="q-pt-none">
                    <q-banner v-if="error" class="bg-red-10 text-red-2 q-mb-md" role="alert">{{ error }}</q-banner>
                    <div class="theme-preference q-mb-lg">
                        <div class="q-mb-sm">Theme accent</div>
                        <div class="theme-color-inputs">
                            <input type="color" aria-label="Pick theme accent color" :value="isAccentColor(draft.accentColor) ? draft.accentColor : defaultResultSettings.accentColor"
                                :disabled="loading || saving" @input="draft.accentColor = ($event.target as HTMLInputElement).value" />
                            <q-input v-model="draft.accentColor" label="Accent color (hex)" outlined dense :disable="loading || saving"
                                :rules="[value => isAccentColor(value) || 'Use a six-digit hex color, such as #59cbb7']" />
                        </div>
                        <div class="theme-presets q-mt-sm">
                            <button v-for="preset in accentPresets" :key="preset.color" type="button" :title="preset.name"
                                :aria-label="`Use ${preset.name} theme`" :aria-pressed="draft.accentColor.toLowerCase() === preset.color"
                                :style="{ backgroundColor: preset.color }" :disabled="loading || saving" @click="draft.accentColor = preset.color" />
                        </div>
                        <div class="text-caption text-grey-5 q-mt-sm">Preview changes here. Save to keep them; Cancel restores your previous color.</div>
                    </div>
                    <q-input v-model.number="draft.rowsPerPage" type="number" label="Rows per page" outlined dense
                        :min="1" :disable="loading || saving"
                        :rules="[value => Number.isSafeInteger(value) && value >= 1 || 'Enter a positive whole number']"
                        hint="No upper limit. Larger pages use more memory and take longer to load. Applies on the next run or refresh." />
                    <q-input v-model.number="draft.queryTimeoutSeconds" type="number" label="Query timeout (seconds)" outlined dense
                        class="q-mt-md" :min="0" :max="86400" :disable="loading || saving"
                        :rules="[value => Number.isInteger(value) && value >= 0 && value <= 86400 || 'Enter a whole number from 0 to 86400']"
                        hint="Maximum time per query, page load, or script. 0 disables the timeout. Applies to new executions." />
                    <q-select v-model="draft.fontSize" :options="[10, 11, 12, 13, 14, 16, 18]"
                        label="Workspace and grid font size (px)" outlined dense class="q-mt-md" :disable="loading || saving"
                        hint="Tables, row details, tabs, toolbars, menus, and dialogs. Updates immediately after saving." />
                    <q-select v-model="draft.sidebarFontSize" :options="[10, 11, 12, 13, 14, 16, 18, 20, 22, 24]"
                        label="Sidebar font size (px)" outlined dense class="q-mt-md" :disable="loading || saving" />
                    <q-select v-model="draft.consoleFontSize" :options="[10, 11, 12, 13, 14, 16, 18, 20, 22, 24]"
                        label="SQL editor and DDL font size (px)" outlined dense class="q-mt-md" :disable="loading || saving" />
                    <q-input v-model.number="draft.importLimitMiB" type="number" clearable label="Import file size limit (MiB)" outlined dense
                        class="q-mt-md" :min="0" :disable="loading || saving"
                        :rules="[value => value == null || value === '' || Number.isSafeInteger(value) && value >= 0 || 'Enter a nonnegative whole number']"
                        hint="Empty or 0 means unlimited. Applies to SQL scripts and data imports." />
                    <q-toggle v-model="draft.stripedRows" label="Alternating row colors" color="primary"
                        class="block q-mt-sm" :disable="loading || saving" />
                    <q-toggle v-model="draft.showNulls" label="Show NULL values explicitly" color="primary" class="block"
                        :disable="loading || saving" />
                </q-card-section>
                <q-card-actions align="right" class="q-pa-md">
                    <q-btn flat no-caps label="Restore defaults" :disable="loading || saving"
                        @click="draft = { ...defaultResultSettings }" />
                    <q-space />
                    <q-btn flat no-caps label="Cancel" :disable="saving" v-close-popup />
                    <q-btn unelevated no-caps type="submit" color="primary" text-color="black" label="Save"
                        :loading="saving" :disable="loading" />
                </q-card-actions>
                <q-inner-loading :showing="loading" color="primary" />
            </q-form>
        </q-card>
    </q-dialog>
</template>

<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from "vue";
import { defaultResultSettings, useResultSettingsStore } from "@/stores/resultSettingsStore";
import { isAccentColor } from '@/utils/accentTheme';

const store = useResultSettingsStore();
const visible = ref(false);
const loading = ref(false);
const saving = ref(false);
const error = ref("");
const draft = ref({ ...store.settings });
const accentPresets = [
    { name: 'Teal', color: '#59cbb7' }, { name: 'Blue', color: '#7cb7f0' },
    { name: 'Lavender', color: '#b7a5e8' }, { name: 'Rose', color: '#e59bb0' },
    { name: 'Amber', color: '#e5b769' }, { name: 'Sage', color: '#a8c68d' },
];
watch(() => [visible.value, draft.value.accentColor] as const, ([open, color]) => {
    store.accentPreview = open && isAccentColor(color) ? color : null;
});
onBeforeUnmount(() => { if (visible.value) store.accentPreview = null; });
async function openSettings() {
    visible.value = true;
    loading.value = true;
    error.value = "";
    try {
        // Pick up preferences saved in another app window.
        await store.load();
        draft.value = { ...store.settings };
    } catch (failure) {
        draft.value = { ...store.settings };
        error.value = `Could not load settings: ${String(failure)}`;
    } finally { loading.value = false; }
}
async function save() {
    if (loading.value || saving.value) return;
    saving.value = true;
    error.value = "";
    try {
        await store.save({ ...draft.value, importLimitMiB: Number(draft.value.importLimitMiB) || 0 });
        visible.value = false;
    } catch (failure) {
        error.value = `Could not save settings: ${String(failure)}`;
    } finally { saving.value = false; }
}
</script>

<style scoped>
.result-settings-card {
    width: 460px;
    max-width: calc(100vw - 32px);
    background: var(--surface-panel);
    color: var(--text-primary);
}
.theme-color-inputs { display: flex; align-items: flex-start; gap: 12px; }
.theme-color-inputs input[type="color"] { width: 44px; height: 40px; flex-shrink: 0; padding: 3px; border: 1px solid var(--border-default); border-radius: 5px; background: var(--surface-data); cursor: pointer; }
.theme-color-inputs .q-field { flex: 1; min-width: 0; }
.theme-presets { display: flex; gap: 10px; }
.theme-presets button { width: 26px; height: 26px; border: 2px solid transparent; border-radius: 50%; cursor: pointer; }
.theme-presets button[aria-pressed="true"] { border-color: var(--surface-base); outline: 2px solid var(--text-primary); }
.theme-presets button:focus-visible, .theme-color-inputs input:focus-visible { outline: 2px solid var(--text-primary); outline-offset: 3px; }
.theme-presets button:disabled { opacity: .5; cursor: default; }

.pagination-button {
    width: 24px;
    height: 24px;
    min-height: 24px;
    padding: 0;
    border: 1px solid var(--border-default);
    background: var(--surface-data);
    color: var(--text-muted);
}

.pagination-button:hover {
    color: var(--accent-color);
    background: var(--border-default);
}
</style>
