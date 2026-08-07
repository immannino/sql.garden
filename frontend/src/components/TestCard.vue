<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useSchemaStore } from '../stores/schema'
import { useDuckDB } from '../composables/useDuckDB'
import type { TestNode, TestOperator, TestAssertionMode } from '../stores/schema'

const props = defineProps<{ node: TestNode; selected?: boolean }>()
const emit = defineEmits<{
  dragStart: [{ id: string; mouseX: number; mouseY: number; shiftKey: boolean }]
  resizeStart: [{ id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }]
}>()

const schemaStore = useSchemaStore()
const { query } = useDuckDB()

const DEFAULT_W = 400
const DEFAULT_H = 280

// ── Drag / resize ──────────────────────────────────────────────────────────────
function onMouseDown(e: MouseEvent) {
  const t = e.target as HTMLElement
  if (t.closest('input,textarea,select,button')) return
  emit('dragStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY, shiftKey: e.shiftKey })
}

function onResizeStart(e: MouseEvent, direction: 'e' | 's' | 'se') {
  emit('resizeStart', {
    id: props.node.id, mouseX: e.clientX, mouseY: e.clientY,
    startW: props.node.w ?? DEFAULT_W, startH: props.node.h ?? DEFAULT_H, direction,
  })
}

// ── Rename ─────────────────────────────────────────────────────────────────────
const renaming = ref(false)
const renameValue = ref('')
const renameInputRef = ref<HTMLInputElement | null>(null)
function startRename(e: MouseEvent) {
  if ((e.target as HTMLElement).closest('button')) return
  renaming.value = true
  renameValue.value = props.node.name
  import('vue').then(({ nextTick }) => nextTick(() => renameInputRef.value?.select()))
}
function commitRename() {
  const v = renameValue.value.trim()
  if (v && v !== props.node.name) schemaStore.renameNode(props.node.id, v)
  renaming.value = false
}

// ── Edit mode ──────────────────────────────────────────────────────────────────
const isEdit = ref(false)

// ── Local edit state ───────────────────────────────────────────────────────────
const localSql      = ref(props.node.sql)
const localMode     = ref<TestAssertionMode>(props.node.assertionMode)
const localExpected = ref<number>(props.node.expectedValue ?? 0)
const localOperator = ref<TestOperator>(props.node.operator ?? '==')
const localInterval = ref(props.node.interval)

watch(() => props.node.sql,            v => { localSql.value = v })
watch(() => props.node.assertionMode,  v => { localMode.value = v })
watch(() => props.node.expectedValue,  v => { localExpected.value = v ?? 0 })
watch(() => props.node.operator,       v => { localOperator.value = v ?? '==' })
watch(() => props.node.interval,       v => { localInterval.value = v })

function saveConfig() {
  schemaStore.updateTestNode(props.node.id, {
    sql: localSql.value.trim(),
    assertionMode: localMode.value,
    expectedValue: localExpected.value,
    operator: localOperator.value,
    interval: Math.max(0, Math.floor(Number(localInterval.value) || 0)),
  })
  resetTimer()
}

const OPERATORS: { value: TestOperator; label: string }[] = [
  { value: '==', label: '= equals' },
  { value: '>=', label: '≥ at least' },
  { value: '<=', label: '≤ at most' },
  { value: '>',  label: '> more than' },
  { value: '<',  label: '< less than' },
]

const MODE_LABELS: Record<TestAssertionMode, string> = {
  no_rows:       'No rows',
  scalar_equals: 'Scalar value',
  row_count:     'Row count',
}

const MODE_DESCRIPTIONS: Record<TestAssertionMode, string> = {
  no_rows:       'Pass if query returns 0 rows (bad rows = failing records)',
  scalar_equals: 'Pass if the first cell (e.g. COUNT(*)) matches the expected value',
  row_count:     'Pass if number of rows returned matches the expected count',
}

// ── Status derived from history ────────────────────────────────────────────────
const status = computed<'passing' | 'failing' | 'pending'>(() => {
  if (!props.node.history.length) return 'pending'
  return props.node.history[props.node.history.length - 1].passed ? 'passing' : 'failing'
})

const lastRunTs = computed(() => {
  if (!props.node.history.length) return null
  return props.node.history[props.node.history.length - 1].ts
})

const lastRunLabel = computed(() => {
  if (!lastRunTs.value) return 'Never'
  const sec = Math.floor((Date.now() - lastRunTs.value) / 1000)
  if (sec < 5)    return 'just now'
  if (sec < 60)   return `${sec}s ago`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`
  return `${Math.floor(sec / 3600)}h ago`
})

const sparkHistory = computed(() => props.node.history.slice(-12))

// ── Assertion description shown in view mode ───────────────────────────────────
const assertionSummary = computed(() => {
  const mode = props.node.assertionMode
  if (mode === 'no_rows') return 'Returns 0 rows'
  const op = props.node.operator ?? '=='
  const val = props.node.expectedValue ?? 0
  const opLabel = OPERATORS.find(o => o.value === op)?.label.split(' ')[0] ?? op
  if (mode === 'scalar_equals') return `First cell ${opLabel} ${val}`
  return `Row count ${opLabel} ${val}`
})

// ── Run logic ─────────────────────────────────────────────────────────────────
const isRunning = ref(false)
const runError  = ref<string | null>(null)

function evaluate(rows: Record<string, unknown>[]): boolean {
  const mode = props.node.assertionMode
  const op   = props.node.operator ?? '=='
  const exp  = props.node.expectedValue ?? 0

  if (mode === 'no_rows') return rows.length === 0

  if (mode === 'scalar_equals') {
    if (!rows.length) return false
    const raw = Object.values(rows[0])[0]
    const actual = Number(raw)
    if (Number.isNaN(actual)) return false
    return compare(actual, op, exp)
  }

  // row_count
  return compare(rows.length, op, exp)
}

function compare(actual: number, op: TestOperator, expected: number): boolean {
  if (op === '==') return actual === expected
  if (op === '>=') return actual >= expected
  if (op === '<=') return actual <= expected
  if (op === '>')  return actual > expected
  return actual < expected
}

async function runTest() {
  if (isRunning.value) return
  isRunning.value = true
  runError.value = null
  try {
    const sql = props.node.sql.trim()
    if (!sql) { runError.value = 'No SQL configured'; return }
    const result = await query(sql)
    const passed = evaluate(result.rows)
    schemaStore.updateTestNode(props.node.id, {
      history: [...props.node.history.slice(-19), { ts: Date.now(), passed }],
    })
  } catch (err) {
    runError.value = err instanceof Error ? err.message : String(err)
    schemaStore.updateTestNode(props.node.id, {
      history: [...props.node.history.slice(-19), { ts: Date.now(), passed: false }],
    })
  } finally {
    isRunning.value = false
  }
}

// ── Auto-run timer ─────────────────────────────────────────────────────────────
let _timer: ReturnType<typeof setInterval> | null = null

function resetTimer() {
  if (_timer) { clearInterval(_timer); _timer = null }
  if (props.node.interval > 0) _timer = setInterval(runTest, props.node.interval * 1000)
}

watch(() => props.node.interval, resetTimer)
onMounted(resetTimer)
onUnmounted(() => { if (_timer) clearInterval(_timer) })
</script>

<template>
  <div
    class="test-card"
    :class="{ selected }"
    :data-node-id="node.id"
    :style="{ left: `${node.x}px`, top: `${node.y}px`, width: `${node.w ?? DEFAULT_W}px`, minHeight: `${node.h ?? DEFAULT_H}px` }"
  >
    <!-- Header — drag handle -->
    <div class="test-card__header" :style="{ background: node.color }" @mousedown.stop="onMouseDown" @dblclick.stop="startRename">
      <svg class="test-card__icon" viewBox="0 0 16 16" fill="none">
        <path d="M8 2L3 4.5v4C3 11.5 5.5 14 8 14.5 10.5 14 13 11.5 13 8.5v-4L8 2z" stroke="white" stroke-width="1.3" stroke-linejoin="round"/>
        <path d="M5.5 8l2 2 3-3" stroke="white" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
      <input
        v-if="renaming"
        ref="renameInputRef"
        v-model="renameValue"
        class="test-card__rename-input"
        @blur="commitRename"
        @keydown.enter="commitRename"
        @keydown.escape="renaming = false"
        @mousedown.stop
        @click.stop
      />
      <span v-else class="test-card__name" @dblclick.stop="startRename">{{ node.name }}</span>
      <span class="test-card__status-dot" :class="`test-card__status-dot--${status}`" :title="status" />
      <button class="test-card__icon-btn" :class="{ active: isEdit }" :title="isEdit ? 'View' : 'Edit'"
        @mousedown.stop @click.stop="isEdit = !isEdit">
        <svg viewBox="0 0 12 12" fill="none">
          <path d="M8.5 2L10 3.5 4.5 9 2.5 9.5 3 7.5 8.5 2z" stroke="white" stroke-width="1.1" stroke-linejoin="round"/>
        </svg>
      </button>
    </div>

    <!-- Body -->
    <div class="test-card__body" @mousedown.stop>

      <!-- ── EDIT MODE ── -->
      <template v-if="isEdit">

        <!-- Assertion mode -->
        <div class="test-card__section-label">Assertion type</div>
        <div class="test-card__mode-tabs">
          <button
            v-for="m in (['no_rows', 'scalar_equals', 'row_count'] as TestAssertionMode[])"
            :key="m"
            class="test-card__mode-tab"
            :class="{ active: localMode === m }"
            @click.stop="localMode = m"
          >{{ MODE_LABELS[m] }}</button>
        </div>
        <div class="test-card__mode-desc">{{ MODE_DESCRIPTIONS[localMode] }}</div>

        <!-- Expected value (scalar_equals / row_count) -->
        <template v-if="localMode !== 'no_rows'">
          <div class="test-card__section-label">Expected</div>
          <div class="test-card__expected-row">
            <select v-model="localOperator" class="test-card__op-select">
              <option v-for="op in OPERATORS" :key="op.value" :value="op.value">{{ op.label }}</option>
            </select>
            <input v-model.number="localExpected" type="number" class="test-card__expected-input" min="0" />
          </div>
        </template>

        <!-- SQL -->
        <div class="test-card__section-label">SQL</div>
        <textarea
          v-model="localSql"
          class="test-card__sql-editor"
          spellcheck="false"
          :placeholder="localMode === 'no_rows'
            ? 'SELECT * FROM orders WHERE amount < 0'
            : localMode === 'scalar_equals'
              ? 'SELECT COUNT(*) FROM orders'
              : 'SELECT * FROM new_orders'"
        />

        <!-- Interval -->
        <div class="test-card__section-label">Auto-run interval</div>
        <div class="test-card__interval-row">
          <input v-model.number="localInterval" type="number" class="test-card__interval-input" min="0" step="1" />
          <span class="test-card__interval-unit">seconds&nbsp;(0 = manual)</span>
        </div>

        <!-- Save -->
        <div class="test-card__save-row">
          <button class="test-card__save-btn" @click.stop="saveConfig(); isEdit = false">Save</button>
        </div>

      </template>

      <!-- ── VIEW MODE ── -->
      <template v-else>
        <!-- Status + last run -->
        <div class="test-card__status-row">
          <span class="test-card__status-badge" :class="`test-card__status-badge--${status}`">
            {{ status === 'passing' ? 'Passing' : status === 'failing' ? 'Failing' : 'Pending' }}
          </span>
          <span class="test-card__last-run">{{ lastRunLabel }}</span>
        </div>

        <!-- Assertion summary pill -->
        <div class="test-card__assertion-row">
          <span class="test-card__assertion-mode-chip">{{ MODE_LABELS[node.assertionMode] }}</span>
          <span class="test-card__assertion-summary">{{ assertionSummary }}</span>
        </div>

        <!-- Sparkline -->
        <div v-if="node.history.length" class="test-card__sparkline">
          <span class="test-card__sparkline-label">History</span>
          <div class="test-card__dots">
            <span
              v-for="(entry, i) in sparkHistory" :key="i"
              class="test-card__dot"
              :class="entry.passed ? 'test-card__dot--pass' : 'test-card__dot--fail'"
              :title="new Date(entry.ts).toLocaleString()"
            />
          </div>
        </div>

        <!-- SQL preview -->
        <pre class="test-card__sql-block">{{ node.sql || '(no SQL configured)' }}</pre>
      </template>

      <!-- Error -->
      <div v-if="runError" class="test-card__error">
        <span>{{ runError }}</span>
        <button @click.stop="runError = null">✕</button>
      </div>
    </div>

    <!-- Footer -->
    <div class="test-card__footer" @mousedown.stop>
      <span class="test-card__interval-badge">
        {{ node.interval > 0 ? `Every ${node.interval}s` : 'Manual' }}
      </span>
      <button class="test-card__run-btn" :disabled="isRunning" @click.stop="runTest">
        <svg viewBox="0 0 10 10" fill="none" :class="{ 'test-card__spin': isRunning }">
          <path v-if="!isRunning" d="M3 2l5 3-5 3V2z" fill="currentColor"/>
          <path v-else d="M9 5A4 4 0 1 1 5 1" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
        </svg>
        {{ isRunning ? 'Running…' : 'Run' }}
      </button>
    </div>

    <!-- Resize handles -->
    <div class="test-card__rh-e"  @mousedown.stop="onResizeStart($event, 'e')" />
    <div class="test-card__rh-s"  @mousedown.stop="onResizeStart($event, 's')" />
    <div class="test-card__rh-se" @mousedown.stop="onResizeStart($event, 'se')" />
  </div>
</template>

<style scoped>
.test-card {
  position: absolute;
  background: var(--surface-1);
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  box-shadow: 0 2px 8px rgba(0,0,0,0.18);
  min-width: 280px;
}
.test-card.selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 30%, transparent);
}

/* ── Header ── */
.test-card__header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 8px;
  height: 32px;
  flex-shrink: 0;
  border-radius: 7px 7px 0 0;
  cursor: grab;
}
.test-card__header:active { cursor: grabbing; }
.test-card__icon { width: 14px; height: 14px; flex-shrink: 0; }
.test-card__rename-input {
  flex: 1;
  background: transparent;
  border: none;
  outline: 1px solid rgba(255,255,255,0.6);
  border-radius: 3px;
  font-size: 12px;
  font-weight: 600;
  color: white;
  padding: 0 4px;
  min-width: 0;
}
.test-card__name {
  flex: 1;
  font-size: 12px;
  font-weight: 600;
  color: white;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}
.test-card__status-dot {
  width: 8px; height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  border: 1.5px solid rgba(255,255,255,0.5);
}
.test-card__status-dot--passing { background: #22c55e; }
.test-card__status-dot--failing { background: #ef4444; }
.test-card__status-dot--pending { background: rgba(255,255,255,0.35); }
.test-card__icon-btn {
  background: none; border: none; cursor: pointer;
  padding: 2px; display: flex; align-items: center;
  opacity: 0.65; border-radius: 3px; flex-shrink: 0;
  transition: opacity 0.1s, background 0.1s;
}
.test-card__icon-btn:hover { opacity: 1; }
.test-card__icon-btn.active { opacity: 1; background: rgba(255,255,255,0.2); }
.test-card__icon-btn svg { width: 12px; height: 12px; }

/* ── Body ── */
.test-card__body {
  flex: 1; display: flex; flex-direction: column;
  overflow-y: auto; min-height: 0;
}
.test-card__section-label {
  font-size: 10px; font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase; letter-spacing: 0.06em;
  padding: 7px 10px 3px;
}

/* ── Mode tabs ── */
.test-card__mode-tabs {
  display: flex;
  gap: 0;
  padding: 0 10px 4px;
}
.test-card__mode-tab {
  flex: 1;
  padding: 4px 0;
  font-size: 11px;
  font-weight: 500;
  background: var(--surface-2);
  border: 1px solid var(--border);
  color: var(--text-muted);
  cursor: pointer;
  transition: background 0.1s, color 0.1s;
}
.test-card__mode-tab:first-child { border-radius: 4px 0 0 4px; }
.test-card__mode-tab:last-child  { border-radius: 0 4px 4px 0; border-left: none; }
.test-card__mode-tab:not(:first-child):not(:last-child) { border-left: none; }
.test-card__mode-tab.active {
  background: var(--accent);
  border-color: var(--accent);
  color: white;
}

.test-card__mode-desc {
  font-size: 10.5px;
  color: var(--text-muted);
  padding: 0 10px 8px;
  line-height: 1.4;
  border-bottom: 1px solid var(--border);
}

/* ── Expected row ── */
.test-card__expected-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px 8px;
  border-bottom: 1px solid var(--border);
}
.test-card__op-select {
  padding: 3px 5px; font-size: 11px;
  color: var(--text-primary); background: var(--surface-2);
  border: 1px solid var(--border); border-radius: 4px; outline: none;
  flex: 1;
}
.test-card__op-select:focus { border-color: var(--accent); }
.test-card__expected-input {
  width: 72px; padding: 3px 5px; font-size: 11px;
  font-family: var(--font-mono, monospace);
  color: var(--text-primary); background: var(--surface-2);
  border: 1px solid var(--border); border-radius: 4px; outline: none;
  flex-shrink: 0;
}
.test-card__expected-input:focus { border-color: var(--accent); }

/* ── SQL editor ── */
.test-card__sql-editor {
  display: block; width: 100%; box-sizing: border-box;
  min-height: 80px; resize: vertical;
  padding: 6px 10px;
  font-family: var(--font-mono, monospace); font-size: 11px; line-height: 1.5;
  color: var(--text-primary); background: var(--surface-2);
  border: none; border-bottom: 1px solid var(--border); outline: none;
}
.test-card__sql-editor:focus { box-shadow: inset 0 0 0 1px var(--accent); }

/* ── Interval row ── */
.test-card__interval-row {
  display: flex; align-items: center; gap: 8px;
  padding: 4px 10px 8px;
}
.test-card__interval-input {
  width: 64px; padding: 3px 5px; font-size: 11px;
  font-family: var(--font-mono, monospace);
  color: var(--text-primary); background: var(--surface-2);
  border: 1px solid var(--border); border-radius: 4px; outline: none;
}
.test-card__interval-input:focus { border-color: var(--accent); }
.test-card__interval-unit { font-size: 11px; color: var(--text-muted); }

/* ── Save row ── */
.test-card__save-row {
  padding: 6px 10px 8px;
  display: flex; justify-content: flex-end;
}
.test-card__save-btn {
  padding: 4px 14px; font-size: 12px; font-weight: 600;
  background: var(--accent); color: white;
  border: none; border-radius: 5px; cursor: pointer;
  transition: opacity 0.1s;
}
.test-card__save-btn:hover { opacity: 0.85; }

/* ── View mode: status ── */
.test-card__status-row {
  display: flex; align-items: center; gap: 10px; padding: 8px 10px 4px;
}
.test-card__status-badge {
  font-size: 12px; font-weight: 700;
  padding: 2px 10px; border-radius: 8px;
}
.test-card__status-badge--passing {
  color: #22c55e;
  background: color-mix(in srgb, #22c55e 12%, transparent);
  border: 1px solid color-mix(in srgb, #22c55e 30%, transparent);
}
.test-card__status-badge--failing {
  color: #ef4444;
  background: color-mix(in srgb, #ef4444 12%, transparent);
  border: 1px solid color-mix(in srgb, #ef4444 30%, transparent);
}
.test-card__status-badge--pending {
  color: var(--text-muted);
  background: color-mix(in srgb, var(--text-muted) 10%, transparent);
  border: 1px solid var(--border);
}
.test-card__last-run { font-size: 10px; color: var(--text-muted); }

/* ── Assertion summary ── */
.test-card__assertion-row {
  display: flex; align-items: center; gap: 7px;
  padding: 0 10px 8px;
}
.test-card__assertion-mode-chip {
  font-size: 9px; font-weight: 700; letter-spacing: 0.04em;
  color: var(--accent);
  background: color-mix(in srgb, var(--accent) 12%, transparent);
  border-radius: 3px; padding: 1px 5px; flex-shrink: 0;
}
.test-card__assertion-summary {
  font-size: 11px; color: var(--text-secondary);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}

/* ── Sparkline ── */
.test-card__sparkline {
  display: flex; align-items: center; gap: 6px;
  padding: 0 10px 8px;
}
.test-card__sparkline-label {
  font-size: 9px; font-weight: 700;
  color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.06em;
  flex-shrink: 0;
}
.test-card__dots { display: flex; align-items: center; gap: 3px; }
.test-card__dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.test-card__dot--pass { background: #22c55e; }
.test-card__dot--fail { background: #ef4444; }

/* ── SQL preview ── */
.test-card__sql-block {
  margin: 0; padding: 6px 10px;
  font-family: var(--font-mono, monospace); font-size: 11px; line-height: 1.5;
  color: var(--text-secondary); background: var(--surface-2);
  border-top: 1px solid var(--border);
  white-space: pre-wrap; word-break: break-all;
  overflow-x: auto; max-height: 100px; overflow-y: auto;
}

/* ── Error ── */
.test-card__error {
  display: flex; align-items: flex-start; gap: 6px;
  padding: 6px 10px;
  background: color-mix(in srgb, #ef4444 10%, transparent);
  border-top: 1px solid color-mix(in srgb, #ef4444 30%, transparent);
  font-size: 11px; color: #ef4444;
  font-family: var(--font-mono, monospace);
}
.test-card__error button {
  background: none; border: none; cursor: pointer;
  color: #ef4444; margin-left: auto; flex-shrink: 0;
}

/* ── Footer ── */
.test-card__footer {
  display: flex; align-items: center; gap: 6px;
  padding: 4px 8px; border-top: 1px solid var(--border);
  background: var(--surface-2); min-height: 30px; flex-shrink: 0;
}
.test-card__interval-badge {
  flex: 1; font-size: 10px; color: var(--text-muted);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.test-card__run-btn {
  display: flex; align-items: center; gap: 4px;
  padding: 3px 10px;
  background: var(--accent); color: white;
  border: none; border-radius: 4px;
  font-size: 11px; font-weight: 600; cursor: pointer; flex-shrink: 0;
  transition: opacity 0.1s;
}
.test-card__run-btn:hover:not(:disabled) { opacity: 0.85; }
.test-card__run-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.test-card__run-btn svg { width: 10px; height: 10px; }

@keyframes test-card-spin { to { transform: rotate(360deg); } }
.test-card__spin { animation: test-card-spin 0.8s linear infinite; }

/* ── Resize handles ── */
.test-card__rh-e  { position: absolute; right: 0; top: 8px; bottom: 20px; width: 6px; cursor: ew-resize; }
.test-card__rh-s  { position: absolute; bottom: 0; left: 8px; right: 8px; height: 6px; cursor: ns-resize; }
.test-card__rh-se { position: absolute; right: 0; bottom: 0; width: 14px; height: 14px; cursor: se-resize; }
.test-card__rh-e:hover, .test-card__rh-s:hover, .test-card__rh-se:hover { background: var(--accent); opacity: 0.4; }
</style>
