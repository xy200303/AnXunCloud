// 业务接口类型定义（接口文档 §2.10~§2.16，字段蛇形与后端一致）
// 后端已迁移 UUIDv7 主键：凡实体 ID 字段一律 string

// ===== 小区/楼栋 =====
export interface CommunityItem {
  id: string
  name: string
  address: string
  manager_id: string | null
  manager_name: string
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
  unit_no?: number | null
  floor?: number | null
  name: string
  type: string
  type_label?: string
  qrcode_no: string
  longitude: number
  latitude: number
  fence_radius: number
  credential: 'qrcode' | 'nfc' | 'none'
  require_fence: boolean
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
  unit_no?: number | null
  floor?: number | null
  name: string
  type: string
  longitude: number | null
  latitude: number | null
  fence_radius: number
  credential: string
  require_fence: boolean
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

// 点位分配方式：all 每个执行日全量 / split 总量按 执行日数×巡检员数 均分（仅 weekly/monthly）
export type PlanAssignMode = 'all' | 'split'

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
  assign_mode?: PlanAssignMode
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
  assign_mode?: PlanAssignMode
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
  file_key?: string
  url: string
  watermarked_url?: string | null
  exif_time?: string
  exif_check?: { shot_at: string | null; deviation_seconds: number | null; passed: boolean | null } | null
}

export interface TaskPointDetail {
  point_id: string
  point_name: string
  building_name: string
  sort: number
  credential: 'qrcode' | 'nfc' | 'none'
  require_fence: boolean
  status: 'done' | 'doing' | 'pending'
  /** 项级进度（观测层，从过程草稿派生）：item_total=模板项数；doing 时带 done/recognizing/failed */
  item_total?: number
  item_done?: number
  item_recognizing?: number
  item_failed?: number
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
  } | null
}

export interface TaskDetailStats {
  total: number
  done: number
  doing: number
  pending: number
  normal: number
  abnormal: number
  suspect: number
}

export interface TaskDetail {
  task: TaskItem
  points: TaskPointDetail[]
  /** 点位时间线分页（大规模任务单任务可达数百点位） */
  points_total: number
  points_page: number
  points_size: number
  /** 全量状态聚合（汇总条，不随分页变化） */
  stats: TaskDetailStats
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
  // 强制提交（重拍放行后转人工复核）；记录列表接口暂不下发该字段，仅在详情/后端补齐后展示
  force_submit?: boolean
  // 同步判定照片质量结论：null=未校验
  ai_quality_pass?: boolean | null
  ai_quality_issue?: string
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
  // 表计读数类检查项的 AI 读数（其余类型为空）
  ai_reading?: string | null
  // 打卡当时的判定类型快照
  judge_type?: string
  judge_config?: Record<string, unknown> | null
}

export interface CheckinDetail extends CheckinItem {
  plan_name: string
  client_time: string
  longitude: number
  latitude: number
  remark: string
  suspect_reason: string
  photos: CheckinPhoto[]
  audit_by_name?: string
  created_at: string
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
