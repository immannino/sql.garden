<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import TableCard from './TableCard.vue'
import QueryCard from './QueryCard.vue'
import ChartCard from './ChartCard.vue'
import MarkdownCard from './MarkdownCard.vue'
import { useSchemaStore } from '../stores/schema'
import type { CanvasNode } from '../stores/schema'
import { useSelection } from '../composables/useSelection'

const schemaStore = useSchemaStore()
const { selectedIds, selectNode, clearSelection } = useSelection()

const pan = ref({ x: 100, y: 60 })
const zoom = ref(1)
const isPanning = ref(false)
const panStart = ref({ x: 0, y: 0 })

// ── Drag (multi-node) ─────────────────────────────────────────────────────────
interface DragState {
  startMouse: { x: number; y: number }
  startPositions: Map<string, { x: number; y: number }>
}
const drag = ref<DragState | null>(null)

// ── Resize ────────────────────────────────────────────────────────────────────
interface ResizeState {
  id: string
  startMouse: { x: number; y: number }
  startSize: { w: number; h: number }
  direction: 'e' | 's' | 'se'
}
const resize = ref<ResizeState | null>(null)
const resizeDir = computed(() => resize.value?.direction ?? null)

// ── Marquee ───────────────────────────────────────────────────────────────────
interface MarqueeState {
  startX: number; startY: number
  currentX: number; currentY: number
  additive: boolean
}
const marquee = ref<MarqueeState | null>(null)

const marqueeRect = computed(() => {
  if (!marquee.value) return null
  const { startX, startY, currentX, currentY } = marquee.value
  return {
    x: Math.min(startX, currentX),
    y: Math.min(startY, currentY),
    w: Math.abs(currentX - startX),
    h: Math.abs(currentY - startY),
  }
})

// ── Node bounds helpers ───────────────────────────────────────────────────────
const HEADER_H = 34

function getNodeCanvasBounds(node: CanvasNode): { x: number; y: number; w: number; h: number } {
  if (node.viewMode === 'collapsed') {
    const w = node.kind === 'table' ? (node.w ?? 240)
            : node.kind === 'query' ? (node.w ?? 280)
            : node.kind === 'chart' ? (node.w ?? 340)
            : (node.w ?? 300)
    return { x: node.x, y: node.y, w, h: HEADER_H }
  }
  if (node.kind === 'chart' && node.viewMode === 'chart-only') {
    return { x: node.x, y: node.y, w: node.w ?? 340, h: HEADER_H + (node.h ?? 180) + 24 }
  }
  switch (node.kind) {
    case 'table':    return { x: node.x, y: node.y, w: node.w ?? 240, h: node.h ?? Math.max(60, 34 + node.columns.length * 28 + 8) }
    case 'query':    return { x: node.x, y: node.y, w: node.w ?? 280, h: (node.h ?? 84) + 130 }
    case 'chart':    return { x: node.x, y: node.y, w: node.w ?? 340, h: (node.h ?? 180) + 200 }
    case 'markdown': return { x: node.x, y: node.y, w: node.w ?? 300, h: (node.h ?? 200) + 34 }
  }
}

function nodeViewportBounds(node: CanvasNode) {
  const b = getNodeCanvasBounds(node)
  return { x: b.x * zoom.value + pan.value.x, y: b.y * zoom.value + pan.value.y, w: b.w * zoom.value, h: b.h * zoom.value }
}

function rectsIntersect(ax: number, ay: number, aw: number, ah: number, bx: number, by: number, bw: number, bh: number): boolean {
  return ax < bx + bw && ax + aw > bx && ay < by + bh && ay + ah > by
}

function updateMarqueeSelection() {
  const r = marqueeRect.value
  if (!r || (r.w < 2 && r.h < 2)) return
  const base = marquee.value!.additive ? new Set(selectedIds.value) : new Set<string>()
  for (const node of schemaStore.nodes) {
    const vb = nodeViewportBounds(node)
    if (rectsIntersect(r.x, r.y, r.w, r.h, vb.x, vb.y, vb.w, vb.h)) base.add(node.id)
  }
  selectedIds.value = base
}

// ── Viewport ──────────────────────────────────────────────────────────────────
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
  if (target.closest('.table-card') || target.closest('.query-card') || target.closest('.chart-card') || target.closest('.markdown-card')) return
  e.preventDefault()

  if (e.button === 1) {
    isPanning.value = true
    panStart.value = { x: e.clientX - pan.value.x, y: e.clientY - pan.value.y }
    return
  }

  // Left click on empty canvas → marquee select
  if (!e.shiftKey) clearSelection()
  marquee.value = { startX: e.clientX, startY: e.clientY, currentX: e.clientX, currentY: e.clientY, additive: e.shiftKey }
}

function onCardResizeStart(payload: { id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }) {
  resize.value = {
    id: payload.id,
    startMouse: { x: payload.mouseX, y: payload.mouseY },
    startSize: { w: payload.startW, h: payload.startH },
    direction: payload.direction,
  }
}

function onCardDragStart(payload: { id: string; mouseX: number; mouseY: number; shiftKey: boolean }) {
  const { id, mouseX, mouseY, shiftKey } = payload

  if (shiftKey) {
    selectNode(id, true)
    if (!selectedIds.value.has(id)) return // was removed (toggle-off), don't drag
  } else if (!selectedIds.value.has(id)) {
    selectNode(id, false)
  }

  const startPositions = new Map<string, { x: number; y: number }>()
  for (const selId of selectedIds.value) {
    const node = schemaStore.nodes.find((n) => n.id === selId)
    if (node) startPositions.set(selId, { x: node.x, y: node.y })
  }
  drag.value = { startMouse: { x: mouseX, y: mouseY }, startPositions }
}

function onMouseMove(e: MouseEvent) {
  if (isPanning.value) {
    pan.value = { x: e.clientX - panStart.value.x, y: e.clientY - panStart.value.y }
  } else if (drag.value) {
    const dx = (e.clientX - drag.value.startMouse.x) / zoom.value
    const dy = (e.clientY - drag.value.startMouse.y) / zoom.value
    const positions = new Map<string, { x: number; y: number }>()
    for (const [selId, start] of drag.value.startPositions) {
      positions.set(selId, { x: start.x + dx, y: start.y + dy })
    }
    schemaStore.updatePositions(positions)
  } else if (resize.value) {
    const { id, startMouse, startSize, direction } = resize.value
    const dx = (e.clientX - startMouse.x) / zoom.value
    const dy = (e.clientY - startMouse.y) / zoom.value
    const newW = direction !== 's' ? Math.max(160, startSize.w + dx) : startSize.w
    const newH = direction !== 'e' ? Math.max(60,  startSize.h + dy) : startSize.h
    schemaStore.updateNodeSize(id, newW, newH)
  } else if (marquee.value) {
    marquee.value.currentX = e.clientX
    marquee.value.currentY = e.clientY
    updateMarqueeSelection()
  }
}

function onMouseUp() {
  isPanning.value = false
  drag.value = null
  resize.value = null
  marquee.value = null
}

function onKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape') clearSelection()
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

function focusNode(id: string) {
  const node = schemaStore.nodes.find((n) => n.id === id)
  if (!node || !viewportRef.value) return
  const b = getNodeCanvasBounds(node)
  const vw = viewportRef.value.clientWidth
  const vh = viewportRef.value.clientHeight
  pan.value = {
    x: vw / 2 - (b.x + b.w / 2) * zoom.value,
    y: vh / 2 - (b.y + b.h / 2) * zoom.value,
  }
}

defineExpose({ fitView, getCenter, focusNode, zoom, pan })

onMounted(() => {
  window.addEventListener('mousemove', onMouseMove)
  window.addEventListener('mouseup', onMouseUp)
  window.addEventListener('keydown', onKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('mousemove', onMouseMove)
  window.removeEventListener('mouseup', onMouseUp)
  window.removeEventListener('keydown', onKeyDown)
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
          :selected="selectedIds.has(node.id)"
          @drag-start="onCardDragStart"
          @resize-start="onCardResizeStart"
        />
        <QueryCard
          v-else-if="node.kind === 'query'"
          :node="node"
          :selected="selectedIds.has(node.id)"
          @drag-start="onCardDragStart"
          @resize-start="onCardResizeStart"
        />
        <ChartCard
          v-else-if="node.kind === 'chart'"
          :node="node"
          :selected="selectedIds.has(node.id)"
          @drag-start="onCardDragStart"
          @resize-start="onCardResizeStart"
        />
        <MarkdownCard
          v-else-if="node.kind === 'markdown'"
          :node="node"
          :selected="selectedIds.has(node.id)"
          @drag-start="onCardDragStart"
          @resize-start="onCardResizeStart"
        />
      </template>
    </div>

    <!-- Marquee selection rect (viewport-space) -->
    <div
      v-if="marqueeRect && (marqueeRect.w > 2 || marqueeRect.h > 2)"
      class="marquee-rect"
      :style="{
        left: marqueeRect.x + 'px',
        top: marqueeRect.y + 'px',
        width: marqueeRect.w + 'px',
        height: marqueeRect.h + 'px',
      }"
    />

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

.marquee-rect {
  position: absolute;
  border: 1.5px dashed #6ea6ff;
  background: rgba(110, 166, 255, 0.08);
  border-radius: 2px;
  pointer-events: none;
  user-select: none;
  z-index: 9999;
}
</style>
