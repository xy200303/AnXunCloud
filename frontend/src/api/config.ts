// 系统配置接口（接口文档 §2.7）
import { request, type PageResult } from '@/utils/request'
import type { ConfigItem } from './types'

export function listConfigs(params: { page?: number; page_size?: number; key?: string; name?: string; group?: string }) {
  return request<PageResult<ConfigItem>>({ url: '/system/configs', method: 'get', params })
}

export function listConfigGroups() {
  return request<string[]>({ url: '/system/configs/groups', method: 'get' })
}

export function createConfig(data: { key: string; name: string; value: string; config_group: string; remark?: string }) {
  return request<{ id: string }>({ url: '/system/configs', method: 'post', data })
}

export function updateConfig(id: string, data: { name?: string; value?: string; config_group?: string; remark?: string }) {
  return request<null>({ url: `/system/configs/${id}`, method: 'put', data })
}

export function deleteConfig(id: string) {
  return request<null>({ url: `/system/configs/${id}`, method: 'delete' })
}

// AI 连接性测试：用表单当前值（可未保存）向模型发最小文本请求
export function testAIConnection(data: { protocol: string; base_url: string; api_key: string; model: string }) {
  return request<{ latency_ms: number; reply: string }>({ url: '/system/configs/ai-test', method: 'post', data, timeout: 60000 })
}
