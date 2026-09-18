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
  /** 拍照引导语（任务详情模板项透出；空串=未配置，卡片兜底「拍「项名」照片」） */
  guide: string
  /** manual=感官项；equipment_validity=台账有效期（服务端自动判定）；equipment_date_spot=标签抽查合成项（交互同普通拍照项，日期由服务端从 AI 读标签草稿解析）；其余=拍照 AI 识别项 */
  judge_type: string
  /** 观察点 tag 数组（任务详情模板透出；空=无观察点） */
  tags: string[]
  /** 巡检员点选/AI 预标记的异常观察点 tag（⊆ tags；非空即该项判异常） */
  abnormal_tags: string[]
  /** 台账有效期自动判定（judge_type=equipment_validity 时由任务详情带出） */
  auto_judge?: import('@/services/api').EquipmentAutoJudge | null
  /** 拍照要求：none/optional/required（手动档向导据此决定先拍照还是可直接作答） */
  photo_required?: string
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
  exception_type?: string
  quality_pass: boolean
  quality_issue: string
  /** 照片加载失败标记（image @error 置真，显示占位提示，不持久化语义） */
  img_error?: boolean
  /** 最终结论（manual 项由巡检员直接给出；拍照项由 AI 结论推导） */
  pass: boolean
  /** true = 该项是人工确认的结论（手动档作答/跳过识别）：上送时 ai_verdict/ai_reason/ai_reading 一律置空，不冒用 AI 结论 */
  manual_confirmed?: boolean
  /** 异常描述（AI 描述或巡检员手填，可编辑） */
  note: string
  /** 上传失败待补传的本地压缩照片路径（''/undefined = 无待补传；仅会话内有效，页面重进后该项按云端草稿回到待拍） */
  pending_local?: string
  /** 待补传链路：'ai' 拍照识别 / 'escape' 异常佐证 / 'manual' 手动档拍照（重试成功后继续原链路） */
  pending_mode?: '' | 'ai' | 'escape' | 'manual'
  /** escape 链路的异常类型（device_missing / unable_to_capture） */
  pending_exception_type?: string
  /** 拍摄时刻 GCJ-02 经度（防作弊数据源；30s 缓存定位，失败为空 = 不送） */
  shoot_lng?: number
  /** 拍摄时刻 GCJ-02 纬度 */
  shoot_lat?: number
  /** 拍摄时刻 "YYYY-MM-DD HH:mm:ss"（拍照成功即同步写入，定位失败也有） */
  shoot_at?: string
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
  /** 云端凭证草稿恢复的核验方式（qrcode/nfc/fence；空/undefined = 无草稿或会话内新核验） */
  cred_type?: string
  /** 云端凭证草稿恢复的核验时间 "YYYY-MM-DD HH:mm:ss"（凭证步展示「已于 HH:mm 核验」取 HH:mm） */
  cred_verified_at?: string
  /** 云端凭证草稿恢复的围栏核验距离（米；≥0 时围栏判定直接通过，无需等重新定位） */
  fence_distance?: number
  items: WizardItemSnap[]
}
