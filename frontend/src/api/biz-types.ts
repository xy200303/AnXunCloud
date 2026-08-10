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
  checkin_mode: 'qrcode' | 'fence' | 'either' | 'both' | 'nfc'
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
  checkin_mode: string
  required_photo_items: string[]
  template_id: string | null
  nfc_id: string
  sort: number
  status: number
}

// ===== 巡检计划 =====
export interface PlanItem {
  id: string
  community_id: string
  community_name: string
  name: string
  point_count: number
  cycle_type: 'daily' | 'weekly' | 'monthly'
  cycle_config: { weekdays?: number[]; days?: number[] }
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
  point_ids: string[]
  cycle_type: string
  cycle_config: { weekdays?: number[]; days?: number[] }
  inspector_ids: string[]
  start_date: string
  end_date: string
  time_window: string
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
  checkin_mode: string
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
  created_at: string
}

// ===== 异常工单 =====
export interface WorkOrderItem {
  id: string
  order_no: string
  title: string
  community_id: string
  community_name: string
  point_id: string | null
  point_name: string
  priority: 'low' | 'normal' | 'high' | 'urgent'
  status: 'pending' | 'assigned' | 'processing' | 'review' | 'closed' | 'rejected'
  reporter_id: string
  reporter_name: string
  assignee_id: string | null
  assignee_name: string | null
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
  fix_photos: { url: string; watermarked_url: string | null }[]
  fix_remark: string
  finished_at: string | null
  reviewed_by: string | null
  review_remark: string | null
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

// ===== 通知公告 =====
export interface NoticeItem {
  id: string
  title: string
  content: string
  status: number // 0 草稿 / 1 已发布 / 2 已下线
  publish_at: string | null
  created_by: string
  created_at: string
}
