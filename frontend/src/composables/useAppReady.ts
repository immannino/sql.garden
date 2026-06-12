import { ref } from 'vue'

// Module-level — one signal shared across all components.
// App.vue sets this after init + loadAll + seed completes.
// Cards wait on it before auto-running their queries.
const _ready = ref(false)

export function useAppReady() {
  return {
    isAppReady: _ready,
    markAppReady() { _ready.value = true },
  }
}
