<script setup lang="ts">
import { computed } from 'vue'
import { useSchemaStore } from '../stores/schema'
import type { TestNode } from '../stores/schema'

const emit = defineEmits<{ focusNode: [id: string] }>()

const schemaStore = useSchemaStore()

const testNodes = computed(() =>
  (schemaStore.nodes.filter(n => n.kind === 'test') as TestNode[])
    .slice()
    .sort((a, b) => a.y !== b.y ? a.y - b.y : a.x - b.x)
)

const totalCount   = computed(() => testNodes.value.length)
const passingCount = computed(() => testNodes.value.filter(n => lastStatus(n) === 'passing').length)
const failingCount = computed(() => testNodes.value.filter(n => lastStatus(n) === 'failing').length)
const pendingCount = computed(() => totalCount.value - passingCount.value - failingCount.value)

function lastStatus(node: TestNode): 'passing' | 'failing' | 'pending' {
  if (!node.history.length) return 'pending'
  return node.history[node.history.length - 1].passed ? 'passing' : 'failing'
}

function lastRunLabel(node: TestNode): string {
  if (!node.history.length) return 'Never'
  const sec = Math.floor((Date.now() - node.history[node.history.length - 1].ts) / 1000)
  if (sec < 5)    return 'just now'
  if (sec < 60)   return `${sec}s ago`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`
  return `${Math.floor(sec / 3600)}h ago`
}

const MODE_LABELS: Record<string, string> = {
  no_rows:       'No rows',
  scalar_equals: 'Scalar',
  row_count:     'Row count',
}

function sparkline(node: TestNode) {
  return node.history.slice(-8)
}
</script>

<template>
  <aside class="tests-panel">
    <!-- Header -->
    <div class="panel-header">
      <span class="panel-title">Tests</span>
      <span class="panel-score">{{ passingCount }} / {{ totalCount }}</span>
    </div>

    <!-- Stats row -->
    <div class="stats-row" v-if="totalCount > 0">
      <span class="stat passing">{{ passingCount }} passing</span>
      <span class="stat-sep">·</span>
      <span class="stat failing">{{ failingCount }} failing</span>
      <span class="stat-sep">·</span>
      <span class="stat muted">{{ pendingCount }} pending</span>
    </div>

    <div class="panel-divider" />

    <!-- Empty state -->
    <div class="empty-state" v-if="totalCount === 0">
      <svg viewBox="0 0 20 20" fill="none" class="empty-icon">
        <path d="M10 2L4 5v5c0 4 2.5 6.5 6 7 3.5-.5 6-3 6-7V5L10 2z" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>
        <path d="M7 10l2 2 4-4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
      <p>No test nodes in this canvas.</p>
      <p class="empty-hint">Press <kbd>T</kbd> to add one</p>
    </div>

    <!-- Test list -->
    <div class="test-list" v-else>
      <button
        v-for="node in testNodes"
        :key="node.id"
        class="test-row"
        :class="lastStatus(node)"
        @click="emit('focusNode', node.id)"
      >
        <!-- Status dot -->
        <span class="status-dot" :class="lastStatus(node)" />

        <!-- Info -->
        <div class="test-info">
          <span class="test-name">{{ node.name }}</span>
          <div class="test-meta">
            <span class="meta-chip mode">{{ MODE_LABELS[node.assertionMode] ?? node.assertionMode }}</span>
            <span class="meta-chip time">{{ lastRunLabel(node) }}</span>
            <span v-if="node.interval > 0" class="meta-chip interval">Every {{ node.interval }}s</span>
          </div>
          <!-- Mini sparkline -->
          <div v-if="node.history.length" class="mini-spark">
            <span
              v-for="(entry, i) in sparkline(node)"
              :key="i"
              class="spark-dot"
              :class="entry.passed ? 'spark-dot--pass' : 'spark-dot--fail'"
            />
          </div>
        </div>

        <!-- Status chip -->
        <span class="status-chip" :class="lastStatus(node)">
          {{ lastStatus(node) === 'passing' ? 'Pass' : lastStatus(node) === 'failing' ? 'Fail' : '···' }}
        </span>
      </button>
    </div>
  </aside>
</template>

<style scoped>
.tests-panel {
  width: 240px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--surface-1);
  border-right: 1px solid var(--border);
  overflow: hidden;
  font-size: 12px;
}

/* ── Header ── */
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px 6px;
  flex-shrink: 0;
}
.panel-title {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-secondary);
}
.panel-score {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}

/* ── Stats ── */
.stats-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 12px 6px;
  flex-shrink: 0;
}
.stat { font-size: 11px; }
.stat.passing { color: #3fb950; }
.stat.failing  { color: #f85149; }
.stat.muted   { color: var(--text-muted); }
.stat-sep     { color: var(--text-muted); }

.panel-divider {
  height: 1px;
  background: var(--border);
  flex-shrink: 0;
}

/* ── Empty state ── */
.empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px 16px;
  text-align: center;
  color: var(--text-muted);
  gap: 4px;
}
.empty-icon { width: 28px; height: 28px; margin-bottom: 8px; opacity: 0.5; }
.empty-hint { font-size: 10px; opacity: 0.7; margin-top: 4px; }
.empty-hint kbd {
  display: inline-block;
  padding: 0 4px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-bottom-width: 2px;
  border-radius: 3px;
  font-family: var(--font-mono, monospace);
  font-size: 9px;
}

/* ── Test list ── */
.test-list {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 6px 0;
}

.test-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  width: 100%;
  padding: 7px 12px;
  background: none;
  border: none;
  cursor: pointer;
  text-align: left;
  transition: background 0.1s;
}
.test-row:hover { background: var(--surface-2); }

/* ── Status dot ── */
.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  margin-top: 3px;
}
.status-dot.passing { background: #3fb950; }
.status-dot.failing  { background: #f85149; }
.status-dot.pending { background: var(--text-muted); opacity: 0.4; }

/* ── Test info ── */
.test-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.test-name {
  font-size: 12px;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.test-meta {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}
.meta-chip {
  font-size: 9.5px;
  color: var(--text-muted);
}
.meta-chip.mode {
  color: var(--accent);
  font-weight: 600;
}

/* ── Mini sparkline ── */
.mini-spark {
  display: flex;
  gap: 2px;
  align-items: center;
  margin-top: 1px;
}
.spark-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  flex-shrink: 0;
}
.spark-dot--pass { background: #3fb950; }
.spark-dot--fail { background: #f85149; }

/* ── Status chip ── */
.status-chip {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 8px;
  letter-spacing: 0.02em;
  align-self: center;
}
.status-chip.passing { background: rgba(63,185,80,0.15); color: #3fb950; }
.status-chip.failing  { background: rgba(248,81,73,0.1);  color: #f85149; }
.status-chip.pending {
  background: var(--surface-3, var(--surface-2));
  color: var(--text-muted);
}
</style>
