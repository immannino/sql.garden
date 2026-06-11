<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import TableCard from './TableCard.vue'
import QueryCard from './QueryCard.vue'
import ChartCard from './ChartCard.vue'
import MarkdownCard from './MarkdownCard.vue'
import { useSchemaStore } from '../stores/schema'

const schemaStore = useSchemaStore()

const pan = ref({ x: 100, y: 60 })
const zoom = ref(1)
const isPanning = ref(false)
const panStart = ref({ x: 0, y: 0 })
const selectedId = ref<string | null>(null)

interface DragState {
  id: string
  startMouse: { x: number; y: number }
  startPos: { x: number; y: number }
}
const drag = ref<DragState | null>(null)

interface ResizeState {
  id: string
  startMouse: { x: number; y: number }
  startSize: { w: number; h: number }
  direction: 'e' | 's' | 'se'
}
const resize = ref<ResizeState | null>(null)
const resizeDir = computed(() => resize.value?.direction ?? null)

const viewportRef = ref<HTMLElement | null>(null)

const BASE_GRID = 24
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
  if (target.closest('.table-card') || target.closest('.query-card') || target.closest('.chart-card')) return
  selectedId.value = null
  isPanning.value = true
  panStart.value = { x: e.clientX - pan.value.x, y: e.clientY - pan.value.y }
  e.preventDefault()
}

function onCardResizeStart(payload: { id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }) {
  resize.value = {
    id: payload.id,
    startMouse: { x: payload.mouseX, y: payload.mouseY },
    startSize: { w: payload.startW, h: payload.startH },
    direction: payload.direction,
  }
}

function onCardDragStart(payload: { id: string; mouseX: number; mouseY: number }) {
  const node = schemaStore.nodes.find((n) => n.id === payload.id)
  if (!node) return
  selectedId.value = payload.id
  drag.value = {
    id: payload.id,
    startMouse: { x: payload.mouseX, y: payload.mouseY },
    startPos: { x: node.x, y: node.y },
  }
}

function onMouseMove(e: MouseEvent) {
  if (isPanning.value) {
    pan.value = { x: e.clientX - panStart.value.x, y: e.clientY - panStart.value.y }
  } else if (drag.value) {
    const dx = (e.clientX - drag.value.startMouse.x) / zoom.value
    const dy = (e.clientY - drag.value.startMouse.y) / zoom.value
    schemaStore.updatePosition(drag.value.id, drag.value.startPos.x + dx, drag.value.startPos.y + dy)
  } else if (resize.value) {
    const { id, startMouse, startSize, direction } = resize.value
    const dx = (e.clientX - startMouse.x) / zoom.value
    const dy = (e.clientY - startMouse.y) / zoom.value
    const newW = direction !== 's' ? Math.max(160, startSize.w + dx) : startSize.w
    const newH = direction !== 'e' ? Math.max(60,  startSize.h + dy) : startSize.h
    schemaStore.updateNodeSize(id, newW, newH)
  }
}

function onMouseUp() {
  isPanning.value = false
  drag.value = null
  resize.value = null
}

function fitView() {
  if (!schemaStore.nodes.length || !viewportRef.value) return
  const padding = 80
  const vw = viewportRef.value.clientWidth
  const vh = viewportRef.value.clientHeight
  const xs = schemaStore.nodes.map((n) => n.x)
  const ys = schemaStore.nodes.map((n) => n.y)
  const minX = Math.min(...xs)
  const minY = Math.min(...ys)
  const maxX = Math.max(...xs) + 340
  const maxY = Math.max(...ys) + 260
  const contentW = maxX - minX
  const contentH = maxY - minY
  const z = Math.min(4, Math.max(0.08, Math.min((vw - padding * 2) / contentW, (vh - padding * 2) / contentH)))
  zoom.value = z
  pan.value = {
    x: (vw - contentW * z) / 2 - minX * z,
    y: (vh - contentH * z) / 2 - minY * z,
  }
}

function getCenter(): { x: number; y: number } {
  if (!viewportRef.value) return { x: 200, y: 200 }
  const vw = viewportRef.value.clientWidth
  const vh = viewportRef.value.clientHeight
  return {
    x: (vw / 2 - pan.value.x) / zoom.value,
    y: (vh / 2 - pan.value.y) / zoom.value,
  }
}

defineExpose({ fitView, getCenter, zoom, pan })

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
    :class="{ panning: isPanning, 'resize-e': resizeDir === 'e', 'resize-s': resizeDir === 's', 'resize-se': resizeDir === 'se' }"
    @wheel.prevent="onWheel"
    @mousedown="onMouseDown"
  >
    <div class="canvas-grid" />

    <div class="canvas-layer" :style="transformStyle">
      <template v-for="node in schemaStore.nodes" :key="node.id">
        <TableCard
          v-if="node.kind === 'table'"
          :table="node"
          :selected="selectedId === node.id"
          @drag-start="onCardDragStart"
          @resize-start="onCardResizeStart"
        />
        <QueryCard
          v-else-if="node.kind === 'query'"
          :node="node"
          :selected="selectedId === node.id"
          @drag-start="onCardDragStart"
          @resize-start="onCardResizeStart"
        />
        <ChartCard
          v-else-if="node.kind === 'chart'"
          :node="node"
          :selected="selectedId === node.id"
          @drag-start="onCardDragStart"
          @resize-start="onCardResizeStart"
        />
        <MarkdownCard
          v-else-if="node.kind === 'markdown'"
          :node="node"
          :selected="selectedId === node.id"
          @drag-start="onCardDragStart"
          @resize-start="onCardResizeStart"
        />
      </template>
    </div>

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

.canvas-viewport.panning    { cursor: grabbing; }
.canvas-viewport.resize-e   { cursor: ew-resize; }
.canvas-viewport.resize-s   { cursor: ns-resize; }
.canvas-viewport.resize-se  { cursor: se-resize; }

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
