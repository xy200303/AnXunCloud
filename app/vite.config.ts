import { defineConfig } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'

process.env.UNI_INPUT_DIR ||= '.'

export default defineConfig({
  plugins: [uni()],
  resolve: {
    alias: [
      // 项目不用 uniCloud：uni-data-checkbox/select 内部的 mixinDatacom 引用会把
      // @dcloudio/uni-cloud 整包（≈106KB）拖进 bundle，alias 到空壳剔除（仅用 localdata，mixin 为空对象即可）
      { find: '@dcloudio/uni-cloud', replacement: '/scripts/uni-cloud-stub.ts' }
    ]
  },
  server: {
    port: 5174,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8091', changeOrigin: true },
      '/uploads': { target: 'http://127.0.0.1:8091', changeOrigin: true }
    }
  }
})
