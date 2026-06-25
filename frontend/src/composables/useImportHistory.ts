import { ref } from 'vue'

export interface ImportHistoryEntry {
  id: string
  fileName: string
  filePath?: string   // desktop file path — enables re-import
  url?: string        // URL — enables re-import
  tableName: string
  rowCount: number
  importedAt: number
}

const STORAGE_KEY = 'sql.garden:importHistory'
const MAX_ENTRIES = 50

function load(): ImportHistoryEntry[] {
  try { return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]') } catch { return [] }
}

const entries = ref<ImportHistoryEntry[]>(load())

function save() { localStorage.setItem(STORAGE_KEY, JSON.stringify(entries.value)) }

export function useImportHistory() {
  function push(entry: Omit<ImportHistoryEntry, 'id' | 'importedAt'>) {
    const id = `ih_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`
    entries.value = [{ ...entry, id, importedAt: Date.now() }, ...entries.value].slice(0, MAX_ENTRIES)
    save()
  }

  function remove(id: string) {
    entries.value = entries.value.filter((e) => e.id !== id)
    save()
  }

  function clear() { entries.value = []; save() }

  return { entries, push, remove, clear }
}
