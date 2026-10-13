<template>
    <header
        class="h-12 bg-[#353535] border-b border-[#484848] flex items-center justify-between px-4 z-10"
    >
        <!-- Left -->
        <div class="flex items-center gap-3">
            <q-icon
                name="dns"
                size="20px"
                class="text-amber-400"
            />

            <span
                class="font-bold text-white tracking-wide text-sm"
            >
                DB Viewer
            </span>

            <q-badge
                :color="connected ? 'green-10' : 'grey-9'"
                :text-color="connected ? 'green-4' : 'grey-5'"
                class="text-xs font-medium flex items-center gap-1.5"
                role="status"
                aria-live="polite"
            >
                <q-icon name="circle" size="8px" :color="connected ? 'green-4' : 'grey-5'" aria-hidden="true" />
                {{ connected ? 'Connected' : 'Not connected' }}
            </q-badge>
        </div>

        <!-- Right -->
        <div class="flex items-center gap-2">
            <ResultSettings />

            <q-btn
                flat
                dense
                round
                icon="logout"
                size="sm"
                class="text-amber-400 hover:bg-amber-500/10"
                @click="handleDisconnect"
            />
        </div>
    </header>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useConnectionStore } from "@/stores/connectionStore";
import { DbService } from "@bindings/db-lens/internal/app";
import ResultSettings from "@/components/workspace/resultGrid/ResultSettings.vue";

const connectionStore = useConnectionStore();
const connected = ref(false);

watch(() => connectionStore.activeConnection?.id, (id, _previous, onCleanup) => {
    let stopped = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    connected.value = id !== undefined;
    onCleanup(() => {
        stopped = true;
        clearTimeout(timer);
    });
    if (id === undefined) return;

    async function checkConnection() {
        let reachable = false;
        try {
            reachable = await DbService.PingConnection(String(id));
        } catch {
            reachable = false;
        }
        if (stopped) return;
        connected.value = reachable;
        timer = setTimeout(checkConnection, 5000);
    }
    void checkConnection();
}, { immediate: true });

const emit = defineEmits<{
    disconnect: [];
}>();

function handleDisconnect() {
    emit("disconnect");
}

</script>
