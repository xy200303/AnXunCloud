/**
 * 业务 API 层——与后端 /api/app（小程序 /api/mp）接口一一对应，类型对齐响应结构。
 * 参照 docs/接口文档.md §1.3 信封、§3 移动端接口；/api/app 组与 /api/mp 同形（技术方案 §4）。
 */

import { httpGet, httpPost, httpPut, refreshSession, getBaseUrl, getPublicOrigin } from '@/services/request'
import { getAccessToken } from '@/utils/storage'

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
  /** 角色 code 数组，如 ['inspector'] / ['repair'] */
  roles: string[]
  community_ids: string[]
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
  task_date: string
  time_window: string
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
  /** 点位级必拍项（整单照片 item 维度） */
  required_photo_items: string[]
  check_items: CheckItemTpl[]
  my_checkin: {
    id: string
    checkin_time: string
    checkin_type: string
    distance_to_point: number | null
    /** normal/abnormal */
    result: string
    is_suspect: boolean
  } | null
}

export type TaskDetail = {
  id: string
  plan_name: string
  community_name: string
  task_date: string
  time_window: string
  status: string
  total_points: number
  done_points: number
  progress: number
  points: TaskPoint[]
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
  /** YYYY-MM-DD HH:mm:ss（timefmt.Layout） */
  client_time: string
  result: 'normal' | 'abnormal'
  remark: string
  check_items: Array<{ name: string; pass: boolean; note: string; photos: string[] }>
  photos: Array<{ item: string; file_key: string }>
}

/** 打卡响应（对齐后端 resultView） */
export type CheckinResult = {
  checkin_id: string
  checkin_time: string
  distance_to_point: number
  is_suspect: boolean
  suspect_reason: string
  /** 异常打卡自动生成的工单（{id, order_no}），无则 null */
  work_order: any
  task_progress: {
    total_points: number
    done_points: number
    progress: number
    task_status: string
  }
}

// ---- 工单（/workorders，对齐后端 MyOrders / GetForMP / Accept / Finish） ----------

/** 我的工单列表项（对齐 MPService.MyOrders 返回） */
export type MyOrderItem = {
  id: string
  order_no: string
  title: string
  community_name: string
  point_name: string
  /** low/normal/high/urgent */
  priority: string
  /** pending/assigned/processing/review/closed/rejected */
  status: string
  /** reporter=我上报的 / assignee=指派给我 */
  my_role: string
  review_remark: string
  created_at: string
}

export type MyOrdersPage = {
  list: MyOrderItem[]
  total: number
  page: number
  page_size: number
}

/** 工单照片元素（后端 types.PhotoItem） */
export type OrderPhoto = {
  item: string
  url: string
  watermarked_url: string
}

/** 工单详情（对齐 OrderService.detailOf / GetForMP） */
export type WorkOrderDetail = {
  id: string
  order_no: string
  checkin_id: string | null
  title: string
  community_id: string
  community_name: string
  point_id: string | null
  point_name: string
  description: string
  /** 上报时照片（before） */
  photos: OrderPhoto[]
  /** 异常项快照 */
  items: Array<{
    name: string
    remark: string
    before_photo_urls: string[]
    after_photo_urls: string[]
  }>
  reporter_id: string
  reporter_name: string
  assignee_id: string | null
  assignee_name: string
  priority: string
  status: string
  fix_photos: OrderPhoto[]
  fix_remark: string
  finished_at: string
  reviewed_by: string | null
  review_remark: string | null
  created_at: string
  logs: Array<{
    action: string
    operator_name: string
    detail: string
    created_at: string
  }>
}

/** 完工反馈请求（对齐 wodto.FinishReq，维修照片至少 1 张 file_key） */
export type FinishReqPayload = {
  fix_remark: string
  fix_photos: Array<{ file_key: string }>
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
  file_key: string
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
  signature_file_key?: string
}

/** 巡检员确认请求（对齐 dto.InspectorSignReq：proxy_for 非空为代签，reason 必填） */
export type InspectorSignReqPayload = {
  proxy_for?: string
  reason?: string
  signature_file_key?: string
}

// ---- 消息 / 公告（对齐 MPService.Messages / NoticeService.Published） -----------------

/** 消息类型：workorder 工单 / report 月报 / checkin_audit 打卡审核 / 其他按系统消息展示 */
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
  community_ids?: Array<string | number>
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
  task_date?: string
  time_window?: string
  status?: string
  total_points?: number
  done_points?: number
  progress?: number
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
    required_photo_items?: string[]
    check_items?: Array<{ name?: string; requirement?: string; photo_required?: string }>
    my_checkin?: {
      id?: string | number
      checkin_time?: string
      checkin_type?: string
      distance_to_point?: number | null
      result?: string
      is_suspect?: boolean
    } | null
  }>
}

type RawCheckinResult = {
  checkin_id?: string | number
  checkin_time?: string
  distance_to_point?: number
  is_suspect?: boolean
  suspect_reason?: string
  work_order?: any
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
  data?: { file_key?: string; url?: string } | null
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
    community_ids: (raw.community_ids ?? []).map((c) => String(c))
  }
}

/** 权限点判断（超管以 super_admin 角色兜底，与后端通配策略同口径；旧缓存用户缺 perms 字段时容错） */
export function hasPerm(u: UserInfo | null, perm: string): boolean {
  if (u == null) return false
  if ((u.roles ?? []).indexOf('super_admin') >= 0) return true
  return (u.perms ?? []).indexOf(perm) >= 0
}

// ---- 认证 ------------------------------------------------------------------------

/** 登录 POST /login {username, password} */
export function apiLogin(username: string, password: string): Promise<LoginResult> {
  return new Promise<LoginResult>((resolve, reject) => {
    httpPost<RawLoginData>('/login', { username: username, password: password }, false)
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

/** 刷新 POST /refresh（静默刷新由 request 层 refreshSession 完成，此处为对外的同能力入口） */
export function apiRefresh(): Promise<boolean> {
  return refreshSession()
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

/** 注册 POST /auth/register {username,password,name,phone} */
export function apiRegister(username: string, password: string, name: string, phone: string): Promise<null> {
  return httpPost<null>('/auth/register', {
    username: username,
    password: password,
    name: name,
    phone: phone
  }, false)
}

/** 登出 POST /auth/logout */
export function apiLogout(): Promise<null> {
  return httpPost<null>('/auth/logout', null, true)
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
 * signatureFileKey 不传 = 不改动签名；传入则创建/替换当前用户 active 签名资产（下次签字直接用）。
 * avatarFileKey 不传 = 不改动头像；传入则更新头像（存 file_key，展示拼 /uploads/）。
 */
export function apiUpdateProfile(name: string, phone: string, signatureFileKey?: string, avatarFileKey?: string): Promise<UserInfo> {
  const body: Record<string, any> = { name: name, phone: phone }
  if (signatureFileKey != null) body.signature_file_key = signatureFileKey
  if (avatarFileKey != null) body.avatar = avatarFileKey
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
  return new Promise<TodayTasks>((resolve, reject) => {
    httpGet<TodayTasks>('/tasks/today')
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
          task_date: d.task_date ?? '',
          time_window: d.time_window ?? '',
          status: d.status ?? '',
          total_points: d.total_points ?? 0,
          done_points: d.done_points ?? 0,
          progress: d.progress ?? 0,
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
            required_photo_items: p.required_photo_items ?? [],
            check_items: (p.check_items ?? []).map((c) => ({
              name: c.name ?? '',
              requirement: c.requirement ?? '',
              photo_required: c.photo_required ?? ''
            })),
            my_checkin: p.my_checkin == null
              ? null
              : {
                  id: toId(p.my_checkin.id),
                  checkin_time: p.my_checkin.checkin_time ?? '',
                  checkin_type: p.my_checkin.checkin_type ?? '',
                  distance_to_point: p.my_checkin.distance_to_point ?? null,
                  result: p.my_checkin.result ?? '',
                  is_suspect: p.my_checkin.is_suspect ?? false
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
          work_order: d.work_order ?? null,
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

/**
 * 本地（dev）上传 POST /upload/local（multipart）。
 * scene：checkin（打卡）/ workorder（工单维修照片）/ avatar / signature（月报一次性手写签名），默认 checkin，原有调用不受影响。
 * 注意：uni.uploadFile 不走 request.ts 的信封/40102 静默刷新逻辑，token 过期会直接失败，需重新登录后重试。
 */
export function apiUploadLocal(
  filePath: string,
  scene: 'checkin' | 'workorder' | 'avatar' | 'signature' = 'checkin'
): Promise<{ file_key: string; url: string }> {
  return new Promise((resolve, reject) => {
    uni.uploadFile({
      url: getBaseUrl() + '/upload/local',
      filePath: filePath,
      name: 'file',
      formData: { scene: scene },
      header: { Authorization: 'Bearer ' + getAccessToken() },
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
          reject(new Error(env.message || '上传失败'))
          return
        }
        resolve({
          file_key: env.data?.file_key ?? '',
          url: env.data?.url ?? ''
        })
      },
      fail: () => {
        reject(new Error('网络异常，照片上传失败'))
      }
    })
  })
}

// ---- 工单接口 --------------------------------------------------------------------

/** 我的工单 GET /workorders/mine（type: ''=全部 / reported=我上报的 / assigned=指派给我；status 支持逗号多值） */
export function apiMyOrders(
  page: number,
  pageSize: number,
  type: string,
  status: string
): Promise<MyOrdersPage> {
  let path = '/workorders/mine?page=' + page + '&page_size=' + pageSize
  if (type != '') path += '&type=' + type
  if (status != '') path += '&status=' + status
  return new Promise<MyOrdersPage>((resolve, reject) => {
    httpGet<any>(path)
      .then((d) => {
        resolve({
          list: (d?.list ?? []) as MyOrderItem[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 我的工单按状态计数 GET /workorders/mine/counts?type= → {status: count}（chip 角标/红点用） */
export function apiMyOrderCounts(type: string): Promise<Record<string, number>> {
  let path = '/workorders/mine/counts'
  if (type != '') path += '?type=' + type
  return new Promise<Record<string, number>>((resolve, reject) => {
    httpGet<Record<string, number>>(path)
      .then((d) => {
        resolve(d ?? {})
      })
      .catch(reject)
  })
}

/** 工单详情 GET /workorders/:id（仅上报人/处理人可见） */
export function apiOrderDetail(id: string): Promise<WorkOrderDetail> {
  return new Promise<WorkOrderDetail>((resolve, reject) => {
    httpGet<any>('/workorders/' + id)
      .then((d) => {
        if (d == null) {
          reject(new Error('工单详情响应异常'))
          return
        }
        resolve({
          id: d.id ?? '',
          order_no: d.order_no ?? '',
          checkin_id: d.checkin_id ?? null,
          title: d.title ?? '',
          community_id: d.community_id ?? '',
          community_name: d.community_name ?? '',
          point_id: d.point_id ?? null,
          point_name: d.point_name ?? '',
          description: d.description ?? '',
          photos: (d.photos ?? []) as OrderPhoto[],
          items: (d.items ?? []).map((it: any) => ({
            name: it.name ?? '',
            remark: it.remark ?? '',
            before_photo_urls: it.before_photo_urls ?? [],
            after_photo_urls: it.after_photo_urls ?? []
          })),
          reporter_id: d.reporter_id ?? '',
          reporter_name: d.reporter_name ?? '',
          assignee_id: d.assignee_id ?? null,
          assignee_name: d.assignee_name ?? '',
          priority: d.priority ?? '',
          status: d.status ?? '',
          fix_photos: (d.fix_photos ?? []) as OrderPhoto[],
          fix_remark: d.fix_remark ?? '',
          finished_at: d.finished_at ?? '',
          reviewed_by: d.reviewed_by ?? null,
          review_remark: d.review_remark ?? null,
          created_at: d.created_at ?? '',
          logs: (d.logs ?? []).map((l: any) => ({
            action: l.action ?? '',
            operator_name: l.operator_name ?? '',
            detail: l.detail ?? '',
            created_at: l.created_at ?? ''
          }))
        })
      })
      .catch(reject)
  })
}

/** 接单 POST /workorders/:id/accept → {status:'processing'} */
export function apiAcceptOrder(id: string): Promise<null> {
  return httpPost<null>('/workorders/' + id + '/accept', null, true)
}

/** 完工反馈 POST /workorders/:id/finish → {status:'review'} */
export function apiFinishOrder(id: string, req: FinishReqPayload): Promise<null> {
  return httpPost<null>('/workorders/' + id + '/finish', req as unknown as Record<string, any>, true)
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
          file_key: d.file_key ?? '',
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
  pending_workorders: number
  trend_7d: Array<{ date: string; total: number; done: number; rate: number }>
  community_rank: Array<{ community_id: string; community_name: string; total: number; done: number; rate: number }>
  latest_workorders: Array<{
    id: string
    order_no: string
    title: string
    community_name: string
    priority: string
    status: string
    created_at: string
  }>
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

/** 任务监控明细（对齐 TaskService.Detail：{task, points} 结构，与巡检端 /tasks/:id 不同） */
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
  check_items: Array<{ name: string; pass: boolean; note: string; photos: string[]; requirement: string | null }>
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

// ---- 工单派单与验收（对齐 OrderController.List/Detail/Assign/Review） --------------------

/** 管理端工单列表项（对齐 OrderService.toItem） */
export type ManageOrderItem = {
  id: string
  order_no: string
  title: string
  community_id: string
  community_name: string
  point_id: string | null
  point_name: string
  priority: string
  status: string
  reporter_id: string
  reporter_name: string
  assignee_id: string | null
  assignee_name: string | null
  created_at: string
}

/** 管理端工单列表响应（status_counts 为四档聚合计数：processing 含 assigned+processing，closed 含 closed+rejected） */
export type ManageOrdersPage = {
  status_counts: Record<string, number>
  list: ManageOrderItem[]
  total: number
  page: number
  page_size: number
}

/** 派单候选用户（对齐 UserService.toItem；注意：roles 元素只有 {id, name}，无 code 字段） */
export type StaffUser = {
  id: string
  username: string
  name: string
  phone: string
  avatar: string
  roles: Array<{ id: string; name: string }>
  status: number
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
  required_photo_items: string[]
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
  required_photo_items?: string[]
  sort?: number
  status?: number
  remark?: string
}

/** 小区/楼栋树节点（对齐 dto.CommunityTreeNode） */
export type CommunityTreeNode = {
  id: string
  name: string
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
          pending_workorders: d.pending_workorders ?? 0,
          trend_7d: d.trend_7d ?? [],
          community_rank: d.community_rank ?? [],
          latest_workorders: d.latest_workorders ?? [],
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

/** 任务监控明细 GET /inspection/tasks/:id/detail（点位逐个打卡状态） */
export function apiTaskMonitorDetail(id: string): Promise<MonitorTaskDetail> {
  return new Promise<MonitorTaskDetail>((resolve, reject) => {
    httpGet<any>('/inspection/tasks/' + id + '/detail')
      .then((d) => {
        if (d == null || d.task == null) {
          reject(new Error('任务明细响应异常'))
          return
        }
        resolve({ task: d.task, points: (d.points ?? []) as MonitorTaskPoint[] })
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

/**
 * 管理端工单列表 GET /manage/workorders。
 * status 为 tab 档位：pending / processing（实际下传 assigned,processing）/ review / closed（closed,rejected），由调用方展开为多值。
 */
export function apiManageOrders(page: number, pageSize: number, status: string): Promise<ManageOrdersPage> {
  let path = '/manage/workorders?page=' + page + '&page_size=' + pageSize
  if (status != '') path += '&status=' + status
  return new Promise<ManageOrdersPage>((resolve, reject) => {
    httpGet<any>(path)
      .then((d) => {
        resolve({
          status_counts: (d?.status_counts ?? {}) as Record<string, number>,
          list: (d?.list ?? []) as ManageOrderItem[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 管理端工单详情 GET /manage/workorders/:id（结构同 WorkOrderDetail） */
export function apiManageOrderDetail(id: string): Promise<WorkOrderDetail> {
  return new Promise<WorkOrderDetail>((resolve, reject) => {
    httpGet<any>('/manage/workorders/' + id)
      .then((d) => {
        if (d == null) {
          reject(new Error('工单详情响应异常'))
          return
        }
        resolve(d as WorkOrderDetail)
      })
      .catch(reject)
  })
}

/** 派单 POST /manage/workorders/:id/assign {assignee_id, remark?} */
export function apiAssignOrder(id: string, assigneeId: string, remark: string): Promise<null> {
  return httpPost<null>('/manage/workorders/' + id + '/assign', { assignee_id: assigneeId, remark: remark }, true)
}

/** 验收 POST /manage/workorders/:id/review {result: pass|reject, review_remark}（reject 时 remark 后端强制必填） */
export function apiReviewOrder(id: string, result: 'pass' | 'reject', reviewRemark: string): Promise<null> {
  return httpPost<null>('/manage/workorders/' + id + '/review', { result: result, review_remark: reviewRemark }, true)
}

/** 派单选人 GET /system/users（启用用户；维修员候选由前端按角色过滤，见 StaffUser.roles 字段说明） */
export function apiOrderUsers(): Promise<StaffUser[]> {
  return new Promise<StaffUser[]>((resolve, reject) => {
    httpGet<any>('/system/users?page=1&page_size=100&status=1')
      .then((d) => {
        resolve((d?.list ?? []) as StaffUser[])
      })
      .catch(reject)
  })
}

/** 点位列表 GET /inspection/points */
export function apiPointList(page: number, pageSize: number, communityId: string, name: string): Promise<PointsPage> {
  let path = '/inspection/points?page=' + page + '&page_size=' + pageSize
  if (communityId != '') path += '&community_id=' + communityId
  if (name != '') path += '&name=' + encodeURIComponent(name)
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
