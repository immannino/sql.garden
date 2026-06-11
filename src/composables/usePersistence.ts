import { watch } from 'vue'
import { useDuckDB } from './useDuckDB'
import { useSchemaStore } from '../stores/schema'
import type { Column, ChartNode, MarkdownNode } from '../stores/schema'

const CANVAS_KEY = 'sql-garden:canvas:v1'
const IDB_NAME = 'sql-garden'
const IDB_STORE = 'tables'
const IDB_VERSION = 1

let _idb: IDBDatabase | null = null

function openIDB(): Promise<IDBDatabase> {
  if (_idb) return Promise.resolve(_idb)
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(IDB_NAME, IDB_VERSION)
    req.onupgradeneeded = () => { req.result.createObjectStore(IDB_STORE) }
    req.onsuccess = () => { _idb = req.result; resolve(_idb) }
    req.onerror = () => reject(req.error)
  })
}

function idbPut(key: string, value: Uint8Array): Promise<void> {
  return openIDB().then((db) => new Promise((resolve, reject) => {
    const req = db.transaction(IDB_STORE, 'readwrite').objectStore(IDB_STORE).put(value, key)
    req.onsuccess = () => resolve()
    req.onerror = () => reject(req.error)
  }))
}

function idbGet(key: string): Promise<Uint8Array | undefined> {
  return openIDB().then((db) => new Promise((resolve, reject) => {
    const req = db.transaction(IDB_STORE, 'readonly').objectStore(IDB_STORE).get(key)
    req.onsuccess = () => resolve(req.result as Uint8Array | undefined)
    req.onerror = () => reject(req.error)
  }))
}

function idbDelete(key: string): Promise<void> {
  return openIDB().then((db) => new Promise((resolve, reject) => {
    const req = db.transaction(IDB_STORE, 'readwrite').objectStore(IDB_STORE).delete(key)
    req.onsuccess = () => resolve()
    req.onerror = () => reject(req.error)
  }))
}

export function usePersistence() {
  const { exec, query, registerFile, dropFile, copyTableToBuffer, getTableInfo } = useDuckDB()
  const schemaStore = useSchemaStore()

  async function saveTable(name: string): Promise<void> {
    const buffer = await copyTableToBuffer(name)
    await idbPut(name, buffer)
  }

  async function deleteTable(name: string): Promise<void> {
    await idbDelete(name)
  }

  function saveCanvas(): void {
    const payload = schemaStore.nodes.map((node) => {
      if (node.kind === 'table') {
        const { kind, id, name, x, y, color, columns, w, h } = node
        return { kind, id, name, x, y, color, columns, w, h }
      }
      if (node.kind === 'query') {
        const { kind, id, name, x, y, color, sql, w, h } = node
        return { kind, id, name, x, y, color, sql, w, h }
      }
      if (node.kind === 'markdown') {
        const { kind, id, name, x, y, color, content, w, h } = node
        return { kind, id, name, x, y, color, content, w, h }
      }
      // chart
      const { kind, id, name, x, y, color, sourceId, sql, chartType, xColumn, yColumn, w, h } = node
      return { kind, id, name, x, y, color, sourceId, sql, chartType, xColumn, yColumn, w, h }
    })
    localStorage.setItem(CANVAS_KEY, JSON.stringify(payload))
  }

  async function loadAll(): Promise<boolean> {
    const raw = localStorage.getItem(CANVAS_KEY)
    if (!raw) return false

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let saved: any[]
    try { saved = JSON.parse(raw) } catch { return false }
    if (!saved.length) return false

    let tablesRestored = 0

    for (const entry of saved) {
      const kind: string = entry.kind ?? 'table'

      try {
        if (kind === 'table') {
          const parquet = await idbGet(entry.name)
          if (!parquet) continue  // IDB missing — skip this table, may seed later

          const fileName = `__restore_${entry.name}.parquet`
          await registerFile(fileName, parquet)
          const safeFile  = fileName.replace(/'/g, "''")
          const safeTable = entry.name.replace(/"/g, '""')
          await exec(`CREATE TABLE "${safeTable}" AS SELECT * FROM read_parquet('${safeFile}')`)
          await dropFile(fileName)

          const [columns, countResult] = await Promise.all([
            getTableInfo(entry.name),
            query(`SELECT COUNT(*) AS n FROM "${safeTable}"`),
          ])
          const rowCount = Number(countResult.rows[0]?.n ?? 0)
          const cols: Column[] = columns

          schemaStore.addTable({ id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color, columns: cols, w: entry.w, h: entry.h })
          schemaStore.setRowCount(entry.id, rowCount)
          tablesRestored++
        } else if (kind === 'query') {
          // Query/chart nodes have no IDB dependency — always restore them.
          // They do NOT count toward tablesRestored so a chart-only canvas
          // still triggers the seed (the seed tables will coexist with them).
          schemaStore.addQueryNode({ id: entry.id, name: entry.name, x: entry.x, y: entry.y, sql: entry.sql ?? '', color: entry.color, w: entry.w, h: entry.h })
        } else if (kind === 'markdown') {
          const m: Omit<MarkdownNode, 'kind' | 'color'> & { color?: string } = {
            id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color,
            content: entry.content ?? '', w: entry.w, h: entry.h,
          }
          schemaStore.addMarkdownNode(m)
        } else if (kind === 'chart') {
          const c: Omit<ChartNode, 'kind' | 'color'> & { color?: string } = {
            id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color,
            sourceId: entry.sourceId ?? null, sql: entry.sql ?? '',
            chartType: entry.chartType ?? 'barY', xColumn: entry.xColumn ?? '', yColumn: entry.yColumn ?? '',
            w: entry.w, h: entry.h,
          }
          schemaStore.addChartNode(c)
        }
      } catch (e) {
        console.warn(`Failed to restore node "${entry.name ?? entry.id}":`, e)
      }
    }

    // Only suppress the default seed when real table data came back from IDB.
    // If tables were deleted (IDB empty) but chart/query nodes survived in
    // localStorage, we still seed — those nodes will coexist with the defaults.
    if (tablesRestored > 0) {
      schemaStore.setColorCursor(saved.length)
      return true
    }
    return false
  }

  let _saveTimer: ReturnType<typeof setTimeout> | null = null

  function startAutoSave(): void {
    watch(
      schemaStore.nodes,
      () => {
        if (_saveTimer) clearTimeout(_saveTimer)
        _saveTimer = setTimeout(() => { saveCanvas() }, 1000)
      },
      { deep: true },
    )
  }

  return { loadAll, saveCanvas, saveTable, deleteTable, startAutoSave }
}
