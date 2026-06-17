<script setup lang="ts">
import { ref, nextTick } from 'vue'
import type { SectionNode } from '../stores/schema'
import { useSchemaStore } from '../stores/schema'

const props = defineProps<{ node: SectionNode; selected?: boolean }>()
const emit = defineEmits<{
  dragStart: [{ id: string; mouseX: number; mouseY: number; shiftKey: boolean }]
  resizeStart: [{ id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }]
}>()

const schemaStore = useSchemaStore()

function onMouseDown(e: MouseEvent) {
  if (e.button !== 0) return
  e.stopPropagation()
  emit('dragStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY, shiftKey: e.shiftKey })
}

function startResize(e: MouseEvent, direction: 'e' | 's' | 'se') {
  e.stopPropagation()
  emit('resizeStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY, startW: props.node.w, startH: props.node.h, direction })
}

// ── Delete ────────────────────────────────────────────────────────────────────
const deleteConfirm = ref(false)
let deleteTimer: ReturnType<typeof setTimeout> | null = null

function onDeleteClick(e: MouseEvent) {
  e.stopPropagation()
  if (!deleteConfirm.value) {
    deleteConfirm.value = true
    deleteTimer = setTimeout(() => { deleteConfirm.value = false }, 3000)
  } else {
    if (deleteTimer) clearTimeout(deleteTimer)
    schemaStore.removeNode(props.node.id)
  }
}

// ── Rename ────────────────────────────────────────────────────────────────────
const isRenaming = ref(false)
const renameValue = ref('')
const renameInputRef = ref<HTMLInputElement | null>(null)

function startRename(e: MouseEvent) {
  e.stopPropagation()
  isRenaming.value = true
  renameValue.value = props.node.name
  nextTick(() => renameInputRef.value?.select())
}

function commitRename() {
  const v = renameValue.value.trim()
  isRenaming.value = false
  if (v && v !== props.node.name) schemaStore.renameNode(props.node.id, v)
}

function onRenameKey(e: KeyboardEvent) {
  if (e.key === 'Enter') { e.preventDefault(); commitRename() }
  if (e.key === 'Escape') { isRenaming.value = false }
}
</script>

<template>
  <div
    class="section-card"
    :class="{ selected }"
    :style="{
      left: `${node.x}px`,
      top: `${node.y}px`,
      width: `${node.w}px`,
      height: `${node.h}px`,
      '--section-color': node.color,
    }"
    @mousedown="onMouseDown"
  >
    <!-- Label bar at top-left -->
    <div class="section-label-bar" @mousedown.stop>
      <input
        v-if="isRenaming"
        ref="renameInputRef"
        v-model="renameValue"
        class="section-label-input"
        @keydown="onRenameKey"
        @blur="commitRename"
        @mousedown.stop
      />
      <span
        v-else
        class="section-label"
        title="Double-click to rename"
        @dblclick="startRename"
      >{{ node.name }}</span>

      <button
        class="section-delete-btn"
        :class="{ confirming: deleteConfirm }"
        :title="deleteConfirm ? 'Click again to delete' : 'Delete section'"
        @click.stop="onDeleteClick"
      >
        <span v-if="deleteConfirm" style="font-size:9px;font-weight:700;white-space:nowrap">Delete?</span>
        <svg v-else viewBox="0 0 12 12" fill="none" style="width:10px;height:10px">
          <path d="M2 3.5h8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
          <path d="M4.5 3.5V2.5h3v1" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M3.5 3.5l.7 6h3.6l.7-6" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>
    </div>

    <!-- Resize handles -->
    <div class="rh-e"  @mousedown.stop="startResize($event, 'e')" />
    <div class="rh-s"  @mousedown.stop="startResize($event, 's')" />
    <div class="rh-se" @mousedown.stop="startResize($event, 'se')">
      <svg viewBox="0 0 8 8" fill="none">
        <line x1="7" y1="1" x2="1" y2="7" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
        <line x1="7" y1="4" x2="4" y2="7" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
      </svg>
    </div>
  </div>
</template>

<style scoped>
.section-card {
  position: absolute;
  border: 2px solid var(--section-color, #6366f1);
  border-radius: 12px;
  background: color-mix(in srgb, var(--section-color, #6366f1) 6%, transparent);
  cursor: grab;
  user-select: none;
  /* Always behind other canvas nodes */
  z-index: 0;
  transition: border-color 0.15s;
}

.section-card:active { cursor: grabbing; }

.section-card.selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px rgba(88, 166, 255, 0.3);
}

/* Label bar */
.section-label-bar {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  position: absolute;
  top: -1px;
  left: 12px;
  height: 22px;
  background: var(--section-color, #6366f1);
  border-radius: 0 0 6px 6px;
  padding: 0 8px 0 10px;
  cursor: default;
}

.section-label {
  font-size: 11px;
  font-weight: 600;
  color: #fff;
  letter-spacing: 0.03em;
  white-space: nowrap;
  cursor: text;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.section-label-input {
  font-size: 11px;
  font-weight: 600;
  font-family: inherit;
  color: #fff;
  background: rgba(0,0,0,0.25);
  border: 1px solid rgba(255,255,255,0.5);
  border-radius: 3px;
  padding: 1px 4px;
  outline: none;
  width: 120px;
}

.section-delete-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  background: transparent;
  border: none;
  border-radius: 3px;
  color: rgba(255,255,255,0.6);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.15s, background 0.15s;
  padding: 0 3px;
}

.section-label-bar:hover .section-delete-btn { opacity: 1; }
.section-delete-btn.confirming {
  opacity: 1;
  background: rgba(248,81,73,0.4);
  color: #fff;
}
.section-delete-btn:hover { background: rgba(248,81,73,0.3); color: #fff; opacity: 1; }

/* Resize handles */
.rh-e, .rh-s, .rh-se { position: absolute; opacity: 0; transition: opacity 0.15s; }

.rh-e {
  right: -4px; top: 24px; bottom: 16px; width: 8px;
  cursor: ew-resize;
}
.rh-s {
  bottom: -4px; left: 16px; right: 16px; height: 8px;
  cursor: ns-resize;
}
.rh-se {
  right: 0; bottom: 0; width: 20px; height: 20px;
  cursor: se-resize;
  display: flex; align-items: center; justify-content: center;
  color: var(--section-color, #6366f1);
  border-radius: 0 0 10px 0;
}
.rh-se svg { width: 8px; height: 8px; }

.section-card:hover .rh-e,
.section-card:hover .rh-s,
.section-card:hover .rh-se,
.section-card.selected .rh-e,
.section-card.selected .rh-s,
.section-card.selected .rh-se { opacity: 1; }
</style>
