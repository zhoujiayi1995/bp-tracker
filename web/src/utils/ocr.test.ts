import { describe, expect, it } from 'vitest'
import { applyOcrToLastRow, applyOcrToRow, ocrHasValue } from './ocr'

describe('ocrHasValue', () => {
  it('opens the review only when a number was read', () => {
    expect(ocrHasValue({ systolic: null, diastolic: null, pulse: null })).toBe(false)
    expect(ocrHasValue({ systolic: null, diastolic: null, pulse: 70 })).toBe(true)
    expect(ocrHasValue({ systolic: 118, diastolic: null, pulse: null })).toBe(true)
  })
})

describe('applyOcrToLastRow', () => {
  it('overwrites the last row and keeps an empty pulse empty', () => {
    const rows = [
      { systolic: '120', diastolic: '80', pulse: '70' },
      { systolic: '118', diastolic: '63', pulse: '72' },
    ]
    const next = applyOcrToLastRow(rows, { systolic: 130, diastolic: 85, pulse: null })
    expect(next[0]).toEqual(rows[0])
    expect(next[1]).toEqual({ systolic: '130', diastolic: '85', pulse: '' })
    expect(rows[1].pulse).toBe('72')
  })

  it('adds a row when the form has none', () => {
    expect(applyOcrToLastRow([], { systolic: 118, diastolic: 63, pulse: 72 })).toEqual([
      { systolic: '118', diastolic: '63', pulse: '72' },
    ])
  })
})

describe('applyOcrToRow', () => {
  it('overwrites the chosen row and leaves the others', () => {
    const rows = [
      { systolic: '120', diastolic: '80', pulse: '70' },
      { systolic: '118', diastolic: '63', pulse: '72' },
    ]
    const next = applyOcrToRow(rows, 0, { systolic: 130, diastolic: 85, pulse: null })
    expect(next[0]).toEqual({ systolic: '130', diastolic: '85', pulse: '' })
    expect(next[1]).toEqual(rows[1])
    expect(rows[0].systolic).toBe('120')
  })
})
