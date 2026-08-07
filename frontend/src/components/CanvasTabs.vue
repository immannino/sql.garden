<script setup lang="ts">
import { ref, nextTick, onMounted, onUnmounted } from 'vue'

interface Tab { id: string; name: string }

const props = defineProps<{
  canvases: Tab[]
  activeId: string
}>()

const emit = defineEmits<{
  switch:  [id: string]
  add:     []
  remove:  [id: string]
  rename:  [id: string, name: string]
  export:  [id: string]
}>()

const renamingId  = ref<string | null>(null)
const renameValue = ref('')

function startRename(tab: Tab) {
  renamingId.value  = tab.id
  renameValue.value = tab.name
  nextTick(() => {
    const input = document.querySelector<HTMLInputElement>('.tab-rename-input')
    input?.focus()
    input?.select()
  })
}

function commitRename() {
  if (renamingId.value) {
    emit('rename', renamingId.value, renameValue.value)
    renamingId.value = null
  }
}

function onRenameKey(e: KeyboardEvent) {
  if (e.key === 'Enter')  { e.preventDefault(); commitRename() }
  if (e.key === 'Escape') { renamingId.value = null }
}

// ── Tab context menu ─────────────────────────────────────────────────────────
const ctxMenu = ref<{ id: string; x: number; y: number } | null>(null)

function showContextMenu(tab: Tab, e: MouseEvent) {
  ctxMenu.value = { id: tab.id, x: e.clientX, y: e.clientY }
}

function closeCtxMenu() { ctxMenu.value = null }

function ctxRename() {
  const tab = props.canvases.find(c => c.id === ctxMenu.value?.id)
  closeCtxMenu()
  if (tab) startRename(tab)
}

function ctxExport() {
  if (ctxMenu.value) emit('export', ctxMenu.value.id)
  closeCtxMenu()
}

function ctxClose() {
  if (ctxMenu.value && props.canvases.length > 1) emit('remove', ctxMenu.value.id)
  closeCtxMenu()
}

onMounted(() => document.addEventListener('click', closeCtxMenu, true))
onUnmounted(() => document.removeEventListener('click', closeCtxMenu, true))
</script>

<template>
  <div class="tab-bar">
    <div
      v-for="(tab, index) in canvases"
      :key="tab.id"
      class="canvas-tab"
      :class="{ active: tab.id === activeId }"
      :title="tab.name"
      @click="renamingId !== tab.id && emit('switch', tab.id)"
      @dblclick.stop="startRename(tab)"
      @contextmenu.prevent="showContextMenu(tab, $event)"
    >
      <!-- Close button — left side, fades in on hover -->
      <button
        class="tab-close"
        :style="{ visibility: canvases.length > 1 ? undefined : 'hidden' }"
        title="Close tab"
        @click.stop="canvases.length > 1 && emit('remove', tab.id)"
        @mousedown.stop
        @dblclick.stop
      >
        <svg viewBox="0 0 8 8" fill="none">
          <path d="M1.5 1.5l5 5M6.5 1.5l-5 5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
        </svg>
      </button>

      <!-- Tab name or inline rename input -->
      <input
        v-if="renamingId === tab.id"
        class="tab-rename-input"
        :value="renameValue"
        @input="renameValue = ($event.target as HTMLInputElement).value"
        @blur="commitRename"
        @keydown="onRenameKey"
        @click.stop
        @mousedown.stop
      />
      <span v-else class="tab-name">{{ tab.name }}</span>

      <!-- Shortcut hint — right side, always visible for tabs 1–9 -->
      <span v-if="index < 9" class="tab-shortcut">⌘{{ index + 1 }}</span>
    </div>

    <button class="tab-add" title="New canvas (⌘T)" @click="emit('add')" @dblclick.stop>
      <svg viewBox="0 0 10 10" fill="none">
        <path d="M5 1v8M1 5h8" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
      </svg>
    </button>
  </div>

  <Teleport to="body">
    <div
      v-if="ctxMenu"
      class="tab-ctx-menu"
      :style="{ left: ctxMenu.x + 'px', top: ctxMenu.y + 'px' }"
      @click.stop
    >
      <button class="ctx-item" @click="ctxRename">Rename</button>
      <button class="ctx-item" @click="ctxExport">Export tab as .sql.garden.json</button>
      <div class="ctx-divider" />
      <button class="ctx-item" :disabled="canvases.length <= 1" @click="ctxClose">Close tab</button>
    </div>
  </Teleport>
</template>

<style scoped>
.tab-bar {
  display: flex;
  align-items: stretch;
  height: 32px;
  background: var(--surface-1);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: none;
  --wails-draggable: no-drag;
}
.tab-bar::-webkit-scrollbar { display: none; }

.canvas-tab {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 8px;
  min-width: 80px;
  max-width: 200px;
  height: 100%;
  border-right: 1px solid var(--border);
  cursor: pointer;
  color: var(--text-secondary);
  font-size: 11px;
  flex-shrink: 0;
  position: relative;
  user-select: none;
  transition: background 0.1s;
}
.canvas-tab:hover { background: var(--surface-2); }
.canvas-tab.active {
  background: var(--surface-0, var(--canvas-bg));
  color: var(--text-primary);
  box-shadow: inset 0 -2px 0 var(--accent);
}

/* Close button — left, slides in on hover */
.tab-close {
  flex-shrink: 0;
  width: 14px;
  height: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 3px;
  color: var(--text-muted);
  background: none;
  border: none;
  padding: 0;
  opacity: 0;
  cursor: pointer;
  transition: opacity 0.15s, background 0.1s;
}
.canvas-tab:hover .tab-close { opacity: 1; }
.tab-close:hover { background: var(--surface-3, var(--surface-2)); color: var(--text-primary); }
.tab-close svg { width: 8px; height: 8px; }

.tab-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

/* Shortcut hint — right, always shown */
.tab-shortcut {
  flex-shrink: 0;
  font-size: 9px;
  color: var(--text-muted);
  font-family: var(--font-mono, monospace);
  opacity: 0.6;
  letter-spacing: 0;
  margin-left: 0.5rem;
}
.canvas-tab.active .tab-shortcut { opacity: 0.8; }

.tab-rename-input {
  flex: 1;
  min-width: 0;
  background: transparent;
  border: 1px solid var(--accent);
  border-radius: 3px;
  color: var(--text-primary);
  font-size: 11px;
  font-family: inherit;
  padding: 1px 4px;
  outline: none;
}

.tab-add {
  flex-shrink: 0;
  width: 32px;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  background: none;
  border: none;
  border-right: 1px solid var(--border);
  cursor: pointer;
  transition: background 0.1s, color 0.1s;
}
.tab-add:hover { background: var(--surface-2); color: var(--text-primary); }
.tab-add svg { width: 10px; height: 10px; }

/* Tab context menu — teleported to body so it's not clipped by the tab bar */
.tab-ctx-menu {
  position: fixed;
  z-index: 9999;
  background: var(--surface-1);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 4px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.18);
  min-width: 200px;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.ctx-item {
  display: block;
  width: 100%;
  text-align: left;
  background: none;
  border: none;
  border-radius: 4px;
  padding: 6px 10px;
  font-size: 12px;
  color: var(--text-primary);
  cursor: pointer;
  white-space: nowrap;
}
.ctx-item:hover { background: var(--surface-2); }
.ctx-item:disabled { color: var(--text-muted); cursor: default; }
.ctx-item:disabled:hover { background: none; }

.ctx-divider {
  height: 1px;
  background: var(--border);
  margin: 3px 0;
}
</style>
