<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'

export interface MenuItem {
  label: string
  shortcut?: string
  danger?: boolean
  disabled?: boolean
  action: () => void
}

export type MenuSection = MenuItem | { divider: true }

const props = defineProps<{
  x: number
  y: number
  sections: MenuSection[]
}>()

const emit = defineEmits<{ close: [] }>()

const menuStyle = computed(() => ({
  left: `${Math.min(props.x, window.innerWidth - 200)}px`,
  top: `${Math.min(props.y, window.innerHeight - 20)}px`,
}))

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))

function isDivider(s: MenuSection): s is { divider: true } {
  return 'divider' in s
}
</script>

<template>
  <Teleport to="body">
    <!-- backdrop: click outside closes -->
    <div class="ctx-backdrop" @mousedown.self="emit('close')" @contextmenu.prevent.self="emit('close')">
      <div
        class="ctx-menu"
        :style="menuStyle"
        @mousedown.stop
        @contextmenu.prevent.stop
      >
        <template v-for="(item, i) in sections" :key="i">
          <div v-if="isDivider(item)" class="ctx-divider" />
          <button
            v-else
            class="ctx-item"
            :class="{ danger: item.danger, disabled: item.disabled }"
            :disabled="item.disabled"
            @click="item.action(); emit('close')"
          >
            <span class="ctx-label">{{ item.label }}</span>
            <span v-if="item.shortcut" class="ctx-shortcut">{{ item.shortcut }}</span>
          </button>
        </template>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.ctx-backdrop {
  position: fixed;
  inset: 0;
  z-index: 8000;
}

.ctx-menu {
  position: absolute;
  min-width: 192px;
  background: #1c2128;
  border: 1px solid #30363d;
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.6), 0 1px 0 rgba(255,255,255,0.04) inset;
  padding: 4px 0;
  animation: ctx-in 0.08s ease;
}

@keyframes ctx-in {
  from { opacity: 0; transform: scale(0.96) translateY(-4px); }
  to   { opacity: 1; transform: scale(1) translateY(0); }
}

.ctx-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 6px 12px;
  font-size: 12.5px;
  color: #c9d1d9;
  background: transparent;
  border: none;
  text-align: left;
  cursor: pointer;
  gap: 24px;
  transition: background 0.08s, color 0.08s;
}

.ctx-item:hover:not(:disabled) { background: rgba(88, 166, 255, 0.12); color: #e6edf3; }
.ctx-item.danger { color: #f85149; }
.ctx-item.danger:hover:not(:disabled) { background: rgba(248, 81, 73, 0.12); color: #ff7b72; }
.ctx-item.disabled { opacity: 0.35; cursor: not-allowed; }

.ctx-label { flex: 1; }

.ctx-shortcut {
  font-size: 10.5px;
  color: #6e7681;
  font-family: var(--font-mono, monospace);
  flex-shrink: 0;
}

.ctx-divider {
  height: 1px;
  background: #30363d;
  margin: 3px 0;
}
</style>
