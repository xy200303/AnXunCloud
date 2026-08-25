// 问题清单接口：仅含异常（result=abnormal）打卡记录；导出用 utils/download.ts 走文件流
import { request, type PageResult } from '@/utils/request'

export interface IssuePhoto {
  item: string
  url: string
  watermarked_url: string
}

export interface IssueItem {
  id: string
  checkin_time: string
  point_id: string
  point_name: string
  building_name: string
  community_name: string
  inspector_name: string
  result: string
  remark: string
  ai_verdict: string
  ai_reason: string
  audit_status: string
  force_submit: boolean
  is_suspect: boolean
  photos: IssuePhoto[]
}

export interface IssueQuery {
  page?: number
  page_size?: number
  community_id?: string
  patrol_type?: string
  point_type?: string
  audit_status?: string
  force_submit?: boolean
  keyword?: string
  date_from?: string
  date_to?: string
}

export function listIssues(params: IssueQuery) {
  return request<PageResult<IssueItem>>({ url: '/inspection/issues', method: 'get', params })
}
