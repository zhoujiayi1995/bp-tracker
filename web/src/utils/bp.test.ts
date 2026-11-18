import { describe, expect, it } from 'vitest'
import { adviceFor, averageItems, checkItem, classify, isUnusual, parseDigits, shouldAdvance } from './bp'
import { lastDays } from './date'

describe('classify', () => {
  it.each([
    [119, 79, 'normal'],
    [120, 70, 'elevated'],
    [110, 80, 'elevated'],
    [139, 89, 'elevated'],
    [140, 70, 'high'],
    [130, 90, 'high'],
    [89, 70, 'low'],
    [110, 59, 'low'],
    [150, 55, 'high'],
  ] as const)('%i/%i is %s', (sys, dia, want) => {
    expect(classify(sys, dia)).toBe(want)
  })
})

describe('adviceFor', () => {
  it('separates the grade-3 band from ordinary high readings', () => {
    expect(adviceFor(179, 109)).toBe('high')
    expect(adviceFor(180, 95)).toBe('severe')
    expect(adviceFor(150, 110)).toBe('severe')
    expect(adviceFor(118, 76)).toBe('normal')
  })
})

describe('averageItems', () => {
  it('ignores incomplete items and rounds half up like the server', () => {
    expect(
      averageItems([
        { systolic: 131, diastolic: 80, pulse: 70 },
        { systolic: 132, diastolic: 81, pulse: null },
        { systolic: null, diastolic: 90, pulse: 99 },
      ]),
    ).toEqual({ systolic: 132, diastolic: 81, pulse: 70 })
  })

  it('returns null when nothing is complete', () => {
    expect(averageItems([{ systolic: 120, diastolic: null, pulse: null }])).toBeNull()
  })
})

describe('checkItem', () => {
  it('reports the first problem', () => {
    expect(checkItem({ systolic: null, diastolic: 80, pulse: null })).toEqual({ field: 'systolic', reason: 'missing' })
    expect(checkItem({ systolic: 120, diastolic: null, pulse: null })).toEqual({ field: 'diastolic', reason: 'missing' })
    expect(checkItem({ systolic: 300, diastolic: 80, pulse: null })).toEqual({ field: 'systolic', reason: 'range' })
    expect(checkItem({ systolic: 120, diastolic: 80, pulse: 20 })).toEqual({ field: 'pulse', reason: 'range' })
    expect(checkItem({ systolic: 90, diastolic: 95, pulse: 70 })).toEqual({
      field: 'systolic',
      reason: 'not_above_diastolic',
    })
    expect(checkItem({ systolic: 120, diastolic: 80, pulse: null })).toEqual({ field: 'pulse', reason: 'missing' })
    expect(checkItem({ systolic: 120, diastolic: 80, pulse: 70 })).toBeNull()
  })
})

describe('isUnusual', () => {
  it('flags only values that look like typos', () => {
    expect(isUnusual({ systolic: 205, diastolic: 95, pulse: 70 })).toBe(true)
    expect(isUnusual({ systolic: 75, diastolic: 50, pulse: 70 })).toBe(true)
    expect(isUnusual({ systolic: 125, diastolic: 82, pulse: 140 })).toBe(true)
    expect(isUnusual({ systolic: 185, diastolic: 112, pulse: 70 })).toBe(false)
    expect(isUnusual({ systolic: 125, diastolic: 82, pulse: null })).toBe(false)
  })
})

describe('input helpers', () => {
  it.each([
    ['1', false],
    ['12', false],
    ['25', false],
    ['85', true],
    ['120', true],
  ] as const)('shouldAdvance(%s) = %s', (v, want) => {
    expect(shouldAdvance(v)).toBe(want)
  })

  it('parseDigits strips non-digits', () => {
    expect(parseDigits('1a2b3')).toBe(123)
    expect(parseDigits('')).toBeNull()
  })

  it('lastDays is inclusive of today', () => {
    expect(lastDays(7, new Date(2026, 10, 17))).toEqual({ from: '2026-11-11', to: '2026-11-17' })
    expect(lastDays(1, new Date(2026, 0, 1))).toEqual({ from: '2026-01-01', to: '2026-01-01' })
  })
})
