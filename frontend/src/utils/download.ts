// 文件流下载：从 content-disposition 提取文件名并触发浏览器保存
import service from '@/utils/request'
import type { AxiosResponse } from 'axios'

export async function downloadFile(url: string, params?: Record<string, any>, fallbackName = 'download.xlsx') {
  const response = (await service({
    url,
    method: 'get',
    params,
    responseType: 'blob'
  })) as unknown as AxiosResponse<Blob>

  saveBlob(response, fallbackName)
}

// POST 文件流下载（勾选 id 集合导出入口，避免 GET URL 过长）
export async function downloadFilePost(url: string, data?: Record<string, any>, fallbackName = 'download.xlsx') {
  const response = (await service({
    url,
    method: 'post',
    data,
    responseType: 'blob'
  })) as unknown as AxiosResponse<Blob>

  saveBlob(response, fallbackName)
}

function saveBlob(response: AxiosResponse<Blob>, fallbackName: string) {
  const disposition = (response.headers?.['content-disposition'] as string) || ''
  const match = disposition.match(/filename\*?=(?:UTF-8'')?"?([^";]+)"?/i)
  const filename = match ? decodeURIComponent(match[1]) : fallbackName

  const blobUrl = URL.createObjectURL(response.data)
  const a = document.createElement('a')
  a.href = blobUrl
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(blobUrl)
}
