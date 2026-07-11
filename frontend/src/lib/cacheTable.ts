/** Returns the DuckDB table name used to store a query node's cached result. */
export function cacheTableName(nodeId: string): string {
  return '_sqg_cache_' + nodeId.toLowerCase().replace(/[^a-z0-9_]/g, '_')
}
