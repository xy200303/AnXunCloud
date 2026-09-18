/**
 * 巡检任务与点位定位域：今日/历史任务、任务明细、扫码定位、附近点位。
 */

import { httpGet } from '@/services/request'
import { toId, buildQuery } from './common'

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
  /** 巡更轮次名（任务快照；非轮次任务为空串；抽查计划任务固定「抽查」） */
  round_name: string
  /** 完成期限（YYYY-MM-DD；抽查计划任务下发，日常任务为空串） */
  due_date: string
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

/** 台账有效期（equipment_validity）单台设备自动判定结果（v1.6：绑定即启用、逐台独立；
 *  任务详情为每台关联在用设备注入一条合成检查项，本结构挂在该项 auto_judge 上） */
export type EquipmentAutoJudge = {
  /** normal 正常 / warning 临期 / overdue 已逾期 / no_data 台账数据缺失（需补录） */
  status: string
  /** 到期日（YYYY-MM-DD，no_data 为空串） */
  next_due_date: string
  /** 生效的临期阈值（设备覆盖 > 全局） */
  warn_days: number
  /** 已逾期天数（>0 表示逾期） */
  overdue_days: number
  /** 报废日已过（视同逾期，应停用更换） */
  scrap_due?: boolean
  /** 报废日期（可空） */
  scrap_date?: string
  /** 存在待确认维保登记（展示「待确认」） */
  has_pending_register: boolean
  /** 是否展示「已完成维保？登记」入口 */
  show_register: boolean
  equipment_id: string
  equipment_code: string
  equipment_name: string
}

/** 检查项模板项（TaskDetail 点位 check_items 元素；含绑定设备注入的台账合成项） */
export type CheckItemTpl = {
  name: string
  requirement: string
  /** none/optional/required */
  photo_required: string
  /** 判定方式：manual=感官项（人工正常/异常），equipment_validity=台账有效期（合成项，服务端自动判定），其余=拍照 AI 识别项 */
  judge_type: string
  /** 合成项设备快照（judge_type=equipment_validity 时：{equipment_id, equipment_no}） */
  judge_config?: Record<string, any> | null
  /** 台账有效期单台设备自动判定（judge_type=equipment_validity 时后端透出；其余项为空） */
  auto_judge?: EquipmentAutoJudge | null
  /** 多模板并集来源模板 ID（整组拍照点位：检查项由点位组合的多模板并集而来） */
  template_id?: string
  /** 来源模板名（与 template_id 对应，展示/排查用） */
  template_name?: string
  /** 观察点 tag 数组（空=无观察点；提交时 abnormal_tags 须 ⊆ tags，非空即该项判异常） */
  tags?: string[]
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
  /** 拍照模式：group=整组 1 张整体照 AI 一次识别（点位所有模板均为整组模式）；per_item=逐项拍照（缺省） */
  photo_mode: 'group' | 'per_item'
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
    /** 审核状态：pending 审核中 / pass 已通过 / rejected 被打回 */
    audit_status?: string
    /** 打回原因（rejected 时有值） */
    audit_remark?: string
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
  /** 完成期限（抽查任务快照，如 2026-09-30；日常任务为空串） */
  due_date: string
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
  due_date?: string
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
    photo_mode?: string
    check_items?: Array<{ name?: string; requirement?: string; photo_required?: string; judge_type?: string; judge_config?: Record<string, any> | null; auto_judge?: EquipmentAutoJudge | null; template_id?: string | number; template_name?: string; tags?: string[] }>
    my_checkin?: {
      id?: string | number
      checkin_time?: string
      checkin_type?: string
      distance_to_point?: number | null
      altitude?: number | null
      accuracy?: number | null
      result?: string
      is_suspect?: boolean
      audit_status?: string
      audit_remark?: string
      locked?: boolean
    } | null
  }>
}

/** 今日任务 GET /tasks/today */
export function apiTasksToday(): Promise<TodayTasks> {
  return tasksByDate('/tasks/today')
}

/** 历史任务 GET /tasks/history?date=YYYY-MM-DD（逾期任务可进详情补拍） */
export function apiTasksHistory(date: string): Promise<TodayTasks> {
  return tasksByDate('/tasks/history' + buildQuery({ date: date }))
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

/** 附近点位 GET /points/nearby?longitude&latitude（找点辅助：GPS 只能定位到楼栋级，楼内仍需扫码/NFC 确认） */
export function apiNearbyPoints(longitude: number, latitude: number): Promise<{ list: NearbyPoint[]; ai_enabled: boolean }> {
  return new Promise((resolve, reject) => {
    httpGet<{ list: NearbyPoint[]; ai_enabled: boolean }>('/points/nearby' + buildQuery({ longitude: longitude, latitude: latitude }))
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
          due_date: d.due_date ?? '',
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
            photo_mode: p.photo_mode == 'group' ? 'group' : 'per_item',
            check_items: (p.check_items ?? []).map((c) => ({
              name: c.name ?? '',
              requirement: c.requirement ?? '',
              photo_required: c.photo_required ?? '',
              judge_type: c.judge_type ?? '',
              judge_config: c.judge_config ?? null,
              auto_judge: c.auto_judge ?? null,
              template_id: toId(c.template_id),
              template_name: c.template_name ?? '',
              tags: c.tags ?? []
            })),
            my_checkin: p.my_checkin == null
              ? null
              : {
                  id: toId(p.my_checkin.id),
                  checkin_time: p.my_checkin.checkin_time ?? '',
                  checkin_type: p.my_checkin.checkin_type ?? '',
                  distance_to_point: p.my_checkin.distance_to_point ?? null,
                  altitude: p.my_checkin.altitude ?? null,
                  accuracy: p.my_checkin.accuracy ?? null,
                  result: p.my_checkin.result ?? '',
                  is_suspect: p.my_checkin.is_suspect ?? false,
                  audit_status: p.my_checkin.audit_status ?? '',
                  audit_remark: p.my_checkin.audit_remark ?? '',
                  locked: p.my_checkin.locked ?? false
                }
          }))
        })
      })
      .catch(reject)
  })
}
