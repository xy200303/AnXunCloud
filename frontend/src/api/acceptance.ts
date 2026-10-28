import { request, type PageResult } from '@/utils/request'

export interface AcceptanceRow {
  code: string
  name: string
  specification: string
  unit: string
  required: boolean
  sort: number
  status: 'enabled' | 'disabled'
}

export interface AcceptanceSchema {
  code: string
  category: string
  name: string
  table_kind: 'matrix' | 'documents' | 'keys' | 'assets'
  template_id: string | null
  template_name: string
  status: 'draft' | 'enabled' | 'disabled'
  source: 'platform' | 'tenant'
  rows: AcceptanceRow[]
}

export interface AcceptanceTarget {
  id: string
  target_type: string
  ref_id: string
  building_id?: string | null
  category_code: string
  table_codes: string[]
  name: string
  location: string
  coverage_confirmed: boolean
}

export interface AcceptanceBatch {
  id: string
  name: string
  community_id: string
  ledger_type: 'institution_change' | 'unit_acceptance'
  status: 'draft' | 'inspecting' | 'reviewing' | 'handover' | 'done'
  manager_id: string
  manager_name?: string
  revision: number
  target_count: number
  covered_count: number
  problem_count: number
  period_start: string
  period_end: string
  review_flow?: { role_code: string; name: string; mode: string }[]
  review_step: number
  locked_at?: string | null
}

export interface AcceptanceRecord {
  id: string
  target_id: string
  table_code: string
  item_code: string
  category_code: string
  result: 'missing' | 'problem'
  value_text: string
  values?: Record<string, string | number>
  note: string
  photos: string[]
  revision: number
  created_by: string
}

export interface AcceptanceProblem {
  id: string
  record_id: string
  fact: string
  risk_level: 'normal' | 'major'
  safety_measure: string
  measure_approved: boolean
  accepted: boolean
  accepted_by: string
  acceptance_file_id?: string | null
}

export interface AcceptanceBatchDetail {
  batch: AcceptanceBatch & { template_snapshot: { schemas?: AcceptanceSchema[]; verify_templates?: { ID: string; Name: string; Items: { Code: string; Name: string }[] }[] } }
  targets: AcceptanceTarget[]
  members: { id: string; user_id: string; work_scope: string; category_codes: string[]; building_ids: string[]; external_name: string }[]
  records: AcceptanceRecord[]
  problems: AcceptanceProblem[]
  subjects: AcceptanceSignSubject[]
  signatures: AcceptanceSignature[]
  votes: { id: string; step_index: number; user_id: string; action: string; reason: string; created_at: string }[]
  files: AcceptanceReportFile[]
  logs: { id: string; action: string; actor_id: string; from_status: string; to_status: string; reason: string; created_at: string }[]
}

export const listAcceptanceSchemas = () => request<AcceptanceSchema[]>({ url: '/acceptance-schemas', params: { scope: 'tenant' } })
export const saveAcceptanceSchema = (code: string, data: Omit<AcceptanceSchema, 'source' | 'rows' | 'template_name'>) => request<null>({ url: `/acceptance-schemas/${code}`, method: 'put', params: { scope: 'tenant' }, data })
export const saveAcceptanceRow = (tableCode: string, code: string, data: Omit<AcceptanceRow, 'code'>) => request<null>({ url: `/acceptance-schemas/${tableCode}/rows/${code}`, method: 'put', params: { scope: 'tenant' }, data })
export const listAcceptanceBatches = (params: { page: number; page_size: number; community_id?: string; name?: string; status?: string }) => request<PageResult<AcceptanceBatch>>({ url: '/acceptance/batches', params })
export const getAcceptanceBatch = (id: string) => request<AcceptanceBatchDetail>({ url: `/acceptance/batches/${id}` })
export const createAcceptanceBatch = (data: Record<string, unknown>) => request<{ id: string }>({ url: '/acceptance/batches', method: 'post', data })
export const startAcceptanceBatch = (id: string) => request<null>({ url: `/acceptance/batches/${id}/start`, method: 'post' })
export const submitAcceptanceBatch = (id: string) => request<null>({ url: `/acceptance/batches/${id}/submit`, method: 'post' })
export const saveAcceptanceRecord = (batchId: string, data: Record<string, unknown>) => request<{ id: string }>({ url: `/acceptance/batches/${batchId}/records`, method: 'post', data })
export const confirmAcceptanceCoverage = (batchId: string, targetId: string) => request<null>({ url: `/acceptance/batches/${batchId}/targets/${targetId}/coverage`, method: 'post' })

export interface AcceptanceSignSubject {
  id?: string
  code: string
  name: string
  role_code: string
  external: boolean
  required: boolean
  sort: number
}

export interface AcceptanceSignature {
  id: string
  subject_code: string
  representative: string
  report_sha256: string
  file_id: string
  status: string
  signed_at: string
}

export interface AcceptanceReportFile {
  id: string
  volume_name: string
  stage: 'frozen' | 'archived'
  format: 'docx' | 'pdf'
  sha256: string
  status: 'running' | 'ready' | 'failed'
  error_message: string
  created_at: string
}

export const reviewAcceptanceBatch = (id: string, data: { action: 'approve' | 'reject'; reason?: string }) => request<null>({ url: `/acceptance/batches/${id}/review`, method: 'post', data })
export const signAcceptanceBatch = (id: string, data: { subject_code: string; representative?: string; file_id: string }) => request<null>({ url: `/acceptance/batches/${id}/sign`, method: 'post', data })
export const updateAcceptanceSubjects = (id: string, subjects: AcceptanceSignSubject[]) => request<null>({ url: `/acceptance/batches/${id}/sign-subjects`, method: 'put', data: { subjects } })
export const acceptAcceptanceProblem = (batchId: string, problemId: string, data: Omit<AcceptanceProblem, 'id' | 'record_id' | 'fact'>) => request<null>({ url: `/acceptance/batches/${batchId}/problems/${problemId}/acceptance`, method: 'put', data })
