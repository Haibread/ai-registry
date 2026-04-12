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
      // Only proxy actual per-agent paths (/{ns}/{slug}/…), NOT the bare /agents
      // SPA listing route. The trailing slash is critical: Vite's proxy uses
      // prefix matching, so '/agents/' only matches '/agents/foo' whereas
      // '/agents' would also match the bare '/agents' path, sending it to the
      // backend instead of serving the React SPA.
      '/agents/': {
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
