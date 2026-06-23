<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const PALETTE = [
  '#6366f1', '#8b5cf6', '#06b6d4', '#10b981',
  '#f59e0b', '#ef4444', '#ec4899', '#3b82f6',
  '#64748b', '#0ea5e9', '#84cc16', '#f97316',
]

const props = defineProps<{ color: string }>()
const emit = defineEmits<{ pick: [color: string] }>()

const open = ref(false)
const btnRef = ref<HTMLButtonElement | null>(null)
const popStyle = ref<{ top: string; left: string }>({ top: '0px', left: '0px' })

function toggle(e: MouseEvent) {
  e.stopPropagation()
  if (!open.value) {
    const rect = btnRef.value!.getBoundingClientRect()
    popStyle.value = {
      top: `${rect.bottom + 6}px`,
      left: `${Math.min(rect.left, window.innerWidth - 172)}px`,
    }
  }
  open.value = !open.value
}

function pick(color: string) {
  emit('pick', color)
  open.value = false
}

function onOutside(e: MouseEvent) {
  if (!btnRef.value?.contains(e.target as Node)) open.value = false
}

onMounted(() => window.addEventListener('mousedown', onOutside))
onUnmounted(() => window.removeEventListener('mousedown', onOutside))
</script>

<template>
  <button
    ref="btnRef"
    class="ncp-trigger"
    title="Change color"
    @mousedown.stop
    @click.stop="toggle"
  >
    <svg viewBox="0 0 12 12" fill="none">
      <circle cx="6" cy="6" r="4.5" stroke="rgba(255,255,255,0.85)" stroke-width="1.2"/>
      <circle cx="6" cy="6" r="2" fill="rgba(255,255,255,0.85)"/>
    </svg>
  </button>

  <Teleport to="body">
    <div
      v-if="open"
      class="ncp-popover"
      :style="{ top: popStyle.top, left: popStyle.left }"
      @mousedown.stop
      @click.stop
    >
      <div class="ncp-swatches">
        <button
          v-for="c in PALETTE"
          :key="c"
          class="ncp-swatch"
          :style="{ background: c }"
          :class="{ active: c === color }"
          :title="c"
          @click="pick(c)"
        />
      </div>
      <div class="ncp-custom-row">
        <label class="ncp-custom-label">
          Custom
          <input
            type="color"
            :value="color"
            class="ncp-color-input"
            @change="pick(($event.target as HTMLInputElement).value)"
          />
        </label>
        <span class="ncp-hex">{{ color }}</span>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.ncp-trigger {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.18);
  border: 1px solid rgba(255, 255, 255, 0.2);
  opacity: 0;
  transition: opacity 0.12s, background 0.12s;
  flex-shrink: 0;
}
.ncp-trigger svg { width: 12px; height: 12px; }
.ncp-trigger:hover { background: rgba(0, 0, 0, 0.32); opacity: 1 !important; }

/* parent's :hover reveals it — applied via deep in each card */

.ncp-popover {
  position: fixed;
  z-index: 9999;
  background: #1c2128;
  border: 1px solid #30363d;
  border-radius: 8px;
  padding: 10px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.6);
  width: 160px;
}

.ncp-swatches {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 5px;
  margin-bottom: 8px;
}

.ncp-swatch {
  width: 20px;
  height: 20px;
  border-radius: 4px;
  border: 2px solid transparent;
  transition: transform 0.08s, border-color 0.08s;
}
.ncp-swatch:hover { transform: scale(1.15); }
.ncp-swatch.active { border-color: white; }

.ncp-custom-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}

.ncp-custom-label {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 10px;
  color: #8b949e;
  cursor: pointer;
}

.ncp-color-input {
  width: 22px;
  height: 22px;
  border-radius: 4px;
  border: 1px solid #30363d;
  padding: 1px;
  background: transparent;
  cursor: pointer;
}

.ncp-hex {
  font-size: 9px;
  font-family: var(--font-mono, monospace);
  color: #6e7681;
}
</style>
