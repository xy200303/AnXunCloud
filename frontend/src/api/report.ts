// 月度巡检工作报告接口（动态审核链 + PDF 归档）
import { request, type PageResult } from '@/utils/request'

export type ReportStatus = 'pending_review' | 'approved'
export interface ReportReviewStep { slot: string; name: string; mode: 'any' | 'all'; candidate_ids: string[]; users?: { user_id: string; name: string; signed: boolean; signed_at?: string; signature_url?: string | null }[]; signed: any[] }

export interface ReportItem {
  id: string
  community_id: string
  community_name: string
  period: string // YYYY-MM
  title: string
  // 巡查类型：空=综合月报，非空=该类型专项检查报告（patrol_type_label 为后端透出名称）
  patrol_type: string
  patrol_type_label: string
  plan_id: string | null
  status: ReportStatus
  review_steps: ReportReviewStep[]
  review_step: number
  review_current_ids: string[]
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
  issue_count: number
  daily: { date: string; task_total: number; task_done: number; abnormal: number }[]
  /** 设备台账章节（v1.7；旧报告无此字段） */
  equipment?: {
    status_buckets: { normal: number; warning: number; overdue: number; scrap: number; label_missing: number }
    maintenance: { registered: number; confirmed: number; rejected: number }
    spotcheck: { triggered: number; mismatch: number }
    judge_source: { system: number; manual_ai: number }
  }
}

// 打卡记录明细行（与后端 /reports/:id/records 分页接口行结构一致）
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
  patrol_type: string
  patrol_type_label: string
  plan_id: string | null
  status: ReportStatus
  stats: ReportStats
  records: ReportRecord[]
  inspector_ids: string[]
  review_steps: ReportReviewStep[]
  review_step: number
  review_current_ids: string[]
  reject_reason: string
  file_id: string
  file_url: string | null
  created_at: string
  updated_at: string
}

export interface ReportListQuery {
  page?: number
  page_size?: number
  community_id?: string
  period?: string
  // 空=全部；none=仅综合月报；其余按 patrol_type 字典值过滤
  patrol_type?: string
  status?: string
  pending_mine?: string // '1' = 只看待我签
}

export function listReports(params: ReportListQuery) {
  return request<PageResult<ReportItem>>({ url: '/reports', method: 'get', params })
}

export function getReport(id: string) {
  return request<ReportDetail>({ url: `/reports/${id}`, method: 'get' })
}

// 报告打卡明细分页（实时查询，按打卡时间正序；详情页滚动加载用，明细不再随详情整包返回）
export function getReportRecords(id: string, params: { page?: number; page_size?: number }) {
  return request<PageResult<ReportRecord>>({ url: `/reports/${id}/records`, method: 'get', params })
}

// patrol_type 可空=综合月报；非空=该类型专项检查报告（同小区同月按类型各一份）
export function generateReport(data: {
  community_id: string
  period: string
  patrol_type?: string; detail_mode?: string
  plan_id?: string
  sign_steps?: { slot: string; candidate_ids: string[] }[]
}) {
  return request<{ id: string; title: string; status: ReportStatus; regenerated: boolean }>({
    url: '/reports/generate',
    method: 'post',
    data
  })
}

export interface SignCandidate {
  id: string
  name: string
  has_signature: boolean
}

export function getSignCandidates(communityId: string, patrolType?: string) {
  return request<{
    steps: { index: number; slot: string; name: string; mode: 'any' | 'all'; users: SignCandidate[]; default_candidate_ids: string[] }[]
  }>({
    url: '/reports/sign-candidates',
    method: 'get',
    // patrol_type：专项报告主管级默认名单取该类型汇报线槽位
    params: { community_id: communityId, patrol_type: patrolType || undefined }
  })
}

export interface SignBody {
  action: 'approve' | 'reject'
  remark?: string
  reason?: string
  signature_file_id?: string // 一次性签名（未配置手写签名时）
  proxy_for?: string
}

export function signStep(id: string, step: number, data: SignBody) {
  return request<{ status: ReportStatus; review_step: number }>({ url: `/reports/${id}/sign-step/${step}`, method: 'post', data })
}
