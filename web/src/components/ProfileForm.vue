<script setup lang="ts">
import { reactive, ref } from 'vue'
import type { Gender, Profile, ProfileInput } from '../api/types'
import { t } from '../i18n/zh'

const props = defineProps<{ initial?: Profile | null; submitText: string; busy?: boolean }>()
const emit = defineEmits<{ submit: [input: ProfileInput] }>()

const form = reactive({
  displayName: props.initial?.displayName ?? '',
  gender: (props.initial?.gender ?? '') as Gender,
  birthYear: props.initial?.birthYear ? String(props.initial.birthYear) : '',
  medicalHistory: props.initial?.medicalHistory ?? '',
  medications: props.initial?.medications ?? '',
})
const nameError = ref('')

function submit() {
  if (!form.displayName.trim()) {
    nameError.value = t.errors.fields.displayName + t.errors.fieldSeparator + t.errors.reasons.required
    return
  }
  nameError.value = ''
  const year = Number(form.birthYear)
  emit('submit', {
    displayName: form.displayName.trim(),
    gender: form.gender,
    birthYear: form.birthYear && Number.isInteger(year) ? year : null,
    medicalHistory: form.medicalHistory,
    medications: form.medications,
  })
}
</script>

<template>
  <form @submit.prevent="submit">
    <van-cell-group inset class="group">
      <slot name="leading" />
      <van-field
        v-model="form.displayName"
        class="boxed mark-after"
        :label="t.profile.displayName"
        label-align="top"
        required
        :placeholder="t.profile.displayNamePlaceholder"
        :error-message="nameError"
        maxlength="20"
        clearable
      />
      <van-field :label="t.profile.gender" label-align="top">
        <template #input>
          <van-radio-group v-model="form.gender" direction="horizontal" icon-size="24px" class="genders">
            <van-radio name="male">{{ t.profile.male }}</van-radio>
            <van-radio name="female">{{ t.profile.female }}</van-radio>
            <van-radio name="">{{ t.profile.unknown }}</van-radio>
          </van-radio-group>
        </template>
      </van-field>
      <van-field
        v-model="form.birthYear"
        class="boxed"
        type="digit"
        :label="t.profile.birthYear"
        label-align="top"
        :placeholder="t.profile.birthYearPlaceholder"
        maxlength="4"
      />
      <van-field
        v-model="form.medicalHistory"
        class="boxed"
        :label="t.profile.medicalHistory"
        label-align="top"
        type="textarea"
        rows="2"
        autosize
        maxlength="2000"
        :placeholder="t.profile.medicalHistoryPlaceholder"
      />
      <van-field
        v-model="form.medications"
        class="boxed"
        :label="t.profile.medications"
        label-align="top"
        type="textarea"
        rows="2"
        autosize
        maxlength="2000"
        :placeholder="t.profile.medicationsPlaceholder"
      />
    </van-cell-group>
    <van-button class="submit" type="primary" size="large" block round native-type="submit" :loading="busy">
      {{ submitText }}
    </van-button>
  </form>
</template>

<style scoped>
.group {
  --van-radio-checked-icon-color: var(--person);
  --van-field-required-mark-color: var(--bp-high);
  margin: 0 0 16px;
  box-shadow: var(--bp-shadow);
}
.group :deep(.van-field__label) {
  font-weight: 600;
  margin-bottom: 6px;
}
.submit {
  --van-button-primary-background: var(--person);
  --van-button-primary-border-color: var(--person);
  font-weight: 700;
}
.genders {
  gap: 4px 20px;
}
:deep(.van-radio) {
  font-size: 18px;
  min-height: 44px;
  margin-right: 0;
}
</style>
