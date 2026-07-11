<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import SqlEditor from './SqlEditor.vue'
import { useSchemaCompletions } from '../composables/useSchemaCompletions'
import { useSchemaStore } from '../stores/schema'
import type { TableColumnConfig, ColumnFormatType, ColumnAlign, DatePattern } from '../stores/schema'
import { useChartPanel } from '../composables/useChartPanel'
import { useChartResults } from '../composables/useChartResults'
import { useQueryResults } from '../composables/useQueryResults'
import { useDuckDB } from '../composables/useDuckDB'

const schemaStore = useSchemaStore()
const { sqlSchema } = useSchemaCompletions()
const { panelChartId, closePanel } = useChartPanel()
const { chartResults, setChartResult } = useChartResults()
const { results: queryResults, setResult } = useQueryResults()
const { query } = useDuckDB()

// ── Current chart node ────────────────────────────────────────────────────────
const node = computed(() => {
  if (!panelChartId.value) return null
  const n = schemaStore.nodes.find((n) => n.id === panelChartId.value)
  return n?.kind === 'chart' ? n : null
})

// Close if chart node is deleted
watch(node, (n) => { if (!n) closePanel() })

// ── Chart type info ───────────────────────────────────────────────────────────
const CHART_TYPES = [
  { value: 'barY',        label: 'Bar Y',     icon: '▮' },
  { value: 'barX',        label: 'Bar X',     icon: '▬' },
  { value: 'lineY',       label: 'Line',      icon: '╱' },
  { value: 'areaY',       label: 'Area',      icon: '◭' },
  { value: 'dot',         label: 'Scatter',   icon: '●' },
  { value: 'cell',        label: 'Cell',      icon: '▦' },
  { value: 'pie',         label: 'Pie',       icon: '◔' },
  { value: 'donut',       label: 'Donut',     icon: '◎' },
  { value: 'histogram',   label: 'Histogram', icon: '▆' },
  { value: 'boxplot',     label: 'Box Plot',  icon: '⊟' },
  { value: 'sankey',      label: 'Sankey',    icon: '↔' },
  { value: 'number',      label: 'Number',    icon: '#' },
  { value: 'boolean',     label: 'Badge',     icon: '◉' },
  { value: 'conditional', label: 'Status',    icon: '◈' },
  { value: 'mermaid',     label: 'Mermaid',   icon: '⬡' },
  { value: 'table',       label: 'Table',     icon: '⊞' },
] as const

type ChartTypeValue = typeof CHART_TYPES[number]['value']

const CHART_HELP: Record<string, { when: string; columns: string; tip?: string }> = {
  barY:        { when: 'Compare values across categories or time', columns: 'X: category or time · Y: numeric value', tip: 'Sort X with ORDER BY for cleaner bars' },
  barX:        { when: 'Horizontal bars — great for long category names', columns: 'X: numeric value · Y: category' },
  lineY:       { when: 'Show trends over continuous X (time, index)', columns: 'X: time or sequential · Y: numeric value', tip: 'Add a Color column to plot multiple series' },
  areaY:       { when: 'Like Line but filled — good for cumulative or volume data', columns: 'X: time or sequential · Y: numeric value' },
  dot:         { when: 'Correlations between two numeric columns', columns: 'X: numeric · Y: numeric', tip: 'Color groups points; Label annotates them' },
  cell:        { when: 'Color-encoded grid across two categorical axes (heatmap)', columns: 'X: category · Y: category · Color: numeric intensity' },
  pie:         { when: 'Part-to-whole for a small number of categories', columns: 'Label: category · Value: numeric', tip: 'Best with fewer than 8 slices' },
  donut:       { when: 'Like Pie but with a center hole showing the total', columns: 'Label: category · Value: numeric' },
  histogram:   { when: 'Distribution of a single numeric column — bins are automatic', columns: 'X: numeric column to bin', tip: 'Add a Color column to overlay multiple groups' },
  boxplot:     { when: 'Median, IQR, and outliers — compare distributions across groups', columns: 'X: category (group by) · Y: numeric values', tip: 'Leave X empty for a single overall box' },
  sankey:      { when: 'Flow volume between two sets of categories (funnels, networks)', columns: 'Source: origin category · Target: destination · Value: numeric flow weight', tip: 'Each row is one source → target link with its weight' },
  number:      { when: 'Display a single key metric as a large number', columns: 'Value: numeric column — uses the first row only', tip: 'Add a Label for a caption below the number' },
  boolean:     { when: 'Green/red status badge driven by a boolean or truthy value', columns: 'Value: boolean-like column — uses the first row only' },
  conditional: { when: 'Color-coded badge driven by custom match rules', columns: 'Value: any column · Rules: patterns evaluated top-to-bottom' },
  mermaid:     { when: 'Flowcharts, sequence diagrams, ER diagrams via Mermaid.js', columns: 'No data columns needed — write diagram code directly' },
  table:       { when: 'Formatted data grid with per-column rename, formatting, and alignment', columns: 'All result columns shown by default — configure each individually' },
}

const isPlotType     = computed(() => node.value && !['number', 'boolean', 'conditional', 'mermaid', 'table', 'sankey'].includes(node.value.chartType))
const isPieType      = computed(() => node.value?.chartType === 'pie' || node.value?.chartType === 'donut')
const isHistogramType= computed(() => node.value?.chartType === 'histogram')
const isSankeyType   = computed(() => node.value?.chartType === 'sankey')
const isStatType     = computed(() => node.value?.chartType === 'number')
const isBoolType     = computed(() => node.value?.chartType === 'boolean')
const isCondType     = computed(() => node.value?.chartType === 'conditional')
const isMermaidType  = computed(() => node.value?.chartType === 'mermaid')
const isTableType    = computed(() => node.value?.chartType === 'table')

const chartHelp = computed(() => node.value ? CHART_HELP[node.value.chartType] ?? null : null)

// ── Available source nodes (query + data) for source selector ─────────────────
const queryNodes = computed(() => schemaStore.nodes.filter((n) => n.kind === 'query' || n.kind === 'data'))

// ── Effective data (for column list) ─────────────────────────────────────────
const effectiveData = computed(() => {
  if (!node.value) return null
  if (node.value.sourceId) {
    const r = queryResults[node.value.sourceId]
    return (r && !r.error && !r.isRunning) ? r : null
  }
  const r = chartResults[node.value.id]
  return (r && !r.error && !r.isRunning) ? r : null
})

const availableColumns = computed(() => effectiveData.value?.columns ?? [])

// ── Inline SQL editor local state ─────────────────────────────────────────────
const localSql = ref('')
watch(node, (n) => { if (n) localSql.value = n.sql }, { immediate: true })

function onSqlChange(value: string) {
  localSql.value = value
  if (node.value) schemaStore.updateChartConfig(node.value.id, { sql: localSql.value })
}

// ── Run inline SQL ────────────────────────────────────────────────────────────
const isRunning = ref(false)
const runError = ref<string | null>(null)

async function runInline() {
  if (!node.value) return
  const sql = localSql.value.trim()
  if (!sql || isRunning.value) return
  isRunning.value = true
  runError.value = null
  setChartResult(node.value.id, { columns: [], rows: [], error: null, isRunning: true })
  try {
    const result = await query(sql)
    setChartResult(node.value.id, { columns: result.columns, rows: result.rows, error: null, isRunning: false })
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err)
    runError.value = msg
    setChartResult(node.value.id, { columns: [], rows: [], error: msg, isRunning: false })
  } finally {
    isRunning.value = false
  }
}

// ── Refresh linked node (query or data) ──────────────────────────────────────
async function runLinked() {
  if (!node.value?.sourceId || isRunning.value) return
  const sourceNode = schemaStore.nodes.find((n) => n.id === node.value!.sourceId)
  if (!sourceNode) return
  isRunning.value = true
  runError.value = null

  if (sourceNode.kind === 'data') {
    const safe = sourceNode.name.replace(/"/g, '""')
    setResult(sourceNode.id, { columns: [], rows: [], error: null, isRunning: true })
    try {
      const result = await query(`SELECT * FROM "${safe}"`)
      setResult(sourceNode.id, { columns: result.columns, rows: result.rows, error: null, isRunning: false })
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err)
      runError.value = msg
      setResult(sourceNode.id, { columns: [], rows: [], error: msg, isRunning: false })
    } finally {
      isRunning.value = false
    }
    return
  }

  if (sourceNode.kind !== 'query') { isRunning.value = false; return }
  const sql = sourceNode.sql.trim()
  if (!sql) { isRunning.value = false; return }
  setResult(sourceNode.id, { columns: [], rows: [], error: null, isRunning: true })
  try {
    const result = await query(sql)
    setResult(sourceNode.id, { columns: result.columns, rows: result.rows, error: null, isRunning: false })
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err)
    runError.value = msg
    setResult(sourceNode.id, { columns: [], rows: [], error: msg, isRunning: false })
  } finally {
    isRunning.value = false
  }
}

// ── Config helpers ────────────────────────────────────────────────────────────
function setChartType(type: ChartTypeValue) {
  if (!node.value) return
  schemaStore.updateChartConfig(node.value.id, { chartType: type })
}

function setSource(mode: 'inline' | string) {
  if (!node.value) return
  schemaStore.updateChartConfig(node.value.id, { sourceId: mode === 'inline' ? null : mode })
}

function setColumn(field: 'xColumn' | 'yColumn' | 'colorColumn' | 'labelColumn', val: string) {
  if (!node.value) return
  schemaStore.updateChartConfig(node.value.id, { [field]: val || undefined })
}

function setTextField(field: 'chartLabel' | 'trueText' | 'falseText', val: string) {
  if (!node.value) return
  schemaStore.updateChartConfig(node.value.id, { [field]: val || undefined })
}

function setColor(field: 'trueColor' | 'falseColor', val: string) {
  if (!node.value) return
  schemaStore.updateChartConfig(node.value.id, { [field]: val })
}

// ── Mermaid code local state ──────────────────────────────────────────────────
const localMermaid = ref('')
watch(node, (n) => { if (n) localMermaid.value = n.mermaidCode ?? '' }, { immediate: true })

function onMermaidInput(e: Event) {
  localMermaid.value = (e.target as HTMLTextAreaElement).value
  if (node.value) schemaStore.updateChartConfig(node.value.id, { mermaidCode: localMermaid.value })
}

// ── Condition rules ───────────────────────────────────────────────────────────
function addCondition() {
  if (!node.value) return
  const conds = [...(node.value.conditions ?? []), { match: '', label: '', color: '#6b7280' }]
  schemaStore.updateChartConfig(node.value.id, { conditions: conds })
}

function updateCondition(i: number, field: 'match' | 'label' | 'color', val: string) {
  if (!node.value) return
  const conds = [...(node.value.conditions ?? [])]
  conds[i] = { ...conds[i], [field]: val }
  schemaStore.updateChartConfig(node.value.id, { conditions: conds })
}

function removeCondition(i: number) {
  if (!node.value) return
  const conds = [...(node.value.conditions ?? [])]
  conds.splice(i, 1)
  schemaStore.updateChartConfig(node.value.id, { conditions: conds })
}

// ── Table column config ───────────────────────────────────────────────────────
function getColConfig(col: string): TableColumnConfig {
  return node.value?.tableColumnConfigs?.[col] ?? {}
}

function setColConfig(col: string, patch: Partial<TableColumnConfig>) {
  if (!node.value) return
  const current = { ...(node.value.tableColumnConfigs ?? {}) }
  current[col] = { ...current[col], ...patch }
  schemaStore.updateChartConfig(node.value.id, { tableColumnConfigs: current })
}

function toggleColHidden(col: string) {
  setColConfig(col, { hidden: !getColConfig(col).hidden })
}

const FORMAT_TYPES: { value: ColumnFormatType; label: string }[] = [
  { value: 'auto',     label: 'Auto' },
  { value: 'number',   label: 'Number' },
  { value: 'currency', label: 'Currency' },
  { value: 'percent',  label: 'Percent' },
  { value: 'date',     label: 'Date' },
  { value: 'text',     label: 'Text' },
]

const DATE_PATTERNS: { value: DatePattern; label: string }[] = [
  { value: 'date',     label: 'Local date' },
  { value: 'datetime', label: 'Local datetime' },
  { value: 'iso',      label: 'ISO  (YYYY-MM-DD)' },
  { value: 'us',       label: 'US  (MM/DD/YYYY)' },
  { value: 'eu',       label: 'EU  (DD/MM/YYYY)' },
  { value: 'relative', label: 'Relative  (2d ago)' },
]

// ── Escape key ────────────────────────────────────────────────────────────────
function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') closePanel()
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))


</script>

<template>
  <Transition name="panel-slide">
    <div v-if="node" class="props-panel" @mousedown.stop>
      <!-- Header -->
      <div class="pp-header">
        <svg class="pp-header-icon" viewBox="0 0 12 12" fill="none">
          <line x1="1" y1="3" x2="11" y2="3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
          <line x1="1" y1="6" x2="11" y2="6" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
          <line x1="1" y1="9" x2="11" y2="9" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
          <circle cx="4" cy="3" r="1.5" fill="var(--surface-2)" stroke="currentColor" stroke-width="1.1"/>
          <circle cx="8" cy="9" r="1.5" fill="var(--surface-2)" stroke="currentColor" stroke-width="1.1"/>
        </svg>
        <span class="pp-header-title">{{ node.name }}</span>
        <button class="pp-close" title="Close (Esc)" @click="closePanel">
          <svg viewBox="0 0 10 10" fill="none">
            <path d="M2 2l6 6M8 2l-6 6" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <div class="pp-body">
        <!-- ── Chart type ───────────────────────────────────────────────────── -->
        <section class="pp-section">
          <div class="pp-section-label">Chart type</div>
          <div class="chart-type-grid">
            <button
              v-for="ct in CHART_TYPES"
              :key="ct.value"
              class="ct-pill"
              :class="{ active: node.chartType === ct.value }"
              :title="ct.label"
              @click="setChartType(ct.value)"
            >
              <span class="ct-icon">{{ ct.icon }}</span>
              <span class="ct-label">{{ ct.label }}</span>
            </button>
          </div>

          <!-- Help text for selected chart type -->
          <div v-if="chartHelp" class="chart-help">
            <div class="chart-help-when">{{ chartHelp.when }}</div>
            <div class="chart-help-cols">{{ chartHelp.columns }}</div>
            <div v-if="chartHelp.tip" class="chart-help-tip">💡 {{ chartHelp.tip }}</div>
          </div>
        </section>

        <!-- ── Mermaid code editor ─────────────────────────────────────────── -->
        <section v-if="isMermaidType" class="pp-section">
          <div class="pp-section-label">Diagram code</div>
          <textarea
            class="mermaid-editor"
            placeholder="graph TD&#10;  A[Start] --> B[Step]&#10;  B --> C[End]"
            :value="localMermaid"
            spellcheck="false"
            @input="onMermaidInput"
          />
          <div class="pp-hint">Mermaid syntax · changes render live</div>
        </section>

        <!-- ── Data source ─────────────────────────────────────────────────── -->
        <section v-if="!isMermaidType" class="pp-section">
          <div class="pp-section-label">Data source</div>

          <!-- Source tabs -->
          <div class="source-tabs">
            <button
              class="source-tab"
              :class="{ active: !node.sourceId }"
              @click="setSource('inline')"
            >Inline SQL</button>
            <button
              class="source-tab"
              :class="{ active: !!node.sourceId }"
              @click="queryNodes[0] && setSource(queryNodes[0].id)"
              :disabled="queryNodes.length === 0"
              :title="queryNodes.length === 0 ? 'Add a Query or Data node first' : undefined"
            >Linked node</button>
          </div>

          <!-- Inline SQL mode -->
          <template v-if="!node.sourceId">
            <SqlEditor
              :model-value="localSql"
              :height="120"
              :schema="sqlSchema"
              @update:model-value="onSqlChange"
              @run="runInline"
            />
            <button
              class="run-btn"
              :disabled="!localSql.trim() || isRunning"
              @click="runInline"
            >
              <svg viewBox="0 0 10 10" fill="none">
                <path d="M2 1.5l7 3.5-7 3.5V1.5z" fill="currentColor"/>
              </svg>
              {{ isRunning ? 'Running…' : 'Run  ⌘↵' }}
            </button>
          </template>

          <!-- Linked query mode -->
          <template v-else>
            <select
              class="source-select"
              :value="node.sourceId"
              @change="setSource(($event.target as HTMLSelectElement).value)"
            >
              <option v-for="qn in queryNodes" :key="qn.id" :value="qn.id">{{ qn.name }}{{ qn.kind === 'data' ? ' (table)' : '' }}</option>
            </select>
            <button
              class="run-btn"
              :disabled="isRunning"
              @click="runLinked"
            >
              <svg viewBox="0 0 10 10" fill="none">
                <path d="M1 5a4 4 0 1 1 4 4" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" fill="none"/>
                <path d="M1 3v2h2" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round" fill="none"/>
              </svg>
              {{ isRunning ? 'Running…' : 'Refresh' }}
            </button>
          </template>

          <div v-if="runError" class="run-error">{{ runError }}</div>

          <!-- Row count indicator -->
          <div v-if="effectiveData" class="run-ok">
            {{ effectiveData.rows.length.toLocaleString() }} rows · {{ effectiveData.columns.length }} cols
          </div>
        </section>

        <!-- ── Plot columns (barY/barX/line/area/dot/cell/boxplot) ─────────── -->
        <section v-if="isPlotType && !isPieType && !isHistogramType" class="pp-section">
          <div class="pp-section-label">Columns</div>
          <div class="col-row">
            <label>X axis</label>
            <select :value="node.xColumn" @change="setColumn('xColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div class="col-row">
            <label>Y axis</label>
            <select :value="node.yColumn" @change="setColumn('yColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div class="col-row">
            <label>Color</label>
            <select :value="node.colorColumn ?? ''" @change="setColumn('colorColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div class="col-row">
            <label>Label</label>
            <select :value="node.labelColumn ?? ''" @change="setColumn('labelColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div v-if="!availableColumns.length" class="pp-hint">Run a query to see columns</div>
        </section>

        <!-- ── Histogram columns ──────────────────────────────────────────── -->
        <section v-if="isHistogramType" class="pp-section">
          <div class="pp-section-label">Columns</div>
          <div class="col-row">
            <label>X (bins)</label>
            <select :value="node.xColumn" @change="setColumn('xColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div class="col-row">
            <label>Color</label>
            <select :value="node.colorColumn ?? ''" @change="setColumn('colorColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div v-if="!availableColumns.length" class="pp-hint">Run a query to see columns</div>
        </section>

        <!-- ── Sankey columns ─────────────────────────────────────────────── -->
        <section v-if="isSankeyType" class="pp-section">
          <div class="pp-section-label">Columns</div>
          <div class="col-row">
            <label>Source</label>
            <select :value="node.xColumn" @change="setColumn('xColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div class="col-row">
            <label>Target</label>
            <select :value="node.yColumn" @change="setColumn('yColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div class="col-row">
            <label>Value</label>
            <select :value="node.colorColumn ?? ''" @change="setColumn('colorColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div v-if="!availableColumns.length" class="pp-hint">Run a query to see columns</div>
        </section>

        <!-- ── Pie / donut columns ──────────────────────────────────────────── -->
        <section v-if="isPieType" class="pp-section">
          <div class="pp-section-label">Columns</div>
          <div class="col-row">
            <label>Label</label>
            <select :value="node.xColumn" @change="setColumn('xColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div class="col-row">
            <label>Value</label>
            <select :value="node.yColumn" @change="setColumn('yColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div v-if="!availableColumns.length" class="pp-hint">Run a query to see columns</div>
        </section>

        <!-- ── Number stat ─────────────────────────────────────────────────── -->
        <section v-if="isStatType" class="pp-section">
          <div class="pp-section-label">Number display</div>
          <div class="col-row">
            <label>Value</label>
            <select :value="node.yColumn" @change="setColumn('yColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div class="col-row">
            <label>Label</label>
            <input
              type="text"
              class="text-input"
              placeholder="optional caption"
              :value="node.chartLabel ?? ''"
              @input="setTextField('chartLabel', ($event.target as HTMLInputElement).value)"
            />
          </div>
          <div v-if="!availableColumns.length" class="pp-hint">Run a query to see columns</div>
        </section>

        <!-- ── Boolean badge ───────────────────────────────────────────────── -->
        <section v-if="isBoolType" class="pp-section">
          <div class="pp-section-label">Badge display</div>
          <div class="col-row">
            <label>Value</label>
            <select :value="node.yColumn" @change="setColumn('yColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div class="bool-row">
            <div class="bool-cell">
              <span class="bool-label-text">True text</span>
              <input type="text" class="text-input" placeholder="True" :value="node.trueText ?? ''" @input="setTextField('trueText', ($event.target as HTMLInputElement).value)" />
            </div>
            <div class="bool-cell">
              <span class="bool-label-text">True color</span>
              <input type="color" class="color-input" :value="node.trueColor ?? '#10b981'" @input="setColor('trueColor', ($event.target as HTMLInputElement).value)" />
            </div>
          </div>
          <div class="bool-row">
            <div class="bool-cell">
              <span class="bool-label-text">False text</span>
              <input type="text" class="text-input" placeholder="False" :value="node.falseText ?? ''" @input="setTextField('falseText', ($event.target as HTMLInputElement).value)" />
            </div>
            <div class="bool-cell">
              <span class="bool-label-text">False color</span>
              <input type="color" class="color-input" :value="node.falseColor ?? '#ef4444'" @input="setColor('falseColor', ($event.target as HTMLInputElement).value)" />
            </div>
          </div>
          <div v-if="!availableColumns.length" class="pp-hint">Run a query to see columns</div>
        </section>

        <!-- ── Conditional / status ────────────────────────────────────────── -->
        <section v-if="isCondType" class="pp-section">
          <div class="pp-section-label">Status rules</div>
          <div class="col-row">
            <label>Value</label>
            <select :value="node.yColumn" @change="setColumn('yColumn', ($event.target as HTMLSelectElement).value)">
              <option value="">— none —</option>
              <option v-for="c in availableColumns" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div v-if="!availableColumns.length" class="pp-hint">Run a query to see columns</div>

          <div class="cond-rules">
            <div
              v-for="(cond, i) in (node.conditions ?? [])"
              :key="i"
              class="cond-rule"
            >
              <input
                type="text"
                class="text-input cond-match"
                placeholder="> 100, contains err, *"
                :value="cond.match"
                @input="updateCondition(i, 'match', ($event.target as HTMLInputElement).value)"
              />
              <input
                type="text"
                class="text-input cond-label"
                placeholder="label"
                :value="cond.label"
                @input="updateCondition(i, 'label', ($event.target as HTMLInputElement).value)"
              />
              <input
                type="color"
                class="color-input"
                :value="cond.color"
                @input="updateCondition(i, 'color', ($event.target as HTMLInputElement).value)"
              />
              <button class="cond-remove" title="Remove rule" @click="removeCondition(i)">
                <svg viewBox="0 0 10 10" fill="none">
                  <path d="M2 2l6 6M8 2l-6 6" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                </svg>
              </button>
            </div>
          </div>
          <button class="add-cond-btn" @click="addCondition">+ Add rule</button>

          <div class="cond-syntax">
            <div class="cond-syntax-title">Match syntax</div>
            <table class="cond-syntax-table">
              <tr><td class="syn-ex">active</td><td>exact match</td></tr>
              <tr><td class="syn-ex">&gt; 100</td><td>greater than</td></tr>
              <tr><td class="syn-ex">&gt;= 100</td><td>greater than or equal</td></tr>
              <tr><td class="syn-ex">&lt; 0</td><td>less than</td></tr>
              <tr><td class="syn-ex">!= pending</td><td>not equal</td></tr>
              <tr><td class="syn-ex">contains err</td><td>string contains</td></tr>
              <tr><td class="syn-ex">starts warn</td><td>string starts with</td></tr>
              <tr><td class="syn-ex">ends _ok</td><td>string ends with</td></tr>
              <tr><td class="syn-ex">*</td><td>catch-all (put last)</td></tr>
            </table>
            <div class="cond-syntax-note">Rules match top-to-bottom · numeric ops auto-detect numbers</div>
          </div>
        </section>

        <!-- ── Table column config ─────────────────────────────────────────── -->
        <section v-if="isTableType" class="pp-section">
          <div class="pp-section-label">Columns</div>
          <div v-if="!availableColumns.length" class="pp-hint">Run a query to see columns</div>
          <div v-else class="tcol-list">
            <div v-for="col in availableColumns" :key="col" class="tcol-row">
              <!-- visibility toggle -->
              <button
                class="tcol-eye"
                :class="{ hidden: getColConfig(col).hidden }"
                :title="getColConfig(col).hidden ? 'Show column' : 'Hide column'"
                @click="toggleColHidden(col)"
              >
                <svg v-if="!getColConfig(col).hidden" viewBox="0 0 12 12" fill="none">
                  <ellipse cx="6" cy="6" rx="5" ry="3.5" stroke="currentColor" stroke-width="1.2"/>
                  <circle cx="6" cy="6" r="1.5" fill="currentColor"/>
                </svg>
                <svg v-else viewBox="0 0 12 12" fill="none">
                  <path d="M2 2l8 8M4.5 3.5C5 3.2 5.5 3 6 3c2.5 0 4.5 2 4.5 3 0 .5-.3 1-.7 1.4M7.5 8.5C7 8.8 6.5 9 6 9c-2.5 0-4.5-2-4.5-3 0-.5.3-1 .7-1.4" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
                </svg>
              </button>

              <div class="tcol-body" :class="{ 'tcol-hidden': getColConfig(col).hidden }">
                <!-- column name / label override -->
                <div class="tcol-name-row">
                  <span class="tcol-key">{{ col }}</span>
                  <input
                    type="text"
                    class="tcol-label-input text-input"
                    placeholder="rename…"
                    :value="getColConfig(col).label ?? ''"
                    @input="setColConfig(col, { label: ($event.target as HTMLInputElement).value || undefined })"
                  />
                </div>

                <!-- format + alignment row -->
                <div class="tcol-fmt-row">
                  <select
                    class="tcol-fmt-select"
                    :value="getColConfig(col).formatType ?? 'auto'"
                    @change="setColConfig(col, { formatType: ($event.target as HTMLSelectElement).value as ColumnFormatType })"
                  >
                    <option v-for="ft in FORMAT_TYPES" :key="ft.value" :value="ft.value">{{ ft.label }}</option>
                  </select>

                  <!-- alignment chips -->
                  <div class="align-chips">
                    <button
                      v-for="a in ['left','center','right'] as ColumnAlign[]"
                      :key="a"
                      class="align-chip"
                      :class="{ active: (getColConfig(col).align ?? 'left') === a }"
                      :title="a"
                      @click="setColConfig(col, { align: a })"
                    >
                      <svg viewBox="0 0 10 10" fill="none">
                        <template v-if="a === 'left'">
                          <line x1="1" y1="3" x2="9" y2="3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
                          <line x1="1" y1="5.5" x2="6" y2="5.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
                          <line x1="1" y1="8" x2="7.5" y2="8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
                        </template>
                        <template v-else-if="a === 'center'">
                          <line x1="1" y1="3" x2="9" y2="3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
                          <line x1="2.5" y1="5.5" x2="7.5" y2="5.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
                          <line x1="1.5" y1="8" x2="8.5" y2="8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
                        </template>
                        <template v-else>
                          <line x1="1" y1="3" x2="9" y2="3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
                          <line x1="4" y1="5.5" x2="9" y2="5.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
                          <line x1="2.5" y1="8" x2="9" y2="8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
                        </template>
                      </svg>
                    </button>
                  </div>
                </div>

                <!-- conditional sub-options -->
                <div
                  v-if="['number','currency','percent'].includes(getColConfig(col).formatType ?? 'auto')"
                  class="tcol-sub"
                >
                  <label class="tcol-sub-label">Decimals</label>
                  <input
                    type="number" min="0" max="6"
                    class="tcol-sub-input text-input"
                    :value="getColConfig(col).decimals ?? (getColConfig(col).formatType === 'currency' ? 2 : 0)"
                    @input="setColConfig(col, { decimals: Math.max(0, Math.min(6, +($event.target as HTMLInputElement).value)) })"
                  />
                  <template v-if="getColConfig(col).formatType === 'currency'">
                    <label class="tcol-sub-label">Symbol</label>
                    <input
                      type="text" maxlength="4"
                      class="tcol-sub-input text-input"
                      placeholder="$"
                      :value="getColConfig(col).currencySymbol ?? '$'"
                      @input="setColConfig(col, { currencySymbol: ($event.target as HTMLInputElement).value || '$' })"
                    />
                  </template>
                </div>
                <div v-if="getColConfig(col).formatType === 'date'" class="tcol-sub">
                  <label class="tcol-sub-label">Pattern</label>
                  <select
                    class="tcol-fmt-select"
                    :value="getColConfig(col).datePattern ?? 'date'"
                    @change="setColConfig(col, { datePattern: ($event.target as HTMLSelectElement).value as DatePattern })"
                  >
                    <option v-for="dp in DATE_PATTERNS" :key="dp.value" :value="dp.value">{{ dp.label }}</option>
                  </select>
                </div>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.props-panel {
  position: fixed;
  top: 44px; /* below toolbar */
  right: 0;
  bottom: 0;
  width: 290px;
  background: var(--surface-1);
  border-left: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  z-index: 50;
  box-shadow: -4px 0 20px rgba(0, 0, 0, 0.25);
}

.panel-slide-enter-active, .panel-slide-leave-active { transition: transform 0.2s ease, opacity 0.2s ease; }
.panel-slide-enter-from, .panel-slide-leave-to { transform: translateX(20px); opacity: 0; }

.pp-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.pp-header-icon { width: 14px; height: 14px; color: var(--text-muted); flex-shrink: 0; }
.pp-header-title {
  flex: 1;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: 0.02em;
}
.pp-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  background: transparent;
  border: none;
  border-radius: 4px;
  color: var(--text-muted);
  cursor: pointer;
  transition: color 0.12s, background 0.12s;
  flex-shrink: 0;
}
.pp-close svg { width: 9px; height: 9px; }
.pp-close:hover { color: var(--text-primary); background: var(--surface-2); }

.pp-body {
  flex: 1;
  overflow-y: auto;
  padding: 6px 0 24px;
}
.pp-body::-webkit-scrollbar { width: 5px; }
.pp-body::-webkit-scrollbar-track { background: transparent; }
.pp-body::-webkit-scrollbar-thumb { background: var(--border); border-radius: 3px; }

.pp-section {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
}
.pp-section:last-child { border-bottom: none; }

.pp-section-label {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
  margin-bottom: 8px;
}

.pp-hint {
  font-size: 10.5px;
  color: var(--text-muted);
  font-style: italic;
  margin-top: 6px;
}

/* ── Chart type grid ── */
.chart-type-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 4px;
}

/* ── Chart help ── */
.chart-help {
  margin-top: 8px;
  padding: 8px 10px;
  background: var(--surface-0, #0d1117);
  border: 1px solid var(--border);
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.chart-help-when {
  font-size: 10.5px;
  color: var(--text-primary);
  line-height: 1.4;
}
.chart-help-cols {
  font-size: 9.5px;
  color: var(--text-muted);
  font-family: var(--font-mono);
  line-height: 1.5;
}
.chart-help-tip {
  font-size: 9.5px;
  color: var(--text-secondary);
  font-style: italic;
  line-height: 1.4;
}
.ct-pill {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 6px 4px 5px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 6px;
  cursor: pointer;
  transition: border-color 0.12s, background 0.12s, color 0.12s;
  color: var(--text-secondary);
}
.ct-pill:hover { border-color: var(--accent); color: var(--text-primary); }
.ct-pill.active { border-color: var(--accent); background: rgba(88, 166, 255, 0.12); color: var(--accent); }
.ct-icon { font-size: 13px; line-height: 1; }
.ct-label { font-size: 9px; font-weight: 600; letter-spacing: 0.03em; white-space: nowrap; }

/* ── Source tabs ── */
.source-tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 8px;
}
.source-tab {
  flex: 1;
  padding: 4px 8px;
  font-size: 11px;
  font-weight: 500;
  font-family: inherit;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-muted);
  cursor: pointer;
  transition: color 0.12s, background 0.12s, border-color 0.12s;
}
.source-tab.active { background: rgba(88, 166, 255, 0.1); border-color: var(--accent); color: var(--accent); }
.source-tab:hover:not(.active) { color: var(--text-primary); background: var(--surface-3, var(--surface-2)); }
.source-tab:disabled { opacity: 0.4; cursor: not-allowed; }

/* ── SQL editor ── */
.pp-section :deep(.sql-editor-wrap) {
  border: 1px solid var(--border);
  border-radius: 5px;
  overflow: hidden;
  background: var(--surface-0, #0d1117);
  transition: border-color 0.12s;
}
.pp-section :deep(.sql-editor-wrap:focus-within) { border-color: var(--accent); }

/* ── Mermaid editor ── */
.mermaid-editor {
  width: 100%;
  min-height: 160px;
  max-height: 340px;
  padding: 8px 10px;
  font-family: var(--font-mono);
  font-size: 11px;
  line-height: 1.6;
  color: var(--text-primary);
  background: var(--surface-0, #0d1117);
  border: 1px solid var(--border);
  border-radius: 5px;
  resize: vertical;
  outline: none;
  box-sizing: border-box;
  transition: border-color 0.12s;
}
.mermaid-editor:focus { border-color: var(--accent); }

/* ── Run button ── */
.run-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  width: 100%;
  padding: 5px 10px;
  margin-top: 6px;
  font-size: 11.5px;
  font-weight: 600;
  font-family: inherit;
  background: rgba(88, 166, 255, 0.12);
  border: 1px solid rgba(88, 166, 255, 0.3);
  border-radius: 5px;
  color: var(--accent);
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
}
.run-btn svg { width: 9px; height: 9px; flex-shrink: 0; }
.run-btn:hover:not(:disabled) { background: rgba(88, 166, 255, 0.2); border-color: var(--accent); }
.run-btn:disabled { opacity: 0.4; cursor: not-allowed; }

.run-error { font-size: 10.5px; color: var(--error); margin-top: 5px; line-height: 1.4; }
.run-ok { font-size: 10px; color: var(--success); margin-top: 5px; font-family: var(--font-mono); }

/* ── Source select ── */
.source-select {
  width: 100%;
  padding: 5px 8px;
  font-size: 11.5px;
  font-family: inherit;
  color: var(--text-primary);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 5px;
  outline: none;
  cursor: pointer;
  transition: border-color 0.12s;
}
.source-select:focus { border-color: var(--accent); }

/* ── Column rows ── */
.col-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.col-row:last-child { margin-bottom: 0; }
.col-row label {
  width: 38px;
  flex-shrink: 0;
  font-size: 10.5px;
  color: var(--text-muted);
  font-weight: 500;
}
.col-row select, .col-row input {
  flex: 1;
  min-width: 0;
  padding: 4px 7px;
  font-size: 11px;
  font-family: inherit;
  color: var(--text-primary);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  outline: none;
  transition: border-color 0.12s;
}
.col-row select:focus, .col-row input:focus { border-color: var(--accent); }

/* ── Shared inputs ── */
.text-input {
  padding: 4px 7px;
  font-size: 11px;
  font-family: inherit;
  color: var(--text-primary);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  outline: none;
  transition: border-color 0.12s;
}
.text-input:focus { border-color: var(--accent); }

.color-input {
  width: 28px;
  height: 24px;
  padding: 2px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--surface-2);
  cursor: pointer;
  flex-shrink: 0;
}

/* ── Boolean rows ── */
.bool-row { display: flex; gap: 8px; margin-bottom: 6px; }
.bool-cell { display: flex; flex-direction: column; gap: 4px; flex: 1; min-width: 0; }
.bool-label-text { font-size: 10px; color: var(--text-muted); font-weight: 500; }
.bool-cell .text-input { width: 100%; box-sizing: border-box; }
.bool-cell .color-input { align-self: flex-start; }

/* ── Condition rules ── */
.cond-rules { display: flex; flex-direction: column; gap: 4px; margin-bottom: 6px; }
.cond-rule { display: flex; align-items: center; gap: 4px; }
.cond-match { width: 80px; flex-shrink: 0; }
.cond-label { flex: 1; min-width: 0; }
.cond-remove {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  flex-shrink: 0;
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  border-radius: 3px;
  transition: color 0.12s, background 0.12s;
}
.cond-remove svg { width: 9px; height: 9px; }
.cond-remove:hover { color: var(--error); background: rgba(248, 81, 73, 0.1); }

.add-cond-btn {
  width: 100%;
  padding: 4px 8px;
  font-size: 11px;
  font-family: inherit;
  background: var(--surface-2);
  border: 1px dashed var(--border);
  border-radius: 5px;
  color: var(--text-muted);
  cursor: pointer;
  transition: color 0.12s, border-color 0.12s, background 0.12s;
}
.add-cond-btn:hover { color: var(--text-primary); border-color: var(--accent); background: rgba(88, 166, 255, 0.05); }

.cond-syntax {
  margin-top: 10px;
  padding: 8px 10px;
  background: var(--surface-0, #0d1117);
  border: 1px solid var(--border);
  border-radius: 6px;
}
.cond-syntax-title {
  font-size: 9.5px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
  margin-bottom: 6px;
}
.cond-syntax-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 10.5px;
  line-height: 1.7;
  color: var(--text-secondary);
}
.cond-syntax-table td { padding: 0; vertical-align: baseline; }
.syn-ex {
  font-family: var(--font-mono);
  color: var(--accent);
  padding-right: 10px;
  white-space: nowrap;
}
.cond-syntax-note {
  margin-top: 6px;
  font-size: 9.5px;
  color: var(--text-muted);
  font-style: italic;
  line-height: 1.4;
}

/* ── Table column config ── */
.tcol-list { display: flex; flex-direction: column; gap: 6px; }

.tcol-row {
  display: flex;
  gap: 6px;
  align-items: flex-start;
  padding: 7px 8px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 6px;
}

.tcol-eye {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  flex-shrink: 0;
  margin-top: 1px;
  background: transparent;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  border-radius: 3px;
  transition: color 0.12s, background 0.12s;
}
.tcol-eye svg { width: 12px; height: 12px; }
.tcol-eye:hover { color: var(--text-primary); background: var(--surface-3, var(--surface-2)); }
.tcol-eye.hidden { color: var(--text-muted); opacity: 0.5; }

.tcol-body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.tcol-body.tcol-hidden { opacity: 0.4; pointer-events: none; }

.tcol-name-row { display: flex; align-items: center; gap: 6px; }
.tcol-key {
  font-size: 10.5px;
  font-family: var(--font-mono);
  color: var(--text-secondary);
  flex-shrink: 0;
  max-width: 80px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tcol-label-input { flex: 1; min-width: 0; font-size: 10.5px !important; padding: 2px 5px !important; }

.tcol-fmt-row { display: flex; align-items: center; gap: 4px; }
.tcol-fmt-select {
  flex: 1;
  min-width: 0;
  padding: 3px 6px;
  font-size: 10.5px;
  font-family: inherit;
  color: var(--text-primary);
  background: var(--surface-0, #0d1117);
  border: 1px solid var(--border);
  border-radius: 4px;
  outline: none;
  cursor: pointer;
}
.tcol-fmt-select:focus { border-color: var(--accent); }

.align-chips { display: flex; gap: 2px; flex-shrink: 0; }
.align-chip {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 3px;
  color: var(--text-muted);
  cursor: pointer;
  transition: color 0.1s, background 0.1s, border-color 0.1s;
}
.align-chip svg { width: 10px; height: 10px; }
.align-chip:hover { color: var(--text-primary); background: var(--surface-2); }
.align-chip.active { color: var(--accent); border-color: var(--accent); background: rgba(88,166,255,0.1); }

.tcol-sub {
  display: flex;
  align-items: center;
  gap: 5px;
  flex-wrap: wrap;
}
.tcol-sub-label { font-size: 9.5px; color: var(--text-muted); flex-shrink: 0; }
.tcol-sub-input {
  width: 46px !important;
  font-size: 10.5px !important;
  padding: 2px 5px !important;
  flex-shrink: 0;
}
.tcol-sub .tcol-fmt-select { flex: 1; }
</style>
