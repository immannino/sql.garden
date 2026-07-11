<script setup lang="ts">
import { ref, nextTick, watch, computed, watchEffect, onMounted, onUnmounted } from 'vue'
import * as Plot from '@observablehq/plot'
import type { ChartNode, TableColumnConfig } from '../stores/schema'
import { useSchemaStore } from '../stores/schema'
import NodeColorPicker from './NodeColorPicker.vue'
import { useContextMenu } from '../composables/useContextMenu'

const { openNodeMenu } = useContextMenu()
import { useDuckDB } from '../composables/useDuckDB'
import { useQueryResults } from '../composables/useQueryResults'
import { useChartResults } from '../composables/useChartResults'
import { useChartPanel } from '../composables/useChartPanel'
import { useAppReady } from '../composables/useAppReady'
import { exportData, type ExportFormat } from '../lib/exportData'
import { IS_DESKTOP } from '../lib/env'

const props = defineProps<{ node: ChartNode; selected?: boolean }>()
const emit = defineEmits<{
  dragStart: [{ id: string; mouseX: number; mouseY: number; shiftKey: boolean }]
  resizeStart: [{ id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }]
}>()

const schemaStore = useSchemaStore()
const { query } = useDuckDB()
const { results: queryResults } = useQueryResults()
const { chartResults, setChartResult } = useChartResults()
const { openPanel } = useChartPanel()
const { isAppReady } = useAppReady()

// ── View mode ─────────────────────────────────────────────────────────────────
const isCollapsed = computed(() => props.node.viewMode === 'collapsed')
const isChartOnly = computed(() => props.node.viewMode === 'chart-only')

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

// ── Data source & execution ───────────────────────────────────────────────────
const isRunning = ref(false)

const effectiveData = computed(() => {
  if (props.node.sourceId) {
    const r = queryResults[props.node.sourceId]
    return (r && !r.error && !r.isRunning) ? r : null
  }
  const r = chartResults[props.node.id]
  return (r && !r.error && !r.isRunning) ? r : null
})

async function runInline() {
  const sql = props.node.sql.trim()
  if (!sql || isRunning.value) return
  isRunning.value = true
  setChartResult(props.node.id, { columns: [], rows: [], error: null, isRunning: true })
  try {
    const result = await query(sql)
    setChartResult(props.node.id, { columns: result.columns, rows: result.rows, error: null, isRunning: false })
  } catch (err) {
    setChartResult(props.node.id, { columns: [], rows: [], error: err instanceof Error ? err.message : String(err), isRunning: false })
  } finally {
    isRunning.value = false
  }
}

async function runLinked() {
  const sid = props.node.sourceId
  if (!sid || isRunning.value) return
  const sourceNode = schemaStore.nodes.find((n) => n.id === sid)
  if (!sourceNode) return
  const { setResult } = useQueryResults()

  if (sourceNode.kind === 'data') {
    isRunning.value = true
    setResult(sid, { columns: [], rows: [], error: null, isRunning: true })
    try {
      const safe = sourceNode.name.replace(/"/g, '""')
      const result = await query(`SELECT * FROM "${safe}"`)
      setResult(sid, { columns: result.columns, columnTypes: result.columnTypes, rows: result.rows, error: null, isRunning: false })
    } catch (err) {
      setResult(sid, { columns: [], rows: [], error: err instanceof Error ? err.message : String(err), isRunning: false })
    } finally {
      isRunning.value = false
    }
    return
  }

  if (sourceNode.kind !== 'query') return
  const sql = sourceNode.sql.trim()
  if (!sql) return
  isRunning.value = true
  setResult(sid, { columns: [], rows: [], error: null, isRunning: true })
  try {
    const result = await query(sql)
    setResult(sid, { columns: result.columns, columnTypes: result.columnTypes, rows: result.rows, error: null, isRunning: false })
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err)
    setResult(sid, { columns: [], rows: [], error: msg, isRunning: false })
  } finally {
    isRunning.value = false
  }
}

// ── Legend toggle ─────────────────────────────────────────────────────────────
const showLegend = ref(true)

// ── Display-type guards ───────────────────────────────────────────────────────
const isPlotType    = computed(() => !['number', 'boolean', 'conditional', 'mermaid', 'table', 'sankey'].includes(props.node.chartType))
const isStatType    = computed(() => props.node.chartType === 'number')
const isBoolType    = computed(() => props.node.chartType === 'boolean')
const isCondType    = computed(() => props.node.chartType === 'conditional')
const isBadgeType   = computed(() => isBoolType.value || isCondType.value)
const isMermaidType = computed(() => props.node.chartType === 'mermaid')
const isTableType   = computed(() => props.node.chartType === 'table')
const isSankeyType  = computed(() => props.node.chartType === 'sankey')

// ── Stat (number) computed ────────────────────────────────────────────────────
const statValue = computed(() => {
  const data = effectiveData.value
  if (!data || !data.rows.length || !props.node.yColumn) return null
  const raw = data.rows[0][props.node.yColumn]
  return raw !== null && raw !== undefined ? Number(raw) : null
})

const formattedStatValue = computed(() => {
  const v = statValue.value
  if (v === null) return '—'
  return Number.isFinite(v) ? v.toLocaleString(undefined, { maximumFractionDigits: 6 }) : String(v)
})

// ── Condition rule evaluator ──────────────────────────────────────────────────
function evalConditionMatch(match: string, raw: unknown): boolean {
  const t = match.trim()
  if (t === '*') return true

  const str = String(raw ?? '')
  const num = Number(raw)
  const hasNum = raw !== null && raw !== undefined && raw !== '' && !isNaN(num)

  // Comparison operators: >, >=, <, <=, !=, =
  const cmp = t.match(/^(>=|<=|!=|>|<|=)\s*(.+)$/)
  if (cmp) {
    const op = cmp[1]
    const rhs = cmp[2].trim()
    const rhsNum = Number(rhs)
    const numericCmp = hasNum && !isNaN(rhsNum)
    switch (op) {
      case '>':  return numericCmp ? num > rhsNum  : str > rhs
      case '>=': return numericCmp ? num >= rhsNum : str >= rhs
      case '<':  return numericCmp ? num < rhsNum  : str < rhs
      case '<=': return numericCmp ? num <= rhsNum : str <= rhs
      case '!=': return numericCmp ? num !== rhsNum : str !== rhs
      case '=':  return numericCmp ? num === rhsNum : str === rhs
    }
  }

  // String ops (case-insensitive)
  const kw = t.match(/^(contains|starts?|ends?)\s+(.+)$/i)
  if (kw) {
    const [, op, operand] = kw
    const hay = str.toLowerCase()
    const needle = operand.trim().toLowerCase()
    if (/^contains$/i.test(op)) return hay.includes(needle)
    if (/^starts?$/i.test(op))  return hay.startsWith(needle)
    if (/^ends?$/i.test(op))    return hay.endsWith(needle)
  }

  // Bare value → exact match
  return str === t
}

// ── Badge (boolean / conditional) computed ────────────────────────────────────
const badgeState = computed((): { label: string; color: string } | null => {
  const data = effectiveData.value
  if (!data || !data.rows.length || !props.node.yColumn) return null
  const raw = data.rows[0][props.node.yColumn]

  if (isBoolType.value) {
    const isTruthy =
      raw !== null && raw !== undefined && raw !== false && raw !== 0 &&
      raw !== '' && String(raw).toLowerCase() !== 'false'
    return {
      label: isTruthy ? (props.node.trueText  ?? 'True')    : (props.node.falseText  ?? 'False'),
      color: isTruthy ? (props.node.trueColor ?? '#10b981') : (props.node.falseColor ?? '#ef4444'),
    }
  }

  if (isCondType.value) {
    const conditions = props.node.conditions ?? []
    const matched = conditions.find((c) => evalConditionMatch(c.match, raw))
    const strVal = String(raw ?? '')
    return matched
      ? { label: matched.label, color: matched.color }
      : { label: strVal || 'Unknown', color: '#6b7280' }
  }

  return null
})

// ── Table type helpers ────────────────────────────────────────────────────────

const visibleColumns = computed(() => {
  const data = effectiveData.value
  if (!data) return []
  const cfgs = props.node.tableColumnConfigs ?? {}
  return data.columns.filter((c) => !cfgs[c]?.hidden)
})

function relativeTime(d: Date): string {
  const diff = Date.now() - d.getTime()
  const abs = Math.abs(diff)
  if (abs < 60_000) return 'just now'
  const sign = diff < 0 ? 'in ' : ''
  const suffix = diff < 0 ? '' : ' ago'
  if (abs < 3_600_000)     return `${sign}${Math.round(abs / 60_000)}m${suffix}`
  if (abs < 86_400_000)    return `${sign}${Math.round(abs / 3_600_000)}h${suffix}`
  if (abs < 2_592_000_000) return `${sign}${Math.round(abs / 86_400_000)}d${suffix}`
  return `${sign}${Math.round(abs / 2_592_000_000)}mo${suffix}`
}

function formatCell(value: unknown, cfg: TableColumnConfig | undefined): string {
  if (value === null || value === undefined) return '—'
  const fmt = cfg?.formatType ?? 'auto'

  if (fmt === 'number' || (fmt === 'auto' && typeof value === 'number')) {
    const n = Number(value)
    if (isNaN(n)) return String(value)
    const dec = cfg?.decimals ?? (fmt === 'number' ? 0 : undefined)
    return n.toLocaleString(undefined, dec !== undefined ? { minimumFractionDigits: dec, maximumFractionDigits: dec } : undefined)
  }
  if (fmt === 'currency') {
    const n = Number(value); if (isNaN(n)) return String(value)
    const dec = cfg?.decimals ?? 2
    const sym = cfg?.currencySymbol ?? '$'
    return sym + n.toLocaleString(undefined, { minimumFractionDigits: dec, maximumFractionDigits: dec })
  }
  if (fmt === 'percent') {
    const n = Number(value); if (isNaN(n)) return String(value)
    const dec = cfg?.decimals ?? 1
    return (n * 100).toLocaleString(undefined, { minimumFractionDigits: dec, maximumFractionDigits: dec }) + '%'
  }
  if (fmt === 'date') {
    const d = value instanceof Date ? value : new Date(String(value))
    if (isNaN(d.getTime())) return String(value)
    switch (cfg?.datePattern ?? 'date') {
      case 'datetime': return d.toLocaleString()
      case 'iso':      return d.toISOString().slice(0, 10)
      case 'us':       return `${String(d.getMonth()+1).padStart(2,'0')}/${String(d.getDate()).padStart(2,'0')}/${d.getFullYear()}`
      case 'eu':       return `${String(d.getDate()).padStart(2,'0')}/${String(d.getMonth()+1).padStart(2,'0')}/${d.getFullYear()}`
      case 'relative': return relativeTime(d)
      default:         return d.toLocaleDateString()
    }
  }
  if (fmt === 'text') return String(value)
  // auto — detect Date objects and ISO-ish strings
  if (value instanceof Date) return value.toLocaleString()
  return String(value)
}

function cellAlign(cfg: TableColumnConfig | undefined): string {
  if (cfg?.align) return cfg.align
  const ft = cfg?.formatType ?? 'auto'
  return ft === 'number' || ft === 'currency' || ft === 'percent' ? 'right' : 'left'
}

// ── Pie / donut SVG ───────────────────────────────────────────────────────────
const PIE_PALETTE = [
  '#4e79a7', '#f28e2b', '#e15759', '#76b7b2', '#59a14f',
  '#edc948', '#b07aa1', '#ff9da7', '#9c755f', '#bab0ac',
]

function polarXY(cx: number, cy: number, r: number, angle: number): [number, number] {
  return [cx + r * Math.sin(angle), cy - r * Math.cos(angle)]
}

function arcPath(cx: number, cy: number, outerR: number, innerR: number, startAngle: number, endAngle: number): string {
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

function buildPieSvg(rows: Record<string, unknown>[], xColumn: string, yColumn: string, width: number, height: number, isDonut: boolean): SVGSVGElement {
  const ns = 'http://www.w3.org/2000/svg'
  const legendW = 110
  const chartW = Math.max(60, width - legendW)
  const margin = 12
  const r = Math.min(chartW, height) / 2 - margin
  const innerR = isDonut ? r * 0.48 : 0
  const cx = chartW / 2, cy = height / 2
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
    const start = angle; angle += span
    return { row, value, span, start, end: angle, color: PIE_PALETTE[i % PIE_PALETTE.length] }
  })
  slices.forEach((s) => {
    const path = document.createElementNS(ns, 'path')
    path.setAttribute('d', arcPath(cx, cy, r, innerR, s.start, s.end))
    path.setAttribute('fill', s.color)
    path.setAttribute('stroke', 'rgba(0,0,0,0.25)')
    path.setAttribute('stroke-width', '1')
    g.appendChild(path)
    if (s.span > 0.35 && r > 40) {
      const midAngle = s.start + s.span / 2
      const labelR = innerR > 0 ? (innerR + r) / 2 : r * 0.65
      const [lx, ly] = polarXY(cx, cy, labelR, midAngle)
      const pct = total > 0 ? Math.round((s.value / total) * 100) : 0
      const text = document.createElementNS(ns, 'text')
      text.setAttribute('x', String(lx)); text.setAttribute('y', String(ly))
      text.setAttribute('text-anchor', 'middle'); text.setAttribute('dominant-baseline', 'middle')
      text.setAttribute('font-size', '9.5'); text.setAttribute('fill', '#e2e8f0')
      text.setAttribute('pointer-events', 'none'); text.textContent = `${pct}%`
      g.appendChild(text)
    }
  })
  if (isDonut && r > 30) {
    const label = document.createElementNS(ns, 'text')
    label.setAttribute('x', String(cx)); label.setAttribute('y', String(cy))
    label.setAttribute('text-anchor', 'middle'); label.setAttribute('dominant-baseline', 'middle')
    label.setAttribute('font-size', '11'); label.setAttribute('fill', '#8b949e')
    label.textContent = total.toLocaleString()
    g.appendChild(label)
  }
  const legendX = chartW + 8, rowH = 15
  const legendStartY = Math.max(4, (height - slices.length * rowH) / 2)
  slices.forEach((s, i) => {
    const gy = legendStartY + i * rowH
    const swatch = document.createElementNS(ns, 'rect')
    swatch.setAttribute('x', String(legendX)); swatch.setAttribute('y', String(gy + 2))
    swatch.setAttribute('width', '8'); swatch.setAttribute('height', '8')
    swatch.setAttribute('rx', '1.5'); swatch.setAttribute('fill', s.color)
    svg.appendChild(swatch)
    const lbl = document.createElementNS(ns, 'text')
    lbl.setAttribute('x', String(legendX + 11)); lbl.setAttribute('y', String(gy + 10))
    lbl.setAttribute('font-size', '9.5'); lbl.setAttribute('fill', '#8b949e')
    lbl.textContent = String(s.row[xColumn] ?? '').slice(0, 12)
    svg.appendChild(lbl)
  })
  return svg
}

// ── Observable Plot rendering ─────────────────────────────────────────────────
const chartContainer = ref<HTMLDivElement | null>(null)

watchEffect(() => {
  if (!chartContainer.value) return
  if (!isPlotType.value) { chartContainer.value.innerHTML = ''; return }
  const data = effectiveData.value
  const { chartType, xColumn, yColumn, colorColumn, labelColumn, color } = props.node
  const needsY = chartType !== 'histogram'
  if (!data || !xColumn || (needsY && !yColumn)) { chartContainer.value.innerHTML = ''; return }
  try {
    const plotW = (props.node.w ?? 340) - 32
    const plotH = props.node.h ?? 180
    if (chartType === 'pie' || chartType === 'donut') {
      const el = buildPieSvg(data.rows, xColumn, yColumn!, plotW, plotH, chartType === 'donut')
      chartContainer.value.replaceChildren(el); return
    }
    const fill = colorColumn ?? color, stroke = colorColumn ?? color
    const marks: Plot.Markish[] = []
    switch (chartType) {
      case 'barY':      marks.push(Plot.barY(data.rows, { x: xColumn, y: yColumn, fill }), Plot.ruleY([0])); break
      case 'barX':      marks.push(Plot.barX(data.rows, { x: xColumn, y: yColumn, fill }), Plot.ruleX([0])); break
      case 'lineY':     marks.push(Plot.lineY(data.rows, { x: xColumn, y: yColumn, stroke }), Plot.ruleY([0])); break
      case 'areaY':     marks.push(Plot.areaY(data.rows, { x: xColumn, y: yColumn, fill, fillOpacity: 0.4, stroke }), Plot.ruleY([0])); break
      case 'cell':      marks.push(Plot.cell(data.rows, { x: xColumn, y: yColumn, fill })); break
      case 'histogram': marks.push(Plot.rectY(data.rows, { ...Plot.binX({ y: 'count' }, { x: xColumn }), fill: fill ?? '#4e79a7' } as Parameters<typeof Plot.rectY>[1]), Plot.ruleY([0])); break
      case 'boxplot':   marks.push(Plot.boxY(data.rows, { x: xColumn, y: yColumn, fill: fill ?? '#4e79a7' })); break
      default:          marks.push(Plot.dot(data.rows, { x: xColumn, y: yColumn, fill })); break
    }
    if (labelColumn && chartType !== 'histogram' && chartType !== 'boxplot') {
      marks.push(Plot.text(data.rows, { x: xColumn, y: yColumn, text: labelColumn, fontSize: 9, fill: 'currentColor', dy: -6 }))
    }
    const el = Plot.plot({
      width: plotW, height: plotH, marginBottom: 36, marginLeft: 42,
      color: colorColumn ? { legend: showLegend.value } : undefined,
      style: { background: 'none', color: '#8b949e', fontSize: '10px', overflow: 'visible' },
      marks,
    })
    chartContainer.value.replaceChildren(el)
  } catch {
    chartContainer.value.innerHTML = `<p class="chart-err">Can't render — check column types</p>`
  }
})

// ── Mermaid rendering ─────────────────────────────────────────────────────────
const mermaidContainer = ref<HTMLDivElement | null>(null)
let _mermaidReady = false

async function renderMermaid(code: string, el: HTMLDivElement) {
  if (!code.trim()) { el.innerHTML = '<p class="chart-placeholder-msg">Enter a diagram in the properties panel</p>'; return }
  try {
    if (!_mermaidReady) {
      const { default: mermaid } = await import('mermaid')
      mermaid.initialize({ startOnLoad: false, theme: 'dark', securityLevel: 'loose' })
      _mermaidReady = true
    }
    const { default: mermaid } = await import('mermaid')
    const safeId = `mm_${props.node.id.replace(/[^a-z0-9]/gi, '_')}`
    const { svg } = await mermaid.render(safeId, code)
    el.innerHTML = svg
  } catch (err) {
    el.innerHTML = `<p class="chart-err">${err instanceof Error ? err.message.split('\n')[0] : String(err)}</p>`
  }
}

watch(
  [isMermaidType, () => props.node.mermaidCode, mermaidContainer],
  async ([isMermaid, code, el]) => {
    if (!isMermaid || !el) return
    await renderMermaid(code ?? '', el)
  },
  { immediate: true },
)

// ── Sankey rendering ──────────────────────────────────────────────────────────
const sankeyContainer = ref<HTMLDivElement | null>(null)

async function renderSankey(el: HTMLDivElement) {
  const data = effectiveData.value
  const { xColumn, yColumn, colorColumn } = props.node
  if (!data || !xColumn || !yColumn) { el.innerHTML = ''; return }

  try {
    const { sankey } = await import('d3-sankey')

    const nodeIndex = new Map<string, number>()
    const rawNodes: Array<{ name: string }> = []
    const rawLinks: Array<{ source: number; target: number; value: number }> = []

    for (const row of data.rows) {
      const src = String(row[xColumn] ?? '')
      const tgt = String(row[yColumn] ?? '')
      const val = colorColumn ? Math.max(0, Number(row[colorColumn]) || 0) : 1
      if (!src || !tgt || src === tgt) continue

      if (!nodeIndex.has(src)) { nodeIndex.set(src, rawNodes.length); rawNodes.push({ name: src }) }
      if (!nodeIndex.has(tgt)) { nodeIndex.set(tgt, rawNodes.length); rawNodes.push({ name: tgt }) }
      rawLinks.push({ source: nodeIndex.get(src)!, target: nodeIndex.get(tgt)!, value: val })
    }

    if (!rawNodes.length || !rawLinks.length) { el.innerHTML = ''; return }

    const w = (props.node.w ?? 340) - 32
    const h = props.node.h ?? 240

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const layout = (sankey as any)()
      .nodeWidth(16)
      .nodePadding(10)
      .extent([[1, 1], [w - 1, h - 1]])

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const { nodes, links } = layout({
      nodes: rawNodes.map((d) => ({ ...d })),
      links: rawLinks.map((d) => ({ ...d })),
    }) as { nodes: any[]; links: any[] }

    const ns = 'http://www.w3.org/2000/svg'
    const svg = document.createElementNS(ns, 'svg')
    svg.setAttribute('width', String(w))
    svg.setAttribute('height', String(h))
    svg.style.overflow = 'visible'
    svg.style.display = 'block'

    // Links (ribbons)
    for (const link of links) {
      const x0 = link.source.x1, x1 = link.target.x0
      const y0 = link.y0, y1 = link.y1
      const lw = Math.max(1, link.width)
      const mx = (x0 + x1) / 2
      const d = `M${x0},${y0 - lw / 2} C${mx},${y0 - lw / 2} ${mx},${y1 - lw / 2} ${x1},${y1 - lw / 2} L${x1},${y1 + lw / 2} C${mx},${y1 + lw / 2} ${mx},${y0 + lw / 2} ${x0},${y0 + lw / 2} Z`
      const path = document.createElementNS(ns, 'path')
      path.setAttribute('d', d)
      path.setAttribute('fill', PIE_PALETTE[link.source.index % PIE_PALETTE.length])
      path.setAttribute('fill-opacity', '0.38')
      svg.appendChild(path)
    }

    // Nodes + labels
    for (const node of nodes) {
      const color = PIE_PALETTE[node.index % PIE_PALETTE.length]
      const rect = document.createElementNS(ns, 'rect')
      rect.setAttribute('x', String(node.x0))
      rect.setAttribute('y', String(node.y0))
      rect.setAttribute('width', String(node.x1 - node.x0))
      rect.setAttribute('height', String(Math.max(1, node.y1 - node.y0)))
      rect.setAttribute('fill', color)
      rect.setAttribute('rx', '2')
      svg.appendChild(rect)

      const isRightSide = node.x0 > w / 2
      const text = document.createElementNS(ns, 'text')
      text.setAttribute('x', String(isRightSide ? node.x0 - 5 : node.x1 + 5))
      text.setAttribute('y', String((node.y0 + node.y1) / 2))
      text.setAttribute('text-anchor', isRightSide ? 'end' : 'start')
      text.setAttribute('dominant-baseline', 'middle')
      text.setAttribute('font-size', '9.5')
      text.setAttribute('fill', '#8b949e')
      text.textContent = String(node.name).slice(0, 16)
      svg.appendChild(text)
    }

    el.replaceChildren(svg)
  } catch (err) {
    el.innerHTML = `<p class="chart-err">${err instanceof Error ? err.message.split('\n')[0] : 'Sankey render error'}</p>`
  }
}

watch(
  [isSankeyType, effectiveData, () => props.node.xColumn, () => props.node.yColumn, () => props.node.colorColumn, () => props.node.w, () => props.node.h, sankeyContainer],
  async ([isSankey, , , , , , , el]) => {
    if (!isSankey || !el) return
    await renderSankey(el as HTMLDivElement)
  },
  { immediate: true },
)

// ── Export ────────────────────────────────────────────────────────────────────
function onExport(fmt: ExportFormat) {
  const d = effectiveData.value
  if (!d || !d.rows.length) return
  exportData(fmt, d.columns, d.rows as Record<string, unknown>[], props.node.name)
}

const showExportMenu = ref(false)

function triggerDownload(href: string, filename: string) {
  const a = document.createElement('a')
  a.href = href
  a.download = filename
  a.style.display = 'none'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

// Resolve CSS variables, fonts, and currentColor so the SVG is self-contained.
function prepareSvgForExport(svgEl: SVGSVGElement): SVGSVGElement {
  const clone = svgEl.cloneNode(true) as SVGSVGElement
  const docStyle = getComputedStyle(document.documentElement)

  const textColor  = docStyle.getPropertyValue('--text-primary').trim()  || '#e6edf3'
  const mutedColor = docStyle.getPropertyValue('--text-muted').trim()    || '#6e7681'
  const borderColor= docStyle.getPropertyValue('--border').trim()        || '#30363d'
  const fontFamily = getComputedStyle(document.body).fontFamily
    || '"JetBrains Mono", ui-monospace, monospace'

  // Set root color so currentColor in child elements resolves correctly.
  clone.style.color = textColor
  clone.style.fontFamily = fontFamily

  // Inline a <style> block that re-declares the CSS vars used by Observable Plot
  // and stamps font-family on all text elements.
  const styleEl = document.createElementNS('http://www.w3.org/2000/svg', 'style')
  styleEl.textContent = [
    ':root {',
    `  --text-primary: ${textColor};`,
    `  --text-muted: ${mutedColor};`,
    `  --border: ${borderColor};`,
    '}',
    `text, tspan { font-family: ${fontFamily}; fill: ${textColor}; }`,
  ].join('\n')
  clone.insertBefore(styleEl, clone.firstChild)

  // Resolve any remaining currentColor attributes on paths/lines/rects.
  for (const el of Array.from(clone.querySelectorAll('[fill="currentColor"],[stroke="currentColor"]'))) {
    const src = svgEl.querySelector(`[data-id="${el.getAttribute('data-id')}"]`) ?? el
    const computed = getComputedStyle(src as Element)
    if (el.getAttribute('fill') === 'currentColor')   el.setAttribute('fill',   computed.color || textColor)
    if (el.getAttribute('stroke') === 'currentColor') el.setAttribute('stroke', computed.color || textColor)
  }

  return clone
}

function getExportSvg(): SVGSVGElement | null {
  return (chartContainer.value?.querySelector('svg') ?? sankeyContainer.value?.querySelector('svg')) as SVGSVGElement | null
}

async function downloadSvg() {
  showExportMenu.value = false
  const svgEl = getExportSvg()
  if (!svgEl) return
  const prepared = prepareSvgForExport(svgEl)
  const svgStr = new XMLSerializer().serializeToString(prepared)
  const name = `${props.node.name}.svg`
  if (IS_DESKTOP) {
    const { SaveFileWithDialog } = await import('../../wailsjs/go/main/App')
    await SaveFileWithDialog(name, svgStr)
  } else {
    triggerDownload('data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svgStr), name)
  }
}

async function downloadPng() {
  showExportMenu.value = false
  const svgEl = getExportSvg()
  if (!svgEl) return
  const { width, height } = svgEl.getBoundingClientRect()
  if (!width || !height) return
  const prepared = prepareSvgForExport(svgEl)
  const svgStr = new XMLSerializer().serializeToString(prepared)
  const svgDataUrl = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svgStr)
  const img = new Image()
  await new Promise<void>((res) => { img.onload = () => res(); img.onerror = () => res(); img.src = svgDataUrl })
  const dpr = window.devicePixelRatio || 1
  const canvas = document.createElement('canvas')
  canvas.width = Math.round(width * dpr)
  canvas.height = Math.round(height * dpr)
  const ctx = canvas.getContext('2d')!
  ctx.scale(dpr, dpr)
  ctx.fillStyle = getComputedStyle(document.documentElement).getPropertyValue('--canvas-bg').trim() || '#0d1117'
  ctx.fillRect(0, 0, width, height)
  ctx.drawImage(img, 0, 0, width, height)
  const name = `${props.node.name}.png`
  const dataUrl = canvas.toDataURL('image/png')
  if (IS_DESKTOP) {
    const { SaveImageFileWithDialog } = await import('../../wailsjs/go/main/App')
    await SaveImageFileWithDialog(name, dataUrl)
  } else {
    triggerDownload(dataUrl, name)
  }
}

// ── Auto-run on mount ─────────────────────────────────────────────────────────
onMounted(() => {
  window.addEventListener('mousedown', () => { showExportMenu.value = false })
  if (props.node.sourceId) {
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
    :data-node-id="node.id"
    :style="{ left: `${node.x}px`, top: `${node.y}px`, width: `${node.w ?? 340}px` }"
    @mousedown="onMouseDown"
    @contextmenu.prevent.stop="openNodeMenu(node.id, $event.clientX, $event.clientY)"
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

      <!-- Properties panel toggle -->
      <button
        class="collapse-btn props-btn"
        title="Edit chart properties"
        @mousedown.stop
        @click.stop="openPanel(node.id)"
      >
        <svg viewBox="0 0 10 10" fill="none">
          <line x1="1" y1="3" x2="9" y2="3" stroke="white" stroke-width="1.2" stroke-linecap="round"/>
          <line x1="1" y1="5" x2="9" y2="5" stroke="white" stroke-width="1.2" stroke-linecap="round"/>
          <line x1="1" y1="7" x2="9" y2="7" stroke="white" stroke-width="1.2" stroke-linecap="round"/>
          <circle cx="3.5" cy="3" r="1.2" fill="white"/>
          <circle cx="6.5" cy="7" r="1.2" fill="white"/>
        </svg>
      </button>

      <button
        class="collapse-btn chartonly-btn"
        :class="{ active: isChartOnly }"
        title="Chart only / show header"
        @mousedown.stop
        @click.stop="toggleChartOnly"
      >
        <svg viewBox="0 0 10 10" fill="none">
          <rect x="1" y="1" width="8" height="8" rx="1" stroke="white" stroke-width="1.2"/>
          <polyline points="2,6.5 3.8,4.5 5.5,5.8 7.5,3" stroke="white" stroke-width="1.1" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>

      <!-- Chart image export -->
      <div v-if="isPlotType || isSankeyType" class="export-img-wrap">
        <button
          class="collapse-btn"
          title="Export chart image"
          @mousedown.stop
          @click.stop="showExportMenu = !showExportMenu"
        >
          <svg viewBox="0 0 10 10" fill="none">
            <path d="M5 1v5.5M3 4.5l2 2 2-2" stroke="white" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M1 7.5v1h8v-1" stroke="white" stroke-width="1.2" stroke-linecap="round"/>
          </svg>
        </button>
        <div v-if="showExportMenu" class="export-img-menu" @mousedown.stop>
          <button class="export-img-item" @click.stop="downloadPng">PNG</button>
          <button class="export-img-item" @click.stop="downloadSvg">SVG</button>
        </div>
      </div>

      <!-- Legend toggle — only when a color column is mapped -->
      <button
        v-if="isPlotType && node.colorColumn"
        class="collapse-btn"
        :class="{ active: showLegend }"
        title="Toggle legend"
        @mousedown.stop
        @click.stop="showLegend = !showLegend"
      >
        <svg viewBox="0 0 10 10" fill="none">
          <rect x="1" y="2" width="3" height="3" rx="0.5" fill="white" opacity="0.9"/>
          <line x1="5.5" y1="3.5" x2="9" y2="3.5" stroke="white" stroke-width="1.1" stroke-linecap="round"/>
          <rect x="1" y="6" width="3" height="3" rx="0.5" fill="white" opacity="0.5"/>
          <line x1="5.5" y1="7.5" x2="9" y2="7.5" stroke="white" stroke-width="1.1" stroke-linecap="round" opacity="0.5"/>
        </svg>
      </button>

      <NodeColorPicker :color="node.color" @pick="schemaStore.setNodeColor(node.id, $event)" />

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

    <!-- Observable Plot area -->
    <div v-if="!isCollapsed && isPlotType" class="chart-area" :style="{ height: `${(node.h ?? 180) + 36}px` }">
      <div ref="chartContainer" class="chart-plot" />
      <div v-if="!effectiveData || !node.xColumn || !node.yColumn" class="chart-placeholder">
        <svg viewBox="0 0 32 32" fill="none">
          <rect x="2" y="2" width="28" height="28" rx="3" stroke="currentColor" stroke-width="1.5"/>
          <polyline points="6,22 11,14 16,18 22,10 26,13" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <span>{{ !effectiveData ? 'Run a query to load data' : 'Set columns in the properties panel' }}</span>
        <button class="open-props-btn" @mousedown.stop @click.stop="openPanel(node.id)">Open properties</button>
      </div>
    </div>

    <!-- Number / stat display -->
    <div v-if="!isCollapsed && isStatType" class="stat-area" :style="{ height: `${(node.h ?? 140) + 24}px` }">
      <template v-if="statValue !== null">
        <div class="stat-value">{{ formattedStatValue }}</div>
        <div v-if="node.chartLabel" class="stat-label">{{ node.chartLabel }}</div>
      </template>
      <div v-else class="chart-placeholder">
        <svg viewBox="0 0 32 32" fill="none">
          <path d="M16 6v20M6 16h20" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
        </svg>
        <span>{{ !effectiveData ? 'Run a query to load data' : 'Set a Value column in the properties panel' }}</span>
        <button class="open-props-btn" @mousedown.stop @click.stop="openPanel(node.id)">Open properties</button>
      </div>
    </div>

    <!-- Boolean / Conditional badge display -->
    <div v-if="!isCollapsed && isBadgeType" class="badge-area" :style="{ height: `${(node.h ?? 120) + 24}px` }">
      <template v-if="badgeState">
        <div class="badge-display" :style="{ background: badgeState.color }">
          <span class="badge-dot" />
          <span class="badge-text">{{ badgeState.label }}</span>
        </div>
      </template>
      <div v-else class="chart-placeholder">
        <svg viewBox="0 0 32 32" fill="none">
          <circle cx="16" cy="16" r="12" stroke="currentColor" stroke-width="1.5"/>
          <path d="M12 16l3 3 5-5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <span>{{ !effectiveData ? 'Run a query to load data' : 'Set a Value column in the properties panel' }}</span>
        <button class="open-props-btn" @mousedown.stop @click.stop="openPanel(node.id)">Open properties</button>
      </div>
    </div>

    <!-- Mermaid diagram display -->
    <div v-if="!isCollapsed && isMermaidType" class="chart-area mermaid-area" :style="{ height: `${(node.h ?? 240) + 24}px` }">
      <div ref="mermaidContainer" class="mermaid-plot" />
    </div>

    <!-- Sankey diagram display -->
    <div v-if="!isCollapsed && isSankeyType" class="chart-area" :style="{ height: `${(node.h ?? 240) + 36}px` }">
      <div ref="sankeyContainer" class="chart-plot" />
      <div v-if="!effectiveData || !node.xColumn || !node.yColumn" class="chart-placeholder">
        <svg viewBox="0 0 32 32" fill="none">
          <path d="M4 8h6l4 8 6-4h8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M4 24h6l4-8 6 4h8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <span>{{ !effectiveData ? 'Run a query to load data' : 'Set Source and Target columns in the properties panel' }}</span>
        <button class="open-props-btn" @mousedown.stop @click.stop="openPanel(node.id)">Open properties</button>
      </div>
    </div>

    <!-- Table display -->
    <div v-if="!isCollapsed && isTableType" class="table-area" :style="{ height: `${(node.h ?? 280) + 24}px` }">
      <template v-if="effectiveData && effectiveData.rows.length">
        <div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th
                  v-for="col in visibleColumns"
                  :key="col"
                  :style="({ textAlign: cellAlign(node.tableColumnConfigs?.[col]) } as any)"
                >
                  {{ node.tableColumnConfigs?.[col]?.label || col }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, ri) in effectiveData.rows" :key="ri">
                <td
                  v-for="col in visibleColumns"
                  :key="col"
                  :style="({ textAlign: cellAlign(node.tableColumnConfigs?.[col]) } as any)"
                  :class="{ 'cell-null': row[col] === null || row[col] === undefined }"
                >
                  {{ formatCell(row[col], node.tableColumnConfigs?.[col]) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
      <div v-else class="chart-placeholder">
        <svg viewBox="0 0 32 32" fill="none">
          <rect x="2" y="4" width="28" height="24" rx="2" stroke="currentColor" stroke-width="1.5"/>
          <line x1="2" y1="11" x2="30" y2="11" stroke="currentColor" stroke-width="1.5"/>
          <line x1="11" y1="4" x2="11" y2="28" stroke="currentColor" stroke-width="1"/>
        </svg>
        <span>Run a query to load data</span>
        <button class="open-props-btn" @mousedown.stop @click.stop="openPanel(node.id)">Open properties</button>
      </div>
    </div>

    <!-- CSV export footer -->
    <div
      v-if="!isCollapsed && !isMermaidType && effectiveData && effectiveData.rows.length"
      class="chart-footer"
      @mousedown.stop
    >
      <span class="chart-footer-count">{{ effectiveData.rows.length.toLocaleString() }} rows</span>
      <button
        v-for="fmt in (['csv','tsv','json','md'] as const)"
        :key="fmt"
        class="export-csv-btn"
        :title="`Download as ${fmt.toUpperCase()}`"
        @click.stop="onExport(fmt)"
      >{{ fmt.toUpperCase() }}</button>
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
.chart-card {
  position: absolute;
  width: 340px;
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
  background: rgba(0,0,0,0.25);
  border: 1px solid rgba(255,255,255,0.4);
  border-radius: 3px;
  color: white;
  font-size: 12px;
  font-weight: 600;
  font-family: inherit;
  letter-spacing: 0.02em;
  padding: 1px 4px;
  outline: none;
}
.card-name-input:focus { border-color: rgba(255,255,255,0.75); background: rgba(0,0,0,0.35); }

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
.collapse-btn:hover { opacity: 1; background: rgba(255,255,255,0.15); }
.chartonly-btn.active { opacity: 1; background: rgba(255,255,255,0.2); }

.export-img-wrap { position: relative; display: flex; }
.export-img-menu {
  position: absolute;
  top: calc(100% + 4px);
  right: 0;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 5px;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0,0,0,0.4);
  z-index: 9999;
  min-width: 56px;
}
.export-img-item {
  display: block;
  width: 100%;
  padding: 5px 10px;
  font-size: 11px;
  font-family: var(--font-mono);
  color: var(--text-primary);
  background: transparent;
  border: none;
  cursor: pointer;
  text-align: left;
}
.export-img-item:hover { background: var(--surface-1); color: var(--accent); }

.props-btn { opacity: 0; }
.chart-card:hover .props-btn { opacity: 0.55; }
.chart-card.selected .props-btn { opacity: 0.7; }
.props-btn:hover { opacity: 1 !important; background: rgba(255,255,255,0.2) !important; }

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
.chart-card:hover .delete-btn, .delete-btn.confirming { opacity: 1; }
.delete-btn:hover { background: rgba(248, 81, 73, 0.35); }
.delete-btn.confirming { background: rgba(248, 81, 73, 0.5); width: auto; padding: 0 6px; }
.confirm-label { font-size: 10px; font-weight: 700; white-space: nowrap; letter-spacing: 0.02em; }

.chart-area {
  position: relative;
  min-height: 100px;
  padding: 28px 6px 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
.chart-plot { width: 100%; }
.chart-plot :deep(svg) { max-width: 100%; }

.mermaid-area { align-items: flex-start; justify-content: flex-start; padding: 10px; }
.mermaid-plot { width: 100%; overflow: auto; }
.mermaid-plot :deep(svg) { max-width: 100%; height: auto; }

.table-area {
  position: relative;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.table-scroll {
  flex: 1;
  overflow: auto;
  scrollbar-width: thin;
  scrollbar-color: var(--border) transparent;
}
.table-scroll::-webkit-scrollbar { width: 5px; height: 5px; }
.table-scroll::-webkit-scrollbar-track { background: transparent; }
.table-scroll::-webkit-scrollbar-thumb { background: var(--border); border-radius: 3px; }

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 11px;
  font-family: var(--font-mono);
}
.data-table thead th {
  position: sticky;
  top: 0;
  background: var(--surface-2);
  color: var(--text-muted);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  padding: 5px 10px;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
  z-index: 1;
}
.data-table tbody tr { border-bottom: 1px solid rgba(255,255,255,0.04); }
.data-table tbody tr:last-child { border-bottom: none; }
.data-table tbody tr:hover { background: rgba(255,255,255,0.03); }
.data-table tbody td {
  padding: 4px 10px;
  color: var(--text-primary);
  white-space: nowrap;
  max-width: 240px;
  overflow: hidden;
  text-overflow: ellipsis;
}
.cell-null { color: var(--text-muted) !important; font-style: italic; }

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
.chart-placeholder svg { width: 32px; height: 32px; opacity: 0.3; }
.chart-placeholder-msg {
  font-size: 11.5px;
  color: var(--text-muted);
  text-align: center;
  padding: 24px 16px;
  font-style: italic;
}

.open-props-btn {
  margin-top: 4px;
  padding: 4px 10px;
  font-size: 10.5px;
  font-family: inherit;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  cursor: pointer;
  transition: color 0.12s, background 0.12s;
}
.open-props-btn:hover { color: var(--text-primary); background: var(--surface-3, var(--surface-2)); }

.chart-err { font-size: 11px; color: var(--error); padding: 8px; text-align: center; }

.stat-area {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 20px 16px;
  gap: 6px;
  overflow: hidden;
}
.stat-value {
  font-size: 48px;
  font-weight: 700;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.03em;
  line-height: 1;
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}
.stat-label { font-size: 11px; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.09em; text-align: center; }

.badge-area { display: flex; align-items: center; justify-content: center; padding: 16px; overflow: hidden; }
.badge-display {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 28px;
  border-radius: 10px;
  width: 100%;
  justify-content: center;
  transition: background 0.3s;
}
.badge-dot {
  width: 10px; height: 10px;
  border-radius: 50%;
  background: rgba(255,255,255,0.7);
  flex-shrink: 0;
  animation: badge-pulse 2.5s ease-in-out infinite;
}
.badge-text { font-size: 18px; font-weight: 700; color: white; letter-spacing: 0.03em; text-shadow: 0 1px 3px rgba(0,0,0,0.25); }
@keyframes badge-pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.4; } }

.rh-e, .rh-s, .rh-se { position: absolute; opacity: 0; transition: opacity 0.15s; }
.rh-e { right: 0; top: 8px; bottom: 20px; width: 6px; cursor: ew-resize; border-radius: 0 4px 4px 0; }
.rh-s { bottom: 0; left: 8px; right: 20px; height: 6px; cursor: ns-resize; border-radius: 0 0 4px 4px; }
.rh-se { right: 0; bottom: 0; width: 18px; height: 18px; cursor: se-resize; display: flex; align-items: center; justify-content: center; color: var(--text-muted); border-radius: 0 0 7px 0; }
.rh-se svg { width: 8px; height: 8px; }
.rh-e:hover, .rh-s:hover { background: rgba(88, 166, 255, 0.2); }
.chart-card:hover .rh-e, .chart-card:hover .rh-s, .chart-card:hover .rh-se,
.chart-card.selected .rh-e, .chart-card.selected .rh-s, .chart-card.selected .rh-se { opacity: 1; }

.chart-footer { display: flex; align-items: center; gap: 6px; padding: 4px 8px 5px 10px; border-top: 1px solid var(--border); }
.chart-footer-count { flex: 1; font-size: 10px; font-family: var(--font-mono); color: var(--text-muted); }
.export-csv-btn {
  display: flex; align-items: center; gap: 3px;
  padding: 2px 6px; font-size: 10px; font-weight: 600; font-family: var(--font-mono);
  background: var(--surface-2); color: var(--text-muted); border: 1px solid var(--border);
  border-radius: 3px; cursor: pointer; flex-shrink: 0; transition: background 0.1s, color 0.1s;
}
.export-csv-btn svg { width: 9px; height: 9px; }
.export-csv-btn:hover { background: var(--accent); color: #0d1117; border-color: var(--accent); }
</style>
