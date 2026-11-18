<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showConfirmDialog, showSuccessToast, showToast } from 'vant'
import { api } from '../api'
import type { Reading } from '../api/types'
import ResultPanel from './ResultPanel.vue'
import { formatDay, formatTime } from '../utils/date'
import { periodLabel } from '../utils/period'
import { describeError } from '../i18n/describeError'
import { t } from '../i18n/zh'

const props = defineProps<{ reading: Reading | null; canWrite: boolean }>()
const show = defineModel<boolean>('show', { required: true })
const emit = defineEmits<{ changed: [] }>()

const router = useRouter()
const deleting = ref(false)
const when = computed(() => {
  const r = props.reading
  if (!r) return ''
  const d = new Date(r.measuredAt)
  return `${formatDay(d)} ${periodLabel(r.period)} ${formatTime(d)}`
})

function edit() {
  if (!props.reading) return
  show.value = false
  router.push(`/readings/${props.reading.id}/edit`)
}

async function remove() {
  if (!props.reading) return
  try {
    await showConfirmDialog({ message: t.detail.deleteConfirm, confirmButtonText: t.common.delete, cancelButtonText: t.common.cancel })
  } catch {
    return
  }
  deleting.value = true
  try {
    await api.deleteReading(props.reading.id)
    showSuccessToast(t.common.deleted)
    show.value = false
    emit('changed')
  } catch (err) {
    showToast(describeError(err))
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <van-popup v-model:show="show" position="bottom" round :style="{ maxHeight: '88vh' }" :aria-label="t.detail.title">
    <div v-if="reading" class="detail">
      <h2 class="title">{{ when }}</h2>

      <ResultPanel
        :label="t.record.draftLabel(reading.items.length)"
        :systolic="reading.systolic"
        :diastolic="reading.diastolic"
        :pulse="reading.pulse"
      />

      <ul v-if="reading.items.length > 1" class="items">
        <li v-for="(it, i) in reading.items" :key="i">
          <span class="nth">{{ t.detail.nth(i + 1) }}</span>
          <span class="item-bp">{{ it.systolic }}/{{ it.diastolic }}</span>
          <span v-if="it.pulse !== null" class="item-pulse">{{ t.bp.pulse }} {{ it.pulse }}</span>
        </li>
      </ul>

      <div v-if="reading.note" class="note">
        <div class="muted">{{ t.detail.note }}</div>
        <div>{{ reading.note }}</div>
      </div>

      <div class="actions" :class="{ single: !canWrite }">
        <van-button v-if="canWrite" type="primary" size="large" round icon="edit" class="edit-btn" @click="edit">
          {{ t.common.edit }}
        </van-button>
        <van-button size="large" round @click="show = false">{{ t.common.close }}</van-button>
      </div>
      <div v-if="canWrite" class="danger-zone">
        <button type="button" class="text-btn danger" :disabled="deleting" @click="remove">
          <van-icon name="delete-o" aria-hidden="true" />
          {{ t.detail.deleteButton }}
        </button>
      </div>
    </div>
  </van-popup>
</template>

<style scoped>
.detail {
  padding: 22px 18px calc(16px + env(safe-area-inset-bottom));
}
.title {
  font-size: 21px;
  font-weight: 700;
  margin-bottom: 12px;
}
.items {
  list-style: none;
  margin: 0 0 14px;
  padding: 0;
  border-radius: 12px;
  border: 1px solid var(--bp-line);
}
.items li {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding: 10px 14px;
  font-size: 18px;
}
.items li + li {
  border-top: 1px solid var(--bp-line);
}
.nth {
  color: var(--bp-ink-2);
}
.item-bp {
  font-weight: 700;
}
.item-pulse {
  margin-left: auto;
  color: var(--bp-ink-2);
}
.note {
  margin-bottom: 14px;
  background: var(--bp-paper);
  border-radius: 12px;
  padding: 10px 14px;
  font-size: 17px;
  white-space: pre-wrap;
}
.actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-top: 8px;
}
.actions.single {
  grid-template-columns: 1fr;
}
.edit-btn {
  --van-button-primary-background: var(--person);
  --van-button-primary-border-color: var(--person);
}
.danger-zone {
  display: flex;
  justify-content: center;
  margin-top: 18px;
  padding-top: 6px;
  border-top: 1px solid var(--bp-line);
}
</style>
