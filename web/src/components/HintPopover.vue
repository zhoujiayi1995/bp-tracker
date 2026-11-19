<script setup lang="ts">
defineProps<{
  title: string
  items: readonly string[]
  mark?: string
}>()

const open = defineModel<boolean>('open', { default: false })
</script>

<template>
  <van-popover
    v-model:show="open"
    class="hint-pop"
    placement="bottom-end"
    theme="light"
    :offset="[0, 8]"
    teleport=""
  >
    <div class="hint">
      <p class="hint-title">{{ title }}</p>
      <ul>
        <li v-for="item in items" :key="item">{{ item }}</li>
      </ul>
    </div>
    <template #reference>
      <button type="button" class="guide" :aria-expanded="open" :aria-label="title">
        <span aria-hidden="true">{{ mark ?? '?' }}</span>
      </button>
    </template>
  </van-popover>
</template>

<style scoped>
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
  font-size: 16px;
  font-weight: 700;
  line-height: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}
.hint-pop {
  --van-popover-light-background: var(--person-tint);
  --van-popover-radius: 16px;
}
.hint-pop :deep(.van-popover__content) {
  box-shadow: var(--bp-shadow);
}
.hint {
  width: min(300px, calc(100vw - 48px));
  padding: 14px 16px 16px;
}
.hint-title {
  margin: 0 0 10px;
  font-size: 16px;
  font-weight: 700;
  color: var(--person);
}
.hint ul {
  margin: 0;
  padding: 0;
  list-style: none;
}
.hint li {
  position: relative;
  padding-left: 16px;
  font-size: 16px;
  line-height: 1.55;
  color: var(--bp-ink);
}
.hint li + li {
  margin-top: 10px;
}
.hint li::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0.48em;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--person);
}
</style>
