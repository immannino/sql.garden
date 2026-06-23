<script setup lang="ts">
import { ref, computed, nextTick, watch } from 'vue'
import { marked } from 'marked'
import type { MarkdownNode } from '../stores/schema'
import { useSchemaStore } from '../stores/schema'
import NodeColorPicker from './NodeColorPicker.vue'
import { useContextMenu } from '../composables/useContextMenu'

const { openNodeMenu } = useContextMenu()

const props = defineProps<{ node: MarkdownNode; selected?: boolean }>()
const emit = defineEmits<{
  dragStart: [{ id: string; mouseX: number; mouseY: number; shiftKey: boolean }]
  resizeStart: [{ id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }]
}>()

const schemaStore = useSchemaStore()
const DEFAULT_W = 300
const DEFAULT_H = 200

// ── Edit / preview toggle ──────────────────────────────────────────────────────
const isEditing = ref(!props.node.content)
const localContent = ref(props.node.content)
const textareaRef = ref<HTMLTextAreaElement | null>(null)

watch(() => props.node.content, (v) => { if (v !== localContent.value) localContent.value = v })

const rendered = computed(() => String(marked.parse(localContent.value || '')))

let contentTimer: ReturnType<typeof setTimeout> | null = null
function onContentInput() {
  if (contentTimer) clearTimeout(contentTimer)
  contentTimer = setTimeout(() => schemaStore.updateMarkdownContent(props.node.id, localContent.value), 400)
}

function enterEdit() {
  isEditing.value = true
  nextTick(() => textareaRef.value?.focus())
}

function exitEdit() {
  if (contentTimer) { clearTimeout(contentTimer); contentTimer = null }
  schemaStore.updateMarkdownContent(props.node.id, localContent.value)
  isEditing.value = false
}

// ── View mode ─────────────────────────────────────────────────────────────────
const isCollapsed = computed(() => props.node.viewMode === 'collapsed')
function toggleCollapsed(e: MouseEvent) {
  e.stopPropagation()
  schemaStore.updateViewMode(props.node.id, isCollapsed.value ? 'default' : 'collapsed')
}

// ── Drag ──────────────────────────────────────────────────────────────────────
function onMouseDown(e: MouseEvent) {
  if (e.button !== 0) return
  e.stopPropagation()
  emit('dragStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY, shiftKey: e.shiftKey })
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

// ── Inline rename ─────────────────────────────────────────────────────────────
const isRenaming = ref(false)
const renameValue = ref('')
const renameInputRef = ref<HTMLInputElement | null>(null)

function startRename(e: MouseEvent) {
  e.stopPropagation()
  if (deleteConfirm.value) return
  isRenaming.value = true
  renameValue.value = props.node.name
  nextTick(() => renameInputRef.value?.select())
}

function commitRename() {
  const newName = renameValue.value.trim()
  isRenaming.value = false
  if (!newName || newName === props.node.name) return
  schemaStore.renameNode(props.node.id, newName)
}

function onRenameKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') { e.preventDefault(); commitRename() }
  if (e.key === 'Escape') { isRenaming.value = false }
}

// ── Resize ────────────────────────────────────────────────────────────────────
function startResize(e: MouseEvent, direction: 'e' | 's' | 'se') {
  emit('resizeStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY, startW: props.node.w ?? DEFAULT_W, startH: props.node.h ?? DEFAULT_H, direction })
}
</script>

<template>
  <div
    class="markdown-card"
    :class="{ selected, editing: isEditing }"
    :style="{ left: `${node.x}px`, top: `${node.y}px`, width: `${node.w ?? 300}px` }"
    @mousedown="onMouseDown"
    @contextmenu.prevent.stop="openNodeMenu(node.id, $event.clientX, $event.clientY)"
  >
    <!-- Header -->
    <div class="card-header" :style="{ background: node.color }">
      <svg class="card-icon" viewBox="0 0 16 16" fill="none">
        <rect x="1" y="2" width="14" height="12" rx="2" stroke="white" stroke-width="1.4"/>
        <line x1="4" y1="6" x2="12" y2="6" stroke="white" stroke-width="1.2" stroke-linecap="round"/>
        <line x1="4" y1="9" x2="9" y2="9" stroke="white" stroke-width="1.2" stroke-linecap="round"/>
      </svg>

      <input
        v-if="isRenaming"
        ref="renameInputRef"
        v-model="renameValue"
        class="card-name-input"
        @mousedown.stop
        @keydown="onRenameKeydown"
        @blur="commitRename"
      />
      <span v-else class="card-name" title="Double-click to rename" @mousedown.stop @dblclick="startRename">
        {{ node.name }}
      </span>

      <NodeColorPicker :color="node.color" @pick="schemaStore.setNodeColor(node.id, $event)" />

      <button
        class="icon-btn"
        :title="isEditing ? 'Preview' : 'Edit'"
        @mousedown.stop
        @click.stop="isEditing ? exitEdit() : enterEdit()"
      >
        <!-- Pencil (edit mode off → click to edit) -->
        <svg v-if="!isEditing" viewBox="0 0 12 12" fill="none">
          <path d="M8.5 1.5l2 2L4 10H2V8L8.5 1.5z" stroke="white" stroke-width="1.2" stroke-linejoin="round"/>
        </svg>
        <!-- Eye (edit mode on → click to preview) -->
        <svg v-else viewBox="0 0 12 12" fill="none">
          <path d="M1 6C1 6 3 2 6 2s5 4 5 4-2 4-5 4S1 6 1 6z" stroke="white" stroke-width="1.2"/>
          <circle cx="6" cy="6" r="1.5" stroke="white" stroke-width="1.2"/>
        </svg>
      </button>

      <button class="collapse-btn" :title="isCollapsed ? 'Expand' : 'Collapse'" @mousedown.stop @click.stop="toggleCollapsed">
        <svg viewBox="0 0 10 10" fill="none">
          <path v-if="!isCollapsed" d="M2 3.5l3 3 3-3" stroke="white" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
          <path v-else d="M2 6.5l3-3 3 3" stroke="white" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>

      <button
        class="delete-btn"
        :class="{ confirming: deleteConfirm }"
        :title="deleteConfirm ? 'Click again to confirm' : 'Delete note'"
        @mousedown.stop
        @click.stop="onDeleteClick"
      >
        <span v-if="deleteConfirm" class="confirm-label">Delete?</span>
        <svg v-else viewBox="0 0 12 12" fill="none">
          <path d="M2 3.5h8" stroke="white" stroke-width="1.2" stroke-linecap="round"/>
          <path d="M4.5 3.5V2.5h3v1" stroke="white" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M3.5 3.5l.7 6h3.6l.7-6" stroke="white" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>
    </div>

    <!-- Content -->
    <div v-if="!isCollapsed" class="card-body" :style="{ height: `${node.h ?? 200}px` }" @mousedown.stop>
      <textarea
        v-if="isEditing"
        ref="textareaRef"
        v-model="localContent"
        class="md-editor"
        placeholder="# Heading&#10;Write **markdown** here…&#10;&#10;- Lists&#10;- Supported"
        @input="onContentInput"
        @keydown.escape.stop="exitEdit"
      />
      <div
        v-else
        class="md-rendered"
        v-html="rendered || '<p class=\'md-empty\'>Double-click to edit…</p>'"
        @dblclick="enterEdit"
      />
    </div>

    <!-- Resize handles -->
    <template v-if="!isCollapsed">
      <div class="rh-e"  @mousedown.stop="startResize($event, 'e')" />
      <div class="rh-s"  @mousedown.stop="startResize($event, 's')" />
      <div class="rh-se" @mousedown.stop="startResize($event, 'se')">
        <svg viewBox="0 0 8 8" fill="none">
          <line x1="7" y1="1" x2="1" y2="7" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
          <line x1="7" y1="4" x2="4" y2="7" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
        </svg>
      </div>
    </template>
  </div>
</template>

<style scoped>
.markdown-card {
  position: absolute;
  width: 300px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface-1);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
  user-select: none;
  cursor: grab;
  transition: box-shadow 0.15s, border-color 0.15s;
  overflow: hidden;
}

.markdown-card:active { cursor: grabbing; }
.markdown-card.editing { cursor: default; }

.markdown-card.selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px rgba(88, 166, 255, 0.2), 0 4px 16px rgba(0, 0, 0, 0.4);
}

/* ── Header ──────────────────────────────────────────────────────────────── */
.card-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 8px 8px 10px;
  border-radius: 7px 7px 0 0;
  font-size: 12px;
  font-weight: 600;
  color: white;
  letter-spacing: 0.02em;
}

.card-header:hover :deep(.ncp-trigger) { opacity: 0.7; }

.card-icon { width: 13px; height: 13px; flex-shrink: 0; opacity: 0.9; }

.card-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: text;
}

.card-name-input {
  flex: 1;
  min-width: 0;
  background: rgba(0, 0, 0, 0.25);
  border: 1px solid rgba(255, 255, 255, 0.4);
  border-radius: 3px;
  color: white;
  font-size: 12px;
  font-weight: 600;
  font-family: inherit;
  letter-spacing: 0.02em;
  padding: 1px 4px;
  outline: none;
}

.card-name-input:focus {
  border-color: rgba(255, 255, 255, 0.75);
}

.collapse-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 3px;
  border: none;
  background: transparent;
  color: white;
  cursor: pointer;
  opacity: 0.45;
  flex-shrink: 0;
  transition: opacity 0.15s, background 0.15s;
}
.collapse-btn svg { width: 10px; height: 10px; }
.collapse-btn:hover { opacity: 1; background: rgba(255, 255, 255, 0.15); }

.icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 4px;
  border: none;
  background: transparent;
  color: white;
  cursor: pointer;
  flex-shrink: 0;
  opacity: 0.7;
  transition: opacity 0.15s, background 0.15s;
}

.icon-btn svg { width: 11px; height: 11px; }
.icon-btn:hover { opacity: 1; background: rgba(255, 255, 255, 0.15); }

.delete-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 4px;
  border: none;
  background: transparent;
  color: white;
  cursor: pointer;
  opacity: 0;
  flex-shrink: 0;
  transition: opacity 0.15s, background 0.15s, width 0.15s;
}

.delete-btn svg { width: 11px; height: 11px; }

.markdown-card:hover .delete-btn,
.delete-btn.confirming { opacity: 1; }

.delete-btn:hover { background: rgba(248, 81, 73, 0.35); }

.delete-btn.confirming {
  background: rgba(248, 81, 73, 0.5);
  width: auto;
  padding: 0 6px;
}

.confirm-label { font-size: 10px; font-weight: 700; white-space: nowrap; letter-spacing: 0.02em; }

/* ── Body ────────────────────────────────────────────────────────────────── */
.card-body {
  height: 200px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.md-editor {
  flex: 1;
  width: 100%;
  resize: none;
  background: var(--surface-0);
  color: var(--text-primary);
  border: none;
  outline: none;
  padding: 10px 12px;
  font-size: 12px;
  line-height: 1.6;
  font-family: var(--font-mono);
  tab-size: 2;
  box-sizing: border-box;
  cursor: text;
  user-select: text;
}

.md-editor::placeholder { color: var(--text-muted); }

.md-rendered {
  flex: 1;
  overflow-y: auto;
  padding: 10px 12px;
  cursor: default;
  background: var(--surface-0);
}

.md-rendered :deep(.md-empty) {
  color: var(--text-muted);
  font-style: italic;
  font-size: 12px;
  margin: 0;
}

/* Prose styles for rendered markdown */
.md-rendered :deep(h1),
.md-rendered :deep(h2),
.md-rendered :deep(h3),
.md-rendered :deep(h4) {
  color: var(--text-primary);
  font-weight: 600;
  line-height: 1.3;
  margin: 0 0 6px;
}

.md-rendered :deep(h1) { font-size: 15px; }
.md-rendered :deep(h2) { font-size: 13px; }
.md-rendered :deep(h3) { font-size: 12px; }
.md-rendered :deep(h4) { font-size: 11px; }

.md-rendered :deep(p) {
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.6;
  margin: 0 0 8px;
}

.md-rendered :deep(p:last-child) { margin-bottom: 0; }

.md-rendered :deep(strong) { color: var(--text-primary); font-weight: 600; }
.md-rendered :deep(em) { color: var(--text-secondary); font-style: italic; }

.md-rendered :deep(code) {
  font-family: var(--font-mono);
  font-size: 11px;
  background: var(--surface-2);
  padding: 1px 4px;
  border-radius: 3px;
  color: var(--accent);
}

.md-rendered :deep(pre) {
  background: var(--surface-2);
  padding: 8px 10px;
  border-radius: 4px;
  overflow-x: auto;
  margin: 0 0 8px;
}

.md-rendered :deep(pre code) {
  background: none;
  padding: 0;
  font-size: 11px;
  color: var(--text-primary);
}

.md-rendered :deep(ul),
.md-rendered :deep(ol) {
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.6;
  padding-left: 18px;
  margin: 0 0 8px;
}

.md-rendered :deep(li) { margin: 2px 0; }

.md-rendered :deep(a) { color: var(--accent); text-decoration: none; }
.md-rendered :deep(a:hover) { text-decoration: underline; }

.md-rendered :deep(hr) {
  border: none;
  border-top: 1px solid var(--border);
  margin: 8px 0;
}

.md-rendered :deep(blockquote) {
  border-left: 3px solid var(--border);
  padding-left: 10px;
  margin: 0 0 8px;
  color: var(--text-muted);
  font-size: 12px;
}

/* ── Resize handles ──────────────────────────────────────────────────────── */
.rh-e, .rh-s, .rh-se { position: absolute; opacity: 0; transition: opacity 0.15s; }

.rh-e {
  right: 0; top: 8px; bottom: 20px; width: 6px;
  cursor: ew-resize;
  border-radius: 0 4px 4px 0;
}

.rh-s {
  bottom: 0; left: 8px; right: 20px; height: 6px;
  cursor: ns-resize;
  border-radius: 0 0 4px 4px;
}

.rh-se {
  right: 0; bottom: 0; width: 18px; height: 18px;
  cursor: se-resize;
  display: flex; align-items: center; justify-content: center;
  color: var(--text-muted);
  border-radius: 0 0 7px 0;
}

.rh-se svg { width: 8px; height: 8px; }

.rh-e:hover, .rh-s:hover { background: rgba(88, 166, 255, 0.2); }

.markdown-card:hover .rh-e,
.markdown-card:hover .rh-s,
.markdown-card:hover .rh-se,
.markdown-card.selected .rh-e,
.markdown-card.selected .rh-s,
.markdown-card.selected .rh-se { opacity: 1; }
</style>
