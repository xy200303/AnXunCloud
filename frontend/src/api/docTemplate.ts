import { request } from '@/utils/request'

export interface SignSlot {
  code: string
  label: string
  step_index: number
  required?: boolean
}

export interface DocTemplate {
  id?: string
  code: string
  name: string
  doc_kind: 'monthly_report' | 'acceptance' | 'maintenance_ledger'
  engine: 'ooxml' | 'xlsx_template'
  format: 'pdf' | 'docx' | 'xlsx'
  layout: { mother: { profile: string }; sign_slots?: SignSlot[] }
  style_pack_id: string | null
  skeleton_file_id: string | null
  volume_code: string
  system_filter: string
  status: 'draft' | 'enabled'
  source: 'platform' | 'tenant'
  published_at?: string | null
}

export const listDocTemplates = () => request<DocTemplate[]>({ url: '/doc-templates', params: { scope: 'tenant' } })
export const saveDocTemplate = (code: string, data: Omit<DocTemplate, 'source' | 'published_at'>) => request<null>({ url: `/doc-templates/${code}`, method: 'put', params: { scope: 'tenant' }, data })

export interface StylePack {
  id: string
  code: string
  name: string
  config: { font_family: string; title_size: number; heading_size: number; body_size: number; accent_color: string }
  status: string
}

export const listStylePacks = () => request<StylePack[]>({ url: '/doc-templates/style-packs' })

export interface OutputColumn {
  item_code: string
}

export interface TemplateOutputBinding {
  template_id: string
  sort: number
  columns: OutputColumn[]
  row_mode: 'date' | 'point' | 'grid' | 'equipment'
}

export const listTemplateOutputBindings = (docTemplateId: string) => request<TemplateOutputBinding[]>({ url: `/doc-templates/${docTemplateId}/bindings` })
export const saveTemplateOutputBindings = (docTemplateId: string, data: TemplateOutputBinding[]) => request<null>({ url: `/doc-templates/${docTemplateId}/bindings`, method: 'put', data })
