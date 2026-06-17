import * as duckdb from '@duckdb/duckdb-wasm'
import duckdb_mvp_wasm from '@duckdb/duckdb-wasm/dist/duckdb-mvp.wasm?url'
import duckdb_eh_wasm from '@duckdb/duckdb-wasm/dist/duckdb-eh.wasm?url'
import MvpWorker from '@duckdb/duckdb-wasm/dist/duckdb-browser-mvp.worker.js?worker'
import EhWorker from '@duckdb/duckdb-wasm/dist/duckdb-browser-eh.worker.js?worker'
import type { Column } from '../stores/schema'
import type { QueryResult } from './useDuckDB'

let _db: duckdb.AsyncDuckDB | null = null
let _initPromise: Promise<void> | null = null

async function ensureDB(): Promise<duckdb.AsyncDuckDB> {
  if (_db) return _db
  if (_initPromise) { await _initPromise; return _db! }

  _initPromise = (async () => {
    // crossOriginIsolated is true when COOP/COEP headers are present (GitHub Pages
    // doesn't support them, so we fall back to the single-threaded MVP bundle).
    const useEH = !!crossOriginIsolated
    const worker = useEH ? new EhWorker() : new MvpWorker()
    const wasmUrl = useEH ? duckdb_eh_wasm : duckdb_mvp_wasm
    const db = new duckdb.AsyncDuckDB(new duckdb.VoidLogger(), worker)
    await db.instantiate(wasmUrl)
    _db = db
  })()

  await _initPromise
  return _db!
}

function coerce(v: unknown): unknown {
  if (typeof v === 'bigint') return Number(v)
  return v
}

export async function wasmInit(): Promise<void> {
  await ensureDB()
}

export async function wasmQuery(sql: string): Promise<QueryResult> {
  const db = await ensureDB()
  const t0 = performance.now()
  const conn = await db.connect()
  try {
    const table = await conn.query(sql)
    const columns = table.schema.fields.map((f) => f.name)
    const rows: Record<string, unknown>[] = table.toArray().map((row) => {
      const obj: Record<string, unknown> = {}
      for (const col of columns) obj[col] = coerce(row[col])
      return obj
    })
    return { columns, rows, rowCount: rows.length, durationMs: performance.now() - t0 }
  } finally {
    await conn.close()
  }
}

export async function wasmExec(sql: string): Promise<void> {
  const db = await ensureDB()
  const conn = await db.connect()
  try { await conn.query(sql) } finally { await conn.close() }
}

export async function wasmGetTableInfo(tableName: string): Promise<Column[]> {
  const safe = tableName.replace(/'/g, "''")
  const result = await wasmQuery(`PRAGMA table_info('${safe}')`)
  return result.rows.map((row) => ({
    name: String(row['name'] ?? ''),
    type: String(row['type'] ?? ''),
    primaryKey: row['pk'] === 1 || row['pk'] === true,
    nullable: row['notnull'] === 0 || row['notnull'] === false,
  }))
}

export async function wasmRegisterFile(name: string, buffer: Uint8Array): Promise<void> {
  const db = await ensureDB()
  await db.registerFileBuffer(name, buffer)
}

export async function wasmDropFile(name: string): Promise<void> {
  const db = await ensureDB()
  await db.dropFile(name)
}

export async function wasmImportFromUrl(url: string, tableName: string): Promise<void> {
  const safe = tableName.replace(/"/g, '""')
  const ext = url.split('?')[0].split('.').pop()?.toLowerCase() ?? 'csv'

  if (/^(s3|gcs|r2|hf):\/\//i.test(url)) {
    const clean = url.replace(/'/g, "''")
    const readFn = ext === 'parquet' ? `read_parquet('${clean}')` :
                   (ext === 'json' || ext === 'jsonl') ? `read_json_auto('${clean}')` :
                   `read_csv_auto('${clean}', header=true)`
    await wasmExec(`LOAD httpfs`)
    await wasmExec(`CREATE TABLE "${safe}" AS SELECT * FROM ${readFn}`)
    return
  }

  // HTTP/HTTPS — fetch and register to sidestep CORS issues with DuckDB httpfs
  const resp = await fetch(url)
  if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${url}`)
  const buffer = new Uint8Array(await resp.arrayBuffer())
  const fname = `__imp_${Date.now()}.${ext}`
  await wasmRegisterFile(fname, buffer)
  try {
    const readFn = ext === 'parquet' ? `read_parquet('${fname}')` :
                   (ext === 'json' || ext === 'jsonl') ? `read_json_auto('${fname}')` :
                   `read_csv_auto('${fname}', header=true, sample_size=-1)`
    await wasmExec(`CREATE TABLE "${safe}" AS SELECT * FROM ${readFn}`)
  } finally {
    await wasmDropFile(fname)
  }
}

export async function wasmLoadExtension(name: string): Promise<void> {
  await wasmExec(`LOAD ${name}`)
}
