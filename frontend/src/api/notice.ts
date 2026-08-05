// 通知公告接口（接口文档 §2.8）
import { request, type PageResult } from '@/utils/request'
import type { NoticeItem } from './biz-types'

export function listNotices(params: { page?: number; page_size?: number; title?: string; status?: number | '' }) {
  return request<PageResult<NoticeItem>>({ url: '/system/notices', method: 'get', params })
}

export function createNotice(data: { title: string; content: string; status?: number }) {
  return request<{ id: string }>({ url: '/system/notices', method: 'post', data })
}

export function updateNotice(id: string, data: { title?: string; content?: string; status?: number }) {
  return request<null>({ url: `/system/notices/${id}`, method: 'put', data })
}

export function deleteNotice(id: string) {
  return request<null>({ url: `/system/notices/${id}`, method: 'delete' })
}
