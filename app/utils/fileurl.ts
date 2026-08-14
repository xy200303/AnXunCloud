import { getAccessToken } from '@/utils/storage'

/**
 * 统一文件层地址附带登录态：/api/files 需鉴权，而 <image>/previewImage 无法带请求头，
 * 由后端 AuthAny 中间件的 ?token= 查询参数兜底。
 * 仅 /api/files/ 形式的地址需要拼接（签名/公章/导出文件）；/uploads/* 内容图无需处理。
 */
export function withFileToken(url: string | null): string {
  if (url == null || url == '') return ''
  if (url.indexOf('/api/files/') < 0) return url
  const t = getAccessToken()
  if (t == '') return url
  return url + (url.indexOf('?') >= 0 ? '&' : '?') + 'token=' + encodeURIComponent(t)
}
