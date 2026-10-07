import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const backend = 'http://127.0.0.1:8080'

// dev/preview 反向代理：REST 与 WebSocket 一并转发到 wozzle 后端
const proxy = {
  '/api': {
    target: backend,
    changeOrigin: true,
    ws: true,
  },
}

export default defineConfig({
  plugins: [vue()],
  server: { proxy },
  preview: { proxy },
  build: {
    target: 'es2022',
    sourcemap: false,
  },
})
