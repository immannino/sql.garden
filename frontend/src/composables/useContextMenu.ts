import { ref } from 'vue'

export type ContextMenuTarget =
  | { type: 'node'; id: string; x: number; y: number }
  | { type: 'canvas'; x: number; y: number }

const state = ref<ContextMenuTarget | null>(null)

export function useContextMenu() {
  function openNodeMenu(id: string, x: number, y: number) {
    state.value = { type: 'node', id, x, y }
  }
  function openCanvasMenu(x: number, y: number) {
    state.value = { type: 'canvas', x, y }
  }
  function close() {
    state.value = null
  }
  return { contextMenu: state, openNodeMenu, openCanvasMenu, close }
}
