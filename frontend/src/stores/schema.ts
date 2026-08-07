import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export interface Column {
  name: string
  type: string
  primaryKey?: boolean
  nullable?: boolean
  references?: { table: string; column: string }
}

export type ViewMode = 'default' | 'collapsed' | 'chart-only'

export interface TableNode {
  kind: 'table'
  id: string
  name: string
  x: number
  y: number
  columns: Column[]
  color: string
  rowCount?: number
  columnCasts?: Record<string, { type: string; expr: string }>
  w?: number
  h?: number
  viewMode?: ViewMode
}

export interface SqlHistoryEntry {
  sql: string
  ts: number
}

export interface QueryNode {
  kind: 'query'
  id: string
  name: string
  x: number
  y: number
  sql: string
  color: string
  isView?: boolean
  refreshInterval?: number
  w?: number
  h?: number
  viewMode?: ViewMode
  sqlHistory?: SqlHistoryEntry[]
}

export interface ConditionRule {
  // Operators: >, >=, <, <=, !=, = N   (numeric when both sides are numbers)
  //            contains X, starts X, ends X   (case-insensitive string ops)
  //            bare value → exact match   |   * → catch-all
  match: string
  label: string
  color: string
}

export type ColumnFormatType = 'auto' | 'number' | 'currency' | 'percent' | 'date' | 'text'
export type ColumnAlign = 'left' | 'center' | 'right'
export type DatePattern = 'date' | 'datetime' | 'iso' | 'us' | 'eu' | 'relative'

export interface TableColumnConfig {
  hidden?: boolean
  label?: string          // display name override
  formatType?: ColumnFormatType
  decimals?: number       // number / currency / percent
  currencySymbol?: string // currency (default '$')
  datePattern?: DatePattern
  align?: ColumnAlign
}

export interface ChartNode {
  kind: 'chart'
  id: string
  name: string
  x: number
  y: number
  sourceId: string | null
  sql: string
  chartType: 'barY' | 'barX' | 'lineY' | 'areaY' | 'dot' | 'cell' | 'pie' | 'donut' | 'histogram' | 'boxplot' | 'sankey' | 'waterfall' | 'heatmap' | 'scatter-matrix' | 'number' | 'boolean' | 'conditional' | 'mermaid' | 'table'
  xColumn: string
  yColumn: string
  colorColumn?: string
  labelColumn?: string
  // number type
  chartLabel?: string
  // boolean type
  trueText?: string
  falseText?: string
  trueColor?: string
  falseColor?: string
  // conditional type
  conditions?: ConditionRule[]
  // mermaid type
  mermaidCode?: string
  // scatter-matrix type
  matrixColumns?: string[]
  // table type
  tableColumnConfigs?: Record<string, TableColumnConfig>
  color: string
  w?: number
  h?: number
  viewMode?: ViewMode
}

export interface MarkdownNode {
  kind: 'markdown'
  id: string
  name: string
  x: number
  y: number
  content: string
  color: string
  w?: number
  h?: number
  viewMode?: ViewMode
}

export interface SectionNode {
  kind: 'section'
  id: string
  name: string
  x: number
  y: number
  w: number
  h: number
  color: string
  viewMode?: ViewMode
}

export interface DataNode {
  kind: 'data'
  id: string
  name: string          // also the DuckDB table name
  x: number
  y: number
  color: string
  columns: Column[]
  rowCount?: number
  sourceId?: string     // QueryNode that created this
  sourceSql?: string    // SQL used at materialize time — for stale detection
  w?: number
  h?: number
  viewMode?: ViewMode
  columnCasts?: Record<string, { type: string; expr: string }>
}

export interface IngestNode {
  kind: 'ingest'
  id: string
  name: string
  x: number
  y: number
  color: string
  w?: number
  h?: number
  viewMode?: ViewMode
  mode: 'generator' | 'ingestion'
  sql: string           // generator: DML to run on schedule
  url: string           // ingestion: URL to fetch
  targetTable: string   // ingestion: DuckDB table name to write to
  conflictMode: 'append' | 'replace'
  interval: number      // seconds; 0 = manual only
  lastRunAt?: number
  lastRowsAdded?: number
}

export type ExerciseCheckKind =
  | 'set_match'     // result rows == reference SQL rows (order-insensitive)
  | 'row_count'     // row count satisfies operator + expected
  | 'non_empty'     // at least 1 row
  | 'no_nulls'      // no NULLs in specified column
  | 'column_value'  // aggregate expression on result equals expected value
  | 'column_exists' // column name present in result
  | 'sql_pattern'   // student SQL matches / must-not-match a regex

export type CountOperator = '==' | '>=' | '<=' | '>' | '<'

export interface ExerciseCheck {
  id: string
  kind: ExerciseCheckKind
  label: string            // displayed in the check list ("Returns 5 rows")
  feedbackOnFail: string   // shown when this check fails
  hint?: string            // optionally revealed after N failures
  // set_match
  referenceSql?: string
  // row_count
  expectedCount?: number
  countOperator?: CountOperator
  // no_nulls / column_exists / column_value
  column?: string
  // column_value — SQL expression evaluated as: SELECT <expression> FROM (<studentSql>) t
  expression?: string
  expectedValue?: number | string
  tolerance?: number       // for floating-point comparison
  // sql_pattern
  pattern?: string
  mustMatch?: boolean      // true = must match (default), false = must NOT match
}

export interface ExerciseNode {
  kind: 'exercise'
  id: string
  name: string
  x: number
  y: number
  color: string
  w?: number
  h?: number
  viewMode?: ViewMode
  sql: string              // student's starter SQL (edited in-card)
  prompt: string           // markdown shown to student
  successText?: string     // shown to student after all checks pass
  checks: ExerciseCheck[]
  revealHintsAfter: number // attempts before hints auto-reveal; 0 = never
  nextId?: string          // ID of next ExerciseNode in the lesson chain
}

export type TestAssertionMode = 'no_rows' | 'scalar_equals' | 'row_count'
export type TestOperator = '==' | '>=' | '<=' | '>' | '<'

export interface TestNode {
  kind: 'test'
  id: string
  name: string
  x: number
  y: number
  color: string
  w?: number
  h?: number
  viewMode?: ViewMode
  sql: string                      // query to run
  assertionMode: TestAssertionMode // how to evaluate the result
  expectedValue?: number           // scalar_equals / row_count: the target value
  operator?: TestOperator          // scalar_equals / row_count: comparison operator (default ==)
  interval: number                 // seconds between auto-runs; 0 = manual only
  history: { ts: number; passed: boolean }[]
}

export type CanvasNode = TableNode | QueryNode | ChartNode | MarkdownNode | SectionNode | DataNode | IngestNode | ExerciseNode | TestNode

export interface CanvasTab {
  id: string
  name: string
  nodes: CanvasNode[]
}

const PALETTE = [
  '#6366f1', '#8b5cf6', '#06b6d4', '#10b981',
  '#f59e0b', '#ef4444', '#ec4899', '#3b82f6',
]

let colorCursor = 0

function nextColor(override?: string): string {
  return override ?? PALETTE[colorCursor++ % PALETTE.length]
}

export const useSchemaStore = defineStore('schema', () => {
  // ── Multi-canvas ────────────────────────────────────────────────────────────
  const _canvases = ref<CanvasTab[]>([{ id: 'tab_1', name: 'Main', nodes: [] }])
  const _activeId  = ref<string>('tab_1')
  const _viewports = ref<Record<string, { x: number; y: number; zoom: number }>>({})

  const canvases      = computed(() => _canvases.value)
  const activeCanvasId = computed(() => _activeId.value)
  const _activeCanvas = computed(() => _canvases.value.find(c => c.id === _activeId.value) ?? _canvases.value[0])

  // Writable computed — all existing node mutations work unchanged
  const nodes = computed({
    get: () => _activeCanvas.value.nodes,
    set: (val: CanvasNode[]) => { _activeCanvas.value.nodes = val },
  })

  function addCanvas(nameOrOpts?: string | { id?: string; name?: string }): string {
    const resolvedId = typeof nameOrOpts === 'object' && nameOrOpts?.id ? nameOrOpts.id : `tab_${Date.now()}`
    const resolvedName = typeof nameOrOpts === 'string'
      ? nameOrOpts
      : (typeof nameOrOpts === 'object' ? nameOrOpts?.name : undefined) ?? `Canvas ${_canvases.value.length + 1}`
    _canvases.value.push({ id: resolvedId, name: resolvedName, nodes: [] })
    switchCanvas(resolvedId)
    return resolvedId
  }

  function addNodeToCanvas(canvasId: string, addFn: () => void) {
    if (!canvasId || canvasId === _activeId.value) { addFn(); return }
    const saved = _activeId.value
    _activeId.value = canvasId
    addFn()
    _activeId.value = saved
  }

  function removeCanvas(id: string) {
    if (_canvases.value.length <= 1) return
    const idx = _canvases.value.findIndex(c => c.id === id)
    if (idx === -1) return
    const wasActive = _activeId.value === id
    _canvases.value.splice(idx, 1)
    if (wasActive) {
      _activeId.value = _canvases.value[Math.max(0, idx - 1)].id
      _undoStack.value = []
      _redoStack.value = []
    }
  }

  function renameCanvas(id: string, name: string) {
    const tab = _canvases.value.find(c => c.id === id)
    if (tab && name.trim()) tab.name = name.trim()
  }

  function switchCanvas(id: string) {
    if (id === _activeId.value) return
    _undoStack.value = []
    _redoStack.value = []
    _activeId.value = id
  }

  function saveViewport(id: string, vp: { x: number; y: number; zoom: number }) {
    _viewports.value[id] = vp
  }

  function getViewport(id: string): { x: number; y: number; zoom: number } | undefined {
    return _viewports.value[id]
  }

  // Called by persistence layer before filling nodes per-canvas
  function loadCanvases(tabs: Array<{ id: string; name: string }>, activeId: string) {
    _canvases.value = tabs.map(t => ({ id: t.id, name: t.name, nodes: [] }))
    _activeId.value = tabs.find(t => t.id === activeId) ? activeId : (tabs[0]?.id ?? 'tab_1')
    _viewports.value = {}
    _undoStack.value = []
    _redoStack.value = []
    colorCursor = 0
  }

  // ── Undo / Redo ────────────────────────────────────────────────────────────
  const _undoStack = ref<string[]>([])
  const _redoStack = ref<string[]>([])
  const MAX_HISTORY = 60

  function snapshot() {
    _undoStack.value.push(JSON.stringify(nodes.value))
    if (_undoStack.value.length > MAX_HISTORY) _undoStack.value.shift()
    _redoStack.value = []
  }

  function undo() {
    const prev = _undoStack.value.pop()
    if (prev === undefined) return
    _redoStack.value.push(JSON.stringify(nodes.value))
    nodes.value = JSON.parse(prev)
  }

  function redo() {
    const next = _redoStack.value.pop()
    if (next === undefined) return
    _undoStack.value.push(JSON.stringify(nodes.value))
    nodes.value = JSON.parse(next)
  }

  const canUndo = computed(() => _undoStack.value.length > 0)
  const canRedo = computed(() => _redoStack.value.length > 0)

  function addTable(table: Omit<TableNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === table.id)) return
    snapshot()
    nodes.value.push({ kind: 'table', ...table, color: nextColor(table.color) })
  }

  function addQueryNode(node: Omit<QueryNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === node.id)) return
    snapshot()
    nodes.value.push({ kind: 'query', ...node, color: nextColor(node.color) })
  }

  function addChartNode(node: Omit<ChartNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === node.id)) return
    snapshot()
    nodes.value.push({ kind: 'chart', ...node, color: nextColor(node.color) })
  }

  function addMarkdownNode(node: Omit<MarkdownNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === node.id)) return
    snapshot()
    nodes.value.push({ kind: 'markdown', ...node, color: nextColor(node.color) })
  }

  function addSection(node: Omit<SectionNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === node.id)) return
    snapshot()
    nodes.value.unshift({ kind: 'section', ...node, color: nextColor(node.color) })
  }

  function addDataNode(node: Omit<DataNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === node.id)) return
    snapshot()
    nodes.value.push({ kind: 'data', ...node, color: nextColor(node.color) })
  }

  function addIngestNode(node: Omit<IngestNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === node.id)) return
    snapshot()
    nodes.value.push({ kind: 'ingest', ...node, color: nextColor(node.color) })
  }

  function addExerciseNode(node: Omit<ExerciseNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === node.id)) return
    snapshot()
    nodes.value.push({ kind: 'exercise', ...node, color: nextColor(node.color) })
  }

  function updateExerciseNode(id: string, updates: Partial<Pick<ExerciseNode, 'prompt' | 'checks' | 'sql' | 'revealHintsAfter' | 'name' | 'successText' | 'nextId'>>) {
    const n = nodes.value.find((n) => n.id === id)
    if (n?.kind === 'exercise') Object.assign(n, updates)
  }

  function addTestNode(node: Omit<TestNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === node.id)) return
    snapshot()
    nodes.value.push({ kind: 'test', ...node, color: nextColor(node.color) })
  }

  function updateTestNode(id: string, updates: Partial<Pick<TestNode, 'sql' | 'assertionMode' | 'expectedValue' | 'operator' | 'interval' | 'name' | 'history'>>) {
    const n = nodes.value.find((n) => n.id === id)
    if (n?.kind === 'test') Object.assign(n, updates)
  }

  function updateIngestNode(id: string, updates: Partial<Pick<IngestNode, 'sql' | 'url' | 'targetTable' | 'conflictMode' | 'interval' | 'mode' | 'lastRunAt' | 'lastRowsAdded' | 'name'>>) {
    const n = nodes.value.find((n) => n.id === id)
    if (n?.kind === 'ingest') Object.assign(n, updates)
  }

  function updateDataNode(id: string, updates: Partial<Pick<DataNode, 'columns' | 'rowCount' | 'sourceSql' | 'name' | 'columnCasts'>>) {
    const n = nodes.value.find((n) => n.id === id)
    if (n?.kind === 'data') Object.assign(n, updates)
  }

  function updateTableNode(id: string, updates: Partial<Pick<TableNode, 'columns' | 'rowCount' | 'columnCasts'>>) {
    const n = nodes.value.find((n) => n.id === id)
    if (n?.kind === 'table') Object.assign(n, updates)
  }

  function updatePosition(id: string, x: number, y: number) {
    const n = nodes.value.find((n) => n.id === id)
    if (n) { n.x = x; n.y = y }
  }

  function updatePositions(positions: Map<string, { x: number; y: number }>) {
    for (const node of nodes.value) {
      const p = positions.get(node.id)
      if (p) { node.x = p.x; node.y = p.y }
    }
  }

  function removeNode(id: string) {
    snapshot()
    nodes.value = nodes.value.filter((n) => n.id !== id)
  }

  function moveNodeToIndex(id: string, toStoreIndex: number) {
    const from = nodes.value.findIndex((n) => n.id === id)
    if (from === -1) return
    snapshot()
    const arr = [...nodes.value]
    const [node] = arr.splice(from, 1)
    const clampedTo = Math.max(0, Math.min(arr.length, toStoreIndex > from ? toStoreIndex - 1 : toStoreIndex))
    arr.splice(clampedTo, 0, node)
    nodes.value = arr
  }

  function bringToFront(id: string) {
    const idx = nodes.value.findIndex((n) => n.id === id)
    if (idx === -1 || idx === nodes.value.length - 1) return
    snapshot()
    const arr = nodes.value.filter((n) => n.id !== id)
    nodes.value = [...arr, nodes.value[idx]]
  }

  function sendToBack(id: string) {
    const idx = nodes.value.findIndex((n) => n.id === id)
    if (idx === -1 || idx === 0) return
    snapshot()
    const arr = nodes.value.filter((n) => n.id !== id)
    nodes.value = [nodes.value[idx], ...arr]
  }

  function renameNode(id: string, newName: string) {
    const n = nodes.value.find((n) => n.id === id)
    if (n) { n.id = newName; n.name = newName }
  }

  function setRowCount(id: string, count: number) {
    const n = nodes.value.find((n) => n.id === id)
    if (n?.kind === 'table') n.rowCount = count
  }

  function updateQuerySql(id: string, sql: string) {
    const n = nodes.value.find((n) => n.id === id)
    if (n?.kind === 'query') n.sql = sql
  }

  function pushQueryHistory(id: string, sql: string) {
    const n = nodes.value.find((n) => n.id === id)
    if (!n || n.kind !== 'query') return
    const trimmed = sql.trim()
    if (!trimmed) return
    const hist = n.sqlHistory ?? []
    if (hist[0]?.sql === trimmed) return  // skip identical consecutive entries
    n.sqlHistory = [{ sql: trimmed, ts: Date.now() }, ...hist].slice(0, 30)
  }

  function setQueryIsView(id: string, isView: boolean) {
    const n = nodes.value.find((n) => n.id === id)
    if (n?.kind === 'query') n.isView = isView
  }

  function updateChartConfig(id: string, updates: Partial<Pick<ChartNode, 'sourceId' | 'sql' | 'chartType' | 'xColumn' | 'yColumn' | 'colorColumn' | 'labelColumn' | 'chartLabel' | 'trueText' | 'falseText' | 'trueColor' | 'falseColor' | 'conditions' | 'mermaidCode' | 'matrixColumns' | 'tableColumnConfigs'>>) {
    const n = nodes.value.find((n) => n.id === id)
    if (n?.kind === 'chart') Object.assign(n, updates)
  }

  function updateMarkdownContent(id: string, content: string) {
    const n = nodes.value.find((n) => n.id === id)
    if (n?.kind === 'markdown') n.content = content
  }

  function setRefreshInterval(id: string, seconds: number) {
    const n = nodes.value.find((n) => n.id === id)
    if (n?.kind === 'query') n.refreshInterval = seconds
  }

  function updateViewMode(id: string, mode: ViewMode) {
    const idx = nodes.value.findIndex((n) => n.id === id)
    if (idx >= 0) nodes.value.splice(idx, 1, { ...nodes.value[idx], viewMode: mode })
  }

  function updateNodeSize(id: string, w: number, h: number) {
    const n = nodes.value.find((n) => n.id === id)
    if (!n) return
    n.w = w
    n.h = h
  }

  function setNodeColor(id: string, color: string) {
    const n = nodes.value.find((n) => n.id === id)
    if (n) n.color = color
  }

  function duplicateNode(id: string): string | null {
    const src = nodes.value.find((n) => n.id === id)
    if (!src) return null

    const existingNames = new Set(nodes.value.map((n) => n.name))
    function uniqueName(base: string): string {
      let candidate = `${base}_copy`
      let i = 2
      while (existingNames.has(candidate)) candidate = `${base}_copy${i++}`
      return candidate
    }

    const newId = `${src.kind}_${Date.now()}`
    const name = uniqueName(src.name)
    const OFFSET = 24

    let clone: CanvasNode
    if (src.kind === 'table') {
      clone = { ...src, id: newId, name, x: src.x + OFFSET, y: src.y + OFFSET,
        columns: src.columns.map((c) => ({ ...c })) }
    } else if (src.kind === 'query') {
      clone = { ...src, id: newId, name, x: src.x + OFFSET, y: src.y + OFFSET, isView: false }
    } else if (src.kind === 'chart') {
      clone = { ...src, id: newId, name, x: src.x + OFFSET, y: src.y + OFFSET,
        conditions: src.conditions ? src.conditions.map((r) => ({ ...r })) : undefined }
    } else if (src.kind === 'markdown') {
      clone = { ...src, id: newId, name, x: src.x + OFFSET, y: src.y + OFFSET }
    } else if (src.kind === 'data') {
      clone = { ...src, id: newId, name, x: src.x + OFFSET, y: src.y + OFFSET,
        columns: src.columns.map((c) => ({ ...c })) }
    } else {
      clone = { ...src, id: newId, name, x: src.x + OFFSET, y: src.y + OFFSET }
    }

    snapshot()
    nodes.value.push(clone)
    return newId
  }

  function copyNodesToCanvas(ids: string[], targetCanvasId: string) {
    const target = _canvases.value.find(c => c.id === targetCanvasId)
    if (!target || targetCanvasId === _activeId.value) return
    const toCopy = nodes.value.filter(n => ids.includes(n.id))
    if (!toCopy.length) return
    const existingNames = new Set(target.nodes.map(n => n.name))
    const ts = Date.now()
    toCopy.forEach((src, i) => {
      let name = src.name; let counter = 2
      while (existingNames.has(name)) name = `${src.name}_${counter++}`
      existingNames.add(name)
      const newId = `${src.kind}_${ts}_${i}`
      let clone: CanvasNode
      if (src.kind === 'table') {
        clone = { ...src, id: newId, name, columns: src.columns.map(c => ({ ...c })) }
      } else if (src.kind === 'data') {
        clone = { ...src, id: newId, name, columns: src.columns.map(c => ({ ...c })) }
      } else if (src.kind === 'chart') {
        clone = { ...src, id: newId, name, conditions: src.conditions?.map(r => ({ ...r })) }
      } else {
        clone = { ...src, id: newId, name }
      }
      target.nodes.push(clone)
    })
  }

  function setColorCursor(n: number) {
    colorCursor = n
  }

  function clear() {
    _canvases.value = [{ id: 'tab_1', name: 'Main', nodes: [] }]
    _activeId.value = 'tab_1'
    _viewports.value = {}
    colorCursor = 0
    _undoStack.value = []
    _redoStack.value = []
  }

  return {
    nodes,
    canvases, activeCanvasId,
    addCanvas, removeCanvas, renameCanvas, switchCanvas, addNodeToCanvas, saveViewport, getViewport, loadCanvases,
    addTable, addQueryNode, addChartNode, addMarkdownNode, addSection, addDataNode, updateDataNode, updateTableNode, addIngestNode, updateIngestNode, addExerciseNode, updateExerciseNode, addTestNode, updateTestNode,
    updatePosition, updatePositions, updateNodeSize, updateViewMode, removeNode, renameNode,
    moveNodeToIndex, bringToFront, sendToBack,
    setRowCount, updateQuerySql, pushQueryHistory, setQueryIsView, setRefreshInterval, updateChartConfig, updateMarkdownContent,
    setNodeColor, duplicateNode, copyNodesToCanvas, setColorCursor, clear,
    snapshot, undo, redo, canUndo, canRedo,
  }
})
