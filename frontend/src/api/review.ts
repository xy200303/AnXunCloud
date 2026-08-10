// 记录审核接口（接口文档 §2.15）：页面已并入巡检记录（v20），列表走 checkins 检索接口
import { request } from '@/utils/request'

export function passReview(id: string) {
  return request<null>({ url: `/inspection/review/${id}/pass`, method: 'post' })
}

export function rejectReview(id: string, reason: string) {
  return request<null>({ url: `/inspection/review/${id}/reject`, method: 'post', data: { reason } })
}

// 撤销审核：pass/rejected → pending，记录退回待审核队列重新审核
export function reopenReview(id: string) {
  return request<null>({ url: `/inspection/review/${id}/reopen`, method: 'post' })
}

// 批量通过：仅 pending 记录被更新，skipped 为状态已变化被跳过的数量
export function batchPassReview(ids: string[]) {
  return request<{ passed: number; skipped: number }>({ url: '/inspection/review/batch-pass', method: 'post', data: { ids } })
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
