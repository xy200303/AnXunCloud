// 巡检计划接口（接口文档 §2.12）
import { request, type PageResult } from '@/utils/request'
import type { PlanItem, PlanDetail, PlanForm } from './biz-types'

export function listPlans(params: { page?: number; page_size?: number; community_id?: string; name?: string; cycle_type?: string; status?: number | '' }) {
  return request<PageResult<PlanItem>>({ url: '/inspection/plans', method: 'get', params })
}

export function getPlan(id: string) {
  return request<PlanDetail>({ url: `/inspection/plans/${id}`, method: 'get' })
}

export function createPlan(data: PlanForm) {
  return request<{ id: string }>({ url: '/inspection/plans', method: 'post', data })
}

export function updatePlan(id: string, data: PlanForm) {
  return request<null>({ url: `/inspection/plans/${id}`, method: 'put', data })
}

export function deletePlan(id: string) {
  return request<null>({ url: `/inspection/plans/${id}`, method: 'delete' })
}

export function updatePlanStatus(id: string, status: number) {
  return request<null>({ url: `/inspection/plans/${id}/status`, method: 'put', data: { status } })
}
