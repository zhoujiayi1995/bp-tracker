<script setup lang="ts">
import HintPopover from './HintPopover.vue'
import { t } from '../i18n/zh'
</script>

<template>
  <HintPopover :title="t.record.tipsTitle" :items="t.record.tips" mark="i" />
</template>
