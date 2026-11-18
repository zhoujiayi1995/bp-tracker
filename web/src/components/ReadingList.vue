<script setup lang="ts">
import { ref } from 'vue'
import type { Reading } from '../api/types'
import LevelBadge from './LevelBadge.vue'
import ReadingDetailPopup from './ReadingDetailPopup.vue'
import { adviceFor } from '../utils/bp'
import { formatDay, formatTime } from '../utils/date'
import { periodLabel } from '../utils/period'
import { t } from '../i18n/zh'

defineProps<{ readings: Reading[]; canWrite: boolean }>()
const emit = defineEmits<{ changed: [] }>()

const selected = ref<Reading | null>(null)
const open = ref(false)

function select(r: Reading) {
  selected.value = r
  open.value = true
}

function spoken(r: Reading): string {
  const when = `${formatDay(new Date(r.measuredAt))} ${periodLabel(r.period)} ${formatTime(new Date(r.measuredAt))}`
  const pulse = r.pulse !== null ? `，${t.bp.pulse} ${r.pulse}` : ''
  return `${when}，${t.bp.systolic} ${r.systolic}，${t.bp.diastolic} ${r.diastolic}${pulse}，${t.level[adviceFor(r.systolic, r.diastolic)]}`
}
</script>

<template>
  <ul class="list">
    <li v-for="r in readings" :key="r.id">
      <button type="button" class="item" :aria-label="spoken(r)" @click="select(r)">
        <span class="when">
          <span class="day">{{ formatDay(new Date(r.measuredAt)) }}</span>
          <span class="time">
            <span class="period">{{ periodLabel(r.period) }}</span>
            {{ formatTime(new Date(r.measuredAt)) }}
          </span>
        </span>
        <span class="values">
          <span class="bp">{{ r.systolic }}/{{ r.diastolic }}</span>
          <span class="sub">
            <span v-if="r.pulse !== null">{{ t.bp.pulse }} {{ r.pulse }}</span>
            <LevelBadge :systolic="r.systolic" :diastolic="r.diastolic" />
          </span>
        </span>
      </button>
    </li>
  </ul>

  <ReadingDetailPopup v-model:show="open" :reading="selected" :can-write="canWrite" @changed="emit('changed')" />
</template>

<style scoped>
.list {
  list-style: none;
  margin: 0;
  padding: 0;
  border-radius: var(--bp-radius);
  background: var(--bp-card);
  box-shadow: var(--bp-shadow);
  overflow: hidden;
}
li + li {
  border-top: 1px solid var(--bp-line);
}
.item {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: center;
  gap: 4px 10px;
  width: 100%;
  min-height: 72px;
  border: none;
  text-align: left;
  font-family: inherit;
  color: inherit;
  background: transparent;
  padding: 12px 16px;
}
.item:active {
  background: var(--bp-press);
}
.when,
.values {
  display: flex;
  flex-direction: column;
}
.day {
  font-size: 18px;
  font-weight: 600;
}
.time {
  font-size: 16px;
  color: var(--bp-ink-2);
  display: flex;
  align-items: center;
  gap: 6px;
}
.period {
  font-size: 15px;
  padding: 0 6px;
  border-radius: 6px;
  background: var(--bp-paper);
  color: var(--bp-ink-2);
}
.values {
  align-items: flex-end;
  margin-left: auto;
  max-width: 100%;
}
.bp {
  font-size: 28px;
  font-weight: 700;
  line-height: 1.2;
  font-variant-numeric: tabular-nums;
}
.sub {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  justify-content: flex-end;
  font-size: 16px;
  color: var(--bp-ink-2);
}
</style>
