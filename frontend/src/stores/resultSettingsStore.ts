import { defineStore } from "pinia";
import { ref } from "vue";
import { DatabaseService } from "@bindings/db-lens/internal/app";
import type { ResultSettings } from "@bindings/db-lens/internal/types";

export const defaultResultSettings: ResultSettings = {
    accentColor: '#59cbb7',
    queryTimeoutSeconds: 60,
    rowsPerPage: 100,
    fontSize: 11,
    sidebarFontSize: 12,
    consoleFontSize: 12,
    importLimitMiB: 0,
    stripedRows: true,
    showNulls: false,
};

export const useResultSettingsStore = defineStore("resultSettings", () => {
    const settings = ref<ResultSettings>({ ...defaultResultSettings });
    const accentPreview = ref<string | null>(null);
    async function load() {
        settings.value = { ...defaultResultSettings, ...await DatabaseService.GetResultSettings() };
    }
    async function save(value: ResultSettings) {
        await DatabaseService.SaveResultSettings(value);
        settings.value = { ...value };
    }
    return { settings, accentPreview, load, save };
});
