import { ref } from 'vue'
import type { Column } from '../stores/schema'
import * as GoApp from '../../wailsjs/go/main/App'
import { IS_DESKTOP } from '../lib/env'

const isReady = ref(false)
const isLoading = ref(false)
const initError = ref<string | null>(null)

let _initPromise: Promise<void> | null = null
let _webImpl: typeof import('./useDuckDB.web') | null = null

async function getWebImpl() {
  if (!_webImpl) _webImpl = await import('./useDuckDB.web')
  return _webImpl
}

export interface QueryResult {
  columns: string[]
  columnTypes?: string[]
  rows: Record<string, unknown>[]
  rowCount: number
  durationMs: number
}

async function init(): Promise<void> {
  if (isReady.value) return
  if (_initPromise) return _initPromise

  _initPromise = (async () => {
    isLoading.value = true
    initError.value = null
    try {
      if (IS_DESKTOP) {
        await GoApp.Exec('SELECT 1')
      } else {
        const web = await getWebImpl()
        await web.wasmInit()
      }
      isReady.value = true
    } catch (e) {
      console.error('[sql.garden] DuckDB init failed:', e)
      initError.value = e instanceof Error ? e.message : String(e)
      _initPromise = null
      throw e
    } finally {
      isLoading.value = false
    }
  })()

  return _initPromise
}

async function query(sql: string): Promise<QueryResult> {
  if (IS_DESKTOP) return GoApp.Query(sql) as Promise<QueryResult>
  return (await getWebImpl()).wasmQuery(sql)
}

async function exec(sql: string): Promise<void> {
  if (IS_DESKTOP) return GoApp.Exec(sql)
  return (await getWebImpl()).wasmExec(sql)
}

async function registerFile(name: string, buffer: Uint8Array): Promise<void> {
  if (IS_DESKTOP) return
  return (await getWebImpl()).wasmRegisterFile(name, buffer)
}

async function dropFile(name: string): Promise<void> {
  if (IS_DESKTOP) return
  return (await getWebImpl()).wasmDropFile(name)
}

async function getTableInfo(tableName: string): Promise<Column[]> {
  if (IS_DESKTOP) {
    const cols = await GoApp.GetTableInfo(tableName)
    return cols.map((c) => ({
      name: c.name,
      type: c.type,
      primaryKey: c.primaryKey,
      nullable: c.nullable,
    }))
  }
  return (await getWebImpl()).wasmGetTableInfo(tableName)
}

async function copyTableToBuffer(tableName: string): Promise<Uint8Array> {
  if (!IS_DESKTOP) throw new Error('copyTableToBuffer not supported on web')
  const b64 = await GoApp.CopyTableToParquet(tableName)
  const binary = atob(b64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes
}

async function loadExtension(name: string): Promise<void> {
  if (IS_DESKTOP) return GoApp.LoadExtension(name)
  return (await getWebImpl()).wasmLoadExtension(name)
}

async function importFromPath(filePath: string, tableName: string): Promise<void> {
  if (!IS_DESKTOP) throw new Error('importFromPath not available on web')
  return GoApp.ImportFromPath(filePath, tableName)
}

async function importFromUrl(url: string, tableName: string): Promise<void> {
  if (IS_DESKTOP) return GoApp.ImportFromUrl(url, tableName)
  return (await getWebImpl()).wasmImportFromUrl(url, tableName)
}

async function importTableFromJSON(tableName: string, jsonRows: string): Promise<void> {
  if (IS_DESKTOP) {
    return GoApp.ImportTableFromJSON(tableName, jsonRows)
  }
  // Web (WASM): register as a virtual file and read with DuckDB
  const buf = new TextEncoder().encode(jsonRows)
  await registerFile('_sqg_import.json', buf)
  await exec(`CREATE OR REPLACE TABLE "${tableName}" AS FROM read_json_auto('_sqg_import.json')`)
}

async function importSqliteFromPath(filePath: string, prefix = ''): Promise<string[]> {
  if (!IS_DESKTOP) throw new Error('importSqliteFromPath not available on web')
  return GoApp.ImportSqliteFromPath(filePath, prefix)
}

async function openFileDialog(): Promise<string> {
  if (!IS_DESKTOP) return ''
  return GoApp.OpenFileDialog()
}

// Chunked base64 encoder — avoids stack overflow on files larger than ~100 KB.
export function toBase64(bytes: Uint8Array): string {
  let out = ''
  for (let i = 0; i < bytes.length; i += 8192) {
    out += String.fromCharCode(...bytes.subarray(i, i + 8192))
  }
  return btoa(out)
}

export function useDuckDB() {
  return {
    isReady, isLoading, initError,
    init, query, exec, getTableInfo,
    registerFile, dropFile,
    copyTableToBuffer, loadExtension,
    importFromPath, importFromUrl, importTableFromJSON, importSqliteFromPath, openFileDialog,
  }
}
