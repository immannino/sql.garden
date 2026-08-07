<script setup lang="ts">
import { ref, nextTick, watch, computed, onMounted, onUnmounted } from 'vue'
import type { QueryNode, SqlHistoryEntry } from '../stores/schema'
import { useSchemaStore } from '../stores/schema'
import { useDuckDB } from '../composables/useDuckDB'
import { useQueryResults } from '../composables/useQueryResults'
import { useAppReady } from '../composables/useAppReady'
import { exportData, type ExportFormat } from '../lib/exportData'
import { IS_DESKTOP } from '../lib/env'
import { usePendingFullscreen } from '../composables/usePendingFullscreen'
import SqlEditor from './SqlEditor.vue'
import { useSchemaCompletions } from '../composables/useSchemaCompletions'
import NodeColorPicker from './NodeColorPicker.vue'
import { classifyColumnType } from '../lib/columnType'
import { useContextMenu } from '../composables/useContextMenu'

const { openNodeMenu } = useContextMenu()

const props = defineProps<{ node: QueryNode; selected?: boolean }>()
const emit = defineEmits<{
  dragStart: [{ id: string; mouseX: number; mouseY: number; shiftKey: boolean }]
  resizeStart: [{ id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }]
}>()

// ── Fullscreen ─────────────────────────────────────────────────────────────────
const isFullscreen = ref(false)
const fsResultsPos = ref<'bottom' | 'right' | 'left'>('bottom')
const fsResultsTab = ref<'results' | 'history'>('results')

const { pendingFullscreenId } = usePendingFullscreen()

function toggleFullscreen(e?: MouseEvent) {
  e?.stopPropagation()
  isFullscreen.value = !isFullscreen.value
}

function onFullscreenKey(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    // If focus is inside CodeMirror, let it handle ESC first (e.g. dismiss autocomplete)
    if ((e.target as HTMLElement).closest('.cm-editor')) return
    e.preventDefault()
    e.stopImmediatePropagation()
    isFullscreen.value = false
  }
}

watch(isFullscreen, (v) => {
  if (v) window.addEventListener('keydown', onFullscreenKey, { capture: true })
  else window.removeEventListener('keydown', onFullscreenKey, { capture: true })
})

const schemaStore = useSchemaStore()
const { query, exec, getTableInfo } = useDuckDB()
const { sqlSchema } = useSchemaCompletions()
const { results: queryResults, setResult } = useQueryResults()
const { isAppReady } = useAppReady()

// ── View helpers (works in both desktop and web/WASM) ─────────────────────────
function sqlCreateView(name: string, sql: string) {
  return exec(`CREATE OR REPLACE VIEW "${name.replace(/"/g, '""')}" AS ${sql}`)
}
function sqlDropView(name: string) {
  return exec(`DROP VIEW IF EXISTS "${name.replace(/"/g, '""')}"`)
}

// ── Tabs ───────────────────────────────────────────────────────────────────────
const activeTab = ref<'sql' | 'results' | 'history'>('sql')
const nodeResult = computed(() => queryResults[props.node.id] ?? null)
const resultRowCount = computed(() => {
  const r = nodeResult.value
  return (r && !r.error && !r.isRunning) ? r.rows.length : null
})

const TABLE_DISPLAY_CAP = 500
const displayRows = computed(() => nodeResult.value?.rows.slice(0, TABLE_DISPLAY_CAP) ?? [])
const isCapped = computed(() => (nodeResult.value?.rows.length ?? 0) > TABLE_DISPLAY_CAP)

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

// ── Delete (two-click confirm) ─────────────────────────────────────────────────
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

// ── View (DuckDB VIEW) ────────────────────────────────────────────────────────
const viewError = ref<string | null>(null)
const isViewLoading = ref(false)

async function toggleView(e: MouseEvent) {
  e.stopPropagation()
  const sql = localSql.value.trim()
  const wasView = props.node.isView ?? false
  if (!sql && !wasView) return
  isViewLoading.value = true
  viewError.value = null
  try {
    if (!wasView) {
      await sqlCreateView(props.node.name, sql)
      schemaStore.setQueryIsView(props.node.id, true)
    } else {
      await sqlDropView(props.node.name)
      schemaStore.setQueryIsView(props.node.id, false)
    }
  } catch (err) {
    viewError.value = err instanceof Error ? err.message : String(err)
  } finally {
    isViewLoading.value = false
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

async function commitRename() {
  const newName = renameValue.value.trim()
  isRenaming.value = false
  if (!newName || newName === props.node.name) return
  const oldName = props.node.name
  schemaStore.renameNode(props.node.id, newName)
  if (props.node.isView) {
    try {
      await sqlDropView(oldName)
      if (localSql.value.trim()) await sqlCreateView(newName, localSql.value)
    } catch (err) {
      viewError.value = err instanceof Error ? err.message : String(err)
    }
  }
}

function onRenameKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') { e.preventDefault(); commitRename() }
  if (e.key === 'Escape') { isRenaming.value = false }
}

// ── Resize ─────────────────────────────────────────────────────────────────────
const DEFAULT_W = 360
const DEFAULT_H = 84
function startResize(e: MouseEvent, direction: 'e' | 's' | 'se') {
  emit('resizeStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY, startW: props.node.w ?? DEFAULT_W, startH: props.node.h ?? DEFAULT_H, direction })
}

// ── Create chart ───────────────────────────────────────────────────────────────
function createChart() {
  const n = schemaStore.nodes.filter((n) => n.kind === 'chart').length + 1
  schemaStore.addChartNode({
    id: `chart_${Date.now()}`,
    name: `chart_${n}`,
    x: props.node.x + (props.node.w ?? DEFAULT_W) + 40,
    y: props.node.y,
    sourceId: props.node.id,
    sql: '',
    chartType: 'barY',
    xColumn: '',
    yColumn: '',
  })
}

// ── Export ─────────────────────────────────────────────────────────────────────
const hasLimitInSql = computed(() => /\bLIMIT\b/i.test(localSql.value))
const exportPending = ref<ExportFormat | null>(null)
const isExporting = ref(false)
const exportError = ref<string | null>(null)

function onExportClick(fmt: ExportFormat) {
  exportError.value = null
  const r = nodeResult.value
  if (!r || r.isRunning || r.error) return
  if (hasLimitInSql.value) {
    exportPending.value = fmt
    return
  }
  exportData(fmt, r.columns, r.rows as Record<string, unknown>[], props.node.name)
  exportPending.value = null
}

async function exportWithoutLimit() {
  const fmt = exportPending.value
  if (!fmt) return
  const stripped = localSql.value.replace(/\bLIMIT\s+\d+(\s*,\s*\d+)?\b/gi, '').replace(/;?\s*$/, '')
  exportPending.value = null
  isExporting.value = true
  exportError.value = null
  try {
    const result = await query(stripped)
    exportData(fmt, result.columns, result.rows as Record<string, unknown>[], props.node.name)
  } catch (err) {
    exportError.value = err instanceof Error ? err.message : String(err)
  } finally {
    isExporting.value = false
  }
}

// ── SQL editing ────────────────────────────────────────────────────────────────
const localSql = ref(props.node.sql)
let sqlTimer: ReturnType<typeof setTimeout> | null = null

watch(() => props.node.sql, (v) => { if (v !== localSql.value) localSql.value = v })

function onSqlChange(value: string) {
  localSql.value = value
  if (sqlTimer) clearTimeout(sqlTimer)
  sqlTimer = setTimeout(() => {
    schemaStore.updateQuerySql(props.node.id, localSql.value)
    if (props.node.isView && localSql.value.trim()) {
      sqlCreateView(props.node.name, localSql.value).catch((err) => {
        viewError.value = err instanceof Error ? err.message : String(err)
      })
    }
  }, 400)
}

// ── Execution ──────────────────────────────────────────────────────────────────
const isRunning = ref(false)
const runError = ref<string | null>(null)
const runSummary = ref<string | null>(null)

async function run(e?: MouseEvent) {
  e?.stopPropagation()
  if (isRunning.value) return
  const sql = localSql.value.trim()
  if (!sql) return

  isRunning.value = true
  runError.value = null
  runSummary.value = null
  schemaStore.updateQuerySql(props.node.id, sql)

  setResult(props.node.id, { columns: [], rows: [], error: null, isRunning: true })

  try {
    const result = await query(sql)
    runSummary.value = `${result.rowCount.toLocaleString()} ${result.rowCount === 1 ? 'row' : 'rows'}, ${result.columns.length} cols`
    setResult(props.node.id, { columns: result.columns, columnTypes: result.columnTypes, rows: result.rows, error: null, isRunning: false })
    schemaStore.pushQueryHistory(props.node.id, sql)
    activeTab.value = 'results'
    fsResultsTab.value = 'results'
  } catch (err) {
    runError.value = err instanceof Error ? err.message : String(err)
    setResult(props.node.id, { columns: [], rows: [], error: runError.value, isRunning: false })
  } finally {
    isRunning.value = false
  }
}

// ── History ────────────────────────────────────────────────────────────────────
const history = computed<SqlHistoryEntry[]>(() => props.node.sqlHistory ?? [])
const _historyTick = ref(0)
let _historyTimer: ReturnType<typeof setInterval> | null = null

watch(() => activeTab.value === 'history', (open) => {
  if (open && !_historyTimer) _historyTimer = setInterval(() => { _historyTick.value++ }, 30_000)
  else if (!open && _historyTimer) { clearInterval(_historyTimer); _historyTimer = null }
})

function historyRelTime(ts: number): string {
  _historyTick.value // reactive dependency for auto-refresh
  const diff = Date.now() - ts
  if (diff < 60_000) return 'just now'
  if (diff < 3_600_000) return `${Math.round(diff / 60_000)}m ago`
  if (diff < 86_400_000) return `${Math.round(diff / 3_600_000)}h ago`
  return `${Math.round(diff / 86_400_000)}d ago`
}

function restoreHistory(entry: SqlHistoryEntry) {
  localSql.value = entry.sql
  schemaStore.updateQuerySql(props.node.id, entry.sql)
  activeTab.value = 'sql'
}


onMounted(() => {
  if (pendingFullscreenId.value === props.node.id) {
    pendingFullscreenId.value = null
    isFullscreen.value = true
  }
  if (!props.node.sql.trim()) return
  if (isAppReady.value) { run(); return }
  const stop = watch(isAppReady, (ready) => { if (ready) { stop(); run() } })
})

// ── Materialize ────────────────────────────────────────────────────────────────
const isMaterializing = ref(false)
const materializeName = ref('')
const materializeInputRef = ref<HTMLInputElement | null>(null)

function startMaterialize(e: MouseEvent) {
  e.stopPropagation()
  if (!localSql.value.trim()) return
  materializeName.value = `${props.node.name}_snapshot`
  isMaterializing.value = true
  nextTick(() => materializeInputRef.value?.select())
}

async function commitMaterialize() {
  const name = materializeName.value.trim()
  isMaterializing.value = false
  if (!name || !localSql.value.trim()) return
  const sql = localSql.value.trim().replace(/;+$/, '')
  const safe = name.replace(/"/g, '""')
  try {
    await exec(`CREATE OR REPLACE TABLE "${safe}" AS (${sql})`)
    const [columns, countResult] = await Promise.all([
      getTableInfo(name),
      query(`SELECT COUNT(*) AS n FROM "${safe}"`),
    ])
    const rowCount = Number(countResult.rows[0]?.n ?? 0)
    schemaStore.addDataNode({
      id: `data_${Date.now()}`,
      name,
      x: props.node.x + (props.node.w ?? DEFAULT_W) + 40,
      y: props.node.y,
      columns,
      rowCount,
      sourceId: props.node.id,
      sourceSql: sql,
    })
    if (IS_DESKTOP) {
      const { SaveTableData } = await import('../../wailsjs/go/main/App')
      await SaveTableData(name)
    }
  } catch (err) {
    runError.value = err instanceof Error ? err.message : String(err)
  }
}

function onMaterializeKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') { e.preventDefault(); commitMaterialize() }
  if (e.key === 'Escape') { isMaterializing.value = false }
}

// ── Auto-refresh ───────────────────────────────────────────────────────────────
const REFRESH_OPTIONS = [0, 5, 30, 60, 300, 1800]

function labelFor(sec: number): string {
  if (sec === 0) return 'Off'
  if (sec < 60) return `${sec}s`
  return `${sec / 60}m`
}

const intervalTimer = ref<ReturnType<typeof setInterval> | null>(null)
const _nextRefreshAt = ref(0)
const refreshCountdown = ref(0)
let _countdownTimer: ReturnType<typeof setInterval> | null = null

function _tickCountdown() {
  refreshCountdown.value = Math.max(0, Math.ceil((_nextRefreshAt.value - Date.now()) / 1000))
}

function startTimer() {
  if (intervalTimer.value) { clearInterval(intervalTimer.value); intervalTimer.value = null }
  if (_countdownTimer) { clearInterval(_countdownTimer); _countdownTimer = null }
  const ms = (props.node.refreshInterval ?? 0) * 1000
  if (ms > 0) {
    _nextRefreshAt.value = Date.now() + ms
    _tickCountdown()
    intervalTimer.value = setInterval(() => {
      _nextRefreshAt.value = Date.now() + ms
      run()
    }, ms)
    _countdownTimer = setInterval(_tickCountdown, 1000)
  } else {
    refreshCountdown.value = 0
  }
}

const refreshCountdownLabel = computed(() => {
  const s = refreshCountdown.value
  if (s <= 0) return '…'
  if (s < 60) return `${s}s`
  return `${Math.ceil(s / 60)}m`
})

function setRefresh(sec: number) {
  schemaStore.setRefreshInterval(props.node.id, sec)
  startTimer()
}

watch(() => props.node.refreshInterval, startTimer, { immediate: true })

onUnmounted(() => {
  if (intervalTimer.value) clearInterval(intervalTimer.value)
  if (_countdownTimer) clearInterval(_countdownTimer)
  if (_historyTimer) clearInterval(_historyTimer)
})
</script>

<template>
  <div
    class="query-card"
    :class="{ selected }"
    :data-node-id="node.id"
    :style="{ left: `${node.x}px`, top: `${node.y}px`, width: `${node.w ?? 360}px` }"
    @mousedown="onMouseDown"
    @contextmenu.prevent.stop="openNodeMenu(node.id, $event.clientX, $event.clientY)"
  >
    <!-- Header -->
    <div class="card-header" :style="{ background: node.color }">
      <svg class="card-icon" viewBox="0 0 16 16" fill="none">
        <polyline points="2,5 6,9 2,13" stroke="white" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        <line x1="8" y1="4" x2="14" y2="4" stroke="white" stroke-width="1.5" stroke-linecap="round"/>
        <line x1="8" y1="8" x2="14" y2="8" stroke="white" stroke-width="1.5" stroke-linecap="round"/>
        <line x1="8" y1="12" x2="14" y2="12" stroke="white" stroke-width="1.5" stroke-linecap="round"/>
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

      <button class="expand-btn" title="Fullscreen editor" @mousedown.stop @click.stop="toggleFullscreen">
        <svg viewBox="0 0 10 10" fill="none">
          <path d="M1 3.5V1h2.5M6.5 1H9v2.5M9 6.5V9H6.5M3.5 9H1V6.5" stroke="white" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
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
        :title="deleteConfirm ? 'Click again to confirm' : 'Delete query'"
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

    <!-- Collapsible body -->
    <template v-if="!isCollapsed">

    <!-- Tab bar -->
    <div class="card-tabs" @mousedown.stop>
      <button class="card-tab" :class="{ active: activeTab === 'sql' }" @click.stop="activeTab = 'sql'">SQL</button>
      <button class="card-tab" :class="{ active: activeTab === 'results' }" @click.stop="activeTab = 'results'">
        Results
        <span v-if="resultRowCount !== null" class="tab-badge">{{ resultRowCount.toLocaleString() }}</span>
      </button>
      <button class="card-tab" :class="{ active: activeTab === 'history' }" @click.stop="activeTab = 'history'">
        History
        <span v-if="history.length" class="tab-badge">{{ history.length }}</span>
      </button>
    </div>

    <!-- SQL editor -->
    <div v-show="activeTab === 'sql'" class="card-body" @mousedown.stop>
      <SqlEditor
        :model-value="localSql"
        :height="node.h ?? 84"
        :schema="sqlSchema"
        @update:model-value="onSqlChange"
        @run="run()"
      />
    </div>

    <!-- Results -->
    <div v-show="activeTab === 'results'" class="card-results" :style="{ height: `${node.h ?? 84}px` }" @mousedown.stop>
      <!-- Export bar -->
      <div v-if="nodeResult && !nodeResult.isRunning && !nodeResult.error" class="export-bar">
        <span class="export-count">{{ nodeResult.rows.length.toLocaleString() }} rows</span>
        <span v-if="hasLimitInSql" class="export-limit-warn" title="SQL has a LIMIT — result may be partial">⚠ LIMIT</span>
        <template v-if="!exportPending && !isExporting">
          <button v-for="fmt in (['csv','tsv','json','md'] as const)" :key="fmt" class="export-fmt-btn" @click.stop="onExportClick(fmt)">{{ fmt.toUpperCase() }}</button>
        </template>
        <span v-else-if="isExporting" class="export-status">Exporting…</span>
      </div>
      <!-- Limit confirmation -->
      <div v-if="exportPending" class="export-confirm" @mousedown.stop>
        <span>{{ nodeResult?.rows.length }} rows (limited). Export anyway?</span>
        <button class="export-confirm-btn" @click.stop="exportData(exportPending!, nodeResult!.columns, nodeResult!.rows as Record<string, unknown>[], node.name); exportPending = null">Export {{ nodeResult?.rows.length }}</button>
        <button class="export-confirm-btn accent" @click.stop="exportWithoutLimit">Re-run without LIMIT</button>
        <button class="export-dismiss" @click.stop="exportPending = null">✕</button>
      </div>
      <div v-if="exportError" class="export-error-msg" @mousedown.stop>{{ exportError }} <button @click.stop="exportError = null">✕</button></div>

      <template v-if="nodeResult && !nodeResult.isRunning && !nodeResult.error && nodeResult.rows.length">
        <div class="results-scroll">
          <table class="mini-table">
            <thead>
              <tr>
                <th v-for="(col, i) in nodeResult.columns" :key="col">
                  <span class="col-name">{{ col }}</span>
                  <span
                    class="col-type-badge"
                    :class="`type-${classifyColumnType(nodeResult.columnTypes?.[i]).category}`"
                  >{{ classifyColumnType(nodeResult.columnTypes?.[i]).label }}</span>
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, i) in displayRows" :key="i">
                <td v-for="col in nodeResult.columns" :key="col" :class="{ 'is-null': row[col] == null }">
                  {{ row[col] == null ? 'NULL' : String(row[col]) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="isCapped" class="results-cap-notice">
          Showing {{ TABLE_DISPLAY_CAP.toLocaleString() }} of {{ nodeResult.rows.length.toLocaleString() }} rows — export for full data
        </div>
      </template>
      <div v-else-if="nodeResult?.isRunning" class="results-state">Running…</div>
      <div v-else-if="nodeResult?.error" class="results-state results-error">{{ nodeResult.error }}</div>
      <div v-else-if="nodeResult && !nodeResult.rows.length" class="results-state">No rows returned</div>
      <div v-else class="results-state">Run to see results</div>
    </div>

    <!-- History -->
    <div v-show="activeTab === 'history'" class="card-history" :style="{ height: `${node.h ?? 84}px` }" @mousedown.stop>
      <div v-if="!history.length" class="history-empty">No history yet — run a query to start tracking</div>
      <div v-else class="history-list">
        <div v-for="(entry, i) in history" :key="i" class="history-entry" @click.stop="restoreHistory(entry)">
          <div class="history-meta">
            <span class="history-time">{{ historyRelTime(entry.ts) }}</span>
            <button class="history-restore" title="Restore this query" @click.stop="restoreHistory(entry)">Restore</button>
          </div>
          <pre class="history-sql">{{ entry.sql.length > 200 ? entry.sql.slice(0, 200) + '…' : entry.sql }}</pre>
        </div>
      </div>
    </div>

    <!-- View error -->
    <div v-if="viewError" class="view-error-msg" @mousedown.stop>
      {{ viewError.split('\n')[0] }} <button @click.stop="viewError = null">✕</button>
    </div>

    <!-- Footer -->
    <div class="card-footer">
      <span class="footer-status">
        <span v-if="runSummary" class="status-ok">{{ runSummary }}</span>
        <span v-else class="status-hint">⌘↵ to run</span>
      </span>
      <span v-if="(node.refreshInterval ?? 0) > 0" class="refresh-countdown" title="Next auto-refresh">
        ↻ {{ refreshCountdownLabel }}
      </span>
      <select
        class="refresh-select"
        :class="{ 'refresh-active': (node.refreshInterval ?? 0) > 0 }"
        :value="node.refreshInterval ?? 0"
        @mousedown.stop
        @change.stop="setRefresh(+($event.target as HTMLSelectElement).value)"
      >
        <option v-for="sec in REFRESH_OPTIONS" :key="sec" :value="sec">{{ labelFor(sec) }}</option>
      </select>
      <button class="ghost-btn" title="Create a ChartCard linked to this query" @mousedown.stop @click.stop="createChart">
        <svg viewBox="0 0 10 10" fill="none">
          <rect x="0.5" y="0.5" width="9" height="9" rx="1.5" stroke="currentColor" stroke-width="1.1"/>
          <polyline points="2,7 4,4 6,6 8,3" stroke="currentColor" stroke-width="1.1" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        Chart
      </button>
      <button
        class="ghost-btn"
        :class="{ 'view-active': node.isView }"
        :title="node.isView ? 'Drop DuckDB VIEW' : 'Publish as DuckDB VIEW'"
        :disabled="isViewLoading"
        @mousedown.stop
        @click.stop="toggleView"
      >
        <svg viewBox="0 0 10 10" fill="none">
          <circle cx="5" cy="5" r="3.5" stroke="currentColor" stroke-width="1.1"/>
          <circle cx="5" cy="5" r="1.3" fill="currentColor"/>
        </svg>
        View
      </button>
      <template v-if="isMaterializing">
        <input
          ref="materializeInputRef"
          v-model="materializeName"
          class="materialize-input"
          placeholder="table_name"
          @mousedown.stop
          @keydown="onMaterializeKeydown"
          @blur="commitMaterialize"
        />
      </template>
      <button
        v-else
        class="ghost-btn"
        title="Materialize query result as a data table node"
        @mousedown.stop
        @click.stop="startMaterialize"
      >
        <svg viewBox="0 0 10 10" fill="none">
          <ellipse cx="5" cy="3" rx="3" ry="1.2" stroke="currentColor" stroke-width="1.1"/>
          <path d="M2 3v3.8c0 .66 1.34 1.2 3 1.2s3-.54 3-1.2V3" stroke="currentColor" stroke-width="1.1"/>
        </svg>
        → Data
      </button>
      <button
        class="run-btn"
        :class="{ running: isRunning }"
        @mousedown.stop
        @click.stop="run"
      >
        <svg v-if="!isRunning" viewBox="0 0 10 10" fill="none">
          <path d="M2 1.5l7 3.5-7 3.5V1.5z" fill="currentColor"/>
        </svg>
        <svg v-else class="spin" viewBox="0 0 12 12" fill="none">
          <circle cx="6" cy="6" r="4" stroke="currentColor" stroke-width="2" stroke-dasharray="8 14" stroke-linecap="round"/>
        </svg>
        {{ isRunning ? 'Running…' : 'Run' }}
      </button>
    </div>

    <!-- Run error -->
    <div v-if="runError" class="card-run-error" @mousedown.stop>
      <pre>{{ runError }}</pre>
      <button class="run-error-close" @click.stop="runError = null">✕</button>
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

    </template><!-- end collapsible body -->
  </div>

  <!-- ── Fullscreen overlay ──────────────────────────────────────────────── -->
  <Teleport to="body">
    <div v-if="isFullscreen" class="qf-overlay" :class="{ 'qf-overlay--inset': IS_DESKTOP, [`qf-pos-${fsResultsPos}`]: true }" @mousedown.stop @contextmenu.stop>
      <div class="qf-panel">

        <!-- Header -->
        <div class="qf-header" :style="{ background: node.color }">
          <svg class="card-icon" viewBox="0 0 16 16" fill="none">
            <polyline points="2,5 6,9 2,13" stroke="white" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
            <line x1="8" y1="4" x2="14" y2="4" stroke="white" stroke-width="1.5" stroke-linecap="round"/>
            <line x1="8" y1="8" x2="14" y2="8" stroke="white" stroke-width="1.5" stroke-linecap="round"/>
            <line x1="8" y1="12" x2="14" y2="12" stroke="white" stroke-width="1.5" stroke-linecap="round"/>
          </svg>
          <span class="qf-name">{{ node.name }}</span>
          <span class="qf-esc-hint">Esc to close</span>
          <button class="qf-close-btn" title="Exit fullscreen (Esc)" @click.stop="isFullscreen = false">
            <svg viewBox="0 0 10 10" fill="none">
              <path d="M1 1l8 8M9 1L1 9" stroke="white" stroke-width="1.4" stroke-linecap="round"/>
            </svg>
          </button>
        </div>

        <!-- Split body: editor pane + results pane side by side or stacked -->
        <div class="qf-split-body">

          <!-- Editor pane -->
          <div class="qf-editor-pane">
            <SqlEditor
              :model-value="localSql"
              :schema="sqlSchema"
              @update:model-value="onSqlChange"
              @run="run()"
            />
          </div>

          <!-- Results pane -->
          <div class="qf-results-pane">
            <!-- Pane tab bar -->
            <div class="qf-results-tabbar">
              <button class="qf-results-tab" :class="{ active: fsResultsTab === 'results' }" @click.stop="fsResultsTab = 'results'">
                Results
                <span v-if="resultRowCount !== null" class="tab-badge">{{ resultRowCount.toLocaleString() }}</span>
              </button>
              <button class="qf-results-tab" :class="{ active: fsResultsTab === 'history' }" @click.stop="fsResultsTab = 'history'">
                History
                <span v-if="history.length" class="tab-badge">{{ history.length }}</span>
              </button>
              <div class="qf-tabbar-spacer" />
              <!-- Layout position toggles -->
              <div class="qf-layout-btns">
                <button class="qf-layout-btn" :class="{ active: fsResultsPos === 'bottom' }" title="Results at bottom" @click.stop="fsResultsPos = 'bottom'">
                  <svg viewBox="0 0 14 14" fill="none">
                    <rect x="1" y="1" width="12" height="12" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
                    <line x1="1" y1="8" x2="13" y2="8" stroke="currentColor" stroke-width="1.2"/>
                  </svg>
                </button>
                <button class="qf-layout-btn" :class="{ active: fsResultsPos === 'right' }" title="Results at right" @click.stop="fsResultsPos = 'right'">
                  <svg viewBox="0 0 14 14" fill="none">
                    <rect x="1" y="1" width="12" height="12" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
                    <line x1="8.5" y1="1" x2="8.5" y2="13" stroke="currentColor" stroke-width="1.2"/>
                  </svg>
                </button>
                <button class="qf-layout-btn" :class="{ active: fsResultsPos === 'left' }" title="Results at left" @click.stop="fsResultsPos = 'left'">
                  <svg viewBox="0 0 14 14" fill="none">
                    <rect x="1" y="1" width="12" height="12" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
                    <line x1="5.5" y1="1" x2="5.5" y2="13" stroke="currentColor" stroke-width="1.2"/>
                  </svg>
                </button>
              </div>
            </div>

            <!-- Results content -->
            <div v-show="fsResultsTab === 'results'" class="qf-pane-content">
              <div v-if="nodeResult && !nodeResult.isRunning && !nodeResult.error" class="export-bar">
                <span class="export-count">{{ nodeResult.rows.length.toLocaleString() }} rows</span>
                <span v-if="hasLimitInSql" class="export-limit-warn" title="SQL has a LIMIT — result may be partial">⚠ LIMIT</span>
                <template v-if="!exportPending && !isExporting">
                  <button v-for="fmt in (['csv','tsv','json','md'] as const)" :key="fmt" class="export-fmt-btn" @click.stop="onExportClick(fmt)">{{ fmt.toUpperCase() }}</button>
                </template>
                <span v-else-if="isExporting" class="export-status">Exporting…</span>
              </div>
              <div v-if="exportPending" class="export-confirm">
                <span>{{ nodeResult?.rows.length }} rows (limited). Export anyway?</span>
                <button class="export-confirm-btn" @click.stop="exportData(exportPending!, nodeResult!.columns, nodeResult!.rows as Record<string, unknown>[], node.name); exportPending = null">Export {{ nodeResult?.rows.length }}</button>
                <button class="export-confirm-btn accent" @click.stop="exportWithoutLimit">Re-run without LIMIT</button>
                <button class="export-dismiss" @click.stop="exportPending = null">✕</button>
              </div>
              <div v-if="exportError" class="export-error-msg">{{ exportError }} <button @click.stop="exportError = null">✕</button></div>
              <template v-if="nodeResult && !nodeResult.isRunning && !nodeResult.error && nodeResult.rows.length">
                <div class="results-scroll">
                  <table class="mini-table">
                    <thead>
                      <tr>
                        <th v-for="(col, i) in nodeResult.columns" :key="col">
                          <span class="col-name">{{ col }}</span>
                          <span class="col-type-badge" :class="`type-${classifyColumnType(nodeResult.columnTypes?.[i]).category}`">{{ classifyColumnType(nodeResult.columnTypes?.[i]).label }}</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="(row, ri) in displayRows" :key="ri">
                        <td v-for="col in nodeResult.columns" :key="col" :class="{ 'is-null': row[col] == null }">
                          {{ row[col] == null ? 'NULL' : String(row[col]) }}
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
                <div v-if="isCapped" class="results-cap-notice">
                  Showing {{ TABLE_DISPLAY_CAP.toLocaleString() }} of {{ nodeResult.rows.length.toLocaleString() }} rows — export for full data
                </div>
              </template>
              <div v-else-if="nodeResult?.isRunning" class="results-state">Running…</div>
              <div v-else-if="nodeResult?.error" class="results-state results-error">{{ nodeResult.error }}</div>
              <div v-else-if="nodeResult && !nodeResult.rows.length" class="results-state">No rows returned</div>
              <div v-else class="results-state">Run to see results</div>
            </div>

            <!-- History content -->
            <div v-show="fsResultsTab === 'history'" class="qf-pane-content">
              <div v-if="!history.length" class="history-empty">No history yet — run a query to start tracking</div>
              <div v-else class="history-list">
                <div v-for="(entry, i) in history" :key="i" class="history-entry" @click.stop="restoreHistory(entry)">
                  <div class="history-meta">
                    <span class="history-time">{{ historyRelTime(entry.ts) }}</span>
                    <button class="history-restore" @click.stop="restoreHistory(entry)">Restore</button>
                  </div>
                  <pre class="history-sql">{{ entry.sql.length > 400 ? entry.sql.slice(0, 400) + '…' : entry.sql }}</pre>
                </div>
              </div>
            </div>

            <!-- Pane errors -->
            <div v-if="viewError" class="view-error-msg">
              {{ viewError.split('\n')[0] }} <button @click.stop="viewError = null">✕</button>
            </div>
            <div v-if="runError" class="card-run-error">
              <pre>{{ runError }}</pre>
              <button class="run-error-close" @click.stop="runError = null">✕</button>
            </div>
          </div>

        </div><!-- end qf-split-body -->

        <!-- Footer -->
        <div class="card-footer qf-footer">
          <span class="footer-status">
            <span v-if="runSummary" class="status-ok">{{ runSummary }}</span>
            <span v-else class="status-hint">⌘↵ to run</span>
          </span>
          <span v-if="(node.refreshInterval ?? 0) > 0" class="refresh-countdown" title="Next auto-refresh">
            ↻ {{ refreshCountdownLabel }}
          </span>
          <select
            class="refresh-select"
            :class="{ 'refresh-active': (node.refreshInterval ?? 0) > 0 }"
            :value="node.refreshInterval ?? 0"
            @change.stop="setRefresh(+($event.target as HTMLSelectElement).value)"
          >
            <option v-for="sec in REFRESH_OPTIONS" :key="sec" :value="sec">{{ labelFor(sec) }}</option>
          </select>
          <button class="ghost-btn" title="Create a ChartCard linked to this query" @click.stop="createChart">
            <svg viewBox="0 0 10 10" fill="none">
              <rect x="0.5" y="0.5" width="9" height="9" rx="1.5" stroke="currentColor" stroke-width="1.1"/>
              <polyline points="2,7 4,4 6,6 8,3" stroke="currentColor" stroke-width="1.1" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            Chart
          </button>
          <button
            class="ghost-btn"
            :class="{ 'view-active': node.isView }"
            :title="node.isView ? 'Drop DuckDB VIEW' : 'Publish as DuckDB VIEW'"
            :disabled="isViewLoading"
            @click.stop="toggleView"
          >
            <svg viewBox="0 0 10 10" fill="none">
              <circle cx="5" cy="5" r="3.5" stroke="currentColor" stroke-width="1.1"/>
              <circle cx="5" cy="5" r="1.3" fill="currentColor"/>
            </svg>
            View
          </button>
          <template v-if="isMaterializing">
            <input
              ref="materializeInputRef"
              v-model="materializeName"
              class="materialize-input"
              placeholder="table_name"
              @keydown="onMaterializeKeydown"
              @blur="commitMaterialize"
            />
          </template>
          <button v-else class="ghost-btn" title="Materialize query result as a data table node" @click.stop="startMaterialize">
            <svg viewBox="0 0 10 10" fill="none">
              <ellipse cx="5" cy="3" rx="3" ry="1.2" stroke="currentColor" stroke-width="1.1"/>
              <path d="M2 3v3.8c0 .66 1.34 1.2 3 1.2s3-.54 3-1.2V3" stroke="currentColor" stroke-width="1.1"/>
            </svg>
            → Data
          </button>
          <button class="run-btn" :class="{ running: isRunning }" @click.stop="run">
            <svg v-if="!isRunning" viewBox="0 0 10 10" fill="none">
              <path d="M2 1.5l7 3.5-7 3.5V1.5z" fill="currentColor"/>
            </svg>
            <svg v-else class="spin" viewBox="0 0 12 12" fill="none">
              <circle cx="6" cy="6" r="4" stroke="currentColor" stroke-width="2" stroke-dasharray="8 14" stroke-linecap="round"/>
            </svg>
            {{ isRunning ? 'Running…' : 'Run' }}
          </button>
        </div>

      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.query-card {
  position: absolute;
  width: 280px; /* overridden by inline style when w is set */
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface-1);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
  user-select: none;
  cursor: grab;
  transition: box-shadow 0.15s, border-color 0.15s;
}

.query-card:active { cursor: grabbing; }

.query-card.selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px rgba(88, 166, 255, 0.2), 0 4px 16px rgba(0, 0, 0, 0.4);
}

/* ── Global user-select guard ─────────────────────────────────────────────── */
.card-header, .card-name, .card-tabs, .card-footer,
.ghost-btn, .run-btn, .collapse-btn, .delete-btn,
.status-hint, .status-ok, .footer-status {
  user-select: none;
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
  width: 13px;
  height: 13px;
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

.expand-btn {
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
  opacity: 0;
  flex-shrink: 0;
  transition: opacity 0.15s, background 0.15s;
}
.expand-btn svg { width: 10px; height: 10px; }
.expand-btn:hover { opacity: 1; background: rgba(255, 255, 255, 0.15); }
.query-card:hover .expand-btn { opacity: 0.45; }

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
  background: rgba(0, 0, 0, 0.35);
}

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

.query-card:hover .delete-btn,
.delete-btn.confirming { opacity: 1; }

.delete-btn:hover { background: rgba(248, 81, 73, 0.35); }

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

/* ── Tab bar ─────────────────────────────────────────────────────────────── */
.card-tabs {
  display: flex;
  border-bottom: 1px solid var(--border);
  background: var(--surface-1);
  gap: 0;
}

.card-tab {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 0 10px;
  height: 28px;
  font-size: 10.5px;
  font-weight: 500;
  color: var(--text-muted);
  background: transparent;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  transition: color 0.12s, border-color 0.12s;
  margin-bottom: -1px;
}

.card-tab:hover { color: var(--text-secondary); }

.card-tab.active {
  color: var(--accent);
  border-bottom-color: var(--accent);
}

.tab-badge {
  font-size: 9.5px;
  padding: 1px 4px;
  border-radius: 6px;
  background: var(--surface-2);
  color: var(--text-muted);
}

.card-tab.active .tab-badge {
  background: rgba(88, 166, 255, 0.12);
  color: var(--accent);
}

/* ── Body ────────────────────────────────────────────────────────────────── */
.card-body {
  padding: 0;
}

.card-body :deep(.sql-editor-wrap) {
  border-bottom: 1px solid var(--border);
  background: var(--surface-0);
}

/* ── Results ─────────────────────────────────────────────────────────────── */
.card-results {
  overflow: hidden;
  border-bottom: 1px solid var(--border);
  background: var(--surface-0);
  display: flex;
  flex-direction: column;
  min-height: 60px;
}

.results-scroll {
  flex: 1;
  overflow: auto;
  height: 100%;
}

.mini-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 10.5px;
  font-family: var(--font-mono);
}

.mini-table thead th {
  position: sticky;
  top: 0;
  background: var(--surface-2);
  color: var(--text-muted);
  font-size: 9.5px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 3px 8px;
  text-align: left;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}

.mini-table thead th .col-name { margin-right: 4px; }

.col-type-badge {
  display: inline-block;
  font-size: 8px;
  font-weight: 700;
  letter-spacing: 0.03em;
  padding: 1px 3px;
  border-radius: 3px;
  vertical-align: middle;
  opacity: 0.85;
}
.type-int   { background: #1e3a5f; color: #7eb8f7; }
.type-float { background: #1e3a40; color: #6dd5c8; }
.type-text  { background: #2d2d1e; color: #d4c97a; }
.type-bool  { background: #2a1e3a; color: #c084f5; }
.type-date  { background: #1e3a28; color: #6dcc8a; }
.type-other { background: var(--surface-1); color: var(--text-muted); }

.mini-table tbody tr:hover td { background: var(--surface-1); }

.mini-table tbody td {
  padding: 2.5px 8px;
  color: var(--text-primary);
  border-bottom: 1px solid var(--surface-2);
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  user-select: text;
}

.mini-table tbody td.is-null {
  color: var(--text-muted);
  font-style: italic;
}

.results-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10.5px;
  color: var(--text-muted);
  font-style: italic;
  padding: 8px;
  user-select: text;
}

.results-error { color: var(--error); font-style: normal; }

.results-cap-notice {
  flex-shrink: 0;
  padding: 4px 10px;
  font-size: 10px;
  color: var(--text-muted);
  background: var(--surface-0);
  border-top: 1px solid var(--border);
  text-align: center;
}

/* ── Footer ──────────────────────────────────────────────────────────────── */
.card-footer {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  padding: 5px 8px 6px 10px;
  gap: 6px;
}

.footer-status {
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.status-ok {
  font-size: 10.5px;
  font-family: var(--font-mono);
  color: var(--success);
}

.status-hint {
  font-size: 10.5px;
  color: var(--text-muted);
  font-style: italic;
}

.run-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  font-size: 10.5px;
  font-weight: 600;
  background: var(--accent);
  color: #0d1117;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.15s;
}

.run-btn svg { width: 10px; height: 10px; }
.run-btn:hover { background: var(--accent-hover); }
.run-btn.running { opacity: 0.7; cursor: not-allowed; }

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

.ghost-btn.view-active {
  color: var(--success, #3fb950);
  border-color: rgba(63, 185, 80, 0.35);
  background: rgba(63, 185, 80, 0.08);
}
.ghost-btn.view-active:hover { background: rgba(63, 185, 80, 0.16); }

.materialize-input {
  width: 100px;
  padding: 2px 5px;
  font-size: 10px;
  font-family: var(--font-mono);
  background: var(--surface-0);
  border: 1px solid var(--accent);
  border-radius: 4px;
  color: var(--text-primary);
  outline: none;
  flex-shrink: 0;
}

.view-error-msg {
  padding: 3px 8px;
  font-size: 10px;
  color: var(--error);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  background: var(--surface-1);
  user-select: text;
}

.view-error-msg button {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 10px;
  padding: 0 2px;
}

.card-run-error {
  border-top: 1px solid rgba(248, 81, 73, 0.3);
  background: rgba(248, 81, 73, 0.08);
  padding: 6px 8px;
  display: flex;
  align-items: flex-start;
  gap: 6px;
  flex-shrink: 0;
}

.card-run-error pre {
  margin: 0;
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--error);
  white-space: pre-wrap;
  word-break: break-all;
  flex: 1;
  max-height: 150px;
  overflow-y: auto;
  line-height: 1.5;
}

.run-error-close {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 10px;
  padding: 0 2px;
  flex-shrink: 0;
  line-height: 1;
}

/* ── Export bar ──────────────────────────────────────────────────────────── */
.export-bar {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  border-bottom: 1px solid var(--border);
  background: var(--surface-1);
  flex-shrink: 0;
}

.export-count {
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--text-muted);
  flex: 1;
}

.export-limit-warn {
  font-size: 9.5px;
  color: #f59e0b;
  font-weight: 600;
  flex-shrink: 0;
}

.export-fmt-btn {
  padding: 1px 5px;
  font-size: 9.5px;
  font-weight: 600;
  font-family: var(--font-mono);
  background: var(--surface-2);
  color: var(--text-muted);
  border: 1px solid var(--border);
  border-radius: 3px;
  cursor: pointer;
  transition: background 0.1s, color 0.1s;
  flex-shrink: 0;
}

.export-fmt-btn:hover { background: var(--accent); color: #0d1117; border-color: var(--accent); }

.export-status {
  font-size: 10px;
  color: var(--text-muted);
  font-style: italic;
  flex: 1;
  text-align: right;
}

.export-confirm {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border-bottom: 1px solid #f59e0b40;
  background: rgba(245, 158, 11, 0.06);
  flex-shrink: 0;
  flex-wrap: wrap;
}

.export-confirm span {
  font-size: 10px;
  color: #f59e0b;
  flex: 1;
  min-width: 100%;
  margin-bottom: 2px;
}

.export-confirm-btn {
  padding: 2px 7px;
  font-size: 10px;
  font-weight: 600;
  background: var(--surface-2);
  color: var(--text-secondary);
  border: 1px solid var(--border);
  border-radius: 3px;
  cursor: pointer;
  transition: background 0.1s;
}

.export-confirm-btn:hover { background: var(--surface-0); color: var(--text-primary); }
.export-confirm-btn.accent { background: var(--accent); color: #0d1117; border-color: transparent; }
.export-confirm-btn.accent:hover { background: var(--accent-hover); }

.export-dismiss {
  padding: 2px 5px;
  font-size: 10px;
  background: transparent;
  color: var(--text-muted);
  border: none;
  cursor: pointer;
  border-radius: 3px;
  transition: color 0.1s;
}

.export-dismiss:hover { color: var(--text-primary); }

.export-error-msg {
  padding: 3px 8px;
  font-size: 10px;
  color: var(--error);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  user-select: text;
}

.export-error-msg button {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 10px;
  padding: 0 2px;
}

.spin { animation: spin 0.8s linear infinite; }

@keyframes spin { to { transform: rotate(360deg); } }

/* ── History tab ─────────────────────────────────────────────────────────── */
.card-history {
  overflow: hidden;
  border-bottom: 1px solid var(--border);
  background: var(--surface-0);
  display: flex;
  flex-direction: column;
}

.history-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10.5px;
  color: var(--text-muted);
  font-style: italic;
  padding: 12px;
  text-align: center;
}

.history-list {
  flex: 1;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: var(--border) transparent;
}

.history-entry {
  padding: 6px 10px;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  transition: background 0.1s;
}

.history-entry:last-child { border-bottom: none; }
.history-entry:hover { background: var(--surface-1); }
.history-entry:hover .history-restore { opacity: 1; }

.history-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 3px;
}

.history-time {
  font-size: 9.5px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.history-restore {
  font-size: 9.5px;
  font-weight: 600;
  padding: 1px 6px;
  background: rgba(88, 166, 255, 0.1);
  border: 1px solid rgba(88, 166, 255, 0.3);
  border-radius: 3px;
  color: var(--accent);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.1s, background 0.1s;
}

.history-restore:hover { background: rgba(88, 166, 255, 0.2); }

.history-sql {
  margin: 0;
  font-size: 9.5px;
  font-family: var(--font-mono);
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.45;
  max-height: 60px;
  overflow: hidden;
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

.query-card:hover .rh-e,
.query-card:hover .rh-s,
.query-card:hover .rh-se,
.query-card.selected .rh-e,
.query-card.selected .rh-s,
.query-card.selected .rh-se { opacity: 1; }

/* ── Refresh select ──────────────────────────────────────────────────────── */
.refresh-select {
  padding: 2px 4px;
  font-size: 10px;
  color: var(--text-muted);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  cursor: pointer;
  outline: none;
  flex-shrink: 0;
}
.refresh-select.refresh-active {
  color: var(--success);
  border-color: rgba(63, 185, 80, 0.4);
  background: rgba(63, 185, 80, 0.08);
}

.refresh-countdown {
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--success);
  flex-shrink: 0;
  opacity: 0.8;
  min-width: 3ch;
  text-align: right;
}

/* ── Fullscreen overlay ───────────────────────────────────────────────────── */
.qf-overlay {
  position: fixed;
  inset: 0;
  z-index: 9900;
  background: var(--bg, #0d1117);
  display: flex;
  flex-direction: column;
}

/* Offset for macOS TitleBarHiddenInset — toolbar is 44px and contains traffic lights */
.qf-overlay--inset {
  top: 44px;
}

.qf-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.qf-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  flex-shrink: 0;
  border-bottom: 1px solid rgba(255,255,255,0.08);
}

.qf-name {
  font-size: 13px;
  font-weight: 600;
  color: white;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: 0.02em;
}

.qf-esc-hint {
  font-size: 10.5px;
  color: rgba(255, 255, 255, 0.45);
  flex-shrink: 0;
}

.qf-close-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 4px;
  border: none;
  background: rgba(255, 255, 255, 0.08);
  color: white;
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.15s;
}
.qf-close-btn svg { width: 10px; height: 10px; }
.qf-close-btn:hover { background: rgba(248, 81, 73, 0.4); }

/* ── Split body ───────────────────────────────────────────────────────────── */
.qf-split-body {
  flex: 1;
  min-height: 0;
  display: flex;
  overflow: hidden;
}

.qf-pos-bottom .qf-split-body { flex-direction: column; }
.qf-pos-right  .qf-split-body { flex-direction: row; }
.qf-pos-left   .qf-split-body { flex-direction: row-reverse; }

/* ── Editor pane ──────────────────────────────────────────────────────────── */
.qf-editor-pane {
  flex: 1;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: var(--surface-0);
}

.qf-editor-pane :deep(.sql-editor-wrap) {
  height: 100%;
  border-bottom: none;
}

.qf-editor-pane :deep(.cm-editor) {
  height: 100%;
}

/* ── Results pane ─────────────────────────────────────────────────────────── */
.qf-results-pane {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--surface-0);
}

.qf-pos-bottom .qf-results-pane {
  height: 42%;
  min-height: 100px;
  border-top: 1px solid var(--border);
}

.qf-pos-right .qf-results-pane {
  width: 44%;
  min-width: 200px;
  border-left: 1px solid var(--border);
}

.qf-pos-left .qf-results-pane {
  width: 44%;
  min-width: 200px;
  border-right: 1px solid var(--border);
}

/* ── Results pane tab bar ─────────────────────────────────────────────────── */
.qf-results-tabbar {
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--border);
  background: var(--surface-1);
  flex-shrink: 0;
}

.qf-results-tab {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 0 10px;
  height: 30px;
  font-size: 10.5px;
  font-weight: 500;
  color: var(--text-muted);
  background: transparent;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  transition: color 0.12s, border-color 0.12s;
  margin-bottom: -1px;
  flex-shrink: 0;
}

.qf-results-tab:hover { color: var(--text-secondary); }
.qf-results-tab.active { color: var(--accent); border-bottom-color: var(--accent); }

.qf-tabbar-spacer { flex: 1; }

/* ── Layout position toggle buttons ──────────────────────────────────────── */
.qf-layout-btns {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 0 6px;
  flex-shrink: 0;
}

.qf-layout-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: none;
  border-radius: 3px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  transition: background 0.12s, color 0.12s;
}
.qf-layout-btn svg { width: 13px; height: 13px; }
.qf-layout-btn:hover { background: var(--surface-2); color: var(--text-primary); }
.qf-layout-btn.active { background: var(--surface-2); color: var(--accent); }

/* ── Results pane content area ────────────────────────────────────────────── */
.qf-pane-content {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.qf-footer {
  border-top: 1px solid var(--border);
  border-radius: 0;
  flex-shrink: 0;
}
</style>
