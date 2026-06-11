<script setup lang="ts">
import { ref, watch } from 'vue'
import { useDuckDB, type QueryResult } from '../composables/useDuckDB'
import { useSchemaStore } from '../stores/schema'
import { useQueryBridge } from '../composables/useQueryBridge'
import { useTableOps } from '../composables/useTableOps'
import { usePersistence } from '../composables/usePersistence'

const { isReady, query, getTableInfo } = useDuckDB()
const { saveTable } = usePersistence()
const schemaStore = useSchemaStore()
const { pendingQuery } = useQueryBridge()
const { dropTable } = useTableOps()

// Consume queries pushed from canvas cards
watch(pendingQuery, (val) => {
  if (val !== null) {
    sql.value = val
    activeTab.value = 'query'
    pendingQuery.value = null
  }
})

// ── Tabs ──────────────────────────────────────────────────────────────────────
type Tab = 'query' | 'schema'
const activeTab = ref<Tab>('query')

// ── Query tab state ───────────────────────────────────────────────────────────
const sql = ref('SELECT * FROM users LIMIT 20;')
const result = ref<QueryResult | null>(null)
const queryError = ref<string | null>(null)
const isRunning = ref(false)

// ── Schema tab state ──────────────────────────────────────────────────────────
const isRefreshingStats = ref(false)

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Refresh row counts for every table currently in the schema store. */
async function refreshStats() {
  isRefreshingStats.value = true
  const tables = schemaStore.nodes.filter((n) => n.kind === 'table')
  await Promise.allSettled(
    tables.map(async (table) => {
      try {
        const r = await query(`SELECT COUNT(*) AS n FROM "${table.name}"`)
        const count = r.rows[0]?.n
        if (count !== undefined) schemaStore.setRowCount(table.id, Number(count))
      } catch {
        // Table may not exist in DuckDB (manually added node, etc.)
      }
    }),
  )
  isRefreshingStats.value = false
}

/** Parse all table names from CREATE TABLE statements in a SQL string. */
function parseCreatedTables(sqlText: string): string[] {
  const re = /CREATE\s+(?:OR\s+REPLACE\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?["'`]?(\w+)["'`]?/gi
  return [...sqlText.matchAll(re)].map((m) => m[1])
}

/** Pick a canvas position for a newly discovered table. */
function nextPosition(): { x: number; y: number } {
  if (!schemaStore.nodes.length) return { x: 60, y: 80 }
  const maxX = Math.max(...schemaStore.nodes.map((n) => n.x))
  const paired = schemaStore.nodes.find((n) => n.x === maxX)!
  return { x: maxX + 280, y: paired.y }
}

/** Fetch DuckDB schema for a freshly created table and add it to the canvas. */
async function syncCreatedTable(name: string) {
  if (schemaStore.nodes.some((n) => n.kind === 'table' && n.name === name)) return
  try {
    const columns = await getTableInfo(name)
    const { x, y } = nextPosition()
    schemaStore.addTable({ id: name, name, x, y, columns })
    saveTable(name).catch(console.warn)
  } catch (e) {
    console.warn('Failed to sync table to canvas:', name, e)
  }
}

// ── Run query ─────────────────────────────────────────────────────────────────
async function runQuery() {
  if (!isReady.value || isRunning.value) return
  const trimmed = sql.value.trim()
  if (!trimmed) return

  isRunning.value = true
  queryError.value = null
  result.value = null

  try {
    result.value = await query(trimmed)

    // Sync any CREATE TABLE statements to the canvas
    const newTables = parseCreatedTables(trimmed)
    for (const name of newTables) {
      await syncCreatedTable(name)
    }

    // Refresh row counts for all known tables
    await refreshStats()
  } catch (e) {
    queryError.value = e instanceof Error ? e.message : String(e)
  } finally {
    isRunning.value = false
  }
}

function onKeyDown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
    e.preventDefault()
    runQuery()
  }
}

// ── Schema tab actions ────────────────────────────────────────────────────────
function selectTable(name: string) {
  sql.value = `SELECT * FROM "${name}" LIMIT 100;`
  activeTab.value = 'query'
}

const confirmingDelete = ref<string | null>(null)
let confirmTimer: ReturnType<typeof setTimeout> | null = null

function onSchemaDeleteClick(e: MouseEvent, id: string, name: string) {
  e.stopPropagation()
  if (confirmingDelete.value !== id) {
    confirmingDelete.value = id
    if (confirmTimer) clearTimeout(confirmTimer)
    confirmTimer = setTimeout(() => { confirmingDelete.value = null }, 3000)
  } else {
    if (confirmTimer) clearTimeout(confirmTimer)
    confirmingDelete.value = null
    dropTable(id, name)
  }
}

// ── Result helpers ────────────────────────────────────────────────────────────
function formatCell(val: unknown): string {
  if (val === null || val === undefined) return 'NULL'
  return String(val)
}

function isNull(val: unknown): boolean {
  return val === null || val === undefined
}

// ── Panel resize ──────────────────────────────────────────────────────────────
const emit = defineEmits<{ close: [] }>()
const panelWidth = ref(420)

function onResizeHandleMouseDown(e: MouseEvent) {
  e.preventDefault()
  const startX = e.clientX
  const startW = panelWidth.value

  function onMove(ev: MouseEvent) {
    panelWidth.value = Math.max(280, Math.min(900, startW + (startX - ev.clientX)))
  }
  function onUp() {
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

defineExpose({ refreshStats })
</script>

<template>
  <div class="query-panel" :style="{ width: panelWidth + 'px' }">
    <div class="panel-resize-handle" @mousedown="onResizeHandleMouseDown" />
    <!-- Tab bar -->
    <div class="tab-bar">
      <button
        class="tab-btn"
        :class="{ active: activeTab === 'query' }"
        @click="activeTab = 'query'"
      >
        <svg viewBox="0 0 14 14" fill="none">
          <polyline points="1,4 5,8 1,12" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
          <line x1="7" y1="11" x2="13" y2="11" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          <line x1="7" y1="7" x2="13" y2="7" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          <line x1="7" y1="3" x2="13" y2="3" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
        </svg>
        Query
      </button>
      <button
        class="tab-btn"
        :class="{ active: activeTab === 'schema' }"
        @click="activeTab = 'schema'"
      >
        <svg viewBox="0 0 14 14" fill="none">
          <rect x="1" y="1" width="12" height="12" rx="2" stroke="currentColor" stroke-width="1.3"/>
          <line x1="1" y1="5" x2="13" y2="5" stroke="currentColor" stroke-width="1.3"/>
          <line x1="5" y1="5" x2="5" y2="13" stroke="currentColor" stroke-width="1.3"/>
        </svg>
        Schema
        <span class="tab-count">{{ schemaStore.nodes.filter(n => n.kind === 'table').length }}</span>
      </button>
      <div class="tab-spacer" />
      <button class="close-btn" title="Hide panel" @click="emit('close')">
        <svg viewBox="0 0 12 12" fill="none">
          <path d="M9 3L3 9M3 3l6 6" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
        </svg>
      </button>
    </div>

    <!-- ── Query tab ─────────────────────────────────────────────────────── -->
    <template v-if="activeTab === 'query'">
      <div class="editor-area">
        <textarea
          v-model="sql"
          class="sql-input"
          placeholder="SELECT * FROM users;"
          spellcheck="false"
          @keydown="onKeyDown"
        />
        <div class="editor-actions">
          <button
            class="run-btn"
            :disabled="!isReady || isRunning"
            @click="runQuery"
          >
            <svg v-if="!isRunning" viewBox="0 0 14 14" fill="none">
              <path d="M3 2l9 5-9 5V2z" fill="currentColor"/>
            </svg>
            <svg v-else class="spin" viewBox="0 0 14 14" fill="none">
              <circle cx="7" cy="7" r="5" stroke="currentColor" stroke-width="2" stroke-dasharray="10 18" stroke-linecap="round"/>
            </svg>
            {{ isRunning ? 'Running…' : 'Run' }}
            <kbd>⌘↵</kbd>
          </button>
        </div>
      </div>

      <div class="results-area">
        <div v-if="queryError" class="error-box">
          <svg viewBox="0 0 14 14" fill="none">
            <circle cx="7" cy="7" r="5.5" stroke="#f85149" stroke-width="1.3"/>
            <line x1="7" y1="4.5" x2="7" y2="7.5" stroke="#f85149" stroke-width="1.3" stroke-linecap="round"/>
            <circle cx="7" cy="9.5" r="0.7" fill="#f85149"/>
          </svg>
          <pre>{{ queryError }}</pre>
        </div>

        <template v-else-if="result">
          <div class="results-meta">
            <span class="meta-rows">
              {{ result.rowCount.toLocaleString() }} {{ result.rowCount === 1 ? 'row' : 'rows' }}
            </span>
            <span class="meta-time">{{ result.durationMs.toFixed(1) }}ms</span>
          </div>

          <div class="table-wrap" v-if="result.rowCount > 0">
            <table class="results-table">
              <thead>
                <tr>
                  <th v-for="col in result.columns" :key="col">{{ col }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, i) in result.rows" :key="i">
                  <td
                    v-for="col in result.columns"
                    :key="col"
                    :class="{ 'is-null': isNull(row[col]) }"
                  >{{ formatCell(row[col]) }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-else class="empty-state">No rows returned</div>
        </template>

        <div v-else class="empty-state">Run a query to see results</div>
      </div>
    </template>

    <!-- ── Schema tab ────────────────────────────────────────────────────── -->
    <template v-else-if="activeTab === 'schema'">
      <div class="schema-header">
        <span class="schema-title">{{ schemaStore.nodes.filter(n => n.kind === 'table').length }} tables</span>
        <button
          class="refresh-btn"
          :class="{ spinning: isRefreshingStats }"
          :disabled="!isReady || isRefreshingStats"
          title="Refresh row counts"
          @click="refreshStats"
        >
          <svg viewBox="0 0 14 14" fill="none" :class="{ spin: isRefreshingStats }">
            <path d="M12 7A5 5 0 1 1 7 2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            <polyline points="12,2 12,6 8,6" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          Refresh
        </button>
      </div>

      <div class="schema-list">
        <div v-if="!schemaStore.nodes.filter(n => n.kind === 'table').length" class="empty-state">
          No tables yet — create one with SQL or upload a CSV
        </div>

        <div
          v-for="table in schemaStore.nodes.filter(n => n.kind === 'table')"
          :key="table.id"
          class="table-row"
          @click="selectTable(table.name)"
        >
          <span class="table-dot" :style="{ background: table.color }" />
          <span class="table-name">{{ table.name }}</span>
          <span class="table-stats">
            <span class="stat-cols">{{ table.columns.length }}c</span>
            <span class="stat-sep">·</span>
            <span class="stat-rows" v-if="table.rowCount !== undefined">
              {{ table.rowCount.toLocaleString() }}r
            </span>
            <span class="stat-rows loading" v-else>…</span>
          </span>
          <svg class="table-arrow" viewBox="0 0 10 10" fill="none">
            <path d="M2 5h6M5 2l3 3-3 3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          <button
            class="schema-delete-btn"
            :class="{ confirming: confirmingDelete === table.id }"
            :title="confirmingDelete === table.id ? 'Click again to confirm' : 'Delete table'"
            @click.stop="onSchemaDeleteClick($event, table.id, table.name)"
          >
            <span v-if="confirmingDelete === table.id">Delete?</span>
            <svg v-else viewBox="0 0 12 12" fill="none">
              <path d="M2 3.5h8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
              <path d="M4.5 3.5V2.5h3v1" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M3.5 3.5l.7 6h3.6l.7-6" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </button>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.query-panel {
  width: 420px; /* overridden by inline style */
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  border-left: 1px solid var(--border);
  background: var(--surface-0);
  overflow: hidden;
  position: relative;
}

.panel-resize-handle {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 5px;
  cursor: col-resize;
  z-index: 10;
}

.panel-resize-handle:hover,
.panel-resize-handle:active {
  background: rgba(88, 166, 255, 0.15);
}

/* ── Tabs ─────────────────────────────────────────────────────────────────── */
.tab-bar {
  display: flex;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
  gap: 0;
}

.tab-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 0 14px;
  height: 40px;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  background: transparent;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s;
  margin-bottom: -1px;
}

.tab-btn svg {
  width: 13px;
  height: 13px;
}

.tab-btn:hover {
  color: var(--text-secondary);
}

.tab-btn.active {
  color: var(--accent);
  border-bottom-color: var(--accent);
}

.tab-count {
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 8px;
  background: var(--surface-2);
  color: var(--text-muted);
  margin-left: 2px;
}

.tab-spacer { flex: 1; }

.close-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 40px;
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  transition: color 0.15s;
  flex-shrink: 0;
}

.close-btn svg { width: 12px; height: 12px; }
.close-btn:hover { color: var(--text-primary); }

/* ── Query tab ────────────────────────────────────────────────────────────── */
.editor-area {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  border-bottom: 1px solid var(--border);
}

.sql-input {
  width: 100%;
  min-height: 120px;
  resize: vertical;
  background: var(--surface-1);
  color: var(--text-primary);
  border: none;
  outline: none;
  padding: 12px 14px;
  font-size: 12.5px;
  line-height: 1.6;
  font-family: var(--font-mono);
  tab-size: 2;
}

.sql-input::placeholder {
  color: var(--text-muted);
}

.editor-actions {
  padding: 8px 14px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.run-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  background: var(--accent);
  color: #0d1117;
  border: none;
  border-radius: 5px;
  font-size: 12px;
  font-weight: 600;
  transition: background 0.15s, opacity 0.15s;
}

.run-btn svg {
  width: 12px;
  height: 12px;
}

.run-btn:hover:not(:disabled) {
  background: var(--accent-hover);
}

.run-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.run-btn kbd {
  font-family: var(--font-mono);
  font-size: 10px;
  opacity: 0.6;
  margin-left: 2px;
}

.results-area {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.error-box {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 12px 14px;
  background: rgba(248, 81, 73, 0.08);
  border-bottom: 1px solid rgba(248, 81, 73, 0.2);
  flex-shrink: 0;
}

.error-box svg {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  margin-top: 1px;
}

.error-box pre {
  font-size: 11.5px;
  font-family: var(--font-mono);
  color: #f85149;
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
}

.results-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.meta-rows {
  font-size: 11px;
  color: var(--success);
  font-family: var(--font-mono);
}

.meta-time {
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.table-wrap {
  flex: 1;
  overflow: auto;
}

.results-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 11.5px;
  font-family: var(--font-mono);
}

.results-table thead th {
  position: sticky;
  top: 0;
  background: var(--surface-2);
  color: var(--text-secondary);
  font-size: 10.5px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding: 5px 10px;
  text-align: left;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}

.results-table tbody tr:hover {
  background: var(--surface-1);
}

.results-table tbody td {
  padding: 4px 10px;
  color: var(--text-primary);
  border-bottom: 1px solid var(--surface-2);
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.results-table tbody td.is-null {
  color: var(--text-muted);
  font-style: italic;
}

/* ── Schema tab ───────────────────────────────────────────────────────────── */
.schema-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.schema-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.refresh-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  font-size: 11px;
  color: var(--text-secondary);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  cursor: pointer;
  transition: color 0.15s, background 0.15s;
}

.refresh-btn:hover:not(:disabled) {
  color: var(--text-primary);
  background: var(--surface-3);
}

.refresh-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.refresh-btn svg {
  width: 12px;
  height: 12px;
}

.schema-list {
  flex: 1;
  overflow-y: auto;
}

.table-row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 9px 14px;
  background: transparent;
  border: none;
  border-bottom: 1px solid var(--surface-2);
  cursor: pointer;
  text-align: left;
  transition: background 0.1s;
}

.table-row:hover {
  background: var(--surface-1);
}

.table-row:hover .table-arrow {
  opacity: 1;
  transform: translateX(2px);
}

.table-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.table-name {
  flex: 1;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.table-stats {
  display: flex;
  align-items: center;
  gap: 4px;
  font-family: var(--font-mono);
  font-size: 10.5px;
  color: var(--text-muted);
  flex-shrink: 0;
}

.stat-sep {
  opacity: 0.4;
}

.stat-rows.loading {
  opacity: 0.4;
  font-style: italic;
}

.table-arrow {
  width: 12px;
  height: 12px;
  color: var(--text-muted);
  opacity: 0.4;
  flex-shrink: 0;
  transition: opacity 0.15s, transform 0.15s;
}

.schema-delete-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  opacity: 0;
  flex-shrink: 0;
  transition: opacity 0.15s, background 0.15s, color 0.15s, width 0.15s;
  padding: 0;
}

.schema-delete-btn svg {
  width: 11px;
  height: 11px;
}

.table-row:hover .schema-delete-btn,
.schema-delete-btn.confirming {
  opacity: 1;
}

.schema-delete-btn:hover {
  background: rgba(248, 81, 73, 0.12);
  color: #f85149;
}

.schema-delete-btn.confirming {
  background: rgba(248, 81, 73, 0.2);
  color: #f85149;
  width: auto;
  padding: 0 6px;
  font-size: 10px;
  font-weight: 700;
  white-space: nowrap;
}

/* ── Shared ───────────────────────────────────────────────────────────────── */
.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: var(--text-muted);
  padding: 24px 14px;
  text-align: center;
}

.spin {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
