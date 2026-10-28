import { request } from '@/utils/request'
import type { AssistantMessage } from '@/api/assistant'

export interface AgentTool {
  name: string
  title: string
  description: string
  read_only: boolean
  enabled: boolean
  input_schema: Record<string, unknown>
}

export interface AgentCall {
  id: string
  conversation_id: string
  tool_code: string
  arguments: Record<string, unknown>
  result: Record<string, unknown>
  status: 'success' | 'failed'
  error_message: string
  created_at: string
}

export const getAgentStatus = () => request<{ available: boolean; reason: string }>({ url: '/ai/agent/status' })
export const listAgentTools = () => request<AgentTool[]>({ url: '/ai/agent/tools' })
export const setAgentToolEnabled = (code: string, enabled: boolean) => request<null>({ url: `/ai/agent/tools/${code}`, method: 'put', data: { code, enabled } })
export const sendAgentMessage = (conversationId: string, content: string) => request<{ conversation_id: string; message: AssistantMessage }>({ url: '/ai/agent/chat', method: 'post', data: { conversation_id: conversationId, content }, timeout: 300000 })
export const listAgentCalls = (conversationId: string) => request<AgentCall[]>({ url: `/ai/agent/conversations/${conversationId}/calls` })
