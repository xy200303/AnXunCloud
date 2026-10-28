import { request, type PageResult } from '@/utils/request'
import type { ReviewFlowStep } from './post'

export type WorkOrderStatus = 'pending' | 'in_progress' | 'pending_review' | 'done' | 'closed'

export interface WorkOrderItem {
  id: string
  order_no: string
  title: string
  work_type: 'inspection_fix' | 'maintenance'
  status: WorkOrderStatus
  priority: string
  community_id: string
  community_name: string
  assignee_id: string | null
  assignee_name: string
  deadline: string | null
  revision: number
  issue_count: number
  can_claim: boolean
  can_finish: boolean
  created_at: string
}

export interface WorkOrderIssue {
  id: string
  title: string
  problem: string
  location: string
  equipment_id: string | null
  status: WorkOrderStatus
  photos: { file_id: string; url: string; watermarked_url?: string }[]
  source_type: string
  source_record_id: string
  source_item_id: string
}

export interface WorkOrderDetail extends Omit<WorkOrderItem, 'issue_count' | 'can_claim' | 'created_at'> {
  can_review: boolean
  review_flow: ReviewFlowStep[]
  review_step: number
  close_reason: string
  completed_at: string | null
  issues: WorkOrderIssue[]
  attempts: { id: string; issue_id: string; round: number; note: string; photos: WorkOrderIssue['photos']; submitted_at: string }[]
  events: { event_type: string; issue_id: string | null; from_status: string; to_status: string; reason: string; actor_id: string; actor_name: string; revision: number; created_at: string }[]
}

export function listWorkOrders(params: { page?: number; page_size?: number; community_id?: string; status?: string; work_type?: string; mine?: string }) {
  return request<PageResult<WorkOrderItem>>({ url: '/workorders', method: 'get', params })
}

export function getWorkOrder(id: string) {
  return request<WorkOrderDetail>({ url: `/workorders/${id}`, method: 'get' })
}

export function claimWorkOrder(id: string) {
  return request<null>({ url: `/workorders/${id}/claim`, method: 'post' })
}

export function assignWorkOrder(id: string, data: { assignee_id: string; reason?: string; operation_id?: string }) {
  return request<null>({ url: `/workorders/${id}/assign`, method: 'post', data })
}

export function finishWorkOrder(id: string, data: { issue_id: string; note: string; photos: string[]; operation_id?: string }) {
  return request<null>({ url: `/workorders/${id}/finish`, method: 'post', data })
}

export function reviewWorkOrder(id: string, data: { issue_id: string; action: 'approve' | 'reject'; reason?: string; operation_id?: string }) {
  return request<null>({ url: `/workorders/${id}/review`, method: 'post', data })
}

export function closeWorkOrder(id: string, data: { reason: string; operation_id?: string }) {
  return request<null>({ url: `/workorders/${id}/close`, method: 'post', data })
}

export function listAssignCandidates(communityId: string, workType: string) {
  return request<{ id: string; name: string; role_name: string }[]>({
    url: '/workorders/candidates', method: 'get', params: { community_id: communityId, work_type: workType }
  })
}
