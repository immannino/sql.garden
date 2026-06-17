<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useDuckDB } from '../composables/useDuckDB'
import { useSchemaStore } from '../stores/schema'
import { usePersistence } from '../composables/usePersistence'
import { IS_DESKTOP } from '../lib/env'

const props = defineProps<{ initialPaths?: string[]; webFileInput?: HTMLInputElement | null }>()
const emit = defineEmits<{ close: [] }>()

const { exec, query, getTableInfo, loadExtension, importFromPath, importFromUrl, importSqliteFromPath, registerFile, dropFile } = useDuckDB()
const schemaStore = useSchemaStore()
const { saveTable } = usePersistence()

type Status = 'queued' | 'processing' | 'done' | 'error'
type DbStatus = 'queued' | 'scanning' | 'importing' | 'done' | 'error'

interface PathImportItem {
  kind: 'path'
  key: string
  filePath: string
  displayName: string
  tableName: string
  status: Status
  rowCount?: number
  error?: string
}

interface UrlImportItem {
  kind: 'url'
  key: string
  url: string
  tableName: string
  status: Status
  rowCount?: number
  error?: string
}

interface DbImportItem {
  kind: 'db'
  key: string
  filePath: string
  displayName: string
  prefix: string   // user-supplied prefix, e.g. "crm_"
  ready: boolean   // true once user has confirmed and import should start
  status: DbStatus
  tables: string[]
  tablesImported: number
  error?: string
}

interface FileImportItem {
  kind: 'file'
  key: string
  file: File
  tableName: string
  status: Status
  rowCount?: number
  error?: string
}

type ImportItem = PathImportItem | UrlImportItem | DbImportItem | FileImportItem

const items = ref<ImportItem[]>([])
const isProcessing = ref(false)

// URL import state
const urlInput = ref('')
const urlTableName = ref('')
const urlError = ref('')

const urlNameUserEdited = ref<boolean>(false)
watch(urlInput, (val) => {
  if (urlNameUserEdited.value) return
  try {
    const normalized = val.replace(/^(s3|gcs|r2|hf):\/\//i, 'https://')
    const urlPath = new URL(normalized).pathname
    const base = urlPath.split('/').pop()?.replace(/\.[^.]+$/, '') || ''
    urlTableName.value = base ? sanitize(base) : ''
  } catch {
    urlTableName.value = ''
  }
})

// ── Name helpers ──────────────────────────────────────────────────────────────

function basename(filePath: string): string {
  return filePath.split(/[/\\]/).pop() ?? filePath
}

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
  const taken = new Set([
    ...schemaStore.nodes.map((n) => n.name),
    ...items.value
      .filter((i): i is PathImportItem | UrlImportItem | FileImportItem => i.kind === 'path' || i.kind === 'url' || i.kind === 'file')
      .map((i) => i.tableName),
  ])
  if (!taken.has(base)) return base
  let n = 1
  while (taken.has(`${base}_${n}`)) n++
  return `${base}_${n}`
}

// ── Canvas positioning ────────────────────────────────────────────────────────

function nextPosition(): { x: number; y: number } {
  if (!schemaStore.nodes.length) return { x: 60, y: 80 }
  const maxX = Math.max(...schemaStore.nodes.map((n) => n.x))
  const anchor = schemaStore.nodes.find((n) => n.x === maxX)!
  return { x: maxX + 280, y: anchor.y }
}

// ── Processing ────────────────────────────────────────────────────────────────

async function addTableToCanvas(tableName: string): Promise<number> {
  const safeTable = tableName.replace(/"/g, '""')
  const [columns, countResult] = await Promise.all([
    getTableInfo(tableName),
    query(`SELECT COUNT(*) AS n FROM "${safeTable}"`),
  ])
  const rowCount = Number(countResult.rows[0]?.n ?? 0)
  const { x, y } = nextPosition()
  schemaStore.addTable({ id: tableName, name: tableName, x, y, columns })
  schemaStore.setRowCount(tableName, rowCount)
  await saveTable(tableName).catch(console.warn)
  return rowCount
}

async function processPathItem(item: PathImportItem) {
  item.status = 'processing'
  try {
    await importFromPath(item.filePath, item.tableName)
    item.rowCount = await addTableToCanvas(item.tableName)
    item.status = 'done'
  } catch (e) {
    item.status = 'error'
    item.error = e instanceof Error ? e.message : String(e)
  }
}

async function processUrlItem(item: UrlImportItem) {
  item.status = 'processing'
  try {
    if (/^https?:\/\//i.test(item.url)) {
      // Go downloads the file directly — no DuckDB extension required.
      await importFromUrl(item.url, item.tableName)
    } else {
      // Cloud storage schemes (s3://, hf://, gcs://, r2://) need DuckDB httpfs.
      await loadExtension('httpfs')
      const clean = item.url.replace(/'/g, "''")
      let readFn: string
      if (/\.parquet$/i.test(item.url)) readFn = `read_parquet('${clean}')`
      else if (/\.json(l)?$/i.test(item.url)) readFn = `read_json_auto('${clean}')`
      else readFn = `read_csv_auto('${clean}', header = true, sample_size = -1)`
      const safeTable = item.tableName.replace(/"/g, '""')
      await exec(`CREATE TABLE "${safeTable}" AS SELECT * FROM ${readFn}`)
    }
    item.rowCount = await addTableToCanvas(item.tableName)
    item.status = 'done'
  } catch (e) {
    item.status = 'error'
    item.error = e instanceof Error ? e.message : String(e)
  }
}

async function processDbItem(item: DbImportItem) {
  item.status = 'scanning'
  try {
    item.status = 'importing'
    const createdTables = await importSqliteFromPath(item.filePath, item.prefix)
    item.tables = createdTables
    item.tablesImported = createdTables.length
    for (const tableName of createdTables) {
      await addTableToCanvas(tableName)
    }
    item.status = 'done'
  } catch (e) {
    item.status = 'error'
    item.error = e instanceof Error ? e.message : String(e)
  }
}

// ── Enqueue ───────────────────────────────────────────────────────────────────

async function runQueue() {
  if (isProcessing.value) return
  isProcessing.value = true
  for (const item of items.value.filter((i) => i.status === 'queued')) {
    if (item.kind === 'db' && !item.ready) continue  // waiting for user to confirm prefix
    if (item.kind === 'path') await processPathItem(item)
    else if (item.kind === 'url') await processUrlItem(item)
    else if (item.kind === 'file') await processFileItem(item)
    else await processDbItem(item)
  }
  isProcessing.value = false
}

function startDbImport(item: DbImportItem) {
  item.ready = true
  runQueue()
}

async function enqueuePaths(paths: string[]) {
  const DROPPABLE = /\.(csv|tsv|txt|parquet|json|jsonl|sqlite|db|duckdb)$/i
  const filtered = paths.filter((p) => DROPPABLE.test(p))
  const newItems: ImportItem[] = filtered.map((filePath) => {
    const name = basename(filePath)
    if (/\.(sqlite|db|duckdb)$/i.test(name)) {
      return { kind: 'db' as const, key: `${filePath}-${Date.now()}`, filePath, displayName: name, prefix: '', ready: false, status: 'queued' as DbStatus, tables: [], tablesImported: 0 }
    }
    return { kind: 'path' as const, key: `${filePath}-${Date.now()}`, filePath, displayName: name, tableName: uniqueName(sanitize(name)), status: 'queued' as Status }
  })
  items.value.push(...newItems)
  runQueue()
}

async function processFileItem(item: FileImportItem) {
  item.status = 'processing'
  try {
    const buffer = new Uint8Array(await item.file.arrayBuffer())
    const ext = item.file.name.split('.').pop()?.toLowerCase() ?? 'csv'
    const fname = `__imp_${Date.now()}.${ext}`
    await registerFile(fname, buffer)
    const safeTable = item.tableName.replace(/"/g, '""')
    const readFn = ext === 'parquet' ? `read_parquet('${fname}')` :
                   (ext === 'json' || ext === 'jsonl') ? `read_json_auto('${fname}')` :
                   `read_csv_auto('${fname}', header=true, sample_size=-1)`
    await exec(`DROP TABLE IF EXISTS "${safeTable}"`)
    await exec(`CREATE TABLE "${safeTable}" AS SELECT * FROM ${readFn}`)
    await dropFile(fname)
    item.rowCount = await addTableToCanvas(item.tableName)
    item.status = 'done'
  } catch (e) {
    item.status = 'error'
    item.error = e instanceof Error ? e.message : String(e)
  }
}

async function openNativeDialog() {
  if (!IS_DESKTOP) {
    // Trigger the hidden file input passed in from App.vue
    if (props.webFileInput) {
      props.webFileInput.onchange = (e) => {
        const files = (e.target as HTMLInputElement).files
        if (files?.length) enqueueFiles(Array.from(files))
        props.webFileInput!.value = ''
      }
      props.webFileInput.click()
    }
    return
  }
  const { OpenMultipleFilesDialog } = await import('../../wailsjs/go/main/App')
  const paths = await OpenMultipleFilesDialog()
  if (paths?.length) enqueuePaths(paths)
}

function enqueueFiles(files: File[]) {
  const ACCEPTED = /\.(csv|tsv|txt|parquet|json|jsonl)$/i
  const newItems: FileImportItem[] = files
    .filter((f) => ACCEPTED.test(f.name))
    .map((file) => ({
      kind: 'file' as const,
      key: `file-${Date.now()}-${file.name}`,
      file,
      tableName: uniqueName(sanitize(file.name)),
      status: 'queued' as Status,
    }))
  items.value.push(...newItems)
  runQueue()
}

async function enqueueUrl() {
  const raw = urlInput.value.trim()
  urlError.value = ''
  if (!raw) return
  if (!/^(https?|s3|gcs|r2|hf):\/\/.+/i.test(raw)) {
    urlError.value = 'Enter a valid URL (https://, s3://, hf://, …)'
    return
  }
  const nameBase = urlTableName.value.trim()
  if (!nameBase) { urlError.value = 'Enter a table name'; return }

  items.value.push({
    kind: 'url', key: `url-${Date.now()}`, url: raw,
    tableName: uniqueName(sanitize(nameBase)), status: 'queued',
  })
  urlInput.value = ''
  urlTableName.value = ''
  urlNameUserEdited.value = false
  runQueue()
}

// ── Keyboard / outside-click close ───────────────────────────────────────────

function onBackdropClick(e: MouseEvent) {
  if (e.target === e.currentTarget && !isProcessing.value) emit('close')
}
function onKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape' && !isProcessing.value) emit('close')
}

onMounted(() => {
  window.addEventListener('keydown', onKeyDown)
  if (props.initialPaths?.length) enqueuePaths([...props.initialPaths])
})
onUnmounted(() => window.removeEventListener('keydown', onKeyDown))

defineExpose({ enqueuePaths })

// ── Computed ──────────────────────────────────────────────────────────────────

const allSettled = computed(
  () => items.value.length > 0 &&
    items.value.every((i) =>
      i.status === 'done' || i.status === 'error' ||
      (i.kind === 'db' && !i.ready)
    ),
)
const doneCount  = computed(() => items.value.filter((i) => i.status === 'done').length)
const errorCount = computed(() => items.value.filter((i) => i.status === 'error').length)

function itemStatusClass(item: ImportItem): string {
  if (item.kind === 'db') {
    if (item.status === 'done') return 'done'
    if (item.status === 'error') return 'error'
    if (item.status === 'queued' && !item.ready) return 'pending'
    if (item.status === 'queued') return 'queued'
    return 'processing'
  }
  return item.status
}

function fmtRows(n: number) { return n.toLocaleString() + (n === 1 ? ' row' : ' rows') }
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
          Import Data
        </div>
        <button class="close-btn" :disabled="isProcessing" title="Close" @click="emit('close')">
          <svg viewBox="0 0 12 12" fill="none">
            <path d="M1 1l10 10M11 1L1 11" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <!-- Drop zone / browse -->
      <div class="drop-zone" @click="openNativeDialog">
        <svg class="drop-icon" viewBox="0 0 32 32" fill="none">
          <rect x="4" y="6" width="24" height="20" rx="3" stroke="currentColor" stroke-width="1.5"/>
          <path d="M4 12h24" stroke="currentColor" stroke-width="1.5"/>
          <path d="M10 18h5M10 22h8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          <circle cx="24" cy="22" r="5" fill="var(--surface-1)" stroke="currentColor" stroke-width="1.5"/>
          <path d="M24 19v3M24 22l-2-2M24 22l2-2" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <p class="drop-primary">{{ IS_DESKTOP ? 'Browse files or drop onto canvas' : 'Browse files' }}</p>
        <p class="drop-secondary">{{ IS_DESKTOP ? '.csv .parquet .json .sqlite .duckdb' : '.csv .parquet .json .jsonl' }}</p>
      </div>

      <!-- URL import -->
      <div class="url-section">
        <form @submit.prevent="enqueueUrl">
          <div class="url-row">
            <svg class="url-icon" viewBox="0 0 16 16" fill="none">
              <circle cx="8" cy="8" r="6.5" stroke="currentColor" stroke-width="1.3"/>
              <path d="M1.5 8h13M8 1.5C6.5 3.5 5.5 5.6 5.5 8s1 4.5 2.5 6.5M8 1.5C9.5 3.5 10.5 5.6 10.5 8s-1 4.5-2.5 6.5" stroke="currentColor" stroke-width="1.3"/>
            </svg>
            <input
              v-model="urlInput"
              type="url"
              class="url-input"
              placeholder="https://… or s3://… or hf://datasets/…"
              :disabled="isProcessing"
            />
          </div>
          <div class="url-name-row">
            <span class="url-name-label">Table name</span>
            <input
              v-model="urlTableName"
              type="text"
              class="url-name-input"
              placeholder="my_table"
              :disabled="isProcessing"
              @input="urlNameUserEdited = true"
            />
            <button type="submit" class="url-btn" :disabled="isProcessing || !urlInput.trim() || !urlTableName.trim()">
              Import
            </button>
          </div>
        </form>
        <p v-if="urlError" class="url-error">{{ urlError }}</p>
      </div>

      <!-- File list -->
      <div v-if="items.length" class="file-list">
        <div
          v-for="item in items"
          :key="item.key"
          class="file-item"
          :class="itemStatusClass(item)"
        >
          <!-- Status icon -->
          <div class="file-status-icon">
            <!-- pending: db file waiting for user to confirm prefix -->
            <svg v-if="item.kind === 'db' && !item.ready && item.status === 'queued'" class="pending-pulse" viewBox="0 0 14 14" fill="none">
              <circle cx="7" cy="7" r="5.5" stroke="currentColor" stroke-width="1.3"/>
              <circle cx="7" cy="7" r="2" fill="currentColor"/>
            </svg>
            <svg v-else-if="item.status === 'queued'" viewBox="0 0 14 14" fill="none">
              <circle cx="7" cy="7" r="5.5" stroke="currentColor" stroke-width="1.3"/>
            </svg>
            <svg v-else-if="item.status !== 'done' && item.status !== 'error'" class="spin" viewBox="0 0 14 14" fill="none">
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

          <!-- File info: flat files, browser files, and URLs -->
          <div v-if="item.kind === 'path' || item.kind === 'url' || item.kind === 'file'" class="file-info">
            <div class="file-names">
              <span class="file-original">{{ item.kind === 'path' ? item.displayName : item.kind === 'file' ? item.file.name : item.url }}</span>
              <span class="file-arrow">→</span>
              <span class="file-table">{{ item.tableName }}</span>
            </div>
            <div class="file-meta">
              <span v-if="item.kind === 'url'" class="file-size url-label">URL</span>
              <span v-if="item.status === 'processing'" class="file-progress">Importing…</span>
              <span v-else-if="item.status === 'done'" class="file-rows-count">{{ fmtRows(item.rowCount!) }}</span>
              <span v-else-if="item.status === 'error'" class="file-error-inline" :title="item.error">{{ item.error?.split('\n')[0] }}</span>
            </div>
          </div>

          <!-- File info: database files -->
          <div v-else class="file-info">
            <div class="file-names">
              <span class="file-original">{{ item.displayName }}</span>
              <template v-if="item.tables.length">
                <span class="file-arrow">→</span>
                <span class="file-table">{{ item.tables.length }} table{{ item.tables.length !== 1 ? 's' : '' }}</span>
              </template>
            </div>
            <!-- Prefix input shown before import starts -->
            <div v-if="item.status === 'queued' && !item.ready" class="db-prefix-row">
              <input
                v-model="item.prefix"
                type="text"
                class="prefix-input"
                placeholder="prefix_ (optional)"
                @keydown.enter.prevent="startDbImport(item)"
              />
              <button class="prefix-start-btn" @click="startDbImport(item)">Import</button>
            </div>
            <div v-else class="file-meta">
              <span v-if="item.prefix && (item.status === 'importing' || item.status === 'done')" class="file-size">{{ item.prefix }}*</span>
              <span v-if="item.status === 'scanning'" class="file-progress">Scanning…</span>
              <span v-else-if="item.status === 'importing'" class="file-progress">Importing…</span>
              <span v-else-if="item.status === 'done'" class="file-rows-count">
                {{ item.tablesImported }} table{{ item.tablesImported !== 1 ? 's' : '' }} imported
              </span>
              <span v-else-if="item.status === 'error'" class="file-error-inline" :title="item.error">
                {{ item.error?.split('\n')[0] }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="modal-footer">
        <span v-if="allSettled" class="settle-summary">
          <template v-if="errorCount === 0">{{ doneCount }} {{ doneCount === 1 ? 'import' : 'imports' }} complete</template>
          <template v-else>{{ doneCount }} done · <span class="err-count">{{ errorCount }} failed</span></template>
        </span>
        <span v-else-if="isProcessing" class="settle-summary">
          Importing {{ items.filter(i => i.status !== 'done' && i.status !== 'error').length }} remaining…
        </span>
        <span v-else class="settle-summary hint">DuckDB will auto-detect column types</span>

        <button v-if="!allSettled" class="footer-btn secondary" :disabled="isProcessing" @click="emit('close')">Cancel</button>
        <button v-else class="footer-btn primary" @click="emit('close')">Done</button>
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

.modal-title svg { width: 16px; height: 16px; color: var(--accent); }

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
.close-btn:hover:not(:disabled) { background: var(--surface-2); color: var(--text-primary); }
.close-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.close-btn svg { width: 10px; height: 10px; }

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
.drop-zone:hover, .drop-zone.drag-over { border-color: var(--accent); background: rgba(88, 166, 255, 0.05); }
.drop-zone.drag-over { background: rgba(88, 166, 255, 0.1); }

.drop-icon { width: 36px; height: 36px; color: var(--text-muted); margin-bottom: 4px; }
.drop-primary { font-size: 13.5px; font-weight: 500; color: var(--text-primary); }
.drop-secondary { font-size: 12px; color: var(--text-muted); }
.browse-link { color: var(--accent); text-decoration: underline; text-underline-offset: 2px; }

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
}
.file-item:last-child { border-bottom: none; }

.file-status-icon {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.file-status-icon svg { width: 14px; height: 14px; }

.file-item.queued    .file-status-icon { color: var(--text-muted); }
.file-item.processing .file-status-icon { color: var(--accent); }
.file-item.done      .file-status-icon { color: var(--success); }
.file-item.error     .file-status-icon { color: var(--error); }
.file-item.pending   .file-status-icon { color: var(--accent); }

.file-item.pending {
  background: rgba(88, 166, 255, 0.05);
  border-left: 2px solid var(--accent);
  padding-left: 14px; /* 16px - 2px border */
}
.file-item.pending .file-original { color: var(--text-primary); }

.file-info { flex: 1; min-width: 0; }

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
.file-arrow { color: var(--text-muted); flex-shrink: 0; font-size: 11px; }
.file-table { font-family: var(--font-mono); font-size: 11.5px; color: var(--text-primary); flex-shrink: 0; }

.file-meta { display: flex; align-items: center; gap: 8px; font-size: 11px; }
.file-size { color: var(--text-muted); font-family: var(--font-mono); }
.file-progress { color: var(--accent); font-style: italic; }
.file-rows-count { color: var(--success); font-family: var(--font-mono); }
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
.settle-summary { flex: 1; font-size: 12px; color: var(--text-secondary); }
.settle-summary.hint { color: var(--text-muted); font-style: italic; }
.err-count { color: var(--error); }

.footer-btn {
  padding: 6px 16px;
  font-size: 12.5px;
  font-weight: 500;
  border-radius: 5px;
  border: 1px solid var(--border);
  cursor: pointer;
  transition: background 0.15s, color 0.15s, opacity 0.15s;
}
.footer-btn.secondary { background: transparent; color: var(--text-secondary); }
.footer-btn.secondary:hover:not(:disabled) { background: var(--surface-2); color: var(--text-primary); }
.footer-btn.secondary:disabled { opacity: 0.4; cursor: not-allowed; }
.footer-btn.primary { background: var(--accent); color: #0d1117; border-color: transparent; font-weight: 600; }
.footer-btn.primary:hover { background: var(--accent-hover); }

/* ── URL import ──────────────────────────────────────────────────────────── */
.url-section {
  padding: 0 16px 12px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.url-row, .url-name-row {
  display: flex;
  align-items: center;
  gap: 6px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 7px;
  padding: 5px 8px;
  transition: border-color 0.15s;
}
.url-row:focus-within, .url-name-row:focus-within { border-color: var(--accent); }
.url-icon { width: 14px; height: 14px; color: var(--text-muted); flex-shrink: 0; }
.url-input, .url-name-input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  font-size: 12px;
  color: var(--text-primary);
  font-family: var(--font-mono);
  min-width: 0;
}
.url-input::placeholder, .url-name-input::placeholder { color: var(--text-muted); font-family: var(--font-sans, sans-serif); }
.url-name-label { font-size: 11px; color: var(--text-muted); flex-shrink: 0; white-space: nowrap; }
.url-btn {
  padding: 3px 10px;
  font-size: 12px;
  font-weight: 600;
  background: var(--accent);
  color: #0d1117;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.15s, opacity 0.15s;
}
.url-btn:hover:not(:disabled) { background: var(--accent-hover); }
.url-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.url-error { font-size: 11.5px; color: var(--error); padding-left: 2px; }
.url-label { font-style: italic; color: var(--accent); }

/* ── DB prefix row ───────────────────────────────────────────────────────── */
.db-prefix-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
}
.prefix-input {
  flex: 1;
  min-width: 0;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 3px 7px;
  font-size: 11.5px;
  font-family: var(--font-mono);
  color: var(--text-primary);
  outline: none;
}
.prefix-input:focus { border-color: var(--accent); }
.prefix-input::placeholder { color: var(--text-muted); font-family: var(--font-sans, sans-serif); }
.prefix-start-btn {
  flex-shrink: 0;
  padding: 3px 10px;
  font-size: 11.5px;
  font-weight: 600;
  background: var(--accent);
  color: #0d1117;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}
.prefix-start-btn:hover { background: var(--accent-hover); }

/* ── Animations ──────────────────────────────────────────────────────────── */
.spin { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.pending-pulse { animation: pending-pulse 1.5s ease-in-out infinite; }
@keyframes pending-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.35; }
}
</style>
