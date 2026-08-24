import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import { fileURLToPath, URL } from 'node:url'

// 开发期代理：/api 转发到本地后端
export default defineConfig({
  // 管理后台部署在 /admin 子路径（根路径留给品牌官网）
  base: '/admin/',
  plugins: [
    vue(),
    Components({
      dts: false,
      resolvers: [ElementPlusResolver({ importStyle: 'css' })]
    })
  ],
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
    watch: {
      // Docker Desktop 挂载 Windows 目录时 inotify 事件不传递，必须轮询才能热更新
      usePolling: true,
      interval: 1000
    },
    proxy: {
      '/api': {
        // 默认代理本地后端（docker dev 后端主机端口 8091）；docker dev 环境经 VITE_PROXY_TARGET 指向 backend 容器
        target: process.env.VITE_PROXY_TARGET || 'http://localhost:8091',
        changeOrigin: true
      },
      '/uploads': {
        target: process.env.VITE_PROXY_TARGET || 'http://localhost:8091',
        changeOrigin: true
      }
    }
  },
  build: {
    rollupOptions: {
      output: {
        // vite 8（rolldown）：manualChunks 对象形式已移除，改用 advancedChunks 分组
        advancedChunks: {
          groups: [
            { name: 'element-plus', test: /node_modules[\\/]+(element-plus|@element-plus)/ },
            { name: 'echarts', test: /node_modules[\\/]+(echarts|zrender)/ },
            { name: 'vendor', test: /node_modules[\\/]+(vue|vue-router|pinia|axios)/ }
          ]
        }
      }
    }
  }
})
