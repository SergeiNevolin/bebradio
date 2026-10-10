import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { visualizer } from 'rollup-plugin-visualizer'

const backendUrl = process.env.BACKEND_URL || 'http://localhost:8000'
const backendWs = process.env.BACKEND_WS || 'ws://localhost:8000'
const musicUrl = process.env.MUSIC_URL || process.env.MEDIA_URL || 'http://localhost:8100'
// караоке теперь нативно внутри SPA: /karaoke — маршрут bebradio, а все
// запросы к karaoke-service идут через единый префикс /api/karaoke/*.
const karaokeUrl = process.env.KARAOKE_URL || 'http://localhost:8000'

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    // Одноразовый анализ состава бандла: ANALYZE=1 npm run build.
    // Обычную сборку не трогает.
    ...(process.env.ANALYZE
      ? [visualizer({ filename: 'dist/stats.html', template: 'raw-data', gzipSize: true })]
      : []),
  ],
  build: {
    rollupOptions: {
      output: {
        // Вендор отдельно: редко меняется — висит в immutable-кэше между деплоями,
        // а код приложения обновляется своим чанком. Форма-функция: точный матчинг
        // по папкам node_modules (объектная форма сваливала react в соседний чанк).
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined
          if (
            id.includes('node_modules/react-dom/') ||
            id.includes('node_modules/react/') ||
            id.includes('node_modules/scheduler/')
          ) {
            return 'vendor-react'
          }
          if (
            id.includes('node_modules/react-router') ||
            id.includes('node_modules/@remix-run/router')
          ) {
            return 'vendor-router'
          }
          return undefined
        },
      },
    },
  },
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
