import { request } from '@/utils/request'

export interface MCPTool {
  name: string
  title: string
  description: string
  read_only: boolean
  input_schema: Record<string, unknown>
}

export interface MCPToken {
  id: string
  name: string
  prefix: string
  scopes: string[]
  expires_at: string | null
  revoked_at: string | null
  last_used_at: string | null
  created_at: string
}

export interface MCPTokenIssue {
  id: string
  name: string
  token: string
  prefix: string
  scopes: string[]
  expires_at: string
}

export const listMCPTools = () => request<MCPTool[]>({ url: '/ai/mcp/tools' })
export const listMCPTokens = () => request<MCPToken[]>({ url: '/ai/mcp/tokens' })
export const issueMCPToken = (data: { name: string; scopes: string[]; days: number }) => request<MCPTokenIssue>({ url: '/ai/mcp/tokens', method: 'post', data })
export const revokeMCPToken = (id: string) => request<null>({ url: `/ai/mcp/tokens/${id}`, method: 'delete' })
