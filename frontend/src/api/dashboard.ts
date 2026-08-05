// 工作台统计接口（接口文档 §2.2）
import { request } from '@/utils/request'
import type { DashboardData } from './types'

export function getDashboard(params?: { community_id?: string }) {
  return request<DashboardData>({ url: '/dashboard', method: 'get', params })
}
