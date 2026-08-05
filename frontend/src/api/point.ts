// 点位管理接口（接口文档 §2.11）
import { request, type PageResult } from '@/utils/request'
import type { PointItem, PointForm } from './biz-types'

export interface PointQuery {
  page?: number
  page_size?: number
  community_id?: string
  building_id?: string
  name?: string
  type?: string
  checkin_mode?: string
  status?: number | ''
}

export function listPoints(params: PointQuery) {
  return request<PageResult<PointItem>>({ url: '/inspection/points', method: 'get', params })
}

export function getPoint(id: string) {
  return request<PointItem & { referenced_plans?: { id: string; name: string }[] }>({
    url: `/inspection/points/${id}`,
    method: 'get'
  })
}

export function createPoint(data: PointForm) {
  return request<{ id: string; qrcode_no: string }>({ url: '/inspection/points', method: 'post', data })
}

export function updatePoint(id: string, data: PointForm) {
  return request<null>({ url: `/inspection/points/${id}`, method: 'put', data })
}

export function deletePoint(id: string) {
  return request<null>({ url: `/inspection/points/${id}`, method: 'delete' })
}

// 批量生成二维码：返回 A4 排版 PDF 下载信息
export function generateQrcodes(pointIds: string[], withTitle = true) {
  return request<{ file_url: string; file_name: string; expire_at: string }>({
    url: '/inspection/points/qrcodes',
    method: 'post',
    data: { point_ids: pointIds, with_title: withTitle },
    timeout: 60000
  })
}
