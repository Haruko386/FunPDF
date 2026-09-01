import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  build: {
    chunkSizeWarningLimit: 1000,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          if (id.includes('pdfjs-dist') || id.includes('pdf-lib')) return 'pdf'
          if (id.includes('@fortawesome')) return 'icons'
          if (id.includes('marked') || id.includes('dompurify')) return 'markdown'
          if (id.includes('vue') || id.includes('pinia')) return 'vue'
          return 'vendor'
        },
      },
    },
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 9222,
    proxy: {
      '/api': {
        target: 'http://localhost:9384',
        changeOrigin: true,
      },
    },
  },
})
