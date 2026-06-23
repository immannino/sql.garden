<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useSchemaStore } from '../stores/schema'
import { useSelection } from '../composables/useSelection'

const emit = defineEmits<{ close: []; select: [id: string] }>()

const schemaStore = useSchemaStore()
const { selectNode } = useSelection()

const query = ref('')
const activeIdx = ref(0)
const inputRef = ref<HTMLInputElement | null>(null)
const listRef = ref<HTMLElement | null>(null)

const KIND_LABEL: Record<string, string> = {
  table: 'Table',
  query: 'Query',
  chart: 'Chart',
  markdown: 'Note',
  section: 'Section',
}

const results = computed(() => {
  const q = query.value.trim().toLowerCase()
  const nodes = [...schemaStore.nodes].reverse() // front-to-back (most visible first)
  if (!q) return nodes.slice(0, 24)
  return nodes.filter((n) => n.name.toLowerCase().includes(q)).slice(0, 24)
})

function select(id: string) {
  selectNode(id, false)
  emit('select', id)
  emit('close')
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') { emit('close'); return }
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    activeIdx.value = Math.min(activeIdx.value + 1, results.value.length - 1)
    scrollActive()
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    activeIdx.value = Math.max(activeIdx.value - 1, 0)
    scrollActive()
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const node = results.value[activeIdx.value]
    if (node) select(node.id)
  }
}

function onQueryInput() {
  activeIdx.value = 0
}

function scrollActive() {
  nextTick(() => {
    const item = listRef.value?.querySelector('[data-active="true"]') as HTMLElement | null
    item?.scrollIntoView({ block: 'nearest' })
  })
}

function onBackdropClick(e: MouseEvent) {
  if (e.target === e.currentTarget) emit('close')
}

onMounted(() => {
  nextTick(() => inputRef.value?.focus())
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div class="palette-backdrop" @mousedown="onBackdropClick">
    <div class="palette">
      <!-- Search input -->
      <div class="palette-search">
        <svg class="search-icon" viewBox="0 0 16 16" fill="none">
          <circle cx="6.5" cy="6.5" r="4.5" stroke="currentColor" stroke-width="1.4"/>
          <line x1="10" y1="10" x2="14" y2="14" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
        </svg>
        <input
          ref="inputRef"
          v-model="query"
          class="palette-input"
          placeholder="Jump to node…"
          spellcheck="false"
          @input="onQueryInput"
        />
        <kbd class="esc-hint">esc</kbd>
      </div>

      <!-- Results -->
      <div ref="listRef" class="palette-list">
        <button
          v-for="(node, i) in results"
          :key="node.id"
          class="palette-item"
          :class="{ active: i === activeIdx }"
          :data-active="i === activeIdx"
          @mouseenter="activeIdx = i"
          @mousedown.prevent="select(node.id)"
        >
          <!-- Kind icon -->
          <svg v-if="node.kind === 'table'" class="node-icon" viewBox="0 0 14 14" fill="none">
            <rect x="1" y="1" width="12" height="12" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
            <line x1="1" y1="4.5" x2="13" y2="4.5" stroke="currentColor" stroke-width="1.2"/>
            <line x1="4.5" y1="4.5" x2="4.5" y2="13" stroke="currentColor" stroke-width="1.2"/>
          </svg>
          <svg v-else-if="node.kind === 'query'" class="node-icon" viewBox="0 0 14 14" fill="none">
            <polyline points="2,4 5,7 2,10" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
            <line x1="7" y1="10" x2="12" y2="10" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
          </svg>
          <svg v-else-if="node.kind === 'chart'" class="node-icon" viewBox="0 0 14 14" fill="none">
            <rect x="1" y="1" width="12" height="12" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
            <polyline points="3,9 5,5 8,7 11,3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          <svg v-else-if="node.kind === 'section'" class="node-icon" viewBox="0 0 14 14" fill="none">
            <rect x="1" y="1" width="12" height="12" rx="1.5" stroke="currentColor" stroke-width="1.2" stroke-dasharray="2 1.5"/>
          </svg>
          <svg v-else class="node-icon" viewBox="0 0 14 14" fill="none">
            <rect x="1" y="2" width="12" height="10" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
            <line x1="3.5" y1="5.5" x2="10.5" y2="5.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
            <line x1="3.5" y1="8" x2="7.5" y2="8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
          </svg>

          <span class="node-name">{{ node.name }}</span>

          <span class="kind-chip" :class="`kind-${node.kind}`">
            {{ KIND_LABEL[node.kind] ?? node.kind }}
          </span>

          <svg class="enter-hint" viewBox="0 0 12 12" fill="none">
            <path d="M10 3v3a2 2 0 0 1-2 2H2M2 8l2-2M2 8l2 2" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </button>

        <div v-if="results.length === 0" class="palette-empty">
          No nodes match "{{ query }}"
        </div>
      </div>

      <div class="palette-footer">
        <span><kbd>↑↓</kbd> navigate</span>
        <span><kbd>↵</kbd> jump to</span>
        <span><kbd>esc</kbd> dismiss</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.palette-backdrop {
  position: fixed;
  inset: 0;
  z-index: 9000;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 18vh;
}

.palette {
  width: 540px;
  max-width: calc(100vw - 40px);
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 10px;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.7);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

/* ── Search bar ── */
.palette-search {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border-bottom: 1px solid #30363d;
}

.search-icon {
  width: 16px;
  height: 16px;
  color: #6e7681;
  flex-shrink: 0;
}

.palette-input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  font-size: 14px;
  color: #e6edf3;
  font-family: var(--font-sans, sans-serif);
}

.palette-input::placeholder { color: #6e7681; }

.esc-hint {
  font-size: 10px;
  color: #6e7681;
  background: #21262d;
  border: 1px solid #30363d;
  border-radius: 4px;
  padding: 2px 5px;
  font-family: var(--font-sans, sans-serif);
}

/* ── Results list ── */
.palette-list {
  max-height: 340px;
  overflow-y: auto;
  padding: 4px 0;
}

.palette-item {
  display: flex;
  align-items: center;
  gap: 9px;
  width: 100%;
  padding: 8px 14px;
  background: transparent;
  border: none;
  text-align: left;
  cursor: pointer;
  transition: background 0.08s;
}

.palette-item.active { background: rgba(88, 166, 255, 0.12); }

.node-icon {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  color: #6e7681;
}
.palette-item.active .node-icon { color: #58a6ff; }

.node-name {
  flex: 1;
  font-size: 13px;
  font-family: var(--font-mono, monospace);
  color: #e6edf3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.kind-chip {
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.04em;
  padding: 2px 6px;
  border-radius: 3px;
  flex-shrink: 0;
  background: #21262d;
  color: #8b949e;
}
.kind-table   { background: rgba(59,130,246,0.12); color: #60a5fa; }
.kind-query   { background: rgba(16,185,129,0.12); color: #34d399; }
.kind-chart   { background: rgba(139,92,246,0.12); color: #a78bfa; }
.kind-markdown { background: rgba(245,158,11,0.12); color: #fbbf24; }
.kind-section { background: rgba(107,114,128,0.12); color: #9ca3af; }

.enter-hint {
  width: 12px;
  height: 12px;
  color: #6e7681;
  opacity: 0;
  flex-shrink: 0;
  transition: opacity 0.08s;
}
.palette-item.active .enter-hint { opacity: 1; }

.palette-empty {
  padding: 24px 16px;
  text-align: center;
  font-size: 13px;
  color: #6e7681;
  font-style: italic;
}

/* ── Footer ── */
.palette-footer {
  display: flex;
  gap: 16px;
  padding: 8px 14px;
  border-top: 1px solid #30363d;
  font-size: 11px;
  color: #6e7681;
}

.palette-footer kbd {
  font-size: 10px;
  background: #21262d;
  border: 1px solid #30363d;
  border-radius: 3px;
  padding: 1px 4px;
  font-family: var(--font-sans, sans-serif);
  margin-right: 3px;
}
</style>
