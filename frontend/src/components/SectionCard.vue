<script setup lang="ts">
import { ref, nextTick } from 'vue'
import type { SectionNode } from '../stores/schema'
import { useSchemaStore } from '../stores/schema'
import NodeColorPicker from './NodeColorPicker.vue'
import { useContextMenu } from '../composables/useContextMenu'

const { openNodeMenu } = useContextMenu()

const props = defineProps<{ node: SectionNode; selected?: boolean }>()
const emit = defineEmits<{
  dragStart: [{ id: string; mouseX: number; mouseY: number; shiftKey: boolean }]
  resizeStart: [{ id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }]
  mosaicContents: [id: string]
  fitContents: [id: string]
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
    :data-node-id="node.id"
    @contextmenu.prevent.stop="openNodeMenu(node.id, $event.clientX, $event.clientY)"
    :style="{
      left: `${node.x}px`,
      top: `${node.y}px`,
      width: `${node.w}px`,
      height: `${node.h}px`,
      '--section-color': node.color,
    }"
    @mousedown="onMouseDown"
  >
    <!-- Label tab — sits above the top border -->
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
        @dblclick.stop="startRename"
      >{{ node.name }}</span>

      <div class="section-label-actions">
        <NodeColorPicker :color="node.color" @pick="schemaStore.setNodeColor(node.id, $event)" />
        <button
          class="section-icon-btn"
          title="Fit section to contents"
          @click.stop="emit('fitContents', node.id)"
        >
          <svg viewBox="0 0 12 12" fill="none">
            <path d="M1 4.5V1h3.5M7.5 1H11v3.5M11 7.5V11H7.5M4.5 11H1V7.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </button>
        <button
          class="section-icon-btn"
          title="Mosaic contents"
          @click.stop="emit('mosaicContents', node.id)"
        >
          <svg viewBox="0 0 12 12" fill="none">
            <rect x="1" y="1" width="4" height="4" rx="0.8" fill="currentColor" opacity="0.9"/>
            <rect x="7" y="1" width="4" height="4" rx="0.8" fill="currentColor" opacity="0.6"/>
            <rect x="1" y="7" width="4" height="4" rx="0.8" fill="currentColor" opacity="0.6"/>
            <rect x="7" y="7" width="4" height="4" rx="0.8" fill="currentColor" opacity="0.9"/>
          </svg>
        </button>
        <button
          class="section-icon-btn section-delete-btn"
          title="Delete section"
          @click.stop="schemaStore.removeNode(node.id)"
        >
          <svg viewBox="0 0 12 12" fill="none">
            <path d="M2 2l8 8M10 2l-8 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          </svg>
        </button>
      </div>
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
  border: 1.5px solid var(--section-color, #6366f1);
  border-radius: 8px;
  background: color-mix(in srgb, var(--section-color, #6366f1) 5%, transparent);
  cursor: grab;
  user-select: none;
  z-index: 0;
  /* The label tab sits above the border box — overflow:visible lets it render outside */
  overflow: visible;
}

.section-card:active { cursor: grabbing; }

.section-card.selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px rgba(88, 166, 255, 0.25);
}

/* Label pill — inside the section, top-left corner */
.section-label-bar {
  display: flex;
  align-items: center;
  gap: 4px;
  position: absolute;
  top: 8px;
  left: 8px;
  height: 22px;
  min-width: 40px;
  max-width: calc(100% - 16px);
  background: var(--section-color, #6366f1);
  border-radius: 6px;
  padding: 0 6px 0 10px;
  cursor: default;
}

.section-label {
  font-size: 11px;
  font-weight: 600;
  color: #fff;
  letter-spacing: 0.03em;
  white-space: nowrap;
  cursor: text;
  flex: 1;
  min-width: 0;
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
  flex: 1;
  min-width: 80px;
}

.section-label-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
  opacity: 0;
  transition: opacity 0.12s;
}
.section-label-bar:hover .section-label-actions { opacity: 1; }

.section-icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border: none;
  border-radius: 3px;
  background: transparent;
  color: rgba(255,255,255,0.75);
  cursor: pointer;
  padding: 0;
  transition: background 0.1s, color 0.1s;
}
.section-icon-btn svg { width: 10px; height: 10px; }
.section-icon-btn:hover { background: rgba(0,0,0,0.2); color: #fff; }
.section-delete-btn:hover { background: rgba(248,81,73,0.5); color: #fff; }

/* Resize handles */
.rh-e, .rh-s, .rh-se { position: absolute; opacity: 0; transition: opacity 0.15s; }

.rh-e {
  right: -5px; top: 12px; bottom: 12px; width: 10px;
  cursor: ew-resize;
}
.rh-s {
  bottom: -5px; left: 12px; right: 12px; height: 10px;
  cursor: ns-resize;
}
.rh-se {
  right: 0; bottom: 0; width: 20px; height: 20px;
  cursor: se-resize;
  display: flex; align-items: center; justify-content: center;
  color: var(--section-color, #6366f1);
  border-radius: 0 0 8px 0;
}
.rh-se svg { width: 8px; height: 8px; }

.section-card:hover .rh-e,
.section-card:hover .rh-s,
.section-card:hover .rh-se,
.section-card.selected .rh-e,
.section-card.selected .rh-s,
.section-card.selected .rh-se { opacity: 1; }
</style>
