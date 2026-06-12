import { ref } from 'vue'

// Module-level singleton so TableCard and QueryPanel share the same channel
const pendingQuery = ref<string | null>(null)

export function useQueryBridge() {
  return {
    pendingQuery,
    sendQuery(sql: string) {
      pendingQuery.value = sql
    },
  }
}
