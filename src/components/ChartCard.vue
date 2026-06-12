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
  dragStart: [{ id: string; mouseX: number; mouseY: number; shiftKey: boolean }]
  resizeStart: [{ id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }]
}>()

const schemaStore = useSchemaStore()
const { query } = useDuckDB()
const { results: queryResults, setResult } = useQueryResults()
const { isAppReady } = useAppReady()

// ── View mode ─────────────────────────────────────────────────────────────────
const isCollapsed  = computed(() => props.node.viewMode === 'collapsed')
const isChartOnly  = computed(() => props.node.viewMode === 'chart-only')

function toggleCollapsed(e: MouseEvent) {
  e.stopPropagation()
  schemaStore.updateViewMode(props.node.id, isCollapsed.value ? 'default' : 'collapsed')
}
function toggleChartOnly(e: MouseEvent) {
  e.stopPropagation()
  schemaStore.updateViewMode(props.node.id, isChartOnly.value ? 'default' : 'chart-only')
}

// ── Drag ──────────────────────────────────────────────────────────────────────
function onMouseDown(e: MouseEvent) {
  if (e.button !== 0) return
  e.stopPropagation()
  emit('dragStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY, shiftKey: e.shiftKey })
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

async function runLinked() {
  const sid = props.node.sourceId
  if (!sid || isRunningInline.value) return
  const sourceNode = schemaStore.nodes.find((n) => n.id === sid && n.kind === 'query')
  if (!sourceNode || sourceNode.kind !== 'query') return
  const sql = sourceNode.sql.trim()
  if (!sql) return
  isRunningInline.value = true
  setResult(sid, { columns: [], rows: [], error: null, isRunning: true })
  try {
    const result = await query(sql)
    setResult(sid, { columns: result.columns, rows: result.rows, error: null, isRunning: false })
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err)
    setResult(sid, { columns: [], rows: [], error: msg, isRunning: false })
  } finally {
    isRunningInline.value = false
  }
}

function createQuery() {
  const n = schemaStore.nodes.filter((n) => n.kind === 'query').length + 1
  schemaStore.addQueryNode({
    id: `query_${Date.now()}`,
    name: `query_${n}`,
    x: props.node.x + (props.node.w ?? 340) + 40,
    y: props.node.y,
    sql: props.node.sql,
  })
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
function setColorColumn(v: string) { schemaStore.updateChartConfig(props.node.id, { colorColumn: v || undefined }) }
function setLabelColumn(v: string) { schemaStore.updateChartConfig(props.node.id, { labelColumn: v || undefined }) }

const CHART_TYPES: { key: ChartNode['chartType']; label: string }[] = [
  { key: 'barY',  label: 'Bar' },
  { key: 'barX',  label: 'Bar ↔' },
  { key: 'lineY', label: 'Line' },
  { key: 'areaY', label: 'Area' },
  { key: 'dot',   label: 'Scatter' },
  { key: 'cell',  label: 'Heatmap' },
  { key: 'pie',   label: 'Pie' },
  { key: 'donut', label: 'Donut' },
]

// ── Pie / donut helpers (no d3-shape dep needed) ──────────────────────────────
const PIE_PALETTE = [
  '#4e79a7', '#f28e2b', '#e15759', '#76b7b2', '#59a14f',
  '#edc948', '#b07aa1', '#ff9da7', '#9c755f', '#bab0ac',
]

function polarXY(cx: number, cy: number, r: number, angle: number): [number, number] {
  return [cx + r * Math.sin(angle), cy - r * Math.cos(angle)]
}

function arcPath(
  cx: number, cy: number,
  outerR: number, innerR: number,
  startAngle: number, endAngle: number,
): string {
  const [ox1, oy1] = polarXY(cx, cy, outerR, startAngle)
  const [ox2, oy2] = polarXY(cx, cy, outerR, endAngle)
  const large = endAngle - startAngle > Math.PI ? 1 : 0
  if (innerR === 0) {
    return `M ${cx} ${cy} L ${ox1} ${oy1} A ${outerR} ${outerR} 0 ${large} 1 ${ox2} ${oy2} Z`
  }
  const [ix1, iy1] = polarXY(cx, cy, innerR, startAngle)
  const [ix2, iy2] = polarXY(cx, cy, innerR, endAngle)
  return `M ${ox1} ${oy1} A ${outerR} ${outerR} 0 ${large} 1 ${ox2} ${oy2} L ${ix2} ${iy2} A ${innerR} ${innerR} 0 ${large} 0 ${ix1} ${iy1} Z`
}

function buildPieSvg(
  rows: Record<string, unknown>[],
  xColumn: string,
  yColumn: string,
  width: number,
  height: number,
  isDonut: boolean,
): SVGSVGElement {
  const ns = 'http://www.w3.org/2000/svg'
  const legendW = 110
  const chartW = Math.max(60, width - legendW)
  const margin = 12
  const r = Math.min(chartW, height) / 2 - margin
  const innerR = isDonut ? r * 0.48 : 0
  const cx = chartW / 2
  const cy = height / 2

  const total = rows.reduce((s, d) => s + (Number(d[yColumn]) || 0), 0)

  const svg = document.createElementNS(ns, 'svg')
  svg.setAttribute('width', String(width))
  svg.setAttribute('height', String(height))
  Object.assign(svg.style, { overflow: 'visible', display: 'block' })

  const g = document.createElementNS(ns, 'g')
  svg.appendChild(g)

  let angle = 0
  const slices = rows.map((row, i) => {
    const value = Number(row[yColumn]) || 0
    const span = total > 0 ? (value / total) * 2 * Math.PI : 0
    const start = angle
    angle += span
    return { row, value, span, start, end: angle, color: PIE_PALETTE[i % PIE_PALETTE.length] }
  })

  // Slices
  slices.forEach((s) => {
    const path = document.createElementNS(ns, 'path')
    path.setAttribute('d', arcPath(cx, cy, r, innerR, s.start, s.end))
    path.setAttribute('fill', s.color)
    path.setAttribute('stroke', 'rgba(0,0,0,0.25)')
    path.setAttribute('stroke-width', '1')
    g.appendChild(path)

    // Label at centroid for slices ≥ 20°
    if (s.span > 0.35 && r > 40) {
      const midAngle = s.start + s.span / 2
      const labelR = innerR > 0 ? (innerR + r) / 2 : r * 0.65
      const [lx, ly] = polarXY(cx, cy, labelR, midAngle)
      const pct = total > 0 ? Math.round((s.value / total) * 100) : 0
      const text = document.createElementNS(ns, 'text')
      text.setAttribute('x', String(lx))
      text.setAttribute('y', String(ly))
      text.setAttribute('text-anchor', 'middle')
      text.setAttribute('dominant-baseline', 'middle')
      text.setAttribute('font-size', '9.5')
      text.setAttribute('fill', '#e2e8f0')
      text.setAttribute('pointer-events', 'none')
      text.textContent = `${pct}%`
      g.appendChild(text)
    }
  })

  // Donut center: total
  if (isDonut && r > 30) {
    const label = document.createElementNS(ns, 'text')
    label.setAttribute('x', String(cx))
    label.setAttribute('y', String(cy))
    label.setAttribute('text-anchor', 'middle')
    label.setAttribute('dominant-baseline', 'middle')
    label.setAttribute('font-size', '11')
    label.setAttribute('fill', '#8b949e')
    label.textContent = total.toLocaleString()
    g.appendChild(label)
  }

  // Legend
  const legendX = chartW + 8
  const rowH = 15
  const legendStartY = Math.max(4, (height - slices.length * rowH) / 2)
  slices.forEach((s, i) => {
    const gy = legendStartY + i * rowH
    const swatch = document.createElementNS(ns, 'rect')
    swatch.setAttribute('x', String(legendX))
    swatch.setAttribute('y', String(gy + 2))
    swatch.setAttribute('width', '8')
    swatch.setAttribute('height', '8')
    swatch.setAttribute('rx', '1.5')
    swatch.setAttribute('fill', s.color)
    svg.appendChild(swatch)

    const label = document.createElementNS(ns, 'text')
    label.setAttribute('x', String(legendX + 11))
    label.setAttribute('y', String(gy + 10))
    label.setAttribute('font-size', '9.5')
    label.setAttribute('fill', '#8b949e')
    label.textContent = String(s.row[xColumn] ?? '').slice(0, 12)
    svg.appendChild(label)
  })

  return svg
}

// ── Observable Plot rendering ─────────────────────────────────────────────────
const chartContainer = ref<HTMLDivElement | null>(null)

watchEffect(() => {
  if (!chartContainer.value) return

  const data = effectiveData.value
  const { chartType, xColumn, yColumn, colorColumn, labelColumn, color } = props.node

  if (!data || !xColumn || !yColumn) {
    chartContainer.value.innerHTML = ''
    return
  }

  try {
    const plotW = (props.node.w ?? 340) - 32
    const plotH = props.node.h ?? 180

    // Pie / donut — custom SVG renderer (no d3-shape needed)
    if (chartType === 'pie' || chartType === 'donut') {
      const el = buildPieSvg(data.rows, xColumn, yColumn, plotW, plotH, chartType === 'donut')
      chartContainer.value.replaceChildren(el)
      return
    }

    // Observable Plot charts
    const fill   = colorColumn ?? color
    const stroke = colorColumn ?? color
    const marks: Plot.Markish[] = []

    switch (chartType) {
      case 'barY':
        marks.push(Plot.barY(data.rows, { x: xColumn, y: yColumn, fill }))
        marks.push(Plot.ruleY([0]))
        break
      case 'barX':
        marks.push(Plot.barX(data.rows, { x: xColumn, y: yColumn, fill }))
        marks.push(Plot.ruleX([0]))
        break
      case 'lineY':
        marks.push(Plot.lineY(data.rows, { x: xColumn, y: yColumn, stroke }))
        marks.push(Plot.ruleY([0]))
        break
      case 'areaY':
        marks.push(Plot.areaY(data.rows, { x: xColumn, y: yColumn, fill, fillOpacity: 0.4, stroke }))
        marks.push(Plot.ruleY([0]))
        break
      case 'cell':
        marks.push(Plot.cell(data.rows, { x: xColumn, y: yColumn, fill }))
        break
      default: // dot / scatter
        marks.push(Plot.dot(data.rows, { x: xColumn, y: yColumn, fill }))
        break
    }

    if (labelColumn) {
      marks.push(Plot.text(data.rows, { x: xColumn, y: yColumn, text: labelColumn, fontSize: 9, fill: 'currentColor', dy: -6 }))
    }

    const el = Plot.plot({
      width: plotW,
      height: plotH,
      marginBottom: 36,
      marginLeft: 42,
      color: colorColumn ? { legend: true } : undefined,
      style: {
        background: 'none',
        color: '#8b949e',
        fontSize: '10px',
        overflow: 'visible',
      },
      marks,
    })

    chartContainer.value.replaceChildren(el)
  } catch {
    chartContainer.value.innerHTML = `<p class="chart-err">Can't render — check column types</p>`
  }
})

onMounted(() => {
  if (props.node.sourceId) {
    // Auto-run linked query if no cached result yet
    if (queryResults[props.node.sourceId]) return
    if (isAppReady.value) { runLinked(); return }
    const stop = watch(isAppReady, (ready) => { if (ready) { stop(); runLinked() } })
    return
  }
  if (!props.node.sql.trim()) return
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
        class="collapse-btn chartonly-btn"
        :class="{ active: isChartOnly }"
        title="Chart only / show config"
        @mousedown.stop
        @click.stop="toggleChartOnly"
      >
        <svg viewBox="0 0 10 10" fill="none">
          <rect x="1" y="1" width="8" height="8" rx="1" stroke="white" stroke-width="1.2"/>
          <polyline points="2,6.5 3.8,4.5 5.5,5.8 7.5,3" stroke="white" stroke-width="1.1" stroke-linecap="round" stroke-linejoin="round"/>
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

    <!-- Config (hidden when collapsed or chart-only) -->
    <div v-if="!isCollapsed && !isChartOnly" class="card-config" @mousedown.stop>
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
          <button v-if="localSql.trim()" class="ghost-btn" title="Create a QueryCard with this SQL" @click.stop="createQuery">
            <svg viewBox="0 0 10 10" fill="none">
              <polyline points="1,3 3,5 1,7" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
              <line x1="4" y1="2" x2="9" y2="2" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
              <line x1="4" y1="5" x2="9" y2="5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
              <line x1="4" y1="8" x2="9" y2="8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
            </svg>
            Query
          </button>
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

      <!-- Source-linked: show query status + refresh -->
      <div v-else-if="node.sourceId" class="source-status">
        <template v-if="queryResults[node.sourceId]?.isRunning">
          <span class="source-running">Running query…</span>
        </template>
        <template v-else-if="queryResults[node.sourceId]?.error">
          <span class="source-error">{{ queryResults[node.sourceId].error!.split('\n')[0] }}</span>
        </template>
        <template v-else-if="!queryResults[node.sourceId]">
          <span class="source-hint">Loading…</span>
        </template>
        <template v-else>
          <span class="source-ok">{{ queryResults[node.sourceId].rows.length.toLocaleString() }} rows</span>
        </template>
        <button class="run-btn" :disabled="isRunningInline" @click.stop="runLinked">
          <svg v-if="!isRunningInline" viewBox="0 0 10 10" fill="none">
            <path d="M1 5A4 4 0 1 1 5 9" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            <polyline points="1,2 1,5 4,5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          <svg v-else class="spin" viewBox="0 0 12 12" fill="none">
            <circle cx="6" cy="6" r="4" stroke="currentColor" stroke-width="2" stroke-dasharray="8 14" stroke-linecap="round"/>
          </svg>
          Refresh
        </button>
      </div>

      <!-- Chart type -->
      <div class="config-row">
        <span class="config-label">Type</span>
        <div class="type-pills">
          <button
            v-for="t in CHART_TYPES"
            :key="t.key"
            class="type-pill"
            :class="{ active: node.chartType === t.key }"
            @click.stop="setChartType(t.key)"
          >{{ t.label }}</button>
        </div>
      </div>

      <!-- Column selectors -->
      <div class="config-row">
        <span class="config-label">X</span>
        <select class="config-select" :value="node.xColumn" @change="setXColumn(($event.target as HTMLSelectElement).value)">
          <option value="">— pick —</option>
          <option v-for="col in availableColumns" :key="col" :value="col">{{ col }}</option>
        </select>
        <span class="config-label">Y</span>
        <select class="config-select" :value="node.yColumn" @change="setYColumn(($event.target as HTMLSelectElement).value)">
          <option value="">— pick —</option>
          <option v-for="col in availableColumns" :key="col" :value="col">{{ col }}</option>
        </select>
      </div>

      <!-- Color + Label encoding (only when columns are available) -->
      <div v-if="availableColumns.length" class="config-row">
        <span class="config-label">Color</span>
        <select class="config-select" :value="node.colorColumn ?? ''" @change="setColorColumn(($event.target as HTMLSelectElement).value)">
          <option value="">— none —</option>
          <option v-for="col in availableColumns" :key="col" :value="col">{{ col }}</option>
        </select>
        <span class="config-label">Label</span>
        <select class="config-select" :value="node.labelColumn ?? ''" @change="setLabelColumn(($event.target as HTMLSelectElement).value)">
          <option value="">— none —</option>
          <option v-for="col in availableColumns" :key="col" :value="col">{{ col }}</option>
        </select>
      </div>
    </div>

    <!-- Chart area (hidden only when collapsed) -->
    <div v-if="!isCollapsed" class="chart-area" :style="{ height: `${(node.h ?? 180) + 24}px` }">
      <div ref="chartContainer" class="chart-plot" />
      <div v-if="!effectiveData || !node.xColumn || !node.yColumn" class="chart-placeholder">
        <svg viewBox="0 0 32 32" fill="none">
          <rect x="2" y="2" width="28" height="28" rx="3" stroke="currentColor" stroke-width="1.5"/>
          <polyline points="6,22 11,14 16,18 22,10 26,13" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <span>{{ !effectiveData ? 'Run a query to load data' : 'Pick X and Y columns above' }}</span>
      </div>
    </div>

    <!-- Resize handles (hidden when collapsed) -->
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
.chartonly-btn.active { opacity: 1; background: rgba(255, 255, 255, 0.2); }

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
  flex-wrap: wrap;
  gap: 4px;
  flex: 1;
}

.type-pill {
  flex: 1 1 calc(33.333% - 4px);
  padding: 2px 4px;
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
.run-btn:hover:not(:disabled) { background: var(--accent-hover); }
.run-btn:disabled { opacity: 0.5; cursor: not-allowed; }

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
.ghost-btn:hover { background: var(--surface-3, var(--surface-2)); color: var(--text-primary); }

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
