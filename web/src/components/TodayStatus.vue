<script setup lang="ts">
import { computed } from 'vue'
import type { Reading } from '../api/types'
import LevelBadge from './LevelBadge.vue'
import { formatTime } from '../utils/date'
import { PERIODS, periodLabel } from '../utils/period'
import { t } from '../i18n/zh'

const props = defineProps<{ readings: Reading[] }>()

const now = new Date()

// Readings arrive newest first, so the first match is the latest of that period.
const slots = computed(() =>
  PERIODS.map((period) => ({
    period,
    label: periodLabel(period),
    reading: props.readings.find((r) => r.period === period) ?? null,
  })),
)
</script>

<template>
  <section class="today">
    <div class="calendar">
      <span class="month">{{ t.today.month(now.getMonth() + 1) }}</span>
      <span class="day">{{ now.getDate() }}</span>
      <span class="weekday">{{ t.fmt.weekdays[now.getDay()] }}</span>
    </div>
    <ul class="slots">
      <li v-for="s in slots" :key="s.period" class="slot" :class="{ done: s.reading }">
        <span class="slot-name">{{ s.label }}</span>
        <template v-if="s.reading">
          <span class="slot-state">
            <van-icon name="checked" class="check" aria-hidden="true" />
            {{ t.today.measured }}
            <span class="slot-time">{{ formatTime(new Date(s.reading.measuredAt)) }}</span>
          </span>
          <span class="slot-value">
            <strong>{{ s.reading.systolic }}/{{ s.reading.diastolic }}</strong>
            <LevelBadge :systolic="s.reading.systolic" :diastolic="s.reading.diastolic" />
          </span>
        </template>
        <span v-else class="slot-state todo">{{ t.today.notYet }}</span>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.today {
  display: flex;
  gap: 14px;
  align-items: stretch;
  margin-bottom: 18px;
}
/* A tear-off desk calendar page: the daily ritual this screen is built around. */
.calendar {
  flex-shrink: 0;
  width: 120px;
  display: flex;
  flex-direction: column;
  align-items: center;
  border-radius: 14px;
  overflow: hidden;
  background: var(--bp-card);
  box-shadow: var(--bp-shadow);
}
.month {
  align-self: stretch;
  text-align: center;
  background: var(--person);
  color: #fff;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: 0.04em;
  padding: 10px 0;
}
.day {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 48px;
  font-weight: 700;
  line-height: 1;
  letter-spacing: -0.02em;
}
.weekday {
  font-size: 20px;
  font-weight: 600;
  color: var(--bp-ink-2);
  padding: 4px 0 14px;
}
.slots {
  flex: 1;
  min-width: 0;
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.slot {
  flex: 1;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  column-gap: 10px;
  row-gap: 2px;
  padding: 8px 12px;
  border-radius: 12px;
  background: var(--bp-card);
  border: 1.5px dashed var(--bp-line-strong);
}
.slot.done {
  border: 1.5px solid transparent;
  background: var(--person-tint);
}
.slot-name {
  font-size: 18px;
  font-weight: 700;
}
.slot-state {
  font-size: 16px;
  color: var(--bp-ink-2);
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.slot-state.todo {
  color: var(--bp-ink-2);
}
.check {
  color: var(--person);
  font-size: 18px;
}
.slot-time {
  margin-left: 2px;
}
.slot-value {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 20px;
}
</style>
