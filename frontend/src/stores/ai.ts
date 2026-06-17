import { defineStore } from 'pinia'
import { ref } from 'vue'
import { GetAISettings, SaveAISettings, SendAIMessage } from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'

export interface ChatMsg {
  role: 'user' | 'assistant'
  content: string
  canvasActions?: main.CanvasAction[]
  error?: string
}

const ANTHROPIC_MODELS = ['claude-sonnet-4-6', 'claude-opus-4-8', 'claude-haiku-4-5-20251001']
const OPENAI_MODELS    = ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo']

export { ANTHROPIC_MODELS, OPENAI_MODELS }

export const useAiStore = defineStore('ai', () => {
  const settings = ref<main.AISettings>({
    enabled: false, provider: 'anthropic', apiKey: '', model: 'claude-sonnet-4-6', userPrompt: '',
  })
  const messages = ref<ChatMsg[]>([])
  const thinking = ref(false)

  async function loadSettings() {
    try {
      const s = await GetAISettings()
      settings.value = s
    } catch { /* persist not ready */ }
  }

  async function saveSettings(patch: Partial<main.AISettings>) {
    const next = { ...settings.value, ...patch }
    await SaveAISettings(next as main.AISettings)
    settings.value = next as main.AISettings
  }

  async function send(content: string): Promise<main.CanvasAction[]> {
    messages.value.push({ role: 'user', content })
    thinking.value = true
    try {
      // Send last 20 messages to keep context window reasonable
      const history = messages.value
        .slice(-20)
        .map((m) => ({ role: m.role, content: m.content })) as main.AIChatMessage[]

      const resp = await SendAIMessage(history)
      const actions = resp.canvasActions ?? []
      messages.value.push({ role: 'assistant', content: resp.content, canvasActions: actions })
      return actions
    } catch (e) {
      const errMsg = e instanceof Error ? e.message : String(e)
      messages.value.push({ role: 'assistant', content: '', error: errMsg })
      return []
    } finally {
      thinking.value = false
    }
  }

  function clear() { messages.value = [] }

  return { settings, messages, thinking, loadSettings, saveSettings, send, clear }
})
