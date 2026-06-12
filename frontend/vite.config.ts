import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Wails branch: PWA and WASM-specific config removed.
// DuckDB queries go through Go bindings; no duckdb-wasm needed.
export default defineConfig({
  plugins: [vue()],
  // Wails injects its runtime at wails://wails/ — no COOP/COEP headers needed
  // in desktop mode (those were only required for SharedArrayBuffer in the browser).
})
