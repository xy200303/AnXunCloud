/**
 * 连续巡检向导断点快照（uni storage 持久化）。
 *
 * - 普通模式 key：CHECKIN_WIZARD_<taskId>
 * - 修改模式 key：CHECKIN_WIZARD_<taskId>_M_<pointId>（单点位覆盖修改，独立于普通快照）
 * - 每次状态变更即写入；点位提交成功从快照移除；任务完成清除快照
 * - detail.vue 只读取快照做「继续巡检」入口与「AI 检查中」状态展示
 */

/** 向导内单个检查项状态 */
export type WizardItemSnap = {
  name: string
  requirement: string
  /** manual=感官项；其余=拍照 AI 识别项 */
  judge_type: string
  /** 水印烧录后的本地照片路径 */
  photos: string[]
  /** 已上传的 file_key（与 photos 一一对应） */
  file_keys: string[]
  /** AI 识别 job（拍照项提交后轮询用） */
  job_id: string
  /** todo 待拍 / recognizing 识别中 / done 已有结论 / failed 失败（回退待拍） */
  status: 'todo' | 'recognizing' | 'done' | 'failed'
  /** pass / review / abnormal / '' */
  verdict: string
  reason: string
  reading: string
  quality_pass: boolean
  quality_issue: string
  /** 最终结论（manual 项由巡检员直接给出；拍照项由 AI 结论推导） */
  pass: boolean
  /** 异常描述（AI 描述或巡检员手填，可编辑） */
  note: string
}

/** 向导内单点位状态 */
export type WizardPointSnap = {
  point_id: string
  /** doing 巡检中 / submitted 已提交（提交成功的点位保留在快照里做进度展示，进入向导时跳过） */
  status: 'doing' | 'submitted'
  /** 已核验的扫码编号（空 = 未核验） */
  scannedNo: string
  /** 已核验的 NFC 卡号（空 = 未核验） */
  nfcCardId: string
  items: WizardItemSnap[]
}

export type WizardSnap = {
  task_id: string
  /** true = 单点位修改模式（只走 points[0]，提交走覆盖语义） */
  modify: boolean
  pointIdx: number
  itemIdx: number
  points: WizardPointSnap[]
  saved_at: number
}

const KEY_PREFIX = 'CHECKIN_WIZARD_'

export function wizardSnapKey(taskId: string): string {
  return KEY_PREFIX + taskId
}

export function wizardModifySnapKey(taskId: string, pointId: string): string {
  return KEY_PREFIX + taskId + '_M_' + pointId
}

/** 读取快照；形状不符返回 null（视为无快照） */
export function loadWizardSnap(key: string): WizardSnap | null {
  try {
    const v = uni.getStorageSync(key)
    if (v == null || typeof v != 'object') return null
    const s = v as WizardSnap
    if (typeof s.task_id != 'string' || s.task_id == '') return null
    if (!Array.isArray(s.points)) return null
    if (typeof s.pointIdx != 'number' || typeof s.itemIdx != 'number') return null
    return s
  } catch (e) {
    return null
  }
}

export function saveWizardSnap(key: string, snap: WizardSnap): void {
  try {
    snap.saved_at = Date.now()
    // JSON 深拷贝去 Vue 响应式代理（原生端存储对 proxy 不友好）
    uni.setStorageSync(key, JSON.parse(JSON.stringify(snap)))
  } catch (e) {
    // 存储失败不阻断巡检主链路
  }
}

export function clearWizardSnap(key: string): void {
  try {
    uni.removeStorageSync(key)
  } catch (e) {
    // 忽略
  }
}

/** 概览页展示用：快照进度（done=已提交点位数，total=快照点位数） */
export function wizardSnapProgress(snap: WizardSnap): { done: number; total: number } {
  return {
    done: snap.points.filter((p) => p.status == 'submitted').length,
    total: snap.points.length
  }
}
