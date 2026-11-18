import type { Period } from '../api/types'
import { t } from '../i18n/zh'

/** Morning, afternoon, then evening. Matches the server's Periods order. */
export const PERIODS: Period[] = ['am', 'pm', 'evening']

export function periodLabel(period: Period): string {
  switch (period) {
    case 'am':
      return t.bp.am
    case 'pm':
      return t.bp.pm
    case 'evening':
      return t.bp.evening
  }
}
