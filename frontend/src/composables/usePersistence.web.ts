import { watch } from 'vue'
import { useSchemaStore } from '../stores/schema'
import type { ChartNode, MarkdownNode } from '../stores/schema'

const WEB_CANVAS_KEY = 'sql-garden:canvas:v2'

export function usePersistenceWeb() {
  const schemaStore = useSchemaStore()

  function saveCanvas(): void {
    const payload = schemaStore.nodes.map((node) => {
      if (node.kind === 'table' || node.kind === 'data') {
        const { kind, id, name, x, y, color, w, h, viewMode } = node
        return { kind, id, name, x, y, color, w, h, viewMode }
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
      const { kind, id, name, x, y, color, sourceId, sql, chartType, xColumn, yColumn, colorColumn, labelColumn, w, h, viewMode } = node
      return { kind, id, name, x, y, color, sourceId, sql, chartType, xColumn, yColumn, colorColumn, labelColumn, w, h, viewMode }
    })
    try {
      localStorage.setItem(WEB_CANVAS_KEY, JSON.stringify(payload))
    } catch { /* storage full or denied — silent */ }
  }

  async function loadAll(): Promise<boolean> {
    const raw = localStorage.getItem(WEB_CANVAS_KEY)
    if (!raw) return false

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let saved: any[]
    try { saved = JSON.parse(raw) } catch { return false }
    if (!saved.length) return false

    schemaStore.clear()

    for (const entry of saved) {
      const kind: string = entry.kind ?? 'table'
      if (kind === 'table' || kind === 'data') continue  // materialized data is gone after page reload; skip

      try {
        if (kind === 'query') {
          schemaStore.addQueryNode({ id: entry.id, name: entry.name, x: entry.x, y: entry.y, sql: entry.sql ?? '', color: entry.color, isView: entry.isView ?? false, w: entry.w, h: entry.h, viewMode: entry.viewMode })
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

    const nonTableCount = saved.filter((e) => (e.kind ?? 'table') !== 'table' && e.kind !== 'data').length
    if (nonTableCount > 0 || saved.length > 0) {
      schemaStore.setColorCursor(saved.length)
      return true
    }
    return false
  }

  async function saveTable(_name: string): Promise<void> {}
  async function deleteTable(_name: string): Promise<void> {}

  let _saveTimer: ReturnType<typeof setTimeout> | null = null

  function startAutoSave(): void {
    watch(
      schemaStore.nodes,
      () => {
        if (_saveTimer) clearTimeout(_saveTimer)
        _saveTimer = setTimeout(() => { saveCanvas() }, 1000)
      },
      { deep: true },
    )
  }

  return { loadAll, saveCanvas, saveTable, deleteTable, startAutoSave }
}
