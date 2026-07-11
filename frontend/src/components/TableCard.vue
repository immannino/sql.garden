<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import type { TableNode } from '../stores/schema'
import { useSchemaStore } from '../stores/schema'
import { useQueryBridge } from '../composables/useQueryBridge'
import { useTableOps } from '../composables/useTableOps'
import NodeColorPicker from './NodeColorPicker.vue'
import { useContextMenu } from '../composables/useContextMenu'
import { useDataPanel } from '../composables/useDataPanel'

const { openNodeMenu } = useContextMenu()
const { openPanel } = useDataPanel()

const props = defineProps<{ table: TableNode; selected?: boolean }>()
const emit = defineEmits<{
  dragStart: [{ id: string; mouseX: number; mouseY: number; shiftKey: boolean }]
  resizeStart: [{ id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }]
}>()

const { sendQuery } = useQueryBridge()
const { dropTable, renameTable } = useTableOps()

const deleteConfirm = ref(false)
let deleteTimer: ReturnType<typeof setTimeout> | null = null

// ── Inline rename ─────────────────────────────────────────────────────────────
const isRenaming = ref(false)
const renameValue = ref('')
const renameInputRef = ref<HTMLInputElement | null>(null)
const renameError = ref(false)

function startRename(e: MouseEvent) {
  e.stopPropagation()
  if (deleteConfirm.value) return
  isRenaming.value = true
  renameValue.value = props.table.name
  renameError.value = false
  nextTick(() => {
    renameInputRef.value?.select()
  })
}

async function commitRename() {
  const newName = renameValue.value.trim()
  if (!newName || newName === props.table.name) {
    cancelRename()
    return
  }
  isRenaming.value = false
  try {
    await renameTable(props.table.id, props.table.name, newName)
  } catch (e) {
    console.warn('Rename failed:', e)
  }
}

function cancelRename() {
  isRenaming.value = false
  renameError.value = false
}

function onRenameKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') { e.preventDefault(); commitRename() }
  if (e.key === 'Escape') cancelRename()
}

// ── View mode ─────────────────────────────────────────────────────────────────
const schemaStore = useSchemaStore()
const { updateViewMode } = schemaStore
const isCollapsed = computed(() => props.table.viewMode === 'collapsed')
function toggleCollapsed(e: MouseEvent) {
  e.stopPropagation()
  updateViewMode(props.table.id, isCollapsed.value ? 'default' : 'collapsed')
}

// ── Canvas drag ───────────────────────────────────────────────────────────────
function onMouseDown(e: MouseEvent) {
  if (e.button !== 0) return
  e.stopPropagation()
  emit('dragStart', { id: props.table.id, mouseX: e.clientX, mouseY: e.clientY, shiftKey: e.shiftKey })
}

// ── Resize ────────────────────────────────────────────────────────────────────
const cardRef = ref<HTMLElement | null>(null)
const DEFAULT_W = 240

function startResize(e: MouseEvent, direction: 'e' | 's' | 'se') {
  const startW = props.table.w ?? cardRef.value?.offsetWidth ?? DEFAULT_W
  const startH = props.table.h ?? cardRef.value?.offsetHeight ?? 200
  emit('resizeStart', { id: props.table.id, mouseX: e.clientX, mouseY: e.clientY, startW, startH, direction })
}

// ── Query button ──────────────────────────────────────────────────────────────
function onQueryClick(e: MouseEvent) {
  e.stopPropagation()
  sendQuery(`SELECT * FROM "${props.table.name}" LIMIT 100;`)
}

// ── Delete button (two-click confirm) ─────────────────────────────────────────
function onDeleteClick(e: MouseEvent) {
  e.stopPropagation()
  if (!deleteConfirm.value) {
    deleteConfirm.value = true
    deleteTimer = setTimeout(() => { deleteConfirm.value = false }, 3000)
  } else {
    if (deleteTimer) clearTimeout(deleteTimer)
    dropTable(props.table.id, props.table.name)
  }
}

function onCardMouseLeave() {
  if (deleteConfirm.value) return // keep confirm visible until timeout
}

// ── Helpers ───────────────────────────────────────────────────────────────────
function typeColor(type: string) {
  const t = type.toLowerCase()
  if (/\b(int|integer|bigint|smallint|tinyint|hugeint|ubigint|decimal|numeric|float|double|real)\b/.test(t)) return '#3b82f6'
  if (/\b(varchar|text|char|string|blob)\b/.test(t)) return '#10b981'
  if (/\b(timestamp|date|time|interval)\b/.test(t)) return '#f59e0b'
  if (/\b(bool|boolean)\b/.test(t)) return '#8b5cf6'
  return '#6b7280'
}
</script>

<template>
  <div
    ref="cardRef"
    class="table-card"
    :class="{ selected, 'has-h': !!table.h }"
    :data-node-id="table.id"
    :style="{ left: `${table.x}px`, top: `${table.y}px`, width: `${table.w ?? 240}px`, ...(table.h ? { height: `${table.h}px` } : {}) }"
    @mousedown="onMouseDown"
    @contextmenu.prevent.stop="openNodeMenu(table.id, $event.clientX, $event.clientY)"
    @mouseleave="onCardMouseLeave"
  >
    <!-- Header -->
    <div class="card-header" :style="{ background: table.color }">
      <svg class="card-icon" viewBox="0 0 16 16" fill="none">
        <rect x="1" y="1" width="14" height="14" rx="2" stroke="white" stroke-width="1.5"/>
        <line x1="1" y1="5.5" x2="15" y2="5.5" stroke="white" stroke-width="1.5"/>
        <line x1="5.5" y1="5.5" x2="5.5" y2="15" stroke="white" stroke-width="1.5"/>
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
      <span
        v-else
        class="card-name"
        title="Double-click to rename"
        @mousedown.stop
        @dblclick="startRename"
      >{{ table.name }}</span>

      <span class="card-count">{{ table.columns.length }}</span>

      <NodeColorPicker :color="table.color" @pick="schemaStore.setNodeColor(table.id, $event)" />

      <button class="collapse-btn" title="Edit column types" @mousedown.stop @click.stop="openPanel(table.id)">
        <svg viewBox="0 0 10 10" fill="none">
          <line x1="1" y1="3" x2="9" y2="3" stroke="white" stroke-width="1.2" stroke-linecap="round"/>
          <line x1="1" y1="5" x2="9" y2="5" stroke="white" stroke-width="1.2" stroke-linecap="round"/>
          <line x1="1" y1="7" x2="9" y2="7" stroke="white" stroke-width="1.2" stroke-linecap="round"/>
          <circle cx="3.5" cy="3" r="1.2" fill="white"/>
          <circle cx="6.5" cy="7" r="1.2" fill="white"/>
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
        :title="deleteConfirm ? 'Click again to confirm deletion' : 'Delete table'"
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

    <!-- Columns -->
    <div v-if="!isCollapsed" class="card-body">
      <div v-for="col in table.columns" :key="col.name" class="col-row">
        <span class="col-pk" v-if="col.primaryKey" title="Primary Key">
          <svg viewBox="0 0 12 12" fill="none">
            <circle cx="4.5" cy="4.5" r="3" stroke="#f59e0b" stroke-width="1.2"/>
            <line x1="7" y1="7" x2="11" y2="11" stroke="#f59e0b" stroke-width="1.2" stroke-linecap="round"/>
          </svg>
        </span>
        <span v-else class="col-spacer" />
        <span class="col-name" :class="{ 'is-pk': col.primaryKey }">{{ col.name }}</span>
        <span class="col-type" :style="{ color: typeColor(col.type) }">{{ col.type }}</span>
        <span v-if="col.references" class="col-fk" title="Foreign Key">
          <svg viewBox="0 0 10 10" fill="none">
            <path d="M1 5h8M5 1l4 4-4 4" stroke="#6b7280" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </span>
      </div>
    </div>

    <!-- Footer: row count + query button -->
    <div v-if="!isCollapsed" class="card-footer">
      <span class="footer-stat">
        <span v-if="table.rowCount === undefined" class="stat-loading">counting…</span>
        <span v-else class="stat-rows">
          {{ table.rowCount.toLocaleString() }} {{ table.rowCount === 1 ? 'row' : 'rows' }}
        </span>
      </span>

      <button
        class="query-btn"
        title="SELECT * FROM this table"
        @mousedown.stop
        @click.stop="onQueryClick"
      >
        <svg viewBox="0 0 10 10" fill="none">
          <path d="M2 5h6M5 2l3 3-3 3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        Query
      </button>
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
.table-card {
  position: absolute;
  width: 240px; /* overridden by inline style when w is set */
  display: flex;
  flex-direction: column;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface-1);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
  user-select: none;
  cursor: grab;
  transition: box-shadow 0.15s, border-color 0.15s;
}

/* When h is explicitly set, body scrolls instead of expanding */
.table-card.has-h .card-body {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.table-card:active {
  cursor: grabbing;
}

.table-card.selected {
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

.card-icon {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  opacity: 0.9;
}

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
  width: 100%;
}

.card-name-input:focus {
  border-color: rgba(255, 255, 255, 0.75);
  background: rgba(0, 0, 0, 0.35);
}

.card-count {
  font-size: 10px;
  opacity: 0.7;
  background: rgba(0, 0, 0, 0.2);
  padding: 1px 5px;
  border-radius: 8px;
  flex-shrink: 0;
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

.delete-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 4px;
  border: none;
  background: rgba(0, 0, 0, 0);
  color: white;
  cursor: pointer;
  opacity: 0;
  flex-shrink: 0;
  transition: opacity 0.15s, background 0.15s, width 0.15s;
}

.delete-btn svg {
  width: 11px;
  height: 11px;
}

.table-card:hover .delete-btn,
.delete-btn.confirming {
  opacity: 1;
}

.delete-btn:hover {
  background: rgba(248, 81, 73, 0.35);
}

.delete-btn.confirming {
  background: rgba(248, 81, 73, 0.5);
  width: auto;
  padding: 0 6px;
}

.confirm-label {
  font-size: 10px;
  font-weight: 700;
  white-space: nowrap;
  letter-spacing: 0.02em;
}

/* ── Columns ─────────────────────────────────────────────────────────────── */
.card-body {
  padding: 4px 0;
}

.col-row {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 3px 10px;
  font-size: 11.5px;
  line-height: 1.4;
  transition: background 0.1s;
}

.col-row:hover {
  background: var(--surface-2);
}

.col-pk svg,
.col-fk svg {
  width: 12px;
  height: 12px;
  display: block;
}

.col-spacer {
  width: 12px;
  flex-shrink: 0;
}

.col-name {
  flex: 1;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--font-mono);
  font-size: 11px;
}

.col-name.is-pk {
  color: #f59e0b;
}

.col-type {
  font-family: var(--font-mono);
  font-size: 10px;
  opacity: 0.85;
  flex-shrink: 0;
}

.col-fk {
  flex-shrink: 0;
  opacity: 0.6;
}

/* ── Footer ──────────────────────────────────────────────────────────────── */
.card-footer {
  display: flex;
  align-items: center;
  padding: 5px 8px 6px 10px;
  border-top: 1px solid var(--surface-2);
  gap: 6px;
}

.footer-stat {
  flex: 1;
}

.stat-rows {
  font-size: 10.5px;
  font-family: var(--font-mono);
  color: var(--text-muted);
}

.stat-loading {
  font-size: 10.5px;
  font-family: var(--font-mono);
  color: var(--text-muted);
  opacity: 0.5;
  font-style: italic;
}

.query-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  font-size: 10.5px;
  font-weight: 500;
  color: var(--text-secondary);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  cursor: pointer;
  transition: color 0.15s, background 0.15s, border-color 0.15s;
  flex-shrink: 0;
}

.query-btn svg {
  width: 10px;
  height: 10px;
  flex-shrink: 0;
}

.query-btn:hover {
  color: var(--accent);
  background: rgba(88, 166, 255, 0.08);
  border-color: rgba(88, 166, 255, 0.3);
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

.table-card:hover .rh-e,
.table-card:hover .rh-s,
.table-card:hover .rh-se,
.table-card.selected .rh-e,
.table-card.selected .rh-s,
.table-card.selected .rh-se { opacity: 1; }
</style>
