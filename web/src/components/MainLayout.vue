<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { showToast } from 'vant'
import { useProfilesStore } from '../stores/profiles'
import { describeError } from '../i18n/describeError'
import { t } from '../i18n/zh'

const profiles = useProfilesStore()
const failed = ref(false)

async function load() {
  failed.value = false
  try {
    await profiles.ensureLoaded()
  } catch (err) {
    failed.value = true
    showToast(describeError(err))
  }
}

function onVisibility() {
  if (document.visibilityState === 'hidden') profiles.markActive()
  else profiles.resumeAfterIdle()
}

onMounted(() => {
  load()
  document.addEventListener('visibilitychange', onVisibility)
})
onBeforeUnmount(() => document.removeEventListener('visibilitychange', onVisibility))
</script>

<template>
  <div>
    <router-view v-if="profiles.loaded" />
    <div v-else-if="failed" class="page center">
      <van-button type="primary" size="large" round @click="load">{{ t.common.retry }}</van-button>
    </div>
    <div v-else class="page center">
      <van-loading size="36px" vertical>{{ t.common.loading }}</van-loading>
    </div>

    <van-tabbar route safe-area-inset-bottom>
      <van-tabbar-item to="/record" icon="calendar-o">{{ t.tabs.record }}</van-tabbar-item>
      <van-tabbar-item to="/trend" icon="chart-trending-o">{{ t.tabs.trend }}</van-tabbar-item>
      <van-tabbar-item to="/family" icon="friends-o">{{ t.tabs.family }}</van-tabbar-item>
      <van-tabbar-item to="/me" icon="user-o">{{ t.tabs.me }}</van-tabbar-item>
    </van-tabbar>
  </div>
</template>

<style scoped>
.center {
  display: flex;
  justify-content: center;
  padding-top: 30vh;
}
</style>
