<script setup lang="ts">
import { computed, watch, ref } from 'vue'
import confetti from 'canvas-confetti'
import { useSchemaStore } from '../stores/schema'
import { useExerciseResults } from '../composables/useExerciseResults'
import type { ExerciseNode } from '../stores/schema'

const emit = defineEmits<{
  focusNode: [id: string]
}>()

const schemaStore = useSchemaStore()
const { getResult } = useExerciseResults()

// Order by nextId chain first, then fall back to canvas position for unlinked nodes
const exerciseNodes = computed(() => {
  const all = schemaStore.nodes.filter(n => n.kind === 'exercise') as ExerciseNode[]
  const byId = new Map(all.map(n => [n.id, n]))
  const hasIncoming = new Set(all.map(n => n.nextId).filter(Boolean) as string[])
  // Chain heads: nodes nothing else points to
  const heads = all.filter(n => !hasIncoming.has(n.id))
    .sort((a, b) => a.y !== b.y ? a.y - b.y : a.x - b.x)
  const ordered: ExerciseNode[] = []
  const visited = new Set<string>()
  function follow(node: ExerciseNode) {
    if (visited.has(node.id)) return
    visited.add(node.id)
    ordered.push(node)
    if (node.nextId && byId.has(node.nextId)) follow(byId.get(node.nextId)!)
  }
  heads.forEach(follow)
  all.filter(n => !visited.has(n.id)).sort((a, b) => a.y !== b.y ? a.y - b.y : a.x - b.x).forEach(follow)
  return ordered
})

const totalCount   = computed(() => exerciseNodes.value.length)
const passedCount  = computed(() => exerciseNodes.value.filter(n => getResult(n.id).passed === true).length)
const failedCount  = computed(() => exerciseNodes.value.filter(n => getResult(n.id).passed === false).length)
const progressPct  = computed(() => totalCount.value ? Math.round((passedCount.value / totalCount.value) * 100) : 0)
const allComplete  = computed(() => totalCount.value > 0 && passedCount.value === totalCount.value)

// Fire confetti the first time all exercises are passed; reset if progress drops
const celebrated = ref(false)
watch(allComplete, (yes) => {
  if (!yes) { celebrated.value = false; return }
  if (celebrated.value) return
  celebrated.value = true
  // Three-burst celebration
  confetti({ particleCount: 120, spread: 80,  origin: { x: 0.5, y: 0.55 } })
  setTimeout(() => confetti({ particleCount: 60, spread: 60, angle: 60,  origin: { x: 0.1, y: 0.6 } }), 300)
  setTimeout(() => confetti({ particleCount: 60, spread: 60, angle: 120, origin: { x: 0.9, y: 0.6 } }), 500)
})

function passRate(id: string): string {
  const r = getResult(id)
  if (!r.attempts) return ''
  return Math.round((r.passCount / r.attempts) * 100) + '%'
}

function statusLabel(id: string): 'pending' | 'pass' | 'fail' {
  const r = getResult(id)
  if (r.passed === null) return 'pending'
  return r.passed ? 'pass' : 'fail'
}

function nextNodeName(node: ExerciseNode): string | null {
  if (!node.nextId) return null
  const next = schemaStore.nodes.find(n => n.id === node.nextId)
  return next ? next.name : null
}
</script>

<template>
  <aside class="exercises-panel">
    <!-- Header -->
    <div class="panel-header">
      <span class="panel-title">Exercises</span>
      <span class="panel-score">{{ passedCount }} / {{ totalCount }}</span>
    </div>

    <!-- Progress bar -->
    <div class="progress-bar-wrap" v-if="totalCount > 0">
      <div
        class="progress-bar-fill"
        :class="{ complete: passedCount === totalCount && totalCount > 0 }"
        :style="{ width: progressPct + '%' }"
      />
    </div>

    <!-- Stats row -->
    <div class="stats-row" v-if="totalCount > 0">
      <span class="stat pass">{{ passedCount }} passed</span>
      <span class="stat-sep">·</span>
      <span class="stat fail">{{ failedCount }} failed</span>
      <span class="stat-sep">·</span>
      <span class="stat muted">{{ totalCount - passedCount - failedCount }} pending</span>
    </div>

    <div class="panel-divider" />

    <!-- All-complete celebration banner -->
    <div v-if="allComplete" class="congrats-banner">
      <div class="congrats-icon">🎉</div>
      <div class="congrats-text">
        <strong>All done!</strong>
        <span>You passed every exercise.</span>
      </div>
    </div>

    <!-- Empty state -->
    <div class="empty-state" v-if="totalCount === 0">
      <svg viewBox="0 0 20 20" fill="none" class="empty-icon">
        <rect x="3" y="4" width="14" height="12" rx="2" stroke="currentColor" stroke-width="1.3"/>
        <path d="M7 8h6M7 11h4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
        <path d="M13 14l2 2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
      </svg>
      <p>No assertion nodes in this canvas.</p>
      <p class="empty-hint">Add one via right-click → Add Assertion node</p>
    </div>

    <!-- Exercise list -->
    <div class="exercise-list" v-else>
      <div v-for="(node, idx) in exerciseNodes" :key="node.id" class="exercise-item">
        <button
          class="exercise-row"
          :class="statusLabel(node.id)"
          @click="emit('focusNode', node.id)"
        >
          <!-- Number badge -->
          <span class="num-badge" :class="statusLabel(node.id)">
            <template v-if="statusLabel(node.id) === 'pass'">
              <svg viewBox="0 0 10 10" fill="none"><path d="M2 5.5l2.5 2.5 3.5-4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
            </template>
            <template v-else-if="statusLabel(node.id) === 'fail'">
              <svg viewBox="0 0 10 10" fill="none"><path d="M3 3l4 4M7 3l-4 4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
            </template>
            <template v-else>{{ idx + 1 }}</template>
          </span>

          <!-- Name + details -->
          <div class="exercise-info">
            <span class="exercise-name">{{ node.name }}</span>
            <div class="exercise-meta" v-if="getResult(node.id).attempts > 0">
              <span class="meta-chip">{{ getResult(node.id).attempts }} attempt{{ getResult(node.id).attempts !== 1 ? 's' : '' }}</span>
              <span class="meta-chip rate">{{ passRate(node.id) }} pass rate</span>
            </div>
          </div>

          <!-- Status chip -->
          <span class="status-chip" :class="statusLabel(node.id)">
            {{ statusLabel(node.id) === 'pass' ? 'Pass' : statusLabel(node.id) === 'fail' ? 'Fail' : '···' }}
          </span>
        </button>

        <!-- Chain arrow shown when this node links to a next lesson -->
        <div v-if="node.nextId && nextNodeName(node)" class="chain-arrow">
          <span class="chain-line" />
          <span class="chain-tip">→ {{ nextNodeName(node) }}</span>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.exercises-panel {
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

/* ── Progress bar ── */
.progress-bar-wrap {
  height: 3px;
  background: var(--surface-3, var(--surface-2));
  flex-shrink: 0;
  margin: 0 12px 8px;
  border-radius: 2px;
  overflow: hidden;
}
.progress-bar-fill {
  height: 100%;
  background: var(--accent, #58a6ff);
  border-radius: 2px;
  transition: width 0.4s ease, background 0.3s;
}
.progress-bar-fill.complete {
  background: #3fb950;
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
.stat.pass { color: #3fb950; }
.stat.fail { color: #f85149; }
.stat.muted { color: var(--text-muted); }
.stat-sep { color: var(--text-muted); }

.panel-divider {
  height: 1px;
  background: var(--border);
  flex-shrink: 0;
}

/* ── Congrats banner ── */
.congrats-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 10px 10px 4px;
  padding: 10px 12px;
  background: linear-gradient(135deg, rgba(63,185,80,0.12), rgba(88,166,255,0.08));
  border: 1px solid rgba(63, 185, 80, 0.35);
  border-radius: 8px;
  flex-shrink: 0;
  animation: pop-in 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}
.congrats-icon {
  font-size: 22px;
  line-height: 1;
  flex-shrink: 0;
}
.congrats-text {
  display: flex;
  flex-direction: column;
  gap: 1px;
  font-size: 11.5px;
}
.congrats-text strong {
  color: #3fb950;
  font-weight: 700;
}
.congrats-text span {
  color: var(--text-secondary);
  font-size: 10.5px;
}
@keyframes pop-in {
  from { transform: scale(0.85); opacity: 0; }
  to   { transform: scale(1);    opacity: 1; }
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
.empty-icon {
  width: 28px;
  height: 28px;
  margin-bottom: 8px;
  opacity: 0.5;
}
.empty-hint {
  font-size: 10px;
  opacity: 0.7;
  margin-top: 4px;
}

/* ── Exercise list ── */
.exercise-list {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 6px 0;
}

.exercise-item { display: flex; flex-direction: column; }

.exercise-row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 7px 12px;
  background: none;
  border: none;
  cursor: pointer;
  text-align: left;
  transition: background 0.1s;
  position: relative;
}
.exercise-row:hover { background: var(--surface-2); }

/* ── Number badge ── */
.num-badge {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.num-badge.pending {
  background: var(--surface-3, var(--surface-2));
  color: var(--text-muted);
}
.num-badge.pass {
  background: rgba(63, 185, 80, 0.2);
  color: #3fb950;
}
.num-badge.fail {
  background: rgba(248, 81, 73, 0.15);
  color: #f85149;
}
.num-badge svg { width: 10px; height: 10px; }

/* ── Exercise info ── */
.exercise-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.exercise-name {
  color: var(--text-primary);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.exercise-meta {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}
.meta-chip {
  font-size: 10px;
  color: var(--text-muted);
}
.meta-chip.rate { color: var(--text-secondary); }

/* ── Chain arrow ── */
.chain-arrow {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 12px 2px 40px;
}
.chain-line {
  width: 1px;
  height: 14px;
  background: var(--border);
  display: block;
  margin-left: 9px;
}
.chain-tip {
  font-size: 10px;
  color: var(--text-muted);
  font-style: italic;
}

/* ── Status chip ── */
.status-chip {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 8px;
  letter-spacing: 0.02em;
}
.status-chip.pending {
  background: var(--surface-3, var(--surface-2));
  color: var(--text-muted);
}
.status-chip.pass {
  background: rgba(63, 185, 80, 0.15);
  color: #3fb950;
}
.status-chip.fail {
  background: rgba(248, 81, 73, 0.1);
  color: #f85149;
}
</style>
