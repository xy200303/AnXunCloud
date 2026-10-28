import { request } from '@/utils/request'

export interface AIQuota {
  max_requests_daily: number
  max_tokens_daily: number
  max_concurrent: number
}

export interface AIUsage {
  task_type: string
  date: string
  requests: number
  tokens: number
  running: number
  quota: AIQuota
  source: 'default' | 'tenant'
}

export const getAIUsage = () => request<AIUsage>({ url: '/ai/usage' })
export const saveAIQuota = (taskType: string, data: AIQuota) => request<null>({ url: `/ai/quotas/${taskType}`, method: 'put', data })
export const resetAIQuota = (taskType: string) => request<null>({ url: `/ai/quotas/${taskType}`, method: 'delete' })
