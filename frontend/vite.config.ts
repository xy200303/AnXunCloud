import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// 开发期代理：/api 转发到本地后端
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  css: {
    preprocessorOptions: {
      scss: {
        // 设计令牌变量全局注入，组件内直接使用 $color-primary 等
        additionalData: '@use "@/styles/variables.scss" as *;',
        api: 'modern-compiler',
        silenceDeprecations: ['legacy-js-api']
      }
    }
  },
  server: {
    port: 5180,
    strictPort: true,
    proxy: {
      '/api': {
        // 默认代理本地后端；docker dev 环境经 VITE_PROXY_TARGET 指向 backend 容器
        target: process.env.VITE_PROXY_TARGET || 'http://localhost:8090',
        changeOrigin: true
      },
      '/uploads': {
        target: process.env.VITE_PROXY_TARGET || 'http://localhost:8090',
        changeOrigin: true
      }
    }
  },
  build: {
    rollupOptions: {
      output: {
        // 大依赖独立分包，避免单 chunk 过大
        manualChunks: {
          'element-plus': ['element-plus', '@element-plus/icons-vue'],
          echarts: ['echarts'],
          vendor: ['vue', 'vue-router', 'pinia', 'axios']
        }
      }
    }
  }
})
