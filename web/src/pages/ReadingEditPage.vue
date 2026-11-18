<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showSuccessToast, showToast } from 'vant'
import { api } from '../api'
import type { Profile, Reading } from '../api/types'
import ReadingForm from '../components/ReadingForm.vue'
import { useProfilesStore } from '../stores/profiles'
import { personStyle } from '../utils/person'
import { describeError } from '../i18n/describeError'
import { t } from '../i18n/zh'

const props = defineProps<{ id: string }>()
const router = useRouter()
const profiles = useProfilesStore()
const reading = ref<Reading | null>(null)
const owner = ref<Profile | null>(null)
const style = computed(() => (owner.value ? personStyle(owner.value.id) : undefined))

function back() {
  if (window.history.state?.back) router.back()
  else router.replace('/record')
}

onMounted(async () => {
  try {
    await profiles.ensureLoaded()
    const r = await api.reading(Number(props.id))
    const p = profiles.list.find((x) => x.id === r.profileId)
    if (!p?.canWrite) {
      back()
      return
    }
    owner.value = p
    reading.value = r
  } catch (err) {
    showToast(describeError(err))
    back()
  }
})

function onSaved() {
  showSuccessToast(t.common.saved)
  back()
}
</script>

<template>
  <van-nav-bar :title="t.record.editTitle" left-arrow :left-text="t.common.back" @click-left="back" />
  <div class="page page--plain" :style="style">
    <ReadingForm
      v-if="reading && owner"
      :profile-id="reading.profileId"
      :owner-name="owner.displayName"
      :is-self="owner.isSelf"
      :initial="reading"
      @saved="onSaved"
    />
    <van-loading v-else class="loading" size="36px" vertical>{{ t.common.loading }}</van-loading>
  </div>
</template>

<style scoped>
.loading {
  padding-top: 25vh;
}
</style>
