export type Level = 'low' | 'normal' | 'elevated' | 'high'
/** Level plus the grade-3 band, which warrants an explicit "seek care" message. */
export type Advice = Level | 'severe'

// Must match the server-side limits in internal/reading.
export const LIMITS = {
  systolic: { min: 60, max: 260 },
  diastolic: { min: 30, max: 160 },
  pulse: { min: 30, max: 200 },
} as const

export type Field = keyof typeof LIMITS

/** Raw text of one measurement as typed into the form. */
export type RowDraft = Record<Field, string>

export interface ItemDraft {
  systolic: number | null
  diastolic: number | null
  pulse: number | null
}

export interface Averaged {
  systolic: number
  diastolic: number
  pulse: number | null
}

/**
 * Classification follows the Chinese hypertension guideline thresholds. High wins over low so a
 * wide pulse pressure such as 150/55 is reported as high.
 */
export function classify(systolic: number, diastolic: number): Level {
  if (systolic >= 140 || diastolic >= 90) return 'high'
  if (systolic < 90 || diastolic < 60) return 'low'
  if (systolic >= 120 || diastolic >= 80) return 'elevated'
  return 'normal'
}

export function isSevere(systolic: number, diastolic: number): boolean {
  return systolic >= 180 || diastolic >= 110
}

export function adviceFor(systolic: number, diastolic: number): Advice {
  return isSevere(systolic, diastolic) ? 'severe' : classify(systolic, diastolic)
}

/** Averages complete items (both pressures present). Rounds half up, same as the server. */
export function averageItems(items: ItemDraft[]): Averaged | null {
  const complete = items.filter((it) => it.systolic !== null && it.diastolic !== null)
  if (complete.length === 0) return null
  const sys = complete.reduce((s, it) => s + (it.systolic as number), 0)
  const dia = complete.reduce((s, it) => s + (it.diastolic as number), 0)
  const pulses = complete.map((it) => it.pulse).filter((p): p is number => p !== null)
  return {
    systolic: Math.round(sys / complete.length),
    diastolic: Math.round(dia / complete.length),
    pulse: pulses.length ? Math.round(pulses.reduce((s, p) => s + p, 0) / pulses.length) : null,
  }
}

export type ItemProblem =
  | { field: Field; reason: 'missing' | 'range' }
  | { field: 'systolic'; reason: 'not_above_diastolic' }

export function checkItem(it: ItemDraft): ItemProblem | null {
  if (it.systolic === null) return { field: 'systolic', reason: 'missing' }
  if (it.diastolic === null) return { field: 'diastolic', reason: 'missing' }
  const systolic = it.systolic
  const diastolic = it.diastolic
  if (systolic < LIMITS.systolic.min || systolic > LIMITS.systolic.max) return { field: 'systolic', reason: 'range' }
  if (diastolic < LIMITS.diastolic.min || diastolic > LIMITS.diastolic.max) return { field: 'diastolic', reason: 'range' }
  if (it.pulse === null) return { field: 'pulse', reason: 'missing' }
  if (it.pulse < LIMITS.pulse.min || it.pulse > LIMITS.pulse.max) return { field: 'pulse', reason: 'range' }
  if (it.systolic <= it.diastolic) return { field: 'systolic', reason: 'not_above_diastolic' }
  return null
}

/** Valid but rare values that are more likely a typo than a reading; asked about before saving. */
export function isUnusual(a: Averaged): boolean {
  return (
    a.systolic >= 200 ||
    a.diastolic >= 130 ||
    a.systolic < 80 ||
    a.diastolic < 45 ||
    (a.pulse !== null && (a.pulse > 130 || a.pulse < 45))
  )
}

/**
 * Decides when to jump to the next input while typing. Three digits are always complete;
 * two digits are complete when no valid three-digit value could start with them
 * (every valid three-digit value starts with 1 or 2).
 */
export function shouldAdvance(value: string): boolean {
  if (value.length >= 3) return true
  return value.length === 2 && Number(value[0]) >= 3
}

export function parseDigits(value: string): number | null {
  const digits = value.replace(/\D/g, '')
  return digits === '' ? null : Number(digits)
}

export const emptyRow = (): RowDraft => ({ systolic: '', diastolic: '', pulse: '' })

export function isEmptyRow(r: RowDraft): boolean {
  return !r.systolic && !r.diastolic && !r.pulse
}
