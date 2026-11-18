import { t } from '../i18n/zh'

const pad = (n: number) => String(n).padStart(2, '0')

export function toYMD(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** Inclusive range of `days` calendar days ending today, as YYYY-MM-DD strings. */
export function lastDays(days: number, today = new Date()): { from: string; to: string } {
  const from = new Date(today.getFullYear(), today.getMonth(), today.getDate() - (days - 1))
  return { from: toYMD(from), to: toYMD(today) }
}

export function formatTime(d: Date): string {
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** "Today", "Yesterday", or month/day plus weekday for older dates. */
export function formatDay(d: Date, today = new Date()): string {
  const startOf = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime()
  const diffDays = Math.round((startOf(today) - startOf(d)) / 86_400_000)
  if (diffDays === 0) return t.fmt.today
  if (diffDays === 1) return t.fmt.yesterday
  return `${t.fmt.monthDay(d.getMonth() + 1, d.getDate())} ${t.fmt.weekdays[d.getDay()]}`
}

export function formatDateTime(d: Date, today = new Date()): string {
  return `${formatDay(d, today)} ${formatTime(d)}`
}

/** "11-15" from "2026-11-15". */
export function shortDate(ymd: string): string {
  return ymd.slice(5)
}
