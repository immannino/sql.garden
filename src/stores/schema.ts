import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Column {
  name: string
  type: string
  primaryKey?: boolean
  nullable?: boolean
  references?: { table: string; column: string }
}

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
}

export interface QueryNode {
  kind: 'query'
  id: string
  name: string
  x: number
  y: number
  sql: string
  color: string
  w?: number
  h?: number
}

export interface ChartNode {
  kind: 'chart'
  id: string
  name: string
  x: number
  y: number
  sourceId: string | null
  sql: string
  chartType: 'barY' | 'lineY' | 'areaY' | 'dot'
  xColumn: string
  yColumn: string
  color: string
  w?: number
  h?: number
}

export type CanvasNode = TableNode | QueryNode | ChartNode

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
    nodes.value.push({ kind: 'table', ...table, color: nextColor(table.color) })
  }

  function addQueryNode(node: Omit<QueryNode, 'kind' | 'color'> & { color?: string }) {
    nodes.value.push({ kind: 'query', ...node, color: nextColor(node.color) })
  }

  function addChartNode(node: Omit<ChartNode, 'kind' | 'color'> & { color?: string }) {
    nodes.value.push({ kind: 'chart', ...node, color: nextColor(node.color) })
  }

  function updatePosition(id: string, x: number, y: number) {
    const n = nodes.value.find((n) => n.id === id)
    if (n) { n.x = x; n.y = y }
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

  function updateChartConfig(id: string, updates: Partial<Pick<ChartNode, 'sourceId' | 'sql' | 'chartType' | 'xColumn' | 'yColumn'>>) {
    const n = nodes.value.find((n) => n.id === id)
    if (n?.kind === 'chart') Object.assign(n, updates)
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
    addTable, addQueryNode, addChartNode,
    updatePosition, updateNodeSize, removeNode, renameNode,
    setRowCount, updateQuerySql, updateChartConfig,
    setColorCursor, clear,
  }
})
