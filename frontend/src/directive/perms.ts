// v-perms 按钮权限指令：无权限时移除元素
// 用法：<el-button v-perms="'system:user:create'"> 或 v-perms="['a','b']"（满足其一）
import type { Directive } from 'vue'
import { useUserStore } from '@/store/user'

export const perms: Directive = {
  mounted(el, binding) {
    const value: string | string[] = binding.value
    if (!value || (Array.isArray(value) && value.length === 0)) return
    const userStore = useUserStore()
    const required = Array.isArray(value) ? value : [value]
    const ok = userStore.isSuperAdmin || required.some((p) => userStore.perms.includes(p))
    if (!ok) {
      el.parentNode?.removeChild(el)
    }
  }
}
