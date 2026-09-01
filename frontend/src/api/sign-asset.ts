// 签章资产管理接口（公章 / 用户签名，版本链：active → replaced / revoked）
// 后端并行开发中：以接口契约为准，字段缺失时页面需优雅降级
import { request, type PageResult } from '@/utils/request'

export type SignAssetType = 'user_signature' | 'company_seal'
export type SignAssetStatus = 'active' | 'replaced' | 'revoked'

export interface SignAssetItem {
  id: string
  asset_type: SignAssetType
  owner_id: string | null
  owner_name: string | null
  file_key: string
  url?: string | null
  sha256?: string | null
  version: number
  status: SignAssetStatus
  remark?: string | null
  created_by?: string | null
  created_at: string
  revoked_at?: string | null
  revoked_reason?: string | null
}

export interface SignAssetListQuery {
  page?: number
  page_size?: number
  asset_type?: SignAssetType
  owner_id?: string
  status?: SignAssetStatus | ''
}

// 租户上下文：由请求拦截器统一携带 X-Tenant-Id 头（超管切换后生效），api 层不再传 tenant_id
export function listSignAssets(params: SignAssetListQuery) {
  return request<PageResult<SignAssetItem>>({ url: '/system/sign-assets', method: 'get', params })
}

// 创建即 active；同类型同属主旧版本自动 replaced（company_seal 全局仅一条 active）；file_id 为上传接口返回的文件 ID
export function createSignAsset(data: { asset_type: SignAssetType; owner_id?: string; file_id: string; remark?: string }) {
  return request<SignAssetItem>({ url: '/system/sign-assets', method: 'post', data })
}

// 废止资产，reason 必填
export function revokeSignAsset(id: string, reason: string) {
  return request<{ status: SignAssetStatus }>({ url: `/system/sign-assets/${id}/revoke`, method: 'post', data: { reason } })
}
