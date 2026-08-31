/**
 * 连续巡检向导的内存状态类型。
 *
 * 巡检进度不落本地：逐项照片/AI 结论/手动项选择均实时写服务端 checkin_item_draft，
 * 进入向导时从 GET /checkin/item-drafts 整体重建；点位正式提交后服务端删除草稿。
 * 本地不再使用 uni storage 快照（临时路径失效、对账复杂，已废弃）。
 */

/** 向导内单个检查项状态 */
export type WizardItemSnap = {
  name: string
  requirement: string
  /** manual=感官项；其余=拍照 AI 识别项 */
  judge_type: string
  /** 照片展示地址（上传成功后的服务端 URL；本地临时路径仅即时预览，重启后可能失效） */
  photos: string[]
  /** 已上传的 upload_file.id（与 photos 一一对应） */
  file_ids: string[]
  /** 最近一张照片的 upload_file.id */
  file_id?: string
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
  /** 照片加载失败标记（image @error 置真，显示占位提示，不持久化语义） */
  img_error?: boolean
  /** 最终结论（manual 项由巡检员直接给出；拍照项由 AI 结论推导） */
  pass: boolean
  /** 异常描述（AI 描述或巡检员手填，可编辑） */
  note: string
}

/** 向导内单点位状态 */
export type WizardPointSnap = {
  point_id: string
  /** doing 巡检中 / submitted 已提交（会话内进度展示用；重新进入时以服务端 my_checkin 为准） */
  status: 'doing' | 'submitted'
  /** 已核验的扫码编号（空 = 未核验） */
  scannedNo: string
  /** 已核验的 NFC 卡号（空 = 未核验） */
  nfcCardId: string
  items: WizardItemSnap[]
}
