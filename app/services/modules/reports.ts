/**
 * 月报域：报告列表/详情、动态审核步骤签批、PDF 预览、签字候选人与手动生成。
 * 对齐后端 ReportService List/Detail/sign-*。
 */

import { httpGet, httpPost } from '@/services/request'
import { getBaseUrl, getPublicOrigin } from '@/services/request'
import { getAccessToken } from '@/utils/storage'
import { buildQuery } from './common'
import { toastErr } from '@/utils/ui'

/** 报告状态：pending_review/approved */
export type ReportStatus = string

/** 报告列表项（对齐 ReportService.List 返回） */
export type ReportListItem = {
  id: string
  community_id: string
  community_name: string
  /** YYYY-MM */
  period: string
  title: string
  status: ReportStatus
	review_steps: ReportReviewStep[]
	review_step: number
	review_current_ids: string[]
  has_file: boolean
  created_at: string
  updated_at: string
}

export type ReportListPage = {
  list: ReportListItem[]
  total: number
  page: number
  page_size: number
}

/** 动态审核步骤候选人明细 */
export type ReportSigner = {
  user_id: string
  name: string
  signed: boolean
  signed_at?: string
  signature_url?: string | null
}
export type ReportReviewStep = { slot: string; name: string; mode: 'any' | 'all'; candidate_ids: string[]; signed: any[]; users?: ReportSigner[] }

/** 报告详情（对齐 ReportService.Detail；stats 为 JSONMap 快照，字段按 buildStats） */
export type ReportDetail = {
  id: string
  community_id: string
  community_name: string
  period: string
  title: string
  status: ReportStatus
  stats: Record<string, any>
  inspector_ids: string[]
	review_steps: ReportReviewStep[]
	review_step: number
	review_current_ids: string[]
  reject_reason: string
  /** 作废留痕（status=voided 时有值） */
  void_reason: string
  voided_by: string | null
  voided_by_name: string
  voided_at: string | null
  file_id: string
  file_url: string | null
  created_at: string
  updated_at: string
}

/** 动态审核步骤签批请求（action=approve/reject，驳回 reason 必填） */
export type ReportSignReq = {
  action: 'approve' | 'reject'
  remark?: string
  reason?: string
  /** 未配置签名时随请求提交的一次性签名（须本人 scene=signature 上传） */
  signature_file_id?: string
}

export type ReportSignCandidate = {
  id: string
  name: string
  has_signature: boolean
}

/** 报告列表 GET /reports（pendingMine=true 只看待我签；signedMine: '1'=我签过的+已归档 'doing'=我签过未归档；status 按状态过滤） */
export function apiReports(page: number, pageSize: number, pendingMine: boolean, status?: string, signedMine?: string): Promise<ReportListPage> {
  const path = '/reports' + buildQuery({
    page: page,
    page_size: pageSize,
    pending_mine: pendingMine ? 1 : '',
    signed_mine: signedMine,
    status: status
  })
  return new Promise<ReportListPage>((resolve, reject) => {
    httpGet<any>(path)
      .then((d) => {
        resolve({
          list: (d?.list ?? []) as ReportListItem[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 报告详情 GET /reports/:id（stats 全量 + 动态审核明细） */
export function apiReportDetail(id: string): Promise<ReportDetail> {
  return new Promise<ReportDetail>((resolve, reject) => {
    httpGet<any>('/reports/' + id)
      .then((d) => {
        if (d == null) {
          reject(new Error('报告详情响应异常'))
          return
        }
        resolve({
          id: d.id ?? '',
          community_id: d.community_id ?? '',
          community_name: d.community_name ?? '',
          period: d.period ?? '',
          title: d.title ?? '',
          status: d.status ?? '',
          stats: (d.stats ?? {}) as Record<string, any>,
          inspector_ids: (d.inspector_ids ?? []).map((x: any) => String(x)),
          review_steps: (d.review_steps ?? []) as ReportReviewStep[],
          review_step: Number(d.review_step ?? 0),
          review_current_ids: (d.review_current_ids ?? []).map((x: any) => String(x)),
          reject_reason: d.reject_reason ?? '',
          void_reason: d.void_reason ?? '',
          voided_by: d.voided_by ?? null,
          voided_by_name: d.voided_by_name ?? '',
          voided_at: d.voided_at ?? null,
          file_id: d.file_id ?? '',
          file_url: d.file_url ?? null,
          created_at: d.created_at ?? '',
          updated_at: d.updated_at ?? ''
        })
      })
      .catch(reject)
  })
}

/** 动态审核步骤签字 POST /reports/:id/sign-step/:step */
export function apiSignStep(id: string, step: number, req: ReportSignReq): Promise<{ status: string; review_step: number }> {
  return new Promise<{ status: string; review_step: number }>((resolve, reject) => {
    httpPost<any>('/reports/' + id + '/sign-step/' + step, req as unknown as Record<string, any>, true)
      .then((d) => {
        resolve({ status: d?.status ?? '', review_step: Number(d?.review_step ?? step) })
      })
      .catch(reject)
  })
}

/** 签发报告 PDF 预览 ticket POST /reports/:id/pdf-ticket（web-view 无法带登录头，凭 ticket 走限时公开通道） */
export function apiReportPdfTicket(id: string): Promise<string> {
  return new Promise<string>((resolve, reject) => {
    httpPost<any>('/reports/' + id + '/pdf-ticket', null, true)
      .then((d) => {
        if (d == null || d.ticket == null || d.ticket == '') {
          reject(new Error('预览凭证签发失败'))
          return
        }
        resolve(String(d.ticket))
      })
      .catch(reject)
  })
}

/**
 * 查看报告 PDF。
 * - App 端：签 ticket → web-view 页内嵌 pdf.js 直接渲染（不依赖系统 PDF 阅读器）；
 * - 小程序端：web-view 需业务域名白名单，暂用下载 + 微信内置 openDocument。
 */
export function openReportPdf(id: string) {
  // #ifdef APP-PLUS
  uni.showLoading({ title: '正在加载报告…', mask: true })
  apiReportPdfTicket(id)
    .then((ticket) => {
      uni.hideLoading()
      const file = '/api/public/report-pdf/' + encodeURIComponent(id) + '?ticket=' + encodeURIComponent(ticket)
      const viewer = getPublicOrigin() + '/pdfjs/viewer.html?file=' + encodeURIComponent(file)
      // file 绝对地址传给预览页：导航栏「打开/分享」直接下载后用系统面板调起 WPS/QQ/微信
      const fileUrl = getPublicOrigin() + file
      uni.navigateTo({
        url: '/pages/reports/pdf?src=' + encodeURIComponent(viewer) + '&file=' + encodeURIComponent(fileUrl)
      })
    })
    .catch((e: Error) => {
      uni.hideLoading()
      toastErr(e)
    })
  // #endif
  // #ifndef APP-PLUS
  const token = getAccessToken()
  uni.showLoading({ title: '正在加载报告…', mask: true })
  uni.downloadFile({
    url: getBaseUrl() + '/reports/' + encodeURIComponent(id) + '/pdf',
    header: { Authorization: 'Bearer ' + token },
    success: (res) => {
      uni.hideLoading()
      if (res.statusCode != 200) {
        uni.showToast({ title: '报告加载失败（' + res.statusCode + '）', icon: 'none' })
        return
      }
      uni.openDocument({
        filePath: res.tempFilePath,
        fileType: 'pdf',
        showMenu: true, // 右上角菜单：可转发/保存
        fail: (e) => {
          uni.showToast({ title: '打开失败：' + (e.errMsg || '请安装 PDF 阅读器'), icon: 'none' })
        }
      })
    },
    fail: (e) => {
      uni.hideLoading()
      uni.showToast({ title: '下载失败：' + (e.errMsg || ''), icon: 'none' })
    }
  })
  // #endif
}

/** 生成报告时读取审核路径及按审核级别筛选的可选签字人。筛选由后端完成。 */
export function apiReportSignCandidates(communityId: string, patrolType?: string, period?: string): Promise<{
  steps: { index: number; slot: string; name: string; mode: 'any' | 'all'; users: ReportSignCandidate[]; default_candidate_ids: string[] }[]
}> {
  return new Promise((resolve, reject) => {
    httpGet<{
      steps: { index: number; slot: string; name: string; mode: 'any' | 'all'; users: ReportSignCandidate[]; default_candidate_ids: string[] }[]
    }>('/reports/sign-candidates' + buildQuery({ community_id: communityId, patrol_type: patrolType, period: period }))
      .then((d) => resolve(d ?? { steps: [] }))
      .catch(reject)
  })
}

/** App 管理端手动生成报告 POST /reports/generate（签字人可按候选名单调整） */
export function apiReportGenerate(body: {
  community_id: string
  period: string
  patrol_type?: string
  detail_mode?: string
  sign_steps?: { slot: string; candidate_ids: string[] }[]
}): Promise<{ id: string; title: string; status: string; regenerated: boolean }> {
  return new Promise((resolve, reject) => {
    httpPost<{ id: string; title: string; status: string; regenerated: boolean }>('/reports/generate', body as unknown as Record<string, any>)
      .then((d) => {
        if (d == null) {
          reject(new Error('生成响应异常'))
          return
        }
        resolve(d)
      })
      .catch(reject)
  })
}
