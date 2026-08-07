<script setup lang="ts">
import { ref, nextTick, onMounted, onUnmounted, watch } from 'vue'
import Canvas from './components/Canvas.vue'
import CanvasTabs from './components/CanvasTabs.vue'
import QueryPanel from './components/QueryPanel.vue'
import ImportModal from './components/ImportModal.vue'
import Sidebar from './components/Sidebar.vue'
import ExercisesPanel from './components/ExercisesPanel.vue'
import TestsPanel from './components/TestsPanel.vue'
import SettingsModal from './components/SettingsModal.vue'
import DatasetPickerModal from './components/DatasetPickerModal.vue'
import ChartPropertiesPanel from './components/ChartPropertiesPanel.vue'
import DataPropertiesPanel from './components/DataPropertiesPanel.vue'
import HelpPanel from './components/HelpPanel.vue'
import SearchPalette from './components/SearchPalette.vue'
import ContextMenu from './components/ContextMenu.vue'
import { useContextMenu } from './composables/useContextMenu'
import type { MenuSection } from './components/ContextMenu.vue'
import { useDuckDB } from './composables/useDuckDB'
import { useSchemaStore } from './stores/schema'
import { usePersistence } from './composables/usePersistence'
import { useAppReady } from './composables/useAppReady'
import { useTheme } from './composables/useTheme'
import { useChartPanel } from './composables/useChartPanel'
import { useQueryResults } from './composables/useQueryResults'
import { useChartResults } from './composables/useChartResults'
import { exportData, type ExportFormat } from './lib/exportData'
import { useSelection } from './composables/useSelection'
import { IS_DESKTOP } from './lib/env'
import { usePendingFullscreen } from './composables/usePendingFullscreen'
import { useExerciseResults } from './composables/useExerciseResults'
import type { main } from '../wailsjs/go/models'

const { init, isReady, isLoading, initError, exec, query, getTableInfo, importTableFromJSON } = useDuckDB()
const schemaStore = useSchemaStore()
const { loadAll, startAutoSave } = usePersistence()
const { markAppReady } = useAppReady()
const { loadTheme } = useTheme()
const { selectedIds } = useSelection()
const { contextMenu, close: closeContextMenu } = useContextMenu()
const { openPanel, closePanel } = useChartPanel()
const { results: queryResults } = useQueryResults()
const { chartResults } = useChartResults()
const { pendingFullscreenId } = usePendingFullscreen()
const { navigateRequest } = useExerciseResults()
const showPalette = ref(false)
const showSettings = ref(false)
const settingsInitialTab = ref<'appearance' | 'mcp' | 'updates' | undefined>(undefined)
const showDatasetPicker = ref(false)
const showHelp = ref(false)
const updateBanner = ref(false)
const loadError = ref<string | null>(null)
const pendingPackUrl = ref<string | null>(null)
const packImporting = ref(false)

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
| Jump to any node | Press \`⌘K\` |

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

function addIngestNode() {
  const center = canvasRef.value?.getCenter() ?? { x: 200, y: 200 }
  const n = schemaStore.nodes.filter((n) => n.kind === 'ingest').length + 1
  schemaStore.addIngestNode({
    id: `ingest_${Date.now()}`,
    name: `ingest_${n}`,
    x: center.x - 170,
    y: center.y - 80,
    color: '#6366f1',
    mode: 'generator',
    sql: '',
    url: '',
    targetTable: '',
    conflictMode: 'append',
    interval: 0,
  })
}

function addExerciseNode() {
  const center = canvasRef.value?.getCenter() ?? { x: 200, y: 200 }
  const n = schemaStore.nodes.filter((n) => n.kind === 'exercise').length + 1
  schemaStore.addExerciseNode({
    id: `exercise_${Date.now()}`,
    name: `exercise_${n}`,
    x: center.x - 220,
    y: center.y - 160,
    color: '#f59e0b',
    sql: '',
    prompt: '',
    checks: [],
    revealHintsAfter: 0,
  })
}

function addTestNode() {
  const n = schemaStore.nodes.filter((n) => n.kind === 'test').length + 1
  schemaStore.addTestNode({
    id: `test_${Date.now()}`,
    name: `Test ${n}`,
    x: 100 + Math.random() * 200,
    y: 100 + Math.random() * 200,
    sql: 'SELECT 1 AS ok',
    assertionMode: 'no_rows',
    interval: 0,
    history: [],
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

function onPanelCreate(payload: { type: 'query' | 'chart'; sql: string; openFullscreen?: boolean }) {
  const center = canvasRef.value?.getCenter() ?? { x: 300, y: 300 }
  if (payload.type === 'query') {
    const n = schemaStore.nodes.filter((n) => n.kind === 'query').length + 1
    const id = `query_${Date.now()}`
    schemaStore.addQueryNode({
      id,
      name: `query_${n}`,
      x: center.x - 140,
      y: center.y - 80,
      sql: payload.sql,
    })
    if (payload.openFullscreen) pendingFullscreenId.value = id
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

// ── Canvas tab management ─────────────────────────────────────────────────────
function onTabSwitch(id: string) {
  if (id === schemaStore.activeCanvasId) return
  // Save current viewport before switching
  if (canvasRef.value) schemaStore.saveViewport(schemaStore.activeCanvasId, canvasRef.value.getViewport())
  closePanel()
  pendingFullscreenId.value = null
  schemaStore.switchCanvas(id)
  nextTick(() => {
    const vp = schemaStore.getViewport(id)
    if (vp) canvasRef.value?.setViewport(vp.x, vp.y, vp.zoom)
    else canvasRef.value?.fitView()
  })
}

function onTabAdd() {
  if (canvasRef.value) schemaStore.saveViewport(schemaStore.activeCanvasId, canvasRef.value.getViewport())
  closePanel()
  schemaStore.addCanvas()
  nextTick(() => canvasRef.value?.fitView())
}

function onTabRemove(id: string) {
  schemaStore.removeCanvas(id)
  if (id !== schemaStore.activeCanvasId) return
  nextTick(() => {
    const vp = schemaStore.getViewport(schemaStore.activeCanvasId)
    if (vp) canvasRef.value?.setViewport(vp.x, vp.y, vp.zoom)
    else canvasRef.value?.fitView()
  })
}

const canvasRef = ref<InstanceType<typeof Canvas> | null>(null)
const queryPanelRef = ref<InstanceType<typeof QueryPanel> | null>(null)
const importModalRef = ref<InstanceType<typeof ImportModal> | null>(null)
const webFileInputRef = ref<HTMLInputElement | null>(null)
const showQuery = ref(true)
const showSidebar = ref(true)
const showAssertions = ref(false)
const showTests = ref(false)
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

async function handleFileDrop(_x: number, _y: number, paths: string[]) {
  if (!isReady.value) return

  const bundles = paths.filter((p) => p.endsWith('.sql.garden.json'))
  const dataFiles = paths.filter((p) => DROPPABLE.test(p) && !p.endsWith('.sql.garden.json'))

  for (const path of bundles) {
    try {
      const { ReadTextFile } = await import('../wailsjs/go/main/App')
      const text = await ReadTextFile(path)
      const bundle = JSON.parse(text)
      if (bundle.version === 1 && Array.isArray(bundle.nodes)) {
        await importNodeBundle(bundle.nodes)
      } else {
        throw new Error('Not a valid node bundle')
      }
    } catch (err) {
      loadError.value = `Node import failed: ${err instanceof Error ? err.message : String(err)}`
      setTimeout(() => { loadError.value = null }, 6000)
    }
  }

  if (!dataFiles.length) return
  if (showImport.value && importModalRef.value) {
    importModalRef.value.enqueuePaths(dataFiles)
  } else {
    initialPathsForModal.value = dataFiles
    showImport.value = true
  }
}

function onFocusNode(id: string) {
  canvasRef.value?.focusNode(id)
}

function onPaletteSelect(id: string) {
  canvasRef.value?.focusNode(id)
}

function contextMenuSections(): MenuSection[] {
  const cm = contextMenu.value
  if (!cm) return []

  if (cm.type === 'canvas') {
    return [
      { label: 'Add Query node',    shortcut: 'Q', action: () => { if (isReady.value) addQueryNode() } },
      { label: 'Add Chart node',    shortcut: 'C', action: () => { if (isReady.value) addChartNode() } },
      { label: 'Add Ingest node',     shortcut: 'G', action: () => { if (isReady.value) addIngestNode() } },
      { label: 'Add Exercise node',   shortcut: 'E', action: () => { if (isReady.value) addExerciseNode() } },
      { label: 'Add Test node',       shortcut: 'T', action: () => { if (isReady.value) addTestNode() } },
      { label: 'Add Markdown note',   shortcut: 'N', action: addMarkdownNode },
      { label: 'Add Section',       shortcut: 'S', action: addSection },
      { divider: true },
      { label: 'Import nodes from .sql.garden.json…', action: () => jsonImportInputRef.value?.click() },
      { divider: true },
      { label: 'Fit View', shortcut: 'F', action: () => canvasRef.value?.fitView() },
    ]
  }

  // node menu
  const { id } = cm
  const node = schemaStore.nodes.find((n) => n.id === id)
  if (!node) return []

  const isSection = node.kind === 'section'
  const multiNonSection = selectedIds.value.size >= 2 &&
    [...selectedIds.value].every((sid) => schemaStore.nodes.find((n) => n.id === sid)?.kind !== 'section')

  const items: MenuSection[] = []

  if (isSection) {
    items.push({ label: 'Fit to Contents', action: () => fitSectionContents(id) })
    items.push({ label: 'Mosaic Contents', action: () => mosaicSectionContents(id) })
    items.push({ divider: true })
  }

  if (multiNonSection) {
    items.push({ label: 'Wrap in Section', action: wrapInSection })
    items.push({ divider: true })
  }

  if (!isSection) {
    items.push({
      label: 'Duplicate',
      shortcut: '⌘D',
      action: () => {
        const newId = schemaStore.duplicateNode(id)
        if (newId) { selectedIds.value = new Set([newId]); canvasRef.value?.focusNode(newId) }
      },
    })
  }

  // Cross-tab copy — show one item per other canvas
  const copyIds = selectedIds.value.size > 1 && selectedIds.value.has(id)
    ? [...selectedIds.value].filter(sid => schemaStore.nodes.find(n => n.id === sid)?.kind !== 'section')
    : (node.kind !== 'section' ? [id] : [])
  const otherCanvases = schemaStore.canvases.filter(c => c.id !== schemaStore.activeCanvasId)
  if (copyIds.length && otherCanvases.length) {
    items.push({ divider: true })
    for (const canvas of otherCanvases) {
      items.push({
        label: `Copy to "${canvas.name}"`,
        action: () => schemaStore.copyNodesToCanvas(copyIds, canvas.id),
      })
    }
  }

  if (!isSection) items.push({ divider: true })

  const exportIds = selectedIds.value.size > 1 && selectedIds.value.has(id)
    ? [...selectedIds.value]
    : [id]
  items.push(
    { label: 'Bring to Front', action: () => schemaStore.bringToFront(id) },
    { label: 'Send to Back',   action: () => schemaStore.sendToBack(id) },
    { divider: true },
    {
      label: exportIds.length > 1 ? `Export ${exportIds.length} nodes…` : 'Export node…',
      action: () => doExportNodes(exportIds),
    },
    { divider: true },
    {
      label: 'Delete',
      danger: true,
      action: () => {
        schemaStore.removeNode(id)
        selectedIds.value = new Set()
      },
    },
  )

  return items
}

function onCreateQueryFromConnection(payload: { name: string; sql: string }) {
  const center = canvasRef.value?.getCenter() ?? { x: 200, y: 200 }
  const baseName = payload.name
  const names = new Set(schemaStore.nodes.map((n) => n.name))
  let name = baseName
  let i = 2
  while (names.has(name)) name = `${baseName}_${i++}`
  const id = `query_${Date.now()}`
  schemaStore.addQueryNode({ id, name, x: center.x - 140, y: center.y - 80, sql: payload.sql })
  canvasRef.value?.focusNode(id)
}

// ── Node export / import ──────────────────────────────────────────────────────

const jsonImportInputRef = ref<HTMLInputElement | null>(null)

import type { CanvasNode, ExerciseCheck } from './stores/schema'

function serializeForExport(node: CanvasNode): Record<string, unknown> {
  // Spread all fields; drop sqlHistory (internal undo state, not useful in export)
  const { sqlHistory: _drop, ...rest } = node as any
  return rest
}

const CLIPBOARD_TAG = '__sqlgarden_nodes__'

async function copySelectedToClipboard() {
  const ids = [...selectedIds.value]
  if (!ids.length) return
  const toCopy = schemaStore.nodes.filter(n => ids.includes(n.id))
  try {
    await navigator.clipboard.writeText(JSON.stringify({ [CLIPBOARD_TAG]: true, nodes: toCopy.map(serializeForExport) }))
  } catch { /* clipboard access denied */ }
}

async function pasteFromClipboard() {
  try {
    const text = await navigator.clipboard.readText()
    if (!text) return
    const data = JSON.parse(text)
    if (data[CLIPBOARD_TAG] !== true || !Array.isArray(data.nodes)) return
    await importNodeBundle(data.nodes)
  } catch { /* not our format or access denied */ }
}

async function doExportNodes(nodeIds: string[]) {
  const toExport = schemaStore.nodes.filter((n) => nodeIds.includes(n.id))
  if (!toExport.length) return
  const exportedNodes: Record<string, unknown>[] = []
  for (const node of toExport) {
    const n = serializeForExport(node)
    if ((node.kind === 'data' || node.kind === 'table') && isReady.value) {
      try {
        const res = await query(`SELECT * FROM "${node.name}"`)
        n._rows = res.rows
      } catch { /* table not in DuckDB — export metadata only */ }
    }
    exportedNodes.push(n)
  }
  const bundle = { version: 1, exportedAt: new Date().toISOString(), nodes: exportedNodes }
  const json = JSON.stringify(bundle, null, 2)
  const filename = toExport.length === 1 ? `${toExport[0].name}.sql.garden.json` : 'canvas_nodes.sql.garden.json'
  if (IS_DESKTOP) {
    const { SaveFileWithDialog } = await import('../wailsjs/go/main/App')
    await SaveFileWithDialog(filename, json)
  } else {
    const url = URL.createObjectURL(new Blob([json], { type: 'application/json' }))
    const a = document.createElement('a')
    a.href = url; a.download = filename
    document.body.appendChild(a); a.click(); document.body.removeChild(a)
    setTimeout(() => URL.revokeObjectURL(url), 150)
  }
}

async function exportCanvasTab(canvasId: string) {
  const canvas = schemaStore.canvases.find(c => c.id === canvasId)
  if (!canvas || !canvas.nodes.length) return
  const exportedNodes: Record<string, unknown>[] = []
  for (const node of canvas.nodes) {
    const n = serializeForExport(node)
    if ((node.kind === 'data' || node.kind === 'table') && isReady.value) {
      try {
        const res = await query(`SELECT * FROM "${node.name}"`)
        n._rows = res.rows
      } catch { /* table not in DuckDB — export metadata only */ }
    }
    exportedNodes.push(n)
  }
  const bundle = { version: 1, exportedAt: new Date().toISOString(), nodes: exportedNodes }
  const json = JSON.stringify(bundle, null, 2)
  const filename = `${canvas.name}.sql.garden.json`
  if (IS_DESKTOP) {
    const { SaveFileWithDialog } = await import('../wailsjs/go/main/App')
    await SaveFileWithDialog(filename, json)
  } else {
    const url = URL.createObjectURL(new Blob([json], { type: 'application/json' }))
    const a = document.createElement('a')
    a.href = url; a.download = filename
    document.body.appendChild(a); a.click(); document.body.removeChild(a)
    setTimeout(() => URL.revokeObjectURL(url), 150)
  }
}

async function onNodeImportFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  try {
    const text = await file.text()
    const bundle = JSON.parse(text)
    if (bundle.version !== 1 || !Array.isArray(bundle.nodes)) throw new Error('Not a valid node bundle (expected version:1)')
    await importNodeBundle(bundle.nodes)
  } catch (err) {
    loadError.value = `Node import failed: ${err instanceof Error ? err.message : String(err)}`
    setTimeout(() => { loadError.value = null }, 6000)
  }
}

async function importNodeBundle(rawNodes: Record<string, unknown>[]) {
  if (!rawNodes.length) return
  schemaStore.snapshot()

  const idMap: Record<string, string> = {}
  const existingNames = new Set(schemaStore.nodes.map((n) => n.name))
  const ts = Date.now()

  function uniqueName(base: string): string {
    let name = base
    let i = 2
    while (existingNames.has(name)) name = `${base}_${i++}`
    existingNames.add(name)
    return name
  }

  // Center the imported bundle on the current viewport
  const center = canvasRef.value?.getCenter() ?? { x: 400, y: 300 }
  let minX = Infinity, minY = Infinity
  for (const n of rawNodes) {
    if (typeof n.x === 'number' && n.x < minX) minX = n.x
    if (typeof n.y === 'number' && n.y < minY) minY = n.y
  }
  const dx = isFinite(minX) ? center.x - minX - 120 : 40
  const dy = isFinite(minY) ? center.y - minY - 80 : 40

  // Sort: charts last so their sourceId refs are already in idMap
  const sorted = [...rawNodes].sort((a, b) => (a.kind === 'chart' ? 1 : 0) - (b.kind === 'chart' ? 1 : 0))

  for (let i = 0; i < sorted.length; i++) {
    const node = sorted[i] as any
    const oldId = String(node.id ?? '')
    const newId = `${node.kind}_imp_${ts}_${i}`
    if (oldId) idMap[oldId] = newId
    const name = uniqueName(String(node.name ?? `imported_${i}`))
    const x = (typeof node.x === 'number' ? node.x : 0) + dx
    const y = (typeof node.y === 'number' ? node.y : 0) + dy
    const base = { id: newId, name, x, y, color: node.color, w: node.w, h: node.h, viewMode: node.viewMode }

    if (node.kind === 'query') {
      schemaStore.addQueryNode({ ...base, sql: node.sql ?? '' })
    } else if (node.kind === 'markdown') {
      schemaStore.addMarkdownNode({ ...base, content: node.content ?? '' })
    } else if (node.kind === 'section') {
      schemaStore.addSection({ ...base, w: node.w ?? 300, h: node.h ?? 200 })
    } else if (node.kind === 'chart') {
      const sourceId = node.sourceId ? (idMap[node.sourceId] ?? null) : null
      schemaStore.addChartNode({
        ...base,
        sourceId,
        sql: node.sql ?? '',
        chartType: node.chartType ?? 'barY',
        xColumn: node.xColumn ?? '',
        yColumn: node.yColumn ?? '',
        colorColumn: node.colorColumn,
        labelColumn: node.labelColumn,
        chartLabel: node.chartLabel,
        mermaidCode: node.mermaidCode,
        conditions: node.conditions,
        tableColumnConfigs: node.tableColumnConfigs,
        trueText: node.trueText,
        falseText: node.falseText,
        trueColor: node.trueColor,
        falseColor: node.falseColor,
      })
    } else if (node.kind === 'ingest') {
      schemaStore.addIngestNode({
        ...base,
        mode: node.mode ?? 'generator',
        sql: node.sql ?? '',
        url: node.url ?? '',
        targetTable: node.targetTable ?? '',
        conflictMode: node.conflictMode ?? 'append',
        interval: node.interval ?? 0,
      })
    } else if (node.kind === 'exercise') {
      schemaStore.addExerciseNode({
        ...base,
        sql: typeof node.sql === 'string' ? node.sql : '',
        prompt: node.prompt ?? '',
        successText: typeof node.successText === 'string' ? node.successText : '',
        checks: Array.isArray(node.checks) ? node.checks : [],
        revealHintsAfter: typeof node.revealHintsAfter === 'number' ? node.revealHintsAfter : 0,
        nextId: typeof node.nextId === 'string' ? node.nextId : undefined,
      })
    } else if (node.kind === 'data' || node.kind === 'table') {
      let columns = Array.isArray(node.columns) ? node.columns : []
      let rowCount = typeof node.rowCount === 'number' ? node.rowCount : 0
      if (Array.isArray(node._rows) && node._rows.length > 0) {
        await importTableFromJSON(name, JSON.stringify(node._rows))
        columns = await getTableInfo(name)
        const rc = await query(`SELECT COUNT(*) AS n FROM "${name}"`)
        rowCount = Number(rc.rows[0]?.n ?? 0)
      }
      if (node.kind === 'data') {
        schemaStore.addDataNode({ ...base, name, columns, rowCount, columnCasts: node.columnCasts, sourceSql: node.sourceSql })
      } else {
        schemaStore.addTable({ ...base, name, columns, rowCount, columnCasts: node.columnCasts })
      }
    }
  }
}

async function confirmPackImport() {
  const packURL = pendingPackUrl.value
  if (!packURL) return
  packImporting.value = true
  try {
    let text: string
    if (IS_DESKTOP) {
      const { FetchPackJSON } = await import('../wailsjs/go/main/App')
      text = await FetchPackJSON(packURL)
    } else {
      const resp = await fetch(packURL)
      if (!resp.ok) throw new Error(`Server returned ${resp.status}`)
      text = await resp.text()
    }
    const bundle = JSON.parse(text)
    if (bundle.version !== 1 || !Array.isArray(bundle.nodes)) throw new Error('Not a valid sql.garden pack (expected version:1)')
    await importNodeBundle(bundle.nodes)
    pendingPackUrl.value = null
  } catch (err) {
    loadError.value = `Pack import failed: ${err instanceof Error ? err.message : String(err)}`
    setTimeout(() => { loadError.value = null }, 8000)
    pendingPackUrl.value = null
  } finally {
    packImporting.value = false
  }
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
  // ── Canvas tab lifecycle ────────────────────────────────────────────────────
  if (action.type === 'add_canvas') {
    schemaStore.addCanvas({ id: action.canvasId || undefined, name: action.name || undefined })
    return
  }
  if (action.type === 'remove_canvas') {
    if (action.canvasId) schemaStore.removeCanvas(action.canvasId)
    return
  }
  if (action.type === 'rename_canvas') {
    if (action.canvasId && action.name) schemaStore.renameCanvas(action.canvasId, action.name)
    return
  }
  if (action.type === 'switch_canvas') {
    if (action.canvasId) onTabSwitch(action.canvasId)
    return
  }

  // ── Canvas-wide operations ──────────────────────────────────────────────────
  if (action.type === 'clear') {
    if (action.canvasId) {
      const tab = schemaStore.canvases.find(c => c.id === action.canvasId)
      if (tab) tab.nodes = []
    } else {
      schemaStore.clear()
      aiPlacementIndex = 0; aiOriginX = null; aiOriginY = null
    }
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

  // ── Node mutation (no canvas targeting needed — nodes are referenced by id) ─
  if (action.type === 'resize_node') {
    if (action.nodeId && action.width && action.height)
      schemaStore.updateNodeSize(action.nodeId, action.width, action.height)
    return
  }
  if (action.type === 'move_node') {
    if (action.nodeId != null && action.x != null && action.y != null)
      schemaStore.updatePositions(new Map([[action.nodeId, { x: action.x, y: action.y }]]))
    return
  }
  if (action.type === 'focus_node') {
    if (action.nodeId) {
      const node = action.nodeId
      setTimeout(() => canvasRef.value?.focusNode(node), 60)
    }
    return
  }
  if (action.type === 'update_query') {
    if (action.nodeId) {
      schemaStore.updateQuerySql(action.nodeId, action.sql ?? '')
      if (action.name) {
        const node = schemaStore.nodes.find(n => n.id === action.nodeId)
        if (node) node.name = action.name
      }
    }
    return
  }
  if (action.type === 'set_color') {
    if (action.nodeId) schemaStore.setNodeColor(action.nodeId, action.name)
    return
  }

  // ── Node creation — route to target canvas if specified ────────────────────
  const pos = action.hasPosition ? { x: action.x ?? 0, y: action.y ?? 0 } : nextAIPosition()
  const id = action.nodeId ?? `ai_${Date.now()}`
  const targetCanvasId = (action as any).canvasId as string | undefined

  const withTarget = (fn: () => void) => {
    if (targetCanvasId) schemaStore.addNodeToCanvas(targetCanvasId, fn)
    else fn()
  }

  if (action.type === 'query') {
    withTarget(() => schemaStore.addQueryNode({ id, name: action.name, sql: action.sql ?? '', ...pos }))
  } else if (action.type === 'chart') {
    withTarget(() => schemaStore.addChartNode({
      id, name: action.name, ...pos,
      sql: action.sql ?? '',
      sourceId: action.sourceId ?? null,
      chartType: (action.chartType as any) || 'barY',
      xColumn: action.xColumn ?? '',
      yColumn: action.yColumn ?? '',
      colorColumn: action.colorColumn || undefined,
      labelColumn: action.labelColumn || undefined,
    }))
  } else if (action.type === 'markdown') {
    withTarget(() => schemaStore.addMarkdownNode({ id, name: action.name, ...pos, content: action.content ?? '' }))
  } else if (action.type === 'table') {
    const cols = (action.columns ?? []).map((c) => ({ name: c.name, type: c.type }))
    withTarget(() => {
      schemaStore.addTable({ id, name: action.name, ...pos, columns: cols })
      if (action.rowCount) schemaStore.setRowCount(id, action.rowCount)
    })
  } else if (action.type === 'data') {
    const cols = (action.columns ?? []).map((c) => ({ name: c.name, type: c.type }))
    withTarget(() => schemaStore.addDataNode({
      id,
      name: action.name,
      ...pos,
      columns: cols,
      rowCount: Number(action.rowCount ?? 0),
      sourceId: action.sourceId || undefined,
      sourceSql: action.sql || undefined,
    }))
  } else if (action.type === 'section') {
    withTarget(() => schemaStore.addSection({
      id,
      name: action.name,
      ...pos,
      w: action.width || 400,
      h: action.height || 300,
    }))
  } else if (action.type === 'ingest') {
    withTarget(() => schemaStore.addIngestNode({
      id,
      name: action.name,
      ...pos,
      color: '#6366f1',
      mode: (action.mode as 'generator' | 'ingestion') || 'generator',
      sql: action.sql ?? '',
      url: action.url ?? '',
      targetTable: action.targetTable ?? '',
      conflictMode: (action.conflictMode as 'append' | 'replace') || 'append',
      interval: Number(action.interval ?? 0),
    }))
  } else if (action.type === 'exercise') {
    withTarget(() => schemaStore.addExerciseNode({
      id,
      name: action.name,
      ...pos,
      color: '#f59e0b',
      sql: action.sql ?? '',
      prompt: action.content ?? '',
      checks: Array.isArray(action.checks) ? action.checks as ExerciseCheck[] : [],
      revealHintsAfter: action.revealHintsAfter ?? 0,
      successText: action.successText ?? '',
      nextId: action.nextId || undefined,
    }))
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
    loadError.value = `Failed to load dataset: ${e instanceof Error ? e.message : String(e)}`
    setTimeout(() => { loadError.value = null }, 6000)
  }
}

async function loadLearnTrack(id: string) {
  showDatasetPicker.value = false
  try {
    const { LoadLearnTrack } = await import('../wailsjs/go/main/App')
    const actions = await LoadLearnTrack(id)
    schemaStore.clear()
    aiPlacementIndex = 0; aiOriginX = null; aiOriginY = null
    for (const action of actions) {
      handleCanvasAction(action)
    }
    setTimeout(() => canvasRef.value?.fitView(), 200)
  } catch (e) {
    console.error('loadLearnTrack failed', e)
    loadError.value = `Failed to load learning track: ${e instanceof Error ? e.message : String(e)}`
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

// Clipboard shim for plain <input>/<textarea> elements in Wails on macOS.
// CodeMirror editors handle their own clipboard via SqlEditor.vue keymaps.
async function onDesktopClipboardKey(e: KeyboardEvent) {
  if (!e.metaKey && !e.ctrlKey) return
  const target = e.target as HTMLElement
  const tag = target.tagName
  if (tag !== 'INPUT' && tag !== 'TEXTAREA') return
  if (target.closest('.cm-editor')) return // CodeMirror handles its own

  const el = target as HTMLInputElement | HTMLTextAreaElement
  const { ClipboardGet, ClipboardSet } = await import('../wailsjs/go/main/App')

  if (e.key === 'a') {
    e.preventDefault()
    el.select()
  } else if (e.key === 'c') {
    const text = el.value.slice(el.selectionStart ?? 0, el.selectionEnd ?? el.value.length)
    if (text) { e.preventDefault(); ClipboardSet(text) }
  } else if (e.key === 'x') {
    const start = el.selectionStart ?? 0
    const end = el.selectionEnd ?? el.value.length
    const text = el.value.slice(start, end)
    if (text) {
      e.preventDefault()
      ClipboardSet(text)
      el.setRangeText('', start, end, 'end')
      el.dispatchEvent(new Event('input', { bubbles: true }))
    }
  } else if (e.key === 'v') {
    const text = await ClipboardGet()
    if (text) {
      e.preventDefault()
      const start = el.selectionStart ?? el.value.length
      const end = el.selectionEnd ?? el.value.length
      el.setRangeText(text, start, end, 'end')
      el.dispatchEvent(new Event('input', { bubbles: true }))
    }
    // If ClipboardGet returns empty, fall through to native paste
  }
}

// ── Multi-node alignment ───────────────────────────────────────────────────────
type AlignOp = 'left' | 'centerH' | 'right' | 'top' | 'middleV' | 'bottom' | 'distH' | 'distV'

function getAlignBounds() {
  return [...selectedIds.value].map((id) => {
    const n = schemaStore.nodes.find((n) => n.id === id)
    if (!n) return null
    const w = ('w' in n ? n.w : undefined) ?? 280
    const h = ('h' in n ? n.h : undefined) ?? 120
    return { id, x: n.x, y: n.y, w, h }
  }).filter(Boolean) as Array<{ id: string; x: number; y: number; w: number; h: number }>
}

function autoMosaic() {
  const canvas = canvasRef.value
  if (!canvas) return
  const target = selectedIds.value.size > 0
    ? schemaStore.nodes.filter((n) => selectedIds.value.has(n.id))
    : schemaStore.nodes
  if (target.length < 2) return
  const vp = canvas.getViewportRect()
  const aspect = vp.w / Math.max(1, vp.h)
  const { positions } = computeMosaicLayout(target, aspect)
  schemaStore.snapshot()
  schemaStore.updatePositions(positions)
  setTimeout(() => canvas.fitView(), 50)
}

// Shared mosaic engine — returns (positions, contentW, contentH) without committing anything.
function computeMosaicLayout(
  nodeList: typeof schemaStore.nodes,
  aspect: number,
): { positions: Map<string, { x: number; y: number }>; contentW: number; contentH: number } {
  const bounds = nodeList.map((n) => {
    const el = document.querySelector(`[data-node-id="${n.id}"]`) as HTMLElement | null
    const w = el ? el.offsetWidth : (('w' in n ? n.w : undefined) ?? 280)
    const h = el ? el.offsetHeight : (('h' in n ? n.h : undefined) ?? 120)
    return { node: n, w, h }
  })
  const totalArea = bounds.reduce((s, b) => s + b.w * b.h, 0)
  const targetW = Math.sqrt(totalArea * aspect) * 1.25
  const sorted = [...bounds].sort((a, b) => b.w * b.h - a.w * a.h)
  const GAP = 24
  const positions = new Map<string, { x: number; y: number }>()
  let rowX = 0, rowY = 0, rowMaxH = 0
  for (const b of sorted) {
    if (rowX > 0 && rowX + b.w > targetW) { rowY += rowMaxH + GAP; rowX = 0; rowMaxH = 0 }
    positions.set(b.node.id, { x: rowX, y: rowY })
    rowX += b.w + GAP
    rowMaxH = Math.max(rowMaxH, b.h)
  }
  // Content bounding box of the layout result
  let contentW = 0, contentH = 0
  for (const b of sorted) {
    const p = positions.get(b.node.id)!
    contentW = Math.max(contentW, p.x + b.w)
    contentH = Math.max(contentH, p.y + b.h)
  }
  return { positions, contentW, contentH }
}

function wrapInSection() {
  const canvas = canvasRef.value
  if (!canvas) return
  const target = schemaStore.nodes.filter(
    (n) => selectedIds.value.has(n.id) && n.kind !== 'section',
  )
  if (target.length < 2) return

  const vp = canvas.getViewportRect()
  const aspect = vp.w / Math.max(1, vp.h)
  const { positions: relPos, contentW, contentH } = computeMosaicLayout(target, aspect)

  const PAD = 20
  const TOP_PAD = 52
  const center = canvas.getCenter()
  const sectionW = contentW + PAD * 2
  const sectionH = contentH + TOP_PAD + PAD
  const sectionX = center.x - sectionW / 2
  const sectionY = center.y - sectionH / 2

  const positions = new Map<string, { x: number; y: number }>()
  for (const [id, pos] of relPos) {
    positions.set(id, { x: sectionX + PAD + pos.x, y: sectionY + TOP_PAD + pos.y })
  }

  schemaStore.snapshot()
  schemaStore.updatePositions(positions)
  const sectionId = `section_${Date.now()}`
  schemaStore.addSection({ id: sectionId, name: 'Group', x: sectionX, y: sectionY, w: sectionW, h: sectionH })
  selectedIds.value = new Set()
  setTimeout(() => canvas.fitView(), 50)
}

function fitSectionContents(sectionId: string) {
  const section = schemaStore.nodes.find((n) => n.id === sectionId)
  if (!section || section.kind !== 'section') return

  const inside = schemaStore.nodes.filter((n) => {
    if (n.kind === 'section' || n.id === sectionId) return false
    const el = document.querySelector(`[data-node-id="${n.id}"]`) as HTMLElement | null
    const nw = el ? el.offsetWidth : (('w' in n ? n.w : undefined) ?? 280)
    const nh = el ? el.offsetHeight : (('h' in n ? n.h : undefined) ?? 120)
    const cx = n.x + nw / 2
    const cy = n.y + nh / 2
    return cx >= section.x && cx <= section.x + section.w && cy >= section.y && cy <= section.y + section.h
  })
  if (!inside.length) return

  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity
  for (const n of inside) {
    const el = document.querySelector(`[data-node-id="${n.id}"]`) as HTMLElement | null
    const nw = el ? el.offsetWidth : (('w' in n ? n.w : undefined) ?? 280)
    const nh = el ? el.offsetHeight : (('h' in n ? n.h : undefined) ?? 120)
    minX = Math.min(minX, n.x); minY = Math.min(minY, n.y)
    maxX = Math.max(maxX, n.x + nw); maxY = Math.max(maxY, n.y + nh)
  }

  const PAD = 20, TOP_PAD = 52
  schemaStore.snapshot()
  schemaStore.updatePositions(new Map([[sectionId, { x: minX - PAD, y: minY - TOP_PAD }]]))
  schemaStore.updateNodeSize(sectionId, maxX - minX + PAD * 2, maxY - minY + TOP_PAD + PAD)
}

function mosaicSectionContents(sectionId: string) {
  const canvas = canvasRef.value
  if (!canvas) return
  const section = schemaStore.nodes.find((n) => n.id === sectionId)
  if (!section || section.kind !== 'section') return

  // Find nodes whose center falls inside the section bounds
  const inside = schemaStore.nodes.filter((n) => {
    if (n.kind === 'section' || n.id === sectionId) return false
    const el = document.querySelector(`[data-node-id="${n.id}"]`) as HTMLElement | null
    const nw = el ? el.offsetWidth : (('w' in n ? n.w : undefined) ?? 280)
    const nh = el ? el.offsetHeight : (('h' in n ? n.h : undefined) ?? 120)
    const cx = n.x + nw / 2
    const cy = n.y + nh / 2
    return cx >= section.x && cx <= section.x + section.w && cy >= section.y && cy <= section.y + section.h
  })
  if (!inside.length) return

  const vp = canvas.getViewportRect()
  const aspect = vp.w / Math.max(1, vp.h)
  const { positions: relPos, contentW, contentH } = computeMosaicLayout(inside, aspect)

  const PAD = 20         // horizontal + bottom padding
  const TOP_PAD = 52    // clears label bar (8px offset + 22px height + 22px breathing room)
  const positions = new Map<string, { x: number; y: number }>()
  for (const [id, pos] of relPos) {
    positions.set(id, { x: section.x + PAD + pos.x, y: section.y + TOP_PAD + pos.y })
  }

  // Resize section to fit
  const newW = Math.max(section.w, contentW + PAD * 2)
  const newH = Math.max(section.h, contentH + TOP_PAD + PAD)

  schemaStore.snapshot()
  schemaStore.updatePositions(positions)
  schemaStore.updateNodeSize(sectionId, newW, newH)
}

function alignNodes(op: AlignOp) {
  const bounds = getAlignBounds()
  if (bounds.length < 2) return
  schemaStore.snapshot()
  const positions = new Map<string, { x: number; y: number }>()

  if (op === 'left') {
    const anchor = Math.min(...bounds.map((b) => b.x))
    bounds.forEach((b) => positions.set(b.id, { x: anchor, y: b.y }))
  } else if (op === 'centerH') {
    const anchor = (Math.min(...bounds.map((b) => b.x)) + Math.max(...bounds.map((b) => b.x + b.w))) / 2
    bounds.forEach((b) => positions.set(b.id, { x: anchor - b.w / 2, y: b.y }))
  } else if (op === 'right') {
    const anchor = Math.max(...bounds.map((b) => b.x + b.w))
    bounds.forEach((b) => positions.set(b.id, { x: anchor - b.w, y: b.y }))
  } else if (op === 'top') {
    const anchor = Math.min(...bounds.map((b) => b.y))
    bounds.forEach((b) => positions.set(b.id, { x: b.x, y: anchor }))
  } else if (op === 'middleV') {
    const anchor = (Math.min(...bounds.map((b) => b.y)) + Math.max(...bounds.map((b) => b.y + b.h))) / 2
    bounds.forEach((b) => positions.set(b.id, { x: b.x, y: anchor - b.h / 2 }))
  } else if (op === 'bottom') {
    const anchor = Math.max(...bounds.map((b) => b.y + b.h))
    bounds.forEach((b) => positions.set(b.id, { x: b.x, y: anchor - b.h }))
  } else if (op === 'distH') {
    const sorted = [...bounds].sort((a, b) => a.x - b.x)
    const totalW = sorted.reduce((s, b) => s + b.w, 0)
    const span = sorted.at(-1)!.x + sorted.at(-1)!.w - sorted[0].x
    const gap = (span - totalW) / (sorted.length - 1)
    let cur = sorted[0].x
    sorted.forEach((b) => { positions.set(b.id, { x: cur, y: b.y }); cur += b.w + gap })
  } else if (op === 'distV') {
    const sorted = [...bounds].sort((a, b) => a.y - b.y)
    const totalH = sorted.reduce((s, b) => s + b.h, 0)
    const span = sorted.at(-1)!.y + sorted.at(-1)!.h - sorted[0].y
    const gap = (span - totalH) / (sorted.length - 1)
    let cur = sorted[0].y
    sorted.forEach((b) => { positions.set(b.id, { x: b.x, y: cur }); cur += b.h + gap })
  }

  schemaStore.updatePositions(positions)
}

function onGlobalKey(e: KeyboardEvent) {
  // Cmd+K opens the search palette from anywhere — check before the input guard
  if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
    e.preventDefault()
    closeContextMenu()
    showPalette.value = !showPalette.value
    return
  }

  // ⌘1–⌘9: switch tabs — works even from code editors
  if ((e.metaKey || e.ctrlKey) && e.key >= '1' && e.key <= '9') {
    const idx = parseInt(e.key) - 1
    const tabTarget = schemaStore.canvases[idx]
    if (tabTarget && tabTarget.id !== schemaStore.activeCanvasId) {
      e.preventDefault()
      onTabSwitch(tabTarget.id)
    }
    return
  }

  // Never intercept when focus is in a text field or code editor
  const target = e.target as HTMLElement
  const tag = target.tagName
  const inInput = tag === 'INPUT' || tag === 'TEXTAREA' || target.isContentEditable
    || target.closest('.cm-editor') !== null
  if (inInput) return

  if (e.metaKey || e.ctrlKey) {
    if (e.key === 'z' && !e.shiftKey) { e.preventDefault(); schemaStore.undo(); return }
    if ((e.key === 'z' && e.shiftKey) || e.key === 'y') { e.preventDefault(); schemaStore.redo(); return }
    if (e.key === 'c' && selectedIds.value.size > 0 && !window.getSelection()?.toString()) { e.preventDefault(); copySelectedToClipboard(); return }
    if (e.key === 'v') { e.preventDefault(); pasteFromClipboard(); return }
    if (e.key === ',') { e.preventDefault(); showSettings.value = !showSettings.value }
    if (e.key === 't' && e.shiftKey) { e.preventDefault(); showTests.value = !showTests.value; return }
    if (e.key === 't') { e.preventDefault(); onTabAdd(); return }
    if (e.key === '=' || e.key === '+') { e.preventDefault(); canvasRef.value?.zoomIn() }
    if (e.key === '-') { e.preventDefault(); canvasRef.value?.zoomOut() }
    if (e.key === '0') { e.preventDefault(); canvasRef.value?.fitView() }
    if (e.key === 'd') {
      const ids = [...selectedIds.value]
      if (ids.length) {
        e.preventDefault()
        const newIds = ids.map((id) => schemaStore.duplicateNode(id)).filter(Boolean) as string[]
        if (newIds.length) {
          selectedIds.value = new Set(newIds)
          if (newIds.length === 1) canvasRef.value?.focusNode(newIds[0])
        }
      }
    }
    return
  }
  if (e.altKey) return

  switch (e.key.toLowerCase()) {
    case 'q': if (isReady.value) { e.preventDefault(); addQueryNode() } break
    case 'c': if (isReady.value) { e.preventDefault(); addChartNode() } break
    case 'g': if (isReady.value) { e.preventDefault(); addIngestNode() } break
    case 'e': if (isReady.value) { e.preventDefault(); addExerciseNode() } break
    case 't': if (isReady.value) { e.preventDefault(); addTestNode() } break
    case 'n': e.preventDefault(); addMarkdownNode(); break
    case 's': e.preventDefault(); addSection(); break
    case 'i': if (isReady.value) { e.preventDefault(); showImport.value = true } break
    case 'f': e.preventDefault(); canvasRef.value?.fitView(); break
    case 'l': e.preventDefault(); showSidebar.value = !showSidebar.value; break
    case '/': e.preventDefault(); showQuery.value = !showQuery.value; break
    case '?': e.preventDefault(); showHelp.value = !showHelp.value; break
  }
}

// Navigate to a specific exercise node when the student clicks "Next Lesson" in AssertionCard
watch(navigateRequest, (req) => {
  if (!req) return
  nextTick(() => canvasRef.value?.focusNode(req.id))
})

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
    EventsOn('menu:toggle-layers',     () => { showSidebar.value = !showSidebar.value })
    EventsOn('menu:toggle-query',      () => { showQuery.value = !showQuery.value })
    EventsOn('menu:toggle-exercises', () => { showAssertions.value = !showAssertions.value })
    EventsOn('menu:toggle-tests',     () => { showTests.value = !showTests.value })
    EventsOn('menu:shortcuts',      () => { showHelp.value = !showHelp.value })
    EventsOn('deep-link:import',    (packURL: string) => { pendingPackUrl.value = packURL })
  }
  // Web sandbox: check for ?import=<url> query param (deep-link equivalent for browser)
  if (!IS_DESKTOP) {
    const importParam = new URLSearchParams(window.location.search).get('import')
    if (importParam) pendingPackUrl.value = importParam
  }
  window.addEventListener('keydown', onGlobalKey)
  // Wails macOS: native clipboard shortcuts don't reach the WKWebView without
  // a properly wired Edit menu. Handle them manually for <input>/<textarea>.
  if (IS_DESKTOP) {
    window.addEventListener('keydown', onDesktopClipboardKey, true)
  }
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
  if (IS_DESKTOP) window.removeEventListener('keydown', onDesktopClipboardKey, true)
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
            <path d="M12.3 5.5A5 5 0 1 1 8 3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            <polyline points="6,1 8,3 6,5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          {{ restoreConfirm ? 'Reset?' : 'Restore' }}
        </button>

        <!-- Alignment tools — shown when 2+ nodes are selected -->
        <template v-if="selectedIds.size >= 2">
          <div class="toolbar-divider" />
          <div class="align-group" title="Align selected nodes">
            <button class="align-btn" title="Align left edges"    @click="alignNodes('left')">
              <svg viewBox="0 0 14 14" fill="none">
                <line x1="2" y1="1" x2="2" y2="13" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                <rect x="3.5" y="2.5" width="7" height="3" rx="0.8" fill="currentColor" opacity="0.75"/>
                <rect x="3.5" y="8.5" width="5" height="3" rx="0.8" fill="currentColor" opacity="0.75"/>
              </svg>
            </button>
            <button class="align-btn" title="Align centers horizontally" @click="alignNodes('centerH')">
              <svg viewBox="0 0 14 14" fill="none">
                <line x1="7" y1="1" x2="7" y2="13" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                <rect x="2.5" y="2.5" width="9" height="3" rx="0.8" fill="currentColor" opacity="0.75"/>
                <rect x="3.5" y="8.5" width="7" height="3" rx="0.8" fill="currentColor" opacity="0.75"/>
              </svg>
            </button>
            <button class="align-btn" title="Align right edges"   @click="alignNodes('right')">
              <svg viewBox="0 0 14 14" fill="none">
                <line x1="12" y1="1" x2="12" y2="13" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                <rect x="3.5" y="2.5" width="7" height="3" rx="0.8" fill="currentColor" opacity="0.75"/>
                <rect x="5.5" y="8.5" width="5" height="3" rx="0.8" fill="currentColor" opacity="0.75"/>
              </svg>
            </button>
            <div class="align-sep" />
            <button class="align-btn" title="Align top edges"     @click="alignNodes('top')">
              <svg viewBox="0 0 14 14" fill="none">
                <line x1="1" y1="2" x2="13" y2="2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                <rect x="1.5" y="3.5" width="4" height="8" rx="0.8" fill="currentColor" opacity="0.75"/>
                <rect x="8.5" y="3.5" width="4" height="6" rx="0.8" fill="currentColor" opacity="0.75"/>
              </svg>
            </button>
            <button class="align-btn" title="Align middles vertically" @click="alignNodes('middleV')">
              <svg viewBox="0 0 14 14" fill="none">
                <line x1="1" y1="7" x2="13" y2="7" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                <rect x="1.5" y="2" width="4" height="10" rx="0.8" fill="currentColor" opacity="0.75"/>
                <rect x="8.5" y="3.5" width="4" height="7" rx="0.8" fill="currentColor" opacity="0.75"/>
              </svg>
            </button>
            <button class="align-btn" title="Align bottom edges"  @click="alignNodes('bottom')">
              <svg viewBox="0 0 14 14" fill="none">
                <line x1="1" y1="12" x2="13" y2="12" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                <rect x="1.5" y="2.5" width="4" height="8" rx="0.8" fill="currentColor" opacity="0.75"/>
                <rect x="8.5" y="4.5" width="4" height="6" rx="0.8" fill="currentColor" opacity="0.75"/>
              </svg>
            </button>
            <div class="align-sep" />
            <button class="align-btn" title="Distribute horizontally" @click="alignNodes('distH')">
              <svg viewBox="0 0 14 14" fill="none">
                <line x1="1" y1="2" x2="1" y2="12" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                <line x1="13" y1="2" x2="13" y2="12" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                <rect x="3" y="4" width="3" height="6" rx="0.8" fill="currentColor" opacity="0.75"/>
                <rect x="8" y="4" width="3" height="6" rx="0.8" fill="currentColor" opacity="0.75"/>
              </svg>
            </button>
            <button class="align-btn" title="Distribute vertically" @click="alignNodes('distV')">
              <svg viewBox="0 0 14 14" fill="none">
                <line x1="2" y1="1" x2="12" y2="1" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                <line x1="2" y1="13" x2="12" y2="13" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                <rect x="4" y="3" width="6" height="3" rx="0.8" fill="currentColor" opacity="0.75"/>
                <rect x="4" y="8" width="6" height="3" rx="0.8" fill="currentColor" opacity="0.75"/>
              </svg>
            </button>
          </div>
        </template>

        <div class="toolbar-divider" />

        <button
          class="toolbar-btn"
          :title="selectedIds.size > 0 ? 'Auto-arrange selected nodes into a mosaic' : 'Auto-arrange all nodes into a mosaic'"
          :disabled="schemaStore.nodes.length < 2"
          @click="autoMosaic()"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <rect x="1" y="1" width="8" height="6" rx="1" stroke="currentColor" stroke-width="1.3"/>
            <rect x="11" y="1" width="4" height="6" rx="1" stroke="currentColor" stroke-width="1.3"/>
            <rect x="1" y="9" width="4" height="6" rx="1" stroke="currentColor" stroke-width="1.3"/>
            <rect x="7" y="9" width="8" height="6" rx="1" stroke="currentColor" stroke-width="1.3"/>
          </svg>
          Mosaic
        </button>

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
            <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" stroke="currentColor" stroke-width="1.3"/>
            <line x1="6.5" y1="2.5" x2="6.5" y2="13.5" stroke="currentColor" stroke-width="1.3"/>
            <line x1="9" y1="5.5" x2="12.5" y2="5.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            <line x1="9" y1="8" x2="12.5" y2="8" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            <line x1="9" y1="10.5" x2="12.5" y2="10.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          </svg>
          Query
        </button>
      </div>
    </header>

    <!-- Canvas tabs -->
    <CanvasTabs
      :canvases="schemaStore.canvases"
      :active-id="schemaStore.activeCanvasId"
      @switch="onTabSwitch"
      @add="onTabAdd"
      @remove="onTabRemove"
      @rename="schemaStore.renameCanvas"
      @export="exportCanvasTab"
    />

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

        <button
          class="rail-btn"
          :class="{ active: showAssertions }"
          title="Exercise nodes"
          @click="showAssertions = !showAssertions"
        >
          <!-- Checkmark list icon -->
          <svg viewBox="0 0 16 16" fill="none">
            <path d="M2 4l1.5 1.5L6 3" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M8 4.5h6" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            <path d="M2 8.5l1.5 1.5L6 7.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M8 9h6" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            <rect x="2" y="11.5" width="3" height="3" rx="0.5" stroke="currentColor" stroke-width="1.2"/>
            <path d="M8 13h6" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          </svg>
        </button>

        <button
          class="rail-btn"
          :class="{ active: showTests }"
          title="Test nodes (⌘⇧T)"
          @click="showTests = !showTests"
        >
          <!-- Shield icon -->
          <svg viewBox="0 0 16 16" fill="none">
            <path d="M8 2L3 4.5v4C3 11.5 5.5 14 8 14.5 10.5 14 13 11.5 13 8.5v-4L8 2z" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>
            <path d="M5.5 8.5l2 2 3.5-3.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
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

      <Sidebar v-if="showSidebar" @focus-node="onFocusNode" @create-query="onCreateQueryFromConnection" />
      <ExercisesPanel v-if="showAssertions" @focus-node="onFocusNode" />
      <TestsPanel v-if="showTests" @focus-node="onFocusNode" />
      <Canvas ref="canvasRef" @mosaic-contents="mosaicSectionContents" @fit-contents="fitSectionContents" />
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

        <button class="fab-btn" :disabled="!isReady" title="Ingest — add generator/ingestion node (G)" @click="addIngestNode">
          <svg viewBox="0 0 16 16" fill="none">
            <path d="M2 3h12l-4.5 5.5v4l-3-1.5V8.5L2 3z" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>
            <circle cx="13" cy="13" r="2" fill="currentColor" fill-opacity="0.2" stroke="currentColor" stroke-width="1.2"/>
            <path d="M12 13h2M13 12v2" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
          </svg>
        </button>

        <div class="fab-divider" />

        <button class="fab-btn" title="Note — add markdown node (N)" @click="addMarkdownNode">
          <span class="fab-badge">MD</span>
        </button>
      </div>
    </div>

    <!-- Cmd+K search palette -->
    <SearchPalette
      v-if="showPalette"
      @close="showPalette = false"
      @select="onPaletteSelect"
    />

    <!-- Right-click context menu -->
    <ContextMenu
      v-if="contextMenu"
      :x="contextMenu.x"
      :y="contextMenu.y"
      :sections="contextMenuSections()"
      @close="closeContextMenu"
    />

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
      @load-learn="loadLearnTrack"
    />

    <!-- Chart properties panel (Figma-style right rail) -->
    <ChartPropertiesPanel />

    <!-- Data node schema panel -->
    <DataPropertiesPanel />


    <!-- Import modal -->
    <ImportModal
      v-if="showImport"
      ref="importModalRef"
      :initial-paths="initialPathsForModal"
      :web-file-input="webFileInputRef"
@close="showImport = false; initialPathsForModal = []"
      @created="onImportCreated"
    />

    <!-- Hidden file input for JSON node import -->
    <input
      ref="jsonImportInputRef"
      type="file"
      accept=".sql.garden.json,.json"
      style="display:none"
      @change="onNodeImportFile"
    />

    <!-- sqlgarden:// deep-link import confirmation -->
    <Teleport to="body">
      <div v-if="pendingPackUrl" class="pack-import-backdrop" @mousedown.self="pendingPackUrl = null">
        <div class="pack-import-dialog">
          <h3 class="pack-import-title">Import pack?</h3>
          <p class="pack-import-body">
            The following URL wants to add nodes and SQL to your canvas.
            Only import packs from sources you trust.
          </p>
          <code class="pack-import-url">{{ pendingPackUrl }}</code>
          <div class="pack-import-actions">
            <button class="pack-btn-cancel" :disabled="packImporting" @click="pendingPackUrl = null">Cancel</button>
            <button class="pack-btn-import" :disabled="packImporting" @click="confirmPackImport">
              {{ packImporting ? 'Importing…' : 'Import' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Dataset load error toast -->
    <Transition name="toast">
      <div v-if="loadError" class="load-error-toast">
        <span>⚠ {{ loadError }}</span>
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

/* ── Alignment tools ── */
.align-group {
  display: flex;
  align-items: center;
  gap: 1px;
}

.align-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border-radius: 4px;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  flex-shrink: 0;
  transition: color 0.12s, background 0.12s, border-color 0.12s;
  -webkit-app-region: no-drag;
}

.align-btn svg { width: 14px; height: 14px; }
.align-btn:hover { color: var(--text-primary); background: var(--surface-2); border-color: var(--border); }

.align-sep {
  width: 1px;
  height: 14px;
  background: var(--border);
  margin: 0 2px;
  flex-shrink: 0;
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
  border-radius: 7px;
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
  bottom: 52px;
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
  border-radius: 8px;
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

/* ── Deep-link import confirm ───────────────────────────────────────────────── */
.pack-import-backdrop {
  position: fixed;
  inset: 0;
  z-index: 9100;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
}
.pack-import-dialog {
  background: var(--surface-1);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 24px;
  width: 480px;
  max-width: calc(100vw - 40px);
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.5);
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.pack-import-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}
.pack-import-body {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.5;
}
.pack-import-url {
  display: block;
  font-size: 11px;
  font-family: var(--font-mono);
  color: var(--accent);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 5px;
  padding: 8px 10px;
  word-break: break-all;
}
.pack-import-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}
.pack-btn-cancel {
  padding: 6px 16px;
  font-size: 13px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-primary);
  cursor: pointer;
}
.pack-btn-cancel:hover:not(:disabled) { border-color: var(--text-muted); }
.pack-btn-import {
  padding: 6px 16px;
  font-size: 13px;
  font-weight: 600;
  background: var(--accent);
  border: none;
  border-radius: 6px;
  color: white;
  cursor: pointer;
  transition: opacity 0.1s;
}
.pack-btn-import:hover:not(:disabled) { opacity: 0.85; }
.pack-btn-cancel:disabled,
.pack-btn-import:disabled { opacity: 0.5; cursor: not-allowed; }

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
