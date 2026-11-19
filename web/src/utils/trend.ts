import type { Period, Trend, TrendBucket, TrendSummary } from '../api/types'
import { adviceFor } from './bp'
import { shortDate } from './date'
import { PERIODS, periodLabel } from './period'
import { t } from '../i18n/zh'

export type ChartMode = 'ampm' | 'daily' | 'raw'

export interface DayAverage {
  date: string
  systolic: number | null
  diastolic: number | null
  pulse: number | null
  count: number
}

export type ChartData =
  | {
      kind: 'time'
      points: { at: number; systolic: number; diastolic: number; pulse: number | null }[]
    }
  | {
      kind: 'category'
      /** Unique category names, also used as tooltip titles. */
      names: string[]
      /** Text under each tick; empty strings hide a tick label. */
      axisLabels: string[]
      systolic: (number | null)[]
      diastolic: (number | null)[]
      pulse: (number | null)[]
    }

type Metric = 'systolic' | 'diastolic' | 'pulse'

function weighted(buckets: TrendBucket[], key: Metric): number | null {
  let sum = 0
  let n = 0
  for (const b of buckets) {
    sum += b[`${key}Sum`]
    n += key === 'pulse' ? b.pulseCount : b.count
  }
  return n ? Math.round(sum / n) : null
}

export function dailyAverages(buckets: TrendBucket[]): DayAverage[] {
  const byDate = new Map<string, TrendBucket[]>()
  for (const b of buckets) {
    const list = byDate.get(b.date)
    if (list) list.push(b)
    else byDate.set(b.date, [b])
  }
  return [...byDate].map(([date, bs]) => ({
    date,
    systolic: weighted(bs, 'systolic'),
    diastolic: weighted(bs, 'diastolic'),
    pulse: weighted(bs, 'pulse'),
    count: bs.reduce((s, b) => s + b.count, 0),
  }))
}

/** Drops whole days before the first measurement so a short history doesn't hug the right edge. */
export function fromFirstMeasuredDay<T extends { date: string; count: number }>(xs: T[]): T[] {
  const first = xs.find((x) => x.count > 0)
  return first ? xs.filter((x) => x.date >= first.date) : []
}

/** Highest versus lowest period systolic average. Null when fewer than two periods have readings. */
export function periodGap(buckets: TrendBucket[]): { higher: Period; lower: Period; gap: number } | null {
  const avgs = PERIODS.map((period) => ({ period, value: weighted(buckets.filter((b) => b.period === period), 'systolic') }))
    .filter((x): x is { period: Period; value: number } => x.value !== null)
  if (avgs.length < 2) return null
  let higher = avgs[0]
  let lower = avgs[0]
  for (const a of avgs.slice(1)) {
    if (a.value > higher.value) higher = a
    if (a.value < lower.value) lower = a
  }
  return { higher: higher.period, lower: lower.period, gap: higher.value - lower.value }
}

const SAME_THRESHOLD = 3
const GAP_THRESHOLD = 5

/** Plain-language reading of the chart, one short sentence per fact. */
export function describeTrend(
  days: number,
  summary: TrendSummary,
  previous: TrendSummary | null,
  buckets: TrendBucket[],
): string[] {
  const { systolic: s, diastolic: d } = summary
  if (summary.count === 0 || s === null || d === null) return []
  const out = [t.trend.insightLevel(days, `${s}/${d}`, t.level[adviceFor(s, d)])]

  if (previous && previous.count > 0 && previous.systolic !== null) {
    const diff = s - previous.systolic
    if (Math.abs(diff) < SAME_THRESHOLD) out.push(t.trend.insightSame(days))
    else if (diff > 0) out.push(t.trend.insightUp(days, diff))
    else out.push(t.trend.insightDown(days, -diff))
  }

  const gap = periodGap(buckets)
  if (gap && gap.gap >= GAP_THRESHOLD) {
    out.push(t.trend.insightPeriodHigher(periodLabel(gap.higher), periodLabel(gap.lower), gap.gap))
  }
  return out
}

export function chartData(mode: ChartMode, ampm: Trend, raw: Trend | null): ChartData {
  if (mode === 'raw' && raw) {
    return {
      kind: 'time',
      points: (raw.points ?? []).map((p) => ({
        at: new Date(p.at).getTime(),
        systolic: p.systolic,
        diastolic: p.diastolic,
        pulse: p.pulse,
      })),
    }
  }
  if (mode === 'daily') {
    const days = fromFirstMeasuredDay(dailyAverages(ampm.buckets ?? []))
    const names = days.map((x) => shortDate(x.date))
    return {
      kind: 'category',
      names,
      axisLabels: names,
      systolic: days.map((x) => x.systolic),
      diastolic: days.map((x) => x.diastolic),
      pulse: days.map((x) => x.pulse),
    }
  }
  const bs = fromFirstMeasuredDay(ampm.buckets ?? [])
  return {
    kind: 'category',
    names: bs.map((b) => `${shortDate(b.date)} ${periodLabel(b.period)}`),
    // Date on the morning slot; each period is named on the second line.
    axisLabels: bs.map((b) =>
      b.period === 'am' ? `${shortDate(b.date)}\n${periodLabel(b.period)}` : `\n${periodLabel(b.period)}`,
    ),
    systolic: bs.map((b) => b.systolic),
    diastolic: bs.map((b) => b.diastolic),
    pulse: bs.map((b) => b.pulse),
  }
}
