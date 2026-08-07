<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { marked } from 'marked'
import { useSchemaStore } from '../stores/schema'
import type { ExerciseNode, ExerciseCheck, ExerciseCheckKind } from '../stores/schema'
import { useDuckDB } from '../composables/useDuckDB'
import { useSchemaCompletions } from '../composables/useSchemaCompletions'
import { useExerciseResults } from '../composables/useExerciseResults'
import SqlEditor from './SqlEditor.vue'
import NodeColorPicker from './NodeColorPicker.vue'

const props = defineProps<{ node: ExerciseNode; selected?: boolean }>()
const emit = defineEmits<{
  dragStart: [{ id: string; mouseX: number; mouseY: number; shiftKey: boolean }]
  resizeStart: [{ id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }]
}>()

const schemaStore = useSchemaStore()
const { query: runQuery } = useDuckDB()
const { sqlSchema } = useSchemaCompletions()
const { recordAttempt, navigateToNode } = useExerciseResults()

// ── Drag / Resize ─────────────────────────────────────────────────────────────
function onMouseDown(e: MouseEvent) {
  if (e.button !== 0) return
  e.stopPropagation()
  emit('dragStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY, shiftKey: e.shiftKey })
}
function startResize(e: MouseEvent, direction: 'e' | 's' | 'se') {
  e.stopPropagation()
  emit('resizeStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY, startW: props.node.w ?? 720, startH: props.node.h ?? 480, direction })
}

// ── Delete ────────────────────────────────────────────────────────────────────
const deleteConfirm = ref(false)
let deleteTimer: ReturnType<typeof setTimeout> | null = null
function onDeleteClick(e: MouseEvent) {
  e.stopPropagation()
  if (!deleteConfirm.value) {
    deleteConfirm.value = true
    deleteTimer = setTimeout(() => { deleteConfirm.value = false }, 3000)
  } else {
    if (deleteTimer) clearTimeout(deleteTimer)
    schemaStore.removeNode(props.node.id)
  }
}

// ── Rename ────────────────────────────────────────────────────────────────────
const isRenaming = ref(false)
const renameValue = ref('')
const renameInputRef = ref<HTMLInputElement | null>(null)
function startRename(e: MouseEvent) {
  e.stopPropagation()
  if (deleteConfirm.value) return
  renameValue.value = props.node.name
  isRenaming.value = true
  import('vue').then(({ nextTick }) => nextTick(() => renameInputRef.value?.select()))
}
function commitRename() {
  const v = renameValue.value.trim()
  if (v && v !== props.node.name) schemaStore.renameNode(props.node.id, v)
  isRenaming.value = false
}

// ── Mode ──────────────────────────────────────────────────────────────────────
// 'student' = default view; 'edit' = educator edits prompt + checks
const editorMode = ref<'student' | 'edit'>('student')

// ── Prompt ────────────────────────────────────────────────────────────────────
const promptDraft = ref(props.node.prompt)
watch(() => props.node.prompt, (v) => { promptDraft.value = v })
const renderedPrompt = computed(() => String(marked.parse(props.node.prompt || '')))

function savePrompt() {
  schemaStore.updateExerciseNode(props.node.id, { prompt: promptDraft.value })
}

// ── Success text ──────────────────────────────────────────────────────────────
const successDraft = ref(props.node.successText ?? '')
watch(() => props.node.successText, (v) => { successDraft.value = v ?? '' })
const renderedSuccessText = computed(() => String(marked.parse(props.node.successText || '')))

function saveSuccessText() {
  schemaStore.updateExerciseNode(props.node.id, { successText: successDraft.value })
}

// ── SQL draft ─────────────────────────────────────────────────────────────────
const sqlDraft = ref(props.node.sql)
watch(() => props.node.sql, (v) => { sqlDraft.value = v })

// ── Other exercise nodes for "Next lesson" dropdown ─────────────────────────
const otherAssertions = computed(() =>
  schemaStore.nodes.filter(n => n.kind === 'exercise' && n.id !== props.node.id)
)

// ── Student results ───────────────────────────────────────────────────────────
const studentRows = ref<Record<string, unknown>[]>([])
const studentCols = ref<string[]>([])
const runError = ref<string | null>(null)

// ── Check editor ──────────────────────────────────────────────────────────────
const CHECK_KINDS: { value: ExerciseCheckKind; label: string }[] = [
  { value: 'set_match',     label: 'Exact result match (order-insensitive)' },
  { value: 'row_count',     label: 'Row count' },
  { value: 'non_empty',     label: 'Result is non-empty' },
  { value: 'no_nulls',      label: 'No NULLs in column' },
  { value: 'column_value',  label: 'Aggregate expression equals value' },
  { value: 'column_exists', label: 'Column exists in result' },
  { value: 'sql_pattern',   label: 'SQL text pattern (structural)' },
]

const editingCheck = ref<ExerciseCheck | null>(null)
const checkDraft = ref<Partial<ExerciseCheck>>({})

function makeCheck(kind: ExerciseCheckKind): ExerciseCheck {
  const id = `chk_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`
  const defaults: Partial<ExerciseCheck> = {
    id, kind, label: '', feedbackOnFail: '',
    countOperator: '==', mustMatch: true,
  }
  return defaults as ExerciseCheck
}

function addCheck(kind: ExerciseCheckKind) {
  const chk = makeCheck(kind)
  editingCheck.value = chk
  checkDraft.value = { ...chk }
}

function editCheck(chk: ExerciseCheck) {
  editingCheck.value = chk
  checkDraft.value = { ...chk }
}

function saveCheck() {
  if (!editingCheck.value) return
  const updated = { ...editingCheck.value, ...checkDraft.value } as ExerciseCheck
  const existing = props.node.checks.find((c) => c.id === updated.id)
  let newChecks: ExerciseCheck[]
  if (existing) {
    newChecks = props.node.checks.map((c) => (c.id === updated.id ? updated : c))
  } else {
    newChecks = [...props.node.checks, updated]
  }
  schemaStore.updateExerciseNode(props.node.id, { checks: newChecks })
  editingCheck.value = null
}

function cancelCheckEdit() {
  editingCheck.value = null
}

function removeCheck(id: string) {
  schemaStore.updateExerciseNode(props.node.id, {
    checks: props.node.checks.filter((c) => c.id !== id),
  })
}

function moveCheck(id: string, dir: -1 | 1) {
  const arr = [...props.node.checks]
  const idx = arr.findIndex((c) => c.id === id)
  const swap = idx + dir
  if (swap < 0 || swap >= arr.length) return
  ;[arr[idx], arr[swap]] = [arr[swap], arr[idx]]
  schemaStore.updateExerciseNode(props.node.id, { checks: arr })
}

// ── Validation engine ─────────────────────────────────────────────────────────
interface CheckResult {
  passed: boolean
  message: string
  actual?: string
}

const checkResults = ref<Map<string, CheckResult>>(new Map())
const isValidating = ref(false)
const attemptCount = ref(0)
const hintsRevealed = ref(false)

const allPassed = computed(() => {
  if (props.node.checks.length === 0 || checkResults.value.size === 0) return false
  return props.node.checks.every((c) => checkResults.value.get(c.id)?.passed === true)
})

const anyRan = computed(() => checkResults.value.size > 0)
const truncatedRows = computed(() => studentRows.value.slice(0, 100))

function normalizeRow(row: Record<string, unknown>): string {
  return JSON.stringify(Object.values(row).map((v) => (v === null || v === undefined ? null : String(v))))
}

function normalizeRows(rows: Record<string, unknown>[]): string[] {
  return rows.map(normalizeRow).sort()
}

async function runAndValidate() {
  if (isValidating.value) return

  // Save SQL to node store
  schemaStore.updateExerciseNode(props.node.id, { sql: sqlDraft.value })

  // Run student query and collect results locally
  runError.value = null
  try {
    const result = await runQuery(sqlDraft.value)
    studentRows.value = result.rows ?? []
    studentCols.value = result.columns ?? []
  } catch (e) {
    runError.value = e instanceof Error ? e.message : String(e)
    return
  }

  isValidating.value = true
  attemptCount.value++

  const studentSql = sqlDraft.value.trim()
  const results = new Map<string, CheckResult>()

  for (const chk of props.node.checks) {
    let result: CheckResult = { passed: false, message: chk.feedbackOnFail }

    try {
      switch (chk.kind) {
        case 'non_empty':
          result = studentRows.value.length > 0
            ? { passed: true, message: `${studentRows.value.length} rows returned` }
            : { passed: false, message: chk.feedbackOnFail || 'Query returned no rows' }
          break

        case 'row_count': {
          const op = chk.countOperator ?? '=='
          const expected = chk.expectedCount ?? 0
          const actual = studentRows.value.length
          const passed = op === '==' ? actual === expected
            : op === '>=' ? actual >= expected
            : op === '<=' ? actual <= expected
            : op === '>'  ? actual > expected
            : actual < expected
          result = {
            passed,
            message: passed
              ? `${actual} rows — correct`
              : chk.feedbackOnFail || `Expected ${op} ${expected} rows, got ${actual}`,
            actual: String(actual),
          }
          break
        }

        case 'no_nulls': {
          const col = chk.column ?? ''
          if (!col) { result = { passed: false, message: 'No column specified' }; break }
          const nullCount = studentRows.value.filter((r) => r[col] === null || r[col] === undefined).length
          result = {
            passed: nullCount === 0,
            message: nullCount === 0
              ? `No NULLs in "${col}"`
              : chk.feedbackOnFail || `Found ${nullCount} NULL${nullCount > 1 ? 's' : ''} in "${col}"`,
          }
          break
        }

        case 'column_exists': {
          const col = chk.column ?? ''
          const found = studentCols.value.includes(col)
          result = {
            passed: found,
            message: found
              ? `Column "${col}" present`
              : chk.feedbackOnFail || `Column "${col}" not found in result. Got: ${studentCols.value.join(', ')}`,
          }
          break
        }

        case 'set_match': {
          if (!chk.referenceSql?.trim()) { result = { passed: false, message: 'No reference SQL configured' }; break }
          let refRows: Record<string, unknown>[] = []
          try { refRows = (await runQuery(chk.referenceSql)).rows ?? [] }
          catch (e) { result = { passed: false, message: `Reference query error: ${e instanceof Error ? e.message : String(e)}` }; break }
          const expectedNorm = normalizeRows(refRows)
          const actualNorm = normalizeRows(studentRows.value)
          const passed = JSON.stringify(expectedNorm) === JSON.stringify(actualNorm)
          result = {
            passed,
            message: passed
              ? `Matches reference output (${actualNorm.length} rows)`
              : chk.feedbackOnFail || `Result doesn't match. Expected ${expectedNorm.length} rows, got ${actualNorm.length}.`,
          }
          break
        }

        case 'column_value': {
          if (!chk.expression?.trim()) { result = { passed: false, message: 'No expression configured' }; break }
          const wrapSql = `WITH _student AS (${studentSql}) SELECT (${chk.expression}) AS _val FROM _student`
          let valRows: Record<string, unknown>[] = []
          try { valRows = (await runQuery(wrapSql)).rows ?? [] }
          catch (e) { result = { passed: false, message: `Expression error: ${e instanceof Error ? e.message : String(e)}` }; break }
          const actual = valRows[0]?.['_val']
          const expected = chk.expectedValue
          let passed = false
          if (typeof expected === 'number') {
            const tol = chk.tolerance ?? 0.001
            passed = Math.abs(Number(actual) - expected) <= tol
          } else {
            passed = String(actual) === String(expected)
          }
          result = {
            passed,
            message: passed
              ? `${chk.expression} = ${actual} ✓`
              : chk.feedbackOnFail || `Expected ${chk.expression} = ${expected}, got ${actual}`,
            actual: String(actual),
          }
          break
        }

        case 'sql_pattern': {
          if (!chk.pattern) { result = { passed: false, message: 'No pattern configured' }; break }
          const re = new RegExp(chk.pattern, 'i')
          const matches = re.test(studentSql)
          const mustMatch = chk.mustMatch !== false
          const passed = mustMatch ? matches : !matches
          result = {
            passed,
            message: passed
              ? mustMatch ? `SQL contains required pattern` : `SQL correctly avoids pattern`
              : chk.feedbackOnFail || (mustMatch ? `SQL must include: ${chk.pattern}` : `SQL must not include: ${chk.pattern}`),
          }
          break
        }
      }
    } catch (err) {
      result = { passed: false, message: `Validation error: ${err instanceof Error ? err.message : String(err)}` }
    }

    results.set(chk.id, result)
  }

  checkResults.value = results
  isValidating.value = false

  recordAttempt(props.node.id, allPassed.value)

  if (
    props.node.revealHintsAfter > 0 &&
    attemptCount.value >= props.node.revealHintsAfter &&
    !allPassed.value
  ) {
    hintsRevealed.value = true
  }
}

function resetState() {
  checkResults.value = new Map()
  attemptCount.value = 0
  hintsRevealed.value = false
  studentRows.value = []
  studentCols.value = []
  runError.value = null
}

watch(() => props.node.checks, resetState, { deep: true })
</script>

<template>
  <div
    class="exercise-card"
    :class="{ selected, 'all-passed': allPassed && anyRan, 'has-failures': anyRan && !allPassed }"
    :style="{ left: node.x + 'px', top: node.y + 'px', width: (node.w ?? 720) + 'px', minHeight: (node.h ?? 480) + 'px', '--node-color': node.color }"
  >
    <!-- Header — drag handle lives here -->
    <div class="card-header" @mousedown.stop="onMouseDown" @dblclick.stop="startRename">
      <span class="card-icon">🎯</span>
      <input
        v-if="isRenaming"
        ref="renameInputRef"
        v-model="renameValue"
        class="rename-input"
        @blur="commitRename"
        @keydown.enter="commitRename"
        @keydown.escape="isRenaming = false"
        @mousedown.stop
        @click.stop
      />
      <span v-else class="card-name" @dblclick.stop="startRename">{{ node.name }}</span>
      <span class="spacer" />
      <NodeColorPicker :color="node.color" @pick="schemaStore.setNodeColor(node.id, $event)" />
      <button
        class="mode-btn"
        :class="{ active: editorMode === 'edit' }"
        title="Toggle editor mode"
        @click.stop="editorMode = editorMode === 'edit' ? 'student' : 'edit'"
        @mousedown.stop
      >
        <svg viewBox="0 0 14 14" fill="none">
          <path d="M2 10.5l1.5-4 6.5-6.5 2 2-6.5 6.5-3.5 2z" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/>
          <line x1="8" y1="2" x2="10" y2="4" stroke="currentColor" stroke-width="1.2"/>
        </svg>
      </button>
      <button class="delete-btn" :class="{ confirm: deleteConfirm }" @click.stop="onDeleteClick" @mousedown.stop>
        {{ deleteConfirm ? '?' : '×' }}
      </button>
    </div>

    <!-- ── Student mode ─────────────────────────────────────────────────────── -->
    <template v-if="editorMode === 'student'">
      <div class="card-body">
        <!-- Left panel: prompt + editor + results -->
        <div class="left-panel">
          <div v-if="node.prompt" class="prompt-body" @mousedown.stop>
            <div class="prompt-md" v-html="renderedPrompt" />
          </div>
          <div v-else class="prompt-empty">No prompt — switch to edit mode to add one.</div>

          <div class="sql-editor-section" @mousedown.stop @click.stop @keydown.stop>
            <SqlEditor v-model="sqlDraft" :height="130" :schema="sqlSchema" @run="runAndValidate" />
            <div v-if="runError" class="run-error">{{ runError }}</div>
          </div>

          <!-- Results table -->
          <div class="results-section" @mousedown.stop>
            <div v-if="!anyRan && !runError" class="results-placeholder">
              Press <kbd>⌘ Enter</kbd> or click Run &amp; Validate to see results
            </div>
            <template v-else-if="studentRows.length === 0 && !runError">
              <div class="results-placeholder">Query returned no rows</div>
            </template>
            <template v-else-if="studentRows.length > 0">
              <div class="results-scroll">
                <table class="mini-table">
                  <thead>
                    <tr><th v-for="col in studentCols" :key="col">{{ col }}</th></tr>
                  </thead>
                  <tbody>
                    <tr v-for="(row, i) in truncatedRows" :key="i">
                      <td v-for="col in studentCols" :key="col">{{ row[col] ?? 'NULL' }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div class="results-meta">
                {{ studentRows.length }} row{{ studentRows.length !== 1 ? 's' : '' }}
                <span v-if="studentRows.length > 100"> · showing first 100</span>
              </div>
            </template>
          </div>
        </div>

        <!-- Right panel: checks + footer -->
        <div class="right-panel">
          <div v-if="node.checks.length" class="checks-list" @mousedown.stop>
            <div
              v-for="chk in node.checks"
              :key="chk.id"
              class="check-row"
              :class="{
                'check-pass': checkResults.get(chk.id)?.passed === true,
                'check-fail': checkResults.get(chk.id)?.passed === false,
              }"
            >
              <span class="check-status">
                <span v-if="checkResults.get(chk.id)?.passed === true" class="dot dot-pass">✓</span>
                <span v-else-if="checkResults.get(chk.id)?.passed === false" class="dot dot-fail">✗</span>
                <span v-else class="dot dot-pending">·</span>
              </span>
              <div class="check-content">
                <span class="check-label">{{ chk.label || chk.kind }}</span>
                <span v-if="checkResults.get(chk.id)?.passed === false" class="check-feedback">
                  {{ checkResults.get(chk.id)?.message }}
                </span>
                <span v-if="hintsRevealed && chk.hint && checkResults.get(chk.id)?.passed === false" class="check-hint">
                  💡 {{ chk.hint }}
                </span>
              </div>
            </div>
          </div>
          <div v-else class="checks-empty">No checks configured.</div>

          <!-- Success banner — shown when all checks pass -->
          <div v-if="anyRan && allPassed && node.successText" class="success-banner" @mousedown.stop>
            <div class="success-md" v-html="renderedSuccessText" />
          </div>

          <div class="card-footer" @mousedown.stop>
            <!-- Status row — attempt count + verdict, only shown after first run -->
            <div v-if="anyRan" class="footer-status">
              <span class="attempt-count">{{ attemptCount }} attempt{{ attemptCount !== 1 ? 's' : '' }}</span>
              <span class="spacer" />
              <span v-if="allPassed" class="verdict-badge verdict-pass">All passed</span>
              <span v-else class="verdict-badge verdict-fail">
                {{ [...checkResults.values()].filter((r) => r.passed).length }}/{{ node.checks.length }} passing
              </span>
              <button v-if="!allPassed" class="reset-btn" @click.stop="resetState">Reset</button>
            </div>
            <!-- Action row — Next and Run buttons always on the same line -->
            <div class="footer-actions">
              <span class="spacer" />
              <button
                v-if="anyRan && allPassed && node.nextId"
                class="next-lesson-btn"
                @click.stop="navigateToNode(node.nextId!)"
              >Next →</button>
              <button class="validate-btn" :disabled="isValidating" @click.stop="runAndValidate">
                <span v-if="isValidating" class="spinner" />
                <span v-else>Run</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- ── Edit mode ──────────────────────────────────────────────────────────── -->
    <template v-else>
      <!-- Prompt editor -->
      <div class="edit-section" @mousedown.stop>
        <label class="edit-label">Prompt (markdown)</label>
        <textarea
          v-model="promptDraft"
          class="edit-textarea"
          placeholder="Write the exercise prompt in markdown…"
          rows="5"
          @blur="savePrompt"
          @keydown.stop
          @click.stop
        />
      </div>

      <!-- Success text -->
      <div class="edit-section" @mousedown.stop>
        <label class="edit-label">Success text (shown after student passes)</label>
        <textarea
          v-model="successDraft"
          class="edit-textarea"
          placeholder="Great work! Here's what's happening in this query…"
          rows="4"
          @blur="saveSuccessText"
          @keydown.stop
          @click.stop
        />
      </div>

      <!-- Hints reveal -->
      <div class="edit-section edit-inline" @mousedown.stop>
        <label class="edit-label">Reveal hints after</label>
        <input
          type="number"
          class="edit-number"
          min="0"
          :value="node.revealHintsAfter"
          @change="schemaStore.updateExerciseNode(node.id, { revealHintsAfter: +($event.target as HTMLInputElement).value })"
          @mousedown.stop
        />
        <span class="edit-unit">attempts (0 = never)</span>
      </div>

      <!-- Next lesson link -->
      <div class="edit-section edit-inline" @mousedown.stop>
        <label class="edit-label">Next lesson</label>
        <select
          class="edit-select-sm next-select"
          :value="node.nextId ?? ''"
          @change="schemaStore.updateExerciseNode(node.id, { nextId: ($event.target as HTMLSelectElement).value || undefined })"
          @mousedown.stop
        >
          <option value="">(none)</option>
          <option v-for="n in otherAssertions" :key="n.id" :value="n.id">{{ n.name }}</option>
        </select>
      </div>

      <!-- Checks list in edit mode -->
      <div class="edit-section" @mousedown.stop>
        <div class="edit-checks-header">
          <label class="edit-label">Checks</label>
          <div class="add-check-row">
            <select class="add-kind-select" @change="addCheck(($event.target as HTMLSelectElement).value as ExerciseCheckKind); ($event.target as HTMLSelectElement).value = ''">
              <option value="">+ Add check…</option>
              <option v-for="k in CHECK_KINDS" :key="k.value" :value="k.value">{{ k.label }}</option>
            </select>
          </div>
        </div>

        <div v-if="node.checks.length === 0" class="checks-empty">No checks yet.</div>

        <div v-for="(chk, idx) in node.checks" :key="chk.id" class="edit-check-row">
          <div class="edit-check-summary">
            <span class="check-kind-chip">{{ chk.kind }}</span>
            <span class="edit-check-label">{{ chk.label || '(no label)' }}</span>
            <span class="spacer" />
            <button class="ec-btn" title="Move up" :disabled="idx === 0" @click.stop="moveCheck(chk.id, -1)">↑</button>
            <button class="ec-btn" title="Move down" :disabled="idx === node.checks.length - 1" @click.stop="moveCheck(chk.id, 1)">↓</button>
            <button class="ec-btn" title="Edit" @click.stop="editCheck(chk)">✎</button>
            <button class="ec-btn ec-del" title="Remove" @click.stop="removeCheck(chk.id)">×</button>
          </div>
        </div>
      </div>

      <!-- Inline check editor -->
      <div v-if="editingCheck" class="check-editor" @mousedown.stop @click.stop>
        <div class="check-editor-title">{{ CHECK_KINDS.find((k) => k.value === editingCheck?.kind)?.label }}</div>

        <label class="edit-label">Label (shown to student)</label>
        <input v-model="checkDraft.label" class="edit-input" placeholder="e.g. Returns 5 rows" @keydown.stop />

        <label class="edit-label">Feedback on fail</label>
        <input v-model="checkDraft.feedbackOnFail" class="edit-input" placeholder="e.g. Hint: use GROUP BY month" @keydown.stop />

        <label class="edit-label">Hint (optional, revealed after N attempts)</label>
        <input v-model="checkDraft.hint" class="edit-input" placeholder="e.g. Try: SELECT COUNT(*) …" @keydown.stop />

        <!-- Kind-specific fields -->
        <template v-if="editingCheck.kind === 'set_match'">
          <label class="edit-label">Reference SQL (hidden from student)</label>
          <textarea v-model="checkDraft.referenceSql" class="edit-textarea" rows="4" placeholder="SELECT … FROM …" @keydown.stop />
        </template>

        <template v-else-if="editingCheck.kind === 'row_count'">
          <div class="edit-inline">
            <label class="edit-label">Row count</label>
            <select v-model="checkDraft.countOperator" class="edit-select-sm">
              <option value="==">==</option>
              <option value=">=">>=</option>
              <option value="<="><=</option>
              <option value=">">&gt;</option>
              <option value="<">&lt;</option>
            </select>
            <input v-model.number="checkDraft.expectedCount" class="edit-number" type="number" min="0" @keydown.stop />
          </div>
        </template>

        <template v-else-if="editingCheck.kind === 'no_nulls' || editingCheck.kind === 'column_exists'">
          <label class="edit-label">Column name</label>
          <input v-model="checkDraft.column" class="edit-input" placeholder="e.g. revenue" @keydown.stop />
        </template>

        <template v-else-if="editingCheck.kind === 'column_value'">
          <label class="edit-label">SQL expression (evaluated against student result)</label>
          <input v-model="checkDraft.expression" class="edit-input" placeholder="e.g. SUM(revenue)" @keydown.stop />
          <label class="edit-label">Expected value</label>
          <input v-model="checkDraft.expectedValue" class="edit-input" placeholder="e.g. 12345.00" @keydown.stop />
          <label class="edit-label">Tolerance (for numbers)</label>
          <input v-model.number="checkDraft.tolerance" class="edit-number" type="number" min="0" step="0.001" placeholder="0.001" @keydown.stop />
        </template>

        <template v-else-if="editingCheck.kind === 'sql_pattern'">
          <label class="edit-label">Regex pattern (case-insensitive)</label>
          <input v-model="checkDraft.pattern" class="edit-input" placeholder="e.g. \bJOIN\b" @keydown.stop />
          <div class="edit-inline" style="gap:8px; margin-top: 6px;">
            <label class="edit-label" style="margin:0">Must</label>
            <label class="radio-label">
              <input type="radio" :value="true" v-model="checkDraft.mustMatch" /> match
            </label>
            <label class="radio-label">
              <input type="radio" :value="false" v-model="checkDraft.mustMatch" /> NOT match
            </label>
          </div>
        </template>

        <div class="check-editor-actions">
          <button class="save-check-btn" @click.stop="saveCheck">Save check</button>
          <button class="cancel-check-btn" @click.stop="cancelCheckEdit">Cancel</button>
        </div>
      </div>
    </template>

    <!-- Resize handles -->
    <div class="resize-e" @mousedown.stop="startResize($event, 'e')" />
    <div class="resize-s" @mousedown.stop="startResize($event, 's')" />
    <div class="resize-se" @mousedown.stop="startResize($event, 'se')" />
  </div>
</template>

<style scoped>
.exercise-card {
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface-1);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
  user-select: none;
  cursor: grab;
  display: flex;
  flex-direction: column;
  position: absolute;
  transition: border-color 0.15s, box-shadow 0.15s;
  overflow: hidden;
}
.exercise-card:active { cursor: grabbing; }
.exercise-card.selected { border-color: var(--accent); box-shadow: 0 0 0 2px var(--accent), 0 4px 16px rgba(0,0,0,0.4); }
.exercise-card.all-passed { border-color: #34d399; }
.exercise-card.has-failures { border-color: #f87171; }

/* Header — this is the drag handle */
.card-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 10px;
  border-bottom: 1px solid var(--border);
  background: var(--surface-2);
  flex-shrink: 0;
}
.card-icon { font-size: 13px; flex-shrink: 0; }
.card-name { font-size: 11.5px; font-weight: 600; color: var(--text-primary); flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rename-input { flex: 1; background: transparent; border: none; outline: 1px solid var(--accent); border-radius: 3px; font-size: 11.5px; font-weight: 600; color: var(--text-primary); padding: 0 4px; }
.spacer { flex: 1; }

.mode-btn {
  width: 22px; height: 22px; display: flex; align-items: center; justify-content: center;
  background: transparent; border: none; border-radius: 4px; color: var(--text-muted);
  transition: color 0.15s, background 0.15s;
}
.mode-btn:hover, .mode-btn.active { color: var(--accent); background: color-mix(in srgb, var(--accent) 12%, transparent); }
.mode-btn svg { width: 12px; height: 12px; }

.delete-btn {
  width: 20px; height: 20px; display: flex; align-items: center; justify-content: center;
  background: none; border: none; border-radius: 4px; font-size: 14px;
  color: var(--text-muted); line-height: 1; transition: color 0.15s, background 0.15s;
}
.delete-btn:hover { color: var(--error); }
.delete-btn.confirm { color: var(--error); background: color-mix(in srgb, var(--error) 15%, transparent); }

/* Side-by-side body */
.card-body {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.left-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  border-right: 1px solid var(--border);
  overflow: hidden;
}
.right-panel {
  width: 260px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

/* Results */
.results-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  border-top: 1px solid var(--border);
}
.results-placeholder {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  color: var(--text-muted);
  font-style: italic;
  padding: 12px;
  text-align: center;
}
.results-placeholder kbd {
  font-family: var(--font-mono);
  font-size: 10px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 3px;
  padding: 1px 4px;
  font-style: normal;
}
.results-scroll {
  flex: 1;
  overflow: auto;
  min-height: 0;
}
.mini-table {
  width: max-content;
  min-width: 100%;
  border-collapse: collapse;
  font-size: 11px;
  font-family: var(--font-mono);
}
.mini-table th {
  position: sticky;
  top: 0;
  padding: 4px 10px;
  background: var(--surface-2);
  border-bottom: 1px solid var(--border);
  text-align: left;
  font-weight: 600;
  color: var(--text-muted);
  white-space: nowrap;
  font-size: 10.5px;
  font-family: var(--font-sans, sans-serif);
}
.mini-table td {
  padding: 3px 10px;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
  color: var(--text-primary);
  white-space: nowrap;
  user-select: text;
  max-width: 240px;
  overflow: hidden;
  text-overflow: ellipsis;
}
.mini-table tbody tr:hover td { background: var(--surface-2); }
.results-meta {
  padding: 3px 10px;
  font-size: 10px;
  color: var(--text-muted);
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

/* Prompt */
.prompt-body { padding: 10px 14px 6px; flex-shrink: 0; user-select: text; overflow-y: auto; max-height: 120px; }
.prompt-md { font-size: 12.5px; line-height: 1.6; color: var(--text-primary); }
.prompt-md :deep(h1) { font-size: 14px; font-weight: 700; margin: 0 0 6px; }
.prompt-md :deep(h2) { font-size: 13px; font-weight: 600; margin: 8px 0 4px; }
.prompt-md :deep(p) { margin: 0 0 6px; }
.prompt-md :deep(code) { font-family: var(--font-mono); font-size: 11px; background: var(--surface-2); border: 1px solid var(--border); border-radius: 3px; padding: 1px 4px; }
.prompt-md :deep(ul), .prompt-md :deep(ol) { margin: 4px 0 6px 16px; }
.prompt-md :deep(li) { margin-bottom: 2px; }

.prompt-empty { padding: 10px 14px; font-size: 11.5px; color: var(--text-muted); font-style: italic; }

/* SQL editor section */
.sql-editor-section {
  flex-shrink: 0;
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
  background: var(--surface-0);
  overflow: hidden;
}
.run-error {
  padding: 4px 10px 6px;
  font-size: 11px;
  color: #f87171;
  font-family: var(--font-mono);
  background: rgba(248, 113, 113, 0.07);
  white-space: pre-wrap;
  user-select: text;
}

/* Checks */
.checks-list { flex: 1; min-height: 0; padding: 4px 10px 8px; display: flex; flex-direction: column; gap: 5px; overflow-y: auto; }
.checks-empty { flex: 1; display: flex; align-items: center; justify-content: center; font-size: 11.5px; color: var(--text-muted); font-style: italic; padding: 8px; }

.success-banner {
  flex-shrink: 0;
  padding: 10px 12px;
  margin: 0 8px 6px;
  background: rgba(63, 185, 80, 0.08);
  border: 1px solid rgba(63, 185, 80, 0.25);
  border-radius: 6px;
  font-size: 11.5px;
  color: var(--text-primary);
  overflow-y: auto;
  max-height: 120px;
}
.success-md :deep(p) { margin: 0 0 6px; }
.success-md :deep(p:last-child) { margin-bottom: 0; }
.success-md :deep(code) { background: rgba(63,185,80,0.12); border-radius: 3px; padding: 0 3px; font-size: 10.5px; }
.success-md :deep(a) { color: #3fb950; }

.check-row {
  display: flex; align-items: flex-start; gap: 8px;
  padding: 7px 10px; border-radius: 6px; border: 1px solid var(--border);
  background: var(--surface-0); transition: border-color 0.15s, background 0.15s;
}
.check-row.check-pass { border-color: rgba(52, 211, 153, 0.4); background: rgba(52, 211, 153, 0.07); }
.check-row.check-fail { border-color: rgba(248, 113, 113, 0.4); background: rgba(248, 113, 113, 0.07); }

.check-status { flex-shrink: 0; padding-top: 1px; }
.dot { font-size: 13px; font-weight: 700; line-height: 1; display: block; }
.dot-pass { color: #34d399; }
.dot-fail { color: #f87171; }
.dot-pending { color: var(--text-muted); }

.check-content { flex: 1; display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.check-label { font-size: 12px; font-weight: 500; color: var(--text-primary); user-select: text; }
.check-feedback { font-size: 11px; color: #f87171; line-height: 1.4; user-select: text; }
.check-hint { font-size: 11px; color: #f59e0b; line-height: 1.4; user-select: text; }

/* Footer */
.card-footer {
  display: flex; flex-direction: column; gap: 0;
  border-top: 1px solid var(--border); background: var(--surface-2); flex-shrink: 0;
}

.footer-status {
  display: flex; align-items: center; gap: 6px;
  padding: 5px 10px 4px;
  border-bottom: 1px solid var(--border);
}
.footer-actions {
  display: flex; align-items: center; gap: 6px;
  padding: 6px 10px;
}

.attempt-count { font-size: 10.5px; color: var(--text-muted); }

.verdict-badge { font-size: 10.5px; font-weight: 600; border-radius: 4px; padding: 2px 8px; }
.verdict-pass { color: #34d399; background: rgba(52, 211, 153, 0.12); }
.verdict-fail { color: #f87171; background: rgba(248, 113, 113, 0.12); }

.reset-btn {
  font-size: 10.5px; color: var(--text-muted); background: none; border: none;
  border-radius: 4px; padding: 2px 6px; cursor: pointer; transition: color 0.15s;
}
.reset-btn:hover { color: var(--text-primary); }

.validate-btn {
  padding: 4px 16px; font-size: 11.5px; font-weight: 600;
  color: #fff; background: var(--accent); border: none; border-radius: 5px;
  transition: opacity 0.15s; display: flex; align-items: center; gap: 5px; justify-content: center;
}
.validate-btn:hover:not(:disabled) { opacity: 0.85; }
.validate-btn:disabled { opacity: 0.4; cursor: not-allowed; }

.next-lesson-btn {
  padding: 4px 12px; font-size: 11.5px; font-weight: 600;
  color: #3fb950; background: rgba(63, 185, 80, 0.12);
  border: 1px solid rgba(63, 185, 80, 0.3); border-radius: 5px;
  transition: background 0.15s, border-color 0.15s;
}
.next-lesson-btn:hover { background: rgba(63, 185, 80, 0.2); border-color: rgba(63, 185, 80, 0.5); }
.spinner {
  width: 11px; height: 11px; border: 2px solid rgba(255,255,255,0.3); border-top-color: #fff;
  border-radius: 50%; animation: spin 0.6s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* ── Edit mode ─────────────────────────────────────────────────────────────── */
.edit-section { padding: 8px 14px; border-bottom: 1px solid var(--border); flex-shrink: 0; }
.edit-label { font-size: 10.5px; font-weight: 600; color: var(--text-muted); display: block; margin-bottom: 4px; text-transform: uppercase; letter-spacing: 0.04em; }
.edit-inline { display: flex; align-items: center; gap: 6px; }
.edit-input { width: 100%; background: var(--surface-0); border: 1px solid var(--border); border-radius: 5px; color: var(--text-primary); font-size: 11.5px; padding: 4px 8px; outline: none; }
.edit-input:focus { border-color: var(--accent); }
.edit-textarea { width: 100%; background: var(--surface-0); border: 1px solid var(--border); border-radius: 5px; color: var(--text-primary); font-size: 11px; font-family: var(--font-mono); padding: 6px 8px; outline: none; resize: vertical; box-sizing: border-box; }
.edit-textarea:focus { border-color: var(--accent); }
.edit-select { background: var(--surface-0); border: 1px solid var(--border); border-radius: 5px; color: var(--text-primary); font-size: 11.5px; padding: 4px 6px; }
.edit-select-sm { background: var(--surface-0); border: 1px solid var(--border); border-radius: 5px; color: var(--text-primary); font-size: 11px; padding: 3px 4px; }
.edit-number { width: 70px; background: var(--surface-0); border: 1px solid var(--border); border-radius: 5px; color: var(--text-primary); font-size: 11.5px; padding: 4px 6px; }
.edit-unit { font-size: 11px; color: var(--text-muted); }
.next-select { flex: 1; min-width: 0; }

.edit-checks-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.add-kind-select { background: var(--surface-0); border: 1px solid var(--border); border-radius: 5px; color: var(--text-primary); font-size: 11px; padding: 3px 6px; }

.edit-check-row { background: var(--surface-0); border: 1px solid var(--border); border-radius: 6px; padding: 6px 8px; margin-bottom: 4px; }
.edit-check-summary { display: flex; align-items: center; gap: 6px; }
.check-kind-chip { font-size: 9.5px; font-weight: 700; background: var(--surface-2); border: 1px solid var(--border); border-radius: 3px; padding: 1px 5px; color: var(--text-muted); font-family: var(--font-mono); flex-shrink: 0; }
.edit-check-label { font-size: 11.5px; color: var(--text-primary); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.ec-btn { width: 20px; height: 20px; display: flex; align-items: center; justify-content: center; background: none; border: none; border-radius: 3px; font-size: 11px; color: var(--text-muted); transition: color 0.1s, background 0.1s; }
.ec-btn:hover:not(:disabled) { color: var(--text-primary); background: var(--surface-2); }
.ec-btn:disabled { opacity: 0.3; cursor: not-allowed; }
.ec-del:hover { color: var(--error) !important; }

.check-editor {
  padding: 12px 14px; background: var(--surface-0); border-bottom: 1px solid var(--border);
  display: flex; flex-direction: column; gap: 6px; flex-shrink: 0;
}
.check-editor-title { font-size: 11px; font-weight: 700; color: var(--accent); text-transform: uppercase; letter-spacing: 0.04em; margin-bottom: 4px; }
.check-editor-actions { display: flex; gap: 6px; margin-top: 4px; }
.save-check-btn { padding: 4px 12px; font-size: 11.5px; background: var(--accent); color: #fff; border: none; border-radius: 5px; font-weight: 600; }
.save-check-btn:hover { opacity: 0.85; }
.cancel-check-btn { padding: 4px 10px; font-size: 11.5px; background: var(--surface-2); color: var(--text-secondary); border: 1px solid var(--border); border-radius: 5px; }
.cancel-check-btn:hover { color: var(--text-primary); }

.radio-label { display: flex; align-items: center; gap: 4px; font-size: 11.5px; color: var(--text-primary); cursor: pointer; }

/* Resize handles */
.resize-e  { position: absolute; right: 0; top: 8px; bottom: 8px; width: 5px; cursor: ew-resize; }
.resize-s  { position: absolute; bottom: 0; left: 8px; right: 8px; height: 5px; cursor: ns-resize; }
.resize-se { position: absolute; right: 0; bottom: 0; width: 12px; height: 12px; cursor: se-resize; }
</style>
