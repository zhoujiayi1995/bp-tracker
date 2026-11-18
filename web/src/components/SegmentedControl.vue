<script setup lang="ts" generic="T extends string | number">
defineProps<{ options: { value: T; label: string }[] }>()
const model = defineModel<T>({ required: true })
</script>

<template>
  <div class="segmented" role="radiogroup">
    <button
      v-for="opt in options"
      :key="String(opt.value)"
      type="button"
      role="radio"
      :aria-checked="model === opt.value"
      :class="{ active: model === opt.value }"
      @click="model = opt.value"
    >
      {{ opt.label }}
    </button>
  </div>
</template>

<style scoped>
.segmented {
  display: flex;
  background: var(--bp-press);
  border-radius: 14px;
  padding: 4px;
  gap: 4px;
}
button {
  flex: 1;
  min-height: 48px;
  padding: 0 6px;
  border: none;
  border-radius: 11px;
  background: transparent;
  font-size: 17px;
  color: var(--bp-ink);
  font-family: inherit;
}
button.active {
  background: var(--bp-card);
  color: var(--person);
  font-weight: 700;
  box-shadow: 0 1px 2px rgba(60, 40, 20, 0.1), 0 2px 6px rgba(60, 40, 20, 0.08);
}
</style>
