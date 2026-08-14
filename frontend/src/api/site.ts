// 品牌官网管理：页面配置 + 下载渠道发布物
import { request } from '@/utils/request'

export interface SiteConfigItem {
  id: string
  key: string
  name: string
  value: string
  remark: string
}

export interface AppRelease {
  id: string
  platform: string // android / harmony / ios / wechat_mp
  version: string
  file_key: string
  name: string
  size: number
  note: string
  created_at: string
}

export function getSiteConfig() {
  return request<SiteConfigItem[]>({ url: '/system/site/config', method: 'get' })
}

export function saveSiteConfig(values: Record<string, string>) {
  return request({ url: '/system/site/config', method: 'put', data: { values } })
}

export function listReleases() {
  return request<AppRelease[]>({ url: '/system/site/releases', method: 'get' })
}

export function uploadRelease(form: FormData, onProgress?: (percent: number) => void) {
  return request<AppRelease>({
    url: '/system/site/releases',
    method: 'post',
    data: form,
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 10 * 60 * 1000, // 安装包较大，放宽到 10 分钟
    onUploadProgress: (e) => {
      if (onProgress && e.total) onProgress(Math.round((e.loaded / e.total) * 100))
    }
  })
}

export function deleteRelease(id: string) {
  return request({ url: `/system/site/releases/${id}`, method: 'delete' })
}
