import { defineStore } from 'pinia'

/**
 * 租户上下文（仅超管有意义）：App 端「当前公司」选择。
 * 持久化到本地存储，请求层逐请求读取注入 X-Tenant-Id 头；
 * 非超管账号即使残留该存储，后端也只认超管身份，无越权风险。
 */

export const TENANT_CTX_KEY = 'tenant_context'

type TenantCtx = { id: string; name: string }

/** 请求层用（不经 pinia，避免模块循环依赖）：读当前租户上下文 id，无 = 空串（后端按默认租户处理） */
export function currentTenantId(): string {
  const v = uni.getStorageSync(TENANT_CTX_KEY) as TenantCtx | '' | null
  if (v == null || v === '' || typeof v !== 'object' || v.id == null) return ''
  return v.id
}

export const useTenantStore = defineStore('tenant', {
  state: () => ({
    /** 当前公司 id（空 = 默认租户） */
    tenantId: '',
    tenantName: ''
  }),
  actions: {
    load() {
      const v = uni.getStorageSync(TENANT_CTX_KEY) as TenantCtx | '' | null
      if (v != null && v !== '' && typeof v === 'object' && v.id != null) {
        this.tenantId = v.id
        this.tenantName = v.name
      }
    },
    set(id: string, name: string) {
      this.tenantId = id
      this.tenantName = name
      const v: TenantCtx = { id: id, name: name }
      uni.setStorageSync(TENANT_CTX_KEY, v)
    },
    clear() {
      this.tenantId = ''
      this.tenantName = ''
      uni.removeStorageSync(TENANT_CTX_KEY)
    }
  }
})
