<script setup lang="ts">
import { ref, nextTick, watch, computed, watchEffect, onMounted, onUnmounted } from 'vue'
import * as Plot from '@observablehq/plot'
import type { ChartNode } from '../stores/schema'
import { useSchemaStore } from '../stores/schema'
import { useDuckDB } from '../composables/useDuckDB'
import { useQueryResults } from '../composables/useQueryResults'
import { useAppReady } from '../composables/useAppReady'

const props = defineProps<{ node: ChartNode; selected?: boolean }>()
const emit = defineEmits<{
  dragStart: [{ id: string; mouseX: number; mouseY: number }]
  resizeStart: [{ id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }]
}>()

const schemaStore = useSchemaStore()
const { query } = useDuckDB()
const { results: queryResults } = useQueryResults()
const { isAppReady } = useAppReady()

// ── Drag ──────────────────────────────────────────────────────────────────────
function onMouseDown(e: MouseEvent) {
  if (e.button !== 0) return
  e.stopPropagation()
  emit('dragStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY })
}

// ── Resize ────────────────────────────────────────────────────────────────────
const DEFAULT_W = 340
const DEFAULT_H = 180
function startResize(e: MouseEvent, direction: 'e' | 's' | 'se') {
  emit('resizeStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY, startW: props.node.w ?? DEFAULT_W, startH: props.node.h ?? DEFAULT_H, direction })
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

// ── Data source ───────────────────────────────────────────────────────────────
const queryNodes = computed(() =>
  schemaStore.nodes.filter((n) => n.kind === 'query'),
)

const localSql = ref(props.node.sql)
watch(() => props.node.sql, (v) => { if (v !== localSql.value) localSql.value = v })

const inlineResult = ref<{ columns: string[]; rows: Record<string, unknown>[] } | null>(null)
const inlineError = ref<string | null>(null)
const isRunningInline = ref(false)

const effectiveData = computed(() => {
  if (props.node.sourceId) {
    const r = queryResults[props.node.sourceId]
    return (r && !r.error && !r.isRunning) ? r : null
  }
  return inlineResult.value
})

const availableColumns = computed(() => effectiveData.value?.columns ?? [])

async function runInline(e?: MouseEvent) {
  e?.stopPropagation()
  const sql = localSql.value.trim()
  if (!sql || isRunningInline.value) return
  isRunningInline.value = true
  inlineError.value = null
  schemaStore.updateChartConfig(props.node.id, { sql })
  try {
    const result = await query(sql)
    inlineResult.value = { columns: result.columns, rows: result.rows }
  } catch (err) {
    inlineError.value = err instanceof Error ? err.message : String(err)
    inlineResult.value = null
  } finally {
    isRunningInline.value = false
  }
}

function onInlineSqlKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') { e.preventDefault(); runInline() }
}

function setSource(val: string) {
  schemaStore.updateChartConfig(props.node.id, { sourceId: val === '__inline__' ? null : val })
}

function setChartType(t: ChartNode['chartType']) {
  schemaStore.updateChartConfig(props.node.id, { chartType: t })
}

function setXColumn(v: string) { schemaStore.updateChartConfig(props.node.id, { xColumn: v }) }
function setYColumn(v: string) { schemaStore.updateChartConfig(props.node.id, { yColumn: v }) }

// ── Observable Plot rendering ─────────────────────────────────────────────────
const chartContainer = ref<HTMLDivElement | null>(null)

watchEffect(() => {
  if (!chartContainer.value) return

  const data = effectiveData.value
  const { chartType, xColumn, yColumn, color } = props.node

  if (!data || !xColumn || !yColumn) {
    chartContainer.value.innerHTML = ''
    return
  }

  try {
    let mark: Plot.Markish
    switch (chartType) {
      case 'barY':  mark = Plot.barY(data.rows, { x: xColumn, y: yColumn, fill: color }); break
      case 'lineY': mark = Plot.lineY(data.rows, { x: xColumn, y: yColumn, stroke: color }); break
      case 'areaY': mark = Plot.areaY(data.rows, { x: xColumn, y: yColumn, fill: color, fillOpacity: 0.4, stroke: color }); break
      default:      mark = Plot.dot(data.rows, { x: xColumn, y: yColumn, fill: color })
    }

    const plotW = (props.node.w ?? 340) - 32
    const plotH = props.node.h ?? 180
    const el = Plot.plot({
      width: plotW,
      height: plotH,
      marginBottom: 36,
      marginLeft: 42,
      style: {
        background: 'none',
        color: '#8b949e',
        fontSize: '10px',
        overflow: 'visible',
      },
      marks: [mark, Plot.ruleY([0])],
    })

    chartContainer.value.replaceChildren(el)
  } catch {
    chartContainer.value.innerHTML = `<p class="chart-err">Can't render — check column types</p>`
  }
})

onMounted(() => {
  if (props.node.sourceId || !props.node.sql.trim()) return
  if (isAppReady.value) { runInline(); return }
  const stop = watch(isAppReady, (ready) => { if (ready) { stop(); runInline() } })
})

onUnmounted(() => {
  if (chartContainer.value) chartContainer.value.innerHTML = ''
})
</script>

<template>
  <div
    class="chart-card"
    :class="{ selected }"
    :style="{ left: `${node.x}px`, top: `${node.y}px`, width: `${node.w ?? 340}px` }"
    @mousedown="onMouseDown"
  >
    <!-- Header -->
    <div class="card-header" :style="{ background: node.color }">
      <svg class="card-icon" viewBox="0 0 16 16" fill="none">
        <rect x="1" y="1" width="14" height="14" rx="2" stroke="white" stroke-width="1.4"/>
        <polyline points="3,11 6,6 9,9 13,4" stroke="white" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
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

      <button
        class="delete-btn"
        :class="{ confirming: deleteConfirm }"
        :title="deleteConfirm ? 'Click again to confirm' : 'Delete chart'"
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

    <!-- Config -->
    <div class="card-config" @mousedown.stop>
      <!-- Source -->
      <div class="config-row">
        <span class="config-label">Source</span>
        <select class="config-select" :value="node.sourceId ?? '__inline__'" @change="setSource(($event.target as HTMLSelectElement).value)">
          <option value="__inline__">Inline SQL</option>
          <option v-for="qn in queryNodes" :key="qn.id" :value="qn.id">
            Query: {{ qn.name }}
          </option>
        </select>
      </div>

      <!-- Inline SQL (shown only when no sourceId) -->
      <div v-if="!node.sourceId" class="inline-sql-section">
        <textarea
          v-model="localSql"
          class="inline-sql-editor"
          spellcheck="false"
          placeholder="SELECT category, SUM(revenue) FROM sales GROUP BY 1"
          @keydown="onInlineSqlKeydown"
        />
        <div class="inline-sql-footer">
          <span v-if="inlineError" class="inline-error" :title="inlineError">{{ inlineError.split('\n')[0] }}</span>
          <span v-else class="inline-hint">⌘↵ to run</span>
          <button class="run-btn" @click.stop="runInline">
            <svg v-if="!isRunningInline" viewBox="0 0 10 10" fill="none">
              <path d="M2 1.5l7 3.5-7 3.5V1.5z" fill="currentColor"/>
            </svg>
            <svg v-else class="spin" viewBox="0 0 12 12" fill="none">
              <circle cx="6" cy="6" r="4" stroke="currentColor" stroke-width="2" stroke-dasharray="8 14" stroke-linecap="round"/>
            </svg>
            Run
          </button>
        </div>
      </div>

      <!-- Source-linked: show query status -->
      <div v-else-if="node.sourceId" class="source-status">
        <template v-if="queryResults[node.sourceId]?.isRunning">
          <span class="source-running">Running query…</span>
        </template>
        <template v-else-if="queryResults[node.sourceId]?.error">
          <span class="source-error">Query error</span>
        </template>
        <template v-else-if="!queryResults[node.sourceId]">
          <span class="source-hint">Run the query node to load data</span>
        </template>
        <template v-else>
          <span class="source-ok">{{ queryResults[node.sourceId].rows.length.toLocaleString() }} rows</span>
        </template>
      </div>

      <!-- Chart type -->
      <div class="config-row">
        <span class="config-label">Type</span>
        <div class="type-pills">
          <button
            v-for="t in (['barY', 'lineY', 'areaY', 'dot'] as const)"
            :key="t"
            class="type-pill"
            :class="{ active: node.chartType === t }"
            @click.stop="setChartType(t)"
          >{{ { barY: 'Bar', lineY: 'Line', areaY: 'Area', dot: 'Scatter' }[t] }}</button>
        </div>
      </div>

      <!-- Column selectors -->
      <div class="config-row">
        <span class="config-label">X</span>
        <select class="config-select" :value="node.xColumn" @change="setXColumn(($event.target as HTMLSelectElement).value)">
          <option value="">— pick column —</option>
          <option v-for="col in availableColumns" :key="col" :value="col">{{ col }}</option>
        </select>
        <span class="config-label">Y</span>
        <select class="config-select" :value="node.yColumn" @change="setYColumn(($event.target as HTMLSelectElement).value)">
          <option value="">— pick column —</option>
          <option v-for="col in availableColumns" :key="col" :value="col">{{ col }}</option>
        </select>
      </div>
    </div>

    <!-- Chart area -->
    <div class="chart-area" :style="{ height: `${(node.h ?? 180) + 24}px` }">
      <div ref="chartContainer" class="chart-plot" />
      <div v-if="!effectiveData || !node.xColumn || !node.yColumn" class="chart-placeholder">
        <svg viewBox="0 0 32 32" fill="none">
          <rect x="2" y="2" width="28" height="28" rx="3" stroke="currentColor" stroke-width="1.5"/>
          <polyline points="6,22 11,14 16,18 22,10 26,13" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <span>{{ !effectiveData ? 'Run a query to load data' : 'Pick X and Y columns above' }}</span>
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
.chart-card {
  position: absolute;
  width: 340px; /* overridden by inline style when w is set */
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface-1);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
  user-select: none;
  cursor: grab;
  transition: box-shadow 0.15s, border-color 0.15s;
}

.chart-card:active { cursor: grabbing; }

.chart-card.selected {
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

.card-icon { width: 14px; height: 14px; flex-shrink: 0; opacity: 0.9; }

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

.chart-card:hover .delete-btn,
.delete-btn.confirming { opacity: 1; }

.delete-btn:hover { background: rgba(248, 81, 73, 0.35); }

.delete-btn.confirming {
  background: rgba(248, 81, 73, 0.5);
  width: auto;
  padding: 0 6px;
}

.confirm-label { font-size: 10px; font-weight: 700; white-space: nowrap; letter-spacing: 0.02em; }

/* ── Config panel ────────────────────────────────────────────────────────── */
.card-config {
  padding: 8px 10px 6px;
  border-bottom: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 6px;
  cursor: default;
}

.config-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.config-label {
  font-size: 10.5px;
  color: var(--text-muted);
  flex-shrink: 0;
  width: 36px;
  text-align: right;
}

.config-select {
  flex: 1;
  min-width: 0;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 11px;
  padding: 3px 6px;
  outline: none;
  font-family: var(--font-mono);
  cursor: pointer;
}

.config-select:focus { border-color: var(--accent); }

.type-pills {
  display: flex;
  gap: 4px;
  flex: 1;
}

.type-pill {
  flex: 1;
  padding: 2px 0;
  font-size: 10.5px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-muted);
  cursor: pointer;
  transition: background 0.12s, color 0.12s, border-color 0.12s;
}

.type-pill:hover { color: var(--text-primary); }

.type-pill.active {
  background: rgba(88, 166, 255, 0.12);
  border-color: rgba(88, 166, 255, 0.4);
  color: var(--accent);
}

/* ── Inline SQL ──────────────────────────────────────────────────────────── */
.inline-sql-section {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.inline-sql-editor {
  width: 100%;
  min-height: 54px;
  resize: none;
  background: var(--surface-0);
  color: var(--text-primary);
  border: 1px solid var(--border);
  border-radius: 4px;
  outline: none;
  padding: 6px 8px;
  font-size: 11px;
  line-height: 1.5;
  font-family: var(--font-mono);
  tab-size: 2;
  box-sizing: border-box;
  cursor: text;
}

.inline-sql-editor:focus { border-color: var(--accent); }
.inline-sql-editor::placeholder { color: var(--text-muted); }

.inline-sql-footer {
  display: flex;
  align-items: center;
  gap: 6px;
}

.inline-hint, .inline-error, .source-hint, .source-running, .source-error, .source-ok {
  flex: 1;
  font-size: 10.5px;
  font-family: var(--font-mono);
}

.inline-hint, .source-hint { color: var(--text-muted); font-style: italic; }
.inline-error, .source-error { color: var(--error); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.source-running { color: var(--accent); }
.source-ok { color: var(--success); }

.source-status { padding: 0 0 2px; }

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

.run-btn svg { width: 9px; height: 9px; }
.run-btn:hover { background: var(--accent-hover); }

/* ── Chart area ──────────────────────────────────────────────────────────── */
.chart-area {
  position: relative;
  min-height: 100px;
  padding: 8px 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.chart-plot {
  width: 100%;
}

.chart-plot :deep(svg) {
  max-width: 100%;
}

.chart-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  color: var(--text-muted);
  font-size: 11.5px;
  padding: 20px;
  text-align: center;
}

.chart-placeholder svg {
  width: 32px;
  height: 32px;
  opacity: 0.3;
}

.chart-err {
  font-size: 11px;
  color: var(--error);
  padding: 8px;
  text-align: center;
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

.chart-card:hover .rh-e,
.chart-card:hover .rh-s,
.chart-card:hover .rh-se,
.chart-card.selected .rh-e,
.chart-card.selected .rh-s,
.chart-card.selected .rh-se { opacity: 1; }
</style>
