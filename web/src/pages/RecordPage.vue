<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { showSuccessToast, showToast } from 'vant'
import { api } from '../api'
import type { Profile, Reading } from '../api/types'
import PersonPicker from '../components/PersonPicker.vue'
import TodayStatus from '../components/TodayStatus.vue'
import ReadingForm from '../components/ReadingForm.vue'
import ResultPanel from '../components/ResultPanel.vue'
import ReadingList from '../components/ReadingList.vue'
import { useProfilesStore } from '../stores/profiles'
import { adviceFor } from '../utils/bp'
import { toYMD } from '../utils/date'
import { personStyle } from '../utils/person'
import { describeError } from '../i18n/describeError'
import { t } from '../i18n/zh'

const RECENT_LIMIT = 5

const router = useRouter()
const profiles = useProfilesStore()
const current = computed(() => profiles.current)

const recent = ref<Reading[]>([])
const todayReadings = ref<Reading[]>([])
const lastSaved = ref<Reading | null>(null)
const savedPanel = ref<HTMLElement | null>(null)
const form = ref<InstanceType<typeof ReadingForm> | null>(null)

function greeting(hour: number): string {
  const g = t.today.greetings
  if (hour < 11) return g.morning
  if (hour < 13) return g.noon
  if (hour < 18) return g.afternoon
  return g.evening
}

function titleOf(p: Profile): string {
  if (p.isSelf) return t.today.titleSelf(greeting(new Date().getHours()), p.displayName)
  return p.canWrite ? t.today.titleManaged(p.displayName) : t.today.titleReadonly(p.displayName)
}

const savedAdvice = computed(() => (lastSaved.value ? adviceFor(lastSaved.value.systolic, lastSaved.value.diastolic) : null))

let requestSeq = 0

async function load() {
  const p = current.value
  if (!p) return
  const seq = ++requestSeq
  const today = toYMD(new Date())
  try {
    const [rec, tod] = await Promise.all([
      api.readings(p.id, { limit: RECENT_LIMIT }),
      api.readings(p.id, { from: today, to: today }),
    ])
    if (seq !== requestSeq) return
    recent.value = rec
    todayReadings.value = tod
  } catch (err) {
    if (seq === requestSeq) showToast(describeError(err))
  }
}

async function onSaved(r: Reading) {
  lastSaved.value = r
  showSuccessToast(t.common.saved)
  load()
  await nextTick()
  savedPanel.value?.scrollIntoView({ block: 'start', behavior: 'smooth' })
}

function remeasure() {
  lastSaved.value = null
  nextTick(() => form.value?.focusFirst())
}

watch(
  () => current.value,
  (p) => {
    if (p && !p.canWrite && profiles.self) profiles.select(profiles.self.id)
  },
  { immediate: true },
)
watch(
  () => current.value?.id,
  () => {
    lastSaved.value = null
    recent.value = []
    todayReadings.value = []
    load()
  },
  { immediate: true },
)
</script>

<template>
  <div v-if="current" class="page" :style="personStyle(current.id)">
    <PersonPicker writable-only />

    <header class="head">
      <h1 class="page-title">{{ titleOf(current) }}</h1>
      <p v-if="!current.isSelf && !current.canWrite" class="role">
        {{ t.today.roleReadonlyBefore(current.displayName) }}<span class="keep">{{ t.today.roleReadonlyKeep }}</span>
      </p>
    </header>

    <TodayStatus :readings="todayReadings" />

    <p v-if="current.medications" class="meds">
      <span class="meds-label">{{ t.today.medications }}</span>
      {{ current.medications }}
    </p>

    <template v-if="current.canWrite">
      <div v-if="lastSaved" ref="savedPanel" class="saved">
        <ResultPanel
          :label="t.record.savedLabel"
          :systolic="lastSaved.systolic"
          :diastolic="lastSaved.diastolic"
          :pulse="lastSaved.pulse"
        >
          <template v-if="savedAdvice === 'high' || savedAdvice === 'severe'" #default>
            <van-button round icon="replay" class="panel-btn" @click="remeasure">
              {{ t.record.remeasureAfterRest }}
            </van-button>
            <a v-if="savedAdvice === 'severe'" class="call" href="tel:120">
              <van-icon name="phone-o" aria-hidden="true" />
              {{ t.record.call120 }}
            </a>
          </template>
        </ResultPanel>
      </div>

      <ReadingForm
        :key="current.id"
        ref="form"
        :profile-id="current.id"
        :owner-name="current.displayName"
        :is-self="current.isSelf"
        :last-reading="recent[0] ?? null"
        @saved="onSaved"
        @edit="lastSaved = null"
      />
    </template>

    <van-button v-else class="go-trend" size="large" block round icon="chart-trending-o" @click="router.push('/trend')">
      {{ t.today.goTrend }}
    </van-button>

    <h2 class="section-title">{{ t.record.recent }}</h2>
    <ReadingList v-if="recent.length" :readings="recent" :can-write="current.canWrite" @changed="load" />
    <p v-else class="empty">{{ t.record.noRecent }}</p>
  </div>
</template>

<style scoped>
.head {
  margin: 0 2px 16px;
}
.role {
  margin: 6px 0 0;
  max-width: 36em;
  font-size: 17px;
  line-height: 1.55;
  color: var(--bp-ink-2);
  text-wrap: pretty;
}
.keep {
  white-space: nowrap;
}
.meds {
  margin: -4px 2px 16px;
  font-size: 17px;
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.meds-label {
  display: inline-block;
  margin-right: 6px;
  padding: 0 8px;
  border-radius: 6px;
  background: var(--person-tint);
  color: var(--person);
  font-size: 15px;
  font-weight: 600;
}
.saved {
  scroll-margin-top: 12px;
}
.panel-btn {
  --van-button-default-background: var(--bp-card);
}
.call {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 48px;
  padding: 0 20px;
  border-radius: 999px;
  background: var(--bp-high);
  color: #fff;
  font-size: 18px;
  font-weight: 700;
  text-decoration: none;
}
.go-trend {
  --van-button-default-background: var(--bp-card);
}
.empty {
  margin: 0 2px;
  padding: 18px 16px;
  border-radius: var(--bp-radius);
  border: 1.5px dashed var(--bp-line-strong);
  font-size: 17px;
  color: var(--bp-ink-2);
}
</style>
