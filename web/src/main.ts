import { createApp } from 'vue'
import { createPinia } from 'pinia'
import 'vant/es/toast/style'
import 'vant/es/dialog/style'
import 'vant/es/notify/style'
import './styles/global.css'
import App from './App.vue'
import { router } from './router'
import { setUnauthorizedHandler } from './api/client'
import { useSessionStore } from './stores/session'
import { useProfilesStore } from './stores/profiles'
import { useDraftsStore } from './stores/drafts'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)

setUnauthorizedHandler(() => {
  useSessionStore(pinia).clear()
  useProfilesStore(pinia).reset()
  useDraftsStore(pinia).reset()
  if (router.currentRoute.value.path !== '/login') {
    router.replace('/login')
  }
})

app.mount('#app')
