<script setup lang="ts">
import { computed } from 'vue'
import { adviceFor } from '../utils/bp'
import { t } from '../i18n/zh'

const props = defineProps<{ systolic: number; diastolic: number }>()
const advice = computed(() => adviceFor(props.systolic, props.diastolic))
</script>

<template>
  <span class="badge" :class="advice === 'severe' ? 'high' : advice">{{ t.level[advice] }}</span>
</template>

<style scoped>
/* Text label plus colour, so the level is readable without colour vision. */
.badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 10px 2px 8px;
  border-radius: 999px;
  font-size: 16px;
  font-weight: 600;
  line-height: 1.6;
  white-space: nowrap;
}
.badge::before {
  content: '';
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
}
.low {
  color: var(--bp-low);
  background: var(--bp-low-bg);
}
.normal {
  color: var(--bp-normal);
  background: var(--bp-normal-bg);
}
.elevated {
  color: var(--bp-elevated);
  background: var(--bp-elevated-bg);
}
.high {
  color: var(--bp-high);
  background: var(--bp-high-bg);
}
</style>
