<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { showConfirmDialog, showSuccessToast, showToast } from 'vant'
import { api } from '../api'
import type { FamilyView, Profile, Reading } from '../api/types'
import LevelBadge from '../components/LevelBadge.vue'
import { useProfilesStore } from '../stores/profiles'
import { useSessionStore } from '../stores/session'
import { formatDay, formatTime, toYMD } from '../utils/date'
import { initialOf, personStyle } from '../utils/person'
import { describeError } from '../i18n/describeError'
import { t } from '../i18n/zh'

const router = useRouter()
const session = useSessionStore()
const profiles = useProfilesStore()

const view = ref<FamilyView | null>(null)
const latest = ref<Record<number, Reading | null>>({})
const dialog = ref<'create' | 'join' | null>(null)
const input = ref('')
const busy = ref(false)

const codeDisplay = computed(() => {
  const c = view.value?.family?.inviteCode ?? ''
  return `${c.slice(0, 3)} ${c.slice(3)}`
})
const todayYMD = toYMD(new Date())

async function load() {
  try {
    view.value = await api.family()
  } catch (err) {
    showToast(describeError(err))
  }
}

// One request per person is fine at family scale and keeps the API unchanged.
async function loadLatest() {
  const entries = await Promise.all(
    profiles.list.map(async (p) => {
      try {
        const [r] = await api.readings(p.id, { limit: 1 })
        return [p.id, r ?? null] as const
      } catch {
        return [p.id, null] as const
      }
    }),
  )
  latest.value = Object.fromEntries(entries)
}

function roleOf(p: Profile): string {
  if (p.isSelf) return t.people.self
  if (p.isManaged) return p.canWrite ? t.people.managedByMe : t.people.managedBy(p.managerName ?? '')
  return t.people.familyReadonly
}

const people = computed(() =>
  profiles.list.map((p) => {
    const r = latest.value[p.id] ?? null
    const today = Boolean(r && toYMD(new Date(r.measuredAt)) === todayYMD)
    let when = t.family.noReading
    if (r) {
      const at = new Date(r.measuredAt)
      when = today ? `${t.family.measuredToday} ${formatTime(at)}` : formatDay(at)
    }
    return { profile: p, reading: r, today, when }
  }),
)

function openPerson(p: Profile) {
  profiles.select(p.id)
  router.push(p.canWrite ? '/record' : '/trend')
}

/** Family membership changes who can see what, so refresh everything that depends on it. */
async function applyChange(next: FamilyView | null, message: string) {
  view.value = next ?? (await api.family())
  await Promise.all([session.load(), profiles.load()])
  showSuccessToast(message)
}

function openDialog(kind: 'create' | 'join') {
  input.value = ''
  dialog.value = kind
}

async function confirmDialog(): Promise<boolean> {
  const value = input.value.trim()
  if (!value) return false
  busy.value = true
  try {
    if (dialog.value === 'create') await applyChange(await api.createFamily(value), t.family.created)
    else await applyChange(await api.joinFamily(value), t.family.joined)
    return true
  } catch (err) {
    showToast(describeError(err))
    return false
  } finally {
    busy.value = false
  }
}

// van-dialog calls beforeClose with the action; returning false keeps it open on errors.
async function beforeClose(action: string): Promise<boolean> {
  if (action !== 'confirm') return true
  return confirmDialog()
}

async function copyCode() {
  const code = view.value?.family?.inviteCode
  if (!code) return
  try {
    await navigator.clipboard.writeText(code)
    showSuccessToast(t.common.copied)
  } catch {
    // Clipboard API needs HTTPS; fall back to showing the code prominently.
    showToast(code)
  }
}

async function resetCode() {
  try {
    await showConfirmDialog({ message: t.family.resetConfirm, confirmButtonText: t.common.confirm, cancelButtonText: t.common.cancel })
  } catch {
    return
  }
  try {
    view.value = await api.resetInviteCode()
  } catch (err) {
    showToast(describeError(err))
  }
}

async function leave() {
  try {
    await showConfirmDialog({ message: t.family.leaveConfirm, confirmButtonText: t.family.leave, cancelButtonText: t.common.cancel })
  } catch {
    return
  }
  try {
    await api.leaveFamily()
    await applyChange(null, t.family.left)
  } catch (err) {
    showToast(describeError(err))
  }
}

onMounted(load)
watch(() => profiles.list, loadLatest, { immediate: true })
</script>

<template>
  <div class="page">
    <h1 class="page-title head">{{ view?.family?.name ?? t.family.title }}</h1>

    <ul class="people">
      <li v-for="{ profile: p, reading: r, today, when } in people" :key="p.id" class="person" :style="personStyle(p.id)">
        <button type="button" class="person-main" @click="openPerson(p)">
          <span class="avatar" aria-hidden="true">{{ initialOf(p.displayName) }}</span>
          <span class="who">
            <span class="name">{{ p.displayName }}</span>
            <span class="role">{{ roleOf(p) }}</span>
          </span>
          <span class="status">
            <span v-if="r" class="latest">
              <strong>{{ r.systolic }}/{{ r.diastolic }}</strong>
              <LevelBadge :systolic="r.systolic" :diastolic="r.diastolic" />
            </span>
            <span class="when" :class="{ today }">
              <van-icon v-if="today" name="checked" aria-hidden="true" />
              {{ when }}
            </span>
          </span>
        </button>
        <button
          v-if="p.isManaged && p.canWrite"
          type="button"
          class="text-btn edit"
          @click="router.push(`/profiles/${p.id}/edit`)"
        >
          {{ t.family.editProfile }}
        </button>
      </li>
    </ul>

    <van-button class="add" size="large" block round icon="add-o" @click="router.push('/profiles/new')">
      {{ t.family.addManaged }}
    </van-button>
    <p class="muted hint">{{ t.family.managedHint }}</p>

    <template v-if="view">
      <section v-if="!view.family" class="card join">
        <h2 class="join-title">{{ t.family.noFamilyTitle }}</h2>
        <p class="join-desc">{{ t.family.noFamilyDesc }}</p>
        <div class="stack">
          <van-button type="primary" size="large" block round icon="plus" @click="openDialog('create')">
            {{ t.family.create }}
          </van-button>
          <van-button size="large" block round icon="friends-o" @click="openDialog('join')">
            {{ t.family.join }}
          </van-button>
        </div>
      </section>

      <template v-else>
        <details class="card invite">
          <summary>
            <van-icon name="friends-o" aria-hidden="true" />
            <span>{{ t.family.inviteTitle(view.family.name) }}</span>
            <van-icon name="arrow-down" class="chevron" aria-hidden="true" />
          </summary>
          <div class="invite-body">
            <div class="muted">{{ t.family.inviteCode }}</div>
            <div class="code">{{ codeDisplay }}</div>
            <p class="muted">{{ t.family.inviteHint }}</p>
            <van-button type="primary" size="large" block round icon="description-o" @click="copyCode">
              {{ t.family.copyCode }}
            </van-button>
            <button type="button" class="text-btn reset" @click="resetCode">{{ t.family.resetCode }}</button>
          </div>
        </details>

        <div class="leave">
          <button type="button" class="text-btn danger" @click="leave">{{ t.family.leave }}</button>
        </div>
      </template>
    </template>

    <van-dialog
      :show="dialog !== null"
      :title="dialog === 'create' ? t.family.createTitle : t.family.joinTitle"
      show-cancel-button
      :confirm-button-text="t.common.confirm"
      :cancel-button-text="t.common.cancel"
      :before-close="beforeClose"
      @update:show="(v: boolean) => { if (!v) dialog = null }"
    >
      <div class="dialog-body">
        <input
          v-if="dialog === 'join'"
          v-model="input"
          class="dialog-input code-input"
          inputmode="numeric"
          pattern="[0-9]*"
          maxlength="6"
          :aria-label="t.family.joinTitle"
          :placeholder="t.family.joinPlaceholder"
        />
        <input
          v-else
          v-model="input"
          class="dialog-input"
          maxlength="20"
          :aria-label="t.family.createTitle"
          :placeholder="t.family.createPlaceholder"
        />
      </div>
    </van-dialog>
  </div>
</template>

<style scoped>
.head {
  margin: 0 2px 16px;
}
.people {
  list-style: none;
  margin: 0 0 16px;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.person {
  background: var(--bp-card);
  border-radius: var(--bp-radius);
  box-shadow: var(--bp-shadow);
  overflow: hidden;
}
.person-main {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
  width: 100%;
  min-height: 84px;
  padding: 14px 16px;
  border: none;
  background: transparent;
  font: inherit;
  color: inherit;
  text-align: left;
}
.person-main:active {
  background: var(--bp-press);
}
.avatar {
  flex-shrink: 0;
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: var(--person);
  color: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  font-weight: 700;
}
.who {
  display: flex;
  flex-direction: column;
  flex: 1 0 auto;
  white-space: nowrap;
}
.name {
  font-size: 20px;
  font-weight: 700;
}
.role {
  font-size: 16px;
  color: var(--bp-ink-2);
}
.status {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
  margin-left: auto;
  max-width: 100%;
}
.latest {
  display: inline-flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  align-items: center;
  gap: 4px 8px;
  font-size: 22px;
}
.when {
  font-size: 16px;
  color: var(--bp-ink-2);
  display: inline-flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  align-items: center;
  gap: 4px;
}
.when.today {
  color: var(--person);
  font-weight: 600;
}
.edit {
  width: 100%;
  border-top: 1px solid var(--bp-line);
  text-decoration: none;
  color: var(--person);
  font-weight: 600;
}
.add {
  --van-button-default-background: var(--bp-card);
}
.hint {
  margin: 8px 4px 24px;
}
.join-title {
  font-size: 22px;
}
.join-desc {
  margin: 6px 0 16px;
  font-size: 17px;
  color: var(--bp-ink-2);
}
.invite {
  padding: 0;
  margin-bottom: 0;
}
.invite summary {
  list-style: none;
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 60px;
  padding: 0 16px;
  font-size: 18px;
  font-weight: 600;
  cursor: pointer;
}
.invite summary::-webkit-details-marker {
  display: none;
}
.chevron {
  margin-left: auto;
  color: var(--bp-ink-2);
  transition: transform 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}
.invite[open] .chevron {
  transform: rotate(180deg);
}
.invite-body {
  padding: 0 16px 12px;
  text-align: center;
}
.code {
  font-size: 44px;
  font-weight: 700;
  letter-spacing: 6px;
  font-variant-numeric: tabular-nums;
  color: var(--bp-ink);
  margin: 2px 0 6px;
}
.reset {
  margin-top: 6px;
}
.leave {
  display: flex;
  justify-content: center;
  margin-top: 8px;
}
.dialog-body {
  padding: 16px 20px 8px;
}
.dialog-input {
  width: 100%;
  height: 56px;
  border: 2px solid var(--bp-line-strong);
  border-radius: 12px;
  padding: 0 14px;
  font-size: 20px;
  font-family: inherit;
  background: #fff;
  color: var(--bp-ink);
}
.dialog-input:focus {
  outline: none;
  border-color: var(--bp-accent);
}
.code-input {
  text-align: center;
  font-size: 30px;
  letter-spacing: 8px;
  font-weight: 700;
}
@media (prefers-reduced-motion: reduce) {
  .chevron {
    transition: none;
  }
}
</style>
