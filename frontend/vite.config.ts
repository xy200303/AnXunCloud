import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import { fileURLToPath, URL } from 'node:url'

// 开发期代理：默认转发到统一开发/测试域名；本地联调可用 VITE_PROXY_TARGET 覆盖
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
        target: process.env.VITE_PROXY_TARGET || 'https://pi.hbuer.com',
        changeOrigin: true
      },
      '/uploads': {
        target: process.env.VITE_PROXY_TARGET || 'https://pi.hbuer.com',
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
