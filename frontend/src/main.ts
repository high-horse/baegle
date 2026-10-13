import { createApp } from 'vue'
import App from './App.vue'
import {createPinia} from 'pinia'
import { Quasar, Dark, Notify, Dialog } from "quasar"

import "quasar/src/css/index.sass"
import "@quasar/extras/material-icons/material-icons.css"
import "./style.css"
import router from './router'
import { useResultSettingsStore } from './stores/resultSettingsStore'
const app = createApp(App)
const pinia = createPinia()

app.use(Quasar, {
  plugins: {
    Dark,
    Notify,
    Dialog,
  }
})

app.use(router)
app.use(pinia)

const savedPreference = localStorage.getItem("dark-mode")
const isDark = savedPreference === null ? true : savedPreference === "true"

Dark.set(true)

// CRITICAL FOR TAILWIND: Sync the HTML element class on startup
document.documentElement.classList.toggle("dark", isDark)

useResultSettingsStore(pinia).load().catch(() => {
  Notify.create({ message: "Could not load result settings. Using defaults.", color: "negative" })
}).finally(() => app.mount("#app"))
