<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useTheme, type Theme } from '../composables/useTheme'
import { IS_DESKTOP } from '../lib/env'
import type { main } from '../../wailsjs/go/models'

const props = defineProps<{ initialTab?: 'appearance' | 'mcp' | 'updates' }>()
const emit = defineEmits<{ close: [] }>()

const { theme, setTheme } = useTheme()

type Tab = 'appearance' | 'mcp' | 'updates'
const activeTab = ref<Tab>(props.initialTab ?? 'appearance')

// ── MCP ───────────────────────────────────────────────────────────────────────
const mcpPort = 37421
const copiedKey = ref<string | null>(null)
const mcpConfigStatus = ref<main.MCPConfigStatus | null>(null)
const mcpWriting = ref(false)
const mcpWriteResult = ref<'ok' | 'err' | null>(null)

const jsonSnippet = `{
  "mcpServers": {
    "sql-garden": {
      "url": "http://127.0.0.1:${mcpPort}/mcp"
    }
  }
}`

const CLIENTS = [
  {
    id: 'claude-code',
    label: 'Claude Code',
    hint: 'Run in your terminal:',
    snippet: `mcp add --transport http sql-garden http://127.0.0.1:${mcpPort}/mcp`,
    mono: true,
  },
  {
    id: 'claude-desktop',
    label: 'Claude Desktop',
    hint: 'Add to claude_desktop_config.json → mcpServers:',
    snippet: jsonSnippet,
    mono: false,
  },
  {
    id: 'cursor',
    label: 'Cursor',
    hint: 'Add to ~/.cursor/mcp.json → mcpServers:',
    snippet: jsonSnippet,
    mono: false,
  },
  {
    id: 'windsurf',
    label: 'Windsurf',
    hint: 'Add to ~/.codeium/windsurf/mcp_config.json → mcpServers:',
    snippet: jsonSnippet,
    mono: false,
  },
]

function copySnippet(id: string, text: string) {
  navigator.clipboard.writeText(text)
  copiedKey.value = id
  setTimeout(() => { if (copiedKey.value === id) copiedKey.value = null }, 2000)
}

async function loadMCPStatus() {
  if (!IS_DESKTOP) return
  const { GetMCPConfigStatus } = await import('../../wailsjs/go/main/App')
  mcpConfigStatus.value = await GetMCPConfigStatus()
}

async function autoConfigureClaude() {
  if (mcpWriting.value) return
  mcpWriting.value = true; mcpWriteResult.value = null
  try {
    const { WriteMCPToClaudeDesktop } = await import('../../wailsjs/go/main/App')
    await WriteMCPToClaudeDesktop()
    mcpWriteResult.value = 'ok'
    await loadMCPStatus()
  } catch {
    mcpWriteResult.value = 'err'
  } finally {
    mcpWriting.value = false
  }
}

// ── Updates ───────────────────────────────────────────────────────────────────
const updateInfo = ref<main.UpdateInfo | null>(null)
const checkingUpdate = ref(false)
const updateError = ref(false)

async function checkForUpdates() {
  if (!IS_DESKTOP || checkingUpdate.value) return
  checkingUpdate.value = true
  updateError.value = false
  try {
    const { CheckForUpdate } = await import('../../wailsjs/go/main/App')
    updateInfo.value = await CheckForUpdate()
  } catch {
    updateError.value = true
  } finally {
    checkingUpdate.value = false
  }
}

watch(activeTab, (tab) => {
  if (tab === 'updates' && !updateInfo.value && !checkingUpdate.value) checkForUpdates()
  if (tab === 'mcp' && !mcpConfigStatus.value) loadMCPStatus()
})

// ── Lifecycle ─────────────────────────────────────────────────────────────────
onMounted(() => {
  window.addEventListener('keydown', onKey)
  if (activeTab.value === 'mcp') loadMCPStatus()
})
onUnmounted(() => window.removeEventListener('keydown', onKey))

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}

function onBackdrop(e: MouseEvent) {
  if ((e.target as HTMLElement).classList.contains('modal-backdrop')) emit('close')
}
</script>

<template>
  <div class="modal-backdrop" @mousedown="onBackdrop">
    <div class="modal" role="dialog" aria-label="Settings">
      <!-- Header -->
      <div class="modal-header">
        <h2 class="modal-title">Settings</h2>
        <button class="close-btn" title="Close" @click="emit('close')">
          <svg viewBox="0 0 16 16" fill="none">
            <line x1="3" y1="3" x2="13" y2="13" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
            <line x1="13" y1="3" x2="3" y2="13" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <div class="modal-body">
        <!-- Sidebar tabs -->
        <nav class="settings-nav">
          <button :class="['nav-item', { active: activeTab === 'appearance' }]" @click="activeTab = 'appearance'">
            <svg viewBox="0 0 16 16" fill="none">
              <circle cx="8" cy="8" r="6.5" stroke="currentColor" stroke-width="1.3"/>
              <path d="M8 1.5v13M1.5 8h13" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" opacity=".4"/>
              <circle cx="8" cy="8" r="2.5" fill="currentColor" opacity=".7"/>
            </svg>
            Appearance
          </button>
          <button v-if="IS_DESKTOP" :class="['nav-item', { active: activeTab === 'mcp' }]" @click="activeTab = 'mcp'">
            <svg viewBox="0 0 16 16" fill="none">
              <rect x="1" y="4" width="14" height="8" rx="1.5" stroke="currentColor" stroke-width="1.3"/>
              <path d="M4 8h2M7 8h5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
              <circle cx="4" cy="2.5" r="1" fill="currentColor" opacity=".5"/>
              <circle cx="8" cy="2.5" r="1" fill="currentColor" opacity=".5"/>
              <circle cx="12" cy="2.5" r="1" fill="currentColor" opacity=".5"/>
            </svg>
            MCP
          </button>
          <button v-if="IS_DESKTOP" :class="['nav-item', { active: activeTab === 'updates' }]" @click="activeTab = 'updates'">
            <span class="nav-icon-wrap">
              <svg viewBox="0 0 16 16" fill="none">
                <circle cx="8" cy="8" r="6.5" stroke="currentColor" stroke-width="1.3"/>
                <line x1="8" y1="5" x2="8" y2="8.5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
                <circle cx="8" cy="11" r=".8" fill="currentColor"/>
              </svg>
              <span v-if="updateInfo?.hasUpdate" class="update-dot" />
            </span>
            Updates
          </button>
        </nav>

        <!-- Panes -->
        <div class="settings-pane">

          <!-- Appearance -->
          <template v-if="activeTab === 'appearance'">
            <h3 class="pane-title">Appearance</h3>

            <div class="field-group">
              <label class="field-label">Theme</label>
              <div class="theme-toggle">
                <button
                  v-for="opt in ([['dark','Dark'],['system','System'],['light','Light']] as [Theme,string][])"
                  :key="opt[0]"
                  :class="['theme-btn', { active: theme === opt[0] }]"
                  @click="setTheme(opt[0])"
                >
                  <svg v-if="opt[0] === 'dark'" viewBox="0 0 16 16" fill="none">
                    <path d="M13.5 10.5A6 6 0 0 1 5.5 2.5a6 6 0 1 0 8 8z" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>
                  </svg>
                  <svg v-else-if="opt[0] === 'system'" viewBox="0 0 16 16" fill="none">
                    <rect x="1" y="2" width="14" height="10" rx="1.5" stroke="currentColor" stroke-width="1.3"/>
                    <path d="M5 14h6M8 12v2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                  </svg>
                  <svg v-else viewBox="0 0 16 16" fill="none">
                    <circle cx="8" cy="8" r="3.5" stroke="currentColor" stroke-width="1.3"/>
                    <path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.1 3.1l1.4 1.4M11.5 11.5l1.4 1.4M3.1 12.9l1.4-1.4M11.5 4.5l1.4-1.4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                  </svg>
                  {{ opt[1] }}
                </button>
              </div>
            </div>
          </template>

          <!-- MCP (desktop only) -->
          <template v-if="IS_DESKTOP && activeTab === 'mcp'">
            <h3 class="pane-title">MCP Server</h3>
            <p class="pane-desc">sql.garden runs a local MCP server so AI assistants can query your data, add nodes, and build dashboards autonomously.</p>

            <div class="field-group">
              <label class="field-label">Status</label>
              <span class="status-badge running">● Running on port {{ mcpPort }}</span>
            </div>

            <!-- Per-client setup -->
            <div v-for="client in CLIENTS" :key="client.id" class="field-group">
              <label class="field-label">{{ client.label }}</label>
              <span class="field-hint" style="margin-top: -2px;">{{ client.hint }}</span>
              <div :class="['code-block', { 'code-block--multi': !client.mono }]">
                <code v-if="client.mono">{{ client.snippet }}</code>
                <pre v-else>{{ client.snippet }}</pre>
                <button class="copy-btn" @click="copySnippet(client.id, client.snippet)">
                  {{ copiedKey === client.id ? '✓' : 'Copy' }}
                </button>
              </div>

              <!-- Auto-configure button for Claude Desktop -->
              <div v-if="client.id === 'claude-desktop'" class="auto-configure-row">
                <button
                  class="auto-btn"
                  :disabled="mcpWriting || mcpConfigStatus?.configured"
                  @click="autoConfigureClaude"
                >
                  <span v-if="mcpWriting" class="spinner" />
                  {{ mcpConfigStatus?.configured ? 'Already configured ✓' : 'Auto-configure Claude Desktop' }}
                </button>
                <span v-if="mcpConfigStatus?.path" class="config-path">{{ mcpConfigStatus.path }}</span>
                <span v-if="mcpWriteResult === 'ok'" class="write-result ok">Config updated successfully</span>
                <span v-if="mcpWriteResult === 'err'" class="write-result err">Failed to write config</span>
              </div>
            </div>
          </template>

          <!-- Updates (desktop only) -->
          <template v-if="IS_DESKTOP && activeTab === 'updates'">
            <h3 class="pane-title">Updates</h3>

            <!-- Loading -->
            <div v-if="checkingUpdate" class="update-checking">
              <span class="spinner" />
              Checking for updates…
            </div>

            <!-- Error -->
            <div v-else-if="updateError" class="update-row">
              <span class="update-badge error">Could not check — no internet?</span>
              <button class="retry-btn" @click="checkForUpdates">Retry</button>
            </div>

            <!-- Result -->
            <template v-else-if="updateInfo">
              <div class="field-group">
                <label class="field-label">Installed</label>
                <span class="version-badge">{{ updateInfo.currentVersion }}</span>
              </div>

              <div class="field-group">
                <label class="field-label">Latest</label>
                <div class="update-row">
                  <span class="version-badge">{{ updateInfo.latestVersion || '—' }}</span>
                  <span v-if="updateInfo.hasUpdate" class="update-badge new">Update available</span>
                  <span v-else class="update-badge ok">Up to date</span>
                </div>
              </div>

              <a v-if="updateInfo.hasUpdate && updateInfo.releaseURL" class="update-cta" :href="updateInfo.releaseURL" target="_blank">
                Download {{ updateInfo.latestVersion }}
                <svg viewBox="0 0 12 12" fill="none">
                  <path d="M2 10L10 2M5 2h5v5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </a>

              <a v-else class="ext-link" href="https://github.com/tonymannino/sql.garden/releases" target="_blank">
                View all releases
                <svg viewBox="0 0 12 12" fill="none">
                  <path d="M2 10L10 2M5 2h5v5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </a>
            </template>

            <p class="pane-desc update-note">
              sql.garden doesn't auto-update yet. Download the latest release from GitHub and replace the existing app.
            </p>
          </template>

        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  width: 580px;
  max-height: 560px;
  background: var(--surface-1);
  border: 1px solid var(--border);
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.45);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.modal-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.close-btn {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  background: transparent;
  border: none;
  border-radius: 4px;
  transition: color 0.15s, background 0.15s;
}
.close-btn:hover { color: var(--text-primary); background: var(--surface-2); }
.close-btn svg { width: 14px; height: 14px; }

.modal-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* Sidebar nav */
.settings-nav {
  width: 140px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 12px 8px;
  border-right: 1px solid var(--border);
  background: var(--surface-0);
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  font-size: 12.5px;
  color: var(--text-secondary);
  background: transparent;
  border: none;
  border-radius: 6px;
  text-align: left;
  transition: color 0.15s, background 0.15s;
}
.nav-item svg { width: 14px; height: 14px; }
.nav-icon-wrap { position: relative; display: flex; flex-shrink: 0; }
.update-dot {
  position: absolute;
  top: -2px;
  right: -3px;
  width: 6px;
  height: 6px;
  background: #f0a800;
  border-radius: 50%;
  border: 1.5px solid var(--surface-0);
}
.nav-item:hover { color: var(--text-primary); background: var(--surface-2); }
.nav-item.active { color: var(--accent); background: rgba(88, 166, 255, 0.1); font-weight: 500; }
[data-theme="light"] .nav-item.active { background: rgba(9, 105, 218, 0.08); }

/* Settings pane */
.settings-pane {
  flex: 1;
  padding: 20px 24px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.pane-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: -4px;
}

.pane-desc {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.6;
  margin-top: -8px;
}

.field-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-label {
  font-size: 11.5px;
  font-weight: 500;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.field-input, .field-select {
  padding: 7px 10px;
  font-size: 12.5px;
  color: var(--text-primary);
  background: var(--surface-0);
  border: 1px solid var(--border);
  border-radius: 6px;
  outline: none;
  transition: border-color 0.15s;
  width: 100%;
}
.field-input:focus, .field-select:focus { border-color: var(--accent); }

.field-hint {
  font-size: 11px;
  color: var(--text-muted);
}
.field-hint code {
  font-family: var(--font-mono);
  background: var(--surface-2);
  padding: 1px 4px;
  border-radius: 3px;
}

/* Theme toggle */
.theme-toggle {
  display: flex;
  gap: 6px;
}

.theme-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  font-size: 12.5px;
  color: var(--text-secondary);
  background: var(--surface-0);
  border: 1px solid var(--border);
  border-radius: 6px;
  transition: color 0.15s, border-color 0.15s, background 0.15s;
  flex: 1;
  justify-content: center;
}
.theme-btn svg { width: 13px; height: 13px; }
.theme-btn:hover { color: var(--text-primary); border-color: var(--text-muted); }
.theme-btn.active {
  color: var(--accent);
  border-color: var(--accent);
  background: rgba(88, 166, 255, 0.08);
  font-weight: 500;
}
[data-theme="light"] .theme-btn.active { background: rgba(9, 105, 218, 0.06); }

/* Code blocks */
.code-block {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--surface-0);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 8px 12px;
}
.code-block code {
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: var(--text-primary);
  flex: 1;
  word-break: break-all;
}
.code-block--multi {
  align-items: flex-start;
}
.code-block--multi pre {
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: var(--text-primary);
  flex: 1;
  white-space: pre;
}

.copy-btn {
  padding: 3px 10px;
  font-size: 11px;
  color: var(--text-secondary);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  flex-shrink: 0;
  transition: color 0.15s, background 0.15s;
}
.copy-btn:hover { color: var(--text-primary); background: var(--surface-3); }

/* Status badges */
.status-badge {
  font-size: 12px;
  font-weight: 500;
}
.status-badge.running { color: var(--success); }

/* Version badge */
.version-badge {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-secondary);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 3px 8px;
  width: fit-content;
}

/* External link */
.ext-link {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12.5px;
  color: var(--accent);
  text-decoration: none;
}
.ext-link:hover { text-decoration: underline; }
.ext-link svg { width: 11px; height: 11px; }

.update-note { margin-top: -8px; }

/* Update check UI */
.update-checking {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12.5px;
  color: var(--text-muted);
}

.spinner {
  display: inline-block;
  width: 12px;
  height: 12px;
  border: 1.5px solid var(--border);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
  flex-shrink: 0;
}

@keyframes spin { to { transform: rotate(360deg); } }

.update-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.update-badge {
  font-size: 11px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 20px;
}
.update-badge.ok {
  color: var(--success);
  background: rgba(63, 185, 80, 0.1);
  border: 1px solid rgba(63, 185, 80, 0.25);
}
.update-badge.new {
  color: #f0a800;
  background: rgba(240, 168, 0, 0.1);
  border: 1px solid rgba(240, 168, 0, 0.25);
}
.update-badge.error {
  color: var(--danger, #f85149);
  background: rgba(248, 81, 73, 0.08);
  border: 1px solid rgba(248, 81, 73, 0.2);
}

.retry-btn {
  font-size: 11.5px;
  color: var(--accent);
  background: transparent;
  border: none;
  padding: 0;
  text-decoration: underline;
}
.retry-btn:hover { opacity: 0.8; }

.update-cta {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  font-size: 12.5px;
  font-weight: 500;
  color: #fff;
  background: var(--accent);
  border-radius: 6px;
  text-decoration: none;
  width: fit-content;
  transition: opacity 0.15s;
}
.update-cta:hover { opacity: 0.88; }
.update-cta svg { width: 11px; height: 11px; }

/* Auto-configure Claude Desktop */
.auto-configure-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.auto-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  font-size: 12px;
  font-weight: 500;
  color: var(--accent);
  background: rgba(88, 166, 255, 0.08);
  border: 1px solid rgba(88, 166, 255, 0.3);
  border-radius: 6px;
  width: fit-content;
  transition: background 0.15s, opacity 0.15s;
}
.auto-btn:hover:not(:disabled) { background: rgba(88, 166, 255, 0.15); }
.auto-btn:disabled { opacity: 0.6; }
[data-theme="light"] .auto-btn { background: rgba(9, 105, 218, 0.06); border-color: rgba(9, 105, 218, 0.25); }

.config-path {
  font-family: var(--font-mono);
  font-size: 10.5px;
  color: var(--text-muted);
  word-break: break-all;
}

.write-result {
  font-size: 11.5px;
  font-weight: 500;
}
.write-result.ok { color: var(--success); }
.write-result.err { color: var(--danger, #f85149); }
</style>
