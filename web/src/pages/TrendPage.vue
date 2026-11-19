<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { showToast } from 'vant'
import { api } from '../api'
import type { Reading, Trend } from '../api/types'
import PersonPicker from '../components/PersonPicker.vue'
import SegmentedControl from '../components/SegmentedControl.vue'
import BpChart from '../components/BpChart.vue'
import HintPopover from '../components/HintPopover.vue'
import LevelBadge from '../components/LevelBadge.vue'
import ReadingList from '../components/ReadingList.vue'
import { useToday } from '../composables/useToday'
import { useProfilesStore } from '../stores/profiles'
import { lastDays } from '../utils/date'
import { personStyle } from '../utils/person'
import { chartData, describeTrend, type ChartMode } from '../utils/trend'
import { describeError } from '../i18n/describeError'
import { t } from '../i18n/zh'

const PAGE_SIZE = 30
const PREFS_KEY = 'bp.trendPrefs.v2'

const profiles = useProfilesStore()
const current = computed(() => profiles.current)
const today = useToday()

type Days = 7 | 30 | 90
const saved = JSON.parse(localStorage.getItem(PREFS_KEY) ?? '{}') as Partial<{ days: Days }>
const days = ref<Days>(saved.days ?? 7)
const defaultMode = (d: Days): ChartMode => (d === 7 ? 'ampm' : 'daily')
const mode = ref<ChartMode>(defaultMode(days.value))
const guideOpen = ref(false)

const rangeOptions: { value: Days; label: string }[] = [
  { value: 7, label: t.trend.days7 },
  { value: 30, label: t.trend.days30 },
  { value: 90, label: t.trend.days90 },
]
const modeOptions: { value: ChartMode; label: string }[] = [
  { value: 'ampm', label: t.trend.modeAmpm },
  { value: 'daily', label: t.trend.modeDaily },
  { value: 'raw', label: t.trend.modeRaw },
]

const ampm = ref<Trend | null>(null)
const previous = ref<Trend | null>(null)
const raw = ref<Trend | null>(null)
const readings = ref<Reading[]>([])
const loading = ref(false)
const loadError = ref('')
const rawLoading = ref(false)
const rawError = ref('')
const loadingMore = ref(false)
const hasMore = ref(false)

const summary = computed(() => ampm.value?.summary ?? null)
const hasData = computed(() => (summary.value?.count ?? 0) > 0)
const insight = computed(() =>
  ampm.value ? describeTrend(days.value, ampm.value.summary, previous.value?.summary ?? null, ampm.value.buckets ?? []) : [],
)
const chart = computed(() => {
  if (!ampm.value || (mode.value === 'raw' && !raw.value)) return null
  return chartData(mode.value, ampm.value, raw.value)
})
const chartError = computed(() => loadError.value || (mode.value === 'raw' ? rawError.value : ''))
const title = computed(() => {
  const p = current.value
  if (!p) return ''
  return p.isSelf ? t.trend.titleSelf : t.trend.title(p.displayName)
})
const hasInfo = computed(() => Boolean(current.value?.medicalHistory || current.value?.medications))

let requestSeq = 0

async function load() {
  const seq = ++requestSeq
  ampm.value = null
  previous.value = null
  raw.value = null
  readings.value = []
  hasMore.value = false
  loadError.value = ''
  rawError.value = ''
  rawLoading.value = false
  loadingMore.value = false
  guideOpen.value = false

  const p = current.value
  loading.value = Boolean(p)
  if (!p) return

  const date = today.value
  const { from, to } = lastDays(days.value, date)
  const prev = lastDays(days.value, new Date(date.getFullYear(), date.getMonth(), date.getDate() - days.value))
  try {
    const [cur, before, list] = await Promise.all([
      api.trend(p.id, 'ampm', from, to),
      api.trend(p.id, 'ampm', prev.from, prev.to),
      api.readings(p.id, { from, to, limit: PAGE_SIZE + 1 }),
    ])
    if (seq !== requestSeq) return
    ampm.value = cur
    previous.value = before
    readings.value = list.slice(0, PAGE_SIZE)
    hasMore.value = list.length > PAGE_SIZE
  } catch (err) {
    if (seq === requestSeq) loadError.value = describeError(err)
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

async function loadRaw() {
  const p = current.value
  const range = ampm.value
  if (!p || !range?.summary.count || mode.value !== 'raw' || raw.value || rawLoading.value) return

  const seq = requestSeq
  rawLoading.value = true
  rawError.value = ''
  try {
    const result = await api.trend(p.id, 'raw', range.from, range.to)
    if (seq === requestSeq) raw.value = result
  } catch (err) {
    if (seq === requestSeq) rawError.value = describeError(err)
  } finally {
    if (seq === requestSeq) rawLoading.value = false
  }
}

function retryChart() {
  if (ampm.value) return loadRaw()
  return load()
}

async function showMore() {
  const p = current.value
  const range = ampm.value
  if (!p || !range || !hasMore.value || loadingMore.value) return

  const seq = requestSeq
  const { from, to } = range
  loadingMore.value = true
  try {
    const list = await api.readings(p.id, { from, to, limit: PAGE_SIZE + 1, offset: readings.value.length })
    if (seq !== requestSeq) return
    readings.value.push(...list.slice(0, PAGE_SIZE))
    hasMore.value = list.length > PAGE_SIZE
  } catch (err) {
    if (seq === requestSeq) showToast(describeError(err))
  } finally {
    if (seq === requestSeq) loadingMore.value = false
  }
}

watch(days, (d) => {
  mode.value = defaultMode(d)
  localStorage.setItem(PREFS_KEY, JSON.stringify({ days: d }))
})
watch([() => current.value?.id, days, today], load, { immediate: true })
watch([ampm, mode], loadRaw)
</script>

<template>
  <div v-if="current" class="page" :style="personStyle(current.id)">
    <PersonPicker />

    <h1 class="page-title head">{{ title }}</h1>

    <SegmentedControl v-model="days" :options="rangeOptions" :aria-label="t.trend.rangeLabel" class="range" />

    <section v-if="hasData && summary && summary.systolic !== null && summary.diastolic !== null" class="card overview">
      <div class="overview-label">{{ t.trend.average(days) }} · {{ t.trend.count(summary.count) }}</div>
      <div class="overview-main">
        <span class="big-number">{{ summary.systolic }}/{{ summary.diastolic }}</span>
        <LevelBadge :systolic="summary.systolic" :diastolic="summary.diastolic" />
      </div>
      <div v-if="summary.pulse !== null" class="muted">{{ t.bp.pulse }} {{ summary.pulse }} {{ t.bp.bpm }}</div>
      <p v-if="insight.length > 1" class="insight">{{ insight.slice(1).join('') }}</p>
    </section>

    <section class="card chart-card">
      <div v-if="hasData && chart" class="plot">
        <BpChart :data="chart" show-pulse :summary="insight.join('')" />
        <HintPopover
          v-model:open="guideOpen"
          :title="t.trend.referenceTitle"
          :items="t.trend.reference"
          mark="i"
        />
      </div>
      <van-loading v-else-if="loading || (mode === 'raw' && rawLoading)" class="empty" size="32px" vertical>{{ t.common.loading }}</van-loading>
      <van-empty v-else-if="chartError" image="error" :description="chartError">
        <van-button round @click="retryChart">{{ t.common.retry }}</van-button>
      </van-empty>
      <van-empty v-else image="search" :description="t.trend.empty" />
      <SegmentedControl
        v-if="hasData || loading || chartError"
        v-model="mode"
        :options="modeOptions"
        :aria-label="t.trend.modeLabel"
        class="modes"
      />
    </section>

    <section v-if="hasInfo" class="card info">
      <div v-if="current.medicalHistory">
        <h2 class="info-label">{{ t.trend.history }}</h2>
        <p class="info-text">{{ current.medicalHistory }}</p>
      </div>
      <div v-if="current.medications">
        <h2 class="info-label">{{ t.trend.medications }}</h2>
        <p class="info-text">{{ current.medications }}</p>
      </div>
    </section>

    <template v-if="readings.length">
      <h2 class="section-title">{{ t.trend.recent }}</h2>
      <ReadingList :key="current.id" :readings="readings" :can-write="current.canWrite" @changed="load" />
      <van-button
        v-if="hasMore"
        class="show-more"
        block
        round
        :loading="loadingMore"
        @click="showMore"
      >
        {{ t.trend.showMore(PAGE_SIZE) }}
      </van-button>
    </template>
  </div>
</template>

<style scoped>
.head {
  margin: 0 2px 14px;
}
.range {
  margin-bottom: 16px;
}
.overview-label {
  font-size: 17px;
  color: var(--bp-ink-2);
}
.overview-main {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin: 2px 0;
}
.insight {
  margin: 10px 0 0;
  padding-top: 10px;
  border-top: 1px solid var(--bp-line);
  font-size: 18px;
  line-height: 1.6;
}
.chart-card {
  padding: 12px 8px 8px;
}
.plot {
  position: relative;
}
.plot :deep(.van-popover__wrapper) {
  position: absolute;
  top: -4px;
  right: -2px;
  z-index: 2;
}
.modes {
  margin: 12px 4px 4px;
}
.empty {
  padding: 60px 0;
}
.info {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.info-label {
  font-size: 16px;
  font-weight: 600;
  color: var(--bp-ink-2);
}
.info-text {
  margin: 2px 0 0;
  white-space: pre-wrap;
  font-size: 18px;
}
.show-more {
  margin-top: 12px;
  --van-button-default-background: var(--bp-card);
}
</style>
