import { onMounted, onUnmounted, ref } from 'vue'
import { toYMD } from '../utils/date'

export function useToday() {
  const today = ref(new Date())
  let timer: number | undefined

  function refresh() {
    const now = new Date()
    if (toYMD(now) !== toYMD(today.value)) today.value = now

    window.clearTimeout(timer)
    const midnight = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1)
    timer = window.setTimeout(refresh, midnight.getTime() - now.getTime())
  }

  function onVisibility() {
    if (document.visibilityState === 'visible') refresh()
  }

  onMounted(() => {
    refresh()
    document.addEventListener('visibilitychange', onVisibility)
    window.addEventListener('pageshow', refresh)
  })
  onUnmounted(() => {
    window.clearTimeout(timer)
    document.removeEventListener('visibilitychange', onVisibility)
    window.removeEventListener('pageshow', refresh)
  })

  return today
}
