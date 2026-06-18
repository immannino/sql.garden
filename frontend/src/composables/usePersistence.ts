import { watch } from 'vue'
import { useDuckDB } from './useDuckDB'
import { useSchemaStore } from '../stores/schema'
import type { Column, ChartNode, MarkdownNode } from '../stores/schema'
import { IS_DESKTOP } from '../lib/env'
import { usePersistenceWeb } from './usePersistence.web'
import {
  SaveCanvasState,
  LoadCanvasState,
  SaveTableData,
  DeleteTableData,
  GetTableDataPath,
  ImportFromPath,
  // Legacy migration only — remove once all users are on SQLite persistence
  ImportFileFromBase64,
} from '../../wailsjs/go/main/App'
import { toBase64 } from './useDuckDB'

// ── Legacy IDB helpers (migration path only) ──────────────────────────────────

const IDB_NAME = 'sql-garden'
const IDB_STORE = 'tables'
const IDB_VERSION = 1
const LEGACY_CANVAS_KEY = 'sql-garden:canvas:v1'

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

function idbGet(key: string): Promise<Uint8Array | undefined> {
  return openIDB().then((db) => new Promise((resolve, reject) => {
    const req = db.transaction(IDB_STORE, 'readonly').objectStore(IDB_STORE).get(key)
    req.onsuccess = () => resolve(req.result as Uint8Array | undefined)
    req.onerror = () => reject(req.error)
  }))
}

// ── Persistence ───────────────────────────────────────────────────────────────

function useDesktopPersistence() {
  const { query, getTableInfo } = useDuckDB()
  const schemaStore = useSchemaStore()

  // Called after a new table is imported — writes its parquet snapshot to disk.
  async function saveTable(name: string): Promise<void> {
    await SaveTableData(name)
  }

  // Called when a table node is deleted from the canvas.
  async function deleteTable(name: string): Promise<void> {
    await DeleteTableData(name)
  }

  // Serialises the full canvas to SQLite. Called by startAutoSave on every change.
  function saveCanvas(): void {
    const payload = schemaStore.nodes.map((node) => {
      if (node.kind === 'table') {
        const { kind, id, name, x, y, color, columns, w, h, viewMode } = node
        return { kind, id, name, x, y, color, columns, w, h, viewMode }
      }
      if (node.kind === 'query') {
        const { kind, id, name, x, y, color, sql, w, h, viewMode } = node
        return { kind, id, name, x, y, color, sql, w, h, viewMode }
      }
      if (node.kind === 'markdown') {
        const { kind, id, name, x, y, color, content, w, h, viewMode } = node
        return { kind, id, name, x, y, color, content, w, h, viewMode }
      }
      if (node.kind === 'section') {
        const { kind, id, name, x, y, color, w, h } = node
        return { kind, id, name, x, y, color, w, h }
      }
      // chart
      const { kind, id, name, x, y, color, sourceId, sql, chartType, xColumn, yColumn, colorColumn, labelColumn, w, h, viewMode } = node
      return { kind, id, name, x, y, color, sourceId, sql, chartType, xColumn, yColumn, colorColumn, labelColumn, w, h, viewMode }
    })
    SaveCanvasState(JSON.stringify(payload)).catch(console.warn)
  }

  // ── Restore from SQLite ─────────────────────────────────────────────────────

  async function restoreFromSQLite(raw: string): Promise<boolean> {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let saved: any[]
    try { saved = JSON.parse(raw) } catch { return false }
    if (!saved.length) return false

    // Always start from a clean slate — prevents accumulation on HMR or double-mount.
    schemaStore.clear()

    let tablesRestored = 0

    for (const entry of saved) {
      const kind: string = entry.kind ?? 'table'
      try {
        if (kind === 'table') {
          const parquetPath = await GetTableDataPath(entry.name)
          if (!parquetPath) continue

          // Import directly from the parquet file on disk — no base64, no IDB.
          await ImportFromPath(parquetPath, entry.name)

          const safeTable = entry.name.replace(/"/g, '""')
          const [columns, countResult] = await Promise.all([
            getTableInfo(entry.name),
            query(`SELECT COUNT(*) AS n FROM "${safeTable}"`),
          ])
          const rowCount = Number(countResult.rows[0]?.n ?? 0)
          const cols: Column[] = columns

          schemaStore.addTable({ id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color, columns: cols, w: entry.w, h: entry.h, viewMode: entry.viewMode })
          schemaStore.setRowCount(entry.id, rowCount)
          tablesRestored++
        } else if (kind === 'query') {
          schemaStore.addQueryNode({ id: entry.id, name: entry.name, x: entry.x, y: entry.y, sql: entry.sql ?? '', color: entry.color, isView: entry.isView ?? false, w: entry.w, h: entry.h, viewMode: entry.viewMode })
          if (entry.isView && entry.sql?.trim()) {
            import('../../wailsjs/go/main/App').then(({ CreateView }) =>
              CreateView(entry.name, entry.sql).catch(console.warn)
            )
          }
        } else if (kind === 'markdown') {
          const m: Omit<MarkdownNode, 'kind' | 'color'> & { color?: string } = {
            id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color,
            content: entry.content ?? '', w: entry.w, h: entry.h, viewMode: entry.viewMode,
          }
          schemaStore.addMarkdownNode(m)
        } else if (kind === 'chart') {
          const c: Omit<ChartNode, 'kind' | 'color'> & { color?: string } = {
            id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color,
            sourceId: entry.sourceId ?? null, sql: entry.sql ?? '',
            chartType: entry.chartType ?? 'barY', xColumn: entry.xColumn ?? '', yColumn: entry.yColumn ?? '',
            colorColumn: entry.colorColumn, labelColumn: entry.labelColumn,
            w: entry.w, h: entry.h, viewMode: entry.viewMode,
          }
          schemaStore.addChartNode(c)
        } else if (kind === 'section') {
          schemaStore.addSection({ id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color, w: entry.w ?? 400, h: entry.h ?? 300 })
        }
      } catch (e) {
        console.warn(`Failed to restore node "${entry.name ?? entry.id}":`, e)
      }
    }

    if (tablesRestored > 0) {
      schemaStore.setColorCursor(saved.length)
      return true
    }
    // Non-table nodes (query/chart/markdown) still count as a restored canvas.
    if (saved.length > 0) {
      schemaStore.setColorCursor(saved.length)
      return true
    }
    return false
  }

  // ── One-time migration from localStorage + IDB → SQLite ────────────────────

  async function migrateFromLegacy(): Promise<boolean> {
    const localRaw = localStorage.getItem(LEGACY_CANVAS_KEY)
    if (!localRaw) return false

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let saved: any[]
    try { saved = JSON.parse(localRaw) } catch { return false }

    schemaStore.clear()
    let tablesRestored = 0

    for (const entry of saved) {
      const kind: string = entry.kind ?? 'table'
      try {
        if (kind === 'table') {
          const parquet = await idbGet(entry.name)
          if (!parquet) continue

          // Restore via the old base64 path, then persist the parquet to disk.
          await ImportFileFromBase64(
            toBase64(parquet as unknown as Uint8Array<ArrayBuffer>),
            `__restore_${entry.name}.parquet`,
            entry.name,
          )
          await SaveTableData(entry.name)

          const safeTable = entry.name.replace(/"/g, '""')
          const [columns, countResult] = await Promise.all([
            getTableInfo(entry.name),
            query(`SELECT COUNT(*) AS n FROM "${safeTable}"`),
          ])
          const rowCount = Number(countResult.rows[0]?.n ?? 0)
          const cols: Column[] = columns

          schemaStore.addTable({ id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color, columns: cols, w: entry.w, h: entry.h, viewMode: entry.viewMode })
          schemaStore.setRowCount(entry.id, rowCount)
          tablesRestored++
        } else if (kind === 'query') {
          schemaStore.addQueryNode({ id: entry.id, name: entry.name, x: entry.x, y: entry.y, sql: entry.sql ?? '', color: entry.color, w: entry.w, h: entry.h, viewMode: entry.viewMode })
        } else if (kind === 'markdown') {
          const m: Omit<MarkdownNode, 'kind' | 'color'> & { color?: string } = {
            id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color,
            content: entry.content ?? '', w: entry.w, h: entry.h, viewMode: entry.viewMode,
          }
          schemaStore.addMarkdownNode(m)
        } else if (kind === 'chart') {
          const c: Omit<ChartNode, 'kind' | 'color'> & { color?: string } = {
            id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color,
            sourceId: entry.sourceId ?? null, sql: entry.sql ?? '',
            chartType: entry.chartType ?? 'barY', xColumn: entry.xColumn ?? '', yColumn: entry.yColumn ?? '',
            colorColumn: entry.colorColumn, labelColumn: entry.labelColumn,
            w: entry.w, h: entry.h, viewMode: entry.viewMode,
          }
          schemaStore.addChartNode(c)
        } else if (kind === 'section') {
          schemaStore.addSection({ id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color, w: entry.w ?? 400, h: entry.h ?? 300 })
        }
      } catch (e) {
        console.warn(`Migration failed for "${entry.name ?? entry.id}":`, e)
      }
    }

    // Persist the migrated canvas to SQLite and clear the localStorage entry.
    await SaveCanvasState(localRaw).catch(console.warn)
    localStorage.removeItem(LEGACY_CANVAS_KEY)

    if (tablesRestored > 0) {
      schemaStore.setColorCursor(saved.length)
      return true
    }
    if (saved.length > 0) {
      schemaStore.setColorCursor(saved.length)
      return true
    }
    return false
  }

  // ── Public entry point ──────────────────────────────────────────────────────

  async function loadAll(): Promise<boolean> {
    // Primary: SQLite
    const sqliteRaw = await LoadCanvasState()
    if (sqliteRaw) {
      return restoreFromSQLite(sqliteRaw)
    }

    // Fallback: migrate from localStorage + IDB (runs once, then deletes legacy data)
    return migrateFromLegacy()
  }

  let _saveTimer: ReturnType<typeof setTimeout> | null = null

  function startAutoSave(): void {
    watch(
      () => schemaStore.nodes,
      () => {
        if (_saveTimer) clearTimeout(_saveTimer)
        _saveTimer = setTimeout(() => { saveCanvas() }, 1000)
      },
      { deep: true },
    )
  }

  return { loadAll, saveCanvas, saveTable, deleteTable, startAutoSave }
}

export function usePersistence() {
  if (!IS_DESKTOP) return usePersistenceWeb()
  return useDesktopPersistence()
}
