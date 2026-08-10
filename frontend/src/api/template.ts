// 检查项模板接口（v10 检查项与记录审核）
import { request, type PageResult } from '@/utils/request'
import type { TemplateItem, TemplateForm, PhotoRequired } from './biz-types'

export interface TemplateQuery {
  page?: number
  page_size?: number
  name?: string
  point_type?: string
  status?: number | ''
}

export function listTemplates(params: TemplateQuery) {
  return request<PageResult<TemplateItem>>({ url: '/inspection/templates', method: 'get', params })
}

export function getTemplate(id: string) {
  return request<TemplateItem>({ url: `/inspection/templates/${id}`, method: 'get' })
}

export function createTemplate(data: TemplateForm) {
  return request<{ id: string }>({ url: '/inspection/templates', method: 'post', data })
}

export function updateTemplate(id: string, data: TemplateForm) {
  return request<null>({ url: `/inspection/templates/${id}`, method: 'put', data })
}

export function deleteTemplate(id: string) {
  return request<null>({ url: `/inspection/templates/${id}`, method: 'delete' })
}

// ===== 项级粒度：检查项按行增删改 =====

// 项级接口返回的检查项行（含 id/sort，区别于模板内嵌 items 快照）
export interface TemplateItemRow {
  id: string
  name: string
  requirement: string | null
  required: boolean
  photo_required: PhotoRequired
  sort: number
  created_at: string
}

// 新增/修改单个检查项；sort 缺省时新增追加到末尾、修改保持不变
export interface TemplateItemForm {
  name: string
  requirement?: string
  required: boolean
  photo_required?: PhotoRequired
  sort?: number
}

export function listTemplateItems(templateId: string) {
  return request<TemplateItemRow[]>({ url: `/inspection/templates/${templateId}/items`, method: 'get' })
}

export function createTemplateItem(templateId: string, data: TemplateItemForm) {
  return request<{ id: string }>({ url: `/inspection/templates/${templateId}/items`, method: 'post', data })
}

export function updateTemplateItem(templateId: string, itemId: string, data: TemplateItemForm) {
  return request<null>({ url: `/inspection/templates/${templateId}/items/${itemId}`, method: 'put', data })
}

export function deleteTemplateItem(templateId: string, itemId: string) {
  return request<null>({ url: `/inspection/templates/${templateId}/items/${itemId}`, method: 'delete' })
}
