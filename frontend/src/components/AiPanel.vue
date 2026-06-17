<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useAiStore, ANTHROPIC_MODELS, OPENAI_MODELS } from '../stores/ai'
import { AISystemPrompt } from '../lib/aiPrompt'
import type { main } from '../../wailsjs/go/models'

const MCP_PORT = 37421
const mcpSSEUrl = `http://127.0.0.1:${MCP_PORT}/sse`

// Claude Desktop only supports stdio transport for local servers.
// Build the bridge binary first, then point Claude Desktop at it.
const mcpBridgeBuildCmd = `go build -o ~/sql-garden-mcp-bridge ./cmd/mcp-bridge`
const mcpDesktopConfig = JSON.stringify({
  mcpServers: { 'sql-garden': { command: `${String.fromCharCode(126)}/sql-garden-mcp-bridge` } }
}, null, 2)

// Claude Code supports SSE directly — no bridge needed.
const mcpCodeCmd = `claude mcp add --transport sse sql-garden ${mcpSSEUrl}`

const mcpCopied = ref<'build' | 'config' | 'cmd' | null>(null)
function copyMCP(which: 'build' | 'config' | 'cmd') {
  const text = which === 'build' ? mcpBridgeBuildCmd
    : which === 'config' ? mcpDesktopConfig
    : mcpCodeCmd
  navigator.clipboard.writeText(text)
  mcpCopied.value = which
  setTimeout(() => { mcpCopied.value = null }, 1800)
}

const emit = defineEmits<{
  close: []
  canvasAction: [action: main.CanvasAction]
}>()

const store = useAiStore()
const activeTab = ref<'chat' | 'settings'>('chat')
const inputText = ref('')
const messagesEl = ref<HTMLElement | null>(null)
const showSystemPrompt = ref(false)

const models = computed(() =>
  store.settings.provider === 'openai' ? OPENAI_MODELS : ANTHROPIC_MODELS
)

// Keep model in sync when provider changes
watch(() => store.settings.provider, (p) => {
  const list = p === 'openai' ? OPENAI_MODELS : ANTHROPIC_MODELS
  if (!list.includes(store.settings.model)) {
    store.saveSettings({ model: list[0] })
  }
})

async function send() {
  const text = inputText.value.trim()
  if (!text || store.thinking) return
  inputText.value = ''
  const actions = await store.send(text)
  for (const a of actions) emit('canvasAction', a)
  await nextTick()
  scrollToBottom()
}

function scrollToBottom() {
  if (messagesEl.value) {
    messagesEl.value.scrollTop = messagesEl.value.scrollHeight
  }
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    send()
  }
}

onMounted(scrollToBottom)
</script>

<template>
  <aside class="ai-panel">
    <!-- Header -->
    <div class="ai-header">
      <span class="ai-title">AI Assistant</span>
      <div class="ai-header-actions">
        <button
          class="ai-hbtn"
          :class="{ active: activeTab === 'settings' }"
          title="Settings"
          @click="activeTab = activeTab === 'settings' ? 'chat' : 'settings'"
        >
          <svg viewBox="0 0 16 16" fill="none">
            <circle cx="8" cy="8" r="2.5" stroke="currentColor" stroke-width="1.3"/>
            <path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.2 3.2l1.4 1.4M11.4 11.4l1.4 1.4M11.4 3.2l-1.4 1.4M4.6 11.4l-1.4 1.4"
              stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          </svg>
        </button>
        <button class="ai-hbtn" title="Close" @click="emit('close')">
          <svg viewBox="0 0 16 16" fill="none">
            <line x1="3" y1="3" x2="13" y2="13" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            <line x1="13" y1="3" x2="3" y2="13" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          </svg>
        </button>
      </div>
    </div>

    <!-- ── Settings tab ──────────────────────────────────────────────────── -->
    <div v-if="activeTab === 'settings'" class="ai-settings">

      <div class="setting-group">
        <label class="setting-label">Enable AI assistant</label>
        <label class="toggle-row">
          <input type="checkbox" :checked="store.settings.enabled"
            @change="store.saveSettings({ enabled: ($event.target as HTMLInputElement).checked })" />
          <span>{{ store.settings.enabled ? 'On' : 'Off' }}</span>
        </label>
      </div>

      <div class="setting-group">
        <label class="setting-label">Provider</label>
        <select class="setting-input setting-select"
          :value="store.settings.provider"
          @change="store.saveSettings({ provider: ($event.target as HTMLSelectElement).value })">
          <option value="anthropic">Anthropic (Claude)</option>
          <option value="openai">OpenAI</option>
        </select>
      </div>

      <div class="setting-group">
        <label class="setting-label">Model</label>
        <select class="setting-input setting-select"
          :value="store.settings.model"
          @change="store.saveSettings({ model: ($event.target as HTMLSelectElement).value })">
          <option v-for="m in models" :key="m" :value="m">{{ m }}</option>
        </select>
      </div>

      <div class="setting-group">
        <label class="setting-label">API key</label>
        <input
          type="password"
          class="setting-input"
          placeholder="sk-ant-... or sk-..."
          :value="store.settings.apiKey"
          autocomplete="off"
          @change="store.saveSettings({ apiKey: ($event.target as HTMLInputElement).value })"
        />
      </div>

      <div class="setting-group">
        <label class="setting-label">Your analytics context <span class="setting-hint">(optional)</span></label>
        <textarea
          class="setting-input setting-textarea"
          rows="4"
          placeholder="e.g. This is e-commerce data. Revenue is in USD. The 'orders' table is the source of truth."
          :value="store.settings.userPrompt"
          @change="store.saveSettings({ userPrompt: ($event.target as HTMLTextAreaElement).value })"
        />
      </div>

      <div class="setting-group">
        <button class="sysprompt-toggle" @click="showSystemPrompt = !showSystemPrompt">
          <svg viewBox="0 0 12 12" fill="none" class="sysprompt-caret">
            <polyline :points="showSystemPrompt ? '2,4 6,8 10,4' : '4,2 8,6 4,10'"
              stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          System prompt (read-only)
        </button>
        <pre v-if="showSystemPrompt" class="sysprompt-preview">{{ AISystemPrompt }}</pre>
      </div>

      <div class="mcp-section">
        <div class="mcp-header">
          <span class="mcp-dot" />
          <span class="mcp-title">MCP — Claude Desktop / Claude Code</span>
        </div>
        <p class="mcp-desc">
          Skip the API key. Connect Claude Desktop or Claude Code directly to sql.garden
          and use your existing subscription.
        </p>

        <div class="mcp-block">
          <div class="mcp-block-label">Claude Desktop — step 1: build the stdio bridge (run once from repo root)</div>
          <pre class="mcp-code">{{ mcpBridgeBuildCmd }}</pre>
          <button class="mcp-copy-btn" @click="copyMCP('build')">
            {{ mcpCopied === 'build' ? 'Copied!' : 'Copy' }}
          </button>
        </div>

        <div class="mcp-block">
          <div class="mcp-block-label">Claude Desktop — step 2: add to <code>claude_desktop_config.json</code>, then restart</div>
          <pre class="mcp-code">{{ mcpDesktopConfig }}</pre>
          <button class="mcp-copy-btn" @click="copyMCP('config')">
            {{ mcpCopied === 'config' ? 'Copied!' : 'Copy' }}
          </button>
        </div>

        <div class="mcp-block">
          <div class="mcp-block-label">Claude Code — run once in terminal (no bridge needed)</div>
          <pre class="mcp-code">{{ mcpCodeCmd }}</pre>
          <button class="mcp-copy-btn" @click="copyMCP('cmd')">
            {{ mcpCopied === 'cmd' ? 'Copied!' : 'Copy' }}
          </button>
        </div>

        <p class="mcp-note">sql.garden must be open when Claude uses it. Canvas actions appear live.</p>
      </div>
    </div>

    <!-- ── Chat tab ──────────────────────────────────────────────────────── -->
    <template v-else>
      <div v-if="!store.settings.enabled || !store.settings.apiKey" class="ai-unconfigured">
        <p>Configure your API key in settings to get started.</p>
        <button class="ai-setup-btn" @click="activeTab = 'settings'">Open settings</button>
      </div>

      <template v-else>
        <!-- Messages -->
        <div ref="messagesEl" class="ai-messages">
          <div v-if="!store.messages.length" class="ai-empty">
            Ask me anything about your data.<br>
            I'll explore it and add findings to your canvas.
          </div>

          <div
            v-for="(msg, i) in store.messages"
            :key="i"
            class="ai-msg"
            :class="msg.role"
          >
            <div v-if="msg.error" class="msg-error">{{ msg.error }}</div>
            <div v-else-if="msg.content" class="msg-content">{{ msg.content }}</div>

            <!-- Canvas action cards -->
            <div
              v-for="(action, j) in msg.canvasActions"
              :key="j"
              class="action-card"
              :class="'action-' + action.type"
            >
              <div class="action-header">
                <span class="action-kind">{{ action.type === 'chart' ? 'Chart' : 'Query' }}</span>
                <span class="action-name">{{ action.name }}</span>
              </div>
              <pre class="action-sql">{{ action.sql }}</pre>
              <button class="action-add-btn" @click="emit('canvasAction', action)">
                + Add to canvas
              </button>
            </div>
          </div>

          <div v-if="store.thinking" class="ai-msg assistant ai-thinking">
            <span class="thinking-dot" /><span class="thinking-dot" /><span class="thinking-dot" />
          </div>
        </div>

        <!-- Input -->
        <div class="ai-input-row">
          <textarea
            v-model="inputText"
            class="ai-input"
            rows="3"
            placeholder="Ask about your data…"
            :disabled="store.thinking"
            @keydown="onKeydown"
          />
          <button class="ai-send" :disabled="store.thinking || !inputText.trim()" @click="send">
            <svg viewBox="0 0 16 16" fill="none">
              <line x1="2" y1="8" x2="13" y2="8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
              <polyline points="9,4 13,8 9,12" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </button>
        </div>

        <div class="ai-footer">
          <button class="ai-clear" @click="store.clear">Clear chat</button>
        </div>
      </template>
    </template>
  </aside>
</template>

<style scoped>
.ai-panel {
  width: 340px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--surface-1);
  border-left: 1px solid var(--border);
  overflow: hidden;
}

/* Header */
.ai-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 10px 7px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.ai-title { font-size: 12px; font-weight: 700; color: var(--text-primary); letter-spacing: 0.02em; }
.ai-header-actions { display: flex; gap: 2px; }
.ai-hbtn {
  display: flex; align-items: center; justify-content: center;
  width: 26px; height: 26px; border-radius: 5px;
  border: none; background: transparent; color: var(--text-muted); cursor: pointer;
  transition: color 0.1s, background 0.1s;
}
.ai-hbtn svg { width: 14px; height: 14px; }
.ai-hbtn:hover, .ai-hbtn.active { color: var(--text-primary); background: var(--surface-2); }

/* Settings */
.ai-settings { flex: 1; overflow-y: auto; padding: 10px; display: flex; flex-direction: column; gap: 12px; }
.setting-group { display: flex; flex-direction: column; gap: 4px; }
.setting-label { font-size: 10.5px; font-weight: 600; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.04em; }
.setting-hint { font-weight: 400; text-transform: none; opacity: 0.7; }
.setting-input {
  background: var(--surface-2); border: 1px solid var(--border); border-radius: 5px;
  color: var(--text-primary); font-size: 12px; padding: 5px 7px; outline: none;
  font-family: var(--font-mono, monospace);
}
.setting-input:focus { border-color: var(--accent); }
.setting-select { font-family: var(--font-sans, sans-serif); cursor: pointer; }
.setting-textarea { resize: vertical; min-height: 72px; font-family: var(--font-sans, sans-serif); font-size: 11.5px; line-height: 1.5; }
.toggle-row { display: flex; align-items: center; gap: 6px; cursor: pointer; font-size: 12px; color: var(--text-secondary); }

.sysprompt-toggle {
  display: flex; align-items: center; gap: 5px;
  font-size: 11px; color: var(--text-muted); background: transparent; border: none; cursor: pointer; padding: 0;
}
.sysprompt-toggle:hover { color: var(--text-primary); }
.sysprompt-caret { width: 10px; height: 10px; }
.sysprompt-preview {
  margin-top: 6px; padding: 8px; border-radius: 5px;
  background: var(--surface-2); border: 1px solid var(--border);
  font-size: 10px; line-height: 1.5; color: var(--text-muted);
  white-space: pre-wrap; word-break: break-word; max-height: 280px; overflow-y: auto;
}

/* ── MCP section ──────────────────────────────────────────────────────────── */
.mcp-section {
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 10px;
  background: var(--surface-0);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.mcp-header {
  display: flex;
  align-items: center;
  gap: 6px;
}

.mcp-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--success, #3fb950);
  flex-shrink: 0;
}

.mcp-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-primary);
}

.mcp-desc {
  font-size: 10.5px;
  color: var(--text-muted);
  line-height: 1.5;
  margin: 0;
}

.mcp-block {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.mcp-block-label {
  font-size: 10px;
  color: var(--text-muted);
  font-weight: 500;
}

.mcp-block-label code {
  font-family: var(--font-mono);
  font-size: 9.5px;
  background: var(--surface-2);
  padding: 1px 3px;
  border-radius: 2px;
}

.mcp-code {
  margin: 0;
  padding: 7px 8px;
  padding-right: 52px;
  border-radius: 4px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  font-size: 9.5px;
  font-family: var(--font-mono);
  line-height: 1.5;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-all;
}

.mcp-copy-btn {
  position: absolute;
  right: 6px;
  bottom: 6px;
  padding: 2px 7px;
  font-size: 9.5px;
  font-weight: 600;
  background: var(--surface-1);
  color: var(--text-muted);
  border: 1px solid var(--border);
  border-radius: 3px;
  cursor: pointer;
  transition: color 0.1s, background 0.1s;
}

.mcp-copy-btn:hover { color: var(--text-primary); background: var(--surface-0); }

.mcp-note {
  font-size: 9.5px;
  color: var(--text-muted);
  font-style: italic;
  line-height: 1.4;
  margin: 0;
}

/* Unconfigured state */
.ai-unconfigured {
  flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 12px; padding: 24px; text-align: center;
  font-size: 12.5px; color: var(--text-muted);
}
.ai-setup-btn {
  padding: 6px 14px; border-radius: 5px; font-size: 12px;
  background: var(--accent); border: none; color: #fff; cursor: pointer;
}
.ai-setup-btn:hover { opacity: 0.88; }

/* Messages */
.ai-messages {
  flex: 1; overflow-y: auto; padding: 10px; display: flex; flex-direction: column; gap: 10px;
}
.ai-empty {
  margin: auto; text-align: center; font-size: 12px; color: var(--text-muted);
  opacity: 0.6; line-height: 1.6;
}

.ai-msg { display: flex; flex-direction: column; gap: 6px; max-width: 100%; }
.ai-msg.user { align-items: flex-end; }
.ai-msg.assistant { align-items: flex-start; }

.msg-content {
  padding: 7px 10px; border-radius: 8px;
  font-size: 12.5px; line-height: 1.55; white-space: pre-wrap; word-break: break-word;
  max-width: 92%;
}
.user .msg-content { background: var(--accent); color: #fff; border-radius: 8px 8px 2px 8px; }
.assistant .msg-content { background: var(--surface-2); color: var(--text-primary); border-radius: 8px 8px 8px 2px; }

.msg-error {
  padding: 6px 10px; border-radius: 6px;
  background: rgba(248,113,113,0.12); border: 1px solid rgba(248,113,113,0.3);
  font-size: 11.5px; color: #f87171; max-width: 92%;
}

/* Canvas action cards */
.action-card {
  width: calc(100% - 2px); border-radius: 7px;
  border: 1px solid var(--border); overflow: hidden;
  background: var(--surface-1);
}
.action-header {
  display: flex; align-items: center; gap: 6px;
  padding: 5px 8px; background: var(--surface-2);
}
.action-kind {
  font-size: 9px; font-weight: 700; letter-spacing: 0.04em;
  padding: 1px 4px; border-radius: 3px;
  background: rgba(99,102,241,0.15); color: #818cf8;
}
.action-chart .action-kind { background: rgba(245,158,11,0.15); color: #fbbf24; }
.action-name { font-size: 11.5px; font-weight: 600; color: var(--text-primary); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.action-sql {
  margin: 0; padding: 6px 8px;
  font-size: 10.5px; font-family: var(--font-mono, monospace);
  color: var(--text-secondary); white-space: pre-wrap; word-break: break-all;
  max-height: 80px; overflow-y: auto; border-top: 1px solid var(--border);
}
.action-add-btn {
  display: block; width: 100%; padding: 5px 8px; border-top: 1px solid var(--border);
  background: transparent; border-left: none; border-right: none; border-bottom: none;
  font-size: 11.5px; color: var(--accent); cursor: pointer; text-align: left;
  transition: background 0.1s;
}
.action-add-btn:hover { background: var(--surface-2); }

/* Thinking indicator */
.ai-thinking { flex-direction: row; gap: 4px; padding: 4px 0; }
.thinking-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--text-muted); opacity: 0.5;
  animation: blink 1.2s ease-in-out infinite;
}
.thinking-dot:nth-child(2) { animation-delay: 0.2s; }
.thinking-dot:nth-child(3) { animation-delay: 0.4s; }
@keyframes blink { 0%, 80%, 100% { opacity: 0.25 } 40% { opacity: 1 } }

/* Input area */
.ai-input-row {
  display: flex; gap: 6px; padding: 8px 10px 4px;
  border-top: 1px solid var(--border); flex-shrink: 0;
}
.ai-input {
  flex: 1; resize: none; border-radius: 7px;
  background: var(--surface-2); border: 1px solid var(--border);
  color: var(--text-primary); font-size: 12.5px; padding: 7px 9px;
  line-height: 1.5; outline: none; font-family: inherit;
}
.ai-input:focus { border-color: var(--accent); }
.ai-input:disabled { opacity: 0.5; }
.ai-send {
  width: 34px; border-radius: 7px; flex-shrink: 0;
  background: var(--accent); border: none; color: #fff; cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  transition: opacity 0.1s;
}
.ai-send svg { width: 16px; height: 16px; }
.ai-send:hover:not(:disabled) { opacity: 0.88; }
.ai-send:disabled { opacity: 0.35; cursor: default; }

.ai-footer {
  display: flex; justify-content: flex-end;
  padding: 3px 10px 8px; flex-shrink: 0;
}
.ai-clear {
  font-size: 10.5px; color: var(--text-muted); background: transparent; border: none; cursor: pointer;
}
.ai-clear:hover { color: var(--text-primary); }
</style>
