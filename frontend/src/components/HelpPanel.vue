<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { IS_DESKTOP } from '../lib/env'

const emit = defineEmits<{ close: [], 'open-mcp-settings': [] }>()

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}
onMounted(() => {
  window.addEventListener('keydown', onKey)
  loadDuckDBVersion()
})
onUnmounted(() => window.removeEventListener('keydown', onKey))

// ── DuckDB version + versioned docs link ──────────────────────
const duckdbVersion = ref<string | null>(null)

// Build a docs URL pinned to the running DuckDB version.
// DuckDB archive format: https://duckdb.org/docs/archive/1.1/
// Falls back to stable if we can't parse the version.
function duckdbDocsUrl(raw: string | null): string {
  if (!raw) return 'https://duckdb.org/docs/stable/'
  const m = raw.replace(/^v/, '').match(/^(\d+)\.(\d+)/)
  if (!m) return 'https://duckdb.org/docs/stable/'
  return `https://duckdb.org/docs/archive/${m[1]}.${m[2]}/`
}

async function loadDuckDBVersion() {
  try {
    if (IS_DESKTOP) {
      const { GetDuckDBVersion } = await import('../../wailsjs/go/main/App')
      duckdbVersion.value = await GetDuckDBVersion()
    } else {
      // Web/Wasm: query DuckDB directly via the schema store's db instance
      // Fall back to stable docs if unavailable
      duckdbVersion.value = null
    }
  } catch {
    duckdbVersion.value = null
  }
}

const SHORTCUT_SECTIONS = [
  {
    title: 'Add to Canvas',
    rows: [
      { keys: ['Q'], label: 'Add Query node' },
      { keys: ['C'], label: 'Add Chart node' },
      { keys: ['N'], label: 'Add Markdown note' },
      { keys: ['S'], label: 'Add Section' },
      { keys: ['G'], label: 'Add Generator / Ingestion node' },
      { keys: ['T'], label: 'Add Test node' },
      { keys: ['E'], label: 'Add Exercise node' },
      { keys: ['I'], label: 'Open Import' },
      { keys: ['⌘', 'D'], label: 'Duplicate selected node(s)' },
    ],
  },
  {
    title: 'Canvas',
    rows: [
      { keys: ['⌘', 'K'], label: 'Search & jump to node' },
      { keys: ['F'], label: 'Fit all nodes in view' },
      { keys: ['⌘', '='], label: 'Zoom in' },
      { keys: ['⌘', '−'], label: 'Zoom out' },
      { keys: ['⌘', '0'], label: 'Fit view (menu)' },
      { keys: ['L'], label: 'Toggle Layers panel' },
      { keys: ['⌘', '⇧', 'E'], label: 'Toggle Exercises panel' },
      { keys: ['⌘', '⇧', 'T'], label: 'Toggle Tests panel' },
      { keys: ['/'], label: 'Toggle Query panel' },
    ],
  },
  {
    title: 'App',
    rows: [
      { keys: ['⌘', ','], label: 'Settings' },
      { keys: ['?'], label: 'This help panel' },
      ...(IS_DESKTOP ? [{ keys: ['⌘', 'Q'], label: 'Quit' }] : []),
    ],
  },
]

const STATIC_LINKS = [
  { label: 'GitHub', href: 'https://github.com/immannino/sql.garden' },
  { label: 'Report Issue', href: 'https://github.com/immannino/sql.garden/issues' },
]

function openLink(href: string) {
  if (IS_DESKTOP) {
    // Open in system browser from Wails desktop app
    import('../../wailsjs/runtime/runtime').then(({ BrowserOpenURL }) => BrowserOpenURL(href))
  } else {
    window.open(href, '_blank', 'noopener')
  }
}
</script>

<template>
  <div class="help-panel" role="dialog" aria-label="Keyboard shortcuts">
    <div class="panel-header">
      <span class="panel-title">Help</span>
      <button class="close-btn" title="Close (Esc)" @click="emit('close')">✕</button>
    </div>

    <div class="panel-body">
      <div v-for="section in SHORTCUT_SECTIONS" :key="section.title" class="shortcut-section">
        <div class="section-title">{{ section.title }}</div>
        <div v-for="row in section.rows" :key="row.label" class="shortcut-row">
          <div class="key-group">
            <kbd v-for="(k, i) in row.keys" :key="i" class="kbd">{{ k }}</kbd>
          </div>
          <span class="shortcut-label">{{ row.label }}</span>
        </div>
      </div>

      <div v-if="IS_DESKTOP" class="divider" />

      <div v-if="IS_DESKTOP" class="shortcut-section mcp-section">
        <div class="section-title">MCP / AI Setup</div>
        <p class="mcp-desc">Connect AI assistants (Claude, Cursor, Windsurf) to sql.garden via a local MCP server.</p>
        <button class="mcp-link-btn" @click="emit('open-mcp-settings'); emit('close')">
          <svg viewBox="0 0 16 16" fill="none">
            <rect x="1" y="4" width="14" height="8" rx="1.5" stroke="currentColor" stroke-width="1.3"/>
            <path d="M4 8h2M7 8h5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          </svg>
          Open MCP Setup →
        </button>
      </div>

      <div class="divider" />

      <div class="links-section">
        <div class="section-title">Links</div>
        <div class="link-list">
          <button
            v-for="link in STATIC_LINKS"
            :key="link.label"
            class="link-btn"
            @click="openLink(link.href)"
          >
            <svg viewBox="0 0 12 12" fill="none">
              <path d="M5 2H2a1 1 0 00-1 1v7a1 1 0 001 1h7a1 1 0 001-1V7" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
              <path d="M8 1h3v3M11 1L6 6" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            {{ link.label }}
          </button>
          <button class="link-btn" @click="openLink(duckdbDocsUrl(duckdbVersion))">
            <svg viewBox="0 0 12 12" fill="none">
              <path d="M5 2H2a1 1 0 00-1 1v7a1 1 0 001 1h7a1 1 0 001-1V7" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
              <path d="M8 1h3v3M11 1L6 6" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            DuckDB Docs
            <span v-if="duckdbVersion" class="version-badge">{{ duckdbVersion }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.help-panel {
  position: absolute;
  bottom: 68px;
  right: 24px;
  z-index: 50;
  width: 300px;
  background: var(--surface-1);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5), 0 1px 0 rgba(255,255,255,0.04) inset;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: panel-in 0.12s ease;
}

@keyframes panel-in {
  from { opacity: 0; transform: translateY(8px) scale(0.97); }
  to   { opacity: 1; transform: translateY(0) scale(1); }
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px 10px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.panel-title {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--text-primary);
}

.close-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 12px;
  padding: 2px 4px;
  border-radius: 4px;
  line-height: 1;
}
.close-btn:hover { color: var(--text-primary); background: var(--surface-2); }

.panel-body {
  overflow-y: auto;
  padding: 10px 0 12px;
  max-height: calc(100vh - 180px);
}

.shortcut-section {
  padding: 0 14px;
  margin-bottom: 12px;
}

.section-title {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
  margin-bottom: 6px;
}

.shortcut-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 3px 0;
  gap: 8px;
}

.key-group {
  display: flex;
  align-items: center;
  gap: 3px;
  flex-shrink: 0;
}

.kbd {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 20px;
  padding: 0 5px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-bottom-width: 2px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 10.5px;
  font-weight: 600;
  color: var(--text-secondary);
  line-height: 1;
}

.shortcut-label {
  font-size: 12px;
  color: var(--text-secondary);
  flex: 1;
  text-align: right;
}

.divider {
  height: 1px;
  background: var(--border);
  margin: 8px 0;
}

.links-section {
  padding: 0 14px;
}

.link-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.link-btn {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 5px 6px;
  background: none;
  border: none;
  border-radius: 6px;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  text-align: left;
  transition: background 0.12s, color 0.12s;
}
.link-btn:hover { background: var(--surface-2); color: var(--text-primary); }
.link-btn svg { width: 12px; height: 12px; flex-shrink: 0; color: var(--text-muted); }
.link-btn:hover svg { color: var(--accent); }

.version-badge {
  margin-left: auto;
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--text-muted);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 1px 5px;
  line-height: 1.4;
}

.mcp-section { margin-bottom: 4px; }

.mcp-desc {
  font-size: 11.5px;
  color: var(--text-muted);
  line-height: 1.55;
  margin: 0 0 8px;
}

.mcp-link-btn {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 6px 10px;
  font-size: 12px;
  font-weight: 500;
  color: var(--accent);
  background: rgba(88, 166, 255, 0.08);
  border: 1px solid rgba(88, 166, 255, 0.25);
  border-radius: 6px;
  transition: background 0.12s;
}
.mcp-link-btn:hover { background: rgba(88, 166, 255, 0.15); }
.mcp-link-btn svg { width: 13px; height: 13px; }
[data-theme="light"] .mcp-link-btn { background: rgba(9, 105, 218, 0.06); border-color: rgba(9, 105, 218, 0.2); }
</style>
