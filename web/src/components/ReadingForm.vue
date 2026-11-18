<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, useId, watch } from 'vue'
import { showConfirmDialog, showToast } from 'vant'
import { api } from '../api'
import type { Reading, ReadingInput, ReadingItem } from '../api/types'
import ResultPanel from './ResultPanel.vue'
import MeasureTips from './MeasureTips.vue'
import {
  averageItems,
  checkItem,
  emptyRow,
  isUnusual,
  parseDigits,
  shouldAdvance,
  type Field,
  type ItemDraft,
  type RowDraft,
} from '../utils/bp'
import { formatDateTime } from '../utils/date'
import { useDraftsStore } from '../stores/drafts'
import { describeError, describeItemProblem } from '../i18n/describeError'
import { t } from '../i18n/zh'

const props = defineProps<{
  profileId: number
  ownerName: string
  isSelf: boolean
  initial?: Reading
  lastReading?: Reading | null
}>()
const emit = defineEmits<{ saved: [reading: Reading]; edit: [] }>()

const FIELDS: Field[] = ['systolic', 'diastolic', 'pulse']
const MAX_ROWS = 3
const UNDO_MS = 6000
// A tremor double-tap right after auto-advance would otherwise drop a stray digit into the next box.
const ADVANCE_GUARD_MS = 180

const uid = useId()
const drafts = useDraftsStore()
const isEdit = Boolean(props.initial)
const stored = isEdit ? null : drafts.get(props.profileId)

const toRow = (it: ReadingItem): RowDraft => ({
  systolic: String(it.systolic),
  diastolic: String(it.diastolic),
  pulse: it.pulse === null ? '' : String(it.pulse),
})

const rows = ref<RowDraft[]>(
  props.initial ? props.initial.items.map(toRow) : stored ? stored.rows.map((r) => ({ ...r })) : [emptyRow()],
)
// null means "now": the server stamps the time when saving.
const customTime = ref<Date | null>(
  props.initial ? new Date(props.initial.measuredAt) : stored?.time ? new Date(stored.time) : null,
)
const note = ref(props.initial?.note ?? stored?.note ?? '')
const showNote = ref(Boolean(note.value))
const saving = ref(false)
const problem = ref<{ row: number; field: Field; message: string } | null>(null)

if (!isEdit) {
  watch(
    [rows, note, customTime],
    () => {
      drafts.put(props.profileId, {
        rows: rows.value,
        note: note.value,
        time: customTime.value?.toISOString() ?? null,
      })
    },
    { deep: true },
  )
}

const items = computed<ItemDraft[]>(() =>
  rows.value.map((r) => ({
    systolic: parseDigits(r.systolic),
    diastolic: parseDigits(r.diastolic),
    pulse: parseDigits(r.pulse),
  })),
)
const average = computed(() => averageItems(items.value))
const completeCount = computed(() => items.value.filter((it) => it.systolic !== null && it.diastolic !== null).length)
const canAddRow = computed(() => rows.value.length < MAX_ROWS)

const saveText = computed(() => {
  if (isEdit) return t.record.saveEdit
  return props.isSelf ? t.record.saveSelf : t.record.saveFor(props.ownerName)
})

const lastText = computed(() => {
  if (isEdit) return ''
  const r = props.lastReading
  if (!r) return t.record.lastEmpty
  return t.record.last(formatDateTime(new Date(r.measuredAt)), `${r.systolic}/${r.diastolic}`)
})

// Flat list of inputs in visual order, used to move focus as the user types.
const inputs: (HTMLInputElement | null)[] = []
const inputIndex = (row: number, field: Field) => row * FIELDS.length + FIELDS.indexOf(field)
let guard = { idx: -1, until: 0 }

function setInputRef(el: unknown, idx: number) {
  inputs[idx] = (el as HTMLInputElement | null) ?? null
}

function onInput(row: number, field: Field, ev: Event) {
  const el = ev.target as HTMLInputElement
  const idx = inputIndex(row, field)
  if (idx === guard.idx && performance.now() < guard.until) {
    el.value = rows.value[row][field]
    guard.idx = -1
    return
  }
  const digits = el.value.replace(/\D/g, '').slice(0, 3)
  el.value = digits
  rows.value[row][field] = digits
  problem.value = null
  emit('edit')
  if (shouldAdvance(digits)) advanceFrom(idx, el)
}

function advanceFrom(idx: number, el: HTMLInputElement) {
  const next = inputs[idx + 1]
  if (!next) {
    el.blur()
    return
  }
  // Never jump into a box that already has a value; the user is probably correcting this one.
  if (next.value !== '') return
  next.focus()
  guard = { idx: idx + 1, until: performance.now() + ADVANCE_GUARD_MS }
}

function selectAll(ev: FocusEvent) {
  ;(ev.target as HTMLInputElement).select()
}

async function addRow() {
  if (rows.value.length >= MAX_ROWS) return
  rows.value.push(emptyRow())
  undo.value = null
  await nextTick()
  inputs[inputIndex(rows.value.length - 1, 'systolic')]?.focus()
}

const undo = ref<{ row: RowDraft; index: number } | null>(null)
let undoTimer: number | undefined

function removeRow(i: number) {
  const [row] = rows.value.splice(i, 1)
  inputs.length = rows.value.length * FIELDS.length
  problem.value = null
  undo.value = { row, index: i }
  window.clearTimeout(undoTimer)
  undoTimer = window.setTimeout(() => (undo.value = null), UNDO_MS)
}

function undoRemove() {
  const u = undo.value
  if (!u || rows.value.length >= MAX_ROWS) return
  rows.value.splice(u.index, 0, u.row)
  undo.value = null
  window.clearTimeout(undoTimer)
}

onBeforeUnmount(() => window.clearTimeout(undoTimer))

const fieldColor: Record<Field, string> = {
  systolic: 'var(--bp-systolic)',
  diastolic: 'var(--bp-diastolic)',
  pulse: 'var(--bp-pulse)',
}

// Time picker
const pickerOpen = ref(false)
const pickDate = ref<string[]>([])
const pickTime = ref<string[]>([])
const today = new Date()
const minDate = new Date(today.getFullYear() - 1, today.getMonth(), today.getDate())
const pad = (n: number) => String(n).padStart(2, '0')

const timeLabel = computed(() => (customTime.value ? formatDateTime(customTime.value) : t.record.now))

function openPicker() {
  const d = customTime.value ?? new Date()
  pickDate.value = [String(d.getFullYear()), pad(d.getMonth() + 1), pad(d.getDate())]
  pickTime.value = [pad(d.getHours()), pad(d.getMinutes())]
  pickerOpen.value = true
}

function confirmPicker() {
  const [y, m, d] = pickDate.value.map(Number)
  const [h, min] = pickTime.value.map(Number)
  const chosen = new Date(y, m - 1, d, h, min)
  customTime.value = chosen > new Date() ? null : chosen
  pickerOpen.value = false
}

function useNow() {
  customTime.value = null
  pickerOpen.value = false
}

function focusFirst() {
  const first = inputs.find((el) => el && el.value === '') ?? inputs[0]
  first?.focus()
  first?.scrollIntoView({ block: 'center', behavior: 'smooth' })
}

defineExpose({ focusFirst })

async function save() {
  for (let i = 0; i < items.value.length; i++) {
    const p = checkItem(items.value[i])
    if (p) {
      problem.value = { row: i, field: p.field, message: describeItemProblem(i, p, items.value.length) }
      const el = inputs[inputIndex(i, p.field)]
      el?.focus()
      el?.scrollIntoView({ block: 'center', behavior: 'smooth' })
      return
    }
  }
  const avg = average.value
  if (!avg) return

  if (isUnusual(avg)) {
    try {
      await showConfirmDialog({
        title: t.record.checkTitle,
        message: t.record.checkMessage,
        confirmButtonText: t.record.checkConfirm,
        cancelButtonText: t.record.checkCancel,
      })
    } catch {
      return
    }
  }

  const input: ReadingInput = {
    items: items.value as ReadingItem[],
    note: note.value,
    measuredAt: customTime.value?.toISOString(),
  }
  saving.value = true
  try {
    const saved = props.initial
      ? await api.updateReading(props.initial.id, input)
      : await api.createReading(props.profileId, input)
    if (!isEdit) {
      drafts.clear(props.profileId)
      reset()
    }
    emit('saved', saved)
  } catch (err) {
    showToast(describeError(err))
  } finally {
    saving.value = false
  }
}

function collapseNote() {
  note.value = ''
  showNote.value = false
}

function reset() {
  rows.value = [emptyRow()]
  customTime.value = null
  note.value = ''
  showNote.value = false
  problem.value = null
  undo.value = null
}
</script>

<template>
  <div class="reading-form">
    <div v-if="!isEdit" class="above">
      <p class="last">{{ lastText }}</p>
      <MeasureTips />
    </div>

    <div class="card entry">
      <div v-for="(row, i) in rows" :key="i" class="row">
        <div v-if="rows.length > 1" class="row-head">
          <span class="row-title">{{ t.record.nth(i + 1) }}</span>
          <button type="button" class="remove" :aria-label="t.record.removeNth(i + 1)" @click="removeRow(i)">
            <van-icon name="cross" aria-hidden="true" />
          </button>
        </div>
        <div class="fields">
          <label
            v-for="f in FIELDS"
            :key="f"
            class="field"
            :class="{ error: problem?.row === i && problem.field === f }"
          >
            <span class="field-label" :style="{ color: fieldColor[f] }">{{ t.bp[f] }}</span>
            <input
              :ref="(el) => setInputRef(el, inputIndex(i, f))"
              class="num"
              type="text"
              inputmode="numeric"
              pattern="[0-9]*"
              autocomplete="off"
              maxlength="3"
              :value="row[f]"
              :aria-label="t.record.fieldLabel(rows.length > 1 ? i + 1 : null, t.bp[f])"
              :aria-required="f === 'pulse'"
              :aria-invalid="problem?.row === i && problem.field === f"
              :aria-describedby="problem?.row === i ? `${uid}-err-${i}` : undefined"
              @focus="selectAll"
              @input="onInput(i, f, $event)"
            />
            <span class="unit" aria-hidden="true">{{ f === 'pulse' ? t.bp.bpm : t.bp.mmHg }}</span>
          </label>
        </div>
        <p v-if="problem?.row === i" :id="`${uid}-err-${i}`" class="row-error" role="alert">
          <van-icon name="warning-o" aria-hidden="true" />
          {{ problem.message }}
        </p>
      </div>

      <div v-if="undo && rows.length < MAX_ROWS" class="undo" role="status">
        <span>{{ t.record.removed(undo.index + 1) }}</span>
        <button type="button" class="undo-btn" @click="undoRemove">{{ t.common.undo }}</button>
      </div>

      <button v-if="canAddRow" type="button" class="add" @click="addRow">
        <van-icon name="plus" aria-hidden="true" />
        {{ t.record.addAnother }}
      </button>
    </div>

    <ResultPanel
      v-if="average"
      :label="t.record.draftLabel(completeCount)"
      :systolic="average.systolic"
      :diastolic="average.diastolic"
      :pulse="average.pulse"
    />

    <van-cell-group inset class="extra">
      <van-cell :title="t.record.time" :value="timeLabel" icon="clock-o" is-link @click="openPicker" />
      <van-cell v-if="!showNote" :title="t.record.showNote" icon="notes-o" is-link @click="showNote = true" />
      <van-field
        v-else
        v-model="note"
        class="boxed note-field"
        label-align="top"
        type="textarea"
        rows="2"
        autosize
        maxlength="500"
        :placeholder="t.record.notePlaceholder"
      >
        <template #label>
          <span class="note-head">
            <span>{{ t.record.note }}</span>
            <button v-if="!note.trim()" type="button" class="note-collapse" @click="collapseNote">
              {{ t.record.hideNote }}
            </button>
          </span>
        </template>
      </van-field>
    </van-cell-group>

    <div class="save-bar">
      <van-button class="save-btn" type="primary" size="large" block round :loading="saving" @click="save">
        {{ saveText }}
      </van-button>
    </div>

    <van-popup v-model:show="pickerOpen" position="bottom" round>
      <div class="picker-bar">
        <button type="button" class="picker-text" @click="pickerOpen = false">{{ t.common.cancel }}</button>
        <span class="picker-title">{{ t.record.time }}</span>
        <button type="button" class="picker-text confirm" @click="confirmPicker">{{ t.common.confirm }}</button>
      </div>
      <div class="picker-wheels">
        <div class="wheel">
          <van-date-picker v-model="pickDate" :show-toolbar="false" :min-date="minDate" :max-date="today" :visible-option-num="3" />
          <span class="mark date-mark" aria-hidden="true">-</span>
          <span class="mark date-mark second" aria-hidden="true">-</span>
        </div>
        <div class="wheel">
          <van-time-picker v-model="pickTime" :show-toolbar="false" :visible-option-num="3" />
          <span class="mark time-mark" aria-hidden="true">:</span>
        </div>
      </div>
      <van-button v-if="customTime" block size="large" class="now-btn" @click="useNow">
        {{ t.record.now }}
      </van-button>
    </van-popup>
  </div>
</template>

<style scoped>
.above {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  margin: 0 2px 6px;
  min-height: 44px;
}
.last {
  flex: 1;
  margin: 0;
  font-size: 17px;
  color: var(--bp-ink-2);
}
.entry {
  position: relative;
  padding: 14px 14px 10px;
}
.row + .row {
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--bp-line);
}
.row-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.row-title {
  font-size: 18px;
  font-weight: 700;
}
.remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  padding: 0;
  border: 1.5px solid var(--bp-line-strong);
  border-radius: 50%;
  background: var(--bp-card);
  color: var(--bp-ink-2);
  font: inherit;
  font-size: 18px;
}
.fields {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}
.field {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  min-width: 0;
}
.field-label {
  font-size: 19px;
  font-weight: 700;
}
.num {
  width: 100%;
  height: 68px;
  border: 2px solid var(--bp-line-strong);
  border-radius: 14px;
  text-align: center;
  font-size: 34px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: var(--bp-ink);
  background: #fff;
  font-family: inherit;
  -webkit-appearance: none;
  appearance: none;
}
.num:focus {
  outline: none;
  border-color: var(--person);
  box-shadow: 0 0 0 4px var(--person-tint);
}
.field.error .num {
  border-color: var(--bp-high);
  background: var(--bp-high-bg);
}
.unit {
  font-size: 15px;
  color: var(--bp-ink-2);
}
.row-error {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 10px 0 0;
  font-size: 17px;
  font-weight: 600;
  color: var(--bp-high);
}
.undo {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-top: 12px;
  padding: 4px 4px 4px 14px;
  border-radius: 12px;
  background: var(--bp-paper);
  font-size: 17px;
}
.undo-btn {
  min-height: 44px;
  padding: 0 16px;
  border: none;
  background: none;
  font: inherit;
  font-weight: 700;
  color: var(--person);
}
.add {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  min-height: 52px;
  margin-top: 12px;
  border: 1.5px dashed var(--bp-line-strong);
  border-radius: 14px;
  background: transparent;
  color: var(--person);
  font: inherit;
  font-size: 18px;
  font-weight: 600;
}
.add:active {
  background: var(--person-tint);
}
.extra {
  margin: 0 0 16px;
  box-shadow: var(--bp-shadow);
}
.note-field :deep(.van-field__label) {
  width: 100%;
  margin-right: 0;
}
.note-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}
.note-collapse {
  border: 0;
  background: none;
  padding: 0 0 0 12px;
  font: inherit;
  font-size: 16px;
  font-weight: 600;
  color: var(--bp-ink-2);
  cursor: pointer;
}
.save-bar {
  padding-top: 4px;
}
.save-btn {
  --van-button-primary-background: var(--person);
  --van-button-primary-border-color: var(--person);
  font-weight: 700;
}
.picker-bar {
  display: grid;
  grid-template-columns: 72px 1fr 72px;
  align-items: center;
  min-height: 52px;
  padding: 0 16px;
}
.picker-title {
  font-size: 18px;
  font-weight: 700;
  text-align: center;
}
.picker-text {
  width: 72px;
  border: none;
  background: none;
  font: inherit;
  font-size: 17px;
  font-weight: 600;
  color: var(--bp-ink-2);
  min-height: 44px;
  padding: 0;
  text-align: left;
}
.picker-text.confirm {
  text-align: right;
  color: var(--person);
}
.picker-wheels {
  display: flex;
  flex-direction: column;
}
.wheel {
  position: relative;
}
.wheel + .wheel {
  border-top: 1px solid var(--bp-line);
}
.mark {
  position: absolute;
  top: 50%;
  z-index: 3;
  transform: translate(-50%, -50%);
  pointer-events: none;
  font-size: 20px;
  font-weight: 700;
  line-height: 1;
  color: var(--bp-ink);
}
.date-mark {
  left: 33.33%;
}
.date-mark.second {
  left: 66.67%;
}
.time-mark {
  left: 50%;
}
.wheel:last-child :deep(.van-picker__columns) {
  justify-content: center;
}
.wheel:last-child :deep(.van-picker-column) {
  flex: 0 0 88px;
}
.now-btn {
  border: none;
  border-top: 1px solid var(--bp-line);
}

@media (max-width: 360px) {
  .fields {
    grid-template-columns: 1fr;
  }
  .field {
    display: grid;
    grid-template-columns: 1fr auto;
    grid-template-areas:
      'label unit'
      'num num';
    align-items: baseline;
    gap: 4px 8px;
  }
  .field-label {
    grid-area: label;
  }
  .unit {
    grid-area: unit;
  }
  .num {
    grid-area: num;
    height: 60px;
  }
}
</style>
