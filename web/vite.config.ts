import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { '@': path.resolve(__dirname, './src') },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: process.env.API_URL ?? 'http://localhost:8081',
        changeOrigin: true,
      },
      '/v0': {
        target: process.env.API_URL ?? 'http://localhost:8081',
        changeOrigin: true,
      },
      // Only proxy the A2A Agent Card well-known path under /agents/{ns}/{slug}.
      // Everything else under /agents/* is a React Router client-side route
      // (the /agents list page and /agents/{ns}/{slug} detail page) and must
      // be served by the SPA. Using a regex here (key starting with ^) makes
      // Vite's proxy treat it as a RegExp instead of a prefix match.
      '^/agents/[^/]+/[^/]+/\\.well-known/.*': {
        target: process.env.API_URL ?? 'http://localhost:8081',
        changeOrigin: true,
      },
      '/healthz': {
        target: process.env.API_URL ?? 'http://localhost:8081',
        changeOrigin: true,
      },
      '/readyz': {
        target: process.env.API_URL ?? 'http://localhost:8081',
        changeOrigin: true,
      },
      '/metrics': {
        target: process.env.API_URL ?? 'http://localhost:8081',
        changeOrigin: true,
      },
      '/.well-known': {
        target: process.env.API_URL ?? 'http://localhost:8081',
        changeOrigin: true,
      },
      '/docs': {
        target: process.env.API_URL ?? 'http://localhost:8081',
        changeOrigin: true,
      },
      '/openapi.yaml': {
        target: process.env.API_URL ?? 'http://localhost:8081',
        changeOrigin: true,
      },
      '/config.json': {
        target: process.env.API_URL ?? 'http://localhost:8081',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
  },
})
