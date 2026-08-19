// 异常工单接口（接口文档 §2.15；P2 闭环状态机：分诊 → 派单 → 完工 → 验收）
import { request } from '@/utils/request'
import type { WorkOrderListResult, WorkOrderDetail } from './biz-types'

export interface WorkOrderQuery {
  page?: number
  page_size?: number
  community_id?: string
  status?: string
  priority?: string
  source?: string
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

// 前台代录建单（source 固定 frontdesk）；photos 为已上传的 file_key 引用；
// 指定 assignee_id 时视为直接派单（进入处理中，省略分诊/派单环节）
export function createWorkOrder(data: {
  community_id: string
  point_id?: string
  title: string
  description: string
  photos?: { file_key: string }[]
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

// 分诊：pass 通过（可选定优先级/分类）→ 待派单；reject 驳回（原因必填）→ 已作废
export function triageWorkOrder(id: string, data: { result: 'pass' | 'reject'; priority?: string; category?: string; note?: string }) {
  return request<{ status: string }>({ url: `/workorders/${id}/triage`, method: 'post', data })
}

// 派单：assignee 须为本项目「工单接单」槽位名单成员（派单即视为接单）
export function dispatchWorkOrder(id: string, data: { assignee_id: string; remark?: string }) {
  return request<{ status: string }>({ url: `/workorders/${id}/dispatch`, method: 'post', data })
}

// 派单候选人：本项目「工单接单」槽位名单成员（后端三级回落解析，口径与派单校验一致）
export interface DispatchCandidate {
  user_id: string
  user_name: string
  phone: string
}
export function listDispatchCandidates(communityId: string) {
  return request<{ list: DispatchCandidate[] }>({ url: '/workorders/dispatch-candidates', method: 'get', params: { community_id: communityId } })
}

// 完工提交（后台代录）；after_photos 按检查项 name 合并到工单 items 的 after_photo_urls（可空）
export function finishWorkOrder(id: string, data: { fix_remark: string; after_photos?: Record<string, string[]> }) {
  return request<{ status: string }>({ url: `/workorders/${id}/finish`, method: 'post', data })
}

// 验收：pass 通过 → 已闭环；reject 不通过（原因必填）→ 退回处理中
export function confirmWorkOrder(id: string, data: { result: 'pass' | 'reject'; confirm_note?: string }) {
  return request<{ status: string }>({ url: `/workorders/${id}/confirm`, method: 'post', data })
}
