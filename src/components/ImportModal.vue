<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useDuckDB } from '../composables/useDuckDB'
import { useSchemaStore } from '../stores/schema'

const emit = defineEmits<{ close: [] }>()

const { registerFile, dropFile, exec, query, getTableInfo } = useDuckDB()
const schemaStore = useSchemaStore()

type Status = 'queued' | 'processing' | 'done' | 'error'

interface ImportItem {
  key: string
  file: File
  tableName: string
  status: Status
  rowCount?: number
  error?: string
}

const items = ref<ImportItem[]>([])
const isDragOver = ref(false)
const isProcessing = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)

// ── Name helpers ──────────────────────────────────────────────────────────────

function sanitize(filename: string): string {
  return filename
    .replace(/\.[^.]+$/, '')
    .replace(/[^a-zA-Z0-9_]/g, '_')
    .replace(/^(\d)/, '_$1')
    .replace(/_+/g, '_')
    .replace(/^_|_$/g, '')
    .toLowerCase() || 'imported_table'
}

function uniqueName(base: string): string {
  // Check against both the store AND any names already queued in this session
  const taken = new Set([
    ...schemaStore.tables.map((t) => t.name),
    ...items.value.map((i) => i.tableName),
  ])
  if (!taken.has(base)) return base
  let n = 1
  while (taken.has(`${base}_${n}`)) n++
  return `${base}_${n}`
}

// ── Canvas positioning ────────────────────────────────────────────────────────

function nextPosition(): { x: number; y: number } {
  if (!schemaStore.tables.length) return { x: 60, y: 80 }
  // Place to the right of whatever is furthest right at the moment of insertion
  const maxX = Math.max(...schemaStore.tables.map((t) => t.x))
  const anchor = schemaStore.tables.find((t) => t.x === maxX)!
  return { x: maxX + 280, y: anchor.y }
}

// ── Processing ────────────────────────────────────────────────────────────────

async function processItem(item: ImportItem) {
  item.status = 'processing'
  try {
    const buffer = await item.file.arrayBuffer()
    await registerFile(item.file.name, new Uint8Array(buffer))

    const safeFile  = item.file.name.replace(/'/g, "''")
    const safeTable = item.tableName.replace(/"/g, '""')

    // sample_size=-1 scans the whole file so DuckDB infers types correctly on
    // skewed data (e.g. a column that's NULL for the first 100 rows)
    await exec(
      `CREATE TABLE "${safeTable}" AS ` +
      `SELECT * FROM read_csv_auto('${safeFile}', header = true, sample_size = -1)`,
    )

    const [columns, countResult] = await Promise.all([
      getTableInfo(item.tableName),
      query(`SELECT COUNT(*) AS n FROM "${safeTable}"`),
    ])

    const rowCount = Number(countResult.rows[0]?.n ?? 0)
    const { x, y } = nextPosition()
    schemaStore.addTable({ id: item.tableName, name: item.tableName, x, y, columns })
    schemaStore.setRowCount(item.tableName, rowCount)

    item.rowCount = rowCount
    item.status = 'done'
  } catch (e) {
    item.status = 'error'
    item.error = e instanceof Error ? e.message : String(e)
  } finally {
    await dropFile(item.file.name)
  }
}

async function enqueue(files: File[]) {
  const newItems: ImportItem[] = files.map((file) => ({
    key: `${file.name}-${Math.random().toString(36).slice(2)}`,
    file,
    tableName: uniqueName(sanitize(file.name)),
    status: 'queued' as Status,
  }))

  items.value.push(...newItems)

  if (!isProcessing.value) {
    isProcessing.value = true
    // Process the whole outstanding queue sequentially
    for (const item of items.value.filter((i) => i.status === 'queued')) {
      await processItem(item)
    }
    isProcessing.value = false
  }
}

// ── File input / drag-and-drop ────────────────────────────────────────────────

function onFileInputChange(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files?.length) enqueue(Array.from(input.files))
  input.value = ''
}

function onDragOver(e: DragEvent) {
  e.preventDefault()
  isDragOver.value = true
}

function onDragLeave(e: DragEvent) {
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  if (
    e.clientX < rect.left || e.clientX >= rect.right ||
    e.clientY < rect.top  || e.clientY >= rect.bottom
  ) isDragOver.value = false
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  isDragOver.value = false
  const files = Array.from(e.dataTransfer?.files ?? []).filter((f) =>
    /\.(csv|tsv|txt)$/i.test(f.name),
  )
  if (files.length) enqueue(files)
}

// ── Keyboard / outside-click close ───────────────────────────────────────────

function onBackdropClick(e: MouseEvent) {
  if (e.target === e.currentTarget && !isProcessing.value) emit('close')
}

function onKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape' && !isProcessing.value) emit('close')
}

onMounted(() => window.addEventListener('keydown', onKeyDown))
onUnmounted(() => window.removeEventListener('keydown', onKeyDown))

// ── Computed ──────────────────────────────────────────────────────────────────

const allSettled = computed(
  () => items.value.length > 0 && items.value.every((i) => i.status === 'done' || i.status === 'error'),
)

const doneCount  = computed(() => items.value.filter((i) => i.status === 'done').length)
const errorCount = computed(() => items.value.filter((i) => i.status === 'error').length)

function fmtRows(n: number) {
  return n.toLocaleString() + (n === 1 ? ' row' : ' rows')
}

function fmtSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 ** 2).toFixed(1)} MB`
}
</script>

<template>
  <div class="modal-backdrop" @click="onBackdropClick">
    <div class="modal">
      <!-- Header -->
      <div class="modal-header">
        <div class="modal-title">
          <svg viewBox="0 0 16 16" fill="none">
            <path d="M8 2v8M5 7l3 3 3-3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M3 11v1a1 1 0 001 1h8a1 1 0 001-1v-1" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          </svg>
          Import CSV
        </div>
        <button
          class="close-btn"
          :disabled="isProcessing"
          title="Close"
          @click="emit('close')"
        >
          <svg viewBox="0 0 12 12" fill="none">
            <path d="M1 1l10 10M11 1L1 11" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <!-- Drop zone -->
      <div
        class="drop-zone"
        :class="{ 'drag-over': isDragOver }"
        @dragover="onDragOver"
        @dragleave="onDragLeave"
        @drop="onDrop"
        @click="fileInputRef?.click()"
      >
        <input
          ref="fileInputRef"
          type="file"
          accept=".csv,.tsv,.txt,text/csv,text/tab-separated-values"
          multiple
          hidden
          @change="onFileInputChange"
        />

        <svg class="drop-icon" viewBox="0 0 32 32" fill="none">
          <rect x="4" y="6" width="24" height="20" rx="3" stroke="currentColor" stroke-width="1.5"/>
          <path d="M4 12h24" stroke="currentColor" stroke-width="1.5"/>
          <path d="M10 18h5M10 22h8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          <circle cx="24" cy="22" r="5" fill="var(--surface-1)" stroke="currentColor" stroke-width="1.5"/>
          <path d="M24 19v3M24 22l-2-2M24 22l2-2" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>

        <p class="drop-primary">
          {{ isDragOver ? 'Release to import' : 'Drop CSV files here' }}
        </p>
        <p class="drop-secondary">
          or <span class="browse-link">browse files</span> &nbsp;·&nbsp; .csv .tsv .txt
        </p>
      </div>

      <!-- File list -->
      <div v-if="items.length" class="file-list">
        <div
          v-for="item in items"
          :key="item.key"
          class="file-item"
          :class="item.status"
        >
          <!-- Status icon -->
          <div class="file-status-icon">
            <svg v-if="item.status === 'queued'" viewBox="0 0 14 14" fill="none">
              <circle cx="7" cy="7" r="5.5" stroke="currentColor" stroke-width="1.3"/>
            </svg>
            <svg v-else-if="item.status === 'processing'" class="spin" viewBox="0 0 14 14" fill="none">
              <circle cx="7" cy="7" r="5" stroke="currentColor" stroke-width="2" stroke-dasharray="10 18" stroke-linecap="round"/>
            </svg>
            <svg v-else-if="item.status === 'done'" viewBox="0 0 14 14" fill="none">
              <circle cx="7" cy="7" r="5.5" stroke="currentColor" stroke-width="1.3"/>
              <path d="M4.5 7l2 2 3-3" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <svg v-else viewBox="0 0 14 14" fill="none">
              <circle cx="7" cy="7" r="5.5" stroke="currentColor" stroke-width="1.3"/>
              <path d="M7 4.5v3M7 9.5v.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            </svg>
          </div>

          <!-- File info -->
          <div class="file-info">
            <div class="file-names">
              <span class="file-original">{{ item.file.name }}</span>
              <span class="file-arrow">→</span>
              <span class="file-table">{{ item.tableName }}</span>
            </div>
            <div class="file-meta">
              <span class="file-size">{{ fmtSize(item.file.size) }}</span>
              <template v-if="item.status === 'processing'">
                <span class="file-progress">Importing…</span>
              </template>
              <template v-else-if="item.status === 'done'">
                <span class="file-rows-count">{{ fmtRows(item.rowCount!) }}</span>
              </template>
              <template v-else-if="item.status === 'error'">
                <span class="file-error-inline" :title="item.error">
                  {{ item.error?.split('\n')[0] }}
                </span>
              </template>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="modal-footer">
        <span v-if="allSettled" class="settle-summary">
          <template v-if="errorCount === 0">
            {{ doneCount }} {{ doneCount === 1 ? 'table' : 'tables' }} imported
          </template>
          <template v-else>
            {{ doneCount }} imported · <span class="err-count">{{ errorCount }} failed</span>
          </template>
        </span>
        <span v-else-if="isProcessing" class="settle-summary">
          Importing {{ items.filter(i => i.status !== 'done' && i.status !== 'error').length }} remaining…
        </span>
        <span v-else class="settle-summary hint">DuckDB will auto-detect column types</span>

        <button
          v-if="!allSettled"
          class="footer-btn secondary"
          :disabled="isProcessing"
          @click="emit('close')"
        >Cancel</button>
        <button
          v-else
          class="footer-btn primary"
          @click="emit('close')"
        >Done</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 200;
  backdrop-filter: blur(2px);
}

.modal {
  width: 520px;
  max-width: calc(100vw - 32px);
  max-height: calc(100vh - 64px);
  display: flex;
  flex-direction: column;
  background: var(--surface-1);
  border: 1px solid var(--border);
  border-radius: 10px;
  box-shadow: 0 24px 48px rgba(0, 0, 0, 0.5);
  overflow: hidden;
}

/* ── Header ──────────────────────────────────────────────────────────────── */
.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.modal-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.modal-title svg {
  width: 16px;
  height: 16px;
  color: var(--accent);
}

.close-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}

.close-btn:hover:not(:disabled) {
  background: var(--surface-2);
  color: var(--text-primary);
}

.close-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.close-btn svg {
  width: 10px;
  height: 10px;
}

/* ── Drop zone ───────────────────────────────────────────────────────────── */
.drop-zone {
  margin: 16px;
  flex-shrink: 0;
  border: 1.5px dashed var(--border);
  border-radius: 8px;
  padding: 28px 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}

.drop-zone:hover,
.drop-zone.drag-over {
  border-color: var(--accent);
  background: rgba(88, 166, 255, 0.05);
}

.drop-zone.drag-over {
  background: rgba(88, 166, 255, 0.1);
}

.drop-icon {
  width: 36px;
  height: 36px;
  color: var(--text-muted);
  margin-bottom: 4px;
}

.drop-primary {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--text-primary);
}

.drop-secondary {
  font-size: 12px;
  color: var(--text-muted);
}

.browse-link {
  color: var(--accent);
  text-decoration: underline;
  text-underline-offset: 2px;
}

/* ── File list ───────────────────────────────────────────────────────────── */
.file-list {
  flex: 1;
  overflow-y: auto;
  border-top: 1px solid var(--border);
  min-height: 0;
}

.file-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--surface-2);
  transition: background 0.1s;
}

.file-item:last-child {
  border-bottom: none;
}

/* Status colors */
.file-status-icon {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.file-status-icon svg {
  width: 14px;
  height: 14px;
}

.file-item.queued    .file-status-icon { color: var(--text-muted); }
.file-item.processing .file-status-icon { color: var(--accent); }
.file-item.done      .file-status-icon { color: var(--success); }
.file-item.error     .file-status-icon { color: var(--error); }

.file-info {
  flex: 1;
  min-width: 0;
}

.file-names {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  margin-bottom: 2px;
  overflow: hidden;
}

.file-original {
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex-shrink: 1;
  min-width: 0;
}

.file-arrow {
  color: var(--text-muted);
  flex-shrink: 0;
  font-size: 11px;
}

.file-table {
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: var(--text-primary);
  flex-shrink: 0;
}

.file-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
}

.file-size {
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.file-progress {
  color: var(--accent);
  font-style: italic;
}

.file-rows-count {
  color: var(--success);
  font-family: var(--font-mono);
}

.file-error-inline {
  color: var(--error);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 280px;
  display: block;
}

/* ── Footer ──────────────────────────────────────────────────────────────── */
.modal-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

.settle-summary {
  flex: 1;
  font-size: 12px;
  color: var(--text-secondary);
}

.settle-summary.hint {
  color: var(--text-muted);
  font-style: italic;
}

.err-count {
  color: var(--error);
}

.footer-btn {
  padding: 6px 16px;
  font-size: 12.5px;
  font-weight: 500;
  border-radius: 5px;
  border: 1px solid var(--border);
  cursor: pointer;
  transition: background 0.15s, color 0.15s, opacity 0.15s;
}

.footer-btn.secondary {
  background: transparent;
  color: var(--text-secondary);
}

.footer-btn.secondary:hover:not(:disabled) {
  background: var(--surface-2);
  color: var(--text-primary);
}

.footer-btn.secondary:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.footer-btn.primary {
  background: var(--accent);
  color: #0d1117;
  border-color: transparent;
  font-weight: 600;
}

.footer-btn.primary:hover {
  background: var(--accent-hover);
}

/* ── Animations ──────────────────────────────────────────────────────────── */
.spin {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
