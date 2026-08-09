// 记录审核接口（接口文档 §2.15）
import { request, type PageResult } from '@/utils/request'
import type { CheckinItem } from './biz-types'

export interface ReviewQuery {
  page?: number
  page_size?: number
  audit_status?: string // pending / pass,rejected / pass / rejected
  community_id?: string
  inspector_id?: string
  start_time?: string
  end_time?: string
}

export function listReviewRecords(params: ReviewQuery) {
  return request<PageResult<CheckinItem>>({ url: '/inspection/review/records', method: 'get', params })
}

export function passReview(id: string) {
  return request<null>({ url: `/inspection/review/${id}/pass`, method: 'post' })
}

export function rejectReview(id: string, reason: string) {
  return request<null>({ url: `/inspection/review/${id}/reject`, method: 'post', data: { reason } })
}

export interface SpotcheckBody {
  community_id?: string
  inspector_id?: string
  start_time?: string
  end_time?: string
  mode: 'random' | 'full'
  ratio?: number
  handler: 'manual' | 'ai'
}

export function spotcheck(data: SpotcheckBody) {
  return request<{ picked: number }>({ url: '/inspection/review/spotcheck', method: 'post', data })
}
