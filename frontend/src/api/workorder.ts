// 异常工单接口（接口文档 §2.15）
import { request } from '@/utils/request'
import type { WorkOrderListResult, WorkOrderDetail } from './biz-types'

export interface WorkOrderQuery {
  page?: number
  page_size?: number
  community_id?: string
  status?: string
  priority?: string
  assignee_id?: string
  reporter_id?: string
  order_no?: string
  start_time?: string
  end_time?: string
}

export function listWorkOrders(params: WorkOrderQuery) {
  return request<WorkOrderListResult>({ url: '/workorders', method: 'get', params })
}

export function getWorkOrder(id: string) {
  return request<WorkOrderDetail>({ url: `/workorders/${id}`, method: 'get' })
}

export function createWorkOrder(data: {
  community_id: string
  point_id?: string
  title: string
  description: string
  priority?: string
  assignee_id?: string
}) {
  return request<{ id: string; order_no: string; status: string }>({ url: '/workorders', method: 'post', data })
}

export function updateWorkOrder(id: string, data: { title?: string; description?: string; priority?: string }) {
  return request<null>({ url: `/workorders/${id}`, method: 'put', data })
}

export function deleteWorkOrder(id: string) {
  return request<null>({ url: `/workorders/${id}`, method: 'delete' })
}

// 派单
export function assignWorkOrder(id: string, data: { assignee_id: string; remark?: string }) {
  return request<{ status: string }>({ url: `/workorders/${id}/assign`, method: 'post', data })
}

// 处理反馈（后台代录）；after_photos 按检查项 name 合并到工单 items 的 after_photo_urls（可空）
export function finishWorkOrder(id: string, data: { fix_remark: string; after_photos?: Record<string, string[]> }) {
  return request<{ status: string }>({ url: `/workorders/${id}/finish`, method: 'post', data })
}

// 复核：pass 通过 / reject 驳回（驳回必须填原因）
export function reviewWorkOrder(id: string, data: { result: 'pass' | 'reject'; review_remark?: string }) {
  return request<{ status: string }>({ url: `/workorders/${id}/review`, method: 'post', data })
}
