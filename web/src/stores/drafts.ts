import { defineStore } from 'pinia'
import { ref } from 'vue'
import { isEmptyRow, type RowDraft } from '../utils/bp'

const STORAGE_KEY = 'bp.drafts'
/** A morning draft should not reappear as the evening reading. */
export const DRAFT_MAX_AGE_MS = 6 * 60 * 60 * 1000

export interface Draft {
  rows: RowDraft[]
  note: string
  /** ISO time picked by the user, or null for "now". */
  time: string | null
  updatedAt: number
}

export function isEmptyDraft(d: Pick<Draft, 'rows' | 'note'>): boolean {
  return d.rows.every(isEmptyRow) && !d.note.trim()
}

export function isFresh(d: Draft, now: number): boolean {
  return now - d.updatedAt < DRAFT_MAX_AGE_MS
}

/** Unsaved form input per profile, so switching person never throws typed numbers away. */
export const useDraftsStore = defineStore('drafts', () => {
  const byProfile = ref<Record<number, Draft>>(readAll())

  function get(profileId: number): Draft | null {
    const d = byProfile.value[profileId]
    if (!d) return null
    if (!isFresh(d, Date.now())) {
      clear(profileId)
      return null
    }
    return d
  }

  function has(profileId: number): boolean {
    return get(profileId) !== null
  }

  function put(profileId: number, d: Omit<Draft, 'updatedAt'>) {
    if (isEmptyDraft(d)) {
      clear(profileId)
      return
    }
    byProfile.value[profileId] = { ...d, rows: d.rows.map((r) => ({ ...r })), updatedAt: Date.now() }
    persist()
  }

  function clear(profileId: number) {
    if (!(profileId in byProfile.value)) return
    delete byProfile.value[profileId]
    persist()
  }

  function reset() {
    byProfile.value = {}
    sessionStorage.removeItem(STORAGE_KEY)
  }

  function persist() {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(byProfile.value))
  }

  return { get, has, put, clear, reset }
})

function readAll(): Record<number, Draft> {
  try {
    return JSON.parse(sessionStorage.getItem(STORAGE_KEY) ?? '{}') as Record<number, Draft>
  } catch {
    return {}
  }
}
