import { watch } from 'vue'
import { useDuckDB } from './useDuckDB'
import { useSchemaStore } from '../stores/schema'
import type { Column, CanvasNode, ChartNode, MarkdownNode, DataNode, IngestNode, ExerciseNode, TestNode } from '../stores/schema'
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
  const { query, exec, getTableInfo } = useDuckDB()
  const schemaStore = useSchemaStore()

  // Called after a new table is imported — writes its parquet snapshot to disk.
  async function saveTable(name: string): Promise<void> {
    await SaveTableData(name)
  }

  // Called when a table node is deleted from the canvas.
  async function deleteTable(name: string): Promise<void> {
    await DeleteTableData(name)
  }

  function serializeNode(node: CanvasNode): Record<string, unknown> {
    if (node.kind === 'table') {
      const { kind, id, name, x, y, color, columns, columnCasts, w, h, viewMode } = node
      return { kind, id, name, x, y, color, columns, columnCasts, w, h, viewMode }
    }
    if (node.kind === 'data') {
      const { kind, id, name, x, y, color, columns, rowCount, sourceId, sourceSql, columnCasts, w, h, viewMode } = node
      return { kind, id, name, x, y, color, columns, rowCount, sourceId, sourceSql, columnCasts, w, h, viewMode }
    }
    if (node.kind === 'query') {
      const { kind, id, name, x, y, color, sql, isView, refreshInterval, w, h, viewMode } = node
      return { kind, id, name, x, y, color, sql, isView, refreshInterval, w, h, viewMode }
    }
    if (node.kind === 'markdown') {
      const { kind, id, name, x, y, color, content, w, h, viewMode } = node
      return { kind, id, name, x, y, color, content, w, h, viewMode }
    }
    if (node.kind === 'section') {
      const { kind, id, name, x, y, color, w, h } = node
      return { kind, id, name, x, y, color, w, h }
    }
    if (node.kind === 'ingest') {
      const { kind, id, name, x, y, color, mode, sql, url, targetTable, conflictMode, interval, w, h, viewMode } = node
      return { kind, id, name, x, y, color, mode, sql, url, targetTable, conflictMode, interval, w, h, viewMode }
    }
    if (node.kind === 'exercise') {
      const { kind, id, name, x, y, color, sql, prompt, successText, checks, revealHintsAfter, nextId, w, h } = node
      return { kind, id, name, x, y, color, sql, prompt, successText, checks, revealHintsAfter, nextId, w, h }
    }
    if (node.kind === 'test') {
      const { kind, id, name, x, y, color, sql, assertionMode, expectedValue, operator, interval, w, h } = node
      return { kind, id, name, x, y, color, sql, assertionMode, expectedValue, operator, interval, w, h }
    }
    // chart — include all config fields
    const { kind, id, name, x, y, color, sourceId, sql, chartType, xColumn, yColumn, colorColumn, labelColumn,
            chartLabel, trueText, falseText, trueColor, falseColor, conditions, mermaidCode, matrixColumns,
            tableColumnConfigs, w, h, viewMode } = node
    return { kind, id, name, x, y, color, sourceId, sql, chartType, xColumn, yColumn, colorColumn, labelColumn,
             chartLabel, trueText, falseText, trueColor, falseColor, conditions, mermaidCode, matrixColumns,
             tableColumnConfigs, w, h, viewMode }
  }

  // Serialises all canvases to SQLite. Called by startAutoSave on every change.
  function saveCanvas(): void {
    const payload = {
      version: 2,
      activeCanvasId: schemaStore.activeCanvasId,
      canvases: schemaStore.canvases.map(tab => ({
        id: tab.id,
        name: tab.name,
        nodes: tab.nodes.map(serializeNode),
      })),
    }
    SaveCanvasState(JSON.stringify(payload)).catch(console.warn)
  }

  // ── Restore from SQLite ─────────────────────────────────────────────────────

  // Restores a flat array of node entries into the currently active canvas.
  // Returns the number of physical tables/data nodes actually loaded.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async function restoreNodeEntries(entries: any[]): Promise<number> {
    let tablesRestored = 0
    for (const entry of entries) {
      const kind: string = entry.kind ?? 'table'
      try {
        if (kind === 'table') {
          const parquetPath = await GetTableDataPath(entry.name)
          if (!parquetPath) continue
          await ImportFromPath(parquetPath, entry.name)
          const safeTable = entry.name.replace(/"/g, '""')
          const [columns, countResult] = await Promise.all([
            getTableInfo(entry.name),
            query(`SELECT COUNT(*) AS n FROM "${safeTable}"`),
          ])
          schemaStore.addTable({ id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color, columns: columns as Column[], columnCasts: entry.columnCasts, w: entry.w, h: entry.h, viewMode: entry.viewMode })
          schemaStore.setRowCount(entry.id, Number(countResult.rows[0]?.n ?? 0))
          tablesRestored++
        } else if (kind === 'query') {
          schemaStore.addQueryNode({ id: entry.id, name: entry.name, x: entry.x, y: entry.y, sql: entry.sql ?? '', color: entry.color, isView: entry.isView ?? false, refreshInterval: entry.refreshInterval, w: entry.w, h: entry.h, viewMode: entry.viewMode })
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
            chartLabel: entry.chartLabel, trueText: entry.trueText, falseText: entry.falseText,
            trueColor: entry.trueColor, falseColor: entry.falseColor, conditions: entry.conditions,
            mermaidCode: entry.mermaidCode, matrixColumns: entry.matrixColumns,
            tableColumnConfigs: entry.tableColumnConfigs,
            w: entry.w, h: entry.h, viewMode: entry.viewMode,
          }
          schemaStore.addChartNode(c)
        } else if (kind === 'ingest') {
          schemaStore.addIngestNode({
            id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color ?? '#6366f1',
            mode: entry.mode ?? 'generator', sql: entry.sql ?? '', url: entry.url ?? '',
            targetTable: entry.targetTable ?? '', conflictMode: entry.conflictMode ?? 'append',
            interval: entry.interval ?? 0, w: entry.w, h: entry.h, viewMode: entry.viewMode,
          } as Omit<IngestNode, 'kind' | 'color'> & { color?: string })
        } else if (kind === 'exercise' || kind === 'assertion') {
          schemaStore.addExerciseNode({
            id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color ?? '#f59e0b',
            sql: entry.sql ?? '', prompt: entry.prompt ?? '', successText: entry.successText ?? '',
            checks: entry.checks ?? [], revealHintsAfter: entry.revealHintsAfter ?? 0,
            nextId: entry.nextId ?? undefined,
            w: entry.w, h: entry.h,
          } as Omit<ExerciseNode, 'kind' | 'color'> & { color?: string })
        } else if (kind === 'test') {
          schemaStore.addTestNode({
            id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color ?? '#6366f1',
            sql: entry.sql ?? '',
            assertionMode: entry.assertionMode ?? 'no_rows',
            expectedValue: entry.expectedValue,
            operator: entry.operator,
            interval: entry.interval ?? 0,
            history: [],
            w: entry.w, h: entry.h,
          } as Omit<TestNode, 'kind' | 'color'> & { color?: string })
        } else if (kind === 'section') {
          schemaStore.addSection({ id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color, w: entry.w ?? 400, h: entry.h ?? 300 })
        } else if (kind === 'data') {
          const parquetPath = await GetTableDataPath(entry.name)
          if (!parquetPath) continue
          await ImportFromPath(parquetPath, entry.name)
          const safeTable = entry.name.replace(/"/g, '""')
          if (entry.columnCasts && Object.keys(entry.columnCasts).length > 0) {
            for (const [colName, cast] of Object.entries(entry.columnCasts as Record<string, { type: string; expr: string }>)) {
              const safeCol = colName.replace(/"/g, '""')
              const alterSQL = cast.expr
                ? `ALTER TABLE "${safeTable}" ALTER COLUMN "${safeCol}" TYPE ${cast.type} USING (${cast.expr})`
                : `ALTER TABLE "${safeTable}" ALTER COLUMN "${safeCol}" TYPE ${cast.type}`
              await exec(alterSQL).catch((e: unknown) => console.warn(`Cast restore failed for ${colName}:`, e))
            }
          }
          const [columns, countResult] = await Promise.all([
            getTableInfo(entry.name),
            query(`SELECT COUNT(*) AS n FROM "${safeTable}"`),
          ])
          schemaStore.addDataNode({
            id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color,
            columns: columns as Column[], rowCount: Number(countResult.rows[0]?.n ?? 0),
            sourceId: entry.sourceId, sourceSql: entry.sourceSql, columnCasts: entry.columnCasts,
            w: entry.w, h: entry.h, viewMode: entry.viewMode,
          } as Omit<DataNode, 'kind' | 'color'> & { color?: string })
          tablesRestored++
        }
      } catch (e) {
        console.warn(`Failed to restore node "${entry.name ?? entry.id}":`, e)
      }
    }
    return tablesRestored
  }

  async function restoreFromSQLite(raw: string): Promise<boolean> {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let saved: any
    try { saved = JSON.parse(raw) } catch { return false }

    // ── v2 format: { version: 2, activeCanvasId, canvases: [{id, name, nodes}] }
    if (saved && typeof saved === 'object' && !Array.isArray(saved) && saved.version === 2) {
      const tabs: Array<{ id: string; name: string; nodes: unknown[] }> = saved.canvases ?? []
      if (!tabs.length) return false

      schemaStore.loadCanvases(tabs.map(t => ({ id: t.id, name: t.name })), saved.activeCanvasId ?? tabs[0].id)

      let totalNodes = 0
      for (const tab of tabs) {
        schemaStore.switchCanvas(tab.id)
        const entries = Array.isArray(tab.nodes) ? tab.nodes : []
        await restoreNodeEntries(entries)
        totalNodes += entries.length
      }
      schemaStore.switchCanvas(saved.activeCanvasId ?? tabs[0].id)
      schemaStore.setColorCursor(totalNodes)
      return totalNodes > 0
    }

    // ── v1 format: flat array of nodes (legacy)
    if (Array.isArray(saved) && saved.length > 0) {
      schemaStore.clear()
      await restoreNodeEntries(saved)
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

          schemaStore.addTable({ id: entry.id, name: entry.name, x: entry.x, y: entry.y, color: entry.color, columns: cols, columnCasts: entry.columnCasts, w: entry.w, h: entry.h, viewMode: entry.viewMode })
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
      () => schemaStore.canvases,
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
