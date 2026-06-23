import { reactive, markRaw } from 'vue'

export interface NodeQueryResult {
  columns: string[]
  columnTypes?: string[]
  rows: Record<string, unknown>[]
  error: string | null
  isRunning: boolean
}

// Module-level — shared across all composable consumers so ChartCards can
// reactively consume the latest output of any QueryCard.
const _results = reactive<Record<string, NodeQueryResult>>({})

export function useQueryResults() {
  return {
    results: _results,
    setResult(id: string, r: NodeQueryResult) {
      // markRaw prevents Vue from deep-proxying every row object — rows are
      // read-only display data and don't need per-property change tracking.
      // Guard against Go returning null for empty result sets.
      _results[id] = { ...r, rows: markRaw(r.rows ?? []), columns: r.columns ?? [] }
    },
    clearResult(id: string) { delete _results[id] },
  }
}
