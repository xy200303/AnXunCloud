/**
 * 设备台账与维保周期管理域（《设备台账与维保周期管理设计方案》）。
 * 通道约定与点位管理一致：相对路径随端解析（App=/api/app，小程序=/api/mp），
 * 管理端能力（列表/详情/历史/确认链）复用 PC 控制器 + 同一套权限点，巡检员能力为待维保与登记。
 */

import { httpGet, httpPost, httpPut, httpGetMp } from '@/services/request'
import { buildQuery } from './common'

/** 到期状态：normal 正常 / warning 临期 / overdue 已逾期 / none 无到期日 */
export type EquipmentDueState = 'normal' | 'warning' | 'overdue' | 'none' | 'scrap' | 'label_missing'

/** 台账列表项（/equipment/list 元素；mp 待维保列表同形，另带 overdue_days） */
export type EquipmentListItem = {
  id: string
  code: string
  name: string
  type: string
  type_label: string
  community_id: string
  community_name: string
  building_id: string | null
  building_name?: string
  point_id: string | null
  point_name?: string
  manufacture_date: string
  last_maintenance_date: string
  next_due_date: string
  scrap_date: string
  due_state: EquipmentDueState
  status: string
  status_label: string
  remark: string
  /** 档案扩展信息（导入映射：project_name/room/dept/system/brand/spec/original_value/quantity/put_into_service/maint_status/run_status/origin/各联系人/other_info 等） */
  extra?: Record<string, any>
  /** 标签缺失标记（v1.7） */
  label_missing?: boolean
  /** mp 待维保列表附带：已逾期天数（<=0 表示未逾期） */
  overdue_days?: number
}

export type EquipmentDetail = EquipmentListItem & {
  updated_at: string
  type_rule?: { first_months: number; cycle_months: number; remind: boolean; scrap_months: number }
}

export type EquipmentPage = {
  list: EquipmentListItem[]
  total: number
  page: number
  page_size: number
}

/** 维保流水（待确认列表/设备历史共用元素） */
export type MaintenanceItem = {
  id: string
  equipment_id: string
  equipment_code: string
  equipment_name: string
  point_id?: string
  point_name?: string
  maintenance_type: string
  maintenance_date: string
  vendor: string | null
  operator_name: string
  note: string
  photos: Array<{ file_id: string; url: string }>
  /** pending 待确认 / confirmed 已确认 / rejected 已驳回 */
  confirm_status: string
  /** 标签缺失登记（确认后设备退出自动判定） */
  label_missing?: boolean
  reject_reason: string | null
  /** AI 预检：pass/review/ null（未预检） */
  ai_verdict: string | null
  ai_reason: string | null
  /** 待确认列表透出：当前环节名（如 经理确认） */
  current_step_name?: string
  /** 待确认列表透出：当前用户是否在当前环节授权名单内（false=只能查看） */
  can_confirm?: boolean
  created_by_name: string
  created_at: string
  confirmed_by_name?: string
  confirmed_at?: string
  fix_manufacture_date?: string
  fix_last_maintenance_date?: string
}

export type MaintenancePage = {
  list: MaintenanceItem[]
  total: number
  page: number
  page_size: number
}

/** 批量确认 POST /equipment/maintenance/confirm（幂等：已处理跳过；advanced=多环节链推进中；forbidden=不在当前环节名单） */
export type MaintenanceConfirmResult = { confirmed: number; advanced: number; skipped: number; not_found: string[]; forbidden: string[] }

/** 台账分页 GET /equipment/list（type/community_id/due_state/keyword/point_id 筛选） */
export function apiEquipmentList(
  page: number,
  pageSize: number,
  opts: { type?: string; communityId?: string; dueState?: string; keyword?: string; pointId?: string } = {}
): Promise<EquipmentPage> {
  const path = '/equipment/list' + buildQuery({
    page: page,
    page_size: pageSize,
    type: opts.type,
    community_id: opts.communityId,
    due_state: opts.dueState,
    keyword: opts.keyword,
    point_id: opts.pointId
  })
  return new Promise<EquipmentPage>((resolve, reject) => {
    httpGet<any>(path)
      .then((d) => {
        resolve({
          list: (d?.list ?? []) as EquipmentListItem[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 巡检员设备台账分页 GET /mp/equipment/list（维保登记选设备等免 equipment:list 权限场景；keyword 模糊/due_state 可选。
 *  App 端 /api/app/equipment/list 是管理端口径（RequirePerm equipment:list），故跨组走 mp 基址（会话通道互通）；
 *  响应兼容分页信封 {list,total} 与 MpDueDevices 纯数组两种形态（后端落地后以实际为准）） */
export function apiMpEquipmentList(
  page: number,
  pageSize: number,
  opts: { keyword?: string; dueState?: string } = {}
): Promise<EquipmentPage> {
  const path = '/equipment/list' + buildQuery({
    page: page,
    page_size: pageSize,
    keyword: opts.keyword,
    due_state: opts.dueState
  })
  return new Promise<EquipmentPage>((resolve, reject) => {
    httpGetMp<any>(path)
      .then((d) => {
        const list = (Array.isArray(d) ? d : d?.list ?? []) as EquipmentListItem[]
        resolve({
          list: list,
          total: Array.isArray(d) ? list.length : d?.total ?? list.length,
          page: Array.isArray(d) ? page : d?.page ?? page,
          page_size: Array.isArray(d) ? pageSize : d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 设备详情 GET /equipment/:id */
export function apiEquipmentDetail(id: string): Promise<EquipmentDetail> {
  return new Promise<EquipmentDetail>((resolve, reject) => {
    httpGet<EquipmentDetail>('/equipment/' + id)
      .then((d) => {
        if (d == null) {
          reject(new Error('设备详情响应异常'))
          return
        }
        resolve(d)
      })
      .catch(reject)
  })
}

/** 我的待维保设备 GET /equipment/due（本租户临期+逾期在用设备，逾期在前） */
export function apiEquipmentDue(): Promise<EquipmentListItem[]> {
  return new Promise<EquipmentListItem[]>((resolve, reject) => {
    httpGet<EquipmentListItem[]>('/equipment/due')
      .then((d) => resolve(d ?? []))
      .catch(reject)
  })
}

/** 维保登记 POST /equipment/maintenance（拍新标签即登记；后端同步 AI 核对：可信直接 confirmed 回写台账，存疑 pending 待经理确认） */
export function apiEquipmentRegister(req: {
  equipment_id: string
  maintenance_type?: string
  maintenance_date?: string
  vendor?: string
  note?: string
  file_ids: string[]
  manufacture_date?: string
  last_maintenance_date?: string
}): Promise<{ id: string; confirm_status?: string; confirm_mode?: string }> {
  return new Promise<{ id: string; confirm_status?: string; confirm_mode?: string }>((resolve, reject) => {
    httpPost<{ id: string; confirm_status?: string; confirm_mode?: string }>('/equipment/maintenance', req as unknown as Record<string, any>)
      .then((d) => resolve(d ?? { id: '' }))
      .catch(reject)
  })
}

/** 设备维保历史 GET /equipment/:id/maintenances */
export function apiMaintenanceHistory(equipmentId: string, page: number, pageSize: number): Promise<MaintenancePage> {
  return new Promise<MaintenancePage>((resolve, reject) => {
    httpGet<any>('/equipment/' + equipmentId + '/maintenances' + buildQuery({ page: page, page_size: pageSize }))
      .then((d) => {
        resolve({
          list: (d?.list ?? []) as MaintenanceItem[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 待确认维保登记 GET /equipment/maintenance-pending（AI 存疑置顶，需 equipment:confirm） */
export function apiMaintenancePending(page: number, pageSize: number): Promise<MaintenancePage> {
  return new Promise<MaintenancePage>((resolve, reject) => {
    httpGet<any>('/equipment/maintenance-pending' + buildQuery({ page: page, page_size: pageSize }))
      .then((d) => {
        resolve({
          list: (d?.list ?? []) as MaintenanceItem[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 批量确认 POST /equipment/maintenance/confirm */
export function apiMaintenanceConfirm(ids: string[]): Promise<MaintenanceConfirmResult> {
  return new Promise((resolve, reject) => {
    httpPost<MaintenanceConfirmResult>('/equipment/maintenance/confirm', { ids })
      .then((d) => resolve(d ?? { confirmed: 0, advanced: 0, skipped: 0, not_found: [], forbidden: [] }))
      .catch(reject)
  })
}

/** 驳回 POST /equipment/maintenance/reject（理由必填，通知登记人） */
export function apiMaintenanceReject(id: string, reason: string): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    httpPost('/equipment/maintenance/reject', { id: id, reason: reason })
      .then(() => resolve())
      .catch(reject)
  })
}

/** 我的提交 GET /equipment/maintenance-mine（全部状态，最新在前） */
export function apiMaintenanceMine(page: number, pageSize: number): Promise<MaintenancePage> {
  return new Promise<MaintenancePage>((resolve, reject) => {
    httpGet<any>('/equipment/maintenance-mine' + buildQuery({ page: page, page_size: pageSize }))
      .then((d) => {
        resolve({
          list: (d?.list ?? []) as MaintenanceItem[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 待确认登记修改 PUT /equipment/maintenance/:id（限本人；照片变更后端重新 AI 核验，不合格 43107 拦截） */
export function apiMaintenanceUpdate(
  id: string,
  req: { file_ids: string[]; note?: string; maintenance_date?: string }
): Promise<{ id: string; confirm_status?: string }> {
  return new Promise<{ id: string; confirm_status?: string }>((resolve, reject) => {
    httpPut<{ id: string; confirm_status?: string }>('/equipment/maintenance/' + id, req as unknown as Record<string, any>)
      .then((d) => resolve(d ?? { id: id }))
      .catch(reject)
  })
}
