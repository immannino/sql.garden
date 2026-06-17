import type { QueryResult } from './useDuckDB'
import type { Column } from '../stores/schema'

// Desktop stub — these are never called (IS_DESKTOP=true eliminates the web branch).
export const wasmInit = async (): Promise<void> => {}
export const wasmQuery = async (_sql: string): Promise<QueryResult> => ({ columns: [], rows: [], rowCount: 0, durationMs: 0 })
export const wasmExec = async (_sql: string): Promise<void> => {}
export const wasmGetTableInfo = async (_tableName: string): Promise<Column[]> => []
export const wasmRegisterFile = async (_name: string, _buf: Uint8Array): Promise<void> => {}
export const wasmDropFile = async (_name: string): Promise<void> => {}
export const wasmImportFromUrl = async (_url: string, _tableName: string): Promise<void> => {}
export const wasmLoadExtension = async (_name: string): Promise<void> => {}
