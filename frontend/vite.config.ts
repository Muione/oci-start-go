import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Dev: Vite serves the SPA on :5173 and proxies /api (and /ws) to the Go
// backend on :9856. Build: emits to ../internal/web/dist for go:embed.
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:9856', changeOrigin: true },
      '/ws': { target: 'ws://127.0.0.1:9856', ws: true },
    },
  },
  build: { outDir: '../internal/web/dist', emptyOutDir: true, target: 'esnext' },
})
