// 月度巡检工作报告接口（三级电子确认签字 + PDF 归档）
import { request, type PageResult } from '@/utils/request'

// 报告状态：pending_inspector → pending_supervisor → pending_manager → approved；驳回回 pending_inspector
export type ReportStatus = 'pending_inspector' | 'pending_supervisor' | 'pending_manager' | 'approved'

export interface ReportItem {
  id: string
  community_id: string
  community_name: string
  period: string // YYYY-MM
  title: string
  status: ReportStatus
  inspector_total: number
  inspector_signed_count: number
  supervisor_name: string | null
  manager_name: string | null
  has_file: boolean
  created_at: string
  updated_at: string
}

// 汇总统计（后端 buildStats 产出，数值型字段）
export interface ReportStats {
  task_total: number
  task_done: number
  task_overdue: number
  should_points: number
  done_points: number
  coverage_rate: number
  abnormal_count: number
  suspect_count: number
  wo_created: number
  wo_closed: number
  wo_unclosed: number
  wo_close_rate: number
  daily: { date: string; task_total: number; task_done: number; abnormal: number }[]
}

export interface ReportInspector {
  user_id: string
  name: string
  signed: boolean
  signed_at: string
  // 手写签名图 URL（后端并行开发中，可能缺省；有值时在签字区展示签名图）
  signature_url?: string | null
}

// 打卡记录明细行（stats.records 快照；历史报告由后端实时查询兜底）
export interface ReportRecord {
  checkin_time: string
  inspector_name: string
  point_name: string
  checkin_type: string // qrcode/fence/nfc/offline
  distance: number | null // 距点位距离（米）
  result: string // normal/abnormal
  is_suspect: boolean
  audit_status: 'auto_pass' | 'pending' | 'pass' | 'rejected'
  photos: { url: string }[]
}

export interface ReportDetail {
  id: string
  community_id: string
  community_name: string
  period: string
  title: string
  status: ReportStatus
  stats: ReportStats
  records: ReportRecord[]
  inspector_ids: string[]
  inspectors: ReportInspector[]
  inspector_signed: { user_id: string; name: string; signed_at: string }[]
  supervisor_by: string | null
  supervisor_name: string | null
  supervisor_at: string | null
  supervisor_remark: string
  supervisor_signature_url?: string | null // 主管手写签名图（可空）
  manager_by: string | null
  manager_name: string | null
  manager_at: string | null
  manager_remark: string
  manager_signature_url?: string | null // 经理手写签名图（可空）
  reject_reason: string
  file_key: string
  file_url: string | null
  created_at: string
  updated_at: string
}

export interface ReportListQuery {
  page?: number
  page_size?: number
  community_id?: string
  period?: string
  status?: string
}

export function listReports(params: ReportListQuery) {
  return request<PageResult<ReportItem>>({ url: '/reports', method: 'get', params })
}

export function getReport(id: string) {
  return request<ReportDetail>({ url: `/reports/${id}`, method: 'get' })
}

export function generateReport(data: { community_id: string; period: string }) {
  return request<{ id: string; title: string; status: ReportStatus; regenerated: boolean }>({
    url: '/reports/generate',
    method: 'post',
    data
  })
}

export function signInspector(id: string) {
  return request<{ status: ReportStatus; signed_count: number; inspector_total: number }>({
    url: `/reports/${id}/sign-inspector`,
    method: 'post'
  })
}

export interface SignBody {
  action: 'approve' | 'reject'
  remark?: string
  reason?: string
}

export function signSupervisor(id: string, data: SignBody) {
  return request<{ status: ReportStatus }>({ url: `/reports/${id}/sign-supervisor`, method: 'post', data })
}

export function signManager(id: string, data: SignBody) {
  return request<{ status: ReportStatus }>({ url: `/reports/${id}/sign-manager`, method: 'post', data })
}
