import { ref } from 'vue'

const selectedIds = ref<Set<string>>(new Set())

export function useSelection() {
  function selectNode(id: string, additive: boolean) {
    if (additive) {
      const next = new Set(selectedIds.value)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      selectedIds.value = next
    } else {
      selectedIds.value = new Set([id])
    }
  }

  function clearSelection() {
    selectedIds.value = new Set()
  }

  return { selectedIds, selectNode, clearSelection }
}
