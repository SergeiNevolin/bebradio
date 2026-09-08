import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backendUrl = process.env.BACKEND_URL || 'http://localhost:8000'
const backendWs = process.env.BACKEND_WS || 'ws://localhost:8000'
const mediaUrl = process.env.MEDIA_URL || 'http://localhost:8100'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      // Order matters: most specific key first. Mashup audio/covers and room
      // media both live under /v1/... on media-service, not /api/....
      '/api/mashups/media': {
        target: mediaUrl,
        rewrite: (path) => path.replace(/^\/api\/mashups\/media/, '/v1/mashups'),
      },
      '/api/media': {
        target: mediaUrl,
        rewrite: (path) => path.replace(/^\/api\/media/, '/v1/media'),
      },
      '/api': backendUrl,
      '/ws': {
        target: backendWs,
        ws: true,
      },
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.js',
    exclude: ['e2e/**', 'node_modules/**'],
    css: {
      modules: {
        classNameStrategy: 'non-scoped',
      },
    },
    teardownTimeout: 5000,
  },
})
