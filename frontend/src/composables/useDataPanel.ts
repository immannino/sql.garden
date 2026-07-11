import { ref } from 'vue'

const _panelId = ref<string | null>(null)

export function useDataPanel() {
  return {
    panelNodeId: _panelId,
    openPanel(id: string) { _panelId.value = id },
    closePanel() { _panelId.value = null },
  }
}
