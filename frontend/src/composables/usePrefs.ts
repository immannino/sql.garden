import { ref, watch } from 'vue'

const showGrid = ref<boolean>(localStorage.getItem('pref:showGrid') !== 'false')

watch(showGrid, (v) => localStorage.setItem('pref:showGrid', String(v)))

export function usePrefs() {
  return { showGrid }
}
