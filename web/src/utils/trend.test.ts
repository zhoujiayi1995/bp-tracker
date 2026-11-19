import { describe, expect, it } from 'vitest'
import type { Trend, TrendBucket } from '../api/types'
import { chartData, dailyAverages, describeTrend, fromFirstMeasuredDay, periodGap } from './trend'

const bucket = (date: string, period: 'am' | 'pm' | 'evening', sys: number | null, dia: number | null, count: number): TrendBucket => ({
  date,
  period,
  systolic: sys,
  diastolic: dia,
  pulse: null,
  count,
  systolicSum: (sys ?? 0) * count,
  diastolicSum: (dia ?? 0) * count,
  pulseSum: 0,
  pulseCount: 0,
})

const buckets: TrendBucket[] = [
  bucket('2026-11-15', 'am', null, null, 0),
  bucket('2026-11-15', 'pm', null, null, 0),
  bucket('2026-11-15', 'evening', null, null, 0),
  bucket('2026-11-16', 'am', 150, 90, 2),
  bucket('2026-11-16', 'pm', 140, 86, 1),
  bucket('2026-11-16', 'evening', null, null, 0),
  bucket('2026-11-17', 'am', 146, 88, 1),
  bucket('2026-11-17', 'pm', null, null, 0),
  bucket('2026-11-17', 'evening', 130, 80, 1),
]

describe('dailyAverages', () => {
  it('combines period totals by reading count', () => {
    const days = dailyAverages(buckets)
    expect(days.map((d) => d.date)).toEqual(['2026-11-15', '2026-11-16', '2026-11-17'])
    expect(days[0]).toMatchObject({ systolic: null, count: 0 })
    expect(days[1]).toMatchObject({ systolic: 147, diastolic: 89, count: 3 })
    // 17th: am 146/88 once, evening 130/80 once -> 138/84
    expect(days[2]).toMatchObject({ systolic: 138, diastolic: 84, count: 2 })
  })

  it('rounds only once and counts only readings with a pulse', () => {
    const date = '2026-11-17'
    const [day] = dailyAverages([
      { ...bucket(date, 'am', 140, 80, 2), systolicSum: 279, pulse: 60, pulseSum: 60, pulseCount: 1 },
      { ...bucket(date, 'pm', 139, 80, 1), pulse: 80, pulseSum: 80, pulseCount: 1 },
    ])
    expect(day).toMatchObject({ systolic: 139, pulse: 70, count: 3 })
  })
})

describe('fromFirstMeasuredDay', () => {
  it('drops whole leading days without readings and keeps the three periods of later days', () => {
    const trimmed = fromFirstMeasuredDay(buckets)
    expect(trimmed[0]).toMatchObject({ date: '2026-11-16', period: 'am' })
    expect(trimmed).toHaveLength(6)
  })

  it('returns nothing when there are no readings', () => {
    expect(fromFirstMeasuredDay([bucket('2026-11-15', 'am', null, null, 0)])).toEqual([])
  })
})

describe('periodGap', () => {
  it('compares the highest and lowest period systolic averages', () => {
    // am: (150*2 + 146) / 3 = 148.67 -> 149; pm: 140; evening: 130
    expect(periodGap(buckets)).toEqual({ higher: 'am', lower: 'evening', gap: 19 })
    expect(periodGap(buckets.filter((b) => b.period === 'am'))).toBeNull()
  })
})

describe('describeTrend', () => {
  const summary = { count: 5, systolic: 145, diastolic: 87, pulse: null }

  it('states level, change versus the previous period and the widest period gap', () => {
    const lines = describeTrend(7, summary, { count: 5, systolic: 141, diastolic: 85, pulse: null }, buckets)
    expect(lines).toEqual([
      '近 7 天平均 145/87，偏高。',
      '高压比再往前 7 天高了 4。',
      '上午的高压比晚上平均高 19。',
    ])
  })

  it('calls small changes "about the same" and skips missing comparisons', () => {
    const lines = describeTrend(7, summary, { count: 3, systolic: 146, diastolic: 88, pulse: null }, [])
    expect(lines).toEqual(['近 7 天平均 145/87，偏高。', '和再往前 7 天差不多。'])
    expect(describeTrend(7, summary, null, [])).toHaveLength(1)
  })

  it('says nothing without readings', () => {
    expect(describeTrend(7, { count: 0, systolic: null, diastolic: null, pulse: null }, null, buckets)).toEqual([])
  })
})

describe('chartData', () => {
  const ampm: Trend = { mode: 'ampm', from: '2026-11-15', to: '2026-11-17', buckets, summary: { count: 5, systolic: 145, diastolic: 87, pulse: null } }

  it('names morning, afternoon and evening on the axis, with the date on the morning slot', () => {
    const d = chartData('ampm', ampm, null)
    expect(d.kind).toBe('category')
    if (d.kind !== 'category') return
    expect(d.names).toEqual(['11-16 上午', '11-16 下午', '11-16 晚上', '11-17 上午', '11-17 下午', '11-17 晚上'])
    expect(d.axisLabels).toEqual(['11-16\n上午', '\n下午', '\n晚上', '11-17\n上午', '\n下午', '\n晚上'])
  })

  it('collapses to one point per day in daily mode', () => {
    const d = chartData('daily', ampm, null)
    if (d.kind !== 'category') throw new Error('expected category data')
    expect(d.names).toEqual(['11-16', '11-17'])
    expect(d.systolic).toEqual([147, 138])
  })
})
