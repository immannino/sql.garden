import { watch } from 'vue'
import { useSchemaStore } from '../stores/schema'
import type { CanvasNode, ChartNode, MarkdownNode, IngestNode, ExerciseNode, TestNode } from '../stores/schema'

const WEB_CANVAS_KEY = 'sql-garden:canvas:v3'

export function usePersistenceWeb() {
  const schemaStore = useSchemaStore()

  function serializeNode(node: CanvasNode): Record<string, unknown> {
    if (node.kind === 'table' || node.kind === 'data') {
      const { kind, id, name, x, y, color, w, h, viewMode } = node
      return { kind, id, name, x, y, color, w, h, viewMode }
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
    if (node.kind === 'query') {
      const { kind, id, name, x, y, color, sql, isView, w, h, viewMode } = node
      return { kind, id, name, x, y, color, sql, isView, w, h, viewMode }
    }
    if (node.kind === 'markdown') {
      const { kind, id, name, x, y, color, content, w, h, viewMode } = node
      return { kind, id, name, x, y, color, content, w, h, viewMode }
    }
    if (node.kind === 'section') {
      const { kind, id, name, x, y, color, w, h } = node
      return { kind, id, name, x, y, color, w, h }
    }
    const { kind, id, name, x, y, color, sourceId, sql, chartType, xColumn, yColumn, colorColumn, labelColumn,
            chartLabel, trueText, falseText, trueColor, falseColor, conditions, mermaidCode, matrixColumns,
            tableColumnConfigs, w, h, viewMode } = node
    return { kind, id, name, x, y, color, sourceId, sql, chartType, xColumn, yColumn, colorColumn, labelColumn,
             chartLabel, trueText, falseText, trueColor, falseColor, conditions, mermaidCode, matrixColumns,
             tableColumnConfigs, w, h, viewMode }
  }

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
    try {
      localStorage.setItem(WEB_CANVAS_KEY, JSON.stringify(payload))
    } catch { /* storage full or denied — silent */ }
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  function restoreNodeEntries(entries: any[]) {
    for (const entry of entries) {
      const kind: string = entry.kind ?? 'table'
      if (kind === 'table' || kind === 'data') continue // materialized data is gone after reload
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
        }
      } catch (e) {
        console.warn(`Failed to restore node "${entry.name ?? entry.id}":`, e)
      }
    }
  }

  async function loadAll(): Promise<boolean> {
    const raw = localStorage.getItem(WEB_CANVAS_KEY)
    if (!raw) return false

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let saved: any
    try { saved = JSON.parse(raw) } catch { return false }

    // v2 multi-canvas format
    if (saved && typeof saved === 'object' && !Array.isArray(saved) && saved.version === 2) {
      const tabs: Array<{ id: string; name: string; nodes: unknown[] }> = saved.canvases ?? []
      if (!tabs.length) return false
      schemaStore.loadCanvases(tabs.map(t => ({ id: t.id, name: t.name })), saved.activeCanvasId ?? tabs[0].id)
      let total = 0
      for (const tab of tabs) {
        schemaStore.switchCanvas(tab.id)
        const entries = Array.isArray(tab.nodes) ? tab.nodes : []
        restoreNodeEntries(entries)
        total += entries.length
      }
      schemaStore.switchCanvas(saved.activeCanvasId ?? tabs[0].id)
      schemaStore.setColorCursor(total)
      return total > 0
    }

    // Legacy flat-array format
    if (Array.isArray(saved) && saved.length > 0) {
      schemaStore.clear()
      restoreNodeEntries(saved)
      schemaStore.setColorCursor(saved.length)
      return saved.length > 0
    }

    return false
  }

  async function saveTable(_name: string): Promise<void> {}
  async function deleteTable(_name: string): Promise<void> {}

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
