// 检查项模板接口（v10 检查项与记录审核）
import { request, type PageResult } from '@/utils/request'
import type { TemplateItem, TemplateForm } from './biz-types'

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
