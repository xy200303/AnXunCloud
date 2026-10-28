import { request } from '@/utils/request'

export interface AssistantConversation {
  id: string
  title: string
  created_at: string
  updated_at: string
}

export interface AssistantMessage {
  id: string
  conversation_id: string
  role: 'user' | 'assistant'
  content: string
  provider_code: string
  model_id: string
  prompt_tokens: number
  completion_tokens: number
  status: 'ready' | 'failed'
  error_message: string
  created_at: string
}

export const getAssistantStatus = () => request<{ available: boolean; reason: string }>({ url: '/ai/assistant/status' })
export const listAssistantConversations = () => request<AssistantConversation[]>({ url: '/ai/assistant/conversations' })
export const listAssistantMessages = (id: string) => request<AssistantMessage[]>({ url: `/ai/assistant/conversations/${id}/messages` })
export const sendAssistantMessage = (conversationId: string, content: string) => request<{ conversation_id: string; message: AssistantMessage }>({ url: '/ai/assistant/chat', method: 'post', data: { conversation_id: conversationId, content }, timeout: 300000 })
