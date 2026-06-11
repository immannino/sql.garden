import { reactive } from 'vue'

export interface NodeQueryResult {
  columns: string[]
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
    setResult(id: string, r: NodeQueryResult) { _results[id] = r },
    clearResult(id: string) { delete _results[id] },
  }
}
