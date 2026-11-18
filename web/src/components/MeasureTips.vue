<script setup lang="ts">
import { ref } from 'vue'
import { t } from '../i18n/zh'

const open = ref(false)
</script>

<template>
  <van-popover v-model:show="open" placement="bottom-end" theme="light" :offset="[0, 4]">
    <ol class="tips">
      <li v-for="tip in t.record.tips" :key="tip">{{ tip }}</li>
    </ol>
    <template #reference>
      <button type="button" class="guide" :aria-expanded="open" :aria-label="t.record.tipsTitle">
        <span aria-hidden="true">?</span>
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
.tips {
  margin: 0;
  max-width: 260px;
  padding: 12px 16px 12px 32px;
  font-size: 16px;
  line-height: 1.6;
  color: var(--bp-ink);
}
.tips li + li {
  margin-top: 6px;
}
</style>
