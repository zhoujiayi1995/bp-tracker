import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api } from '../api'
import type { Profile } from '../api/types'
import { useSessionStore } from './session'

const STORAGE_KEY = 'bp.currentProfile'
export const IDLE_RESET_MS = 2 * 60 * 60 * 1000

export interface StoredSelection {
  id: number
  at: number
}

/**
 * The viewed person is remembered only within one app session and falls back to the user's own
 * profile after a long pause, so a morning entry never lands on whoever was viewed yesterday.
 */
export function resolveProfileId(
  stored: StoredSelection | null,
  now: number,
  available: number[],
  selfId: number | null,
): number | null {
  if (stored && now - stored.at < IDLE_RESET_MS && available.includes(stored.id)) return stored.id
  if (selfId !== null && available.includes(selfId)) return selfId
  return available[0] ?? null
}

export const useProfilesStore = defineStore('profiles', () => {
  const list = ref<Profile[]>([])
  const currentId = ref<number | null>(null)
  const loaded = ref(false)

  const current = computed(() => list.value.find((p) => p.id === currentId.value) ?? null)
  const self = computed(() => list.value.find((p) => p.isSelf) ?? null)

  function resolve(): number | null {
    const session = useSessionStore()
    return resolveProfileId(
      readStored(),
      Date.now(),
      list.value.map((p) => p.id),
      session.me?.selfProfileId ?? null,
    )
  }

  async function load() {
    list.value = await api.profiles()
    loaded.value = true
    select(resolve())
  }

  async function ensureLoaded() {
    if (!loaded.value) await load()
  }

  function select(id: number | null) {
    currentId.value = id
    writeStored(id === null ? null : { id, at: Date.now() })
  }

  /** Call when the app goes to the background; the idle clock starts from here. */
  function markActive() {
    if (currentId.value !== null) writeStored({ id: currentId.value, at: Date.now() })
  }

  /** Call when the app comes back; switches to the user's own profile after a long pause. */
  function resumeAfterIdle() {
    if (!loaded.value) return
    const next = resolve()
    if (next !== currentId.value) select(next)
  }

  function reset() {
    list.value = []
    loaded.value = false
    select(null)
  }

  return { list, current, currentId, self, loaded, load, ensureLoaded, select, markActive, resumeAfterIdle, reset }
})

function readStored(): StoredSelection | null {
  try {
    const v = JSON.parse(sessionStorage.getItem(STORAGE_KEY) ?? 'null') as StoredSelection | null
    return v && Number.isInteger(v.id) && typeof v.at === 'number' ? v : null
  } catch {
    return null
  }
}

function writeStored(v: StoredSelection | null) {
  if (v === null) sessionStorage.removeItem(STORAGE_KEY)
  else sessionStorage.setItem(STORAGE_KEY, JSON.stringify(v))
}
