<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { useSchemaStore } from '../stores/schema'
import type { IngestNode } from '../stores/schema'
import SqlEditor from './SqlEditor.vue'
import NodeColorPicker from './NodeColorPicker.vue'
import { useSchemaCompletions } from '../composables/useSchemaCompletions'
import { useDuckDB } from '../composables/useDuckDB'
import { IS_DESKTOP } from '../lib/env'

const props = defineProps<{ node: IngestNode; selected?: boolean }>()
const emit = defineEmits<{
  dragStart: [{ id: string; mouseX: number; mouseY: number; shiftKey: boolean }]
  resizeStart: [{ id: string; mouseX: number; mouseY: number; startW: number; startH: number; direction: 'e' | 's' | 'se' }]
}>()

const schemaStore = useSchemaStore()
const { sqlSchema } = useSchemaCompletions()
const { query } = useDuckDB()

// ── Drag / resize ─────────────────────────────────────────────────────────────
function onMouseDown(e: MouseEvent) {
  const t = e.target as HTMLElement
  if (t.closest('input,textarea,.cm-editor,select,button')) return
  emit('dragStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY, shiftKey: e.shiftKey })
}

const DEFAULT_W = 340

function onResizeStart(e: MouseEvent, direction: 'e' | 's' | 'se') {
  emit('resizeStart', { id: props.node.id, mouseX: e.clientX, mouseY: e.clientY, startW: props.node.w ?? DEFAULT_W, startH: props.node.h ?? 200, direction })
}

// ── Rename ─────────────────────────────────────────────────────────────────────
const isRenaming = ref(false)
const renameValue = ref('')
const renameInputRef = ref<HTMLInputElement | null>(null)

function startRename() {
  renameValue.value = props.node.name
  isRenaming.value = true
  setTimeout(() => renameInputRef.value?.select(), 0)
}

function commitRename() {
  const v = renameValue.value.trim()
  if (v && v !== props.node.name) schemaStore.updateIngestNode(props.node.id, { name: v })
  isRenaming.value = false
}

function onRenameKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') commitRename()
  if (e.key === 'Escape') isRenaming.value = false
}

// ── Delete ────────────────────────────────────────────────────────────────────
const deleteConfirm = ref(false)
let deleteTimer: ReturnType<typeof setTimeout> | null = null

function onDeleteClick() {
  if (!deleteConfirm.value) {
    deleteConfirm.value = true
    deleteTimer = setTimeout(() => { deleteConfirm.value = false }, 3000)
  } else {
    if (deleteTimer) clearTimeout(deleteTimer)
    schemaStore.removeNode(props.node.id)
  }
}

// ── Collapse ──────────────────────────────────────────────────────────────────
const isCollapsed = computed(() => props.node.viewMode === 'collapsed')
function toggleCollapsed() {
  schemaStore.updateViewMode(props.node.id, isCollapsed.value ? 'default' : 'collapsed')
}

// ── Local editable state ──────────────────────────────────────────────────────
const localSql = ref(props.node.sql)
const localUrl = ref(props.node.url)
const localTarget = ref(props.node.targetTable)

watch(() => props.node.sql, v => { if (v !== localSql.value) localSql.value = v })
watch(() => props.node.url, v => { if (v !== localUrl.value) localUrl.value = v })
watch(() => props.node.targetTable, v => { if (v !== localTarget.value) localTarget.value = v })

let saveTimer: ReturnType<typeof setTimeout> | null = null
function scheduleFieldSave() {
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    schemaStore.updateIngestNode(props.node.id, {
      sql: localSql.value,
      url: localUrl.value,
      targetTable: localTarget.value,
    })
  }, 400)
}

function onSqlChange(v: string) { localSql.value = v; scheduleFieldSave() }

// ── Execution ─────────────────────────────────────────────────────────────────
const isRunning = ref(false)
const runError = ref<string | null>(null)
const lastRowsAdded = ref(props.node.lastRowsAdded ?? 0)
const lastRunAt = ref(props.node.lastRunAt ?? 0)

const lastRunLabel = computed(() => {
  if (!lastRunAt.value) return null
  const sec = Math.floor((Date.now() - lastRunAt.value) / 1000)
  if (sec < 60) return `${sec}s ago`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`
  return `${Math.floor(sec / 3600)}h ago`
})

async function run() {
  if (isRunning.value) return
  isRunning.value = true
  runError.value = null

  try {
    let rows = 0
    if (props.node.mode === 'generator') {
      const sql = localSql.value.trim()
      if (!sql) return
      const result = await query(sql)
      // DuckDB returns affected row count as first column of first row for DML
      const firstVal = result.rows[0] ? Object.values(result.rows[0])[0] : null
      rows = typeof firstVal === 'number' ? firstVal : (result.rowCount ?? 1)
    } else {
      if (!localUrl.value.trim() || !localTarget.value.trim()) return
      if (!IS_DESKTOP) {
        runError.value = 'URL ingestion requires the desktop app'
        return
      }
      const { IngestFromUrl } = await import('../../wailsjs/go/main/App')
      rows = await IngestFromUrl(localUrl.value.trim(), localTarget.value.trim(), props.node.conflictMode)
    }

    lastRowsAdded.value = rows
    lastRunAt.value = Date.now()
    schemaStore.updateIngestNode(props.node.id, { lastRowsAdded: rows, lastRunAt: lastRunAt.value })
  } catch (err) {
    runError.value = err instanceof Error ? err.message : String(err)
  } finally {
    isRunning.value = false
  }
}

// ── Interval / countdown ──────────────────────────────────────────────────────
const INTERVAL_OPTIONS = [0, 5, 30, 60, 300, 1800, 3600]

function intervalLabel(sec: number): string {
  if (sec === 0) return 'Manual'
  if (sec < 1) return `${Math.round(sec * 1000)}ms`
  if (sec < 60) return `${sec}s`
  if (sec < 3600) return `${sec / 60}m`
  return `${sec / 3600}h`
}

// custom ms input
const showCustomMs = ref(false)
const customMsInput = ref<HTMLInputElement | null>(null)
const customMsValue = ref('')

const selectValue = computed(() =>
  INTERVAL_OPTIONS.includes(props.node.interval) ? props.node.interval : 'custom'
)

function onIntervalChange(e: Event) {
  const v = (e.target as HTMLSelectElement).value
  if (v === 'custom') {
    customMsValue.value = String(Math.round(props.node.interval * 1000) || 250)
    showCustomMs.value = true
    setTimeout(() => { customMsInput.value?.select() }, 0)
  } else {
    showCustomMs.value = false
    setInterval_(+v)
  }
}

function commitCustomMs() {
  const ms = parseInt(customMsValue.value, 10)
  if (!isNaN(ms) && ms > 0) setInterval_(ms / 1000)
  showCustomMs.value = false
}

function onCustomMsKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') commitCustomMs()
  if (e.key === 'Escape') showCustomMs.value = false
}

const intervalTimer = ref<ReturnType<typeof setInterval> | null>(null)
const _nextRunAt = ref(0)
const countdown = ref(0)
let _countdownTimer: ReturnType<typeof setInterval> | null = null

function _tickCountdown() {
  countdown.value = Math.max(0, Math.ceil((_nextRunAt.value - Date.now()) / 1000))
}

function startTimer() {
  if (intervalTimer.value) { clearInterval(intervalTimer.value); intervalTimer.value = null }
  if (_countdownTimer) { clearInterval(_countdownTimer); _countdownTimer = null }
  const ms = props.node.interval * 1000
  if (ms > 0) {
    _nextRunAt.value = Date.now() + ms
    _tickCountdown()
    intervalTimer.value = setInterval(() => { _nextRunAt.value = Date.now() + ms; run() }, ms)
    _countdownTimer = setInterval(_tickCountdown, 1000)
  } else {
    countdown.value = 0
  }
}

const countdownLabel = computed(() => {
  const s = countdown.value
  if (s <= 0) return props.node.interval > 0 && props.node.interval < 1 ? `${Math.round(props.node.interval * 1000)}ms` : '…'
  if (s < 60) return `${s}s`
  return `${Math.ceil(s / 60)}m`
})

function setInterval_(sec: number) {
  schemaStore.updateIngestNode(props.node.id, { interval: sec })
  startTimer()
}

watch(() => props.node.interval, startTimer, { immediate: true })

onUnmounted(() => {
  if (intervalTimer.value) clearInterval(intervalTimer.value)
  if (_countdownTimer) clearInterval(_countdownTimer)
})
</script>

<template>
  <div
    class="ingest-card"
    :class="{ selected }"
    :data-node-id="node.id"
    :style="{ left: `${node.x}px`, top: `${node.y}px`, width: `${node.w ?? DEFAULT_W}px` }"
    @mousedown="onMouseDown"
  >
    <!-- Header -->
    <div class="card-header" :style="{ background: node.color }">
      <!-- Pipe/funnel icon -->
      <svg class="card-icon" viewBox="0 0 16 16" fill="none">
        <path d="M2 3h12l-4.5 5v4l-3-1.5V8L2 3z" stroke="white" stroke-width="1.3" stroke-linejoin="round"/>
        <circle cx="13" cy="12" r="2.2" fill="white" fill-opacity="0.25" stroke="white" stroke-width="1.2"/>
        <path d="M12 12h2M13 11v2" stroke="white" stroke-width="1.2" stroke-linecap="round"/>
      </svg>

      <input
        v-if="isRenaming"
        ref="renameInputRef"
        v-model="renameValue"
        class="card-name-input"
        @mousedown.stop
        @keydown="onRenameKeydown"
        @blur="commitRename"
      />
      <span v-else class="card-name" title="Double-click to rename" @mousedown.stop @dblclick="startRename">
        {{ node.name }}
      </span>

      <span class="mode-badge">{{ node.mode === 'generator' ? 'Gen' : 'Ingest' }}</span>

      <NodeColorPicker :color="node.color" @pick="schemaStore.setNodeColor(node.id, $event)" />

      <button class="collapse-btn" :title="isCollapsed ? 'Expand' : 'Collapse'" @mousedown.stop @click.stop="toggleCollapsed">
        <svg viewBox="0 0 10 10" fill="none">
          <path v-if="!isCollapsed" d="M2 3.5l3 3 3-3" stroke="white" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
          <path v-else d="M2 6.5l3-3 3 3" stroke="white" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>

      <button
        class="delete-btn"
        :class="{ confirming: deleteConfirm }"
        :title="deleteConfirm ? 'Click again to confirm' : 'Delete node'"
        @mousedown.stop
        @click.stop="onDeleteClick"
      >
        <span v-if="deleteConfirm" class="confirm-label">Delete?</span>
        <svg v-else viewBox="0 0 12 12" fill="none">
          <path d="M2 3.5h8" stroke="white" stroke-width="1.2" stroke-linecap="round"/>
          <path d="M4.5 3.5V2.5h3v1" stroke="white" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M3.5 3.5l.7 6h3.6l.7-6" stroke="white" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>
    </div>

    <!-- Body -->
    <template v-if="!isCollapsed">
      <!-- Mode tabs -->
      <div class="mode-tabs" @mousedown.stop>
        <button
          class="mode-tab"
          :class="{ active: node.mode === 'generator' }"
          @click.stop="schemaStore.updateIngestNode(node.id, { mode: 'generator' })"
        >Generator</button>
        <button
          class="mode-tab"
          :class="{ active: node.mode === 'ingestion' }"
          @click.stop="schemaStore.updateIngestNode(node.id, { mode: 'ingestion' })"
        >Ingestion</button>
      </div>

      <!-- Generator body -->
      <div v-if="node.mode === 'generator'" class="editor-wrap" @mousedown.stop>
        <SqlEditor
          v-model="localSql"
          :schema="sqlSchema"
          placeholder="INSERT INTO my_table VALUES (now(), random() * 100)"
          @update:model-value="onSqlChange"
        />
      </div>

      <!-- Ingestion body -->
      <div v-else class="ingestion-body" @mousedown.stop>
        <label class="field-label">Source URL</label>
        <input
          v-model="localUrl"
          class="field-input"
          placeholder="https://example.com/data.csv"
          spellcheck="false"
          @input="scheduleFieldSave"
        />
        <label class="field-label">Target table</label>
        <input
          v-model="localTarget"
          class="field-input"
          placeholder="my_table"
          spellcheck="false"
          @input="scheduleFieldSave"
        />
        <label class="field-label">On run</label>
        <select
          class="field-select"
          :value="node.conflictMode"
          @change="schemaStore.updateIngestNode(node.id, { conflictMode: ($event.target as HTMLSelectElement).value as 'append' | 'replace' })"
        >
          <option value="append">Append new rows</option>
          <option value="replace">Replace table</option>
        </select>
      </div>

      <!-- Error -->
      <div v-if="runError" class="run-error" @mousedown.stop>
        {{ runError }}
        <button @click.stop="runError = null">✕</button>
      </div>
    </template>

    <!-- Footer -->
    <div class="card-footer" @mousedown.stop>
      <span class="footer-status">
        <template v-if="lastRunAt">
          <span class="status-ok">+{{ lastRowsAdded }} rows · {{ lastRunLabel }}</span>
        </template>
        <span v-else class="status-hint">not yet run</span>
      </span>

      <span v-if="node.interval > 0" class="run-countdown">↻ {{ countdownLabel }}</span>

      <template v-if="showCustomMs">
        <input
          ref="customMsInput"
          v-model="customMsValue"
          class="custom-ms-input"
          type="number"
          min="1"
          placeholder="ms"
          @mousedown.stop
          @keydown="onCustomMsKeydown"
          @blur="commitCustomMs"
        />
      </template>

      <select
        class="interval-select"
        :class="{ 'interval-active': node.interval > 0 }"
        :value="selectValue"
        @change.stop="onIntervalChange"
      >
        <option v-for="sec in INTERVAL_OPTIONS" :key="sec" :value="sec">{{ intervalLabel(sec) }}</option>
        <option v-if="!INTERVAL_OPTIONS.includes(node.interval) && node.interval > 0" :value="node.interval">
          {{ intervalLabel(node.interval) }}
        </option>
        <option value="custom">Custom ms…</option>
      </select>

      <button class="run-btn" :disabled="isRunning" @click.stop="run">
        <svg viewBox="0 0 10 10" fill="none" :class="{ spinning: isRunning }">
          <path v-if="!isRunning" d="M3 2l5 3-5 3V2z" fill="currentColor"/>
          <path v-else d="M9 5A4 4 0 1 1 5 1" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
        </svg>
        {{ isRunning ? 'Running…' : 'Run' }}
      </button>
    </div>

    <!-- Resize handles -->
    <template v-if="!isCollapsed">
      <div class="rh-e" @mousedown.stop="onResizeStart($event, 'e')" />
      <div class="rh-s" @mousedown.stop="onResizeStart($event, 's')" />
      <div class="rh-se" @mousedown.stop="onResizeStart($event, 'se')" />
    </template>
  </div>
</template>

<style scoped>
.ingest-card {
  position: absolute;
  background: var(--surface-1);
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  box-shadow: 0 2px 8px rgba(0,0,0,0.18);
  min-width: 200px;
}
.ingest-card.selected { border-color: var(--accent); box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 30%, transparent); }

/* ── Header ── */
.card-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 8px;
  height: 32px;
  flex-shrink: 0;
  border-radius: 7px 7px 0 0;
}
.card-icon { width: 14px; height: 14px; flex-shrink: 0; }
.card-name {
  flex: 1;
  font-size: 12px;
  font-weight: 600;
  color: white;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: default;
  min-width: 0;
}
.card-name-input {
  flex: 1;
  background: rgba(255,255,255,0.2);
  border: 1px solid rgba(255,255,255,0.4);
  border-radius: 3px;
  color: white;
  font-size: 12px;
  font-weight: 600;
  padding: 1px 4px;
  outline: none;
  min-width: 0;
}
.mode-badge {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: rgba(255,255,255,0.75);
  background: rgba(0,0,0,0.2);
  border-radius: 3px;
  padding: 1px 4px;
  flex-shrink: 0;
}
.collapse-btn, .delete-btn {
  background: none;
  border: none;
  cursor: pointer;
  padding: 2px;
  display: flex;
  align-items: center;
  opacity: 0.7;
  flex-shrink: 0;
}
.collapse-btn:hover, .delete-btn:hover { opacity: 1; }
.collapse-btn svg { width: 10px; height: 10px; }
.delete-btn svg { width: 12px; height: 12px; }
.delete-btn.confirming { opacity: 1; }
.confirm-label { font-size: 10px; color: white; font-weight: 600; white-space: nowrap; }

/* ── Mode tabs ── */
.mode-tabs {
  display: flex;
  border-bottom: 1px solid var(--border);
  background: var(--surface-2);
}
.mode-tab {
  flex: 1;
  padding: 5px 0;
  font-size: 11px;
  font-weight: 500;
  color: var(--text-secondary);
  background: none;
  border: none;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: color 0.1s, border-color 0.1s;
}
.mode-tab.active { color: var(--accent); border-bottom-color: var(--accent); }
.mode-tab:hover:not(.active) { color: var(--text-primary); }

/* ── Generator editor ── */
.editor-wrap { min-height: 80px; }

/* ── Ingestion body ── */
.ingestion-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px;
}
.field-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-top: 2px;
}
.field-label:first-child { margin-top: 0; }
.field-input {
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 12px;
  font-family: var(--font-mono);
  padding: 4px 6px;
  outline: none;
  width: 100%;
}
.field-input:focus { border-color: var(--accent); }
.field-select {
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 11px;
  padding: 3px 6px;
  outline: none;
}

/* ── Error ── */
.run-error {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  padding: 6px 8px;
  background: color-mix(in srgb, var(--error) 10%, transparent);
  border-top: 1px solid color-mix(in srgb, var(--error) 30%, transparent);
  font-size: 11px;
  color: var(--error);
  font-family: var(--font-mono);
}
.run-error button { background: none; border: none; cursor: pointer; color: var(--error); margin-left: auto; flex-shrink: 0; }

/* ── Footer ── */
.card-footer {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border-top: 1px solid var(--border);
  background: var(--surface-2);
  min-height: 30px;
  flex-shrink: 0;
}
.footer-status { flex: 1; font-size: 10px; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.status-ok { color: var(--success); }
.status-hint { color: var(--text-muted); }
.run-countdown { font-size: 10px; font-family: var(--font-mono); color: var(--success); flex-shrink: 0; opacity: 0.8; }
.custom-ms-input {
  width: 56px;
  padding: 2px 4px;
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--accent);
  background: color-mix(in srgb, var(--accent) 8%, var(--surface-2));
  border: 1px solid var(--accent);
  border-radius: 4px;
  outline: none;
  flex-shrink: 0;
}
.custom-ms-input::-webkit-inner-spin-button,
.custom-ms-input::-webkit-outer-spin-button { opacity: 0.4; }
.interval-select {
  padding: 2px 4px;
  font-size: 10px;
  color: var(--text-muted);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  cursor: pointer;
  outline: none;
  flex-shrink: 0;
}
.interval-select.interval-active {
  color: var(--success);
  border-color: rgba(63, 185, 80, 0.4);
  background: rgba(63, 185, 80, 0.08);
}
.run-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  background: var(--accent);
  color: white;
  border: none;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  flex-shrink: 0;
  transition: opacity 0.1s;
}
.run-btn:hover:not(:disabled) { opacity: 0.85; }
.run-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.run-btn svg { width: 10px; height: 10px; }

@keyframes spin { to { transform: rotate(360deg); } }
.spinning { animation: spin 0.8s linear infinite; }

/* ── Resize handles ── */
.rh-e  { position: absolute; right: 0; top: 8px; bottom: 20px; width: 6px; cursor: ew-resize; border-radius: 0 4px 4px 0; }
.rh-s  { position: absolute; bottom: 0; left: 8px; right: 8px; height: 6px; cursor: ns-resize; }
.rh-se { position: absolute; right: 0; bottom: 0; width: 14px; height: 14px; cursor: se-resize; }
.rh-e:hover, .rh-s:hover, .rh-se:hover { background: var(--accent); opacity: 0.4; }
.ingest-card.selected .rh-e,
.ingest-card.selected .rh-s,
.ingest-card.selected .rh-se { opacity: 1; }
</style>
