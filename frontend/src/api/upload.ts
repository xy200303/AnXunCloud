// 管理端图片上传（手写签名、公章等）
// 后端上传接口并行开发中：未就绪时调用方需优雅降级（不阻断页面）
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

// 本地存储模式下静态目录为 /uploads（后端 r.Static("/uploads", ...)）。
export function fileUrl(fileKey: string) {
  return fileKey ? `/uploads/${fileKey}` : ''
}

// 统一文件层地址附带登录态：/api/files 需鉴权，而 <img>/el-image/直链下载无法带请求头，
// 由 AuthAny 中间件的 ?token= 查询参数兜底。仅 /api/files/ 形式的地址需要拼接，/uploads/* 内容图无需处理。
export function withFileToken(url: string) {
  if (!url || !url.includes('/api/files/')) return url
  const t = getToken()
  if (!t) return url
  return url + (url.includes('?') ? '&' : '?') + 'token=' + encodeURIComponent(t)
}
