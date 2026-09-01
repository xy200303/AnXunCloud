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
  // 巡查类型：空=综合月报，非空=该类型专项检查报告（patrol_type_label 为后端透出名称）
  patrol_type: string
  patrol_type_label: string
  plan_id: string | null
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
  issue_count: number
  daily: { date: string; task_total: number; task_done: number; abnormal: number }[]
}

export interface ReportInspector {
  user_id: string
  name: string
  signed: boolean
  signed_at: string
  // 手写签名图 URL（后端并行开发中，可能缺省；有值时在签字区展示签名图）
  signature_url?: string | null
  // 代签留痕（signed=true 且为代签时存在）
  proxy_name?: string
  proxy_reason?: string
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
  patrol_type: string
  patrol_type_label: string
  plan_id: string | null
  status: ReportStatus
  stats: ReportStats
  records: ReportRecord[]
  inspector_ids: string[]
  inspectors: ReportInspector[]
  inspector_signed: { user_id: string; name: string; signed_at: string; proxy_by?: string }[]
  // 指定签字人（生成时圈定；空数组 = 该级跳过，PDF 签字栏留空）
  supervisor_ids: string[]
  supervisors: { user_id: string; name: string; signed: boolean }[]
  manager_ids: string[]
  managers: { user_id: string; name: string; signed: boolean }[]
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

// patrol_type 可空=综合月报；非空=该类型专项检查报告（同小区同月按类型各一份）
export function generateReport(data: {
  community_id: string
  period: string
  patrol_type?: string; detail_mode?: string
  plan_id?: string
  supervisor_ids?: string[]
  manager_ids?: string[]
}) {
  return request<{ id: string; title: string; status: ReportStatus; regenerated: boolean }>({
    url: '/reports/generate',
    method: 'post',
    data
  })
}

// 生成报告时的可选签字人：users 为全部启用用户（主管/经理两级共用此池，可手动圈选，名单即授权）；
// default_*_ids 为职责槽位默认名单（项目级覆盖 → 平台默认绑定 → 编制在职成员；空数组 = 该级默认跳过，不回退全选）
export interface SignCandidate {
  id: string
  name: string
  has_signature: boolean
}

export function getSignCandidates(communityId: string, patrolType?: string) {
  return request<{
    users: SignCandidate[]
    default_supervisor_ids: string[]
    default_manager_ids: string[]
  }>({
    url: '/reports/sign-candidates',
    method: 'get',
    // patrol_type：专项报告主管级默认名单取该类型汇报线槽位
    params: { community_id: communityId, patrol_type: patrolType || undefined }
  })
}

// 巡检员确认：无 body 为本人确认；带 proxy_for+reason 为代签（须 report:sign:proxy 权限）
// signature_file_id：未配置手写签名时随请求提交的一次性签名（签字页弹出签名板手写产生）
export interface InspectorSignPayload {
  proxy_for?: string
  reason?: string
  signature_file_id?: string
}

export function signInspector(id: string, payload?: InspectorSignPayload) {
  return request<{ status: ReportStatus; signed_count: number; inspector_total: number }>({
    url: `/reports/${id}/sign-inspector`,
    method: 'post',
    data: payload
  })
}

export interface SignBody {
  action: 'approve' | 'reject'
  remark?: string
  reason?: string
  signature_file_id?: string // 一次性签名（未配置手写签名时）
}

export function signSupervisor(id: string, data: SignBody) {
  return request<{ status: ReportStatus }>({ url: `/reports/${id}/sign-supervisor`, method: 'post', data })
}

export function signManager(id: string, data: SignBody) {
  return request<{ status: ReportStatus }>({ url: `/reports/${id}/sign-manager`, method: 'post', data })
}
