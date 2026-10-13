<template>
    <q-btn flat dense type="button" class="pagination-button" title="Result grid and text settings"
        aria-label="Result grid and text settings" @click="openSettings">
        <q-icon name="settings" size="16px" />
    </q-btn>
    <q-dialog v-model="visible" :persistent="saving">
        <q-card class="result-settings-card">
            <q-form @submit.prevent="save">
                <q-card-section>
                    <div class="text-h6">Result grid and text settings</div>
                    <div class="text-caption text-grey-5">Shared across all connections and saved for next time.</div>
                </q-card-section>
                <q-card-section class="q-pt-none">
                    <q-banner v-if="error" class="bg-red-10 text-red-2 q-mb-md" role="alert">{{ error }}</q-banner>
                    <q-input v-model.number="draft.rowsPerPage" type="number" label="Rows per page" outlined dense
                        :min="1" :max="500" :disable="loading || saving"
                        :rules="[value => Number.isInteger(value) && value >= 1 && value <= 500 || 'Enter a whole number from 1 to 500']"
                        hint="Applies on the next query run or refresh." />
                    <q-select v-model="draft.fontSize" :options="[10, 11, 12, 13, 14, 16, 18]"
                        label="Grid font size (px)" outlined dense class="q-mt-md" :disable="loading || saving" />
                    <q-select v-model="draft.sidebarFontSize" :options="[10, 11, 12, 13, 14, 16, 18, 20, 22, 24]"
                        label="Sidebar table list font size (px)" outlined dense class="q-mt-md" :disable="loading || saving" />
                    <q-select v-model="draft.consoleFontSize" :options="[10, 11, 12, 13, 14, 16, 18, 20, 22, 24]"
                        label="Query console font size (px)" outlined dense class="q-mt-md" :disable="loading || saving" />
                    <q-toggle v-model="draft.stripedRows" label="Alternating row colors" color="amber"
                        class="block q-mt-sm" :disable="loading || saving" />
                    <q-toggle v-model="draft.showNulls" label="Show NULL values explicitly" color="amber" class="block"
                        :disable="loading || saving" />
                </q-card-section>
                <q-card-actions align="right" class="q-pa-md">
                    <q-btn flat no-caps label="Restore defaults" :disable="loading || saving"
                        @click="draft = { ...defaultResultSettings }" />
                    <q-space />
                    <q-btn flat no-caps label="Cancel" :disable="saving" v-close-popup />
                    <q-btn unelevated no-caps type="submit" color="amber" text-color="black" label="Save"
                        :loading="saving" :disable="loading" />
                </q-card-actions>
                <q-inner-loading :showing="loading" color="amber" />
            </q-form>
        </q-card>
    </q-dialog>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { defaultResultSettings, useResultSettingsStore } from "@/stores/resultSettingsStore";

const store = useResultSettingsStore();
const visible = ref(false);
const loading = ref(false);
const saving = ref(false);
const error = ref("");
const draft = ref({ ...store.settings });
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
        await store.save(draft.value);
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
    background: #353535;
    color: #d1d5db;
}

.pagination-button {
    width: 24px;
    height: 24px;
    min-height: 24px;
    padding: 0;
    border: 1px solid #484848;
    background: #2d2d2d;
    color: #a0a0a0;
}

.pagination-button:hover {
    color: #fbbf24;
    background: #484848;
}
</style>
