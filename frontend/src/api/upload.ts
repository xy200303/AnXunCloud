// 管理端文件上传（头像、手写签名、公章、公告附件）。
import { request } from '@/utils/request'
import { getToken } from '@/utils/auth'

export interface UploadResult {
  file_id: string
  url: string
}

// scene：signature（手写签名）/ seal（公章）等，与后端 upload_file.scene 对应
export function uploadImage(file: File, scene: string) {
  const form = new FormData()
  form.append('scene', scene)
  form.append('file', file)
  return request<UploadResult>({
    url: '/system/upload',
    method: 'post',
    data: form,
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 30000
  })
}

// 统一文件层地址附带登录态：图片组件无法直接附加请求头，因此通过 token 查询参数鉴权。
export function withFileToken(url: string) {
  if (!url || !url.includes('/api/files/')) return url
  const t = getToken()
  if (!t) return url
  return url + (url.includes('?') ? '&' : '?') + 'token=' + encodeURIComponent(t)
}
