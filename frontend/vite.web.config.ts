import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  define: {
    // IS_DESKTOP=false eliminates all Go/Wails branches and enables the WASM path.
    __IS_DESKTOP__: 'false',
  },
  base: '/sandbox/',
  build: {
    outDir: 'dist-web',
    // duckdb-wasm WASM files are large — don't warn about them
    chunkSizeWarningLimit: 60000,
  },
  server: {
    // COOP/COEP required for SharedArrayBuffer (multi-threaded DuckDB WASM bundle).
    // On GitHub Pages, these headers aren't supported — duckdb-wasm automatically
    // falls back to the single-threaded MVP bundle. To opt into the faster bundle
    // on Pages, add coi-serviceworker (see deploy.yml for instructions).
    headers: {
      'Cross-Origin-Opener-Policy': 'same-origin',
      'Cross-Origin-Embedder-Policy': 'require-corp',
    },
  },
  optimizeDeps: {
    exclude: ['@duckdb/duckdb-wasm'],
  },
})
