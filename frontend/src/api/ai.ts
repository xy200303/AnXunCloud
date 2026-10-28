import { request } from '@/utils/request'

export type AIScope = 'platform' | 'tenant'
export type AIStatus = 'enabled' | 'disabled'
export type AICapability = 'vision' | 'text' | 'tools'

export interface AIProvider {
  code: string
  name: string
  protocol: 'openai_chat' | 'openai_responses' | 'gemini' | 'claude'
  base_url: string
  credential_ref: string
  status: AIStatus
  source: AIScope
}

export interface AIModel {
  provider_code: string
  model_id: string
  display_name: string
  context_window: number
  max_output: number
  capabilities: AICapability[]
  status: AIStatus
  source: AIScope
}

export interface AITaskBinding {
  task_type: string
  provider_code: string
  model_id: string
  timeout_seconds: number
  max_retries: number
  custom_prompt: string
  status: AIStatus
  source: AIScope
}

export interface AITask {
  task_type: string
  name: string
  required_capabilities: AICapability[]
}

export const listAIProviders = (scope: AIScope) => request<AIProvider[]>({ url: '/ai/providers', params: { scope } })
export const listAIModels = (scope: AIScope) => request<AIModel[]>({ url: '/ai/models', params: { scope } })
export const listAIBindings = (scope: AIScope) => request<AITaskBinding[]>({ url: '/ai/bindings', params: { scope } })
export const listAITasks = () => request<AITask[]>({ url: '/ai/task-catalog', params: { scope: 'tenant' } })

export function saveAIProvider(code: string, data: Omit<AIProvider, 'source'>, scope: AIScope) {
  return request<null>({ url: `/ai/providers/${code}`, method: 'put', params: { scope }, data })
}

export function saveAIModel(providerCode: string, modelId: string, data: Omit<AIModel, 'source' | 'provider_code' | 'model_id'>, scope: AIScope) {
  return request<null>({ url: `/ai/models/${providerCode}/${modelId}`, method: 'put', params: { scope }, data })
}

export function saveAIBinding(taskType: string, data: Omit<AITaskBinding, 'source' | 'task_type'>, scope: AIScope) {
  return request<null>({ url: `/ai/bindings/${taskType}`, method: 'put', params: { scope }, data })
}
