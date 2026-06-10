<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import TableCard from './TableCard.vue'
import { useSchemaStore } from '../stores/schema'

const schemaStore = useSchemaStore()

const pan = ref({ x: 100, y: 60 })
const zoom = ref(1)
const isPanning = ref(false)
const panStart = ref({ x: 0, y: 0 })
const selectedId = ref<string | null>(null)

interface DragState {
  tableId: string
  startMouse: { x: number; y: number }
  startPos: { x: number; y: number }
}
const drag = ref<DragState | null>(null)

const viewportRef = ref<HTMLElement | null>(null)

const BASE_GRID = 24

// v-bind in <style> requires these to be plain strings
const gridCellSize = computed(() => `${BASE_GRID * zoom.value}px`)
const bgOffset = computed(() => `${pan.value.x}px ${pan.value.y}px`)

const transformStyle = computed(() => ({
  transform: `translate(${pan.value.x}px, ${pan.value.y}px) scale(${zoom.value})`,
  transformOrigin: '0 0',
}))

const zoomPct = computed(() => Math.round(zoom.value * 100))

function onWheel(e: WheelEvent) {
  e.preventDefault()
  const delta = e.ctrlKey ? e.deltaY * 0.01 : e.deltaY * 0.001
  const factor = Math.exp(-delta)
  const newZoom = Math.min(4, Math.max(0.08, zoom.value * factor))
  const rect = viewportRef.value!.getBoundingClientRect()
  const cx = e.clientX - rect.left
  const cy = e.clientY - rect.top
  pan.value = {
    x: cx - (cx - pan.value.x) * (newZoom / zoom.value),
    y: cy - (cy - pan.value.y) * (newZoom / zoom.value),
  }
  zoom.value = newZoom
}

function onMouseDown(e: MouseEvent) {
  if (e.button !== 0 && e.button !== 1) return
  const target = e.target as HTMLElement
  if (target.closest('.table-card')) return
  selectedId.value = null
  isPanning.value = true
  panStart.value = { x: e.clientX - pan.value.x, y: e.clientY - pan.value.y }
  e.preventDefault()
}

function onCardDragStart(payload: { tableId: string; mouseX: number; mouseY: number }) {
  const table = schemaStore.tables.find((t) => t.id === payload.tableId)
  if (!table) return
  selectedId.value = payload.tableId
  drag.value = {
    tableId: payload.tableId,
    startMouse: { x: payload.mouseX, y: payload.mouseY },
    startPos: { x: table.x, y: table.y },
  }
}

function onMouseMove(e: MouseEvent) {
  if (isPanning.value) {
    pan.value = {
      x: e.clientX - panStart.value.x,
      y: e.clientY - panStart.value.y,
    }
  } else if (drag.value) {
    const dx = (e.clientX - drag.value.startMouse.x) / zoom.value
    const dy = (e.clientY - drag.value.startMouse.y) / zoom.value
    schemaStore.updatePosition(
      drag.value.tableId,
      drag.value.startPos.x + dx,
      drag.value.startPos.y + dy,
    )
  }
}

function onMouseUp() {
  isPanning.value = false
  drag.value = null
}

function fitView() {
  if (!schemaStore.tables.length || !viewportRef.value) return
  const padding = 80
  const vw = viewportRef.value.clientWidth
  const vh = viewportRef.value.clientHeight
  const xs = schemaStore.tables.map((t) => t.x)
  const ys = schemaStore.tables.map((t) => t.y)
  const minX = Math.min(...xs)
  const minY = Math.min(...ys)
  const maxX = Math.max(...xs) + 240
  const maxY = Math.max(...ys) + 200
  const contentW = maxX - minX
  const contentH = maxY - minY
  const z = Math.min(4, Math.max(0.08, Math.min((vw - padding * 2) / contentW, (vh - padding * 2) / contentH)))
  zoom.value = z
  pan.value = {
    x: (vw - contentW * z) / 2 - minX * z,
    y: (vh - contentH * z) / 2 - minY * z,
  }
}

defineExpose({ fitView, zoom, pan })

onMounted(() => {
  window.addEventListener('mousemove', onMouseMove)
  window.addEventListener('mouseup', onMouseUp)
})

onUnmounted(() => {
  window.removeEventListener('mousemove', onMouseMove)
  window.removeEventListener('mouseup', onMouseUp)
})
</script>

<template>
  <div
    ref="viewportRef"
    class="canvas-viewport"
    :class="{ panning: isPanning }"
    @wheel.prevent="onWheel"
    @mousedown="onMouseDown"
  >
    <!-- Grid background uses v-bind CSS to move with pan/zoom -->
    <div class="canvas-grid" />

    <!-- All table nodes live in this transform layer -->
    <div class="canvas-layer" :style="transformStyle">
      <TableCard
        v-for="table in schemaStore.tables"
        :key="table.id"
        :table="table"
        :selected="selectedId === table.id"
        @drag-start="onCardDragStart"
      />
    </div>

    <!-- Zoom badge -->
    <div class="zoom-badge">{{ zoomPct }}%</div>
  </div>
</template>

<style scoped>
.canvas-viewport {
  position: relative;
  flex: 1;
  overflow: hidden;
  cursor: default;
}

.canvas-viewport.panning {
  cursor: grabbing;
}

.canvas-grid {
  position: absolute;
  inset: 0;
  background-color: var(--canvas-bg);
  background-image: radial-gradient(circle, #2d3748 1.5px, transparent 1.5px);
  background-size: v-bind(gridCellSize) v-bind(gridCellSize);
  background-position: v-bind(bgOffset);
  pointer-events: none;
}

.canvas-layer {
  position: absolute;
  top: 0;
  left: 0;
  width: 0;
  height: 0;
  overflow: visible;
}

.zoom-badge {
  position: absolute;
  bottom: 16px;
  right: 16px;
  font-size: 11px;
  font-family: var(--font-mono);
  color: var(--text-muted);
  background: var(--surface-1);
  border: 1px solid var(--border);
  padding: 3px 8px;
  border-radius: 4px;
  pointer-events: none;
  user-select: none;
}
</style>
