// 报告生成计划接口（/reports/plans）
import { request } from '@/utils/request'

export interface ReportPlan {
  id: string
  community_id: string
  community_name: string
  name: string
  patrol_type: string
  patrol_type_label: string
  cycle_type: string // daily/weekly/monthly
  cycle_config: Record<string, any>
  cycle_text: string
  gen_time: string
  status: string
  detail_mode: string
  last_period: string
  last_error: string
  remark: string
  created_at: string
}

export interface ReportPlanBody {
  community_id: string
  name: string
  patrol_type?: string
  cycle_type: string
  cycle_config: Record<string, any>
  gen_time: string
  status?: string
  remark?: string
  detail_mode?: string
}

export function listReportPlans(communityId?: string) {
  return request<ReportPlan[]>({ url: '/reports/plans', method: 'get', params: { community_id: communityId || undefined } })
}

export function createReportPlan(data: ReportPlanBody) {
  return request<ReportPlan>({ url: '/reports/plans', method: 'post', data })
}

export function updateReportPlan(id: string, data: ReportPlanBody) {
  return request<ReportPlan>({ url: `/reports/plans/${id}`, method: 'put', data })
}

export function deleteReportPlan(id: string) {
  return request<null>({ url: `/reports/plans/${id}`, method: 'delete' })
}

// 手动触发：立即按周期规则生成上一份完整周期报告
export function runReportPlan(id: string) {
  return request<{ id: string; title: string; status: string; regenerated: boolean }>({
    url: `/reports/plans/${id}/run`, method: 'post'
  })
}
