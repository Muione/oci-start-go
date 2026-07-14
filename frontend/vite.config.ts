import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

// Dev: Vite serves the SPA on :5173 and proxies API requests to the Go
// backend on :9856. Build: emits to ../internal/web/dist for go:embed.
//
// Several proxy prefixes (/instances, /tenants, /boot, /proxies) are also
// client-side SPA routes. A browser page navigation to such a path (Accept:
// text/html) must be served by Vite as the SPA — if proxied, the backend's
// SPA fallback returns the production index.html with hashed asset refs the
// dev server can't resolve, so the JS never loads and the page stays blank.
// bypass() serves the SPA for HTML navigations and proxies only API/XHR
// calls (axios sends Accept: application/json, … — no text/html).
const bypass = (req: { headers: { accept?: string } }) =>
  req.headers.accept?.includes('text/html') ? '/index.html' : undefined

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:9856', changeOrigin: true, bypass },
      '/ws': { target: 'ws://127.0.0.1:9856', ws: true, bypass },
      '/instances': { target: 'http://127.0.0.1:9856', changeOrigin: true, bypass },
      '/tenants': { target: 'http://127.0.0.1:9856', changeOrigin: true, bypass },
      '/backup': { target: 'http://127.0.0.1:9856', changeOrigin: true, bypass },
      '/dns': { target: 'http://127.0.0.1:9856', changeOrigin: true, bypass },
      '/oci': { target: 'http://127.0.0.1:9856', changeOrigin: true, bypass },
      '/proxies': { target: 'http://127.0.0.1:9856', changeOrigin: true, bypass },
      '/boot': { target: 'http://127.0.0.1:9856', changeOrigin: true, bypass },
      '/ssh-keys': { target: 'http://127.0.0.1:9856', changeOrigin: true, bypass },
      '/system': { target: 'http://127.0.0.1:9856', changeOrigin: true, bypass },
      '/healthz': { target: 'http://127.0.0.1:9856', changeOrigin: true, bypass },
    },
  },
  build: { outDir: '../internal/web/dist', emptyOutDir: true, target: 'esnext' },
})
