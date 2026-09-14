import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backendUrl = process.env.BACKEND_URL || 'http://localhost:8000'
const backendWs = process.env.BACKEND_WS || 'ws://localhost:8000'
const musicUrl = process.env.MUSIC_URL || process.env.MEDIA_URL || 'http://localhost:8100'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      // Order matters: most specific key first. Mashup audio/covers and room
      // music both live under /v1/... on music-service, not /api/....
      '/api/mashups/music': {
        target: musicUrl,
        rewrite: (path) => path.replace(/^\/api\/mashups\/music/, '/v1/mashups'),
      },
      '/api/music': {
        target: musicUrl,
        rewrite: (path) => path.replace(/^\/api\/music/, '/v1/music'),
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
