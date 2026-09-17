/**
 * 消息 / 公告 / 推送设备域。
 * 对齐 MPService.Messages / NoticeService.Published；推送为 uniPush 2.0（与 sys_message 同源下发）。
 */

import { httpDelete, httpGet, httpPost, httpPut } from '@/services/request'
import { buildQuery } from './common'

/** 消息类型：report 月报 / checkin_audit 打卡审核 / announcement 公告 / 其他按系统消息展示 */
export type MessageItem = {
  id: number
  type: string
  title: string
  content: string
  biz_id: string | null
  is_read: boolean
  created_at: string
}

export type MessagesResult = {
  unread_count: number
  list: MessageItem[]
  total: number
  page: number
  page_size: number
}

export type NoticeAttachment = {
  name: string
  url: string
}

export type AnnouncementItem = {
  id: string
  title: string
  content: string
  publish_at: string
  attachments?: NoticeAttachment[]
}

export type AnnouncementsPage = {
  list: AnnouncementItem[]
  total: number
  page: number
  page_size: number
}

/** 公告详情（GET /announcements/:id，仅已发布可见） */
export type AnnouncementDetail = {
  id: string
  title: string
  content: string
  status: number
  attachments: NoticeAttachment[]
  publish_at: string
  created_by: string
  created_at: string
}

/** 推送设备绑定请求体（POST /push/device） */
export type PushDeviceBindPayload = {
  /** uniPush CID（uni.getPushClientId 获取） */
  cid: string
  /** android / ios */
  platform: string
}

/** 绑定推送设备 POST /push/device（同 cid 重复绑定会改绑到当前用户；登录态） */
export function apiBindPushDevice(cid: string, platform: string): Promise<null> {
  const body: PushDeviceBindPayload = { cid: cid, platform: platform }
  return httpPost<null>('/push/device', body as unknown as Record<string, any>, true)
}

/** 解绑推送设备 DELETE /push/device（登出清 token 前调用） */
export function apiUnbindPushDevice(cid: string): Promise<null> {
  return httpDelete<null>('/push/device', { cid: cid }, true)
}

/** 消息列表 GET /messages（type/is_read 可选过滤；返回含 unread_count） */
export function apiMessages(page: number, pageSize: number, type?: string, isRead?: string): Promise<MessagesResult> {
  const path = '/messages' + buildQuery({ page: page, page_size: pageSize, type: type, is_read: isRead })
  return new Promise<MessagesResult>((resolve, reject) => {
    httpGet<any>(path)
      .then((d) => {
        resolve({
          unread_count: d?.unread_count ?? 0,
          list: (d?.list ?? []) as MessageItem[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 标记已读 PUT /messages/:id/read（id=0 全部已读） */
export function apiMarkMessageRead(id: number | string): Promise<null> {
  return httpPut<null>('/messages/' + id + '/read', null, true)
}

/** 公告列表 GET /announcements（已发布公告分页） */
export function apiAnnouncements(page: number, pageSize: number): Promise<AnnouncementsPage> {
  return new Promise<AnnouncementsPage>((resolve, reject) => {
    httpGet<any>('/announcements' + buildQuery({ page: page, page_size: pageSize }))
      .then((d) => {
        resolve({
          list: (d?.list ?? []) as AnnouncementItem[],
          total: d?.total ?? 0,
          page: d?.page ?? page,
          page_size: d?.page_size ?? pageSize
        })
      })
      .catch(reject)
  })
}

/** 公告详情 GET /announcements/:id（仅已发布可见，404 由信封错误上抛） */
export function apiAnnouncementDetail(id: string): Promise<AnnouncementDetail> {
  return new Promise<AnnouncementDetail>((resolve, reject) => {
    httpGet<any>('/announcements/' + encodeURIComponent(id))
      .then((d) => {
        if (d == null) {
          reject(new Error('公告详情响应异常'))
          return
        }
        resolve({
          id: d.id ?? '',
          title: d.title ?? '',
          content: d.content ?? '',
          status: d.status ?? 0,
          attachments: (d.attachments ?? []) as NoticeAttachment[],
          publish_at: d.publish_at ?? '',
          created_by: d.created_by ?? '',
          created_at: d.created_at ?? ''
        })
      })
      .catch(reject)
  })
}
