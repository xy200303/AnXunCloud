/**
 * 管理端域（App 管理功能：复用 PC 控制器，路由挂在 /api/app 下，见 router.go「管理功能」段）：
 * 今日看板、任务监控/催办、打卡审核、点位管理、小区树、模板、租户切换。
 */

import { httpGet, httpPost, httpPut } from '@/services/request'
import { buildQuery } from './common'
import { OrderPhoto } from './checkin'

// ---- 今日看板 / 任务监控（对齐 StatsService.Dashboard / TaskService.List/Detail/Remind） ----

/** 今日看板 GET /dashboard 响应（today_completion.total/done 为点位粒度计数，rate 为百分比数值） */
export type DashboardData = {
  today_completion: { total: number; done: number; rate: number }
  doing_tasks: number
  overdue_tasks: number
  trend_7d: Array<{ date: string; total: number; done: number; rate: number }>
  community_rank: Array<{ community_id: string; community_name: string; total: number; done: number; rate: number }>
  /** 今日执行动态（task_name 实为打卡点位名，见 StatsService.Dashboard） */
  task_timeline: Array<{ time: string; inspector_name: string; task_id: string; task_name: string; action: string }>
}

/** 任务监控列表项（对齐 TaskService.toItem） */
export type MonitorTask = {
  id: string
  plan_id: string
  plan_name: string
  community_id: string
  community_name: string
  inspector_id: string
  inspector_name: string
  task_date: string
  time_window: string
  /** pending/doing/done/overdue */
  status: string
  total_points: number
  done_points: number
  progress: number
  abnormal_count: number
  suspect_count: number
  missing_count: number
  started_at: string | null
  finished_at: string | null
  /** 当前用户是否在汇报线名单内（false=无催办权限，前端不展示催办按钮） */
  can_remind?: boolean
}

export type MonitorTasksPage = {
  list: MonitorTask[]
  total: number
  page: number
  page_size: number
}

/** 任务明细点位（对齐 TaskService.Detail points 元素；checkin 为打卡摘要，未打卡为 null） */
export type MonitorTaskPoint = {
  point_id: string
  point_name: string
  building_name: string
  sort: number
  credential: string
  require_fence: boolean
  /** pending/done */
  status: string
  checkin: {
    id: string
    checkin_time: string
    checkin_type: string
    distance_to_point: number | null
    result: string
    is_suspect: boolean
    suspect_reason: string
    remark: string
    audit_status: string
  } | null
}

/** 任务监控明细（对齐 TaskService.Detail：{task, points, stats, 分页} 结构，与巡检端 /tasks/:id 不同） */
export type MonitorTaskDetail = {
  task: {
    id: string
    plan_name: string
    community_name: string
    inspector_id: string
    inspector_name: string
    task_date: string
    time_window: string
    status: string
    total_points: number
    done_points: number
    progress: number
    started_at: string | null
    finished_at: string | null
  }
  points: MonitorTaskPoint[]
  /** 全量状态聚合（不随分页变化）：total/done/doing/pending/normal/abnormal/suspect */
  stats: { total: number; done: number; doing: number; pending: number; normal: number; abnormal: number; suspect: number }
  points_total: number
  points_page: number
  points_size: number
}

/** 管理端打卡明细 GET /inspection/checkins/:id（看板/任务监控点位详情；数据范围按小区校验） */
export type AdminCheckinDetail = {
  id: string
  task_id: string
  plan_name: string
  point_id: string
  point_name: string
  community_name: string
  inspector_name: string
  checkin_time: string
  /** qrcode/fence/nfc/offline */
  checkin_type: string
  distance_to_point: number | null
  /** normal/abnormal */
  result: string
  remark: string
  is_suspect: boolean
  suspect_reason: string
  /** 整单照片（由逐项照片聚合，含 EXIF 校验结论 exif_check） */
  photos: OrderPhoto[]
  check_items: Array<{
    name: string
    pass: boolean
    note: string
    photo_urls: string[]
    requirement: string | null
    ai_verdict: string | null
    ai_reason: string | null
    /** ''=未处置 / on_site_resolved / maintenance_registered / report_pending */
    disposition: string
    resolution_note: string
    resolution_photo_urls: string[]
  }>
  audit_status: string
  audit_at: string | null
  audit_remark: string
  ai_verdict: string
  ai_reason: string
}

/** 审核记录列表项（对齐 ReviewService.reviewItem；photos 为 PhotoArray[{item,url,watermarked_url}]） */
export type ReviewRecord = {
  id: string
  task_id: string
  point_id: string
  point_name: string
  community_id: string
  community_name: string
  inspector_id: string
  inspector_name: string
  checkin_time: string
  /** qrcode/fence/nfc/offline */
  checkin_type: string
  distance_to_point: number | null
  /** normal/abnormal */
  result: string
  remark: string
  is_suspect: boolean
  suspect_reason: string
  photos: OrderPhoto[]
  check_items: Array<{
    name: string
    pass: boolean
    note: string
    photos: string[]
    photo_urls?: string[]
    requirement: string | null
    ai_verdict?: string | null
    ai_reason?: string | null
    /** ''=未处置 / on_site_resolved / maintenance_registered / report_pending */
    disposition?: string
    resolution_note?: string
    resolution_photo_urls?: string[]
  }>
  /** pending/passed/rejected */
  audit_status: string
  /** 待审核时：当前环节名（如 主管审核） */
  current_step_name?: string
  /** 待审核时：当前用户是否在该环节授权名单内（false=只能查看，操作会被后端 40304 拦） */
  can_audit?: boolean
  audit_by: string | null
  audit_at: string | null
  audit_remark: string
  ai_verdict: string
  ai_reason: string
}

export type ReviewRecordsPage = {
  list: ReviewRecord[]
  total: number
  page: number
  page_size: number
}

// ---- 点位管理（对齐 PointService.List/Detail/Create/Update + TemplateService.List） -------

/** 点位列表/详情项（对齐 PointService.toItem；status 为 1 启用 / 0 停用） */
export type PointItem = {
  id: string
  community_id: string
  community_name: string
  building_id: string | null
  building_name: string
  name: string
  type: string
  type_label: string
  qrcode_no: string
  nfc_id: string
  /** 多模板组合：点位检查项 = 全部模板并集 */
  template_ids: string[]
  template_names: string[]
  longitude: number
  latitude: number
  fence_radius: number
  /** qrcode/nfc/none/any */
  credential: string
  require_fence: boolean
  sort: number
  status: number
  created_at: string
  remark?: string
}

export type PointsPage = {
  list: PointItem[]
  total: number
  page: number
  page_size: number
}

/** 点位保存请求体（对齐 dto.PointSaveReq；qrcode_no 由后端发号，不可改） */
export type PointSavePayload = {
  community_id: string
  building_id?: string | null
  name: string
  type: string
  longitude: number
  latitude: number
  fence_radius?: number
  credential?: string
  require_fence?: boolean
  template_ids?: string[]
  nfc_id?: string
  sort?: number
  status?: number
  remark?: string
}

/** 小区/楼栋树节点（对齐 dto.CommunityTreeNode） */
export type CommunityTreeNode = {
  id: string
  name: string
  tenant_id?: string
  tenant_name?: string
  buildings: Array<{ id: string; name: string; type: string }>
}

/** 检查项模板列表项（对齐 TemplateService.templateItem） */
export type TemplateListItem = {
  id: string
  name: string
  /** 空为通用模板 */
  point_type: string
  status: number
}

/** 今日看板 GET /dashboard */
export function apiAdminDashboard(): Promise<DashboardData> {
  return new Promise<DashboardData>((resolve, reject) => {
    httpGet<any>('/dashboard')
      .then((d) => {
        if (d == null) {
          reject(new Error('看板响应异常'))
          return
        }
        const tc = d.today_completion ?? {}
        resolve({
          today_completion: { total: tc.total ?? 0, done: tc.done ?? 0, rate: tc.rate ?? 0 },
          doing_tasks: d.doing_tasks ?? 0,
          overdue_tasks: d.overdue_tasks ?? 0,
          trend_7d: d.trend_7d ?? [],
          community_rank: d.community_rank ?? [],
          task_timeline: d.task_timeline ?? []
        })
      })
      .catch(reject)
  })
}

/** 任务监控列表 GET /inspection/tasks（filter: ''=全部 / missing=有漏点 / abnormal=异常 / suspect=疑似） */
export function apiTaskMonitorList(page: number, pageSize: number, filter: string, taskDate?: string): Promise<MonitorTasksPage> {
  const path = '/inspection/tasks' + buildQuery({ page: page, page_size: pageSize, filter: filter, task_date: taskDate })
  return new Promise<MonitorTasksPage>((resolve, reject) => {
    httpGet<any>(path)
      .then((d) => {
        resolve({
          list: (d?.list ?? []) as MonitorTask[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 任务监控明细 GET /inspection/tasks/:id/detail（点位逐个打卡状态；points 分页，默认每页 50） */
export function apiTaskMonitorDetail(id: string, page = 1, pageSize = 50): Promise<MonitorTaskDetail> {
  return new Promise<MonitorTaskDetail>((resolve, reject) => {
    httpGet<any>('/inspection/tasks/' + id + '/detail' + buildQuery({ points_page: page, points_page_size: pageSize }))
      .then((d) => {
        if (d == null || d.task == null) {
          reject(new Error('任务明细响应异常'))
          return
        }
        resolve({
          task: d.task,
          points: (d.points ?? []) as MonitorTaskPoint[],
          stats: d.stats ?? { total: 0, done: 0, doing: 0, pending: 0, normal: 0, abnormal: 0, suspect: 0 },
          points_total: d.points_total ?? 0,
          points_page: d.points_page ?? page,
          points_size: d.points_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 任务催办 POST /inspection/tasks/:id/remind（已完成任务后端报错，直接 toast message） */
export function apiTaskRemind(id: string): Promise<null> {
  return httpPost<null>('/inspection/tasks/' + id + '/remind', null, true)
}

export function apiAdminCheckinDetail(id: string): Promise<AdminCheckinDetail> {
  return new Promise<AdminCheckinDetail>((resolve, reject) => {
    httpGet<AdminCheckinDetail>('/inspection/checkins/' + id)
      .then((d) => {
        if (d == null) {
          reject(new Error('打卡明细响应异常'))
          return
        }
        resolve(d)
      })
      .catch(reject)
  })
}

// ---- 打卡审核（对齐 ReviewService.List / Pass / Reject） -------------------------------

/** 打卡审核记录 GET /inspection/review/records（id 精确查单条：消息深链直达详情用） */
export function apiReviewRecords(page: number, pageSize: number, auditStatus: string, id?: string): Promise<ReviewRecordsPage> {
  const path = '/inspection/review/records' + buildQuery({ page: page, page_size: pageSize, audit_status: auditStatus, id: id })
  return new Promise<ReviewRecordsPage>((resolve, reject) => {
    httpGet<any>(path)
      .then((d) => {
        resolve({
          list: (d?.list ?? []) as ReviewRecord[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 审核通过 POST /inspection/review/:id/pass（仅 pending 可审） */
export function apiReviewPass(id: string): Promise<null> {
  return httpPost<null>('/inspection/review/' + id + '/pass', null, true)
}

/** 审核驳回 POST /inspection/review/:id/reject {reason}（必填） */
export function apiReviewReject(id: string, reason: string): Promise<null> {
  return httpPost<null>('/inspection/review/' + id + '/reject', { reason: reason }, true)
}

/** 租户列表 GET /tenants（超管「当前公司」切换用；tenant:list 权限点，非超管 403） */
export function apiTenants(): Promise<Array<{ id: string; name: string; code: string }>> {
  return new Promise((resolve, reject) => {
    httpGet<any>('/tenants' + buildQuery({ page: 1, page_size: 100 }))
      .then((d) => {
        const list = d != null && d.list != null ? d.list : []
        resolve(list.map((t: any) => ({ id: t.id ?? '', name: t.name ?? '', code: t.code ?? '' })))
      })
      .catch(reject)
  })
}

/** 点位列表 GET /inspection/points（支持类型/凭证/楼栋过滤） */
export function apiPointList(
  page: number,
  pageSize: number,
  communityId: string,
  name: string,
  tenantId = '',
  opts: { type?: string; credential?: string; buildingId?: string } = {}
): Promise<PointsPage> {
  const path = '/inspection/points' + buildQuery({
    page: page,
    page_size: pageSize,
    tenant_id: tenantId,
    community_id: communityId,
    name: name,
    type: opts.type,
    credential: opts.credential,
    building_id: opts.buildingId
  })
  return new Promise<PointsPage>((resolve, reject) => {
    httpGet<any>(path)
      .then((d) => {
        resolve({
          list: (d?.list ?? []) as PointItem[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 点位详情 GET /inspection/points/:id */
export function apiPointDetail(id: string): Promise<PointItem> {
  return new Promise<PointItem>((resolve, reject) => {
    httpGet<any>('/inspection/points/' + id)
      .then((d) => {
        if (d == null) {
          reject(new Error('点位详情响应异常'))
          return
        }
        resolve(d as PointItem)
      })
      .catch(reject)
  })
}

/** 新增点位 POST /inspection/points → {id, qrcode_no} */
export function apiPointCreate(req: PointSavePayload): Promise<{ id: string; qrcode_no: string }> {
  return new Promise<{ id: string; qrcode_no: string }>((resolve, reject) => {
    httpPost<any>('/inspection/points', req as Record<string, any>, true)
      .then((d) => {
        resolve({ id: d?.id ?? '', qrcode_no: d?.qrcode_no ?? '' })
      })
      .catch(reject)
  })
}

/** 更新点位 PUT /inspection/points/:id */
export function apiPointUpdate(id: string, req: PointSavePayload): Promise<null> {
  return httpPut<null>('/inspection/points/' + id, req as Record<string, any>, true)
}

/** 小区/楼栋树 GET /communities/tree */
export function apiCommunityTree(): Promise<CommunityTreeNode[]> {
  return new Promise<CommunityTreeNode[]>((resolve, reject) => {
    httpGet<any>('/communities/tree')
      .then((d) => {
        resolve((d ?? []) as CommunityTreeNode[])
      })
      .catch(reject)
  })
}

/** 检查项模板 GET /inspection/templates（建点位下拉选项，一次取全） */
export function apiTemplateList(): Promise<TemplateListItem[]> {
  return new Promise<TemplateListItem[]>((resolve, reject) => {
    httpGet<any>('/inspection/templates' + buildQuery({ page: 1, page_size: 100 }))
      .then((d) => {
        resolve((d?.list ?? []) as TemplateListItem[])
      })
      .catch(reject)
  })
}
