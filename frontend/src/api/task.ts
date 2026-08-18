// 任务监控接口（接口文档 §2.13）
import { request, type PageResult } from '@/utils/request'
import type { TaskItem, TaskDetail } from './biz-types'

export interface TaskQuery {
  page?: number
  page_size?: number
  community_id?: string
  inspector_id?: string
  plan_id?: string
  patrol_type?: string
  task_date?: string
  start_date?: string
  end_date?: string
  status?: string
  filter?: '' | 'missing' | 'abnormal' | 'suspect'
}

export function listTasks(params: TaskQuery) {
  return request<PageResult<TaskItem>>({ url: '/inspection/tasks', method: 'get', params })
}

export function getTaskDetail(id: string) {
  return request<TaskDetail>({ url: `/inspection/tasks/${id}/detail`, method: 'get' })
}

// 手动触发任务生成（缺省为今天，幂等）
export function generateTasks(date?: string) {
  return request<{ date: string; created: number; eligible_plans: number }>({
    url: '/inspection/tasks/generate',
    method: 'post',
    data: date ? { date } : {}
  })
}
