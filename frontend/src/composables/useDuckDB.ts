import * as duckdb from '@duckdb/duckdb-wasm'
import { ref } from 'vue'
import type { Column } from '../stores/schema'

// Module-level singleton — shared across all composable consumers
let _db: duckdb.AsyncDuckDB | null = null
let _initPromise: Promise<void> | null = null

const isReady = ref(false)
const isLoading = ref(false)
const initError = ref<string | null>(null)

async function init(): Promise<void> {
  if (_db) return
  if (_initPromise) return _initPromise

  _initPromise = (async () => {
    isLoading.value = true
    initError.value = null
    try {
      const bundles = duckdb.getJsDelivrBundles()
      const bundle = await duckdb.selectBundle(bundles)

      const workerUrl = URL.createObjectURL(
        new Blob([`importScripts("${bundle.mainWorker!}");`], { type: 'text/javascript' }),
      )
      const worker = new Worker(workerUrl)
      const logger = new duckdb.ConsoleLogger(duckdb.LogLevel.WARNING)
      _db = new duckdb.AsyncDuckDB(logger, worker)
      await _db.instantiate(bundle.mainModule, bundle.pthreadWorker)
      URL.revokeObjectURL(workerUrl)
      isReady.value = true
      // Pre-load httpfs so remote URLs work in QueryCard and ImportModal without setup.
      loadExtension('httpfs').catch(() => {})
    } catch (e) {
      initError.value = e instanceof Error ? e.message : String(e)
      _initPromise = null
      throw e
    } finally {
      isLoading.value = false
    }
  })()

  return _initPromise
}

export interface QueryResult {
  columns: string[]
  rows: Record<string, unknown>[]
  rowCount: number
  durationMs: number
}

async function query(sql: string): Promise<QueryResult> {
  if (!_db) throw new Error('DuckDB not ready')
  const conn = await _db.connect()
  const start = performance.now()
  try {
    const result = await conn.query(sql)
    const durationMs = performance.now() - start
    const columns = result.schema.fields.map((f) => f.name)
    const rows = result.toArray().map((row) => {
      const obj: Record<string, unknown> = {}
      for (const col of columns) {
        const val = (row as Record<string, unknown>)[col]
        if (val === null || val === undefined) obj[col] = null
        else if (typeof val === 'bigint') obj[col] = Number(val)
        else if (val instanceof Date) obj[col] = val.toISOString()
        else if (typeof val === 'object' && !Array.isArray(val)) obj[col] = JSON.stringify(val)
        else obj[col] = val
      }
      return obj
    })
    return { columns, rows, rowCount: rows.length, durationMs }
  } finally {
    await conn.close()
  }
}

async function exec(sql: string): Promise<void> {
  if (!_db) throw new Error('DuckDB not ready')
  const conn = await _db.connect()
  try {
    await conn.query(sql)
  } finally {
    await conn.close()
  }
}

async function registerFile(name: string, buffer: Uint8Array): Promise<void> {
  if (!_db) throw new Error('DuckDB not ready')
  await _db.registerFileBuffer(name, buffer)
}

async function dropFile(name: string): Promise<void> {
  if (!_db) throw new Error('DuckDB not ready')
  try { await _db.dropFile(name) } catch { /* file may already be gone */ }
}

async function getTableInfo(tableName: string): Promise<Column[]> {
  // PRAGMA table_info returns: cid, name, type, notnull, dflt_value, pk
  const safe = tableName.replace(/'/g, "''")
  const result = await query(`PRAGMA table_info('${safe}')`)
  return result.rows.map((row) => ({
    name: String(row['name']),
    type: String(row['type']),
    primaryKey: Number(row['pk']) === 1,
    nullable: Number(row['notnull']) === 0,
  }))
}

async function copyTableToBuffer(tableName: string): Promise<Uint8Array> {
  if (!_db) throw new Error('DuckDB not ready')
  const safe = tableName.replace(/"/g, '""')
  const fileName = `__export_${tableName}.parquet`
  const conn = await _db.connect()
  try {
    await conn.query(`COPY "${safe}" TO '${fileName}' (FORMAT PARQUET)`)
  } finally {
    await conn.close()
  }
  const buffer = await _db.copyFileToBuffer(fileName)
  await _db.dropFile(fileName)
  return buffer
}

const _loadedExtensions = new Set<string>()

async function loadExtension(name: string): Promise<void> {
  if (_loadedExtensions.has(name)) return
  try {
    await exec(`LOAD '${name}'`)
  } catch {
    await exec(`INSTALL '${name}'`)
    await exec(`LOAD '${name}'`)
  }
  _loadedExtensions.add(name)
}

export function useDuckDB() {
  return {
    isReady, isLoading, initError,
    init, query, exec, getTableInfo,
    registerFile, dropFile,
    copyTableToBuffer, loadExtension,
  }
}
