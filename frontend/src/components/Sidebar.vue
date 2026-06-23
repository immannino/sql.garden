<script setup lang="ts">
import { ref, computed, reactive, onMounted } from 'vue'
import { useSchemaStore } from '../stores/schema'
import { useSelection } from '../composables/useSelection'
import type { CanvasNode } from '../stores/schema'
import {
  ListConnections,
  SaveConnection,
  DeleteConnection,
  ConnectSaved,
  DisconnectSaved,
  AutoConnectAll,
  OpenFileDialog,
  GetDatabaseSchemas,
  GetSchemaTables,
  GetTableColumns,
} from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'

const emit = defineEmits<{
  focusNode: [id: string]
  createQuery: [{ name: string; sql: string }]
}>()

const schemaStore = useSchemaStore()
const { selectedIds, selectNode } = useSelection()

// ── Layers tab ────────────────────────────────────────────────────────────────

// Display order is reversed: index 0 = frontmost (top of list), last = back
const displayLayers = computed(() => [...schemaStore.nodes].reverse())

function onItemClick(e: MouseEvent, node: CanvasNode) {
  selectNode(node.id, e.shiftKey)
  if (!e.shiftKey) emit('focusNode', node.id)
}

// Drag-to-reorder state
const dragId = ref<string | null>(null)
const dropIndex = ref<number | null>(null) // display-space insert position

function onDragStart(e: DragEvent, id: string) {
  dragId.value = id
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onDragOver(e: DragEvent, displayIdx: number) {
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
  const el = (e.currentTarget as HTMLElement)
  const mid = el.getBoundingClientRect().top + el.offsetHeight / 2
  dropIndex.value = e.clientY < mid ? displayIdx : displayIdx + 1
}

// function onDragLeave() {
//   // Only clear if leaving the list entirely — handled by dragend
// }

function onDrop(e: DragEvent) {
  e.preventDefault()
  if (dragId.value === null || dropIndex.value === null) return
  const layers = displayLayers.value
  const fromDisplay = layers.findIndex((n) => n.id === dragId.value)
  if (fromDisplay === -1) return

  const n = layers.length
  // Where the item actually lands in display space after removal
  const effectiveTarget = dropIndex.value <= fromDisplay ? dropIndex.value : dropIndex.value - 1
  if (effectiveTarget === fromDisplay) { dragId.value = null; dropIndex.value = null; return }

  // Convert to store-space indices (display is reversed: display[d] = store[n-1-d])
  const targetStore = n - 1 - effectiveTarget
  const fromStore = n - 1 - fromDisplay
  // moveNodeToIndex uses "final desired position in original array" semantics
  const storeArg = targetStore < fromStore ? targetStore : targetStore + 1
  schemaStore.moveNodeToIndex(dragId.value, storeArg)
  dragId.value = null
  dropIndex.value = null
}

function onDragEnd() {
  dragId.value = null
  dropIndex.value = null
}

// ── Connections tab ───────────────────────────────────────────────────────────

type ConnType = 'postgres' | 'sqlite' | 'duckdb' | 'mysql'

// ── Explorer types ────────────────────────────────────────────────────────────

interface ExplorerCol   { name: string; type: string; nullable: boolean }
interface ExplorerTable { name: string; kind: string; open: boolean; cols: ExplorerCol[] | null; loading: boolean }
interface ExplorerSchema { name: string; open: boolean; tables: ExplorerTable[] | null; loading: boolean }
interface ExplorerState { schemas: ExplorerSchema[] | null; loading: boolean; error: string }

const activeTab = ref<'layers' | 'connections'>('layers')
const connections = ref<main.ConnectionRecord[]>([])
const connectedIds = ref(new Set<string>())
const connectedAliases = reactive<Record<string, string>>({})
const explorer = reactive<Record<string, ExplorerState>>({})
const connectingId = ref<string | null>(null)
const connectErrors = reactive<Record<string, string>>({})
const showForm = ref(false)
const formError = ref('')

const emptyForm = (): { name: string; type: ConnType; dsn: string; readOnly: boolean } => ({
  name: '', type: 'postgres', dsn: '', readOnly: true,
})
const form = ref(emptyForm())

const TYPE_LABELS: Record<ConnType, string> = {
  postgres: 'PostgreSQL',
  sqlite: 'SQLite',
  duckdb: 'DuckDB File',
  mysql: 'MySQL',
}

const TYPE_BADGE: Record<ConnType, string> = {
  postgres: 'PG',
  sqlite: 'SL',
  duckdb: 'DK',
  mysql: 'MY',
}

const DSN_PLACEHOLDER: Record<ConnType, string> = {
  postgres: 'postgres://user:pass@host:5432/dbname',
  sqlite:   '/path/to/database.db',
  duckdb:   '/path/to/database.duckdb',
  mysql:    'mysql://user:pass@host:3306/dbname',
}

const URL_SCHEME_TO_TYPE: Partial<Record<string, ConnType>> = {
  'postgres:': 'postgres', 'postgresql:': 'postgres',
  'mysql:': 'mysql',
  'sqlite:': 'sqlite',
  'duckdb:': 'duckdb',
}

const isFileBased = computed(() => form.value.type === 'sqlite' || form.value.type === 'duckdb')

function onDsnInput() {
  const val = form.value.dsn.trim()
  try {
    const url = new URL(val)
    const detected = URL_SCHEME_TO_TYPE[url.protocol]
    if (!detected) return
    form.value.type = detected
    // Auto-fill name from the database path if still empty
    if (!form.value.name) {
      const db = url.pathname.replace(/^\/+/, '')
      if (db) form.value.name = db
    }
  } catch { /* not a URL — leave type alone */ }
}

async function loadConnections() {
  try {
    connections.value = (await ListConnections()) ?? []
  } catch (e) {
    console.warn('Failed to load connections:', e)
  }
}

async function browseFile() {
  const path = await OpenFileDialog()
  if (path) form.value.dsn = path
}

function openForm() {
  form.value = emptyForm()
  formError.value = ''
  showForm.value = true
}

function cancelForm() {
  showForm.value = false
  formError.value = ''
}

async function submitForm() {
  formError.value = ''
  const name = form.value.name.trim()
  const dsn  = form.value.dsn.trim()
  if (!name) { formError.value = 'Name is required'; return }
  if (!dsn)  { formError.value = 'DSN / path is required'; return }

  try {
    const saved = await SaveConnection({
      id: '', name, type: form.value.type, dsn, readOnly: form.value.readOnly, createdAt: 0,
    } as main.ConnectionRecord)
    connections.value.push(saved)
    showForm.value = false
  } catch (e) {
    formError.value = e instanceof Error ? e.message : String(e)
  }
}

async function removeConnection(id: string) {
  if (connectedIds.value.has(id)) {
    try { await DisconnectSaved(id) } catch { /* IF EXISTS — never throws */ }
    connectedIds.value.delete(id)
    delete connectedAliases[id]
    delete explorer[id]
  }
  delete connectErrors[id]
  await DeleteConnection(id)
  connections.value = connections.value.filter((c) => c.id !== id)
}

async function toggleConnect(conn: main.ConnectionRecord) {
  connectingId.value = conn.id
  delete connectErrors[conn.id]
  try {
    if (connectedIds.value.has(conn.id)) {
      await DisconnectSaved(conn.id)
      connectedIds.value.delete(conn.id)
      delete connectedAliases[conn.id]
      delete explorer[conn.id]
    } else {
      const alias = await ConnectSaved(conn.id)
      connectedIds.value.add(conn.id)
      connectedAliases[conn.id] = alias
      await loadSchemas(conn.id, alias)
    }
  } catch (e) {
    connectErrors[conn.id] = e instanceof Error ? e.message : String(e)
    // Roll back optimistic state if connect failed
    connectedIds.value.delete(conn.id)
    delete connectedAliases[conn.id]
    delete explorer[conn.id]
  } finally {
    connectingId.value = null
  }
}

async function loadSchemas(connId: string, alias: string) {
  explorer[connId] = { schemas: null, loading: true, error: '' }
  try {
    const raw = await GetDatabaseSchemas(alias)
    explorer[connId].schemas = (raw ?? []).map((s) => ({
      name: s.name, open: false, tables: null, loading: false,
    }))
  } catch (e) {
    explorer[connId].error = e instanceof Error ? e.message : String(e)
  } finally {
    explorer[connId].loading = false
  }
}

async function toggleSchema(connId: string, schema: ExplorerSchema) {
  schema.open = !schema.open
  if (schema.open && schema.tables === null) {
    schema.loading = true
    try {
      const alias = connectedAliases[connId]
      const raw = await GetSchemaTables(alias, schema.name)
      schema.tables = (raw ?? []).map((t) => ({
        name: t.name, kind: t.kind, open: false, cols: null, loading: false,
      }))
    } catch { schema.tables = [] }
    finally { schema.loading = false }
  }
}

async function toggleTable(connId: string, schemaName: string, table: ExplorerTable) {
  table.open = !table.open
  if (table.open && table.cols === null) {
    table.loading = true
    try {
      const alias = connectedAliases[connId]
      table.cols = (await GetTableColumns(alias, schemaName, table.name)) ?? []
    } catch { table.cols = [] }
    finally { table.loading = false }
  }
}

function createQueryFromTable(connId: string, schemaName: string, tableName: string) {
  const alias = connectedAliases[connId]
  const sql = `SELECT * FROM "${alias}"."${schemaName}"."${tableName}" LIMIT 1000`
  const name = tableName
  emit('createQuery', { name, sql })
}

onMounted(async () => {
  await loadConnections()
  try {
    const results = await AutoConnectAll()
    for (const r of results ?? []) {
      if (r.error) {
        connectErrors[r.id] = r.error
      } else {
        connectedIds.value.add(r.id)
        connectedAliases[r.id] = r.alias
        loadSchemas(r.id, r.alias) // background — no await
      }
    }
  } catch (e) {
    console.warn('AutoConnectAll failed:', e)
  }
})
</script>

<template>
  <aside class="sidebar">
    <!-- Tab header -->
    <div class="sidebar-tabs">
      <button class="tab-btn" :class="{ active: activeTab === 'layers' }" @click="activeTab = 'layers'">Layers</button>
      <button class="tab-btn" :class="{ active: activeTab === 'connections' }" @click="activeTab = 'connections'">Connections</button>
    </div>

    <!-- ── Layers tab ─────────────────────────────────────────────────────── -->
    <div v-if="activeTab === 'layers'" class="sidebar-body">
      <div class="layers-list" @dragover.prevent @drop="onDrop" @dragend="onDragEnd">
        <div v-if="dropIndex === 0 && dragId !== null" class="drop-indicator" />
        <template v-for="(node, i) in displayLayers" :key="node.id">
          <div
            class="layer-item"
            :class="{ selected: selectedIds.has(node.id), dragging: dragId === node.id }"
            draggable="true"
            @dragstart="onDragStart($event, node.id)"
            @dragover="onDragOver($event, i)"
            @click="onItemClick($event, node)"
          >
            <span class="drag-handle">⠿</span>
            <span class="layer-color-dot" :style="{ background: node.color }" />
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
            <div class="layer-actions">
              <button class="layer-act-btn" title="Bring to front" @click.stop="schemaStore.bringToFront(node.id)">↑</button>
              <button class="layer-act-btn" title="Send to back" @click.stop="schemaStore.sendToBack(node.id)">↓</button>
            </div>
          </div>
          <div v-if="dropIndex === i + 1 && dragId !== null" class="drop-indicator" />
        </template>
      </div>
      <div v-if="!schemaStore.nodes.length" class="empty-hint">No nodes on canvas</div>
    </div>

    <!-- ── Connections tab ────────────────────────────────────────────────── -->
    <div v-else class="sidebar-body">

      <!-- Add-connection form -->
      <div v-if="showForm" class="conn-form">
        <div class="form-row">
          <label class="form-label">Name</label>
          <input v-model="form.name" class="form-input" placeholder="prod_db" @keydown.enter="submitForm" />
        </div>
        <div class="form-row">
          <label class="form-label">Type</label>
          <select v-model="form.type" class="form-input form-select">
            <option v-for="(label, key) in TYPE_LABELS" :key="key" :value="key">{{ label }}</option>
          </select>
        </div>
        <div class="form-row">
          <label class="form-label">{{ isFileBased ? 'File path' : 'Connection string' }}</label>
          <div v-if="isFileBased" class="dsn-row">
            <input v-model="form.dsn" class="form-input dsn-input" :placeholder="DSN_PLACEHOLDER[form.type]" @input="onDsnInput" />
            <button class="browse-btn" title="Browse…" @click="browseFile">…</button>
          </div>
          <textarea v-else v-model="form.dsn" class="form-input form-textarea" :placeholder="DSN_PLACEHOLDER[form.type]" rows="2" @input="onDsnInput" />
        </div>
        <label class="form-checkbox">
          <input v-model="form.readOnly" type="checkbox" />
          <span>Read-only</span>
        </label>
        <div v-if="formError" class="form-error">{{ formError }}</div>
        <div class="form-actions">
          <button class="form-cancel" @click="cancelForm">Cancel</button>
          <button class="form-save" @click="submitForm">Save</button>
        </div>
      </div>

      <!-- "Add" button (hidden while form is open) -->
      <button v-else class="add-conn-btn" @click="openForm">
        <span class="add-icon">+</span> Add connection
      </button>

      <!-- Connection list -->
      <div v-for="conn in connections" :key="conn.id" class="conn-item">
        <div class="conn-top">
          <span class="conn-badge" :class="`badge-${conn.type}`">{{ TYPE_BADGE[conn.type as ConnType] ?? '??' }}</span>
          <span class="conn-name" :title="conn.name">{{ conn.name }}</span>
          <span class="conn-dot" :class="{ connected: connectedIds.has(conn.id) }" :title="connectedIds.has(conn.id) ? 'Connected' : 'Not connected'" />
        </div>
        <div class="conn-actions">
          <button
            class="conn-btn"
            :class="{ 'conn-btn-active': connectedIds.has(conn.id) }"
            :disabled="connectingId === conn.id"
            @click="toggleConnect(conn)"
          >
            {{ connectingId === conn.id ? '…' : connectedIds.has(conn.id) ? 'Disconnect' : 'Connect' }}
          </button>
          <button class="conn-delete" :disabled="connectingId === conn.id" @click="removeConnection(conn.id)">✕</button>
        </div>
        <div v-if="connectErrors[conn.id]" class="conn-error">{{ connectErrors[conn.id] }}</div>

        <!-- Schema explorer tree -->
        <div v-if="explorer[conn.id]" class="explorer">
          <div v-if="explorer[conn.id].loading" class="ex-loading">Loading…</div>
          <div v-else-if="explorer[conn.id].error" class="ex-error">{{ explorer[conn.id].error }}</div>
          <template v-else v-for="schema in explorer[conn.id].schemas" :key="schema.name">
            <!-- Schema row -->
            <button class="ex-schema" @click="toggleSchema(conn.id, schema)">
              <span class="ex-caret">{{ schema.open ? '▾' : '▸' }}</span>
              <span class="ex-schema-name">{{ schema.name }}</span>
              <span v-if="schema.loading" class="ex-spin">⋯</span>
            </button>
            <!-- Tables inside schema -->
            <template v-if="schema.open && schema.tables">
              <template v-for="table in schema.tables" :key="table.name">
                <!-- Table / view row -->
                <div class="ex-table-row">
                  <button class="ex-table" @click="toggleTable(conn.id, schema.name, table)">
                    <span class="ex-caret">{{ table.open ? '▾' : '▸' }}</span>
                    <span class="ex-kind" :class="table.kind === 'view' ? 'ex-kind-view' : 'ex-kind-table'">
                      {{ table.kind === 'view' ? 'V' : 'T' }}
                    </span>
                    <span class="ex-table-name">{{ table.name }}</span>
                    <span v-if="table.loading" class="ex-spin">⋯</span>
                  </button>
                  <button
                    class="ex-table-query-btn"
                    title="Create query node for this table"
                    @click.stop="createQueryFromTable(conn.id, schema.name, table.name)"
                  >+</button>
                </div>
                <!-- Columns inside table -->
                <template v-if="table.open && table.cols">
                  <div v-for="col in table.cols" :key="col.name" class="ex-col">
                    <span class="ex-col-name">{{ col.name }}</span>
                    <span class="ex-col-type">{{ col.type }}</span>
                    <span v-if="col.nullable" class="ex-nullable" title="nullable">?</span>
                  </div>
                </template>
              </template>
            </template>
          </template>
        </div>
      </div>

      <div v-if="!connections.length && !showForm" class="empty-hint">No saved connections</div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: 240px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--surface-1);
  border-right: 1px solid var(--border);
  overflow: hidden;
}

/* ── Tabs ── */
.sidebar-tabs {
  display: flex;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.tab-btn {
  flex: 1;
  padding: 7px 4px 6px;
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-muted);
  background: transparent;
  border: none;
  cursor: pointer;
  transition: color 0.1s, background 0.1s;
}
.tab-btn:hover { color: var(--text-primary); background: var(--surface-2); }
.tab-btn.active { color: var(--accent); border-bottom: 2px solid var(--accent); }

/* ── Shared body ── */
.sidebar-body {
  flex: 1;
  overflow-y: auto;
  padding: 6px 0 12px;
}

/* ── Layers ── */
.layers-list { padding: 4px 0; }

.layer-item {
  display: flex; align-items: center; gap: 6px;
  width: 100%; padding: 4px 8px 4px 4px;
  font-size: 12px; color: var(--text-secondary);
  background: transparent; border: none; border-radius: 0;
  text-align: left; cursor: pointer;
  transition: color 0.1s, background 0.1s; user-select: none;
  position: relative;
}
.layer-item:hover { color: var(--text-primary); background: var(--surface-2); }
.layer-item.selected { color: var(--accent); background: rgba(88, 166, 255, 0.1); }
.layer-item.dragging { opacity: 0.4; }

.drag-handle {
  font-size: 11px; color: var(--text-muted); opacity: 0;
  cursor: grab; flex-shrink: 0; padding: 0 2px; line-height: 1;
  transition: opacity 0.1s;
}
.layer-item:hover .drag-handle { opacity: 0.5; }
.layer-item:active .drag-handle { cursor: grabbing; }

.layer-color-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.node-icon { width: 13px; height: 13px; flex-shrink: 0; opacity: 0.7; }
.layer-item.selected .node-icon { opacity: 1; }
.node-name {
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  font-family: var(--font-mono); font-size: 11.5px; flex: 1; min-width: 0;
}

.layer-actions {
  display: none; gap: 2px; flex-shrink: 0;
}
.layer-item:hover .layer-actions { display: flex; }
.layer-act-btn {
  padding: 1px 4px; border-radius: 3px; font-size: 10px; line-height: 1.4;
  background: var(--surface-2); border: 1px solid var(--border);
  color: var(--text-muted); cursor: pointer;
  transition: color 0.1s, border-color 0.1s;
}
.layer-act-btn:hover { color: var(--accent); border-color: var(--accent); }

.drop-indicator {
  height: 2px; margin: 0 6px;
  background: var(--accent); border-radius: 1px;
  pointer-events: none;
}

/* ── Connections: add button ── */
.add-conn-btn {
  display: flex; align-items: center; gap: 5px;
  width: calc(100% - 24px); margin: 6px 12px;
  padding: 5px 8px; border-radius: 5px;
  font-size: 11.5px; color: var(--text-muted);
  background: transparent; border: 1px dashed var(--border);
  cursor: pointer; transition: color 0.1s, border-color 0.1s;
}
.add-conn-btn:hover { color: var(--accent); border-color: var(--accent); }
.add-icon { font-size: 14px; line-height: 1; }

/* ── Connections: form ── */
.conn-form {
  padding: 8px 10px;
  border-bottom: 1px solid var(--border);
  display: flex; flex-direction: column; gap: 6px;
}
.form-row { display: flex; flex-direction: column; gap: 2px; }
.form-label { font-size: 10px; font-weight: 600; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.04em; }
.form-input {
  background: var(--surface-2); border: 1px solid var(--border);
  border-radius: 4px; color: var(--text-primary);
  font-size: 11.5px; padding: 4px 6px;
  font-family: var(--font-mono, monospace); outline: none;
}
.form-input:focus { border-color: var(--accent); }
.form-select { font-family: var(--font-sans, sans-serif); cursor: pointer; }
.form-textarea { resize: vertical; min-height: 40px; font-size: 11px; }
.dsn-row { display: flex; gap: 4px; }
.dsn-input { flex: 1; min-width: 0; }
.browse-btn {
  padding: 4px 7px; border-radius: 4px;
  background: var(--surface-3, var(--surface-2)); border: 1px solid var(--border);
  color: var(--text-muted); font-size: 12px; cursor: pointer; flex-shrink: 0;
}
.browse-btn:hover { color: var(--text-primary); }
.form-checkbox {
  display: flex; align-items: center; gap: 5px;
  font-size: 11.5px; color: var(--text-secondary); cursor: pointer;
}
.form-error { font-size: 11px; color: #f87171; }
.form-actions { display: flex; gap: 6px; justify-content: flex-end; }
.form-cancel {
  padding: 4px 10px; border-radius: 4px; font-size: 11.5px;
  background: transparent; border: 1px solid var(--border);
  color: var(--text-muted); cursor: pointer;
}
.form-cancel:hover { color: var(--text-primary); }
.form-save {
  padding: 4px 10px; border-radius: 4px; font-size: 11.5px;
  background: var(--accent); border: none; color: #fff; cursor: pointer;
}
.form-save:hover { opacity: 0.88; }

/* ── Connections: list items ── */
.conn-error {
  font-size: 10.5px; color: #f87171;
  padding: 3px 8px 4px; line-height: 1.3;
  word-break: break-word;
}
.conn-item {
  padding: 7px 10px 6px;
  border-bottom: 1px solid var(--border);
}
.conn-top { display: flex; align-items: center; gap: 6px; margin-bottom: 5px; }
.conn-badge {
  font-size: 9px; font-weight: 700; letter-spacing: 0.04em;
  padding: 2px 4px; border-radius: 3px; flex-shrink: 0;
  background: var(--surface-2); color: var(--text-muted);
}
.badge-postgres { background: rgba(59,130,246,0.15); color: #60a5fa; }
.badge-sqlite   { background: rgba(16,185,129,0.15); color: #34d399; }
.badge-duckdb   { background: rgba(245,158,11,0.15); color: #fbbf24; }
.badge-mysql    { background: rgba(239,68,68,0.15);  color: #f87171; }
.conn-name {
  flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  font-family: var(--font-mono); font-size: 11.5px; color: var(--text-primary);
}
.conn-dot {
  width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0;
  background: var(--border);
  transition: background 0.2s;
}
.conn-dot.connected { background: #34d399; }
.conn-actions { display: flex; gap: 5px; align-items: center; }
.conn-btn {
  flex: 1; padding: 3px 0; border-radius: 4px; font-size: 11px;
  background: var(--surface-2); border: 1px solid var(--border);
  color: var(--text-muted); cursor: pointer; transition: all 0.1s;
}
.conn-btn:hover:not(:disabled) { color: var(--accent); border-color: var(--accent); }
.conn-btn.conn-btn-active { color: #f87171; border-color: rgba(248,113,113,0.4); }
.conn-btn.conn-btn-active:hover:not(:disabled) { background: rgba(248,113,113,0.08); }
.conn-btn:disabled { opacity: 0.5; cursor: default; }
.conn-delete {
  padding: 3px 7px; border-radius: 4px; font-size: 11px;
  background: transparent; border: 1px solid transparent;
  color: var(--text-muted); cursor: pointer; transition: all 0.1s; flex-shrink: 0;
}
.conn-delete:hover:not(:disabled) { color: #f87171; border-color: rgba(248,113,113,0.4); }
.conn-delete:disabled { opacity: 0.5; cursor: default; }

/* ── Explorer tree ── */
.explorer { border-top: 1px solid var(--border); padding-bottom: 2px; }
.ex-loading, .ex-error {
  padding: 4px 10px; font-size: 10.5px; color: var(--text-muted); font-style: italic;
}
.ex-error { color: #f87171; font-style: normal; }

.ex-schema {
  display: flex; align-items: center; gap: 4px;
  width: 100%; padding: 3px 8px;
  font-size: 10.5px; font-weight: 600; color: var(--text-secondary);
  background: transparent; border: none; text-align: left; cursor: pointer;
  transition: color 0.1s, background 0.1s;
}
.ex-schema:hover { color: var(--text-primary); background: var(--surface-2); }
.ex-schema-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }

.ex-table-row {
  display: flex;
  align-items: center;
  position: relative;
}
.ex-table-row:hover .ex-table-query-btn { opacity: 1; }

.ex-table {
  display: flex; align-items: center; gap: 4px;
  flex: 1; padding: 2px 8px 2px 18px;
  font-size: 10.5px; color: var(--text-secondary);
  background: transparent; border: none; text-align: left; cursor: pointer;
  transition: color 0.1s, background 0.1s;
  min-width: 0;
}
.ex-table:hover { color: var(--text-primary); background: var(--surface-2); }

.ex-table-query-btn {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  margin-right: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 400;
  line-height: 1;
  color: var(--text-muted);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 3px;
  opacity: 0;
  transition: opacity 0.1s, color 0.1s, background 0.1s, border-color 0.1s;
}
.ex-table-query-btn:hover {
  color: var(--accent);
  background: rgba(88, 166, 255, 0.1);
  border-color: rgba(88, 166, 255, 0.3);
}

.ex-table-name {
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1;
  font-family: var(--font-mono);
}

.ex-kind {
  font-size: 8px; font-weight: 700; padding: 1px 3px; border-radius: 2px; flex-shrink: 0;
  background: var(--surface-2); color: var(--text-muted);
}
.ex-kind-view { background: rgba(139,92,246,0.15); color: #a78bfa; }
.ex-kind-table { background: rgba(59,130,246,0.12); color: #60a5fa; }

.ex-col {
  display: flex; align-items: center; gap: 4px;
  padding: 1px 8px 1px 28px; font-size: 10px;
}
.ex-col-name {
  font-family: var(--font-mono); color: var(--text-secondary);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; min-width: 0;
}
.ex-col-type {
  font-family: var(--font-mono); color: var(--text-muted); font-size: 9.5px;
  flex-shrink: 0; max-width: 70px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.ex-nullable { font-size: 9px; color: var(--text-muted); opacity: 0.6; flex-shrink: 0; }
.ex-caret { font-size: 8px; color: var(--text-muted); flex-shrink: 0; width: 8px; }
.ex-spin { font-size: 10px; color: var(--text-muted); flex-shrink: 0; }

/* ── Shared ── */
.empty-hint {
  padding: 16px 12px; font-size: 11.5px;
  color: var(--text-muted); opacity: 0.5; font-style: italic;
}
</style>
