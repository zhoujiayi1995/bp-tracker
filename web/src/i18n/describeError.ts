import { ApiError, NetworkError } from '../api/client'
import type { ItemProblem } from '../utils/bp'
import { t } from './zh'

/**
 * Message for a client-side check failure on the index-th (0-based) measurement. The ordinal is
 * left out when there is only one measurement, where "the 1st" would only confuse.
 */
export function describeItemProblem(index: number, problem: ItemProblem, total: number): string {
  const field = t.bp[problem.field]
  if (problem.reason === 'missing') {
    return total > 1 ? t.record.missingValue(index + 1, field) : `${field}还没填`
  }
  const label = total > 1 ? t.errors.nth(index + 1) + field : field
  return label + t.errors.fieldSeparator + (t.errors.reasons[problem.reason] ?? t.errors.unknown)
}

const itemField = /^items\[(\d+)\]\.(\w+)$/

/** Turns any thrown value into a sentence suitable for a toast. */
export function describeError(err: unknown): string {
  if (err instanceof NetworkError) return t.errors.network
  if (!(err instanceof ApiError)) return t.errors.unknown

  if (err.code === 'invalid_input') {
    return describeField(err.field ?? '', err.reason ?? '')
  }
  const msg = t.errors.codes[err.code]
  if (typeof msg === 'function') return msg(err.retryAfter ?? 60)
  return msg ?? t.errors.unknown
}

function describeField(field: string, reason: string): string {
  let label = t.errors.fields[field] ?? field
  const m = itemField.exec(field)
  if (m) {
    label = t.errors.nth(Number(m[1]) + 1) + (t.errors.fields[m[2]] ?? m[2])
  }
  const why = t.errors.reasons[reason] ?? t.errors.unknown
  return label + t.errors.fieldSeparator + why
}
