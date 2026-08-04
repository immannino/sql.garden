import { ref } from 'vue'

// Module-level singleton — QueryPanel sets the id, QueryCard consumes it on mount
const pendingFullscreenId = ref<string | null>(null)

export function usePendingFullscreen() {
  return { pendingFullscreenId }
}
