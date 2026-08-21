// 业务接口类型定义（接口文档 §2.10~§2.16，字段蛇形与后端一致）
// 后端已迁移 UUIDv7 主键：凡实体 ID 字段一律 string

// ===== 小区/楼栋 =====
export interface CommunityItem {
  id: string
  name: string
  address: string
  manager_id: string | null
  manager_name: string
  // 工单分诊开关：关闭后上报直接进待派单（跳过待分诊）
  wo_triage_enabled: boolean
  // 工单抢单开关：开启后维修工可从工单池抢单
  wo_grab_enabled: boolean
  building_count: number
  point_count: number
  status: number
  created_at: string
}

// ===== 项目岗位编制 / 职责槽位绑定（设计方案 §3.2；名单制授权） =====
export interface PostDictItem {
  id: string
  code: string
  name: string
  line: string // 业务线（safety/engineering/environment/service/general）
  is_supervisor: boolean
  status: number // 1 启用 / 0 停用（sysmodel.StatusInt）
  remark: string
}

export interface StaffItem {
  id: string
  project_id: string
  user_id: string
  user_name: string
  phone: string
  avatar: string
  posts: string[] // post_dict.code
  post_names: string[]
  building_ids: string[]
  building_names: string[]
  status: number
  created_at: string
}

export interface StaffForm {
  user_id: string
  posts: string[]
  building_ids?: string[]
  status?: number
}

// 职责槽位绑定视图：source=platform 平台默认 / project 项目级覆盖
export interface DutyBindingItem {
  slot: string
  name: string
  post_codes: string[]
  post_names: string[]
  source: 'platform' | 'project'
}

// 楼管员岗位 code（选此岗位时需圈定责任楼栋）
export const POST_BUILDING_MANAGER = 'building_manager'

export interface BuildingItem {
  id: string
  community_id: string
  community_name?: string
  name: string
  type: 'building' | 'area'
  point_count: number
  created_at: string
}

// ===== 检查项模板 =====
export type PhotoRequired = 'none' | 'optional' | 'required'

export interface TemplateCheckItem {
  name: string
  required: boolean
  // 检查标准要求文本（可选）
  requirement?: string
  // 拍照要求：none 无需 / optional 选拍 / required 必拍（缺省 none）
  photo_required?: PhotoRequired
}

export interface TemplateItem {
  id: string
  name: string
  point_type: string // 空表示通用（所有类型）
  items: TemplateCheckItem[]
  sort: number
  status: number
  remark: string
  created_at: string
}

export interface TemplateForm {
  name: string
  point_type: string
  sort: number
  status: number
  remark: string
}

// ===== 点位 =====
export interface PointItem {
  id: string
  community_id: string
  community_name: string
  building_id: string
  building_name: string
  name: string
  type: string
  type_label?: string
  qrcode_no: string
  longitude: number
  latitude: number
  fence_radius: number
  credential: 'qrcode' | 'nfc' | 'none'
  require_fence: boolean
  required_photo_items: string[]
  template_id: string | null
  template_name?: string
  nfc_id?: string
  sort: number
  status: number
  created_at: string
}

export interface PointForm {
  community_id: string | null
  building_id: string | null
  name: string
  type: string
  longitude: number | null
  latitude: number | null
  fence_radius: number
  credential: string
  require_fence: boolean
  required_photo_items: string[]
  template_id: string | null
  nfc_id: string
  sort: number
  status: number
}

// ===== 巡检计划 =====
// 巡查类型（字典 patrol_type 驱动，attrs.category 分大类；任务生成时从计划快照）
export type PatrolType = string

// 兼容旧页面的硬编码兜底选项（新入口一律走 usePatrolTypes 字典）
export const PATROL_TYPE_OPTIONS: { value: string; label: string }[] = [
  { value: 'safety', label: '安全巡查' },
  { value: 'equipment', label: '设备设施专项' },
  { value: 'environment', label: '环境巡查' },
  { value: 'building', label: '楼栋巡查' }
]

export function patrolTypeLabel(t: string) {
  return PATROL_TYPE_OPTIONS.find((o) => o.value === t)?.label || t || '安全巡查'
}

// 巡更轮次（cycle_config.rounds，仅 daily/weekly；window 允许跨零点，起止相等非法）
export interface PlanRound {
  name: string
  window: string // HH:MM-HH:MM
}

export interface PlanCycleConfig {
  interval?: number
  weekdays?: number[]
  days?: number[]
  rounds?: PlanRound[]
  daily_min_rounds?: number | null
}

// 选点方式：explicit 手动名单 / by_point_types 按点位类型圈选（任务生成时实时展开）
export type PlanSelectionMode = 'explicit' | 'by_point_types'

export interface PlanItem {
  id: string
  community_id: string
  community_name: string
  name: string
  patrol_type: PatrolType
  point_count: number
  selection_mode?: PlanSelectionMode
  point_types?: string[] | null
  cycle_type: 'daily' | 'weekly' | 'monthly'
  cycle_config: PlanCycleConfig
  inspector_ids: string[]
  inspector_names: string[]
  start_date: string
  end_date: string
  time_window: string
  status: number
  created_at: string
}

export interface PlanDetail extends Omit<PlanItem, 'point_count'> {
  points: { id: string; name: string; building_name: string; sort: number }[]
}

export interface PlanForm {
  community_id: string | null
  name: string
  patrol_type: PatrolType
  selection_mode?: PlanSelectionMode
  point_ids?: string[]
  point_types?: string[]
  cycle_type: string
  cycle_config: PlanCycleConfig
  inspector_ids: string[]
  start_date: string
  end_date: string
  time_window?: string
  status: number
}

// ===== 任务监控 =====
export interface TaskItem {
  id: string
  plan_id: string
  plan_name: string
  community_id: string
  community_name: string
  inspector_id: string
  inspector_name: string
  patrol_type: PatrolType
  task_date: string
  time_window: string
  status: 'pending' | 'doing' | 'done' | 'overdue'
  total_points: number
  done_points: number
  progress: number
  abnormal_count: number
  suspect_count: number
  missing_count: number
  started_at: string | null
  finished_at: string | null
}

export interface CheckinPhoto {
  item: string
  url: string
  watermarked_url: string | null
  required?: boolean
  exif_check?: { shot_at: string | null; deviation_seconds: number | null; passed: boolean | null } | null
}

export interface TaskPointDetail {
  point_id: string
  point_name: string
  building_name: string
  sort: number
  credential: 'qrcode' | 'nfc' | 'none'
  require_fence: boolean
  status: 'done' | 'pending'
  checkin: {
    id: string
    checkin_time: string
    client_time: string
    checkin_type: 'qrcode' | 'fence' | 'offline'
    distance_to_point: number
    longitude: number
    latitude: number
    result: 'normal' | 'abnormal'
    is_suspect: boolean
    suspect_reason: string
    remark: string
    photos: CheckinPhoto[]
    work_order_no?: string | null
  } | null
}

export interface TaskDetail {
  task: TaskItem
  points: TaskPointDetail[]
}

// ===== 打卡记录 =====
export interface CheckinItem {
  id: string
  task_id: string
  point_id: string
  point_name: string
  community_name: string
  inspector_id: string
  inspector_name: string
  checkin_time: string
  checkin_type: 'qrcode' | 'fence' | 'offline' | 'nfc'
  distance_to_point: number
  result: 'normal' | 'abnormal'
  is_suspect: boolean
  photo_count: number
  // 记录审核（v10）
  audit_status: 'auto_pass' | 'pending' | 'pass' | 'rejected'
  // 审批链（扩展方案 §3）：已通过环节数与待审核时的当前环节名（列表"待<环节名>"展示用）
  audit_step?: number
  current_step_name?: string
  audit_by?: string
  audit_at?: string
  audit_remark?: string
  ai_verdict?: 'pass' | 'review' | 'error' | ''
  ai_reason?: string
  check_items?: CheckinCheckItem[]
}

export interface CheckinCheckItem {
  name: string
  pass: boolean
  note: string
  // 打卡当时的检查标准快照（可选，后端 v18 起返回）
  requirement?: string
  // 该项照片 URL 列表（可空，后端并行开发中）
  photo_urls?: string[]
  // 逐项 AI 初判（仅管理端展示）：ai_hint 识别要点快照，ai_verdict pass/review/error，空=未送 AI
  ai_hint?: string
  ai_verdict?: 'pass' | 'review' | 'error' | ''
  ai_reason?: string
}

export interface CheckinDetail extends CheckinItem {
  plan_name: string
  client_time: string
  longitude: number
  latitude: number
  remark: string
  suspect_reason: string
  photos: CheckinPhoto[]
  work_order_no?: string | null
  audit_by_name?: string
  created_at: string
}

// ===== 异常工单（P2 闭环状态机：reported → pending_dispatch → processing → pending_confirm → closed；closed_invalid 已作废） =====
export type WorkOrderStatus =
  | 'reported' // 待分诊
  | 'pending_dispatch' // 待派单
  | 'processing' // 处理中
  | 'pending_confirm' // 待验收
  | 'closed' // 已闭环
  | 'closed_invalid' // 已作废（分诊驳回）

// 工单来源（字典 order_source）：inspection 巡检异常转单 / active 主动上报 / frontdesk 前台代录
export type WorkOrderSource = 'inspection' | 'active' | 'frontdesk'

export type WorkOrderPriority = 'low' | 'normal' | 'high' | 'urgent'

export interface WorkOrderItem {
  id: string
  order_no: string
  title: string
  community_id: string
  community_name: string
  point_id: string | null
  point_name: string
  source: WorkOrderSource
  category: string
  priority: WorkOrderPriority
  status: WorkOrderStatus
  reporter_id: string
  reporter_name: string
  assignee_id: string | null
  assignee_name: string | null
  // SLA 期望完成时间 / 是否超时（后端按优先级简化计算，仅展示）
  sla_deadline: string | null
  sla_overdue: boolean
  created_at: string
}

export interface WorkOrderLog {
  action: string
  operator_name: string
  detail: string
  created_at: string
}

// 工单不合格项快照（建单时从不合格检查项生成，整改回传按 name 合并 after_photo_urls）
export interface WorkOrderCheckItem {
  name: string
  remark: string
  before_photo_urls: string[]
  after_photo_urls: string[]
}

export interface WorkOrderDetail extends WorkOrderItem {
  checkin_id: string | null
  description: string
  photos: { url: string; watermarked_url: string | null }[]
  // 派单人（抢单工单为空）
  dispatcher_id: string | null
  dispatcher_name: string | null
  // 分诊
  triage_by: string | null
  triage_by_name: string | null
  triage_at: string | null
  triage_note: string | null
  // 派单/接单（派单即视为接单，两时间同时写入）
  dispatch_at: string | null
  accept_at: string | null
  // 完工
  finish_photos: { url: string; watermarked_url: string | null }[]
  finish_note: string | null
  finish_at: string | null
  // 验收
  confirm_by: string | null
  confirm_by_name: string | null
  confirm_at: string | null
  confirm_note: string | null
  // 最近一次驳回原因（分诊驳回 / 验收退回）
  reject_reason: string | null
  logs: WorkOrderLog[]
  // 不合格项快照（可空，旧工单无此字段）
  items?: WorkOrderCheckItem[]
}

export interface WorkOrderListResult {
  status_counts: Record<string, number>
  list: WorkOrderItem[]
  total: number
  page: number
  page_size: number
}

// ===== 统计报表 =====
export interface CoverageData {
  summary: { should_points: number; done_points: number; coverage_rate: number; abnormal_count: number; suspect_count: number }
  daily: { date: string; should_points: number; done_points: number; coverage_rate: number }[]
  by_community: { community_id: string; community_name: string; should_points: number; done_points: number; coverage_rate: number }[]
}

export interface TimelinessData {
  summary: { total_tasks: number; on_time_tasks: number; timeliness_rate: number; overdue_tasks: number }
  daily: { date: string; total_tasks: number; on_time_tasks: number; timeliness_rate: number }[]
  by_community: { community_id: string; community_name: string; total_tasks: number; on_time_tasks: number; timeliness_rate: number }[]
}

export interface PerformanceItem {
  inspector_id: string
  inspector_name: string
  community_names: string[]
  total_tasks: number
  done_tasks: number
  should_points: number
  done_points: number
  coverage_rate: number
  avg_duration_min: number
  abnormal_found: number
  suspect_count: number
}

// ===== 巡更达成率（接口文档 §2.23，以轮次任务为最小单元） =====
export interface PatrolRoundsSummary {
  should_rounds: number
  done_rounds: number
  open_rounds: number // 待开始/进行中
  overdue_rounds: number
  achievement_rate: number
  avg_point_completion: number // 单轮点位完成率
  daily_min_rounds: number | null // 每日达标轮次线（未设为 null）
}

export interface PatrolRoundsDaily {
  date: string
  should_rounds: number
  done_rounds: number
  overdue_rounds: number
  achievement_rate: number
  met: boolean | null // 达标线判定；未设达标线为 null
}

export interface PatrolRoundsOverdueItem {
  task_id: string
  task_date: string
  round_name: string
  time_window: string
  inspector_name: string
  done_points: number
  total_points: number
  // overdue 已翻转逾期 / expired_doing 窗口已过未翻转（动态判定）
  state: 'overdue' | 'expired_doing'
}

export interface PatrolRoundsData {
  summary: PatrolRoundsSummary
  daily: PatrolRoundsDaily[]
  overdue_list: PatrolRoundsOverdueItem[]
}

// ===== 通知公告 =====
export interface NoticeAttachment {
  name: string
  url: string
}

export interface NoticeItem {
  id: string
  title: string
  content: string
  status: number // 0 草稿 / 1 已发布 / 2 已下线
  attachments: NoticeAttachment[]
  publish_at: string | null
  created_by: string
  created_at: string
}
