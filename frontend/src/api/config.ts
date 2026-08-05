// 参数配置接口（接口文档 §2.7）
import { request, type PageResult } from '@/utils/request'
import type { ConfigItem } from './types'

export function listConfigs(params: { page?: number; page_size?: number; key?: string; name?: string }) {
  return request<PageResult<ConfigItem>>({ url: '/system/configs', method: 'get', params })
}

export function createConfig(data: { key: string; name: string; value: string; remark?: string }) {
  return request<{ id: string }>({ url: '/system/configs', method: 'post', data })
}

export function updateConfig(id: string, data: { name?: string; value?: string; remark?: string }) {
  return request<null>({ url: `/system/configs/${id}`, method: 'put', data })
}

export function deleteConfig(id: string) {
  return request<null>({ url: `/system/configs/${id}`, method: 'delete' })
}
