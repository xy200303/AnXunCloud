// 设备台账与维保周期管理接口（《设备台账与维保周期管理设计方案》）
import { request, type PageResult } from '@/utils/request'

// 到期状态：normal 正常 / warning 临期 / overdue 已逾期 / none 无到期日 / scrap 应报废 / label_missing 标签缺失（v1.7）
export type DueState = 'normal' | 'warning' | 'overdue' | 'none' | 'scrap' | 'label_missing'

// 设备状态：in_service 在用 / maintaining 维保中 / stopped 停用 / scrapped 报废
export type EquipmentStatus = 'in_service' | 'maintaining' | 'stopped' | 'scrapped'

// 维保类型（字典 equipment_maint_type）：repair 维修充粉 / maintain 保养 / inspect 检测 / replace 更换 / ledger_fix 台账补录
export type MaintenanceType = 'repair' | 'maintain' | 'inspect' | 'replace' | 'ledger_fix'

// 确认状态：pending 待确认 / confirmed 已确认 / rejected 已驳回
export type ConfirmStatus = 'pending' | 'confirmed' | 'rejected'

export interface EquipmentItem {
  id: string
  community_id: string
  community_name: string
  building_id: string | null
  building_name?: string
  point_id: string | null
  point_name?: string
  type: string
  type_label: string
  code: string
  name: string
  manufacture_date: string
  last_maintenance_date: string
  next_due_date: string
  scrap_date: string
  due_state: DueState
  label_missing: boolean
  warn_days: number | null
  status: EquipmentStatus
  status_label: string
  extra: Record<string, string>
  remark: string
  created_at: string
}

export interface EquipmentDetail extends EquipmentItem {
  updated_at: string
  last_notified_at: string
  type_rule?: {
    first_months: number
    cycle_months: number
    remind: boolean
    scrap_months: number
  }
}

export interface EquipmentQuery {
  page?: number
  page_size?: number
  type?: string
  community_id?: string
  point_id?: string
  status?: EquipmentStatus | ''
  due_state?: DueState | ''
  keyword?: string
}

export interface EquipmentForm {
  community_id: string
  building_id?: string | null
  point_id?: string | null
  type: string
  code: string
  name: string
  manufacture_date?: string
  next_due_date?: string // 留空：新增按类型规则计算，修改保持原值
  warn_days?: number | null
  status?: EquipmentStatus
  extra?: Record<string, string>
  remark?: string
}

// 新增/编辑响应：warning 为点位类型软校验提示（非错误）
export interface EquipmentSaveResult {
  id: string
  warning: string
}

export function listEquipment(params: EquipmentQuery) {
  return request<PageResult<EquipmentItem>>({ url: '/equipment/list', method: 'get', params })
}

export function getEquipment(id: string) {
  return request<EquipmentDetail>({ url: `/equipment/${id}`, method: 'get' })
}

export function createEquipment(data: EquipmentForm) {
  return request<EquipmentSaveResult>({ url: '/equipment', method: 'post', data })
}

export function updateEquipment(id: string, data: EquipmentForm) {
  return request<EquipmentSaveResult>({ url: `/equipment/${id}`, method: 'put', data })
}

export function deleteEquipment(id: string) {
  return request<null>({ url: `/equipment/${id}`, method: 'delete' })
}

// ===== 维保登记 / 确认链 =====

export interface MaintenancePhoto {
  file_id: string
  url: string
}

export interface MaintenanceItem {
  id: string
  equipment_id: string
  equipment_code: string
  equipment_name: string
  point_id?: string
  point_name?: string
  maintenance_type: MaintenanceType
  maintenance_date: string
  vendor: string | null
  operator_name: string
  note: string
  photos: MaintenancePhoto[]
  confirm_status: ConfirmStatus
  label_missing: boolean
  reject_reason: string | null
  ai_verdict: 'pass' | 'review' | null
  ai_reason: string | null
  created_by: string
  created_by_name: string
  created_at: string
  confirmed_by?: string
  confirmed_by_name?: string
  confirmed_at?: string
  // 台账补录（ledger_fix）随单提交的补录日期，确认后回写台账
  fix_manufacture_date?: string
  fix_last_maintenance_date?: string
}

export interface MaintenanceRegisterForm {
  equipment_id: string
  maintenance_type?: MaintenanceType
  maintenance_date?: string // YYYY-MM-DD，缺省今天
  vendor?: string
  operator_name?: string // 缺省当前用户
  note?: string
  file_ids: string[] // 新维修标签照片（必传至少 1 张；标签缺失时拍设备本体作证）
  label_missing?: boolean // 标签缺失/无法辨认：日期免填，确认后设备退出自动判定
  // 仅 ledger_fix（台账补录）有效：随单提交，确认后回写台账
  manufacture_date?: string
  last_maintenance_date?: string
}

export function registerMaintenance(data: MaintenanceRegisterForm) {
  return request<{ id: string }>({ url: '/equipment/maintenance', method: 'post', data })
}

export function listMaintenancePending(params: { page?: number; page_size?: number }) {
  return request<PageResult<MaintenanceItem>>({ url: '/equipment/maintenance-pending', method: 'get', params })
}

export interface ConfirmResult {
  confirmed: number
  skipped: number
  not_found: string[]
}

export function confirmMaintenances(ids: string[]) {
  return request<ConfirmResult>({ url: '/equipment/maintenance/confirm', method: 'post', data: { ids } })
}

export function rejectMaintenance(id: string, reason: string) {
  return request<null>({ url: '/equipment/maintenance/reject', method: 'post', data: { id, reason } })
}

export function listMaintenanceHistory(equipmentId: string, params: { page?: number; page_size?: number }) {
  return request<PageResult<MaintenanceItem>>({ url: `/equipment/${equipmentId}/maintenances`, method: 'get', params })
}

// ===== 导入导出 =====

export interface EquipmentImportResult {
  total: number
  created_count: number
  updated_count: number
  fail_count: number
  fail_details: { row: number; code: string; reason: string }[]
}

// 整个文件导入到指定小区（列头兼容甲方台账结构，同编号按更新）
export function importEquipment(communityId: string, file: File) {
  const form = new FormData()
  form.append('community_id', communityId)
  form.append('file', file)
  return request<EquipmentImportResult>({
    url: '/equipment/import',
    method: 'post',
    data: form,
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 120000
  })
}
