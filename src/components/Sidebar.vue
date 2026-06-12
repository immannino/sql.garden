<script setup lang="ts">
import { computed } from 'vue'
import { useSchemaStore } from '../stores/schema'
import { useSelection } from '../composables/useSelection'
import type { CanvasNode } from '../stores/schema'

const emit = defineEmits<{
  focusNode: [id: string]
}>()

const schemaStore = useSchemaStore()
const { selectedIds, selectNode } = useSelection()

const groups = computed(() => [
  { label: 'Tables',  kind: 'table',    nodes: schemaStore.nodes.filter((n) => n.kind === 'table')    },
  { label: 'Queries', kind: 'query',    nodes: schemaStore.nodes.filter((n) => n.kind === 'query')    },
  { label: 'Charts',  kind: 'chart',    nodes: schemaStore.nodes.filter((n) => n.kind === 'chart')    },
  { label: 'Notes',   kind: 'markdown', nodes: schemaStore.nodes.filter((n) => n.kind === 'markdown') },
] as const)

function onItemClick(e: MouseEvent, node: CanvasNode) {
  selectNode(node.id, e.shiftKey)
  if (!e.shiftKey) emit('focusNode', node.id)
}
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-header">Layers</div>

    <div class="sidebar-body">
      <template v-for="group in groups" :key="group.kind">
        <div v-if="group.nodes.length" class="group">
          <div class="group-label">{{ group.label }}</div>
          <button
            v-for="node in group.nodes"
            :key="node.id"
            class="node-item"
            :class="{ selected: selectedIds.has(node.id) }"
            @click="onItemClick($event, node)"
          >
            <!-- Table icon -->
            <svg v-if="node.kind === 'table'" class="node-icon" viewBox="0 0 14 14" fill="none">
              <rect x="1" y="1" width="12" height="12" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
              <line x1="1" y1="4.5" x2="13" y2="4.5" stroke="currentColor" stroke-width="1.2"/>
              <line x1="4.5" y1="4.5" x2="4.5" y2="13" stroke="currentColor" stroke-width="1.2"/>
            </svg>
            <!-- Query icon -->
            <svg v-else-if="node.kind === 'query'" class="node-icon" viewBox="0 0 14 14" fill="none">
              <polyline points="2,4 5,7 2,10" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
              <line x1="7" y1="10" x2="12" y2="10" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
            </svg>
            <!-- Chart icon -->
            <svg v-else-if="node.kind === 'chart'" class="node-icon" viewBox="0 0 14 14" fill="none">
              <rect x="1" y="1" width="12" height="12" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
              <polyline points="3,9 5,5 8,7 11,3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <!-- Markdown icon -->
            <svg v-else class="node-icon" viewBox="0 0 14 14" fill="none">
              <rect x="1" y="2" width="12" height="10" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
              <line x1="3.5" y1="5.5" x2="10.5" y2="5.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
              <line x1="3.5" y1="8" x2="7.5" y2="8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
            </svg>

            <span class="node-name">{{ node.name }}</span>
          </button>
        </div>
      </template>

      <div v-if="!schemaStore.nodes.length" class="empty-hint">
        No nodes on canvas
      </div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: 196px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--surface-1);
  border-right: 1px solid var(--border);
  overflow: hidden;
}

.sidebar-header {
  padding: 8px 12px 6px;
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.sidebar-body {
  flex: 1;
  overflow-y: auto;
  padding: 6px 0 12px;
}

.group {
  margin-bottom: 4px;
}

.group-label {
  padding: 4px 12px 2px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-muted);
  opacity: 0.6;
}

.node-item {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  padding: 4px 12px;
  font-size: 12px;
  color: var(--text-secondary);
  background: transparent;
  border: none;
  border-radius: 0;
  text-align: left;
  cursor: pointer;
  transition: color 0.1s, background 0.1s;
  user-select: none;
}

.node-item:hover {
  color: var(--text-primary);
  background: var(--surface-2);
}

.node-item.selected {
  color: var(--accent);
  background: rgba(88, 166, 255, 0.1);
}

.node-icon {
  width: 13px;
  height: 13px;
  flex-shrink: 0;
  opacity: 0.7;
}

.node-item.selected .node-icon {
  opacity: 1;
}

.node-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--font-mono);
  font-size: 11.5px;
}

.empty-hint {
  padding: 16px 12px;
  font-size: 11.5px;
  color: var(--text-muted);
  opacity: 0.5;
  font-style: italic;
}
</style>
