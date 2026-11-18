<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { showToast } from 'vant'
import type { Profile } from '../api/types'
import { useProfilesStore } from '../stores/profiles'
import { useDraftsStore } from '../stores/drafts'
import { initialOf, personStyle } from '../utils/person'
import { t } from '../i18n/zh'

const props = defineProps<{ writableOnly?: boolean }>()

const profiles = useProfilesStore()
const drafts = useDraftsStore()

const scroller = ref<HTMLElement | null>(null)
const canLeft = ref(false)
const canRight = ref(false)
const people = computed(() =>
  props.writableOnly ? profiles.list.filter((p) => p.canWrite) : profiles.list,
)
const visible = computed(() => people.value.length > 1)

function roleOf(p: Profile): string {
  if (p.isSelf) return t.people.self
  if (p.isManaged) return p.canWrite ? t.people.managedByMe : t.people.managedBy(p.managerName ?? '')
  return t.people.familyReadonly
}

function pick(p: Profile) {
  const leaving = profiles.current
  if (!leaving || leaving.id === p.id) return
  profiles.select(p.id)
  if (drafts.has(leaving.id)) showToast(t.people.draftKept(leaving.isSelf ? t.people.self : leaving.displayName))
}

function updateEdges() {
  const el = scroller.value
  if (!el) return
  canLeft.value = el.scrollLeft > 4
  canRight.value = el.scrollLeft + el.clientWidth < el.scrollWidth - 4
}

// Scroll the chip row only. scrollIntoView would also move the page.
function revealCurrent() {
  const el = scroller.value
  const active = el?.querySelector<HTMLElement>('.person.active')
  if (!el || !active) return
  const left = active.offsetLeft - (el.clientWidth - active.offsetWidth) / 2
  el.scrollTo({ left: Math.max(0, left), behavior: 'smooth' })
  updateEdges()
}

watch(
  () => profiles.currentId,
  async () => {
    await nextTick()
    revealCurrent()
  },
)

let observer: ResizeObserver | null = null
onMounted(() => {
  revealCurrent()
  scroller.value?.addEventListener('scroll', updateEdges, { passive: true })
  if (scroller.value) {
    observer = new ResizeObserver(updateEdges)
    observer.observe(scroller.value)
  }
})
onBeforeUnmount(() => {
  scroller.value?.removeEventListener('scroll', updateEdges)
  observer?.disconnect()
})
</script>

<template>
  <nav
    v-if="visible"
    ref="scroller"
    class="people"
    :class="{ 'can-left': canLeft, 'can-right': canRight }"
    :aria-label="t.people.pickerLabel"
  >
    <button
      v-for="p in people"
      :key="p.id"
      type="button"
      class="person"
      :class="{ active: p.id === profiles.currentId }"
      :style="personStyle(p.id)"
      :aria-pressed="p.id === profiles.currentId"
      :aria-label="`${p.displayName}，${roleOf(p)}`"
      @click="pick(p)"
    >
      <span class="avatar" aria-hidden="true">{{ initialOf(p.displayName) }}</span>
      <span class="who">
        <span class="name">{{ p.displayName }}</span>
        <span class="role">{{ roleOf(p) }}</span>
      </span>
      <span v-if="drafts.has(p.id) && p.id !== profiles.currentId" class="draft-dot">
        <span class="sr-only">{{ t.people.hasDraft }}</span>
      </span>
    </button>
  </nav>
</template>

<style scoped>
.people {
  display: flex;
  gap: 10px;
  overflow-x: auto;
  margin: 0 -16px 18px;
  padding: 2px 16px 8px;
  scroll-padding-inline: 28px;
  scrollbar-width: none;
}
.people::-webkit-scrollbar {
  display: none;
}
.people.can-left {
  mask-image: linear-gradient(to right, transparent, #000 28px);
}
.people.can-right {
  mask-image: linear-gradient(to right, #000 calc(100% - 28px), transparent);
}
.people.can-left.can-right {
  mask-image: linear-gradient(to right, transparent, #000 28px, #000 calc(100% - 28px), transparent);
}
.person {
  position: relative;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 60px;
  padding: 6px 16px 6px 6px;
  border: 1.5px solid var(--bp-line);
  border-radius: 999px;
  background: var(--bp-card);
  color: var(--bp-ink);
  font: inherit;
  text-align: left;
}
.person:active {
  background: var(--bp-press);
}
.person.active {
  background: var(--person);
  border-color: var(--person);
  color: #fff;
}
.avatar {
  width: 42px;
  height: 42px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--person);
  color: #fff;
  font-size: 19px;
  font-weight: 700;
}
.person.active .avatar {
  background: #fff;
  color: var(--person);
}
.who {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.name {
  font-size: 18px;
  font-weight: 700;
  white-space: nowrap;
  line-height: 1.2;
}
.role {
  font-size: 15px;
  font-weight: 600;
  line-height: 1.2;
  color: var(--bp-ink-2);
  white-space: nowrap;
}
.person.active .role {
  color: #fff;
}
.draft-dot {
  position: absolute;
  top: 4px;
  right: 6px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--bp-accent);
  border: 2px solid var(--bp-card);
}
</style>
