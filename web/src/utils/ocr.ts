import type { RowDraft } from './bp'

/** Three readings from a monitor photo. Null means that field was not recognized. */
export interface OcrReading {
  systolic: number | null
  diastolic: number | null
  pulse: number | null
}

export function ocrHasValue(reading: OcrReading): boolean {
  return reading.systolic !== null || reading.diastolic !== null || reading.pulse !== null
}

export function ocrToRow(reading: OcrReading): RowDraft {
  return {
    systolic: reading.systolic === null ? '' : String(reading.systolic),
    diastolic: reading.diastolic === null ? '' : String(reading.diastolic),
    pulse: reading.pulse === null ? '' : String(reading.pulse),
  }
}

/** Replace one measurement. An empty list gains one row. Other rows stay as they were. */
export function applyOcrToRow(rows: readonly RowDraft[], index: number, reading: OcrReading): RowDraft[] {
  const target = ocrToRow(reading)
  if (rows.length === 0) return [target]
  if (index < 0 || index >= rows.length) return rows.map((row) => ({ ...row }))
  const next = rows.map((row) => ({ ...row }))
  next[index] = target
  return next
}

/** Replace the last measurement. An empty list gains one row. Earlier rows stay as they were. */
export function applyOcrToLastRow(rows: readonly RowDraft[], reading: OcrReading): RowDraft[] {
  return applyOcrToRow(rows, rows.length - 1, reading)
}
