import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'
import { perms } from './directive/perms'
import '@/styles/index.scss'
import 'element-plus/es/components/message/style/css'
import 'element-plus/es/components/message-box/style/css'

const app = createApp(App)

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
