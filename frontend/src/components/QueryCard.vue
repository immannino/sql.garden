<script setup lang="ts">
import { ref, nextTick, watch, computed, onMounted, onUnmounted } from 'vue'
import type { QueryNode } from '../stores/schema'
import { useSchemaStore } from '../stores/schema'
import { useDuckDB } from '../composables/useDuckDB'
import { useQueryResults } from '../composables/useQueryResults'
import { useAppReady } from '../composables/useAppReady'
import { CreateView, DropView } from '../../wailsjs/go/main/App'

const props = defineProps<{ node: QueryNode; selected?: boolean }>()
const emit = defineEmits<{
  dragStart: [{ id: string; mouseX: number; mouseY: number; shiftKey: boolean }]
  resizeStart: [{ id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }]
}>()

const schemaStore = useSchemaStore()
const { query } = useDuckDB()
const { results: queryResults, setResult } = useQueryResults()
const { isAppReady } = useAppReady()

// ── Tabs ───────────────────────────────────────────────────────────────────────
const activeTab = ref<'sql' | 'results'>('sql')
const nodeResult = computed(() => queryResults[props.node.id] ?? null)
const resultRowCount = computed(() => {
  const r = nodeResult.value
  return (r && !r.error && !r.isRunning) ? r.rows.length : null
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
      await CreateView(props.node.name, sql)
      schemaStore.setQueryIsView(props.node.id, true)
    } else {
      await DropView(props.node.name)
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
      await DropView(oldName)
      if (localSql.value.trim()) await CreateView(newName, localSql.value)
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
const DEFAULT_W = 280
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
const exportPending = ref<'csv' | 'tsv' | 'json' | 'md' | null>(null)
const isExporting = ref(false)
const exportError = ref<string | null>(null)

function onExportClick(fmt: 'csv' | 'tsv' | 'json' | 'md') {
  exportError.value = null
  const r = nodeResult.value
  if (!r || r.isRunning || r.error) return
  if (hasLimitInSql.value) {
    exportPending.value = fmt
    return
  }
  doExport(fmt, r.columns, r.rows as Record<string, unknown>[])
}

function doExport(fmt: string, columns: string[], rows: Record<string, unknown>[]) {
  let content: string
  let mimeType: string
  let ext: string

  if (fmt === 'csv') {
    const esc = (v: unknown) => {
      const s = v === null || v === undefined ? '' : String(v)
      return s.includes(',') || s.includes('"') || s.includes('\n') ? `"${s.replace(/"/g, '""')}"` : s
    }
    content = [columns.join(','), ...rows.map((r) => columns.map((c) => esc(r[c])).join(','))].join('\n')
    mimeType = 'text/csv'; ext = 'csv'
  } else if (fmt === 'tsv') {
    content = [columns.join('\t'), ...rows.map((r) => columns.map((c) => String(r[c] ?? '')).join('\t'))].join('\n')
    mimeType = 'text/tab-separated-values'; ext = 'tsv'
  } else if (fmt === 'json') {
    content = JSON.stringify(rows, null, 2)
    mimeType = 'application/json'; ext = 'json'
  } else {
    const sep = '| ' + columns.map(() => '---').join(' | ') + ' |'
    const header = '| ' + columns.join(' | ') + ' |'
    const body = rows.map((r) => '| ' + columns.map((c) => String(r[c] ?? '')).join(' | ') + ' |').join('\n')
    content = [header, sep, body].join('\n')
    mimeType = 'text/plain'; ext = 'md'
  }

  const blob = new Blob([content], { type: mimeType })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${props.node.name}.${ext}`
  a.click()
  URL.revokeObjectURL(url)
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
    doExport(fmt, result.columns, result.rows as Record<string, unknown>[])
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

function onSqlInput() {
  if (sqlTimer) clearTimeout(sqlTimer)
  sqlTimer = setTimeout(() => {
    schemaStore.updateQuerySql(props.node.id, localSql.value)
    if (props.node.isView && localSql.value.trim()) {
      CreateView(props.node.name, localSql.value).catch((err) => {
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
    setResult(props.node.id, { columns: result.columns, rows: result.rows, error: null, isRunning: false })
    activeTab.value = 'results'
  } catch (err) {
    runError.value = err instanceof Error ? err.message : String(err)
    setResult(props.node.id, { columns: [], rows: [], error: runError.value, isRunning: false })
  } finally {
    isRunning.value = false
  }
}

function onKeyDown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') { e.preventDefault(); run() }
}

onMounted(() => {
  if (!props.node.sql.trim()) return
  if (isAppReady.value) { run(); return }
  const stop = watch(isAppReady, (ready) => { if (ready) { stop(); run() } })
})

// ── Auto-refresh ───────────────────────────────────────────────────────────────
const REFRESH_OPTIONS = [0, 5, 30, 60, 300, 1800]

function labelFor(sec: number): string {
  if (sec === 0) return 'Off'
  if (sec < 60) return `${sec}s`
  return `${sec / 60}m`
}

const intervalTimer = ref<ReturnType<typeof setInterval> | null>(null)

function startTimer() {
  if (intervalTimer.value) { clearInterval(intervalTimer.value); intervalTimer.value = null }
  const ms = (props.node.refreshInterval ?? 0) * 1000
  if (ms > 0) intervalTimer.value = setInterval(() => run(), ms)
}

function setRefresh(sec: number) {
  schemaStore.setRefreshInterval(props.node.id, sec)
  startTimer()
}

watch(() => props.node.refreshInterval, startTimer, { immediate: true })

onUnmounted(() => {
  if (intervalTimer.value) clearInterval(intervalTimer.value)
})
</script>

<template>
  <div
    class="query-card"
    :class="{ selected }"
    :style="{ left: `${node.x}px`, top: `${node.y}px`, width: `${node.w ?? 280}px` }"
    @mousedown="onMouseDown"
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
    </div>

    <!-- SQL editor -->
    <div v-show="activeTab === 'sql'" class="card-body" @mousedown.stop>
      <textarea
        v-model="localSql"
        class="sql-editor"
        spellcheck="false"
        placeholder="SELECT * FROM ..."
        :style="{ height: `${node.h ?? 84}px` }"
        @input="onSqlInput"
        @keydown="onKeyDown"
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
        <button class="export-confirm-btn" @click.stop="doExport(exportPending, nodeResult!.columns, nodeResult!.rows as Record<string, unknown>[])">Export {{ nodeResult?.rows.length }}</button>
        <button class="export-confirm-btn accent" @click.stop="exportWithoutLimit">Re-run without LIMIT</button>
        <button class="export-dismiss" @click.stop="exportPending = null">✕</button>
      </div>
      <div v-if="exportError" class="export-error-msg" @mousedown.stop>{{ exportError }} <button @click.stop="exportError = null">✕</button></div>

      <template v-if="nodeResult && !nodeResult.isRunning && !nodeResult.error && nodeResult.rows.length">
        <div class="results-scroll">
          <table class="mini-table">
            <thead>
              <tr><th v-for="col in nodeResult.columns" :key="col">{{ col }}</th></tr>
            </thead>
            <tbody>
              <tr v-for="(row, i) in nodeResult.rows" :key="i">
                <td v-for="col in nodeResult.columns" :key="col" :class="{ 'is-null': row[col] == null }">
                  {{ row[col] == null ? 'NULL' : String(row[col]) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
      <div v-else-if="nodeResult?.isRunning" class="results-state">Running…</div>
      <div v-else-if="nodeResult?.error" class="results-state results-error">{{ nodeResult.error.split('\n')[0] }}</div>
      <div v-else-if="nodeResult && !nodeResult.rows.length" class="results-state">No rows returned</div>
      <div v-else class="results-state">Run to see results</div>
    </div>

    <!-- View error -->
    <div v-if="viewError" class="view-error-msg" @mousedown.stop>
      {{ viewError.split('\n')[0] }} <button @click.stop="viewError = null">✕</button>
    </div>

    <!-- Footer -->
    <div class="card-footer">
      <span class="footer-status">
        <span v-if="runError" class="status-error" :title="runError">
          {{ runError.split('\n')[0] }}
        </span>
        <span v-else-if="runSummary" class="status-ok">{{ runSummary }}</span>
        <span v-else class="status-hint">⌘↵ to run</span>
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

.sql-editor {
  width: 100%;
  min-height: 60px;
  resize: none;
  background: var(--surface-0);
  color: var(--text-primary);
  border: none;
  border-bottom: 1px solid var(--border);
  outline: none;
  padding: 8px 10px;
  font-size: 11.5px;
  line-height: 1.6;
  font-family: var(--font-mono);
  tab-size: 2;
  display: block;
  cursor: text;
  box-sizing: border-box;
  overflow: auto;
}

.sql-editor::placeholder { color: var(--text-muted); }

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

.mini-table tbody tr:hover td { background: var(--surface-1); }

.mini-table tbody td {
  padding: 2.5px 8px;
  color: var(--text-primary);
  border-bottom: 1px solid var(--surface-2);
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
}

.results-error { color: var(--error); font-style: normal; }

/* ── Footer ──────────────────────────────────────────────────────────────── */
.card-footer {
  display: flex;
  align-items: center;
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

.status-error {
  font-size: 10.5px;
  font-family: var(--font-mono);
  color: var(--error);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
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
}

.view-error-msg button {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 10px;
  padding: 0 2px;
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
</style>
