import { defineConfig } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'

process.env.UNI_INPUT_DIR ||= '.'

export default defineConfig({
  plugins: [uni()],
  server: {
    port: 5174,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8091', changeOrigin: true },
      '/uploads': { target: 'http://127.0.0.1:8091', changeOrigin: true }
    }
  }
})
