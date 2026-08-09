// 管理端图片上传（手写签名、公章等）
// 后端上传接口并行开发中：未就绪时调用方需优雅降级（不阻断页面）
import { request } from '@/utils/request'

export interface UploadResult {
  file_key: string
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

// 本地存储模式下静态目录为 /uploads（后端 r.Static("/uploads", ...)），file_key 即相对路径
export function fileUrl(fileKey: string) {
  return fileKey ? `/uploads/${fileKey}` : ''
}
