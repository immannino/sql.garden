<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useDuckDB } from '../composables/useDuckDB'
import { useSchemaStore } from '../stores/schema'
import { usePersistence } from '../composables/usePersistence'
import { useImportHistory } from '../composables/useImportHistory'
import { IS_DESKTOP } from '../lib/env'

const props = defineProps<{
  initialPaths?: string[]
  webFileInput?: HTMLInputElement | null
  initialTab?: 'files' | 'paste'
}>()
const emit = defineEmits<{ close: []; created: [tableName: string] }>()

const { exec, query, getTableInfo, loadExtension, importFromPath, importFromUrl, importSqliteFromPath, registerFile, dropFile } = useDuckDB()
const schemaStore = useSchemaStore()
const { saveTable } = usePersistence()
const { entries: historyEntries, push: pushHistory, remove: removeHistory, clear: clearHistory } = useImportHistory()

// ── Tabs ──────────────────────────────────────────────────────────────────────
const activeTab = ref<'files' | 'paste' | 'recent'>(props.initialTab ?? 'files')

// ── File import types ─────────────────────────────────────────────────────────
type Status = 'queued' | 'processing' | 'done' | 'error'
type DbStatus = 'queued' | 'scanning' | 'importing' | 'done' | 'error'

interface PathImportItem {
  kind: 'path'; key: string; filePath: string; displayName: string
  tableName: string; status: Status; rowCount?: number; error?: string
}
interface UrlImportItem {
  kind: 'url'; key: string; url: string
  tableName: string; status: Status; rowCount?: number; error?: string
}
interface DbImportItem {
  kind: 'db'; key: string; filePath: string; displayName: string
  prefix: string; ready: boolean; status: DbStatus
  tables: string[]; tablesImported: number; error?: string
}
interface FileImportItem {
  kind: 'file'; key: string; file: File
  tableName: string; status: Status; rowCount?: number; error?: string
}
type ImportItem = PathImportItem | UrlImportItem | DbImportItem | FileImportItem

const items = ref<ImportItem[]>([])
const isProcessing = ref(false)

// URL import state
const urlInput = ref('')
const urlTableName = ref('')
const urlError = ref('')
const urlNameUserEdited = ref(false)

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
function basename(filePath: string): string { return filePath.split(/[/\\]/).pop() ?? filePath }

function sanitize(filename: string): string {
  return filename
    .replace(/\.[^.]+$/, '').replace(/[^a-zA-Z0-9_]/g, '_')
    .replace(/^(\d)/, '_$1').replace(/_+/g, '_').replace(/^_|_$/g, '')
    .toLowerCase() || 'imported_table'
}

function uniqueName(base: string): string {
  const taken = new Set([
    ...schemaStore.nodes.map((n) => n.name),
    ...items.value
      .filter((i): i is PathImportItem | UrlImportItem | FileImportItem =>
        i.kind === 'path' || i.kind === 'url' || i.kind === 'file')
      .map((i) => i.tableName),
  ])
  if (!taken.has(base)) return base
  let n = 1; while (taken.has(`${base}_${n}`)) n++
  return `${base}_${n}`
}

// ── Canvas positioning ────────────────────────────────────────────────────────
function nextPosition(): { x: number; y: number } {
  if (!schemaStore.nodes.length) return { x: 60, y: 80 }
  const maxX = Math.max(...schemaStore.nodes.map((n) => n.x))
  const anchor = schemaStore.nodes.find((n) => n.x === maxX)!
  return { x: maxX + 280, y: anchor.y }
}

// ── File import processing ────────────────────────────────────────────────────
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
    pushHistory({ fileName: item.displayName, filePath: item.filePath, tableName: item.tableName, rowCount: item.rowCount })
  } catch (e) { item.status = 'error'; item.error = e instanceof Error ? e.message : String(e) }
}
async function processUrlItem(item: UrlImportItem) {
  item.status = 'processing'
  try {
    if (/^https?:\/\//i.test(item.url)) {
      await importFromUrl(item.url, item.tableName)
    } else {
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
    const displayUrl = item.url.length > 60 ? item.url.slice(0, 57) + '…' : item.url
    pushHistory({ fileName: displayUrl, url: item.url, tableName: item.tableName, rowCount: item.rowCount })
  } catch (e) { item.status = 'error'; item.error = e instanceof Error ? e.message : String(e) }
}
async function processDbItem(item: DbImportItem) {
  item.status = 'scanning'
  try {
    item.status = 'importing'
    const createdTables = await importSqliteFromPath(item.filePath, item.prefix)
    item.tables = createdTables; item.tablesImported = createdTables.length
    for (const t of createdTables) await addTableToCanvas(t)
    item.status = 'done'
  } catch (e) { item.status = 'error'; item.error = e instanceof Error ? e.message : String(e) }
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
    pushHistory({ fileName: item.file.name, tableName: item.tableName, rowCount: item.rowCount })
  } catch (e) { item.status = 'error'; item.error = e instanceof Error ? e.message : String(e) }
}

async function runQueue() {
  if (isProcessing.value) return
  isProcessing.value = true
  for (const item of items.value.filter((i) => i.status === 'queued')) {
    if (item.kind === 'db' && !item.ready) continue
    if (item.kind === 'path') await processPathItem(item)
    else if (item.kind === 'url') await processUrlItem(item)
    else if (item.kind === 'file') await processFileItem(item)
    else await processDbItem(item)
  }
  isProcessing.value = false
}

function startDbImport(item: DbImportItem) { item.ready = true; runQueue() }

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
  items.value.push(...newItems); runQueue()
}

function enqueueFiles(files: File[]) {
  const ACCEPTED = /\.(csv|tsv|txt|parquet|json|jsonl)$/i
  const newItems: FileImportItem[] = files.filter((f) => ACCEPTED.test(f.name)).map((file) => ({
    kind: 'file' as const, key: `file-${Date.now()}-${file.name}`, file,
    tableName: uniqueName(sanitize(file.name)), status: 'queued' as Status,
  }))
  items.value.push(...newItems); runQueue()
}

async function openNativeDialog() {
  if (!IS_DESKTOP) {
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

async function enqueueUrl() {
  const raw = urlInput.value.trim(); urlError.value = ''
  if (!raw) return
  if (!/^(https?|s3|gcs|r2|hf):\/\/.+/i.test(raw)) { urlError.value = 'Enter a valid URL (https://, s3://, hf://, …)'; return }
  const nameBase = urlTableName.value.trim()
  if (!nameBase) { urlError.value = 'Enter a table name'; return }
  items.value.push({ kind: 'url', key: `url-${Date.now()}`, url: raw, tableName: uniqueName(sanitize(nameBase)), status: 'queued' })
  urlInput.value = ''; urlTableName.value = ''; urlNameUserEdited.value = false; runQueue()
}

// ── Paste tab ─────────────────────────────────────────────────────────────────
const rawText = ref('')
const pasteTableName = ref('')
const pasteIsCreating = ref(false)
const pasteError = ref<string | null>(null)
const pasteSuccess = ref<string | null>(null)
const pasteTextareaRef = ref<HTMLTextAreaElement | null>(null)

watch(activeTab, (tab) => { if (tab === 'paste') pasteTextareaRef.value?.focus() })

function pasteSuggestedName(): string {
  const existing = new Set(schemaStore.nodes.filter(n => n.kind === 'table').map(n => n.name))
  let i = 1; while (existing.has(`paste_${i}`)) i++
  return `paste_${i}`
}

const detectedFormat = computed<'csv' | 'tsv' | 'json' | null>(() => {
  const t = rawText.value.trim()
  if (!t) return null
  if (t.startsWith('[') || t.startsWith('{')) return 'json'
  const firstLine = t.split('\n')[0]
  if (firstLine.includes('\t')) return 'tsv'
  if (firstLine.includes(',')) return 'csv'
  return null
})

function parseSv(text: string, delimiter: string): { columns: string[]; rows: Record<string, string>[] } {
  const lines: string[][] = []
  let cur = '', inQuote = false, fields: string[] = []
  for (let i = 0; i < text.length; i++) {
    const ch = text[i]
    if (inQuote) {
      if (ch === '"' && text[i + 1] === '"') { cur += '"'; i++ }
      else if (ch === '"') { inQuote = false }
      else { cur += ch }
    } else if (ch === '"') { inQuote = true }
    else if (ch === delimiter) { fields.push(cur); cur = '' }
    else if (ch === '\r' || ch === '\n') {
      fields.push(cur); cur = ''; lines.push(fields); fields = []
      if (ch === '\r' && text[i + 1] === '\n') i++
    } else { cur += ch }
  }
  if (cur || fields.length > 0) { fields.push(cur); lines.push(fields) }
  while (lines.length > 1 && lines[lines.length - 1].every(f => f === '')) lines.pop()
  if (lines.length === 0) return { columns: [], rows: [] }
  const columns = lines[0].map((h, i) => h.trim() || `col${i + 1}`)
  const rows = lines.slice(1).map(fs => {
    const row: Record<string, string> = {}
    columns.forEach((col, i) => { row[col] = fs[i] ?? '' }); return row
  })
  return { columns, rows }
}

function parseJson(text: string): { columns: string[]; rows: Record<string, unknown>[] } {
  const parsed = JSON.parse(text)
  const arr: Record<string, unknown>[] = Array.isArray(parsed) ? parsed : [parsed]
  if (arr.length === 0) return { columns: [], rows: [] }
  const seen = new Set<string>(); const columns: string[] = []
  for (const item of arr.slice(0, 200)) {
    if (item && typeof item === 'object' && !Array.isArray(item))
      for (const k of Object.keys(item)) if (!seen.has(k)) { seen.add(k); columns.push(k) }
  }
  const rows = arr.map(r => { const row: Record<string, unknown> = {}; columns.forEach(c => { row[c] = r?.[c] ?? null }); return row })
  return { columns, rows }
}

const parsedPaste = computed<{ columns: string[]; rows: Record<string, unknown>[] } | null>(() => {
  const t = rawText.value.trim()
  if (!t || !detectedFormat.value) return null
  try {
    if (detectedFormat.value === 'json') return parseJson(t)
    return parseSv(t, detectedFormat.value === 'tsv' ? '\t' : ',')
  } catch { return null }
})

const previewRows = computed(() => parsedPaste.value?.rows.slice(0, 10) ?? [])

function inferSqlType(col: string, rows: Record<string, unknown>[]): string {
  const vals = rows.map(r => r[col]).filter(v => v !== null && v !== undefined && v !== '')
  if (vals.length === 0) return 'VARCHAR'
  if (vals.every(v => typeof v === 'boolean')) return 'BOOLEAN'
  if (vals.every(v => typeof v === 'number'))
    return vals.every(v => Number.isInteger(v as number)) ? 'BIGINT' : 'DOUBLE'
  const asNums = vals.map(v => Number(String(v)))
  if (asNums.every(n => !isNaN(n)))
    return asNums.every(n => Number.isInteger(n)) ? 'BIGINT' : 'DOUBLE'
  return 'VARCHAR'
}

async function onPasteSubmit() {
  if (!parsedPaste.value?.columns.length || !pasteTableName.value.trim()) return
  pasteIsCreating.value = true; pasteError.value = null; pasteSuccess.value = null

  const name = pasteTableName.value.trim()
  const { columns, rows } = parsedPaste.value
  const q = (s: string) => `"${s.replace(/"/g, '""')}"`
  const s = (v: string) => `'${v.replace(/'/g, "''")}'`

  const types: Record<string, string> = {}
  for (const col of columns) types[col] = inferSqlType(col, rows)

  try {
    const colDefs = columns.map(c => `${q(c)} ${types[c]}`).join(', ')
    await exec(`CREATE TABLE ${q(name)} (${colDefs})`)

    const BATCH = 200
    for (let i = 0; i < rows.length; i += BATCH) {
      const batch = rows.slice(i, i + BATCH)
      const values = batch.map(row => {
        const vals = columns.map(col => {
          const v = row[col]
          if (v === null || v === undefined || v === '') return 'NULL'
          if (types[col] === 'BOOLEAN') return String(v) === 'true' ? 'TRUE' : 'FALSE'
          if (types[col] === 'VARCHAR') return s(String(v))
          const n = Number(v); return isNaN(n) ? 'NULL' : String(n)
        })
        return `(${vals.join(', ')})`
      }).join(', ')
      await exec(`INSERT INTO ${q(name)} VALUES ${values}`)
    }

    const rowCount = await addTableToCanvas(name)
    pasteSuccess.value = `"${name}" created · ${rowCount.toLocaleString()} rows`
    rawText.value = ''; pasteError.value = null
    pasteTableName.value = pasteSuggestedName()
    emit('created', name)
  } catch (e) {
    pasteError.value = e instanceof Error ? e.message : String(e)
  } finally {
    pasteIsCreating.value = false
  }
}

// ── Keyboard / close ──────────────────────────────────────────────────────────
function onBackdropClick(e: MouseEvent) { if (e.target === e.currentTarget && !isProcessing.value) emit('close') }
function onKeyDown(e: KeyboardEvent) { if (e.key === 'Escape' && !isProcessing.value) emit('close') }

onMounted(() => {
  window.addEventListener('keydown', onKeyDown)
  if (props.initialPaths?.length) enqueuePaths([...props.initialPaths])
  if (props.initialTab === 'paste') pasteTableName.value = pasteSuggestedName()
})
onUnmounted(() => window.removeEventListener('keydown', onKeyDown))

defineExpose({ enqueuePaths })

// ── Computed (files) ──────────────────────────────────────────────────────────
const allSettled = computed(() =>
  items.value.length > 0 &&
  items.value.every(i => i.status === 'done' || i.status === 'error' || (i.kind === 'db' && !i.ready))
)
const doneCount  = computed(() => items.value.filter(i => i.status === 'done').length)
const errorCount = computed(() => items.value.filter(i => i.status === 'error').length)

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

function fmtRelTime(ts: number): string {
  const diff = Date.now() - ts
  if (diff < 60_000) return 'just now'
  if (diff < 3_600_000) return `${Math.round(diff / 60_000)}m ago`
  if (diff < 86_400_000) return `${Math.round(diff / 3_600_000)}h ago`
  if (diff < 2_592_000_000) return `${Math.round(diff / 86_400_000)}d ago`
  return new Date(ts).toLocaleDateString()
}

function reImport(entry: (typeof historyEntries.value)[number]) {
  if (entry.filePath) {
    enqueuePaths([entry.filePath])
    activeTab.value = 'files'
  } else if (entry.url) {
    urlInput.value = entry.url
    urlTableName.value = uniqueName(entry.tableName)
    urlNameUserEdited.value = true
    activeTab.value = 'files'
  }
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
          Import Data
        </div>
        <button class="close-btn" :disabled="isProcessing" title="Close" @click="emit('close')">
          <svg viewBox="0 0 12 12" fill="none">
            <path d="M1 1l10 10M11 1L1 11" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <!-- Tab strip -->
      <div class="tab-strip">
        <button class="tab" :class="{ active: activeTab === 'files' }" @click="activeTab = 'files'">
          <svg viewBox="0 0 14 14" fill="none">
            <path d="M2 3a1 1 0 011-1h3.5L8 4h4a1 1 0 011 1v6a1 1 0 01-1 1H3a1 1 0 01-1-1V3z" stroke="currentColor" stroke-width="1.2"/>
          </svg>
          Files
        </button>
        <button class="tab" :class="{ active: activeTab === 'paste' }" @click="activeTab = 'paste'; pasteTableName ||= pasteSuggestedName()">
          <svg viewBox="0 0 14 14" fill="none">
            <rect x="3" y="1" width="8" height="2.5" rx="0.8" stroke="currentColor" stroke-width="1.2"/>
            <path d="M2 2.5h10a1 1 0 011 1v8a1 1 0 01-1 1H2a1 1 0 01-1-1v-8a1 1 0 011-1z" stroke="currentColor" stroke-width="1.2"/>
            <line x1="4" y1="6.5"  x2="10" y2="6.5"  stroke="currentColor" stroke-width="1.1" stroke-linecap="round"/>
            <line x1="4" y1="9"    x2="8"  y2="9"    stroke="currentColor" stroke-width="1.1" stroke-linecap="round"/>
          </svg>
          Paste
        </button>
        <button class="tab" :class="{ active: activeTab === 'recent' }" @click="activeTab = 'recent'">
          <svg viewBox="0 0 14 14" fill="none">
            <circle cx="7" cy="7" r="5.5" stroke="currentColor" stroke-width="1.2"/>
            <path d="M7 4v3.5l2 1.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          Recent
          <span v-if="historyEntries.length" class="tab-count">{{ historyEntries.length }}</span>
        </button>
      </div>

      <!-- ── Files tab ───────────────────────────────────────────────────────── -->
      <template v-if="activeTab === 'files'">
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
              <input v-model="urlInput" type="url" class="url-input" placeholder="https://… or s3://… or hf://datasets/…" :disabled="isProcessing"/>
            </div>
            <div class="url-name-row">
              <span class="url-name-label">Table name</span>
              <input v-model="urlTableName" type="text" class="url-name-input" placeholder="my_table" :disabled="isProcessing" @input="urlNameUserEdited = true"/>
              <button type="submit" class="url-btn" :disabled="isProcessing || !urlInput.trim() || !urlTableName.trim()">Import</button>
            </div>
          </form>
          <p v-if="urlError" class="url-error">{{ urlError }}</p>
        </div>

        <!-- File list -->
        <div v-if="items.length" class="file-list">
          <div v-for="item in items" :key="item.key" class="file-item" :class="itemStatusClass(item)">
            <div class="file-status-icon">
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
            <div v-else class="file-info">
              <div class="file-names">
                <span class="file-original">{{ item.displayName }}</span>
                <template v-if="item.tables.length">
                  <span class="file-arrow">→</span>
                  <span class="file-table">{{ item.tables.length }} table{{ item.tables.length !== 1 ? 's' : '' }}</span>
                </template>
              </div>
              <div v-if="item.status === 'queued' && !item.ready" class="db-prefix-row">
                <input v-model="item.prefix" type="text" class="prefix-input" placeholder="prefix_ (optional)" @keydown.enter.prevent="startDbImport(item)"/>
                <button class="prefix-start-btn" @click="startDbImport(item)">Import</button>
              </div>
              <div v-else class="file-meta">
                <span v-if="item.prefix && (item.status === 'importing' || item.status === 'done')" class="file-size">{{ item.prefix }}*</span>
                <span v-if="item.status === 'scanning'" class="file-progress">Scanning…</span>
                <span v-else-if="item.status === 'importing'" class="file-progress">Importing…</span>
                <span v-else-if="item.status === 'done'" class="file-rows-count">{{ item.tablesImported }} table{{ item.tablesImported !== 1 ? 's' : '' }} imported</span>
                <span v-else-if="item.status === 'error'" class="file-error-inline" :title="item.error">{{ item.error?.split('\n')[0] }}</span>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- ── Recent tab ───────────────────────────────────────────────────────── -->
      <template v-else-if="activeTab === 'recent'">
        <div class="recent-tab">
          <div v-if="!historyEntries.length" class="recent-empty">
            <svg viewBox="0 0 32 32" fill="none">
              <circle cx="16" cy="16" r="12" stroke="currentColor" stroke-width="1.5"/>
              <path d="M16 9v7.5l4 3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            No imports yet — imported files will appear here
          </div>
          <template v-else>
            <div class="recent-header">
              <span class="recent-count">{{ historyEntries.length }} import{{ historyEntries.length !== 1 ? 's' : '' }}</span>
              <button class="recent-clear-btn" @click="clearHistory">Clear all</button>
            </div>
            <div class="recent-list">
              <div v-for="entry in historyEntries" :key="entry.id" class="recent-entry">
                <div class="recent-source-icon">
                  <!-- URL icon -->
                  <svg v-if="entry.url" viewBox="0 0 14 14" fill="none">
                    <circle cx="7" cy="7" r="5.5" stroke="currentColor" stroke-width="1.2"/>
                    <path d="M1.5 7h11M7 1.5C5.8 3 5 5 5 7s.8 4 2 5.5M7 1.5C8.2 3 9 5 9 7s-.8 4-2 5.5" stroke="currentColor" stroke-width="1.2"/>
                  </svg>
                  <!-- File icon -->
                  <svg v-else viewBox="0 0 14 14" fill="none">
                    <path d="M2 2a1 1 0 011-1h5l4 4v7a1 1 0 01-1 1H3a1 1 0 01-1-1V2z" stroke="currentColor" stroke-width="1.2"/>
                    <path d="M8 1v4h4" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/>
                  </svg>
                </div>
                <div class="recent-info">
                  <div class="recent-names">
                    <span class="recent-filename" :title="entry.filePath ?? entry.url ?? entry.fileName">{{ entry.fileName }}</span>
                    <span class="recent-arrow">→</span>
                    <span class="recent-tablename">{{ entry.tableName }}</span>
                  </div>
                  <div class="recent-meta">
                    <span class="recent-rows">{{ fmtRows(entry.rowCount) }}</span>
                    <span class="recent-dot">·</span>
                    <span class="recent-time">{{ fmtRelTime(entry.importedAt) }}</span>
                    <span v-if="!entry.filePath && !entry.url" class="recent-no-reimport">(no re-import — web file)</span>
                  </div>
                </div>
                <div class="recent-actions">
                  <button
                    v-if="entry.filePath || entry.url"
                    class="recent-reimport-btn"
                    :title="entry.filePath ? `Re-import from ${entry.filePath}` : `Re-import from URL`"
                    @click="reImport(entry)"
                  >
                    <svg viewBox="0 0 12 12" fill="none">
                      <path d="M1 6A5 5 0 1 1 4 2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                      <polyline points="1,1 1,4 4,4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
                    </svg>
                    Re-import
                  </button>
                  <button class="recent-remove-btn" title="Remove from history" @click="removeHistory(entry.id)">
                    <svg viewBox="0 0 10 10" fill="none">
                      <path d="M2 2l6 6M8 2l-6 6" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                    </svg>
                  </button>
                </div>
              </div>
            </div>
          </template>
        </div>
      </template>

      <!-- ── Paste tab ───────────────────────────────────────────────────────── -->
      <template v-else>
        <div class="paste-tab">
          <!-- Format badge -->
          <div class="paste-header-row">
            <span class="paste-hint">Paste CSV, TSV, or JSON below</span>
            <span v-if="detectedFormat" class="format-badge" :class="detectedFormat">{{ detectedFormat.toUpperCase() }}</span>
            <span v-else-if="rawText.trim()" class="format-badge unknown">Unknown</span>
          </div>

          <!-- Textarea — user-resizable vertically -->
          <textarea
            ref="pasteTextareaRef"
            v-model="rawText"
            class="paste-area"
            placeholder="id,name,value&#10;1,Alice,100&#10;2,Bob,200"
            spellcheck="false"
          />

          <!-- Preview -->
          <template v-if="parsedPaste && parsedPaste.columns.length">
            <div class="preview-meta-row">
              <span class="meta-pill">{{ parsedPaste.columns.length }} cols</span>
              <span class="meta-pill">{{ parsedPaste.rows.length.toLocaleString() }} rows</span>
            </div>
            <!-- Preview table — user-resizable vertically -->
            <div class="preview-scroll">
              <table class="preview-table">
                <thead>
                  <tr>
                    <th v-for="col in parsedPaste.columns" :key="col">{{ col }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(row, ri) in previewRows" :key="ri">
                    <td v-for="col in parsedPaste.columns" :key="col">
                      {{ row[col] !== null && row[col] !== undefined ? row[col] : '' }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-if="parsedPaste.rows.length > 10" class="preview-more">
              +{{ (parsedPaste.rows.length - 10).toLocaleString() }} more rows not shown
            </div>
          </template>

          <!-- Unknown format notice -->
          <div v-else-if="rawText.trim() && !detectedFormat" class="paste-notice warn">
            Could not detect format — expected CSV (commas), TSV (tabs), or a JSON array/object.
          </div>

          <!-- Success notice -->
          <div v-if="pasteSuccess" class="paste-notice success">✓ {{ pasteSuccess }}</div>
          <!-- Error notice -->
          <div v-if="pasteError" class="paste-notice error">{{ pasteError }}</div>
        </div>
      </template>

      <!-- ── Footer ─────────────────────────────────────────────────────────── -->
      <div class="modal-footer">
        <template v-if="activeTab === 'files'">
          <span v-if="allSettled" class="settle-summary">
            <template v-if="errorCount === 0">{{ doneCount }} {{ doneCount === 1 ? 'import' : 'imports' }} complete</template>
            <template v-else>{{ doneCount }} done · <span class="err-count">{{ errorCount }} failed</span></template>
          </span>
          <span v-else-if="isProcessing" class="settle-summary">Importing {{ items.filter(i => i.status !== 'done' && i.status !== 'error').length }} remaining…</span>
          <span v-else class="settle-summary hint">DuckDB will auto-detect column types</span>
          <button v-if="!allSettled" class="footer-btn secondary" :disabled="isProcessing" @click="emit('close')">Cancel</button>
          <button v-else class="footer-btn primary" @click="emit('close')">Done</button>
        </template>

        <template v-else-if="activeTab === 'recent'">
          <span class="settle-summary hint">Click Re-import to load a previous file</span>
          <button class="footer-btn secondary" @click="emit('close')">Close</button>
        </template>

        <template v-else>
          <input
            v-model="pasteTableName"
            class="paste-name-input"
            placeholder="Table name"
            @keydown.enter="onPasteSubmit"
          />
          <button class="footer-btn secondary" @click="emit('close')">Cancel</button>
          <button
            class="footer-btn primary"
            :disabled="!parsedPaste?.columns.length || !pasteTableName.trim() || pasteIsCreating"
            @click="onPasteSubmit"
          >
            {{ pasteIsCreating ? 'Creating…' : 'Create Table' }}
          </button>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed; inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex; align-items: center; justify-content: center;
  z-index: 200; backdrop-filter: blur(2px);
}

.modal {
  width: 580px;
  max-width: calc(100vw - 32px);
  max-height: calc(100vh - 64px);
  display: flex; flex-direction: column;
  background: var(--surface-1);
  border: 1px solid var(--border);
  border-radius: 10px;
  box-shadow: 0 24px 48px rgba(0, 0, 0, 0.5);
  overflow: hidden;
}

/* ── Header ──────────────────────────────────────────────────────────────── */
.modal-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.modal-title {
  display: flex; align-items: center; gap: 8px;
  font-size: 14px; font-weight: 600; color: var(--text-primary);
}
.modal-title svg { width: 16px; height: 16px; color: var(--accent); }

.close-btn {
  display: flex; align-items: center; justify-content: center;
  width: 24px; height: 24px;
  border: none; border-radius: 4px; background: transparent;
  color: var(--text-muted); cursor: pointer; transition: background 0.15s, color 0.15s;
}
.close-btn:hover:not(:disabled) { background: var(--surface-2); color: var(--text-primary); }
.close-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.close-btn svg { width: 10px; height: 10px; }

/* ── Tab strip ───────────────────────────────────────────────────────────── */
.tab-strip {
  display: flex; align-items: center; gap: 2px;
  padding: 6px 12px 0;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
  background: var(--surface-1);
}
.tab {
  display: flex; align-items: center; gap: 5px;
  padding: 5px 12px 6px;
  font-size: 12.5px; font-weight: 500;
  color: var(--text-muted);
  background: none; border: none; border-radius: 6px 6px 0 0;
  cursor: pointer; transition: color 0.12s, background 0.12s;
  position: relative;
}
.tab svg { width: 13px; height: 13px; }
.tab:hover:not(.active) { color: var(--text-secondary); background: var(--surface-2); }
.tab.active {
  color: var(--text-primary);
  background: var(--surface-1);
}
.tab.active::after {
  content: '';
  position: absolute; bottom: -1px; left: 0; right: 0; height: 1px;
  background: var(--surface-1);
}

.tab-count {
  font-size: 9px; font-weight: 700;
  padding: 1px 4px; border-radius: 8px;
  background: var(--surface-2); color: var(--text-muted);
}
.tab.active .tab-count { background: rgba(88,166,255,0.12); color: var(--accent); }

/* ── Recent tab ──────────────────────────────────────────────────────────── */
.recent-tab { display: flex; flex-direction: column; flex: 1; overflow: hidden; }

.recent-empty {
  flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 10px; color: var(--text-muted); font-size: 12px; font-style: italic; padding: 32px;
  text-align: center;
}
.recent-empty svg { width: 32px; height: 32px; opacity: 0.3; }

.recent-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 14px 6px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.recent-count { font-size: 10.5px; color: var(--text-muted); }
.recent-clear-btn {
  font-size: 10.5px; color: var(--text-muted); background: none; border: none;
  cursor: pointer; padding: 2px 6px; border-radius: 4px; transition: color 0.12s, background 0.12s;
}
.recent-clear-btn:hover { color: var(--error); background: rgba(248,81,73,0.08); }

.recent-list { flex: 1; overflow-y: auto; scrollbar-width: thin; scrollbar-color: var(--border) transparent; }

.recent-entry {
  display: flex; align-items: center; gap: 10px;
  padding: 9px 14px; border-bottom: 1px solid var(--border);
  transition: background 0.1s;
}
.recent-entry:last-child { border-bottom: none; }
.recent-entry:hover { background: var(--surface-2); }
.recent-entry:hover .recent-reimport-btn { opacity: 1; }
.recent-entry:hover .recent-remove-btn { opacity: 1; }

.recent-source-icon { flex-shrink: 0; color: var(--text-muted); }
.recent-source-icon svg { width: 14px; height: 14px; display: block; }

.recent-info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }

.recent-names { display: flex; align-items: center; gap: 5px; min-width: 0; }
.recent-filename {
  font-size: 11.5px; color: var(--text-primary); font-weight: 500;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 200px;
}
.recent-arrow { font-size: 10px; color: var(--text-muted); flex-shrink: 0; }
.recent-tablename { font-size: 11px; color: var(--accent); font-family: var(--font-mono); flex-shrink: 0; }

.recent-meta { display: flex; align-items: center; gap: 4px; }
.recent-rows { font-size: 10px; color: var(--text-muted); font-family: var(--font-mono); }
.recent-dot { font-size: 10px; color: var(--border); }
.recent-time { font-size: 10px; color: var(--text-muted); }
.recent-no-reimport { font-size: 9.5px; color: var(--text-muted); font-style: italic; margin-left: 2px; }

.recent-actions { display: flex; align-items: center; gap: 4px; flex-shrink: 0; }

.recent-reimport-btn {
  display: flex; align-items: center; gap: 4px;
  padding: 3px 8px; font-size: 10.5px; font-weight: 600;
  background: rgba(88,166,255,0.1); border: 1px solid rgba(88,166,255,0.3);
  border-radius: 4px; color: var(--accent); cursor: pointer;
  opacity: 0; transition: opacity 0.12s, background 0.12s;
}
.recent-reimport-btn svg { width: 10px; height: 10px; }
.recent-reimport-btn:hover { background: rgba(88,166,255,0.2); }

.recent-remove-btn {
  display: flex; align-items: center; justify-content: center;
  width: 20px; height: 20px; border-radius: 3px;
  background: none; border: none; color: var(--text-muted);
  cursor: pointer; opacity: 0; transition: opacity 0.12s, color 0.12s, background 0.12s;
}
.recent-remove-btn svg { width: 9px; height: 9px; }
.recent-remove-btn:hover { color: var(--error); background: rgba(248,81,73,0.1); }

/* ── Drop zone ───────────────────────────────────────────────────────────── */
.drop-zone {
  margin: 14px 16px 0;
  flex-shrink: 0;
  border: 1.5px dashed var(--border); border-radius: 8px;
  padding: 24px 20px;
  display: flex; flex-direction: column; align-items: center; gap: 6px;
  cursor: pointer; transition: border-color 0.15s, background 0.15s;
}
.drop-zone:hover { border-color: var(--accent); background: rgba(88, 166, 255, 0.05); }
.drop-icon { width: 36px; height: 36px; color: var(--text-muted); margin-bottom: 4px; }
.drop-primary { font-size: 13.5px; font-weight: 500; color: var(--text-primary); }
.drop-secondary { font-size: 12px; color: var(--text-muted); }

/* ── URL import ──────────────────────────────────────────────────────────── */
.url-section {
  padding: 10px 16px 12px;
  flex-shrink: 0; display: flex; flex-direction: column; gap: 6px;
}
.url-row, .url-name-row {
  display: flex; align-items: center; gap: 6px;
  background: var(--surface-2); border: 1px solid var(--border); border-radius: 7px;
  padding: 5px 8px; transition: border-color 0.15s;
}
.url-row:focus-within, .url-name-row:focus-within { border-color: var(--accent); }
.url-icon { width: 14px; height: 14px; color: var(--text-muted); flex-shrink: 0; }
.url-input, .url-name-input {
  flex: 1; background: transparent; border: none; outline: none;
  font-size: 12px; color: var(--text-primary); font-family: var(--font-mono); min-width: 0;
}
.url-input::placeholder, .url-name-input::placeholder { color: var(--text-muted); font-family: var(--font-sans, sans-serif); }
.url-name-label { font-size: 11px; color: var(--text-muted); flex-shrink: 0; white-space: nowrap; }
.url-btn {
  padding: 3px 10px; font-size: 12px; font-weight: 600;
  background: var(--accent); color: #0d1117; border: none; border-radius: 4px;
  cursor: pointer; flex-shrink: 0; transition: background 0.15s, opacity 0.15s;
}
.url-btn:hover:not(:disabled) { background: var(--accent-hover); }
.url-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.url-error { font-size: 11.5px; color: var(--error); padding-left: 2px; }
.url-label { font-style: italic; color: var(--accent); }

/* ── File list ───────────────────────────────────────────────────────────── */
.file-list { flex: 1; overflow-y: auto; border-top: 1px solid var(--border); min-height: 0; }
.file-item {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 16px; border-bottom: 1px solid var(--surface-2);
}
.file-item:last-child { border-bottom: none; }
.file-status-icon { flex-shrink: 0; width: 16px; height: 16px; display: flex; align-items: center; justify-content: center; }
.file-status-icon svg { width: 14px; height: 14px; }
.file-item.queued     .file-status-icon { color: var(--text-muted); }
.file-item.processing .file-status-icon { color: var(--accent); }
.file-item.done       .file-status-icon { color: var(--success); }
.file-item.error      .file-status-icon { color: var(--error); }
.file-item.pending    .file-status-icon { color: var(--accent); }
.file-item.pending { background: rgba(88, 166, 255, 0.05); border-left: 2px solid var(--accent); padding-left: 14px; }
.file-info { flex: 1; min-width: 0; }
.file-names { display: flex; align-items: center; gap: 6px; font-size: 12.5px; margin-bottom: 2px; overflow: hidden; }
.file-original { color: var(--text-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex-shrink: 1; min-width: 0; }
.file-arrow { color: var(--text-muted); flex-shrink: 0; font-size: 11px; }
.file-table { font-family: var(--font-mono); font-size: 11.5px; color: var(--text-primary); flex-shrink: 0; }
.file-meta { display: flex; align-items: center; gap: 8px; font-size: 11px; }
.file-size { color: var(--text-muted); font-family: var(--font-mono); }
.file-progress { color: var(--accent); font-style: italic; }
.file-rows-count { color: var(--success); font-family: var(--font-mono); }
.file-error-inline { color: var(--error); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 280px; display: block; }

/* ── DB prefix ───────────────────────────────────────────────────────────── */
.db-prefix-row { display: flex; align-items: center; gap: 6px; margin-top: 4px; }
.prefix-input {
  flex: 1; min-width: 0; background: var(--surface-2); border: 1px solid var(--border); border-radius: 4px;
  padding: 3px 7px; font-size: 11.5px; font-family: var(--font-mono); color: var(--text-primary); outline: none;
}
.prefix-input:focus { border-color: var(--accent); }
.prefix-input::placeholder { color: var(--text-muted); font-family: var(--font-sans, sans-serif); }
.prefix-start-btn {
  flex-shrink: 0; padding: 3px 10px; font-size: 11.5px; font-weight: 600;
  background: var(--accent); color: #0d1117; border: none; border-radius: 4px; cursor: pointer;
}
.prefix-start-btn:hover { background: var(--accent-hover); }

/* ── Paste tab ───────────────────────────────────────────────────────────── */
.paste-tab {
  flex: 1; display: flex; flex-direction: column;
  overflow: hidden; min-height: 0;
}

.paste-header-row {
  display: flex; align-items: center; gap: 8px;
  padding: 10px 16px 6px;
  flex-shrink: 0;
}
.paste-hint { font-size: 12px; color: var(--text-muted); flex: 1; }

.format-badge {
  font-size: 10px; font-weight: 700; letter-spacing: 0.06em;
  padding: 2px 7px; border-radius: 4px; flex-shrink: 0;
}
.format-badge.csv     { background: rgba(59, 130, 246, 0.15); color: #60a5fa; }
.format-badge.tsv     { background: rgba(52, 211, 153, 0.15); color: #34d399; }
.format-badge.json    { background: rgba(251, 146, 60, 0.15);  color: #fb923c; }
.format-badge.unknown { background: rgba(156, 163, 175, 0.15); color: #9ca3af; }

.paste-area {
  width: 100%; box-sizing: border-box;
  padding: 10px 14px;
  background: var(--surface-2);
  border: none; border-top: 1px solid var(--border); border-bottom: 1px solid var(--border);
  color: var(--text-primary);
  font-family: var(--font-mono); font-size: 12px; line-height: 1.55;
  outline: none;
  resize: vertical;
  min-height: 100px;
  max-height: 360px;
  flex-shrink: 0;
}
.paste-area::placeholder { color: var(--text-muted); font-family: var(--font-sans, sans-serif); }

.preview-meta-row {
  display: flex; align-items: center; gap: 6px;
  padding: 7px 14px; flex-shrink: 0;
  border-bottom: 1px solid var(--border);
}
.meta-pill {
  font-size: 11px; color: var(--text-muted);
  background: var(--surface-2); padding: 2px 8px; border-radius: 20px;
}

/* Resizable preview container */
.preview-scroll {
  overflow: auto;
  resize: vertical;
  min-height: 60px;
  max-height: 280px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.preview-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}
.preview-table th {
  position: sticky; top: 0; z-index: 1;
  /* Solid background so rows don't show through */
  background: var(--surface-1);
  box-shadow: 0 1px 0 var(--border);
  color: var(--text-muted); font-weight: 600;
  text-align: left; padding: 6px 10px;
  white-space: nowrap;
}
.preview-table td {
  color: var(--text-primary); padding: 5px 10px;
  border-bottom: 1px solid rgba(255,255,255,0.04);
  white-space: nowrap; max-width: 200px;
  overflow: hidden; text-overflow: ellipsis;
}
.preview-table tbody tr:hover td { background: rgba(255,255,255,0.03); }

.preview-more {
  padding: 5px 14px; font-size: 11px; color: var(--text-muted); flex-shrink: 0;
}

.paste-notice {
  margin: 0; padding: 8px 14px; font-size: 12px; flex-shrink: 0;
}
.paste-notice.warn    { background: rgba(245,158,11,0.08); color: #f59e0b; border-top: 1px solid rgba(245,158,11,0.2); }
.paste-notice.error   { background: rgba(239,68,68,0.08);  color: #ef4444; border-top: 1px solid rgba(239,68,68,0.2); }
.paste-notice.success { background: rgba(52,211,153,0.08); color: #34d399; border-top: 1px solid rgba(52,211,153,0.2); }

/* ── Footer ──────────────────────────────────────────────────────────────── */
.modal-footer {
  display: flex; align-items: center; gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}
.settle-summary { flex: 1; font-size: 12px; color: var(--text-secondary); }
.settle-summary.hint { color: var(--text-muted); font-style: italic; }
.err-count { color: var(--error); }

.paste-name-input {
  flex: 1; min-width: 0;
  background: var(--surface-2); border: 1px solid var(--border); border-radius: 5px;
  color: var(--text-primary); font-size: 12.5px; font-family: var(--font-mono);
  padding: 5px 10px; outline: none;
}
.paste-name-input:focus { border-color: var(--accent); }

.footer-btn {
  padding: 6px 16px; font-size: 12.5px; font-weight: 500;
  border-radius: 5px; border: 1px solid var(--border);
  cursor: pointer; transition: background 0.15s, color 0.15s, opacity 0.15s;
  white-space: nowrap; flex-shrink: 0;
}
.footer-btn.secondary { background: transparent; color: var(--text-secondary); }
.footer-btn.secondary:hover:not(:disabled) { background: var(--surface-2); color: var(--text-primary); }
.footer-btn.secondary:disabled { opacity: 0.4; cursor: not-allowed; }
.footer-btn.primary { background: var(--accent); color: #0d1117; border-color: transparent; font-weight: 600; }
.footer-btn.primary:hover:not(:disabled) { background: var(--accent-hover); }
.footer-btn.primary:disabled { opacity: 0.45; cursor: not-allowed; }

/* ── Animations ──────────────────────────────────────────────────────────── */
.spin { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.pending-pulse { animation: pending-pulse 1.5s ease-in-out infinite; }
@keyframes pending-pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.35; } }
</style>
