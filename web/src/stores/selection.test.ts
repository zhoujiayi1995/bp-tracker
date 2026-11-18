import { describe, expect, it } from 'vitest'
import { IDLE_RESET_MS, resolveProfileId } from './profiles'
import { DRAFT_MAX_AGE_MS, isEmptyDraft, isFresh } from './drafts'

describe('resolveProfileId', () => {
  const now = 1_000_000_000
  const ids = [1, 2, 3]

  it('keeps a recent selection', () => {
    expect(resolveProfileId({ id: 2, at: now - 60_000 }, now, ids, 1)).toBe(2)
  })

  it('falls back to the own profile after a long pause', () => {
    expect(resolveProfileId({ id: 2, at: now - IDLE_RESET_MS }, now, ids, 1)).toBe(1)
  })

  it('falls back when nothing is stored or the stored profile is gone', () => {
    expect(resolveProfileId(null, now, ids, 1)).toBe(1)
    expect(resolveProfileId({ id: 9, at: now }, now, ids, 1)).toBe(1)
  })

  it('uses the first visible profile when the own profile is unknown', () => {
    expect(resolveProfileId(null, now, ids, null)).toBe(1)
    expect(resolveProfileId(null, now, [], null)).toBeNull()
  })
})

describe('drafts', () => {
  const row = { systolic: '', diastolic: '', pulse: '' }

  it('treats blank rows and whitespace notes as empty', () => {
    expect(isEmptyDraft({ rows: [row, row], note: '  ' })).toBe(true)
    expect(isEmptyDraft({ rows: [{ ...row, systolic: '1' }], note: '' })).toBe(false)
    expect(isEmptyDraft({ rows: [row], note: '吃了药' })).toBe(false)
  })

  it('expires drafts after the maximum age', () => {
    const d = { rows: [row], note: 'x', time: null, updatedAt: 0 }
    expect(isFresh(d, DRAFT_MAX_AGE_MS - 1)).toBe(true)
    expect(isFresh(d, DRAFT_MAX_AGE_MS)).toBe(false)
  })
})
