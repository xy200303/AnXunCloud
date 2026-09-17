import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { ElMessage } from 'element-plus'

import App from './App.vue'
import router from './router'
import { perms } from './directive/perms'
import '@/styles/index.scss'
import 'element-plus/es/components/message/style/css'
import 'element-plus/es/components/message-box/style/css'

const app = createApp(App)

// chunk 加载失败（重新部署后旧哈希失效）由 router/index.ts 的 onError 整页刷新自愈，
// 全局兜底对这类错误保持静默，避免与自愈逻辑冲突（重复提示/重复刷新）
const CHUNK_ERROR_RE = /dynamically imported module|Importing a module script failed|ChunkLoadError|Failed to fetch/i
const isChunkError = (err: unknown) => CHUNK_ERROR_RE.test((err as Error)?.message || String(err ?? ''))

// Vue 渲染/生命周期内未捕获异常兜底
app.config.errorHandler = (err) => {
  if (isChunkError(err)) return
  console.error('[unhandled vue error]', err)
  ElMessage.error('系统异常，请刷新重试')
}

// Promise 未处理 rejection 兜底（接口拦截器已处理的不会走到这里）
window.onunhandledrejection = (event) => {
  if (isChunkError(event.reason)) return
  console.error('[unhandled rejection]', event.reason)
  ElMessage.error('系统异常，请刷新重试')
}

// favicon 走 public 目录，拼接部署子路径（/admin/），避免写死绝对路径在子路径部署下 404
const favicon = document.createElement('link')
favicon.rel = 'icon'
favicon.type = 'image/svg+xml'
favicon.href = `${import.meta.env.BASE_URL}brand/anxuncloud-mark.svg`
document.head.appendChild(favicon)

app.use(createPinia())
app.use(router)

app.directive('perms', perms)

app.mount('#app')
