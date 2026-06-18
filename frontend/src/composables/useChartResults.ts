import { reactive, markRaw } from 'vue'

export interface ChartResult {
  columns: string[]
  rows: Record<string, unknown>[]
  error: string | null
  isRunning: boolean
}

// Module-level — shared between ChartCard (writes after running inline SQL)
// and ChartPropertiesPanel (reads for column lists, also runs SQL).
const _results = reactive<Record<string, ChartResult>>({})

export function useChartResults() {
  return {
    chartResults: _results,
    setChartResult(id: string, r: ChartResult) {
      _results[id] = { ...r, rows: markRaw(r.rows) }
    },
    clearChartResult(id: string) { delete _results[id] },
  }
}
