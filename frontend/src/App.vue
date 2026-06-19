<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import Canvas from './components/Canvas.vue'
import QueryPanel from './components/QueryPanel.vue'
import ImportModal from './components/ImportModal.vue'
import Sidebar from './components/Sidebar.vue'
import SettingsModal from './components/SettingsModal.vue'
import DatasetPickerModal from './components/DatasetPickerModal.vue'
import ChartPropertiesPanel from './components/ChartPropertiesPanel.vue'
import HelpPanel from './components/HelpPanel.vue'
import { useDuckDB } from './composables/useDuckDB'
import { useSchemaStore } from './stores/schema'
import { usePersistence } from './composables/usePersistence'
import { useAppReady } from './composables/useAppReady'
import { useTheme } from './composables/useTheme'
import { useChartPanel } from './composables/useChartPanel'
import { useQueryResults } from './composables/useQueryResults'
import { useChartResults } from './composables/useChartResults'
import { exportData, type ExportFormat } from './lib/exportData'
import { IS_DESKTOP } from './lib/env'
import type { main } from '../wailsjs/go/models'

const { init, isReady, isLoading, initError, exec, query } = useDuckDB()
const schemaStore = useSchemaStore()
const { loadAll, startAutoSave } = usePersistence()
const { markAppReady } = useAppReady()
const { loadTheme } = useTheme()
const { openPanel } = useChartPanel()
const { results: queryResults } = useQueryResults()
const { chartResults } = useChartResults()
const showSettings = ref(false)
const settingsInitialTab = ref<'appearance' | 'mcp' | 'updates' | undefined>(undefined)
const showDatasetPicker = ref(false)
const showHelp = ref(false)
const updateBanner = ref(false)
const loadError = ref<string | null>(null)

const WEB_QUICKSTART = `# Welcome to sql.garden 🌱

An infinite canvas SQL workspace powered by **DuckDB WebAssembly** — everything runs locally in your browser.

---

## Get started

| Action | How |
|---|---|
| Load sample data | **Samples** in the toolbar |
| Import CSV / Parquet / JSON | **Import** or press \`I\` |
| Add a SQL query node | Press \`Q\` |
| Add a chart | Press \`C\` |
| Add a note | Press \`N\` |
| Fit canvas to view | Press \`F\` |

Press **\`?\`** anytime to see all keyboard shortcuts.

---

> Data is **in-memory only** in the browser — re-import or reload a sample each session. Canvas layout is saved automatically.`

function addQueryNode() {
  const center = canvasRef.value?.getCenter() ?? { x: 200, y: 200 }
  const n = schemaStore.nodes.filter((n) => n.kind === 'query').length + 1
  schemaStore.addQueryNode({
    id: `query_${Date.now()}`,
    name: `query_${n}`,
    x: center.x - 140,
    y: center.y - 80,
    sql: 'SHOW TABLES',
  })
}

function addChartNode() {
  const center = canvasRef.value?.getCenter() ?? { x: 200, y: 200 }
  const n = schemaStore.nodes.filter((n) => n.kind === 'chart').length + 1
  const id = `chart_${Date.now()}`
  schemaStore.addChartNode({
    id,
    name: `chart_${n}`,
    x: center.x - 170,
    y: center.y - 120,
    sourceId: null,
    sql: '',
    chartType: 'barY',
    xColumn: '',
    yColumn: '',
  })
  openPanel(id)
}

function addMarkdownNode() {
  const center = canvasRef.value?.getCenter() ?? { x: 200, y: 200 }
  const n = schemaStore.nodes.filter((n) => n.kind === 'markdown').length + 1
  schemaStore.addMarkdownNode({
    id: `md_${Date.now()}`,
    name: `note_${n}`,
    x: center.x - 150,
    y: center.y - 100,
    content: '',
  })
}

function addSection() {
  const center = canvasRef.value?.getCenter() ?? { x: 200, y: 200 }
  schemaStore.addSection({
    id: `section_${Date.now()}`,
    name: 'Section',
    x: center.x - 200,
    y: center.y - 150,
    w: 400,
    h: 300,
  })
}

async function exportCanvasMarkdown() {
  const parts: string[] = []
  for (const node of schemaStore.nodes) {
    if (node.kind === 'markdown') {
      parts.push(`## ${node.name}\n\n${node.content}`)
    } else if (node.kind === 'query') {
      parts.push(`## ${node.name}\n\n\`\`\`sql\n${node.sql}\n\`\`\``)
    } else if (node.kind === 'table') {
      const header = '| Column | Type |\n| --- | --- |'
      parts.push(`## ${node.name}\n\n${header}`)
    }
  }
  const md = `# Canvas Export\n\n${parts.join('\n\n')}`
  if (IS_DESKTOP) {
    const { SaveFileWithDialog } = await import('../wailsjs/go/main/App')
    await SaveFileWithDialog('canvas-export.md', md)
  } else {
    const url = URL.createObjectURL(new Blob([md], { type: 'text/markdown' }))
    const a = document.createElement('a')
    a.href = url; a.download = 'canvas-export.md'
    document.body.appendChild(a); a.click(); document.body.removeChild(a)
    setTimeout(() => URL.revokeObjectURL(url), 150)
  }
}

function onPanelCreate(payload: { type: 'query' | 'chart'; sql: string }) {
  const center = canvasRef.value?.getCenter() ?? { x: 300, y: 300 }
  if (payload.type === 'query') {
    const n = schemaStore.nodes.filter((n) => n.kind === 'query').length + 1
    schemaStore.addQueryNode({
      id: `query_${Date.now()}`,
      name: `query_${n}`,
      x: center.x - 140,
      y: center.y - 80,
      sql: payload.sql,
    })
  } else {
    const n = schemaStore.nodes.filter((n) => n.kind === 'chart').length + 1
    schemaStore.addChartNode({
      id: `chart_${Date.now()}`,
      name: `chart_${n}`,
      x: center.x - 170,
      y: center.y - 120,
      sourceId: null,
      sql: payload.sql,
      chartType: 'barY',
      xColumn: '',
      yColumn: '',
    })
  }
}

const canvasRef = ref<InstanceType<typeof Canvas> | null>(null)
const queryPanelRef = ref<InstanceType<typeof QueryPanel> | null>(null)
const importModalRef = ref<InstanceType<typeof ImportModal> | null>(null)
const webFileInputRef = ref<HTMLInputElement | null>(null)
const showQuery = ref(true)
const showSidebar = ref(true)
const showImport = ref(false)
const initialPathsForModal = ref<string[]>([])

// ── Restore defaults ──────────────────────────────────────────────────────────
const restoreConfirm = ref(false)
let restoreTimer: ReturnType<typeof setTimeout> | null = null

function onRestoreClick() {
  if (!restoreConfirm.value) {
    restoreConfirm.value = true
    restoreTimer = setTimeout(() => { restoreConfirm.value = false }, 3000)
  } else {
    if (restoreTimer) clearTimeout(restoreTimer)
    restoreConfirm.value = false
    restoreDefaults()
  }
}

async function restoreDefaults() {
  try {
    // Drop demo tables in FK-safe order, then recreate via SEED_SQL
    await exec('DROP TABLE IF EXISTS order_items; DROP TABLE IF EXISTS orders; DROP TABLE IF EXISTS products; DROP TABLE IF EXISTS users;')
    await exec(SEED_SQL)
  } catch { /* ignore — tables may not have existed */ }
  schemaStore.clear()
  for (const table of SAMPLE_SCHEMA) {
    schemaStore.addTable(table)
  }
  setTimeout(() => canvasRef.value?.fitView(), 100)
}

const DROPPABLE = /\.(csv|tsv|txt|parquet|json|jsonl|sqlite|db|duckdb)$/i

function handleFileDrop(_x: number, _y: number, paths: string[]) {
  if (!isReady.value) return
  const valid = paths.filter((p) => DROPPABLE.test(p))
  if (!valid.length) return
  if (showImport.value && importModalRef.value) {
    // Modal already open — add directly to its queue
    importModalRef.value.enqueuePaths(valid)
  } else {
    initialPathsForModal.value = valid
    showImport.value = true
  }
}

function onFocusNode(id: string) {
  canvasRef.value?.focusNode(id)
}

// Sample e-commerce schema seeded into DuckDB at startup
const SEED_SQL = `
CREATE TABLE users (
  id        INTEGER PRIMARY KEY,
  name      VARCHAR NOT NULL,
  email     VARCHAR UNIQUE NOT NULL,
  created_at TIMESTAMP
);
INSERT INTO users VALUES
  (1, 'Alice Chen',    'alice@example.com',   '2024-01-15 09:00:00'),
  (2, 'Bob Smith',     'bob@example.com',     '2024-02-20 14:30:00'),
  (3, 'Carol Davis',   'carol@example.com',   '2024-03-05 11:15:00'),
  (4, 'Dave Wilson',   'dave@example.com',    '2024-04-10 16:45:00'),
  (5, 'Eve Martinez',  'eve@example.com',     '2024-05-22 08:20:00');

CREATE TABLE products (
  id    INTEGER PRIMARY KEY,
  name  VARCHAR NOT NULL,
  price DECIMAL(10,2) NOT NULL,
  stock INTEGER DEFAULT 0
);
INSERT INTO products VALUES
  (1, 'Ergonomic Chair',  349.99, 42),
  (2, 'Standing Desk',    599.00, 18),
  (3, 'Monitor 27"',      449.95, 31),
  (4, 'Mechanical Keyboard', 129.99, 85),
  (5, 'USB-C Hub',         49.99, 120);

CREATE TABLE orders (
  id         INTEGER PRIMARY KEY,
  user_id    INTEGER REFERENCES users(id),
  status     VARCHAR DEFAULT 'pending',
  total      DECIMAL(10,2),
  created_at TIMESTAMP
);
INSERT INTO orders VALUES
  (1, 1, 'completed', 999.94, '2024-06-01 10:00:00'),
  (2, 2, 'completed', 349.99, '2024-06-03 14:00:00'),
  (3, 1, 'shipped',   449.95, '2024-06-10 09:30:00'),
  (4, 3, 'pending',   179.98, '2024-06-12 16:00:00'),
  (5, 4, 'completed', 649.98, '2024-06-14 11:00:00');

CREATE TABLE order_items (
  id         INTEGER PRIMARY KEY,
  order_id   INTEGER REFERENCES orders(id),
  product_id INTEGER REFERENCES products(id),
  quantity   INTEGER NOT NULL,
  unit_price DECIMAL(10,2) NOT NULL
);
INSERT INTO order_items VALUES
  (1, 1, 2, 1, 599.00),
  (2, 1, 4, 1, 129.99),
  (3, 1, 5, 2, 49.99),
  (4, 1, 5, 4, 49.99),
  (5, 2, 1, 1, 349.99),
  (6, 3, 3, 1, 449.95),
  (7, 4, 4, 1, 129.99),
  (8, 4, 5, 1, 49.99),
  (9, 5, 1, 1, 349.99),
  (10,5, 3, 1, 449.95);
`

const SAMPLE_SCHEMA = [
  {
    id: 'users',
    name: 'users',
    x: 60,
    y: 80,
    columns: [
      { name: 'id',         type: 'INTEGER',   primaryKey: true  },
      { name: 'name',       type: 'VARCHAR',   nullable: false   },
      { name: 'email',      type: 'VARCHAR',   nullable: false   },
      { name: 'created_at', type: 'TIMESTAMP', nullable: true    },
    ],
  },
  {
    id: 'products',
    name: 'products',
    x: 360,
    y: 320,
    columns: [
      { name: 'id',    type: 'INTEGER',       primaryKey: true },
      { name: 'name',  type: 'VARCHAR',       nullable: false  },
      { name: 'price', type: 'DECIMAL(10,2)', nullable: false  },
      { name: 'stock', type: 'INTEGER',       nullable: true   },
    ],
  },
  {
    id: 'orders',
    name: 'orders',
    x: 360,
    y: 60,
    columns: [
      { name: 'id',         type: 'INTEGER',       primaryKey: true  },
      { name: 'user_id',    type: 'INTEGER',       references: { table: 'users', column: 'id' } },
      { name: 'status',     type: 'VARCHAR',       nullable: true    },
      { name: 'total',      type: 'DECIMAL(10,2)', nullable: true    },
      { name: 'created_at', type: 'TIMESTAMP',     nullable: true    },
    ],
  },
  {
    id: 'order_items',
    name: 'order_items',
    x: 660,
    y: 180,
    columns: [
      { name: 'id',         type: 'INTEGER',       primaryKey: true  },
      { name: 'order_id',   type: 'INTEGER',       references: { table: 'orders',   column: 'id' } },
      { name: 'product_id', type: 'INTEGER',       references: { table: 'products', column: 'id' } },
      { name: 'quantity',   type: 'INTEGER',       nullable: false   },
      { name: 'unit_price', type: 'DECIMAL(10,2)', nullable: false   },
    ],
  },
]

// ── AI node mosaic placement ──────────────────────────────────────────────────
// The grid origin is locked on the FIRST placement of each session so that
// newly-added AI nodes don't shift it right on subsequent calls.
let aiPlacementIndex = 0
let aiOriginX: number | null = null  // null = needs anchoring
let aiOriginY: number | null = null
const AI_W = 340, AI_H = 300, AI_GAP_X = 25, AI_GAP_Y = 25, AI_COLS = 3

function nextAIPosition(): { x: number; y: number } {
  if (aiOriginX === null) {
    // Snapshot the canvas right-edge once; subsequent nodes use the same anchor.
    const nodes = schemaStore.nodes
    if (nodes.length === 0) {
      aiOriginX = 100
      aiOriginY = 80
    } else {
      let maxRight = -Infinity
      let minY = Infinity
      for (const n of nodes) {
        const w = (n as any).w ?? AI_W
        if (n.x + w > maxRight) maxRight = n.x + w
        if (n.y < minY) minY = n.y
      }
      aiOriginX = maxRight + AI_GAP_X
      aiOriginY = minY
    }
  }
  const col = aiPlacementIndex % AI_COLS
  const row = Math.floor(aiPlacementIndex / AI_COLS)
  aiPlacementIndex++
  return {
    x: aiOriginX + col * (AI_W + AI_GAP_X),
    y: (aiOriginY ?? 80) + row * (AI_H + AI_GAP_Y),
  }
}

async function exportNodeData(nodeId: string, fmt: ExportFormat) {
  const node = schemaStore.nodes.find((n) => n.id === nodeId)
  if (!node) return

  if (node.kind === 'markdown') {
    // Markdown nodes: export raw content as .md
    if (IS_DESKTOP) {
      const { SaveFileWithDialog } = await import('../wailsjs/go/main/App')
      await SaveFileWithDialog(`${node.name}.md`, node.content)
    } else {
      const url = URL.createObjectURL(new Blob([node.content], { type: 'text/markdown' }))
      const a = document.createElement('a')
      a.href = url; a.download = `${node.name}.md`
      document.body.appendChild(a); a.click(); document.body.removeChild(a)
      setTimeout(() => URL.revokeObjectURL(url), 150)
    }
    return
  }

  let columns: string[] = []
  let rows: Record<string, unknown>[] = []

  if (node.kind === 'query') {
    const cached = queryResults[nodeId]
    if (cached && !cached.error && !cached.isRunning) {
      columns = cached.columns; rows = cached.rows as Record<string, unknown>[]
    } else if (node.sql.trim()) {
      const result = await query(node.sql)
      columns = result.columns; rows = result.rows as Record<string, unknown>[]
    }
  } else if (node.kind === 'chart') {
    const sourceId = node.sourceId
    const cached = sourceId ? queryResults[sourceId] : chartResults[nodeId]
    if (cached && !cached.error && !cached.isRunning) {
      columns = cached.columns; rows = cached.rows as Record<string, unknown>[]
    } else if (!sourceId && node.sql.trim()) {
      const result = await query(node.sql)
      columns = result.columns; rows = result.rows as Record<string, unknown>[]
    }
  }

  if (columns.length) exportData(fmt, columns, rows, node.name)
}

function handleCanvasAction(action: main.CanvasAction) {
  if (action.type === 'clear') {
    schemaStore.clear()
    aiPlacementIndex = 0; aiOriginX = null; aiOriginY = null
    return
  }
  if (action.type === 'fit_view') {
    if (action.name === 'reset') {
      canvasRef.value?.resetZoom()
    } else {
      setTimeout(() => canvasRef.value?.fitView(), 60)
    }
    return
  }
  if (action.type === 'export') {
    const a = action as any
    const fmt = (a.exportFormat ?? 'csv') as ExportFormat
    const nodeId = a.nodeId as string | undefined
    if (nodeId) exportNodeData(nodeId, fmt)
    return
  }
  const pos = action.hasPosition ? { x: action.x ?? 0, y: action.y ?? 0 } : nextAIPosition()
  const id = action.nodeId ?? `ai_${Date.now()}`
  if (action.type === 'query') {
    schemaStore.addQueryNode({ id, name: action.name, sql: action.sql ?? '', ...pos })
  } else if (action.type === 'chart') {
    schemaStore.addChartNode({
      id, name: action.name, ...pos,
      sql: action.sql ?? '',
      sourceId: action.sourceId ?? null,
      chartType: (action.chartType as any) || 'barY',
      xColumn: action.xColumn ?? '',
      yColumn: action.yColumn ?? '',
      colorColumn: action.colorColumn || undefined,
      labelColumn: action.labelColumn || undefined,
    })
  } else if (action.type === 'markdown') {
    schemaStore.addMarkdownNode({ id, name: action.name, ...pos, content: action.content ?? '' })
  } else if (action.type === 'table') {
    const cols = (action.columns ?? []).map((c) => ({ name: c.name, type: c.type }))
    schemaStore.addTable({ id, name: action.name, ...pos, columns: cols })
    if (action.rowCount) schemaStore.setRowCount(id, action.rowCount)
  }
  if (!action.hasPosition) setTimeout(() => canvasRef.value?.fitView(), 120)
}

async function onImportCreated(_tableName: string) {
  await queryPanelRef.value?.refreshStats()
  setTimeout(() => canvasRef.value?.fitView(), 120)
}

// ── Sample datasets ───────────────────────────────────────────────────────────
async function loadDataset(id: string) {
  showDatasetPicker.value = false
  try {
    let actions: main.CanvasAction[]
    if (IS_DESKTOP) {
      const { LoadSampleDataset } = await import('../wailsjs/go/main/App')
      actions = await LoadSampleDataset(id)
    } else {
      const { loadWebDataset } = await import('./lib/webSampleData')
      actions = await loadWebDataset(id)
    }
    schemaStore.clear()
    aiPlacementIndex = 0; aiOriginX = null; aiOriginY = null
    for (const action of actions) {
      handleCanvasAction(action)
    }
    setTimeout(() => canvasRef.value?.fitView(), 200)
  } catch (e) {
    console.error('loadDataset failed', e)
    loadError.value = e instanceof Error ? e.message : String(e)
    setTimeout(() => { loadError.value = null }, 6000)
  }
}

// ── MCP canvas-action stream ──────────────────────────────────────────────────
let mcpStream: EventSource | null = null

function connectMCPStream() {
  mcpStream = new EventSource('http://localhost:37421/canvas-stream')
  mcpStream.addEventListener('canvas-action', (e) => {
    try { handleCanvasAction(JSON.parse(e.data)) } catch { /* ignore malformed */ }
  })
  // Reset placement index each time the stream (re-)connects so the grid
  // starts fresh relative to the current canvas state.
  mcpStream.addEventListener('open', () => { aiPlacementIndex = 0; aiOriginX = null; aiOriginY = null })
}

async function onToolbarDblClick(e: MouseEvent) {
  if (!IS_DESKTOP) return
  if ((e.target as HTMLElement).closest('button, a, input, select')) return
  const { WindowToggleMaximise } = await import('../wailsjs/runtime/runtime')
  WindowToggleMaximise()
}

function onGlobalKey(e: KeyboardEvent) {
  // Never intercept when focus is in a text field or code editor
  const target = e.target as HTMLElement
  const tag = target.tagName
  const inInput = tag === 'INPUT' || tag === 'TEXTAREA' || target.isContentEditable
    || target.closest('.cm-editor') !== null
  if (inInput) return

  if (e.metaKey || e.ctrlKey) {
    if (e.key === ',') { e.preventDefault(); showSettings.value = !showSettings.value }
    if (e.key === '=' || e.key === '+') { e.preventDefault(); canvasRef.value?.zoomIn() }
    if (e.key === '-') { e.preventDefault(); canvasRef.value?.zoomOut() }
    if (e.key === '0') { e.preventDefault(); canvasRef.value?.fitView() }
    return
  }
  if (e.altKey) return

  switch (e.key.toLowerCase()) {
    case 'q': if (isReady.value) { e.preventDefault(); addQueryNode() } break
    case 'c': if (isReady.value) { e.preventDefault(); addChartNode() } break
    case 'n': e.preventDefault(); addMarkdownNode(); break
    case 's': e.preventDefault(); addSection(); break
    case 'i': if (isReady.value) { e.preventDefault(); showImport.value = true } break
    case 'f': e.preventDefault(); canvasRef.value?.fitView(); break
    case 'l': e.preventDefault(); showSidebar.value = !showSidebar.value; break
    case '/': e.preventDefault(); showQuery.value = !showQuery.value; break
    case '?': e.preventDefault(); showHelp.value = !showHelp.value; break
  }
}

onMounted(async () => {
  if (IS_DESKTOP) {
    const { OnFileDrop, EventsOn } = await import('../wailsjs/runtime/runtime')
    OnFileDrop(handleFileDrop, false)
    connectMCPStream()
    // Native menu → frontend bridge
    EventsOn('menu:add-query',      () => addQueryNode())
    EventsOn('menu:add-chart',      () => addChartNode())
    EventsOn('menu:add-note',       () => addMarkdownNode())
    EventsOn('menu:add-section',    () => addSection())
    EventsOn('menu:import',         () => { showImport.value = true })
    EventsOn('menu:fit-view',       () => canvasRef.value?.fitView())
    EventsOn('menu:zoom-in',        () => canvasRef.value?.zoomIn())
    EventsOn('menu:zoom-out',       () => canvasRef.value?.zoomOut())
    EventsOn('menu:zoom-reset',     () => canvasRef.value?.fitView())
    EventsOn('menu:toggle-layers',  () => { showSidebar.value = !showSidebar.value })
    EventsOn('menu:toggle-query',   () => { showQuery.value = !showQuery.value })
    EventsOn('menu:shortcuts',      () => { showHelp.value = !showHelp.value })
  }
  window.addEventListener('keydown', onGlobalKey)
  // Startup update check — quiet, non-blocking
  if (IS_DESKTOP) {
    import('../wailsjs/go/main/App').then(({ CheckForUpdate }) =>
      CheckForUpdate().then(info => { if (info?.hasUpdate) updateBanner.value = true }).catch(() => {})
    )
  }
  await loadTheme()
  try {
    await init()
    const restored = await loadAll()
    if (!restored) {
      if (IS_DESKTOP) {
        showDatasetPicker.value = true
      } else {
        // Web: seed the canvas with the Quickstart guide so new users aren't
        // staring at a blank screen. The Samples picker can be opened manually.
        schemaStore.addMarkdownNode({
          id: 'web_quickstart',
          name: 'quickstart',
          x: 60,
          y: 60,
          content: WEB_QUICKSTART,
        })
      }
    }

    // Re-register any QueryNodes that were published as views before the session
    // ended. DuckDB is in-memory so views don't survive a restart.
    const viewNodes = schemaStore.nodes
      .filter((n) => n.kind === 'query')
      .map((n) => n as import('./stores/schema').QueryNode)
      .filter((n) => n.isView && n.sql.trim())
    await Promise.allSettled(
      viewNodes.map((n) =>
        exec(`CREATE OR REPLACE VIEW "${n.name.replace(/"/g, '""')}" AS ${n.sql}`),
      ),
    )

    await queryPanelRef.value?.refreshStats()
    startAutoSave()
    markAppReady()
  } catch {
    // initError already set by useDuckDB
  }
})
onUnmounted(async () => {
  if (IS_DESKTOP) {
    const { OnFileDropOff } = await import('../wailsjs/runtime/runtime')
    OnFileDropOff()
    mcpStream?.close()
  }
  window.removeEventListener('keydown', onGlobalKey)
})
</script>

<template>
  <div class="app-shell">
    <!-- Update available banner -->
    <div v-if="updateBanner" class="update-banner">
      <span>A new version of sql.garden is available.</span>
      <button class="update-banner-settings" @click="settingsInitialTab = 'updates'; showSettings = true">View in Settings</button>
      <button class="update-banner-close" @click="updateBanner = false">✕</button>
    </div>

    <!-- Toolbar -->
    <header class="toolbar" @dblclick="onToolbarDblClick">
      <!-- macOS traffic-light spacer (TitleBarHiddenInset — desktop only) -->
      <div v-if="IS_DESKTOP" class="macos-inset" />

      <div class="toolbar-center">
        <div class="db-status" :class="{ ready: isReady, loading: isLoading, error: !!initError }">
          <span class="status-dot" />
          <span class="status-text">
            {{ initError ? 'DuckDB error' : isLoading ? 'Initializing DuckDB…' : 'DuckDB ready' }}
          </span>
        </div>
      </div>

      <div class="toolbar-right">
        <button
          class="toolbar-btn"
          :disabled="!isReady"
          @click="showImport = true"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <path d="M8 2v8M5 7l3 3 3-3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M3 11v1a1 1 0 001 1h8a1 1 0 001-1v-1" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          </svg>
          Import
        </button>
        <!-- Web-only hidden file input — ImportModal triggers it via ref -->
        <input
          v-if="!IS_DESKTOP"
          ref="webFileInputRef"
          type="file"
          accept=".csv,.tsv,.txt,.parquet,.json,.jsonl"
          multiple
          style="display:none"
        />

        <button
          class="toolbar-btn"
          :disabled="!isReady"
          title="Load a sample dataset"
          @click="showDatasetPicker = true"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <rect x="1" y="4" width="14" height="9" rx="1.5" stroke="currentColor" stroke-width="1.3"/>
            <path d="M1 7h14" stroke="currentColor" stroke-width="1.3"/>
            <path d="M5 2h6" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          </svg>
          Samples
        </button>

        <div class="toolbar-divider" />

        <button
          class="toolbar-btn"
          :disabled="!isReady"
          title="Add a section to the canvas"
          @click="addSection"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <rect x="1" y="1" width="14" height="14" rx="2" stroke="currentColor" stroke-width="1.3"/>
            <rect x="1" y="1" width="7" height="5" rx="1.5" fill="currentColor" opacity=".3"/>
            <line x1="4" y1="10" x2="12" y2="10" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" opacity=".5"/>
          </svg>
          Section
        </button>

        <button
          class="toolbar-btn"
          :disabled="!isReady"
          title="Export canvas as Markdown"
          @click="exportCanvasMarkdown"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <path d="M8 10V2M5 5l3-3 3 3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M3 11v1a1 1 0 001 1h8a1 1 0 001-1v-1" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          </svg>
          Export
        </button>

        <button
          class="toolbar-btn"
          :class="{ 'restore-confirm': restoreConfirm }"
          :disabled="!isReady"
          :title="restoreConfirm ? 'Click again to reset canvas to demo data' : 'Restore default demo data'"
          @click="onRestoreClick"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <path d="M2 8A6 6 0 1 1 5 3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            <polyline points="2,1 2,5 6,5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          {{ restoreConfirm ? 'Reset?' : 'Restore' }}
        </button>

        <div class="toolbar-divider" />

        <button
          class="toolbar-btn"
          title="Fit all tables in view"
          @click="canvasRef?.fitView()"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <rect x="1" y="1" width="5" height="5" rx="1" stroke="currentColor" stroke-width="1.3"/>
            <rect x="10" y="1" width="5" height="5" rx="1" stroke="currentColor" stroke-width="1.3"/>
            <rect x="1" y="10" width="5" height="5" rx="1" stroke="currentColor" stroke-width="1.3"/>
            <rect x="10" y="10" width="5" height="5" rx="1" stroke="currentColor" stroke-width="1.3"/>
          </svg>
          Fit View
        </button>

        <button
          class="toolbar-btn"
          :class="{ active: showQuery }"
          @click="showQuery = !showQuery"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <polyline points="2,5 7,10 2,15" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" :transform="showQuery ? 'rotate(90 8 10)' : ''"/>
            <line x1="8" y1="12" x2="14" y2="12" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            <line x1="8" y1="8" x2="14" y2="8" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            <line x1="8" y1="4" x2="14" y2="4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          </svg>
          Query
        </button>
      </div>
    </header>

    <!-- Main content -->
    <div class="main-area">
      <!-- Left rail: sits below macOS traffic lights, contains Layers toggle -->
      <nav class="left-rail">
        <button
          class="rail-btn"
          :class="{ active: showSidebar }"
          title="Toggle layers panel"
          @click="showSidebar = !showSidebar"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <rect x="1" y="1" width="5" height="14" rx="1.5" stroke="currentColor" stroke-width="1.3"/>
            <line x1="9" y1="4" x2="14" y2="4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            <line x1="9" y1="8" x2="14" y2="8" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            <line x1="9" y1="12" x2="12" y2="12" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          </svg>
        </button>

        <!-- Spacer pushes gear to bottom -->
        <div class="rail-spacer" />

        <button
          class="rail-btn"
          :class="{ active: showSettings }"
          title="Settings (⌘,)"
          @click="showSettings = true"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <circle cx="8" cy="8" r="2.2" stroke="currentColor" stroke-width="1.3"/>
            <path d="M8 1v1.5M8 13.5V15M1 8h1.5M13.5 8H15M3.1 3.1l1.1 1.1M11.8 11.8l1.1 1.1M3.1 12.9l1.1-1.1M11.8 4.2l1.1-1.1" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          </svg>
        </button>
      </nav>

      <Sidebar v-if="showSidebar" @focus-node="onFocusNode" />
      <Canvas ref="canvasRef" />
      <QueryPanel v-if="showQuery" ref="queryPanelRef" @close="showQuery = false" @create="onPanelCreate" />

      <!-- Help button + panel -->
      <button
        class="help-btn"
        :class="{ active: showHelp }"
        title="Help & keyboard shortcuts (?)"
        @click="showHelp = !showHelp"
      >?</button>
      <HelpPanel
        v-if="showHelp"
        @close="showHelp = false"
        @open-mcp-settings="showHelp = false; settingsInitialTab = 'mcp'; showSettings = true"
      />

      <!-- Floating action bar -->
      <div class="fab">
        <button class="fab-btn" :disabled="!isReady" title="Query — add SQL node (Q)" @click="addQueryNode">
          <span class="fab-badge">SQL</span>
        </button>

        <div class="fab-divider" />

        <button class="fab-btn" :disabled="!isReady" title="Chart — add chart node (C)" @click="addChartNode">
          <svg viewBox="0 0 16 16" fill="none">
            <rect x="1" y="1" width="14" height="14" rx="2" stroke="currentColor" stroke-width="1.3"/>
            <polyline points="3,11 6,6 9,9 13,4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </button>

        <div class="fab-divider" />

        <button class="fab-btn" title="Note — add markdown node (N)" @click="addMarkdownNode">
          <span class="fab-badge">MD</span>
        </button>
      </div>
    </div>

    <!-- Settings modal -->
    <SettingsModal
      v-if="showSettings"
      :initial-tab="settingsInitialTab"
      @close="showSettings = false; settingsInitialTab = undefined"
    />

    <!-- Dataset picker modal -->
    <DatasetPickerModal
      v-if="showDatasetPicker"
      @close="showDatasetPicker = false"
      @load="loadDataset"
    />

    <!-- Chart properties panel (Figma-style right rail) -->
    <ChartPropertiesPanel />


    <!-- Import modal -->
    <ImportModal
      v-if="showImport"
      ref="importModalRef"
      :initial-paths="initialPathsForModal"
      :web-file-input="webFileInputRef"
@close="showImport = false; initialPathsForModal = []"
      @created="onImportCreated"
    />

    <!-- Dataset load error toast -->
    <Transition name="toast">
      <div v-if="loadError" class="load-error-toast">
        <span>⚠ Failed to load dataset: {{ loadError }}</span>
      </div>
    </Transition>

    <!-- Init error overlay -->
    <div v-if="initError" class="error-overlay">
      <div class="error-card">
        <h2>DuckDB failed to load</h2>
        <pre>{{ initError }}</pre>
        <p class="error-hint">
          This app requires <a href="https://webassembly.org/" target="_blank">WebAssembly</a> support.
          Make sure you're using a modern browser and the dev server is running with the correct headers.
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.app-shell {
  height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.update-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 16px;
  background: rgba(240, 168, 0, 0.12);
  border-bottom: 1px solid rgba(240, 168, 0, 0.3);
  font-size: 12px;
  color: var(--text-secondary);
  flex-shrink: 0;
}
.update-banner-settings {
  font-size: 12px;
  font-weight: 500;
  color: #f0a800;
  background: none;
  border: none;
  padding: 0;
  text-decoration: underline;
}
.update-banner-settings:hover { opacity: 0.8; }
.update-banner-close {
  margin-left: auto;
  font-size: 11px;
  color: var(--text-muted);
  background: none;
  border: none;
  padding: 0 4px;
}
.update-banner-close:hover { color: var(--text-primary); }

.toolbar {
  height: 44px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  padding: 0 12px;
  background: var(--surface-1);
  border-bottom: 1px solid var(--border);
  gap: 12px;
  z-index: 10;
  /* Allow dragging the window from the toolbar */
  --wails-draggable: drag;
}

/* Non-interactive toolbar regions — explicitly draggable so the full bar height works */
.macos-inset,
.toolbar-center,
.toolbar-divider {
  --wails-draggable: drag;
}

/* Interactive elements opt out of drag */
.toolbar button,
.toolbar a,
.toolbar input {
  --wails-draggable: no-drag;
}

/* Spacer that matches the macOS traffic-light inset width */
.macos-inset {
  width: 80px;
  flex-shrink: 0;
  height: 100%;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  justify-content: flex-end;
}

.toolbar-center {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
  height: 100%;
}

.db-status {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11.5px;
  color: var(--text-muted);
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--text-muted);
  transition: background 0.3s;
}

.db-status.loading .status-dot {
  background: var(--warning);
  animation: pulse 1s ease-in-out infinite;
}

.db-status.ready .status-dot {
  background: var(--success);
}

.db-status.ready .status-text {
  color: var(--success);
}

.db-status.error .status-dot {
  background: var(--error);
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.toolbar-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  font-size: 12px;
  white-space: nowrap;
  color: var(--text-secondary);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 5px;
  transition: color 0.15s, border-color 0.15s, background 0.15s;
}

.toolbar-btn svg {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
}

.toolbar-btn:hover {
  color: var(--text-primary);
  background: var(--surface-2);
  border-color: var(--border);
}

.toolbar-divider {
  width: 1px;
  height: 18px;
  background: var(--border);
  flex-shrink: 0;
}

.toolbar-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.toolbar-btn.active {
  color: var(--accent);
  background: rgba(88, 166, 255, 0.08);
  border-color: rgba(88, 166, 255, 0.25);
}

.toolbar-btn.restore-confirm {
  color: var(--error);
  background: rgba(248, 81, 73, 0.08);
  border-color: rgba(248, 81, 73, 0.3);
}

.main-area {
  flex: 1;
  display: flex;
  overflow: hidden;
  position: relative;
}

/* ── Floating action bar ─────────────────────────────────────────────────── */
.fab {
  position: absolute;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 20;
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 5px;
  background: var(--surface-1);
  border: 1px solid var(--border);
  border-radius: 14px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.45), 0 1px 0 rgba(255, 255, 255, 0.04) inset;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  pointer-events: auto;
}

.fab-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 9px 16px;
  color: var(--text-secondary);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 9px;
  cursor: pointer;
  transition: color 0.15s, background 0.15s, border-color 0.15s;

}

.fab-btn svg {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

.fab-badge {
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.05em;
  line-height: 1;
}

.fab-btn:hover:not(:disabled) {
  color: var(--text-primary);
  background: var(--surface-2);
  border-color: var(--border);
}

.fab-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.fab-divider {
  width: 1px;
  height: 36px;
  background: var(--border);
  flex-shrink: 0;
  margin: 0 1px;
}

/* ── Help button ─────────────────────────────────────────────────────────── */
.help-btn {
  position: absolute;
  bottom: 24px;
  right: 24px;
  z-index: 20;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--surface-1);
  border: 1px solid var(--border);
  color: var(--text-muted);
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.35);
  transition: color 0.15s, background 0.15s, border-color 0.15s;

}
.help-btn:hover, .help-btn.active {
  color: var(--text-primary);
  background: var(--surface-2);
  border-color: var(--accent);
}

/* Left rail — sits below the macOS traffic lights */
.left-rail {
  width: 44px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 10px;
  background: var(--surface-1);
  border-right: 1px solid var(--border);
  z-index: 5;
}

.rail-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  color: var(--text-muted);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 6px;
  transition: color 0.15s, border-color 0.15s, background 0.15s;
}

.rail-btn svg {
  width: 16px;
  height: 16px;
}

.rail-btn:hover {
  color: var(--text-primary);
  background: var(--surface-2);
  border-color: var(--border);
}

.rail-btn.active {
  color: var(--accent);
  background: rgba(88, 166, 255, 0.08);
  border-color: rgba(88, 166, 255, 0.25);
}

.rail-spacer {
  flex: 1;
}


.error-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.error-card {
  background: var(--surface-1);
  border: 1px solid var(--error);
  border-radius: 10px;
  padding: 28px 32px;
  max-width: 480px;
  width: 90%;
}

.error-card h2 {
  font-size: 16px;
  color: var(--error);
  margin-bottom: 12px;
}

.error-card pre {
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: var(--text-secondary);
  white-space: pre-wrap;
  margin-bottom: 16px;
}

.error-hint {
  font-size: 13px;
  color: var(--text-muted);
  line-height: 1.5;
}

.error-hint a {
  color: var(--accent);
}

.load-error-toast {
  position: fixed;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  background: #c0392b;
  color: #fff;
  font-size: 12.5px;
  padding: 9px 18px;
  border-radius: 8px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4);
  z-index: 2000;
  white-space: nowrap;
  max-width: 90vw;
  overflow: hidden;
  text-overflow: ellipsis;
}

.toast-enter-active, .toast-leave-active { transition: opacity 0.25s, transform 0.25s; }
.toast-enter-from, .toast-leave-to { opacity: 0; transform: translateX(-50%) translateY(8px); }


</style>
