<script setup lang="ts">
import { ref, computed, nextTick, onMounted, watch } from 'vue'
import type { DataNode } from '../stores/schema'
import { useSchemaStore } from '../stores/schema'
import { useDuckDB } from '../composables/useDuckDB'
import { useQueryResults } from '../composables/useQueryResults'
import { useAppReady } from '../composables/useAppReady'
import { IS_DESKTOP } from '../lib/env'
import NodeColorPicker from './NodeColorPicker.vue'
import { useContextMenu } from '../composables/useContextMenu'
import { useDataPanel } from '../composables/useDataPanel'

const { openNodeMenu } = useContextMenu()
const { openPanel } = useDataPanel()

const props = defineProps<{ node: DataNode; selected?: boolean }>()
const emit = defineEmits<{
  dragStart: [{ id: string; mouseX: number; mouseY: number; shiftKey: boolean }]
  resizeStart: [{ id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }]
}>()

const schemaStore = useSchemaStore()
const { query, exec, getTableInfo } = useDuckDB()
const { setResult } = useQueryResults()
const { isAppReady } = useAppReady()

// ── Stale detection ───────────────────────────────────────────────────────────
const isStale = computed(() => {
  if (!props.node.sourceId || props.node.sourceSql === undefined) return false
  const sourceNode = schemaStore.nodes.find((n) => n.id === props.node.sourceId)
  if (!sourceNode || sourceNode.kind !== 'query') return false
  return sourceNode.sql.trim() !== props.node.sourceSql.trim()
})

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

// ── Resize ────────────────────────────────────────────────────────────────────
const DEFAULT_W = 220
const DEFAULT_H = 160
function startResize(e: MouseEvent, direction: 'e' | 's' | 'se') {
  emit('resizeStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY, startW: props.node.w ?? DEFAULT_W, startH: props.node.h ?? DEFAULT_H, direction })
}

// ── Inline rename ─────────────────────────────────────────────────────────────
const isRenaming = ref(false)
const renameValue = ref('')
const renameInputRef = ref<HTMLInputElement | null>(null)
const renameError = ref(false)

function startRename(e: MouseEvent) {
  e.stopPropagation()
  if (deleteConfirm.value) return
  isRenaming.value = true
  renameValue.value = props.node.name
  renameError.value = false
  nextTick(() => renameInputRef.value?.select())
}

async function commitRename() {
  const newName = renameValue.value.trim()
  isRenaming.value = false
  renameError.value = false
  if (!newName || newName === props.node.name) return
  const safeOld = props.node.name.replace(/"/g, '""')
  const safeNew = newName.replace(/"/g, '""')
  try {
    await exec(`ALTER TABLE "${safeOld}" RENAME TO "${safeNew}"`)
    schemaStore.updateDataNode(props.node.id, { name: newName })
    if (IS_DESKTOP) {
      const { DeleteTableData, SaveTableData } = await import('../../wailsjs/go/main/App')
      await DeleteTableData(props.node.name).catch(() => {})
      await SaveTableData(newName).catch(() => {})
    }
  } catch {
    renameError.value = true
  }
}

function onRenameKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') { e.preventDefault(); commitRename() }
  if (e.key === 'Escape') { isRenaming.value = false }
}

// ── Delete ─────────────────────────────────────────────────────────────────────
const deleteConfirm = ref(false)
let deleteTimer: ReturnType<typeof setTimeout> | null = null

function onDeleteClick(e: MouseEvent) {
  e.stopPropagation()
  if (!deleteConfirm.value) {
    deleteConfirm.value = true
    deleteTimer = setTimeout(() => { deleteConfirm.value = false }, 3000)
  } else {
    if (deleteTimer) clearTimeout(deleteTimer)
    const name = props.node.name
    schemaStore.removeNode(props.node.id)
    exec(`DROP TABLE IF EXISTS "${name.replace(/"/g, '""')}"`)
      .catch(console.warn)
    if (IS_DESKTOP) {
      import('../../wailsjs/go/main/App').then(({ DeleteTableData }) =>
        DeleteTableData(name).catch(console.warn)
      )
    }
  }
}

// ── Refresh ────────────────────────────────────────────────────────────────────
const isRefreshing = ref(false)
const refreshError = ref<string | null>(null)

async function refresh(e: MouseEvent) {
  e.stopPropagation()
  if (isRefreshing.value) return
  const sourceNode = props.node.sourceId
    ? schemaStore.nodes.find((n) => n.id === props.node.sourceId)
    : null
  const sql = (sourceNode?.kind === 'query' ? sourceNode.sql : props.node.sourceSql)?.trim().replace(/;+$/, '')
  if (!sql) return
  isRefreshing.value = true
  refreshError.value = null
  const safe = props.node.name.replace(/"/g, '""')
  try {
    await exec(`CREATE OR REPLACE TABLE "${safe}" AS (${sql})`)
    // Re-apply column casts
    if (props.node.columnCasts) {
      for (const [colName, cast] of Object.entries(props.node.columnCasts)) {
        const safeCol = colName.replace(/"/g, '""')
        const alterSQL = cast.expr
          ? `ALTER TABLE "${safe}" ALTER COLUMN "${safeCol}" TYPE ${cast.type} USING (${cast.expr})`
          : `ALTER TABLE "${safe}" ALTER COLUMN "${safeCol}" TYPE ${cast.type}`
        await exec(alterSQL).catch(console.warn)
      }
    }
    const [columns, countResult] = await Promise.all([
      getTableInfo(props.node.name),
      query(`SELECT COUNT(*) AS n FROM "${safe}"`),
    ])
    const rowCount = Number(countResult.rows[0]?.n ?? 0)
    const newSql = sourceNode?.kind === 'query' ? sourceNode.sql.trim() : sql
    schemaStore.updateDataNode(props.node.id, { columns, rowCount, sourceSql: newSql })
    // Invalidate cached query results so charts re-fetch on next render
    setResult(props.node.id, { columns: [], rows: [], error: null, isRunning: false })
    if (IS_DESKTOP) {
      const { SaveTableData } = await import('../../wailsjs/go/main/App')
      await SaveTableData(props.node.name)
    }
  } catch (err) {
    refreshError.value = err instanceof Error ? err.message : String(err)
  } finally {
    isRefreshing.value = false
  }
}

// ── Auto-load results for chart consumers ─────────────────────────────────────
async function loadResults() {
  const safe = props.node.name.replace(/"/g, '""')
  try {
    const result = await query(`SELECT * FROM "${safe}"`)
    setResult(props.node.id, { columns: result.columns, columnTypes: result.columnTypes, rows: result.rows, error: null, isRunning: false })
  } catch {
    // table may not exist yet; silently ignore
  }
}

onMounted(() => {
  if (isAppReady.value) { loadResults(); return }
  const stop = watch(isAppReady, (ready) => { if (ready) { stop(); loadResults() } })
})

</script>

<template>
  <div
    class="data-card"
    :class="{ selected, stale: isStale }"
    :data-node-id="node.id"
    :style="{ left: `${node.x}px`, top: `${node.y}px`, width: `${node.w ?? 220}px` }"
    @mousedown="onMouseDown"
    @contextmenu.prevent.stop="openNodeMenu(node.id, $event.clientX, $event.clientY)"
  >
    <!-- Header -->
    <div class="card-header" :style="{ background: node.color }">
      <!-- Cylinder icon -->
      <svg class="card-icon" viewBox="0 0 14 16" fill="none">
        <ellipse cx="7" cy="3.5" rx="5" ry="2" stroke="white" stroke-width="1.3"/>
        <path d="M2 3.5v9c0 1.1 2.24 2 5 2s5-.9 5-2v-9" stroke="white" stroke-width="1.3"/>
        <path d="M2 8c0 1.1 2.24 2 5 2s5-.9 5-2" stroke="white" stroke-width="1.3" opacity="0.6"/>
      </svg>

      <input
        v-if="isRenaming"
        ref="renameInputRef"
        v-model="renameValue"
        class="card-name-input"
        :class="{ error: renameError }"
        @mousedown.stop
        @keydown="onRenameKeydown"
        @blur="commitRename"
      />
      <span v-else class="card-name" title="Double-click to rename" @mousedown.stop @dblclick="startRename">
        {{ node.name }}
      </span>

      <span v-if="isStale" class="stale-badge" title="Source query has changed — click Refresh to update">stale</span>

      <NodeColorPicker :color="node.color" @pick="schemaStore.setNodeColor(node.id, $event)" />

      <button class="collapse-btn" title="Edit column types" @mousedown.stop @click.stop="openPanel(node.id)">
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
        :title="deleteConfirm ? 'Click again to confirm' : 'Delete data table'"
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

    <!-- Body: column list -->
    <template v-if="!isCollapsed">
      <div class="card-body" :style="{ maxHeight: `${node.h ?? 160}px` }">
        <div v-if="node.columns.length" class="col-list">
          <div v-for="col in node.columns" :key="col.name" class="col-row">
            <span class="col-name">{{ col.name }}</span>
            <span class="col-type" :class="{ 'col-type-cast': node.columnCasts?.[col.name] }">{{ col.type }}</span>
          </div>
        </div>
        <div v-else class="empty-cols">No columns</div>
      </div>

      <!-- Footer -->
      <div class="card-footer">
        <span class="row-count">
          {{ node.rowCount !== undefined ? `${node.rowCount.toLocaleString()} rows` : '' }}
        </span>
        <span v-if="refreshError" class="refresh-error" :title="refreshError">Error</span>
        <button
          v-if="node.sourceId || node.sourceSql"
          class="ghost-btn"
          :class="{ 'refresh-stale': isStale }"
          :disabled="isRefreshing"
          title="Re-materialize from source query"
          @mousedown.stop
          @click.stop="refresh"
        >
          <svg viewBox="0 0 10 10" fill="none" :class="{ spin: isRefreshing }">
            <path d="M8.5 5A3.5 3.5 0 1 0 6.75 8.03" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
            <polyline points="8.5,2.5 8.5,5 6,5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          {{ isRefreshing ? 'Refreshing…' : 'Refresh' }}
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
    </template>
  </div>
</template>

<style scoped>
.data-card {
  position: absolute;
  width: 220px;
  height: fit-content;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface-1);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
  user-select: none;
  cursor: grab;
  transition: box-shadow 0.15s, border-color 0.15s;
}
.data-card:active { cursor: grabbing; }
.data-card.selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px rgba(88, 166, 255, 0.2), 0 4px 16px rgba(0, 0, 0, 0.4);
}
.data-card.stale { border-color: rgba(245, 158, 11, 0.5); }

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

.card-icon { width: 12px; height: 14px; flex-shrink: 0; opacity: 0.9; }

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
.card-name-input:focus { border-color: rgba(255, 255, 255, 0.75); background: rgba(0, 0, 0, 0.35); }
.card-name-input.error { border-color: rgba(255, 80, 80, 0.8); }

.stale-badge {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  background: rgba(245, 158, 11, 0.3);
  border: 1px solid rgba(245, 158, 11, 0.5);
  color: #fbbf24;
  padding: 1px 4px;
  border-radius: 3px;
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
  background: transparent;
  color: white;
  cursor: pointer;
  opacity: 0;
  flex-shrink: 0;
  transition: opacity 0.15s, background 0.15s, width 0.15s;
}
.delete-btn svg { width: 11px; height: 11px; }
.data-card:hover .delete-btn, .delete-btn.confirming { opacity: 1; }
.delete-btn:hover { background: rgba(248, 81, 73, 0.35); }
.delete-btn.confirming { background: rgba(248, 81, 73, 0.5); width: auto; padding: 0 6px; }
.confirm-label { font-size: 10px; font-weight: 700; white-space: nowrap; letter-spacing: 0.02em; }

/* ── Body ────────────────────────────────────────────────────────────────── */
.card-body {
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: var(--border) transparent;
  border-bottom: 1px solid var(--border);
  background: var(--surface-0);
}

.col-list { padding: 4px 0; }

.col-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 3px 10px;
  gap: 6px;
  font-size: 10.5px;
  border-bottom: 1px solid var(--surface-2);
}
.col-row:last-child { border-bottom: none; }
.col-row:hover { background: var(--surface-1); }

.col-name {
  font-family: var(--font-mono);
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.col-type {
  font-family: var(--font-mono);
  font-size: 9.5px;
  color: var(--text-muted);
  text-transform: uppercase;
  flex-shrink: 0;
}

.empty-cols {
  padding: 12px;
  font-size: 10.5px;
  color: var(--text-muted);
  font-style: italic;
  text-align: center;
}

/* ── Footer ──────────────────────────────────────────────────────────────── */
.card-footer {
  display: flex;
  align-items: center;
  padding: 5px 8px 6px 10px;
  gap: 6px;
}

.row-count {
  flex: 1;
  font-size: 10.5px;
  font-family: var(--font-mono);
  color: var(--text-muted);
}

.refresh-error {
  font-size: 10px;
  color: var(--error);
  font-family: var(--font-mono);
}

.ghost-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 3px 7px;
  font-size: 10.5px;
  font-weight: 500;
  background: var(--surface-2);
  color: var(--text-secondary);
  border: 1px solid var(--border);
  border-radius: 4px;
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.12s, color 0.12s;
}
.ghost-btn svg { width: 9px; height: 9px; }
.ghost-btn:hover { color: var(--text-primary); background: var(--surface-0); }
.ghost-btn:disabled { opacity: 0.6; cursor: not-allowed; }

.ghost-btn.refresh-stale {
  color: #f59e0b;
  border-color: rgba(245, 158, 11, 0.4);
  background: rgba(245, 158, 11, 0.08);
}
.ghost-btn.refresh-stale:hover { background: rgba(245, 158, 11, 0.15); }

/* ── Resize handles ──────────────────────────────────────────────────────── */
.rh-e, .rh-s, .rh-se { position: absolute; opacity: 0; transition: opacity 0.15s; }
.rh-e { right: 0; top: 8px; bottom: 20px; width: 6px; cursor: ew-resize; border-radius: 0 4px 4px 0; }
.rh-s { bottom: 0; left: 8px; right: 20px; height: 6px; cursor: ns-resize; border-radius: 0 0 4px 4px; }
.rh-se { right: 0; bottom: 0; width: 18px; height: 18px; cursor: se-resize; display: flex; align-items: center; justify-content: center; color: var(--text-muted); border-radius: 0 0 7px 0; }
.rh-se svg { width: 8px; height: 8px; }
.rh-e:hover, .rh-s:hover { background: rgba(88, 166, 255, 0.2); }
.data-card:hover .rh-e, .data-card:hover .rh-s, .data-card:hover .rh-se,
.data-card.selected .rh-e, .data-card.selected .rh-s, .data-card.selected .rh-se { opacity: 1; }

.spin { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* ── Cast indicator ──────────────────────────────────────────────────────── */
.col-type-cast { color: var(--accent); }
</style>
