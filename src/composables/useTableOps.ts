import { useDuckDB } from './useDuckDB'
import { useSchemaStore } from '../stores/schema'

export function useTableOps() {
  const { exec } = useDuckDB()
  const schemaStore = useSchemaStore()

  async function dropTable(id: string, name: string): Promise<void> {
    const safe = name.replace(/"/g, '""')
    // Optimistic removal from canvas first; if DuckDB errors the table is
    // likely already gone or managed externally — either way the card should go.
    schemaStore.removeTable(id)
    try {
      await exec(`DROP TABLE IF EXISTS "${safe}"`)
    } catch (e) {
      console.warn('DuckDB DROP TABLE failed (card removed from canvas anyway):', e)
    }
  }

  return { dropTable }
}
