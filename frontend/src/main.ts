import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'element-plus/dist/index.css'

import App from './App.vue'
import router from './router'
import { perms } from './directive/perms'
import '@/styles/index.scss'

const app = createApp(App)

// favicon 走 public 目录，拼接部署子路径（/admin/），避免写死绝对路径在子路径部署下 404
const favicon = document.createElement('link')
favicon.rel = 'icon'
favicon.type = 'image/svg+xml'
favicon.href = `${import.meta.env.BASE_URL}brand/anxuncloud-mark.svg`
document.head.appendChild(favicon)

app.use(createPinia())
app.use(router)
app.use(ElementPlus, { locale: zhCn })

app.directive('perms', perms)

app.mount('#app')
