<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { showToast } from 'vant'
import { api } from '../api'
import type { Reading, Trend } from '../api/types'
import PersonPicker from '../components/PersonPicker.vue'
import SegmentedControl from '../components/SegmentedControl.vue'
import BpChart from '../components/BpChart.vue'
import LevelBadge from '../components/LevelBadge.vue'
import ReadingList from '../components/ReadingList.vue'
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

type Days = 7 | 30 | 90
const saved = JSON.parse(localStorage.getItem(PREFS_KEY) ?? '{}') as Partial<{ days: Days }>
const days = ref<Days>(saved.days ?? 7)
const defaultMode = (d: Days): ChartMode => (d === 7 ? 'ampm' : 'daily')
const mode = ref<ChartMode>(defaultMode(days.value))
const guideOpen = ref(false)
const listLimit = ref(PAGE_SIZE)

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

const summary = computed(() => ampm.value?.summary ?? null)
const hasData = computed(() => (summary.value?.count ?? 0) > 0)
const insight = computed(() =>
  ampm.value ? describeTrend(days.value, ampm.value.summary, previous.value?.summary ?? null, ampm.value.buckets ?? []) : [],
)
const chart = computed(() => (ampm.value ? chartData(mode.value, ampm.value, raw.value) : null))
const title = computed(() => {
  const p = current.value
  if (!p) return ''
  return p.isSelf ? t.trend.titleSelf : t.trend.title(p.displayName)
})
const hasInfo = computed(() => Boolean(current.value?.medicalHistory || current.value?.medications))

let requestSeq = 0

async function load() {
  const p = current.value
  if (!p) return
  const seq = ++requestSeq
  const { from, to } = lastDays(days.value)
  const today = new Date()
  const prev = lastDays(days.value, new Date(today.getFullYear(), today.getMonth(), today.getDate() - days.value))
  loading.value = true
  try {
    const [cur, before, rawTrend, list] = await Promise.all([
      api.trend(p.id, 'ampm', from, to),
      api.trend(p.id, 'ampm', prev.from, prev.to),
      mode.value === 'raw' ? api.trend(p.id, 'raw', from, to) : Promise.resolve(null),
      api.readings(p.id, { from, to, limit: listLimit.value }),
    ])
    // Ignore responses that were overtaken by a newer selection.
    if (seq !== requestSeq) return
    ampm.value = cur
    previous.value = before
    raw.value = rawTrend
    readings.value = list
  } catch (err) {
    if (seq === requestSeq) showToast(describeError(err))
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

function showMore() {
  listLimit.value += PAGE_SIZE
  load()
}

watch(days, (d) => {
  mode.value = defaultMode(d)
  listLimit.value = PAGE_SIZE
})
watch([() => current.value?.id, days, mode], load, { immediate: true })
watch(days, () => {
  localStorage.setItem(PREFS_KEY, JSON.stringify({ days: days.value }))
})
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
      <template v-if="hasData && chart">
        <div class="plot">
          <BpChart :data="chart" show-pulse :summary="insight.join('')" />
          <van-popover v-model:show="guideOpen" placement="bottom-end" theme="light" :offset="[0, 4]">
            <p class="guide-text">{{ t.trend.reference }}</p>
            <template #reference>
              <button type="button" class="guide" :aria-expanded="guideOpen" :aria-label="t.trend.reference">
                <span aria-hidden="true">i</span>
              </button>
            </template>
          </van-popover>
        </div>

        <SegmentedControl v-model="mode" :options="modeOptions" :aria-label="t.trend.modeLabel" class="modes" />
      </template>
      <van-loading v-else-if="loading" class="empty" size="32px" vertical>{{ t.common.loading }}</van-loading>
      <van-empty v-else image="search" :description="t.trend.empty" />
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
      <ReadingList :readings="readings" :can-write="current.canWrite" @changed="load" />
      <van-button
        v-if="readings.length >= listLimit"
        class="show-more"
        block
        round
        :loading="loading"
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
.guide {
  width: 44px;
  height: 44px;
  border: none;
  background: transparent;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}
.guide span {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  border: 1.5px solid var(--bp-line-strong);
  background: var(--bp-card);
  color: var(--bp-ink-2);
  font-family: Georgia, 'Times New Roman', serif;
  font-size: 15px;
  font-weight: 700;
  line-height: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}
.guide-text {
  margin: 0;
  max-width: 220px;
  padding: 12px 14px;
  font-size: 16px;
  line-height: 1.55;
  color: var(--bp-ink);
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
