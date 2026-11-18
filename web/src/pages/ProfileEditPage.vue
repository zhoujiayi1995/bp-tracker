<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showConfirmDialog, showSuccessToast, showToast } from 'vant'
import { api } from '../api'
import type { ProfileInput } from '../api/types'
import ProfileForm from '../components/ProfileForm.vue'
import { useProfilesStore } from '../stores/profiles'
import { useDraftsStore } from '../stores/drafts'
import { personStyle } from '../utils/person'
import { describeError } from '../i18n/describeError'
import { t } from '../i18n/zh'

const props = defineProps<{ id?: string }>()
const router = useRouter()
const profiles = useProfilesStore()
const drafts = useDraftsStore()

const ready = ref(false)
const busy = ref(false)
const isNew = computed(() => !props.id)
const profile = computed(() => (props.id ? profiles.list.find((p) => p.id === Number(props.id)) ?? null : null))
const style = computed(() => (profile.value ? personStyle(profile.value.id) : undefined))

function back() {
  if (window.history.state?.back) router.back()
  else router.replace('/family')
}

onMounted(async () => {
  try {
    await profiles.ensureLoaded()
  } catch (err) {
    showToast(describeError(err))
  }
  if (!isNew.value && !profile.value?.canWrite) {
    back()
    return
  }
  ready.value = true
})

async function submit(input: ProfileInput) {
  busy.value = true
  try {
    if (isNew.value) {
      const created = await api.createProfile(input)
      await profiles.load()
      profiles.select(created.id)
      showSuccessToast(t.profile.created(t.pronoun(input.gender)))
      router.replace('/record')
    } else if (profile.value) {
      await api.updateProfile(profile.value.id, input)
      await profiles.load()
      showSuccessToast(t.common.saved)
      back()
    }
  } catch (err) {
    showToast(describeError(err))
  } finally {
    busy.value = false
  }
}

async function remove() {
  const p = profile.value
  if (!p) return
  try {
    await showConfirmDialog({ message: t.profile.deleteConfirm, confirmButtonText: t.common.delete, cancelButtonText: t.common.cancel })
  } catch {
    return
  }
  try {
    await api.deleteProfile(p.id)
    drafts.clear(p.id)
    if (profiles.currentId === p.id) profiles.select(null)
    await profiles.load()
    showSuccessToast(t.common.deleted)
    router.replace('/family')
  } catch (err) {
    showToast(describeError(err))
  }
}
</script>

<template>
  <van-nav-bar
    :title="isNew ? t.profile.newTitle : t.profile.editTitle"
    left-arrow
    :left-text="t.common.back"
    @click-left="back"
  />
  <div class="page page--plain" :style="style">
    <template v-if="ready">
      <p v-if="isNew" class="muted hint">{{ t.family.managedHint }}</p>
      <ProfileForm :initial="profile" :submit-text="t.common.save" :busy="busy" @submit="submit" />
      <div v-if="profile?.isManaged" class="danger-zone">
        <button type="button" class="text-btn danger" @click="remove">
          <van-icon name="delete-o" aria-hidden="true" />
          {{ t.profile.deleteButton }}
        </button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.hint {
  margin: 4px 4px 14px;
}
.danger-zone {
  display: flex;
  justify-content: center;
  margin-top: 40px;
  padding-top: 12px;
  border-top: 1px solid var(--bp-line);
}
</style>
