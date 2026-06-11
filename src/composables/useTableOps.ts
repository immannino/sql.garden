import { useDuckDB } from './useDuckDB'
import { useSchemaStore } from '../stores/schema'
import { usePersistence } from './usePersistence'

export function useTableOps() {
  const { exec } = useDuckDB()
  const schemaStore = useSchemaStore()
  const { deleteTable, saveTable } = usePersistence()

  async function dropTable(id: string, name: string): Promise<void> {
    const safe = name.replace(/"/g, '""')
    schemaStore.removeNode(id)
    try {
      await exec(`DROP TABLE IF EXISTS "${safe}"`)
    } catch (e) {
      console.warn('DuckDB DROP TABLE failed (card removed from canvas anyway):', e)
    }
    await deleteTable(name).catch(console.warn)
  }

  async function renameTable(id: string, oldName: string, newName: string): Promise<void> {
    const safeOld = oldName.replace(/"/g, '""')
    const safeNew = newName.replace(/"/g, '""')
    await exec(`ALTER TABLE "${safeOld}" RENAME TO "${safeNew}"`)
    schemaStore.renameNode(id, newName)
    await deleteTable(oldName).catch(console.warn)
    await saveTable(newName).catch(console.warn)
  }

  return { dropTable, renameTable }
}
