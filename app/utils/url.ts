/**
 * 资源 URL 工具：后端返回的附件/图片 url 可能是相对路径（/uploads/...），
 * 需拼上 baseURL 的源（协议 + host，去掉 /api/{app,mp} 前缀）才能在 image/webview 中加载。
 */
import { getBaseUrl } from '@/services/request'

/** 后端源（协议 + host），如 http://10.172.17.43:8090 */
export function apiOrigin(): string {
  const m = getBaseUrl().match(/^https?:\/\/[^/]+/)
  return m != null ? m[0] : ''
}

/** 转绝对地址：http(s):// 开头原样返回；/ 开头拼后端源；其余原样返回。
 * 例外：dev 环境下 PC 端上传产生的 http://127.0.0.1|localhost(:port) 绝对地址
 * 在手机上不可达，统一改写为当前 baseURL 的源（内网调试 IP/正式域名）。 */
export function toAbsUrl(url: string): string {
  if (url == null || url == '') return ''
  if (/^https?:\/\/(127\.0\.0\.1|localhost)(:\d+)?\//.test(url)) {
    return apiOrigin() + url.replace(/^https?:\/\/(127\.0\.0\.1|localhost)(:\d+)?/, '')
  }
  if (/^https?:\/\//.test(url)) return url
  if (url.charAt(0) == '/') return apiOrigin() + url
  return url
}
