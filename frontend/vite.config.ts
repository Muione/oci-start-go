import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Dev: Vite serves the SPA on :5173 and proxies API requests to the Go
// backend on :9856. Build: emits to ../internal/web/dist for go:embed.
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:9856', changeOrigin: true },
      '/ws': { target: 'ws://127.0.0.1:9856', ws: true },
      '/instances': { target: 'http://127.0.0.1:9856', changeOrigin: true },
      '/tenants': { target: 'http://127.0.0.1:9856', changeOrigin: true },
      '/backup': { target: 'http://127.0.0.1:9856', changeOrigin: true },
      '/oci': { target: 'http://127.0.0.1:9856', changeOrigin: true },
      '/proxies': { target: 'http://127.0.0.1:9856', changeOrigin: true },
      '/boot': { target: 'http://127.0.0.1:9856', changeOrigin: true },
      '/ssh-keys': { target: 'http://127.0.0.1:9856', changeOrigin: true },
      '/system': { target: 'http://127.0.0.1:9856', changeOrigin: true },
      '/healthz': { target: 'http://127.0.0.1:9856', changeOrigin: true },
    },
  },
  build: { outDir: '../internal/web/dist', emptyOutDir: true, target: 'esnext' },
})
