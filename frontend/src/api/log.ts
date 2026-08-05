// 日志接口（接口文档 §2.9，只读）
import { request, type PageResult } from '@/utils/request'
import type { OperationLog, LoginLog } from './types'

export interface LogQuery {
  page?: number
  page_size?: number
  username?: string
  status?: number | ''
  start_time?: string
  end_time?: string
}

export function listOperationLogs(params: LogQuery & { module?: string; action?: string }) {
  return request<PageResult<OperationLog>>({ url: '/system/logs/operations', method: 'get', params })
}

export function listLoginLogs(params: LogQuery & { ip?: string }) {
  return request<PageResult<LoginLog>>({ url: '/system/logs/logins', method: 'get', params })
}
