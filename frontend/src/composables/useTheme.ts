import { ref } from 'vue'
import { GetAppSettings, SaveAppSettings } from '../../wailsjs/go/main/App'

export type Theme = 'dark' | 'light' | 'system'

const theme = ref<Theme>('system')
let mediaQuery: MediaQueryList | null = null

function applyTheme(t: Theme) {
  const el = document.documentElement
  if (t === 'system') {
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    el.setAttribute('data-theme', prefersDark ? 'dark' : 'light')
  } else {
    el.setAttribute('data-theme', t)
  }
}

function onSystemChange(e: MediaQueryListEvent) {
  if (theme.value === 'system') {
    document.documentElement.setAttribute('data-theme', e.matches ? 'dark' : 'light')
  }
}

export function useTheme() {
  async function loadTheme() {
    try {
      const s = await GetAppSettings()
      theme.value = (s.theme as Theme) || 'system'
    } catch {
      theme.value = 'system'
    }
    applyTheme(theme.value)

    mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    mediaQuery.addEventListener('change', onSystemChange)
  }

  async function setTheme(t: Theme) {
    theme.value = t
    applyTheme(t)
    try {
      await SaveAppSettings({ theme: t })
    } catch { /* non-fatal */ }
  }

  return { theme, loadTheme, setTheme }
}
