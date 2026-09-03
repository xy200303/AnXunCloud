/**
 * 业务 API 层——与后端 /api/app（小程序 /api/mp）接口一一对应，类型对齐响应结构。
 * 参照 docs/接口文档.md §1.3 信封、§3 移动端接口；/api/app 组与 /api/mp 同形（技术方案 §4）。
 */

import { httpGet, httpPost, httpPut, httpDelete, refreshSession, getBaseUrl, getPublicOrigin } from '@/services/request'
import { getAccessToken } from '@/utils/storage'

// ---- 业务错误码常量 ----

/** 照片质量不达标（信封 data.max_attempts 含放行次数） */
export const CODE_QUALITY_FAIL = 43107
/** AI 未启用（ErrAIDisabled，后端 errs.go 实际值） */
export const CODE_AI_DISABLED = 43108
/** 点位打卡已锁定不可覆盖（ErrCheckinLocked，后端 errs.go 实际值） */
export const CODE_CHECKIN_LOCKED = 43109

// ---- 类型定义（与后端 JSON 蛇形字段对齐；ID 字段全系统 v2 起为 UUIDv7 string） ----

/** 后端 /profile roles 元素：对象（{id, code, name}）或纯字符串 code，两种均兼容 */
export type RoleEntry = string | { id?: string; code?: string; name?: string }

export type UserInfo = {
  id: string
  username: string
  name: string
  phone: string
  avatar: string
  /** 手写签名图 URL（月报签字用，空 = 未配置，签字时弹签名板现场手写） */
  signature_url: string
  /** 权限点集合（report:sign:* 等；超管以 roles 含 super_admin 兜底） */
  perms: string[]
  /** 角色 code 数组；业务身份以 staffs.post_names 岗位为准。 */
  roles: string[]
  /** 我的在职项目（project_staff 推导；name 可空，前端自行解析） */
  projects: Array<{ id: string; name?: string }>
  /** 所属公司（租户名） */
  tenant_name?: string
  /** 在职编制明细：小区 + 岗位名列表 */
  staffs?: Array<{ community_id: string; community_name: string; post_names: string[] }>
}

export type LoginResult = {
  access_token: string
  refresh_token: string
  expires_in: number
  /** APP 账号密码登录是否回包 user 以后端为准，可能为空（为空则登录后调 fetchProfile） */
  user: UserInfo | null
}

export type TodayTask = {
  id: string
  plan_name: string
  community_name: string
  /** 巡查类型：safety 安全 / equipment 设备专项 / environment 环境 / building 楼栋 / fire 消防设施专项（字典驱动） */
  patrol_type: string
  /** 巡查类型字典 label（后端透出；空时前端回落内置映射） */
  patrol_type_label: string
  task_date: string
  time_window: string
  /** 巡更轮次名（任务快照；非轮次任务为空串） */
  round_name: string
  /** pending 待开始 / doing 进行中 / done 已完成 / overdue 已逾期 */
  status: string
  total_points: number
  done_points: number
  progress: number
}

export type TodayTasks = {
  date: string
  total_points: number
  done_points: number
  progress: number
  tasks: TodayTask[]
}

/**
 * GET /points/by-code/:code 响应：点位信息 + 今日任务上下文（任务定位器）。
 * 与后端 MPService.PointByCode 实际返回对齐。
 */
export type PointTaskCtx = {
  task_id: string
  plan_name: string
  status: string
  /** 该任务下此点位是否已打卡 */
  checked: boolean
}

export type PointByCode = {
  point_id: string
  point_name: string
  qrcode_no: string
  /** 点位备案的 NFC 卡号（卡片 UID），用于贴卡预校验；空 = 未绑定 NFC */
  nfc_id: string
  community_id: string
  community_name: string
  building_name: string
  /** 打卡凭证方式：qrcode/nfc/none/any（任一：扫码或 NFC） */
  credential: string
  require_fence: boolean
  longitude: number
  latitude: number
  fence_radius: number
  /** 今日包含该点位的任务（空 = 今日无任务） */
  tasks: PointTaskCtx[]
}

/** 检查项模板项（TaskDetail 点位 check_items 元素） */
export type CheckItemTpl = {
  name: string
  requirement: string
  /** none/optional/required */
  photo_required: string
  /** 判定方式：manual=感官项（人工正常/异常），其余=拍照 AI 识别项；缺省按拍照项处理（待与后端对齐下发） */
  judge_type: string
}

/** 任务明细点位（含我的打卡状态） */
export type TaskPoint = {
  point_id: string
  point_name: string
  building_name: string
  sort: number
  /** qrcode/nfc/none/any */
  credential: string
  require_fence: boolean
  qrcode_no: string
  /** 点位备案的 NFC 卡号（卡片 UID），贴卡预校验用；空 = 未绑定 */
  nfc_id: string
  longitude: number
  latitude: number
  fence_radius: number
  check_items: CheckItemTpl[]
  my_checkin: {
    id: string
    checkin_time: string
    checkin_type: string
    distance_to_point: number | null
    /** 海拔/定位精度（米，可空；仅参考展示） */
    altitude?: number | null
    accuracy?: number | null
    /** normal/abnormal */
    result: string
    is_suspect: boolean
    /** true = 已归档锁定，不可覆盖修改 */
    locked: boolean
  } | null
}

export type TaskDetail = {
  id: string
  plan_name: string
  community_name: string
  /** 巡查类型：safety/equipment/environment/building/fire（字典驱动） */
  patrol_type: string
  /** 巡查类型字典 label（后端透出；空时前端回落内置映射） */
  patrol_type_label: string
  task_date: string
  time_window: string
  /** 巡更轮次名（任务快照；非轮次任务为空串） */
  round_name: string
  status: string
  total_points: number
  done_points: number
  progress: number
  /** 后端是否启用 AI 识别（false/缺省 = 连续巡检回退手动模式） */
  ai_enabled?: boolean
  /** true = 向导异常确认页允许巡检员编辑 AI 描述 */
  ai_result_editable?: boolean
  points: TaskPoint[]
}

/** 打卡逐项填报元素（ai_* 为 AI 预览结论透传落库，可选） */
export type CheckinItemReqPayload = {
  name: string
  pass: boolean
  note: string
  photos: string[]
  /** AI 逐项判定透传：pass/review/abnormal/'' */
  ai_verdict?: string
  ai_reason?: string
  ai_reading?: string
  /** 异常逃生入口的项目异常类型；由服务端草稿校验后写入正式记录 */
  exception_type?: 'device_missing' | 'unable_to_capture' | ''
}

/** 打卡提交请求体（对齐后端 dto.CheckinReq） */
export type CheckinReqPayload = {
  /** 可选：客户端 UUIDv7 幂等 ID（在线打卡不传） */
  id?: string
  task_id: string
  point_id: string
  checkin_type: 'qrcode' | 'fence' | 'nfc'
  qrcode_no?: string
  nfc_id?: string
  longitude: number
  latitude: number
  /** 可选：海拔/定位精度（米，仅参考展示，不参与校验） */
  altitude?: number
  accuracy?: number
  /** YYYY-MM-DD HH:mm:ss（timefmt.Layout） */
  client_time: string
  /** normal/abnormal=巡检员逐项填报（auto 代判已下线，逐项识别走 ai_confirmed 流程） */
  result: 'normal' | 'abnormal'
  /** 可选：质量不达标超放行次数后强制提交（结果转待复核） */
  force?: boolean
  /** true = 巡检员已在识别概要页确认 AI 结论，服务端跳过二次 AI */
  ai_confirmed?: boolean
  remark: string
  /** 逐项填报（照片唯一归属逐项 photos；无记录级照片） */
  check_items?: CheckinItemReqPayload[]
}

/** AI 逐项识别 job 创建请求（POST /checkin/ai-item-jobs） */
export type AiItemJobCreateReq = {
  task_id: string
  point_id: string
  /** 检查项名（与点位模板对齐） */
  name: string
  /** 该项照片 file_id（一项一图硬约束，恰好 1 张） */
  file_ids: string[]
}

/** AI 逐项识别 job 状态（GET /checkin/ai-item-jobs?ids= 元素） */
export type AiItemJob = {
  job_id: string
  /** pending 识别中 / done 完成 / failed 失败（含过期，前端回退重拍） */
  status: 'pending' | 'done' | 'failed' | string
  /** pass / review / abnormal / ''（done 时有效） */
  verdict: string
  reason: string
  /** 仪表读数等识别值，无则空串 */
  reading: string
  /** 照片质量：false 时 quality_issue 为不达标原因，该项需补拍 */
  quality_pass: boolean
  quality_issue: string
}

/** AI 逐项判定（同步判定响应 ai_items 元素；reading 为仪表读数等识别值，无则空串） */
export type CheckinAiItem = {
  name: string
  /** pass / abnormal / review 等 */
  verdict: string
  reason: string
  reading: string
}

/** 打卡响应（对齐后端 resultView） */
export type CheckinResult = {
  checkin_id: string
  checkin_time: string
  distance_to_point: number
  is_suspect: boolean
  suspect_reason: string
  /** 后端是否启用 AI 审核（启用时提交后可轮询 apiCheckinItems 拿逐项结论） */
  ai_enabled: boolean
  /** AI 质量放行次数上限（43107 错误信封 data.max_attempts 同值） */
  ai_max_attempts?: number
  /** 同步判定总判定：pass / review / error */
  ai_verdict?: string
  ai_reason?: string
  /** 照片质量：pass 达标 / 否则 issue 为不达标原因 */
  ai_quality?: { pass: boolean; issue: string }
  ai_items?: CheckinAiItem[]
  /** 审核状态：auto_pass=AI 直接通过 / pending=待管理员复核 */
  audit_status?: string
  task_progress: {
    total_points: number
    done_points: number
    progress: number
    task_status: string
  }
}

/** 打卡逐项 AI 结论（GET /checkins/:id/items 元素；ai_verdict 空 = 模型未给该项结论） */
export type CheckinItemAI = {
  name: string
  pass: boolean
  /** pass / review / error / '' */
  ai_verdict: string
  ai_reason: string
  /** 人工备注（异常项说明） */
  note?: string
  /** 逐项照片可访问 URL（优先水印图；记录卡展示用） */
  photo_urls?: string[]
}

/** 照片元素（后端 types.PhotoItem，打卡/审核记录通用） */
export type OrderPhoto = {
  item: string
  url: string
  watermarked_url: string
}


/** 离线补传响应（对齐 CheckinService.OfflineSync） */
export type OfflineSyncResult = {
  success: Array<{ point_id: string; checkin_id: string; checkin_time: string }>
  failed: Array<{ point_id: string; code: number; message: string }>
}

// ---- 月报签字（/reports，对齐后端 ReportService List/Detail/sign-*） ------------------

/** 报告状态：pending_inspector/pending_supervisor/pending_manager/approved */
export type ReportStatus = string

/** 报告列表项（对齐 ReportService.List 返回） */
export type ReportListItem = {
  id: string
  community_id: string
  community_name: string
  /** YYYY-MM */
  period: string
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

export type ReportListPage = {
  list: ReportListItem[]
  total: number
  page: number
  page_size: number
}

/** 巡检员确认明细（对齐 Detail inspectors 元素；代签带 proxy_name/proxy_reason） */
export type ReportInspector = {
  user_id: string
  name: string
  signed: boolean
  signed_at: string
  signature_url: string | null
  proxy_name?: string
  proxy_reason?: string
}

/** 主管/经理指定签字人明细（signerItems：任一签署即该级完成） */
export type ReportSigner = {
  user_id: string
  name: string
  signed: boolean
}

/** 报告详情（对齐 ReportService.Detail；stats 为 JSONMap 快照，字段按 buildStats） */
export type ReportDetail = {
  id: string
  community_id: string
  community_name: string
  period: string
  title: string
  status: ReportStatus
  stats: Record<string, any>
  inspector_ids: string[]
  inspectors: ReportInspector[]
  inspector_signed: Array<{ user_id: string; proxy_by?: string }>
  supervisor_ids: string[]
  supervisors: ReportSigner[]
  manager_ids: string[]
  managers: ReportSigner[]
  supervisor_by: string | null
  supervisor_name: string | null
  supervisor_at: string | null
  supervisor_remark: string
  supervisor_signature_url: string | null
  manager_by: string | null
  manager_name: string | null
  manager_at: string | null
  manager_remark: string
  manager_signature_url: string | null
  reject_reason: string
  file_id: string
  file_url: string | null
  created_at: string
  updated_at: string
}

/** 主管/经理签批请求（对齐 dto.SignReq：action=approve/reject，驳回 reason 必填） */
export type ReportSignReq = {
  action: 'approve' | 'reject'
  remark?: string
  reason?: string
  /** 未配置签名时随请求提交的一次性签名（须本人 scene=signature 上传） */
  signature_file_id?: string
}

/** 巡检员确认请求（对齐 dto.InspectorSignReq：proxy_for 非空为代签，reason 必填） */
export type InspectorSignReqPayload = {
  proxy_for?: string
  reason?: string
  signature_file_id?: string
}

// ---- 消息 / 公告（对齐 MPService.Messages / NoticeService.Published） -----------------

/** 消息类型：report 月报 / checkin_audit 打卡审核 / announcement 公告 / 其他按系统消息展示 */
export type MessageItem = {
  id: number
  type: string
  title: string
  content: string
  biz_id: string | null
  is_read: boolean
  created_at: string
}

export type MessagesResult = {
  unread_count: number
  list: MessageItem[]
  total: number
  page: number
  page_size: number
}

export type NoticeAttachment = {
  name: string
  url: string
}

export type AnnouncementItem = {
  id: string
  title: string
  content: string
  publish_at: string
  attachments?: NoticeAttachment[]
}

export type AnnouncementsPage = {
  list: AnnouncementItem[]
  total: number
  page: number
  page_size: number
}

/** 公告详情（GET /announcements/:id，仅已发布可见） */
export type AnnouncementDetail = {
  id: string
  title: string
  content: string
  status: number
  attachments: NoticeAttachment[]
  publish_at: string
  created_by: string
  created_at: string
}

// ---- 后端原始响应类型（信封 data 形状） -------------------------------------------

type RawUser = {
  id?: string | number
  username?: string
  name?: string
  phone?: string
  avatar?: string
  signature_url?: string
  perms?: string[]
  roles?: RoleEntry[]
  projects?: Array<{ id?: string | number; name?: string }>
  tenant_name?: string
  staffs?: Array<{ community_id?: string | number; community_name?: string; post_names?: string[] }>
}

type RawLoginData = {
  access_token?: string
  refresh_token?: string
  expires_in?: number
  user?: RawUser | null
}

type RawPoint = {
  id?: string | number
  name?: string
  qrcode_no?: string
  nfc_id?: string
  community_id?: string | number
  community_name?: string
  building_name?: string
  credential?: string
  require_fence?: boolean
  longitude?: number
  latitude?: number
  fence_radius?: number
}

type RawPointByCodeData = {
  point?: RawPoint | null
  tasks?: Array<{
    task_id?: string | number
    plan_name?: string
    status?: string
    checked?: boolean
  }>
}

type RawTaskDetail = {
  id?: string | number
  plan_name?: string
  community_name?: string
  patrol_type?: string
  patrol_type_label?: string
  task_date?: string
  time_window?: string
  round_name?: string
  status?: string
  total_points?: number
  done_points?: number
  progress?: number
  ai_enabled?: boolean
  ai_result_editable?: boolean
  points?: Array<{
    point_id?: string | number
    point_name?: string
    building_name?: string
    sort?: number
    credential?: string
    require_fence?: boolean
    qrcode_no?: string
    nfc_id?: string
    longitude?: number
    latitude?: number
    fence_radius?: number
    check_items?: Array<{ name?: string; requirement?: string; photo_required?: string; judge_type?: string }>
    my_checkin?: {
      id?: string | number
      checkin_time?: string
      checkin_type?: string
      distance_to_point?: number | null
      result?: string
      is_suspect?: boolean
      locked?: boolean
    } | null
  }>
}

type RawAiItemJob = {
  job_id?: string
  status?: string
  verdict?: string
  reason?: string
  reading?: string
  quality_pass?: boolean
  quality_issue?: string
}

type RawCheckinResult = {
  checkin_id?: string | number
  checkin_time?: string
  distance_to_point?: number
  is_suspect?: boolean
  suspect_reason?: string
  ai_enabled?: boolean
  ai_max_attempts?: number
  ai_verdict?: string
  ai_reason?: string
  ai_quality?: { pass?: boolean; issue?: string }
  ai_items?: Array<{ name?: string; verdict?: string; reason?: string; reading?: string }>
  audit_status?: string
  task_progress?: {
    total_points?: number
    done_points?: number
    progress?: number
    task_status?: string
  }
}

/** uni.uploadFile 信封解析用（不走 request.ts） */
type ApiEnvelopeLike = {
  code: number
  message: string
  data?: { file_id?: string; url?: string } | null
}

// ---- 字段提取辅助（ID 字段容错：后端若下发 number 则转 string） --------------------

function toId(v: string | number | undefined): string {
  return v != null ? String(v) : ''
}

export function toUserInfo(raw: RawUser): UserInfo {
  return {
    id: toId(raw.id),
    username: raw.username ?? '',
    name: raw.name ?? '',
    phone: raw.phone ?? '',
    avatar: raw.avatar ?? '',
    signature_url: raw.signature_url ?? '',
    perms: (raw.perms ?? []).map((p) => String(p)),
    roles: (raw.roles ?? [])
      .map((r) => (typeof r == 'string' ? r : r.code ?? ''))
      .filter((c) => c != ''),
    projects: (raw.projects ?? []).map((p) => ({ id: toId(p.id), name: p.name })),
    tenant_name: raw.tenant_name ?? '',
    staffs: (raw.staffs ?? []).map((s) => ({
      community_id: toId(s.community_id),
      community_name: s.community_name ?? '',
      post_names: (s.post_names ?? []).map((n) => String(n))
    }))
  }
}

/** 权限点判断（超管以 super_admin 角色兜底，与后端通配策略同口径；旧缓存用户缺 perms 字段时容错） */
export function hasPerm(u: UserInfo | null, perm: string): boolean {
  if (u == null) return false
  if ((u.roles ?? []).indexOf('super_admin') >= 0) return true
  return (u.perms ?? []).indexOf(perm) >= 0
}

// ---- 认证 ------------------------------------------------------------------------

/** 登录 POST /login {username, password, tenant_code?}（用户名跨租户重名时传所选公司 code 消歧） */
export function apiLogin(username: string, password: string, tenantCode?: string): Promise<LoginResult> {
  return new Promise<LoginResult>((resolve, reject) => {
    httpPost<RawLoginData>('/login', { username: username, password: password, tenant_code: tenantCode || undefined }, false)
      .then((d) => {
        if (d == null) {
          reject(new Error('登录响应异常'))
          return
        }
        resolve({
          access_token: d.access_token ?? '',
          refresh_token: d.refresh_token ?? '',
          expires_in: d.expires_in ?? 0,
          user: d.user != null ? toUserInfo(d.user) : null
        })
      })
      .catch(reject)
  })
}


/** 注册开关 GET /auth/register-config → data.enabled */
export function apiRegisterConfig(): Promise<boolean> {
  return new Promise<boolean>((resolve, reject) => {
    httpGet<{ enabled?: boolean }>('/auth/register-config', false)
      .then((d) => {
        resolve(d?.enabled ?? false)
      })
      .catch(reject)
  })
}

/** 注册可选公司列表 GET /auth/register-tenants → [{code, name}]（免登录，注册开关关闭时 40303） */
export function apiRegisterTenants(): Promise<Array<{ code: string; name: string }>> {
  return new Promise((resolve, reject) => {
    httpGet<Array<{ code?: string; name?: string }>>('/auth/register-tenants', false)
      .then((d) => {
        resolve((d ?? []).map((t) => ({ code: t.code ?? '', name: t.name ?? '' })))
      })
      .catch(reject)
  })
}

/** 注册 POST /auth/register {username,password,name,phone,tenant_code?}（tenant_code 选填：多租户时传所选公司 code，不传 = 默认租户） */
export function apiRegister(username: string, password: string, name: string, phone: string, tenantCode?: string): Promise<null> {
  return httpPost<null>('/auth/register', {
    username: username,
    password: password,
    name: name,
    phone: phone,
    tenant_code: tenantCode || undefined
  }, false)
}

/** 登出 POST /auth/logout */
export function apiLogout(): Promise<null> {
  return httpPost<null>('/auth/logout', null, true)
}

// ---- 推送设备绑定（uniPush 2.0；后端契约 /push/device，与 sys_message 同源下发） -------

/** 推送设备绑定请求体（POST /push/device） */
export type PushDeviceBindPayload = {
  /** uniPush CID（uni.getPushClientId 获取） */
  cid: string
  /** android / ios */
  platform: string
}

/** 绑定推送设备 POST /push/device（同 cid 重复绑定会改绑到当前用户；登录态） */
export function apiBindPushDevice(cid: string, platform: string): Promise<null> {
  const body: PushDeviceBindPayload = { cid: cid, platform: platform }
  return httpPost<null>('/push/device', body as unknown as Record<string, any>, true)
}

/** 解绑推送设备 DELETE /push/device（登出清 token 前调用） */
export function apiUnbindPushDevice(cid: string): Promise<null> {
  return httpDelete<null>('/push/device', { cid: cid }, true)
}

// ---- 业务 ------------------------------------------------------------------------

/** 个人信息 GET /profile */
export function apiProfile(): Promise<UserInfo> {
  return new Promise<UserInfo>((resolve, reject) => {
    httpGet<RawUser>('/profile')
      .then((d) => {
        if (d == null) {
          reject(new Error('个人信息响应异常'))
        } else {
          resolve(toUserInfo(d))
        }
      })
      .catch(reject)
  })
}

/**
 * 修改本人资料 PUT /profile（对齐 userCtl.UpdateProfile：name/phone 必传）；
 * signatureFileID 不传 = 不改动签名；传入则创建/替换当前用户 active 签名资产。
 * avatarFileID 不传 = 不改动头像；传入则更新头像。
 */
export function apiUpdateProfile(name: string, phone: string, signatureFileID?: string, avatarFileID?: string): Promise<UserInfo> {
  const body: Record<string, any> = { name: name, phone: phone }
  if (signatureFileID != null) body.signature_file_id = signatureFileID
  if (avatarFileID != null) body.avatar = avatarFileID
  return new Promise<UserInfo>((resolve, reject) => {
    httpPut<RawUser>('/profile', body, true)
      .then((d) => {
        if (d == null) {
          reject(new Error('资料更新响应异常'))
        } else {
          resolve(toUserInfo(d))
        }
      })
      .catch(reject)
  })
}

/** 今日任务 GET /tasks/today */
export function apiTasksToday(): Promise<TodayTasks> {
  return tasksByDate('/tasks/today')
}

/** 历史任务 GET /tasks/history?date=YYYY-MM-DD（逾期任务可进详情补拍） */
export function apiTasksHistory(date: string): Promise<TodayTasks> {
  return tasksByDate('/tasks/history?date=' + encodeURIComponent(date))
}

/** 按日期任务列表（今日/历史共用响应结构） */
function tasksByDate(path: string): Promise<TodayTasks> {
  return new Promise<TodayTasks>((resolve, reject) => {
    httpGet<TodayTasks>(path)
      .then((d) => {
        if (d == null) {
          reject(new Error('任务响应异常'))
          return
        }
        resolve({
          date: d.date ?? '',
          total_points: d.total_points ?? 0,
          done_points: d.done_points ?? 0,
          progress: d.progress ?? 0,
          tasks: d.tasks ?? []
        })
      })
      .catch(reject)
  })
}

/** 扫码定位 GET /points/by-code/:code */
export function apiPointByCode(code: string): Promise<PointByCode> {
  return new Promise<PointByCode>((resolve, reject) => {
    httpGet<RawPointByCodeData>('/points/by-code/' + code)
      .then((d) => {
        if (d == null || d.point == null) {
          reject(new Error('点位不存在或不在今日任务中'))
          return
        }
        const p = d.point
        resolve({
          point_id: toId(p.id),
          point_name: p.name ?? '',
          qrcode_no: p.qrcode_no ?? '',
          nfc_id: p.nfc_id ?? '',
          community_id: toId(p.community_id),
          community_name: p.community_name ?? '',
          building_name: p.building_name ?? '',
          credential: p.credential ?? '',
          require_fence: p.require_fence ?? false,
          longitude: p.longitude ?? 0,
          latitude: p.latitude ?? 0,
          fence_radius: p.fence_radius ?? 0,
          tasks: (d.tasks ?? []).map((t) => ({
            task_id: toId(t.task_id),
            plan_name: t.plan_name ?? '',
            status: t.status ?? '',
            checked: t.checked ?? false
          }))
        })
      })
      .catch(reject)
  })
}

/** 附近点位行（GET /points/nearby）：今日任务点位按距离升序，未打卡优先 */
export type NearbyPoint = {
  task_id: string
  plan_name: string
  patrol_type: string
  point_id: string
  point_name: string
  building_name: string
  /** 与我的距离（米） */
  distance: number
  checked: boolean
  credential: string
  require_fence: boolean
  task_status: string
}

/** 附近点位 GET /points/nearby?longitude&latitude（找点辅助：GPS 只能定位到楼栋级，楼内仍需扫码/NFC 确认） */
export function apiNearbyPoints(longitude: number, latitude: number): Promise<{ list: NearbyPoint[]; ai_enabled: boolean }> {
  return new Promise((resolve, reject) => {
    httpGet<{ list: NearbyPoint[]; ai_enabled: boolean }>('/points/nearby?longitude=' + longitude + '&latitude=' + latitude)
      .then((d) => resolve({ list: d?.list ?? [], ai_enabled: d?.ai_enabled ?? false }))
      .catch(reject)
  })
}

/** 任务明细 GET /tasks/:id（点位路线 + 检查项模板 + 我的打卡状态） */
export function apiTaskDetail(id: string): Promise<TaskDetail> {
  return new Promise<TaskDetail>((resolve, reject) => {
    httpGet<RawTaskDetail>('/tasks/' + id)
      .then((d) => {
        if (d == null) {
          reject(new Error('任务详情响应异常'))
          return
        }
        resolve({
          id: toId(d.id),
          plan_name: d.plan_name ?? '',
          community_name: d.community_name ?? '',
          patrol_type: d.patrol_type ?? '',
          patrol_type_label: d.patrol_type_label ?? '',
          task_date: d.task_date ?? '',
          time_window: d.time_window ?? '',
          round_name: d.round_name ?? '',
          status: d.status ?? '',
          total_points: d.total_points ?? 0,
          done_points: d.done_points ?? 0,
          progress: d.progress ?? 0,
          ai_enabled: d.ai_enabled ?? false,
          ai_result_editable: d.ai_result_editable ?? false,
          points: (d.points ?? []).map((p) => ({
            point_id: toId(p.point_id),
            point_name: p.point_name ?? '',
            building_name: p.building_name ?? '',
            sort: p.sort ?? 0,
            credential: p.credential ?? '',
            require_fence: p.require_fence ?? false,
            qrcode_no: p.qrcode_no ?? '',
            nfc_id: p.nfc_id ?? '',
            longitude: p.longitude ?? 0,
            latitude: p.latitude ?? 0,
            fence_radius: p.fence_radius ?? 0,
            check_items: (p.check_items ?? []).map((c) => ({
              name: c.name ?? '',
              requirement: c.requirement ?? '',
              photo_required: c.photo_required ?? '',
              judge_type: c.judge_type ?? ''
            })),
            my_checkin: p.my_checkin == null
              ? null
              : {
                  id: toId(p.my_checkin.id),
                  checkin_time: p.my_checkin.checkin_time ?? '',
                  checkin_type: p.my_checkin.checkin_type ?? '',
                  distance_to_point: p.my_checkin.distance_to_point ?? null,
                  result: p.my_checkin.result ?? '',
                  is_suspect: p.my_checkin.is_suspect ?? false,
                  locked: p.my_checkin.locked ?? false
                }
          }))
        })
      })
      .catch(reject)
  })
}

/** 打卡提交 POST /checkin */
export function apiCheckin(req: CheckinReqPayload): Promise<CheckinResult> {
  return new Promise<CheckinResult>((resolve, reject) => {
    httpPost<RawCheckinResult>('/checkin', req as unknown as Record<string, any>)
      .then((d) => {
        if (d == null) {
          reject(new Error('打卡响应异常'))
          return
        }
        const tp = d.task_progress ?? {}
        resolve({
          checkin_id: toId(d.checkin_id),
          checkin_time: d.checkin_time ?? '',
          distance_to_point: d.distance_to_point ?? 0,
          is_suspect: d.is_suspect ?? false,
          suspect_reason: d.suspect_reason ?? '',
          ai_enabled: d.ai_enabled ?? false,
          ai_max_attempts: d.ai_max_attempts ?? 0,
          ai_verdict: d.ai_verdict ?? '',
          ai_reason: d.ai_reason ?? '',
          ai_quality: d.ai_quality == null
            ? undefined
            : { pass: d.ai_quality.pass ?? false, issue: d.ai_quality.issue ?? '' },
          ai_items: (d.ai_items ?? []).map((it) => ({
            name: it.name ?? '',
            verdict: it.verdict ?? '',
            reason: it.reason ?? '',
            reading: it.reading ?? ''
          })),
          audit_status: d.audit_status ?? '',
          task_progress: {
            total_points: tp.total_points ?? 0,
            done_points: tp.done_points ?? 0,
            progress: tp.progress ?? 0,
            task_status: tp.task_status ?? ''
          }
        })
      })
      .catch(reject)
  })
}

/** 创建 AI 逐项识别 job POST /checkin/ai-item-jobs（拍照项拍完立即调用，异步轮询结果） */
export function apiAiItemJobCreate(req: AiItemJobCreateReq): Promise<{ job_id: string }> {
  return new Promise<{ job_id: string }>((resolve, reject) => {
    httpPost<{ job_id?: string | number }>('/checkin/ai-item-jobs', req as unknown as Record<string, any>)
      .then((d) => {
        if (d == null || d.job_id == null) {
          reject(new Error('识别任务响应异常'))
          return
        }
        resolve({ job_id: toId(d.job_id) })
      })
      .catch(reject)
  })
}

/** 批量查询 AI 逐项识别 job GET /checkin/ai-item-jobs?ids=a,b,c（点位收尾轮询用） */
export function apiAiItemJobs(ids: string[]): Promise<AiItemJob[]> {
  return new Promise<AiItemJob[]>((resolve, reject) => {
    httpGet<{ jobs?: RawAiItemJob[] }>('/checkin/ai-item-jobs?ids=' + ids.map(encodeURIComponent).join(','))
      .then((d) => {
        resolve(
          (d?.jobs ?? []).map((j) => ({
            job_id: j.job_id ?? '',
            status: j.status ?? '',
            verdict: j.verdict ?? '',
            reason: j.reason ?? '',
            reading: j.reading ?? '',
            quality_pass: j.quality_pass ?? true,
            quality_issue: j.quality_issue ?? ''
          }))
        )
      })
      .catch(reject)
  })
}

/** 逐项过程草稿（GET /checkin/item-drafts 元素）：云端保存的逐项进度（巡检进度的唯一事实来源） */
export interface ItemDraft {
  point_id: string
  item_name: string
  job_id: string
  file_ids: string[]
  photos: string[]
  ai_status: string
  ai_verdict: string
  ai_reason: string
  ai_reading: string
  exception_type?: string
  quality_pass: boolean
  quality_issue: string
  manual_pass: boolean | null
  manual_note: string
}

/** 查询逐项过程草稿 GET /checkin/item-drafts?task_id[&point_id]（pointId 空=整个任务；断点恢复用） */
export function apiItemDrafts(taskId: string, pointId?: string): Promise<ItemDraft[]> {
  return new Promise<ItemDraft[]>((resolve, reject) => {
    let url = '/checkin/item-drafts?task_id=' + encodeURIComponent(taskId)
    if (pointId != null && pointId != '') url += '&point_id=' + encodeURIComponent(pointId)
    httpGet<{ items?: any[] }>(url)
      .then((d) => {
        resolve(
          (d?.items ?? []).map((it) => ({
            point_id: it.point_id ?? '',
            item_name: it.item_name ?? '',
            job_id: it.job_id ?? '',
            file_ids: it.file_ids ?? [],
            photos: it.photos ?? [],
            ai_status: it.ai_status ?? '',
            ai_verdict: it.ai_verdict ?? '',
            ai_reason: it.ai_reason ?? '',
            ai_reading: it.ai_reading ?? '',
            exception_type: it.exception_type ?? '',
            quality_pass: it.quality_pass ?? true,
            quality_issue: it.quality_issue ?? '',
            manual_pass: it.manual_pass ?? null,
            manual_note: it.manual_note ?? ''
          }))
        )
      })
      .catch(reject)
  })
}

/** 手动确认项选择落云端草稿 POST /checkin/item-drafts/manual */
export function apiItemDraftManual(req: { task_id: string; point_id: string; name: string; pass: boolean; note: string }): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    httpPost('/checkin/item-drafts/manual', req as unknown as Record<string, any>)
      .then(() => resolve())
      .catch(reject)
  })
}

/** 拍照项异常逃生入口 POST /checkin/item-drafts/photo-abnormal */
export function apiItemDraftPhotoAbnormal(req: { task_id: string; point_id: string; name: string; file_ids: string[]; note: string; exception_type: 'device_missing' | 'unable_to_capture' }): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    httpPost('/checkin/item-drafts/photo-abnormal', req as unknown as Record<string, any>)
      .then(() => resolve())
      .catch(reject)
  })
}

/** 打卡逐项 AI 结论 GET /checkins/:id/items（本人记录；AI 审核异步，提交后延迟轮询用） */
export function apiCheckinItems(checkinId: string): Promise<CheckinItemAI[]> {
  return new Promise<CheckinItemAI[]>((resolve, reject) => {
    httpGet<CheckinItemAI[]>('/checkins/' + checkinId + '/items')
      .then((d) => {
        if (d == null) {
          reject(new Error('打卡逐项结论响应异常'))
          return
        }
        resolve(
          d.map((it) => ({
            name: it.name ?? '',
            pass: it.pass ?? false,
            ai_verdict: it.ai_verdict ?? '',
            ai_reason: it.ai_reason ?? ''
          }))
        )
      })
      .catch(reject)
  })
}

/** scene：checkin / avatar / signature，默认 checkin。 */
export function apiUploadLocal(
  filePath: string,
  scene: 'checkin' | 'avatar' | 'signature' = 'checkin',
  retried = false
): Promise<{ file_id: string; url: string }> {
  return new Promise((resolve, reject) => {
    uni.uploadFile({
      url: getBaseUrl() + '/upload/local',
      filePath: filePath,
      name: 'file',
      formData: { scene: scene },
      header: { Authorization: 'Bearer ' + getAccessToken() },
      timeout: 60000, // 弱网保底：60s 必 fail，防止上传永久挂起
      success: (res) => {
        let env: ApiEnvelopeLike | null = null
        try {
          env = JSON.parse(res.data) as ApiEnvelopeLike
        } catch (e) {
          env = null
        }
        if (env == null || typeof env.code != 'number') {
          reject(new Error('上传响应格式异常'))
          return
        }
        if (env.code != 0) {
          if (env.code == 40102 && !retried) {
            refreshSession().then((ok) => {
              if (!ok) {
                reject(new Error('登录状态已失效，请重新登录'))
                return
              }
              apiUploadLocal(filePath, scene, true).then(resolve).catch(reject)
            }).catch(() => reject(new Error('登录状态已失效，请重新登录')))
            return
          }
          reject(new Error(env.message || '上传失败'))
          return
        }
        resolve({
          file_id: env.data?.file_id ?? '',
          url: env.data?.url ?? ''
        })
      },
      fail: () => {
        reject(new Error('网络异常，照片上传失败'))
      }
    })
  })
}

/** 离线补传 POST /checkin/offline-sync（items min=1；逐条处理，单条失败不影响其他） */
export function apiOfflineSync(items: CheckinReqPayload[]): Promise<OfflineSyncResult> {
  return new Promise<OfflineSyncResult>((resolve, reject) => {
    httpPost<any>('/checkin/offline-sync', { items: items })
      .then((d) => {
        resolve({
          success: d?.success ?? [],
          failed: d?.failed ?? []
        })
      })
      .catch(reject)
  })
}

// ---- 月报签字接口 ----------------------------------------------------------------

/** 报告列表 GET /reports（pendingMine=true 只看待我签；signedMine: '1'=我签过的+已归档 'doing'=我签过未归档；status 按状态过滤） */
export function apiReports(page: number, pageSize: number, pendingMine: boolean, status?: string, signedMine?: string): Promise<ReportListPage> {
  let path = '/reports?page=' + page + '&page_size=' + pageSize
  if (pendingMine) path += '&pending_mine=1'
  if (signedMine != null && signedMine != '') path += '&signed_mine=' + signedMine
  if (status != null && status != '') path += '&status=' + encodeURIComponent(status)
  return new Promise<ReportListPage>((resolve, reject) => {
    httpGet<any>(path)
      .then((d) => {
        resolve({
          list: (d?.list ?? []) as ReportListItem[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 报告详情 GET /reports/:id（stats 全量 + 三级签字明细） */
export function apiReportDetail(id: string): Promise<ReportDetail> {
  return new Promise<ReportDetail>((resolve, reject) => {
    httpGet<any>('/reports/' + id)
      .then((d) => {
        if (d == null) {
          reject(new Error('报告详情响应异常'))
          return
        }
        resolve({
          id: d.id ?? '',
          community_id: d.community_id ?? '',
          community_name: d.community_name ?? '',
          period: d.period ?? '',
          title: d.title ?? '',
          status: d.status ?? '',
          stats: (d.stats ?? {}) as Record<string, any>,
          inspector_ids: (d.inspector_ids ?? []).map((x: any) => String(x)),
          inspectors: (d.inspectors ?? []).map((p: any) => ({
            user_id: String(p.user_id ?? ''),
            name: p.name ?? '',
            signed: p.signed ?? false,
            signed_at: p.signed_at ?? '',
            signature_url: p.signature_url ?? null,
            proxy_name: p.proxy_name,
            proxy_reason: p.proxy_reason
          })),
          inspector_signed: (d.inspector_signed ?? []) as Array<{ user_id: string; proxy_by?: string }>,
          supervisor_ids: (d.supervisor_ids ?? []).map((x: any) => String(x)),
          supervisors: (d.supervisors ?? []) as ReportSigner[],
          manager_ids: (d.manager_ids ?? []).map((x: any) => String(x)),
          managers: (d.managers ?? []) as ReportSigner[],
          supervisor_by: d.supervisor_by ?? null,
          supervisor_name: d.supervisor_name ?? null,
          supervisor_at: d.supervisor_at ?? null,
          supervisor_remark: d.supervisor_remark ?? '',
          supervisor_signature_url: d.supervisor_signature_url ?? null,
          manager_by: d.manager_by ?? null,
          manager_name: d.manager_name ?? null,
          manager_at: d.manager_at ?? null,
          manager_remark: d.manager_remark ?? '',
          manager_signature_url: d.manager_signature_url ?? null,
          reject_reason: d.reject_reason ?? '',
          file_id: d.file_id ?? '',
          file_url: d.file_url ?? null,
          created_at: d.created_at ?? '',
          updated_at: d.updated_at ?? ''
        })
      })
      .catch(reject)
  })
}

/** 巡检员电子确认 POST /reports/:id/sign-inspector（空 body 本人确认；proxy_for+reason 代签） */
export function apiSignInspector(id: string, req: InspectorSignReqPayload | null): Promise<{ status: string }> {
  return new Promise<{ status: string }>((resolve, reject) => {
    httpPost<any>('/reports/' + id + '/sign-inspector', req as Record<string, any> | null, true)
      .then((d) => {
        resolve({ status: d?.status ?? '' })
      })
      .catch(reject)
  })
}

/** 主管审批 POST /reports/:id/sign-supervisor（action=approve/reject） */
export function apiSignSupervisor(id: string, req: ReportSignReq): Promise<{ status: string }> {
  return new Promise<{ status: string }>((resolve, reject) => {
    httpPost<any>('/reports/' + id + '/sign-supervisor', req as unknown as Record<string, any>, true)
      .then((d) => {
        resolve({ status: d?.status ?? '' })
      })
      .catch(reject)
  })
}

/** 经理终审 POST /reports/:id/sign-manager（action=approve/reject；approve 后异步归档 PDF） */
export function apiSignManager(id: string, req: ReportSignReq): Promise<{ status: string }> {
  return new Promise<{ status: string }>((resolve, reject) => {
    httpPost<any>('/reports/' + id + '/sign-manager', req as unknown as Record<string, any>, true)
      .then((d) => {
        resolve({ status: d?.status ?? '' })
      })
      .catch(reject)
  })
}

/** 签发报告 PDF 预览 ticket POST /reports/:id/pdf-ticket（web-view 无法带登录头，凭 ticket 走限时公开通道） */
export function apiReportPdfTicket(id: string): Promise<string> {
  return new Promise<string>((resolve, reject) => {
    httpPost<any>('/reports/' + id + '/pdf-ticket', null, true)
      .then((d) => {
        if (d == null || d.ticket == null || d.ticket == '') {
          reject(new Error('预览凭证签发失败'))
          return
        }
        resolve(String(d.ticket))
      })
      .catch(reject)
  })
}

/**
 * 查看报告 PDF。
 * - App 端：签 ticket → web-view 页内嵌 pdf.js 直接渲染（不依赖系统 PDF 阅读器）；
 * - 小程序端：web-view 需业务域名白名单，暂用下载 + 微信内置 openDocument。
 */
export function openReportPdf(id: string) {
  // #ifdef APP-PLUS
  uni.showLoading({ title: '正在加载报告…', mask: true })
  apiReportPdfTicket(id)
    .then((ticket) => {
      uni.hideLoading()
      const file = '/api/public/report-pdf/' + encodeURIComponent(id) + '?ticket=' + encodeURIComponent(ticket)
      const viewer = getPublicOrigin() + '/pdfjs/viewer.html?file=' + encodeURIComponent(file)
      // file 绝对地址传给预览页：导航栏「打开/分享」直接下载后用系统面板调起 WPS/QQ/微信
      const fileUrl = getPublicOrigin() + file
      uni.navigateTo({
        url: '/pages/reports/pdf?src=' + encodeURIComponent(viewer) + '&file=' + encodeURIComponent(fileUrl)
      })
    })
    .catch((e: Error) => {
      uni.hideLoading()
      uni.showToast({ title: e.message, icon: 'none' })
    })
  // #endif
  // #ifndef APP-PLUS
  const token = getAccessToken()
  uni.showLoading({ title: '正在加载报告…', mask: true })
  uni.downloadFile({
    url: getBaseUrl() + '/reports/' + encodeURIComponent(id) + '/pdf',
    header: { Authorization: 'Bearer ' + token },
    success: (res) => {
      uni.hideLoading()
      if (res.statusCode != 200) {
        uni.showToast({ title: '报告加载失败（' + res.statusCode + '）', icon: 'none' })
        return
      }
      uni.openDocument({
        filePath: res.tempFilePath,
        fileType: 'pdf',
        showMenu: true, // 右上角菜单：可转发/保存
        fail: (e) => {
          uni.showToast({ title: '打开失败：' + (e.errMsg || '请安装 PDF 阅读器'), icon: 'none' })
        }
      })
    },
    fail: (e) => {
      uni.hideLoading()
      uni.showToast({ title: '下载失败：' + (e.errMsg || ''), icon: 'none' })
    }
  })
  // #endif
}

// ---- 消息 / 公告接口 ---------------------------------------------------------------

/** 消息列表 GET /messages（type/is_read 可选过滤；返回含 unread_count） */
export function apiMessages(page: number, pageSize: number, type?: string, isRead?: string): Promise<MessagesResult> {
  let path = '/messages?page=' + page + '&page_size=' + pageSize
  if (type != null && type != '') path += '&type=' + type
  if (isRead != null && isRead != '') path += '&is_read=' + isRead
  return new Promise<MessagesResult>((resolve, reject) => {
    httpGet<any>(path)
      .then((d) => {
        resolve({
          unread_count: d?.unread_count ?? 0,
          list: (d?.list ?? []) as MessageItem[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 标记已读 PUT /messages/:id/read（id=0 全部已读） */
export function apiMarkMessageRead(id: number | string): Promise<null> {
  return httpPut<null>('/messages/' + id + '/read', null, true)
}

/** 公告列表 GET /announcements（已发布公告分页） */
export function apiAnnouncements(page: number, pageSize: number): Promise<AnnouncementsPage> {
  return new Promise<AnnouncementsPage>((resolve, reject) => {
    httpGet<any>('/announcements?page=' + page + '&page_size=' + pageSize)
      .then((d) => {
        resolve({
          list: (d?.list ?? []) as AnnouncementItem[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 公告详情 GET /announcements/:id（仅已发布可见，404 由信封错误上抛） */
export function apiAnnouncementDetail(id: string): Promise<AnnouncementDetail> {
  return new Promise<AnnouncementDetail>((resolve, reject) => {
    httpGet<any>('/announcements/' + encodeURIComponent(id))
      .then((d) => {
        if (d == null) {
          reject(new Error('公告详情响应异常'))
          return
        }
        resolve({
          id: d.id ?? '',
          title: d.title ?? '',
          content: d.content ?? '',
          status: d.status ?? 0,
          attachments: (d.attachments ?? []) as NoticeAttachment[],
          publish_at: d.publish_at ?? '',
          created_by: d.created_by ?? '',
          created_at: d.created_at ?? ''
        })
      })
      .catch(reject)
  })
}

// =====================================================================================
// 管理端接口（App 管理功能：复用 PC 控制器，路由挂在 /api/app 下，见 router.go「管理功能」段）
// =====================================================================================

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

/** 任务监控明细 GET /inspection/tasks/:id/detail（点位逐个打卡状态；points 分页，默认每页 50） */
export function apiTaskMonitorDetail(id: string, page = 1, pageSize = 50): Promise<MonitorTaskDetail> {
  return new Promise<MonitorTaskDetail>((resolve, reject) => {
    httpGet<any>('/inspection/tasks/' + id + '/detail?points_page=' + page + '&points_page_size=' + pageSize)
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
  check_items: Array<{
    name: string
    pass: boolean
    note: string
    photo_urls: string[]
    requirement: string | null
    ai_verdict: string | null
    ai_reason: string | null
  }>
  audit_status: string
  ai_verdict: string
  ai_reason: string
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
  check_items: Array<{ name: string; pass: boolean; note: string; photos: string[]; photo_urls?: string[]; requirement: string | null }>
  /** pending/passed/rejected */
  audit_status: string
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
  template_id: string | null
  template_name: string
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
  template_id?: string | null
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
  let path = '/inspection/tasks?page=' + page + '&page_size=' + pageSize
  if (filter != '') path += '&filter=' + filter
  if (taskDate != null && taskDate != '') path += '&task_date=' + taskDate
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

/** 任务催办 POST /inspection/tasks/:id/remind（已完成任务后端报错，直接 toast message） */
export function apiTaskRemind(id: string): Promise<null> {
  return httpPost<null>('/inspection/tasks/' + id + '/remind', null, true)
}

/** 打卡审核记录 GET /inspection/review/records */
export function apiReviewRecords(page: number, pageSize: number, auditStatus: string): Promise<ReviewRecordsPage> {
  let path = '/inspection/review/records?page=' + page + '&page_size=' + pageSize
  if (auditStatus != '') path += '&audit_status=' + auditStatus
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

/** 点位列表 GET /inspection/points（支持类型/凭证/楼栋过滤） */
export function apiPointList(
  page: number,
  pageSize: number,
  communityId: string,
  name: string,
  tenantId = '',
  opts: { type?: string; credential?: string; buildingId?: string } = {}
): Promise<PointsPage> {
  let path = '/inspection/points?page=' + page + '&page_size=' + pageSize
  if (tenantId != '') path += '&tenant_id=' + encodeURIComponent(tenantId)
  if (communityId != '') path += '&community_id=' + communityId
  if (name != '') path += '&name=' + encodeURIComponent(name)
  if (opts.type != null && opts.type != '') path += '&type=' + encodeURIComponent(opts.type)
  if (opts.credential != null && opts.credential != '') path += '&credential=' + encodeURIComponent(opts.credential)
  if (opts.buildingId != null && opts.buildingId != '') path += '&building_id=' + opts.buildingId
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
    httpGet<any>('/inspection/templates?page=1&page_size=100')
      .then((d) => {
        resolve((d?.list ?? []) as TemplateListItem[])
      })
      .catch(reject)
  })
}

/** 业务字典选项 GET /dict-options?type_code=（登录即可；报告类型下拉等） */
export type DictOption = { label: string; value: string; sort: number }

export function apiDictOptions(typeCode: string): Promise<DictOption[]> {
  return new Promise<DictOption[]>((resolve, reject) => {
    httpGet<DictOption[]>('/dict-options?type_code=' + encodeURIComponent(typeCode))
      .then((d) => resolve(d ?? []))
      .catch(reject)
  })
}

/** App 管理端手动生成报告 POST /reports/generate（须 report:generate 权限；签字人走槽位自动圈选） */
export function apiReportGenerate(body: {
  community_id: string
  period: string
  patrol_type?: string
  detail_mode?: string
}): Promise<{ id: string; title: string; status: string; regenerated: boolean }> {
  return new Promise((resolve, reject) => {
    httpPost<{ id: string; title: string; status: string; regenerated: boolean }>('/reports/generate', body as unknown as Record<string, any>)
      .then((d) => {
        if (d == null) {
          reject(new Error('生成响应异常'))
          return
        }
        resolve(d)
      })
      .catch(reject)
  })
}
