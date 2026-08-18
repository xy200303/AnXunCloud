// 租户管理与租户品牌配置接口（P3 多租户；tenant:* 仅超管，tenant:config 租户管理员可管本租户）
import { request, type PageResult } from '@/utils/request'

export interface TenantItem {
  id: string
  code: string
  name: string
  contact_name: string
  contact_phone: string
  status: number // 1 启用 / 0 停用
  user_count: number
  remark: string
  created_at: string
}

export interface TenantQuery {
  page?: number
  page_size?: number
  name?: string
  status?: '' | 'enabled' | 'disabled'
}

// 开通租户：租户信息 + 初始管理员账号（管理员首次登录强制改密）
export interface TenantCreateForm {
  code: string
  name: string
  contact_name?: string
  contact_phone?: string
  remark?: string
  admin_username: string
  admin_password: string
  admin_name?: string
}

export interface TenantUpdateForm {
  name: string
  contact_name?: string
  contact_phone?: string
  remark?: string
}

// 租户品牌配置项（白名单 key：租户覆盖值 + 平台默认值 + 生效值）
export interface TenantConfigItem {
  key: string
  value: string // 租户覆盖值（空=未覆盖）
  platform: string // 平台默认值
  effective: string // 生效值
}

export function listTenants(params: TenantQuery) {
  return request<PageResult<TenantItem>>({ url: '/tenants', method: 'get', params })
}

export function createTenant(data: TenantCreateForm) {
  return request<{ id: string }>({ url: '/tenants', method: 'post', data })
}

export function updateTenant(id: string, data: TenantUpdateForm) {
  return request<null>({ url: `/tenants/${id}`, method: 'put', data })
}

export function updateTenantStatus(id: string, status: number) {
  return request<null>({ url: `/tenants/${id}/status`, method: 'put', data: { status } })
}

// 租户品牌配置（目标租户由请求拦截器 X-Tenant-Id 头承载：超管切换上下文后生效；非超管固定本租户）
export function getTenantConfig() {
  return request<TenantConfigItem[]>({
    url: '/tenant-config',
    method: 'get'
  })
}

// 保存品牌配置：仅白名单 key，空值 = 清除覆盖（回落平台默认）
// 目标租户走 X-Tenant-Id 头（body/query 均不再传 tenant_id）
export function saveTenantConfig(values: Record<string, string>) {
  return request<null>({
    url: '/tenant-config',
    method: 'put',
    data: { values }
  })
}
