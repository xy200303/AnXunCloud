// 消息通知接口（契约见任务书；后端并行开发中，未上线时静默降级不弹错）
import axios from 'axios'
import { getToken } from '@/utils/auth'

export interface MessageItem {
  id: string
  title: string
  content: string
  type: string
  biz_id: string | null
  is_read: boolean
  created_at: string
}

export interface MessageListResult {
  list: MessageItem[]
  total: number
  unread_count: number
}

function bareGet(path: string, params?: Record<string, any>) {
  return axios.get(`/api/admin${path}`, {
    params,
    headers: { Authorization: `Bearer ${getToken()}` }
  })
}

export async function listMessages(params?: { is_read?: number | ''; page?: number; page_size?: number }): Promise<MessageListResult> {
  const resp = await bareGet('/system/messages', params)
  if (resp.data?.code !== 0) throw new Error(resp.data?.message || '获取消息失败')
  return resp.data.data
}

// 标记已读；id 传 '0' 表示全部已读
export async function markMessageRead(id: string): Promise<void> {
  const resp = await axios.put(`/api/admin/system/messages/${id}/read`, null, {
    headers: { Authorization: `Bearer ${getToken()}` }
  })
  if (resp.data?.code !== 0) throw new Error(resp.data?.message || '操作失败')
}
