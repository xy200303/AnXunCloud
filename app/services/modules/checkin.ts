/**
 * 打卡域：打卡提交、AI 逐项识别 job、逐项草稿、照片上传、离线补传、本人记录查询。
 */

import { httpDelete, httpGet, httpPost, refreshSession, getBaseUrl } from '@/services/request'
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
  /** 三态结论：normal 正常 / abnormal 异常 / escaped 无法检查（逃生，没检成） */
  result: 'normal' | 'abnormal' | 'escaped'
  note: string
  photos: string[]
  /** AI 逐项判定透传：pass/review/abnormal/'' */
  ai_verdict?: string
  ai_reason?: string
  ai_reading?: string
  /** 逃生类型（仅 result=escaped 时携带）：device_missing 设备不存在 / unable_to_capture 无法拍摄 / camera_broken 相机故障 / label_missing 标签磨损（仅标签抽查合成项，由服务端草稿校验后写入正式记录） */
  exception_type?: 'device_missing' | 'unable_to_capture' | 'camera_broken' | 'label_missing'
  /** 异常观察点 tag（须 ⊆ 该项 tags；非空服务端强制该项 result=abnormal） */
  abnormal_tags?: string[]
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
  /** 拍摄时空信息（防作弊数据源，可选）：GCJ-02 坐标与拍摄时刻 "YYYY-MM-DD HH:mm:ss" */
  shoot_lng?: number
  shoot_lat?: number
  shoot_at?: string
}

/** 凭证核验草稿（POST /checkin/point-cred）：核验通过即 upsert，断点恢复用 */
export type PointCredSaveReq = {
  task_id: string
  point_id: string
  checkin_type: 'qrcode' | 'nfc' | 'fence'
  /** qrcode/nfc 传对应编号；fence 传空串 */
  cred_no: string
  /** 核验时与点位距离（米；未知传 0） */
  fence_distance: number
}

/** 凭证核验草稿（GET /checkin/item-drafts 响应顶层 credential；断点恢复用） */
export type PointCredDraft = {
  checkin_type: string
  cred_no: string
  fence_distance: number
  /** "YYYY-MM-DD HH:mm:ss"（服务端 now()） */
  verified_at: string
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
  /** AI 判出的异常观察点 tag（done 时有效；须 ⊆ 该项 tags，非空即该项异常） */
  abnormal_tags?: string[]
}

/** AI 逐项判定（同步判定响应 ai_items 元素；reading 为仪表读数等识别值，无则空串） */
export type CheckinAiItem = {
  name: string
  /** pass / abnormal / review 等 */
  verdict: string
  reason: string
  reading: string
  /** AI 判出的异常观察点 tag */
  abnormal_tags?: string[]
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
  /** 三态结论：normal 正常 / abnormal 异常 / escaped 无法检查（escaped 时 exception_type 给出原因） */
  result: string
  /** pass / review / error / '' */
  ai_verdict: string
  ai_reason: string
  /** 人工备注（异常项说明） */
  note?: string
  /** 无法检查原因（仅 escaped 态有意义：device_missing/unable_to_capture/label_missing） */
  exception_type?: string
  /** 逐项照片可访问 URL（优先水印图；记录卡展示用） */
  photo_urls?: string[]
  /** 观察点 tag 快照（记录详情透出；空=无观察点） */
  tags?: string[]
  /** 异常观察点 tag 列表（⊆ tags；非空即该项异常） */
  abnormal_tags?: string[]
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

/** 逐项过程草稿（GET /checkin/item-drafts 元素）：云端保存的逐项进度（巡检进度的唯一事实来源） */
export interface ItemDraft {
  point_id: string
  item_name: string
  /** 草稿种类（恢复分发唯一依据）：ai 识别草稿 / manual 人工结论草稿 / escape 逃生草稿 */
  draft_kind: 'ai' | 'manual' | 'escape'
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
  /** 异常观察点 tag（AI 判出或巡检员点选，断点恢复用） */
  abnormal_tags?: string[]
  /** 拍摄时空信息（草稿带回，再次保存时坐标不丢） */
  shoot_lng?: number
  shoot_lat?: number
  shoot_at?: string
}

type RawAiItemJob = {
  job_id?: string
  status?: string
  verdict?: string
  reason?: string
  reading?: string
  quality_pass?: boolean
  quality_issue?: string
  abnormal_tags?: string[]
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
  ai_items?: Array<{ name?: string; verdict?: string; reason?: string; reading?: string; abnormal_tags?: string[] }>
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
            reading: it.reading ?? '',
            abnormal_tags: it.abnormal_tags ?? []
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
            quality_issue: j.quality_issue ?? '',
            abnormal_tags: j.abnormal_tags ?? []
          }))
        )
      })
      .catch(reject)
  })
}

/** 查询逐项过程草稿 GET /checkin/item-drafts?task_id[&point_id]（pointId 空=整个任务；断点恢复用。
 *  响应顶层 credential 为该点位的凭证核验草稿，仅按点位查询时有效（整任务查询为 null）） */
export function apiItemDrafts(taskId: string, pointId?: string): Promise<{ items: ItemDraft[]; credential: PointCredDraft | null }> {
  return new Promise((resolve, reject) => {
    const url = '/checkin/item-drafts' + buildQuery({ task_id: taskId, point_id: pointId })
    httpGet<{ items?: any[]; credential?: any }>(url)
      .then((d) => {
        const c = d?.credential
        resolve({
          items: (d?.items ?? []).map((it) => ({
            point_id: it.point_id ?? '',
            item_name: it.item_name ?? '',
            draft_kind: it.draft_kind ?? 'ai',
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
            manual_note: it.manual_note ?? '',
            abnormal_tags: it.abnormal_tags ?? [],
            shoot_lng: it.shoot_lng ?? undefined,
            shoot_lat: it.shoot_lat ?? undefined,
            shoot_at: it.shoot_at ?? undefined
          })),
          credential:
            c == null
              ? null
              : {
                  checkin_type: c.checkin_type ?? '',
                  cred_no: c.cred_no ?? '',
                  fence_distance: c.fence_distance ?? 0,
                  verified_at: c.verified_at ?? ''
                }
        })
      })
      .catch(reject)
  })
}

/** 凭证核验通过即落云端草稿 POST /checkin/point-cred（upsert；弱网失败忽略不阻塞——凭证仅断点恢复用，提交时服务端仍复核） */
export function apiPointCredSave(req: PointCredSaveReq): Promise<void> {
  return new Promise<void>((resolve) => {
    httpPost('/checkin/point-cred', req as unknown as Record<string, any>)
      .then(() => resolve())
      .catch(() => resolve())
  })
}

/** 手动结论落云端草稿 POST /checkin/item-drafts/manual（感官项与手动档向导的拍照项通用；拍照项携 file_ids ≤3 与 abnormal_tags ⊆ 模板 tags；shoot_* 拍摄时空信息可选） */
export function apiItemDraftManual(req: { task_id: string; point_id: string; name: string; pass: boolean; note: string; file_ids?: string[]; abnormal_tags?: string[]; shoot_lng?: number; shoot_lat?: number; shoot_at?: string }): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    httpPost('/checkin/item-drafts/manual', req as unknown as Record<string, any>)
      .then(() => resolve())
      .catch(reject)
  })
}

/** 撤销某项过程草稿 DELETE /checkin/item-drafts（逃生选错回到待拍；进行中的 job 落定不会复活草稿） */
export function apiItemDraftDelete(req: { task_id: string; point_id: string; name: string }): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    httpDelete(
      '/checkin/item-drafts?task_id=' + encodeURIComponent(req.task_id) +
        '&point_id=' + encodeURIComponent(req.point_id) + '&item_name=' + encodeURIComponent(req.name),
      null
    )
      .then(() => resolve())
      .catch(reject)
  })
}

/** 拍照项逃生入口 POST /checkin/item-drafts/photo-abnormal（device_missing 携 1 张佐证；unable_to_capture/camera_broken 拍不了照，无 file_ids 直接上报；shoot_* 拍摄时空信息可选） */
export function apiItemDraftPhotoAbnormal(req: { task_id: string; point_id: string; name: string; file_ids?: string[]; note: string; exception_type: 'device_missing' | 'unable_to_capture' | 'camera_broken' | 'ai_failed'; shoot_lng?: number; shoot_lat?: number; shoot_at?: string }): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    httpPost('/checkin/item-drafts/photo-abnormal', req as unknown as Record<string, any>)
      .then(() => resolve())
      .catch(reject)
  })
}

/** 打卡逐项 AI 结论 GET /checkins/:id/items（本人记录；AI 审核异步，提交后延迟轮询用） */
export function apiCheckinItems(checkinId: string): Promise<CheckinItemAI[]> {
  return new Promise<CheckinItemAI[]>((resolve, reject) => {
    httpGet<any[]>('/checkins/' + checkinId + '/items')
      .then((d) => {
        if (d == null) {
          reject(new Error('打卡逐项结论响应异常'))
          return
        }
        resolve(
          d.map((it) => {
            const res: string = it.result ?? ''
            return {
              name: it.name ?? '',
              result: res,
              ai_verdict: it.ai_verdict ?? '',
              ai_reason: it.ai_reason ?? '',
              note: it.note ?? '',
              exception_type: it.exception_type ?? '',
              photo_urls: it.photo_urls ?? [],
              tags: it.tags ?? [],
              abnormal_tags: it.abnormal_tags ?? []
            }
          })
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
