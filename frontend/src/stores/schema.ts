import { defineStore } from 'pinia'
import { ref } from 'vue'

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
  w?: number
  h?: number
  viewMode?: ViewMode
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
}

export interface ConditionRule {
  match: string   // exact string match; '*' = wildcard/default
  label: string
  color: string
}

export interface ChartNode {
  kind: 'chart'
  id: string
  name: string
  x: number
  y: number
  sourceId: string | null
  sql: string
  chartType: 'barY' | 'barX' | 'lineY' | 'areaY' | 'dot' | 'cell' | 'pie' | 'donut' | 'number' | 'boolean' | 'conditional'
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

export type CanvasNode = TableNode | QueryNode | ChartNode | MarkdownNode | SectionNode

const PALETTE = [
  '#6366f1', '#8b5cf6', '#06b6d4', '#10b981',
  '#f59e0b', '#ef4444', '#ec4899', '#3b82f6',
]

let colorCursor = 0

function nextColor(override?: string): string {
  return override ?? PALETTE[colorCursor++ % PALETTE.length]
}

export const useSchemaStore = defineStore('schema', () => {
  const nodes = ref<CanvasNode[]>([])

  function addTable(table: Omit<TableNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === table.id)) return
    nodes.value.push({ kind: 'table', ...table, color: nextColor(table.color) })
  }

  function addQueryNode(node: Omit<QueryNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === node.id)) return
    nodes.value.push({ kind: 'query', ...node, color: nextColor(node.color) })
  }

  function addChartNode(node: Omit<ChartNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === node.id)) return
    nodes.value.push({ kind: 'chart', ...node, color: nextColor(node.color) })
  }

  function addMarkdownNode(node: Omit<MarkdownNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === node.id)) return
    nodes.value.push({ kind: 'markdown', ...node, color: nextColor(node.color) })
  }

  function addSection(node: Omit<SectionNode, 'kind' | 'color'> & { color?: string }) {
    if (nodes.value.some((n) => n.id === node.id)) return
    nodes.value.unshift({ kind: 'section', ...node, color: nextColor(node.color) })
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
    nodes.value = nodes.value.filter((n) => n.id !== id)
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

  function setQueryIsView(id: string, isView: boolean) {
    const n = nodes.value.find((n) => n.id === id)
    if (n?.kind === 'query') n.isView = isView
  }

  function updateChartConfig(id: string, updates: Partial<Pick<ChartNode, 'sourceId' | 'sql' | 'chartType' | 'xColumn' | 'yColumn' | 'colorColumn' | 'labelColumn' | 'chartLabel' | 'trueText' | 'falseText' | 'trueColor' | 'falseColor' | 'conditions'>>) {
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

  function setColorCursor(n: number) {
    colorCursor = n
  }

  function clear() {
    nodes.value = []
    colorCursor = 0
  }

  return {
    nodes,
    addTable, addQueryNode, addChartNode, addMarkdownNode, addSection,
    updatePosition, updatePositions, updateNodeSize, updateViewMode, removeNode, renameNode,
    setRowCount, updateQuerySql, setQueryIsView, setRefreshInterval, updateChartConfig, updateMarkdownContent,
    setColorCursor, clear,
  }
})
