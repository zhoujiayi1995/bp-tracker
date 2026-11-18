<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart, type LineSeriesOption } from 'echarts/charts'
import {
  GridComponent,
  LegendComponent,
  MarkAreaComponent,
  MarkLineComponent,
  TooltipComponent,
  type GridComponentOption,
  type LegendComponentOption,
  type MarkAreaComponentOption,
  type MarkLineComponentOption,
  type TooltipComponentOption,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { ChartData } from '../utils/trend'
import { t } from '../i18n/zh'

echarts.use([
  LineChart,
  GridComponent,
  LegendComponent,
  MarkAreaComponent,
  MarkLineComponent,
  TooltipComponent,
  CanvasRenderer,
])

type Option = echarts.ComposeOption<
  | LineSeriesOption
  | GridComponentOption
  | LegendComponentOption
  | TooltipComponentOption
  | MarkLineComponentOption
  | MarkAreaComponentOption
>

const props = defineProps<{ data: ChartData; showPulse: boolean; summary: string }>()

const el = ref<HTMLDivElement | null>(null)
let chart: echarts.ECharts | null = null
let observer: ResizeObserver | null = null

const css = (name: string) => getComputedStyle(document.documentElement).getPropertyValue(name).trim()

// Upper bound beyond any valid reading; the grid clips the band at the axis maximum.
const BAND_TOP = 300

function buildOption(): Option {
  const colors = {
    sys: css('--bp-systolic'),
    dia: css('--bp-diastolic'),
    pulse: css('--bp-pulse'),
    high: css('--bp-high'),
    ink2: css('--bp-ink-2'),
    line: css('--bp-line'),
  }
  const d = props.data
  const isTime = d.kind === 'time'

  const sys = isTime ? d.points.map((p) => [p.at, p.systolic]) : d.systolic
  const dia = isTime ? d.points.map((p) => [p.at, p.diastolic]) : d.diastolic
  const pulse = isTime ? d.points.map((p) => [p.at, p.pulse]) : d.pulse
  const count = isTime ? d.points.length : d.names.length
  const symbolSize = count > 40 ? 6 : 10

  const line = (name: string, data: unknown[], color: string, extra: LineSeriesOption = {}): LineSeriesOption => ({
    type: 'line',
    name,
    data: data as LineSeriesOption['data'],
    // Empty periods are null; joining the real points keeps the trend readable even when
    // someone only measures in the morning.
    connectNulls: true,
    showSymbol: true,
    symbolSize,
    lineStyle: { width: 3, color },
    itemStyle: { color },
    ...extra,
  })

  const highBand: MarkAreaComponentOption = {
    silent: true,
    itemStyle: { color: 'rgba(180, 35, 24, 0.07)' },
    data: [[{ yAxis: 140 }, { yAxis: BAND_TOP }]],
  }
  const diastolicLine: MarkLineComponentOption = {
    silent: true,
    symbol: 'none',
    lineStyle: { type: 'dashed', width: 2, color: colors.dia },
    label: { formatter: '90', fontSize: 15, color: colors.dia, position: 'insideStartTop' },
    data: [{ yAxis: 90 }],
  }

  const series: LineSeriesOption[] = [
    line(t.bp.systolic, sys, colors.sys, { markArea: highBand }),
    line(t.bp.diastolic, dia, colors.dia, { markLine: diastolicLine }),
  ]
  if (props.showPulse) {
    series.push(
      line(t.bp.pulse, pulse, colors.pulse, {
        lineStyle: { width: 2, type: 'dotted', color: colors.pulse },
        symbolSize: Math.max(5, symbolSize - 3),
      }),
    )
  }

  const twoLine = !isTime && d.axisLabels.some((label) => label.includes('\n'))
  const fitted = twoLine ? fitPeriodLabels(el.value?.clientWidth ?? 320, d.axisLabels) : null
  const axisLabel = { fontSize: 15, color: colors.ink2, lineHeight: 18 }
  return {
    animation: false,
    grid: { left: 8, right: 12, top: 44, bottom: twoLine ? 28 : 8, containLabel: true },
    legend: { top: 6, left: 0, right: 40, itemWidth: 22, itemHeight: 12, textStyle: { fontSize: 16, color: css('--bp-ink') } },
    tooltip: {
      trigger: 'axis',
      confine: true,
      textStyle: { fontSize: 17 },
      valueFormatter: (v) => (v === null || v === undefined ? '-' : String(Array.isArray(v) ? v[1] : v)),
    },
    xAxis:
      d.kind === 'time'
        ? {
            type: 'time',
            axisLabel: { ...axisLabel, hideOverlap: true, formatter: '{MM}-{dd}' },
            axisLine: { lineStyle: { color: colors.line } },
            splitLine: { show: false },
          }
        : {
            type: 'category',
            data: d.names,
            axisLine: { lineStyle: { color: colors.line } },
            axisTick: { show: false },
            axisLabel: {
              ...axisLabel,
              hideOverlap: fitted ? fitted.hideOverlap : true,
              interval: fitted ? fitted.interval : 0,
              formatter: (_: string, index: number) => (fitted ? fitted.text(index) : (d.axisLabels[index] ?? '')),
            },
          },
    yAxis: {
      type: 'value',
      // Bounds snap to multiples of 20 so they coincide with tick labels; the top always
      // leaves room for the 140 band.
      min: (v: { min: number }) => Math.max(0, Math.floor((v.min - 5) / 20) * 20),
      max: (v: { max: number }) => Math.max(160, Math.ceil((v.max + 5) / 20) * 20),
      interval: 20,
      axisLabel,
      splitLine: { lineStyle: { color: colors.line } },
    },
    series,
  }
}

function render() {
  if (!chart) return
  chart.setOption(buildOption(), { notMerge: true })
}

const PERIODS_PER_DAY = 3

// Period labels are two lines (date, then 上午/下午/晚上). Pick a density that fits the chart
// width instead of forcing the chart wider than the screen.
function fitPeriodLabels(width: number, labels: string[]) {
  const count = Math.max(labels.length, 1)
  const plot = Math.max(80, width - 52)
  const slot = plot / count
  const daySlot = slot * PERIODS_PER_DAY
  const text = (index: number) => labels[index] ?? ''
  if (slot >= 52) return { interval: 0, hideOverlap: false, text }
  if (slot >= 34) {
    return {
      interval: 0,
      hideOverlap: false,
      text: (index: number) => text(index).split('\n').pop() ?? '',
    }
  }
  const step = Math.max(1, Math.ceil(48 / daySlot))
  return {
    interval: PERIODS_PER_DAY * step - 1,
    hideOverlap: true,
    text: (index: number) => {
      // These ticks are one per day, so the label is the date. Naming the morning
      // period here would hide that afternoon and evening points sit beside it.
      const date = text(index).split('\n')[0] ?? ''
      return date
    },
  }
}

onMounted(() => {
  if (!el.value) return
  chart = echarts.init(el.value)
  render()
  let seen = el.value.clientWidth
  observer = new ResizeObserver(() => {
    const width = el.value?.clientWidth ?? 0
    chart?.resize()
    if (Math.abs(width - seen) < 8) return
    seen = width
    render()
  })
  observer.observe(el.value)
})

onBeforeUnmount(() => {
  observer?.disconnect()
  chart?.dispose()
  chart = null
})

watch(() => [props.data, props.showPulse], () => {
  chart?.resize()
  render()
})
</script>

<template>
  <div ref="el" class="chart" role="img" :aria-label="summary" />
</template>

<style scoped>
.chart {
  width: 100%;
  height: 260px;
}
</style>
