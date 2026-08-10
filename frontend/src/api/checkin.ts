// 打卡记录检索接口（接口文档 §2.14）
import { request, type PageResult } from '@/utils/request'
import type { CheckinItem, CheckinDetail } from './biz-types'

export interface CheckinQuery {
  page?: number
  page_size?: number
  community_id?: string
  point_id?: string
  inspector_id?: string
  task_id?: string
  result?: string
  checkin_type?: string
  audit_status?: string
  is_suspect?: boolean
  start_time?: string
  end_time?: string
}

export function listCheckins(params: CheckinQuery) {
  return request<PageResult<CheckinItem>>({ url: '/inspection/checkins', method: 'get', params })
}

export function getCheckin(id: string) {
  return request<CheckinDetail>({ url: `/inspection/checkins/${id}`, method: 'get' })
}

export interface AuditCounts {
  auto_pass: number
  pending: number
  pass: number
  rejected: number
}

// 各审核状态计数（tab 徽章，与列表共用过滤条件，不含 audit_status）
export function getCheckinAuditCounts(params: CheckinQuery) {
  return request<AuditCounts>({ url: '/inspection/checkins/audit-counts', method: 'get', params })
}
