<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showConfirmDialog, showSuccessToast, showToast } from 'vant'
import { api } from '../api'
import type { ProfileInput } from '../api/types'
import ProfileForm from '../components/ProfileForm.vue'
import { useProfilesStore } from '../stores/profiles'
import { useSessionStore } from '../stores/session'
import { useDraftsStore } from '../stores/drafts'
import { initialOf, personStyle } from '../utils/person'
import { describeError } from '../i18n/describeError'
import { t } from '../i18n/zh'

const router = useRouter()
const session = useSessionStore()
const profiles = useProfilesStore()
const drafts = useDraftsStore()

const self = computed(() => profiles.self)
const style = computed(() => (self.value ? personStyle(self.value.id) : undefined))
const saving = ref(false)

const pwOpen = ref(false)
const pwBusy = ref(false)
const pw = reactive({ old: '', next: '', confirm: '' })

async function saveProfile(input: ProfileInput) {
  if (!self.value) return
  saving.value = true
  try {
    await api.updateProfile(self.value.id, input)
    await Promise.all([profiles.load(), session.load()])
    showSuccessToast(t.common.saved)
  } catch (err) {
    showToast(describeError(err))
  } finally {
    saving.value = false
  }
}

function openPassword() {
  pw.old = ''
  pw.next = ''
  pw.confirm = ''
  pwOpen.value = true
}

async function changePassword() {
  if (!pw.old || !pw.next) {
    showToast(t.login.fillAll)
    return
  }
  if (pw.next !== pw.confirm) {
    showToast(t.login.passwordMismatch)
    return
  }
  pwBusy.value = true
  try {
    await api.changePassword(pw.old, pw.next)
    pwOpen.value = false
    showSuccessToast(t.me.passwordChanged)
  } catch (err) {
    showToast(describeError(err))
  } finally {
    pwBusy.value = false
  }
}

async function logout() {
  try {
    await showConfirmDialog({ message: t.me.logoutConfirm, confirmButtonText: t.me.logout, cancelButtonText: t.common.cancel })
  } catch {
    return
  }
  try {
    await session.logout()
  } catch (err) {
    showToast(describeError(err))
    return
  }
  profiles.reset()
  drafts.reset()
  router.replace('/login')
}
</script>

<template>
  <div class="page" :style="style">
    <header class="header">
      <span class="avatar" aria-hidden="true">{{ initialOf(session.me?.displayName ?? '') }}</span>
      <div>
        <h1 class="page-title">{{ session.me?.displayName }}</h1>
      </div>
    </header>

    <h2 class="section-title">{{ t.me.myInfo }}</h2>
    <ProfileForm
      v-if="self"
      :key="self.id"
      :initial="self"
      :submit-text="t.me.saveProfile"
      :busy="saving"
      @submit="saveProfile"
    >
      <template #leading>
        <van-field
          class="boxed login-name mark-after"
          :model-value="session.me?.username ?? ''"
          :label="t.me.loginName"
          label-align="top"
          required
          readonly
        />
      </template>
    </ProfileForm>

    <div class="account-links">
      <button type="button" class="text-btn" @click="openPassword">{{ t.me.changePassword }}</button>
      <button type="button" class="text-btn danger" @click="logout">{{ t.me.logout }}</button>
    </div>

    <van-popup v-model:show="pwOpen" position="bottom" round>
      <div class="pw">
        <h2 class="pw-title">{{ t.me.changePassword }}</h2>
        <form @submit.prevent="changePassword">
          <van-cell-group>
            <van-field
              v-model="pw.old"
              class="boxed"
              type="password"
              :label="t.me.oldPassword"
              label-align="top"
              :placeholder="t.me.oldPasswordPlaceholder"
              autocomplete="current-password"
            />
            <van-field
              v-model="pw.next"
              class="boxed"
              type="password"
              :label="t.me.newPassword"
              label-align="top"
              :placeholder="t.me.newPasswordPlaceholder"
              autocomplete="new-password"
            />
            <van-field
              v-model="pw.confirm"
              class="boxed"
              type="password"
              :label="t.login.confirmPassword"
              label-align="top"
              :placeholder="t.me.confirmPasswordPlaceholder"
              autocomplete="new-password"
            />
          </van-cell-group>
          <div class="pw-actions">
            <van-button size="large" round @click="pwOpen = false">{{ t.common.cancel }}</van-button>
            <van-button type="primary" size="large" round native-type="submit" :loading="pwBusy">
              {{ t.common.save }}
            </van-button>
          </div>
        </form>
      </div>
    </van-popup>
  </div>
</template>

<style scoped>
.header {
  display: flex;
  align-items: center;
  gap: 14px;
  margin: 6px 2px 4px;
}
.avatar {
  flex-shrink: 0;
  width: 64px;
  height: 64px;
  border-radius: 50%;
  background: var(--person);
  color: #fff;
  font-size: 28px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
}
.login-name :deep(.van-field__body) {
  background: var(--bp-paper);
}
.login-name :deep(.van-field__body:focus-within) {
  border-color: var(--bp-line-strong);
  box-shadow: none;
}
.login-name :deep(input) {
  color: var(--bp-ink);
}
.account-links {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 28px;
  margin-top: 8px;
}
.pw {
  padding: 22px 4px calc(20px + env(safe-area-inset-bottom));
}
.pw-title {
  font-size: 21px;
  font-weight: 700;
  text-align: center;
  margin-bottom: 8px;
}
.pw-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin: 20px 16px 0;
}
</style>
