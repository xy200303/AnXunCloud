// 租户上下文：超管在顶栏全局切换器选择数据所属租户；非超管锁定本人租户，无需使用本 store
// 缺省（id 为空）= 默认租户，请求不带 X-Tenant-Id 头，由后端按缺省规则解析
import { defineStore } from 'pinia'

const STORAGE_KEY = 'tenant_context'

interface TenantContext {
  id: string
  name: string
}

function load(): TenantContext {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const v = JSON.parse(raw)
      if (v && typeof v.id === 'string' && v.id) return { id: v.id, name: v.name || '' }
    }
  } catch {
    // 本地缓存损坏时回落默认租户
  }
  return { id: '', name: '' }
}

export const useTenantStore = defineStore('tenant', {
  state: (): TenantContext => load(),
  getters: {
    // 请求头用：空串收敛为 undefined（不带 X-Tenant-Id，后端按默认租户处理）
    contextTenantId: (state): string | undefined => state.id || undefined,
    contextTenantName: (state) => state.name || '默认租户'
  },
  actions: {
    switchTo(id: string, name: string) {
      this.id = id
      this.name = id ? name : ''
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ id: this.id, name: this.name }))
    },
    reset() {
      this.switchTo('', '')
    }
  }
})
