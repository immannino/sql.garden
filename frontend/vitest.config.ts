import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  define: {
    __IS_DESKTOP__: 'true',
  },
  test: {
    environment: 'happy-dom',
    globals: true,
  },
})
