import { createApp, watchEffect } from 'vue'
import App from './App.vue'
import {createPinia} from 'pinia'
import { Quasar, Dark, Notify, Dialog } from "quasar"

import "quasar/src/css/index.sass"
import "@quasar/extras/material-icons/material-icons.css"
import "./style.css"
import router from './router'
import { useResultSettingsStore } from './stores/resultSettingsStore'
import { accentTheme } from './utils/accentTheme'
const app = createApp(App)
const pinia = createPinia()

app.use(Quasar, {
  config: {
    notify: { position: 'top-right' },
  },
  plugins: {
    Dark,
    Notify,
    Dialog,
  }
})

app.use(router)
app.use(pinia)

Dark.set(true)
document.documentElement.classList.add("dark")
const resultSettings = useResultSettingsStore(pinia)
watchEffect(() => {
  const style = document.documentElement.style
  for (const [name, value] of Object.entries(accentTheme(resultSettings.accentPreview ?? resultSettings.settings.accentColor))) style.setProperty(name, value)
  style.setProperty('--ui-font-size', `${resultSettings.settings.fontSize}px`)
  style.setProperty('--grid-font-size', `${resultSettings.settings.fontSize}px`)
  style.setProperty('--sidebar-font-size', `${resultSettings.settings.sidebarFontSize}px`)
  style.setProperty('--console-font-size', `${resultSettings.settings.consoleFontSize}px`)
})

resultSettings.load().catch(() => {
  Notify.create({ message: "Could not load result settings. Using defaults.", color: "negative" })
}).finally(() => app.mount("#app"))
