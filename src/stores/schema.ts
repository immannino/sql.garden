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
  id: string
  name: string
  x: number
  y: number
  columns: Column[]
  color: string
  rowCount?: number
}

const PALETTE = [
  '#6366f1', '#8b5cf6', '#06b6d4', '#10b981',
  '#f59e0b', '#ef4444', '#ec4899', '#3b82f6',
]

let colorCursor = 0

export const useSchemaStore = defineStore('schema', () => {
  const tables = ref<TableNode[]>([])

  function addTable(table: Omit<TableNode, 'color'> & { color?: string }) {
    tables.value.push({
      ...table,
      color: table.color ?? PALETTE[colorCursor++ % PALETTE.length],
    })
  }

  function updatePosition(id: string, x: number, y: number) {
    const t = tables.value.find((t) => t.id === id)
    if (t) { t.x = x; t.y = y }
  }

  function removeTable(id: string) {
    tables.value = tables.value.filter((t) => t.id !== id)
  }

  function setRowCount(id: string, count: number) {
    const t = tables.value.find((t) => t.id === id)
    if (t) t.rowCount = count
  }

  function clear() {
    tables.value = []
    colorCursor = 0
  }

  return { tables, addTable, updatePosition, removeTable, setRowCount, clear }
})
