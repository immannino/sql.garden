import path from 'node:path'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  define: {
    // IS_DESKTOP=true marks all WASM branches as dead code.
    __IS_DESKTOP__: 'true',
  },
  resolve: {
    alias: [
      // Redirect web-only modules to empty stubs so @duckdb/duckdb-wasm
      // (and its ~75MB WASM assets) are never included in the desktop bundle.
      {
        find: /.*\/useDuckDB\.web$/,
        replacement: path.resolve(__dirname, 'src/composables/useDuckDB.web.stub.ts'),
      },
      {
        find: /.*\/webSampleData$/,
        replacement: path.resolve(__dirname, 'src/lib/webSampleData.stub.ts'),
      },
    ],
  },
})
