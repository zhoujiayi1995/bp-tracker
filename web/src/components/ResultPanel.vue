<script setup lang="ts">
import { computed } from 'vue'
import LevelBadge from './LevelBadge.vue'
import { adviceFor } from '../utils/bp'
import { t } from '../i18n/zh'

const props = defineProps<{ label: string; systolic: number; diastolic: number; pulse: number | null }>()
const advice = computed(() => adviceFor(props.systolic, props.diastolic))
</script>

<template>
  <section class="result" :class="advice === 'severe' ? 'high' : advice" aria-live="polite">
    <div class="label">{{ label }}</div>
    <div class="numbers">
      <span class="big-number">{{ systolic }}<span class="slash">/</span>{{ diastolic }}</span>
      <span class="unit">{{ t.bp.mmHg }}</span>
      <LevelBadge :systolic="systolic" :diastolic="diastolic" />
    </div>
    <div v-if="pulse !== null" class="pulse">{{ t.bp.pulse }} {{ pulse }} {{ t.bp.bpm }}</div>
    <p class="advice">{{ t.advice[advice] }}</p>
    <div v-if="$slots.default" class="actions"><slot /></div>
  </section>
</template>

<style scoped>
.result {
  border-radius: var(--bp-radius);
  padding: 16px 18px;
  margin-bottom: 16px;
  color: var(--bp-ink);
}
.low {
  background: var(--bp-low-bg);
}
.normal {
  background: var(--bp-normal-bg);
}
.elevated {
  background: var(--bp-elevated-bg);
}
.high {
  background: var(--bp-high-bg);
}
.label {
  font-size: 17px;
  color: var(--bp-ink-2);
}
.numbers {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 4px 10px;
  margin-top: 2px;
}
.slash {
  margin: 0 2px;
  color: var(--bp-ink-2);
  font-weight: 500;
}
.unit {
  font-size: 16px;
  color: var(--bp-ink-2);
}
.numbers :deep(.badge) {
  align-self: center;
  background: var(--bp-card);
}
.pulse {
  font-size: 17px;
  color: var(--bp-ink-2);
  margin-top: 2px;
}
.advice {
  margin: 10px 0 0;
  font-size: 18px;
  line-height: 1.6;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 14px;
}
</style>
