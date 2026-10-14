import { defineStore } from "pinia";
import { ref } from "vue";
import { DatabaseService } from "@bindings/db-lens/internal/app";
import type { ResultSettings } from "@bindings/db-lens/internal/types";

export const defaultResultSettings: ResultSettings = {
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
    async function load() {
        settings.value = await DatabaseService.GetResultSettings();
    }
    async function save(value: ResultSettings) {
        await DatabaseService.SaveResultSettings(value);
        settings.value = { ...value };
    }
    return { settings, load, save };
});
