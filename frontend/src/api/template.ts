// 检查项模板接口（v10 检查项与记录审核）
import { request, type PageResult } from '@/utils/request'
import type { TemplateItem, TemplateForm, TemplatePurpose, TemplateStatus, PhotoRequired, CheckItemValueType, CheckItemEnumOption } from './biz-types'

export interface TemplateQuery {
  page?: number
  page_size?: number
  name?: string
  point_type?: string
  purpose?: TemplatePurpose | ''
  status?: TemplateStatus | ''
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

export interface TemplateItemRow {
  id: string
  code: string
  name: string
  standard: string
  standard_no?: string
  value_type: CheckItemValueType
  unit?: string
  value_min?: number | null
  value_max?: number | null
  enum_options?: CheckItemEnumOption[]
  photo_required: PhotoRequired
  note_required: boolean
  ai_enabled: boolean
  ai_prompt_kind: string
  ai_hint?: string | null
  tags?: string[]
  guide?: string | null
  sort: number
  created_at: string
}

export interface TemplateItemForm {
  code: string
  name: string
  standard?: string
  standard_no?: string
  value_type: CheckItemValueType
  unit?: string
  value_min?: number | null
  value_max?: number | null
  enum_options?: CheckItemEnumOption[]
  photo_required?: PhotoRequired
  note_required?: boolean
  ai_enabled: boolean
  ai_prompt_kind: string
  ai_hint?: string
  tags?: string[]
  guide?: string
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

export interface TemplateImportRow {
  row: number
  point_type: string
  template_name: string
  purpose: string
  item_code: string
  item_name: string
  value_type: string
  standard: string
  photo_required: string
  unit: string
  value_min: string
  value_max: string
  enum_options: string
  note_required: string
  ai_prompt_kind: string
  ai_enabled: string
  tags: string
  guide: string
  standard_no: string
  ai_hint: string
}

export interface TemplateImportResultRow extends TemplateImportRow {
  valid: boolean
  errors: { cell: string; message: string }[]
  template_id?: string
}

export function previewTemplateImport(file: File) {
  const data = new FormData()
  data.append('file', file)
  return request<{ rows: TemplateImportResultRow[]; valid: number; invalid: number }>({ url: '/inspection/templates/import/preview', method: 'post', data })
}

export function applyTemplateImport(rows: TemplateImportRow[]) {
  return request<{ created_templates: number; created_items: number; failed: TemplateImportResultRow[]; template_id: string }>({ url: '/inspection/templates/import/apply', method: 'post', data: { rows } })
}
