import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    host: true,
    allowedHosts: ['.monkeycode-ai.online'],
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    chunkSizeWarningLimit: 2000,
    rollupOptions: {
      output: {
        manualChunks: {
          echarts: ['echarts'],
          tinyvue: ['@opentiny/vue', '@opentiny/vue-icon'],
          vendor: ['vue', 'vue-router', 'pinia', 'axios', 'dayjs', 'dompurify'],
        },
      },
    },
  },
})
