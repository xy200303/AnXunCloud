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
  credential?: string
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

// 点位批量导入结果
export interface PointImportResult {
  total: number
  success_count: number
  fail_count: number
  fail_details: { row: number; name: string; reason: string }[]
}

export function importPoints(file: File) {
  const form = new FormData()
  form.append('file', file)
  return request<PointImportResult>({
    url: '/inspection/points/import',
    method: 'post',
    data: form,
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 60000
  })
}

// 批量建点助手（接口文档 §2.17）：楼栋 × 单元 × 楼层 × 每层数量展开，同楼栋同名跳过（幂等）
export interface PointBatchForm {
  community_id: string
  building_ids?: string[]
  unit_from?: number // 单元起（缺省 1，单单元楼栋保持 1 至 1）
  unit_to?: number
  floor_from?: number // 负数 = 地下层（-1 渲染 B1）
  floor_to?: number
  per_floor?: number
  name_pattern: string // 占位符 {building}/{unit}/{floor}/{seq}
  type: string
  credential?: string
  template_id?: string | null
  longitude?: number | null
  latitude?: number | null
}

export interface PointBatchResult {
  created: number
  skipped: { name: string; reason: string }[]
}

export function batchCreatePoints(data: PointBatchForm) {
  return request<PointBatchResult>({ url: '/inspection/points/batch', method: 'post', data })
}
