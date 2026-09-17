/**
 * 打卡域：打卡提交、AI 逐项/整组识别 job、逐项草稿、照片上传、离线补传、本人记录查询。
 */

import { httpGet, httpPost, refreshSession, getBaseUrl } from '@/services/request'
import { getAccessToken } from '@/utils/storage'
import { toId, buildQuery } from './common'

// ---- 业务错误码常量 ----

/** 照片质量不达标（信封 data.max_attempts 含放行次数） */
export const CODE_QUALITY_FAIL = 43107
/** AI 未启用（ErrAIDisabled，后端 errs.go 实际值） */
export const CODE_AI_DISABLED = 43108
/** 点位打卡已锁定不可覆盖（ErrCheckinLocked，后端 errs.go 实际值） */
export const CODE_CHECKIN_LOCKED = 43109

/** 打卡逐项填报元素（ai_* 为 AI 预览结论透传落库，可选） */
export type CheckinItemReqPayload = {
  name: string
  pass: boolean
  note: string
  photos: string[]
  /** AI 逐项判定透传：pass/review/abnormal/'' */
  ai_verdict?: string
  ai_reason?: string
  ai_reading?: string
  /** 异常逃生入口的项目异常类型；由服务端草稿校验后写入正式记录 */
  exception_type?: 'device_missing' | 'unable_to_capture' | ''
  /** 异常项处置方式：'' / on_site_resolved 现场已处理 / maintenance_registered 已登记维保 / report_pending 上报待处理 */
  disposition?: '' | 'on_site_resolved' | 'maintenance_registered' | 'report_pending'
  /** 处置照片 upload_file.id（disposition=on_site_resolved 时必带） */
  resolution_file_ids?: string[]
  /** 处置说明 */
  resolution_note?: string
  /** 标签抽查合成项（equipment_date_spot）：生产日期/维修日期/无贴纸/标签缺失（服务端四规则比对，pass 被忽略） */
  spot_manufacture_date?: string
  spot_maintenance_date?: string
  spot_no_sticker?: boolean
  spot_label_missing?: boolean
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
  /** 可选：海拔/定位精度（米，仅参考展示，不参与校验） */
  altitude?: number
  accuracy?: number
  /** YYYY-MM-DD HH:mm:ss（timefmt.Layout） */
  client_time: string
  /** normal/abnormal=巡检员逐项填报（auto 代判已下线，逐项识别走 ai_confirmed 流程） */
  result: 'normal' | 'abnormal'
  /** 可选：质量不达标超放行次数后强制提交（结果转待复核） */
  force?: boolean
  /** true = 巡检员已在识别概要页确认 AI 结论，服务端跳过二次 AI */
  ai_confirmed?: boolean
  remark: string
  /** 逐项填报（照片唯一归属逐项 photos；无记录级照片） */
  check_items?: CheckinItemReqPayload[]
}

/** AI 逐项识别 job 创建请求（POST /checkin/ai-item-jobs） */
export type AiItemJobCreateReq = {
  task_id: string
  point_id: string
  /** 检查项名（与点位模板对齐） */
  name: string
  /** 该项照片 file_id（一项一图硬约束，恰好 1 张） */
  file_ids: string[]
}

/** AI 逐项识别 job 状态（GET /checkin/ai-item-jobs?ids= 元素） */
export type AiItemJob = {
  job_id: string
  /** pending 识别中 / done 完成 / failed 失败（含过期，前端回退重拍） */
  status: 'pending' | 'done' | 'failed' | string
  /** pass / review / abnormal / ''（done 时有效） */
  verdict: string
  reason: string
  /** 仪表读数等识别值，无则空串 */
  reading: string
  /** 照片质量：false 时 quality_issue 为不达标原因，该项需补拍 */
  quality_pass: boolean
  quality_issue: string
}

/** AI 逐项判定（同步判定响应 ai_items 元素；reading 为仪表读数等识别值，无则空串） */
export type CheckinAiItem = {
  name: string
  /** pass / abnormal / review 等 */
  verdict: string
  reason: string
  reading: string
}

/** 打卡响应（对齐后端 resultView） */
export type CheckinResult = {
  checkin_id: string
  checkin_time: string
  distance_to_point: number
  is_suspect: boolean
  suspect_reason: string
  /** 后端是否启用 AI 审核（启用时提交后可轮询 apiCheckinItems 拿逐项结论） */
  ai_enabled: boolean
  /** AI 质量放行次数上限（43107 错误信封 data.max_attempts 同值） */
  ai_max_attempts?: number
  /** 同步判定总判定：pass / review / error */
  ai_verdict?: string
  ai_reason?: string
  /** 照片质量：pass 达标 / 否则 issue 为不达标原因 */
  ai_quality?: { pass: boolean; issue: string }
  ai_items?: CheckinAiItem[]
  /** 审核状态：auto_pass=AI 直接通过 / pending=待管理员复核 */
  audit_status?: string
  task_progress: {
    total_points: number
    done_points: number
    progress: number
    task_status: string
  }
}

/** 打卡逐项 AI 结论（GET /checkins/:id/items 元素；ai_verdict 空 = 模型未给该项结论） */
export type CheckinItemAI = {
  name: string
  pass: boolean
  /** pass / review / error / '' */
  ai_verdict: string
  ai_reason: string
  /** 人工备注（异常项说明） */
  note?: string
  /** 逐项照片可访问 URL（优先水印图；记录卡展示用） */
  photo_urls?: string[]
  /** 异常项处置方式（'' / on_site_resolved / maintenance_registered / report_pending） */
  disposition?: string
  /** 处置照片 URL（on_site_resolved 的凭证照片；记录卡展示用） */
  resolution_photo_urls?: string[]
}

/** 照片元素（后端 types.PhotoItem，打卡/审核记录通用） */
export type OrderPhoto = {
  item: string
  url: string
  watermarked_url: string
}


/** 离线补传响应（对齐 CheckinService.OfflineSync） */
export type OfflineSyncResult = {
  success: Array<{ point_id: string; checkin_id: string; checkin_time: string }>
  failed: Array<{ point_id: string; code: number; message: string }>
}

/** AI 整组识别 job 创建请求（POST /mp/checkin/ai-group-jobs；整组拍照点位：1 张整体照一次识别全部检查项） */
export type AiGroupJobCreateReq = {
  task_id: string
  point_id: string
  /** 整体照 file_id（apiUploadLocal 上传后透出），恰好 1 张 */
  file_ids: string[]
}

/** AI 整组识别逐项结论（GET /mp/checkin/ai-group-jobs/:id 响应 result.items 元素；name 与点位检查项名对齐） */
export type AiGroupJobItem = {
  name: string
  /** normal / abnormal / unrecognized（unrecognized=读不出，默认正常不计异常） */
  result: 'normal' | 'abnormal' | 'unrecognized' | string
  /** abnormal 时的判定原因（回填该项异常说明） */
  reason?: string
  /** 识别值（如有效期读数），无则空串 */
  value?: string
}

/** AI 整组识别 job 状态（GET /mp/checkin/ai-group-jobs/:id） */
export type AiGroupJob = {
  id: string
  /** pending / running / done / failed */
  status: 'pending' | 'running' | 'done' | 'failed' | string
  /** 照片中识别到的设备数量（done 时有效；与点位登记设备数比对用，-1 = 未返回） */
  count: number
  items: AiGroupJobItem[]
}

/** 逐项过程草稿（GET /checkin/item-drafts 元素）：云端保存的逐项进度（巡检进度的唯一事实来源） */
export interface ItemDraft {
  point_id: string
  item_name: string
  job_id: string
  file_ids: string[]
  photos: string[]
  ai_status: string
  ai_verdict: string
  ai_reason: string
  ai_reading: string
  exception_type?: string
  quality_pass: boolean
  quality_issue: string
  manual_pass: boolean | null
  manual_note: string
}

type RawAiItemJob = {
  job_id?: string
  status?: string
  verdict?: string
  reason?: string
  reading?: string
  quality_pass?: boolean
  quality_issue?: string
}

type RawCheckinResult = {
  checkin_id?: string | number
  checkin_time?: string
  distance_to_point?: number
  is_suspect?: boolean
  suspect_reason?: string
  ai_enabled?: boolean
  ai_max_attempts?: number
  ai_verdict?: string
  ai_reason?: string
  ai_quality?: { pass?: boolean; issue?: string }
  ai_items?: Array<{ name?: string; verdict?: string; reason?: string; reading?: string }>
  audit_status?: string
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
  data?: { file_id?: string; url?: string } | null
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
          ai_enabled: d.ai_enabled ?? false,
          ai_max_attempts: d.ai_max_attempts ?? 0,
          ai_verdict: d.ai_verdict ?? '',
          ai_reason: d.ai_reason ?? '',
          ai_quality: d.ai_quality == null
            ? undefined
            : { pass: d.ai_quality.pass ?? false, issue: d.ai_quality.issue ?? '' },
          ai_items: (d.ai_items ?? []).map((it) => ({
            name: it.name ?? '',
            verdict: it.verdict ?? '',
            reason: it.reason ?? '',
            reading: it.reading ?? ''
          })),
          audit_status: d.audit_status ?? '',
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

/** 创建 AI 逐项识别 job POST /checkin/ai-item-jobs（拍照项拍完立即调用，异步轮询结果） */
export function apiAiItemJobCreate(req: AiItemJobCreateReq): Promise<{ job_id: string }> {
  return new Promise<{ job_id: string }>((resolve, reject) => {
    httpPost<{ job_id?: string | number }>('/checkin/ai-item-jobs', req as unknown as Record<string, any>)
      .then((d) => {
        if (d == null || d.job_id == null) {
          reject(new Error('识别任务响应异常'))
          return
        }
        resolve({ job_id: toId(d.job_id) })
      })
      .catch(reject)
  })
}

/** 批量查询 AI 逐项识别 job GET /checkin/ai-item-jobs?ids=a,b,c（点位收尾轮询用） */
export function apiAiItemJobs(ids: string[]): Promise<AiItemJob[]> {
  return new Promise<AiItemJob[]>((resolve, reject) => {
    // ids 逐个 encode 后以字面逗号连接（逗号为分隔符，不能整体 encode）
    httpGet<{ jobs?: RawAiItemJob[] }>('/checkin/ai-item-jobs?ids=' + ids.map(encodeURIComponent).join(','))
      .then((d) => {
        resolve(
          (d?.jobs ?? []).map((j) => ({
            job_id: j.job_id ?? '',
            status: j.status ?? '',
            verdict: j.verdict ?? '',
            reason: j.reason ?? '',
            reading: j.reading ?? '',
            quality_pass: j.quality_pass ?? true,
            quality_issue: j.quality_issue ?? ''
          }))
        )
      })
      .catch(reject)
  })
}

/** 创建 AI 整组识别 job POST /mp/checkin/ai-group-jobs（整体照上传成功即调用，异步轮询结果） */
export function apiAiGroupJobCreate(req: AiGroupJobCreateReq): Promise<{ id: string }> {
  return new Promise<{ id: string }>((resolve, reject) => {
    httpPost<{ job_id?: string | number }>('/mp/checkin/ai-group-jobs', req as unknown as Record<string, any>)
      .then((d) => {
        if (d == null || d.job_id == null) {
          reject(new Error('识别任务响应异常'))
          return
        }
        resolve({ id: toId(d.job_id) })
      })
      .catch(reject)
  })
}

/** 查询 AI 整组识别 job GET /mp/checkin/ai-group-jobs/:id（2s 间隔轮询，done 后按 items 回填检查项） */
export function apiAiGroupJob(id: string): Promise<AiGroupJob> {
  return new Promise<AiGroupJob>((resolve, reject) => {
    httpGet<{ job_id?: string | number; status?: string; result?: { count?: number; items?: Array<{ name?: string; result?: string; reason?: string; value?: string }> } }>('/mp/checkin/ai-group-jobs/' + encodeURIComponent(id))
      .then((d) => {
        if (d == null) {
          reject(new Error('识别结果响应异常'))
          return
        }
        resolve({
          id: toId(d.job_id),
          status: d.status ?? '',
          count: typeof d.result?.count == 'number' ? d.result.count : -1,
          items: (d.result?.items ?? []).map((it) => ({
            name: it.name ?? '',
            result: it.result ?? '',
            reason: it.reason ?? '',
            value: it.value ?? ''
          }))
        })
      })
      .catch(reject)
  })
}

/** 查询逐项过程草稿 GET /checkin/item-drafts?task_id[&point_id]（pointId 空=整个任务；断点恢复用） */
export function apiItemDrafts(taskId: string, pointId?: string): Promise<ItemDraft[]> {
  return new Promise<ItemDraft[]>((resolve, reject) => {
    const url = '/checkin/item-drafts' + buildQuery({ task_id: taskId, point_id: pointId })
    httpGet<{ items?: any[] }>(url)
      .then((d) => {
        resolve(
          (d?.items ?? []).map((it) => ({
            point_id: it.point_id ?? '',
            item_name: it.item_name ?? '',
            job_id: it.job_id ?? '',
            file_ids: it.file_ids ?? [],
            photos: it.photos ?? [],
            ai_status: it.ai_status ?? '',
            ai_verdict: it.ai_verdict ?? '',
            ai_reason: it.ai_reason ?? '',
            ai_reading: it.ai_reading ?? '',
            exception_type: it.exception_type ?? '',
            quality_pass: it.quality_pass ?? true,
            quality_issue: it.quality_issue ?? '',
            manual_pass: it.manual_pass ?? null,
            manual_note: it.manual_note ?? ''
          }))
        )
      })
      .catch(reject)
  })
}

/** 手动确认项选择落云端草稿 POST /checkin/item-drafts/manual */
export function apiItemDraftManual(req: { task_id: string; point_id: string; name: string; pass: boolean; note: string }): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    httpPost('/checkin/item-drafts/manual', req as unknown as Record<string, any>)
      .then(() => resolve())
      .catch(reject)
  })
}

/** 拍照项异常逃生入口 POST /checkin/item-drafts/photo-abnormal */
export function apiItemDraftPhotoAbnormal(req: { task_id: string; point_id: string; name: string; file_ids: string[]; note: string; exception_type: 'device_missing' | 'unable_to_capture' }): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    httpPost('/checkin/item-drafts/photo-abnormal', req as unknown as Record<string, any>)
      .then(() => resolve())
      .catch(reject)
  })
}

/** 打卡逐项 AI 结论 GET /checkins/:id/items（本人记录；AI 审核异步，提交后延迟轮询用） */
export function apiCheckinItems(checkinId: string): Promise<CheckinItemAI[]> {
  return new Promise<CheckinItemAI[]>((resolve, reject) => {
    httpGet<CheckinItemAI[]>('/checkins/' + checkinId + '/items')
      .then((d) => {
        if (d == null) {
          reject(new Error('打卡逐项结论响应异常'))
          return
        }
        resolve(
          d.map((it) => ({
            name: it.name ?? '',
            pass: it.pass ?? false,
            ai_verdict: it.ai_verdict ?? '',
            ai_reason: it.ai_reason ?? '',
            note: it.note ?? '',
            photo_urls: it.photo_urls ?? [],
            disposition: it.disposition ?? '',
            resolution_photo_urls: it.resolution_photo_urls ?? []
          }))
        )
      })
      .catch(reject)
  })
}

/** scene：checkin / avatar / signature，默认 checkin。 */
export function apiUploadLocal(
  filePath: string,
  scene: 'checkin' | 'avatar' | 'signature' = 'checkin',
  retried = false
): Promise<{ file_id: string; url: string }> {
  return new Promise((resolve, reject) => {
    uni.uploadFile({
      url: getBaseUrl() + '/upload/local',
      filePath: filePath,
      name: 'file',
      formData: { scene: scene },
      header: { Authorization: 'Bearer ' + getAccessToken() },
      timeout: 60000, // 弱网保底：60s 必 fail，防止上传永久挂起
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
          if (env.code == 40102 && !retried) {
            refreshSession().then((ok) => {
              if (!ok) {
                reject(new Error('登录状态已失效，请重新登录'))
                return
              }
              apiUploadLocal(filePath, scene, true).then(resolve).catch(reject)
            }).catch(() => reject(new Error('登录状态已失效，请重新登录')))
            return
          }
          reject(new Error(env.message || '上传失败'))
          return
        }
        resolve({
          file_id: env.data?.file_id ?? '',
          url: env.data?.url ?? ''
        })
      },
      fail: () => {
        reject(new Error('网络异常，照片上传失败'))
      }
    })
  })
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

/** 本人打卡记录摘要 GET /checkins/:id（消息深链：打回提醒定位记录卡） */
export function apiCheckinBrief(id: string): Promise<{ id: string; task_id: string; point_id: string; audit_status: string }> {
  return new Promise((resolve, reject) => {
    httpGet<any>('/checkins/' + encodeURIComponent(id))
      .then((d) => {
        if (d == null) {
          reject(new Error('打卡记录响应异常'))
          return
        }
        resolve({ id: d.id ?? '', task_id: d.task_id ?? '', point_id: d.point_id ?? '', audit_status: d.audit_status ?? '' })
      })
      .catch(reject)
  })
}
