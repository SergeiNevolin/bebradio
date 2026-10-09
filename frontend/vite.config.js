import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const backendUrl = process.env.BACKEND_URL || 'http://localhost:8000'
const backendWs = process.env.BACKEND_WS || 'ws://localhost:8000'
const musicUrl = process.env.MUSIC_URL || process.env.MEDIA_URL || 'http://localhost:8100'
// караоке теперь нативно внутри SPA: /karaoke — маршрут bebradio, а все
// запросы к karaoke-service идут через единый префикс /api/karaoke/*.
const karaokeUrl = process.env.KARAOKE_URL || 'http://localhost:8000'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 3000,
    proxy: {
      // Room music lives under /v1/... on music-service, not /api/....
      // Upload audio/covers are served by the backend (/api/tracks/:id/...).
      // karaoke-proxy должен идти раньше общего '/api'.
      '/api/karaoke': {
        target: karaokeUrl,
        rewrite: (path) => path.replace(/^\/api\/karaoke/, ''),
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
