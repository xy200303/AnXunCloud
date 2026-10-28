import { request } from '@/utils/request'

export const FLOW_CODES = ['checkin_review', 'maint_review', 'workorder_review', 'report_review', 'acceptance_review'] as const
export type FlowCode = typeof FLOW_CODES[number]

export interface ReviewFlowStep {
  role_code: string
  name: string
  mode?: 'any' | 'all'
  kind?: '' | 'ai'
  on_pass?: string
  on_abnormal?: string
  on_review?: string
}

export interface ReviewFlowView {
  flow_code: FlowCode
  steps: ReviewFlowStep[]
  source: 'project' | 'tenant' | 'platform' | 'default'
}

export function getFlow(code: FlowCode, scope: 'tenant' | 'platform') {
  const base = scope === 'platform' ? '/system/review-flow-templates' : '/system/review-flows'
  return request<ReviewFlowView>({ url: `${base}/${code}`, method: 'get' })
}

export function saveFlow(code: FlowCode, steps: ReviewFlowStep[], scope: 'tenant' | 'platform') {
  const base = scope === 'platform' ? '/system/review-flow-templates' : '/system/review-flows'
  return request<null>({ url: `${base}/${code}`, method: 'put', data: { steps } })
}
