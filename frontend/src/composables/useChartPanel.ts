import { ref } from 'vue'

// Module-level — single panel open at a time.
const _panelId = ref<string | null>(null)

export function useChartPanel() {
  return {
    panelChartId: _panelId,
    openPanel(id: string) { _panelId.value = id },
    closePanel() { _panelId.value = null },
  }
}
