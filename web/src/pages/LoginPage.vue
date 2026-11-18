<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { useSessionStore } from '../stores/session'
import { useProfilesStore } from '../stores/profiles'
import { useDraftsStore } from '../stores/drafts'
import { describeError } from '../i18n/describeError'
import { t } from '../i18n/zh'

const router = useRouter()
const session = useSessionStore()
const profiles = useProfilesStore()
const drafts = useDraftsStore()

const tab = ref<'login' | 'register'>('login')
const busy = ref(false)
const passwordVisible = ref(false)
const form = reactive({ username: '', password: '', confirm: '', displayName: '' })

async function submit() {
  const username = form.username.trim()
  if (!username || !form.password || (tab.value === 'register' && !form.displayName.trim())) {
    showToast(t.login.fillAll)
    return
  }
  if (tab.value === 'register' && form.password !== form.confirm) {
    showToast(t.login.passwordMismatch)
    return
  }

  busy.value = true
  try {
    if (tab.value === 'login') await session.login(username, form.password)
    else await session.register(username, form.password, form.displayName.trim())
    profiles.reset()
    drafts.reset()
    router.replace('/record')
  } catch (err) {
    showToast(describeError(err))
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="login">
    <header class="brand">
      <img class="mark" src="/icon.svg" alt="" width="64" height="64" />
      <div>
        <h1>{{ t.appName }}</h1>
        <p class="slogan">{{ t.appSlogan }}</p>
      </div>
    </header>

    <div class="card form">
      <van-tabs v-model:active="tab" animated>
        <van-tab :title="t.login.loginTab" name="login" />
        <van-tab :title="t.login.registerTab" name="register" />
      </van-tabs>

      <form @submit.prevent="submit">
        <van-cell-group>
          <van-field
            v-if="tab === 'register'"
            v-model="form.displayName"
            class="boxed"
            :label="t.login.displayName"
            label-align="top"
            :placeholder="t.login.displayNamePlaceholder"
            maxlength="20"
            clearable
          />
          <van-field
            v-model="form.username"
            class="boxed"
            :label="t.login.username"
            label-align="top"
            :placeholder="tab === 'login' ? t.login.usernameLoginPlaceholder : t.login.usernameRegisterPlaceholder"
            autocomplete="username"
            autocapitalize="off"
            maxlength="32"
            clearable
          />
          <van-field
            v-model="form.password"
            class="boxed"
            :type="passwordVisible ? 'text' : 'password'"
            :label="t.login.password"
            label-align="top"
            :placeholder="tab === 'register' ? t.login.passwordPlaceholder : ''"
            :autocomplete="tab === 'login' ? 'current-password' : 'new-password'"
            maxlength="72"
          >
            <template #right-icon>
              <button
                type="button"
                class="reveal"
                :aria-label="passwordVisible ? t.login.hidePassword : t.login.showPassword"
                :aria-pressed="passwordVisible"
                @mousedown.prevent
                @click="passwordVisible = !passwordVisible"
              >
                <van-icon :name="passwordVisible ? 'eye-o' : 'closed-eye'" aria-hidden="true" />
              </button>
            </template>
          </van-field>
          <van-field
            v-if="tab === 'register'"
            v-model="form.confirm"
            class="boxed"
            type="password"
            :label="t.login.confirmPassword"
            label-align="top"
            :placeholder="t.login.confirmPlaceholder"
            autocomplete="new-password"
            maxlength="72"
          />
        </van-cell-group>

        <van-button class="submit" type="primary" size="large" block round native-type="submit" :loading="busy">
          {{ tab === 'login' ? t.login.loginButton : t.login.registerButton }}
        </van-button>
      </form>
    </div>
  </div>
</template>

<style scoped>
.login {
  min-height: 100vh;
  padding: 9vh 16px 32px;
  max-width: 480px;
  margin: 0 auto;
}
.brand {
  display: flex;
  align-items: center;
  gap: 14px;
  margin: 0 4px 28px;
}
.mark {
  flex-shrink: 0;
  border-radius: 16px;
}
h1 {
  font-size: 30px;
  font-weight: 700;
  letter-spacing: -0.01em;
}
.slogan {
  margin: 2px 0 0;
  font-size: 17px;
  color: var(--bp-ink-2);
}
.form {
  padding: 4px 4px 20px;
}
.form :deep(.van-field__label) {
  font-weight: 600;
  margin-bottom: 6px;
}
.reveal {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  margin: -6px -8px -6px 0;
  padding: 0;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--bp-ink-2);
  font-size: 22px;
  cursor: pointer;
}
.reveal:active {
  background: var(--bp-press);
}
.submit {
  margin-top: 20px;
  width: calc(100% - 32px);
  margin-left: 16px;
  font-weight: 700;
}
</style>
