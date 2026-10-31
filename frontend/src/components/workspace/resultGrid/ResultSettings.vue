<template>
    <q-btn flat dense type="button" class="pagination-button" title="Workspace settings"
        aria-label="Workspace settings" @click="openSettings">
        <q-icon name="settings" size="16px" />
    </q-btn>
    <q-dialog v-model="visible" :persistent="saving" aria-labelledby="workspace-settings-title">
        <q-card class="result-settings-card">
            <q-form class="settings-form" @submit.prevent="save">
                <header class="settings-header">
                    <div class="settings-heading-icon"><q-icon name="tune" size="23px" /></div>
                    <div class="settings-heading">
                        <h2 id="workspace-settings-title">Workspace settings</h2>
                        <p>Make this workspace feel like yours.</p>
                    </div>
                    <q-btn flat round dense icon="close" aria-label="Close settings" :disable="saving" v-close-popup />
                </header>

                <div class="settings-body">
                    <q-banner v-if="error" rounded class="bg-red-10 text-red-2 q-mb-md" role="alert">{{ error }}</q-banner>
                    <section class="settings-section" aria-labelledby="appearance-title">
                        <div class="section-heading">
                            <q-icon name="palette" size="18px" />
                            <h3 id="appearance-title">Appearance</h3>
                            <span class="section-note">Live accent preview</span>
                        </div>
                        <div class="accent-setting">
                            <div class="setting-copy">
                                <label for="workspace-accent">Accent color</label>
                                <p>A little color across your workspace.</p>
                            </div>
                            <div class="accent-controls">
                                <div class="theme-color-inputs">
                                    <input type="color" aria-label="Pick theme accent color" :value="isAccentColor(draft.accentColor) ? draft.accentColor : defaultResultSettings.accentColor"
                                        :disabled="loading || saving" @input="draft.accentColor = ($event.target as HTMLInputElement).value" />
                                    <q-input v-model="draft.accentColor" for="workspace-accent" aria-label="Accent color hex code" outlined dense hide-bottom-space
                                        :disable="loading || saving" :rules="[value => isAccentColor(value) || 'Use a six-digit hex color']" />
                                </div>
                                <div class="theme-presets" role="group" aria-label="Accent presets">
                                    <button v-for="preset in accentPresets" :key="preset.color" type="button" :title="preset.name"
                                        :aria-label="`Use ${preset.name} theme`" :aria-pressed="draft.accentColor.toLowerCase() === preset.color"
                                        :style="{ backgroundColor: preset.color }" :disabled="loading || saving" @click="draft.accentColor = preset.color">
                                        <q-icon v-if="draft.accentColor.toLowerCase() === preset.color" name="check" size="15px" />
                                    </button>
                                </div>
                            </div>
                        </div>
                        <div class="font-settings settings-grid">
                            <q-select v-model="draft.fontSize" :options="[10, 11, 12, 13, 14, 16, 18]"
                                label="Workspace & grid" suffix="px" outlined dense hide-bottom-space :disable="loading || saving" />
                            <q-select v-model="draft.sidebarFontSize" :options="[10, 11, 12, 13, 14, 16, 18, 20, 22, 24]"
                                label="Sidebar" suffix="px" outlined dense hide-bottom-space :disable="loading || saving" />
                            <q-select v-model="draft.consoleFontSize" :options="[10, 11, 12, 13, 14, 16, 18, 20, 22, 24]"
                                label="SQL editor & DDL" suffix="px" outlined dense hide-bottom-space :disable="loading || saving" />
                        </div>
                        <p class="section-help">Font sizes apply after saving. Cancel restores your previous accent.</p>
                    </section>

                    <section class="settings-section" aria-labelledby="query-settings-title">
                        <div class="section-heading">
                            <q-icon name="speed" size="18px" />
                            <h3 id="query-settings-title">Query & data</h3>
                        </div>
                        <div class="settings-grid data-settings">
                            <div>
                                <q-input v-model.number="draft.rowsPerPage" type="number" label="Rows per page" outlined dense hide-bottom-space
                                    :min="1" :disable="loading || saving"
                                    :rules="[value => Number.isSafeInteger(value) && value >= 1 || 'Enter a positive whole number']" />
                                <p class="field-help">No upper limit. Larger pages need more time and memory.</p>
                            </div>
                            <div>
                                <q-input v-model.number="draft.queryTimeoutSeconds" type="number" label="Query timeout" suffix="sec" outlined dense hide-bottom-space
                                    :min="0" :max="86400" :disable="loading || saving"
                                    :rules="[value => Number.isInteger(value) && value >= 0 && value <= 86400 || 'Enter a whole number from 0 to 86400']" />
                                <p class="field-help">Per query, page load, or script. Use 0 for no timeout.</p>
                            </div>
                            <div class="import-setting">
                                <q-input v-model.number="draft.importLimitMiB" type="number" clearable label="Import size limit" suffix="MiB" outlined dense hide-bottom-space
                                    :min="0" :disable="loading || saving"
                                    :rules="[value => value == null || value === '' || Number.isSafeInteger(value) && value >= 0 || 'Enter a nonnegative whole number']" />
                                <p class="field-help">SQL scripts and data imports. Empty or 0 means unlimited.</p>
                            </div>
                        </div>
                        <p class="section-help">Applies to your next query, refresh, or import.</p>
                    </section>

                    <section class="settings-section display-section" aria-labelledby="display-settings-title">
                        <div class="section-heading">
                            <q-icon name="table_rows" size="18px" />
                            <h3 id="display-settings-title">Result display</h3>
                        </div>
                        <div class="toggle-row">
                            <div class="setting-copy"><div id="striped-rows-label">Alternating row colors</div><p>Keep your place across wide tables.</p></div>
                            <q-toggle v-model="draft.stripedRows" aria-labelledby="striped-rows-label" color="primary" :disable="loading || saving" />
                        </div>
                        <div class="toggle-row">
                            <div class="setting-copy"><div id="show-nulls-label">Show NULL values</div><p>Distinguish missing values from empty cells.</p></div>
                            <q-toggle v-model="draft.showNulls" aria-labelledby="show-nulls-label" color="primary" :disable="loading || saving" />
                        </div>
                    </section>
                    <p class="settings-scope"><q-icon name="sync" size="14px" /> Saved across all connections and app restarts.</p>
                </div>

                <footer class="settings-footer">
                    <q-btn flat no-caps icon="restart_alt" label="Restore defaults" class="restore-button" :disable="loading || saving"
                        @click="draft = { ...defaultResultSettings }" />
                    <div class="footer-actions">
                        <q-btn flat no-caps label="Cancel" :disable="saving" v-close-popup />
                        <q-btn unelevated no-caps type="submit" color="primary" text-color="black" label="Save changes"
                            :loading="saving" :disable="loading" />
                    </div>
                </footer>
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
.result-settings-card.q-card {
    width: 720px;
    max-width: calc(100vw - 32px);
    max-height: calc(100dvh - 48px);
    border: 1px solid var(--border-default);
    border-radius: 14px;
    background: var(--surface-base);
    color: var(--text-primary);
    box-shadow: 0 24px 80px #0005;
    overflow: hidden;
}
.settings-form { display: flex; flex-direction: column; max-height: inherit; position: relative; }
.settings-header { display: flex; align-items: center; gap: 14px; padding: 22px 26px; background: var(--surface-panel); border-bottom: 1px solid var(--border-default); flex-shrink: 0; }
.settings-heading-icon { display: grid; place-items: center; width: 44px; height: 44px; border-radius: 12px; background: var(--accent-tint); color: var(--accent-color); flex-shrink: 0; }
.settings-heading { flex: 1; min-width: 0; }
h2, h3, p { margin: 0; }
h2 { font-size: 19px; line-height: 1.4; font-weight: 650; letter-spacing: -.3px; }
.settings-heading p { color: var(--text-muted); font-size: 12px; margin-top: 3px; }
.settings-body { overflow-y: auto; min-height: 0; padding: 22px 26px 16px; }
.settings-section { padding: 18px; margin-bottom: 16px; border: 1px solid var(--border-default); border-radius: 10px; background: var(--surface-panel); }
.section-heading { display: flex; align-items: center; gap: 9px; margin-bottom: 20px; }
.section-heading > .q-icon { color: var(--accent-color); }
h3 { font-size: 13px; font-weight: 650; }
.section-note { margin-left: auto; color: var(--text-muted); font-size: 11px; }
.accent-setting { display: flex; justify-content: space-between; align-items: center; gap: 20px; margin-bottom: 22px; }
.setting-copy { font-size: 12px; font-weight: 500; }
.setting-copy p, .field-help, .section-help { font-size: 11px; line-height: 1.55; color: var(--text-muted); font-weight: 400; }
.setting-copy p { margin-top: 4px; }
.accent-controls { width: 220px; flex-shrink: 0; }
.theme-color-inputs { display: flex; align-items: flex-start; gap: 8px; }
.theme-color-inputs input[type="color"] { width: 40px; height: 40px; flex-shrink: 0; padding: 4px; border: 1px solid var(--border-default); border-radius: 7px; background: var(--surface-data); cursor: pointer; }
.theme-color-inputs .q-field { flex: 1; min-width: 0; font-family: monospace; }
.theme-presets { display: flex; justify-content: space-between; gap: 8px; margin-top: 12px; padding: 0 3px; }
.theme-presets button { display: grid; place-items: center; width: 24px; height: 24px; padding: 0; color: #20262b; border: 2px solid transparent; border-radius: 50%; cursor: pointer; transition: transform .15s; }
.theme-presets button:hover { transform: translateY(-2px); }
.theme-presets button[aria-pressed="true"] { outline: 2px solid var(--text-primary); outline-offset: 3px; }
.theme-presets button:focus-visible, .theme-color-inputs input:focus-visible { outline: 2px solid var(--text-primary); outline-offset: 3px; }
.theme-presets button:disabled { opacity: .5; cursor: default; }
.settings-grid { display: grid; gap: 16px; }
.font-settings { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.data-settings { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.import-setting { grid-column: 1 / -1; }
.field-help { margin-top: 7px; }
.section-help { margin-top: 14px; }
.settings-section :deep(.q-field__control) { background: var(--surface-data); border-radius: 7px; }
.display-section .section-heading { margin-bottom: 8px; }
.toggle-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 10px 0; }
.toggle-row + .toggle-row { border-top: 1px solid var(--border-default); }
.settings-scope { display: flex; align-items: center; justify-content: center; gap: 7px; color: var(--text-muted); font-size: 11px; }
.settings-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 16px 22px; background: var(--surface-panel); border-top: 1px solid var(--border-default); flex-shrink: 0; }
.restore-button { color: var(--text-muted); }
.footer-actions { display: flex; gap: 8px; }
.footer-actions .q-btn { min-height: 36px; padding: 0 16px; border-radius: 7px; }
.pagination-button { width: 24px; height: 24px; min-height: 24px; padding: 0; border: 1px solid var(--border-default); background: var(--surface-data); color: var(--text-muted); }
.pagination-button:hover { color: var(--accent-color); background: var(--border-default); }
@media (max-width: 600px) {
    .settings-header { padding: 18px; }
    .settings-body { padding: 16px; }
    .settings-section { padding: 16px; }
    .accent-setting { align-items: flex-start; flex-direction: column; gap: 14px; }
    .accent-controls { width: 100%; }
    .theme-presets { justify-content: flex-start; gap: 14px; }
    .font-settings, .data-settings { grid-template-columns: 1fr; }
    .settings-footer { padding: 12px; flex-wrap: wrap; }
    .footer-actions { margin-left: auto; }
    .section-note { font-size: 10px; }
}
</style>
