import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backendUrl = process.env.BACKEND_URL || 'http://localhost:8000'
const backendWs = process.env.BACKEND_WS || 'ws://localhost:8000'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
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
    css: true,
  },
})
