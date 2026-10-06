import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { '@': path.resolve(import.meta.dirname, './src') },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
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
    rollupOptions: {
      output: {
        // Split heavy vendor libs into their own chunks so the main entry
        // stays lean and long-lived caches survive app-code changes.
        // Rationale per chunk:
        //   - react:    react + react-dom + react-router (pulled by every route)
        //   - query:    @tanstack/react-query (used app-wide but changes rarely)
        //   - markdown: react-markdown + remark/rehype chain (heavy; only some
        //               pages actually render markdown)
        manualChunks: (id) => {
          if (!id.includes('node_modules')) return undefined
          if (id.includes('react-router')) return 'react'
          if (id.includes('/react-dom/') || id.includes('/react/')) return 'react'
          if (id.includes('@tanstack/react-query')) return 'query'
          if (
            id.includes('react-markdown') ||
            id.includes('/remark') ||
            id.includes('/rehype') ||
            id.includes('/micromark') ||
            id.includes('/mdast') ||
            id.includes('/hast')
          ) {
            return 'markdown'
          }
          return undefined
        },
      },
    },
  },
})
