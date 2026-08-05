// 统计报表接口（接口文档 §2.16）
import { request, type PageResult } from '@/utils/request'
import type { CoverageData, TimelinessData, PerformanceItem } from './biz-types'

export interface StatsQuery {
  start_date: string
  end_date: string
  community_id?: string
}

export function getCoverage(params: StatsQuery) {
  return request<CoverageData>({ url: '/stats/coverage', method: 'get', params })
}

export function getTimeliness(params: StatsQuery) {
  return request<TimelinessData>({ url: '/stats/timeliness', method: 'get', params })
}

export function getPerformance(params: StatsQuery & { sort_by?: string; sort_order?: string; page?: number; page_size?: number }) {
  return request<PageResult<PerformanceItem>>({ url: '/stats/performance', method: 'get', params })
}

// 报表导出：异步生成，完成后消息中心通知下载
export function exportReport(data: {
  report_type: 'coverage' | 'timeliness' | 'performance' | 'monthly'
  format: 'excel' | 'pdf'
  start_date: string
  end_date: string
  community_id?: string
}) {
  return request<{ export_id: string; status: string; download_url: string | null }>({
    url: '/stats/export',
    method: 'post',
    data
  })
}
