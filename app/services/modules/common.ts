/**
 * API 层内部公共工具：ID 容错转换、query 拼接、通用字典接口。
 * 本文件不经 api.ts 全量 re-export（仅 apiDictOptions/DictOption 选择性透出）。
 */

import { httpGet } from '@/services/request'

/** ID 字段容错：后端若下发 number 则转 string（v2 起全系统为 UUIDv7 string） */
export function toId(v: string | number | undefined): string {
  return v != null ? String(v) : ''
}

/**
 * 拼接 URL query：自动 encodeURIComponent，跳过 null/undefined/空串。
 * 返回以 ? 开头的串（无有效参数时返回空串，可直接拼在路径后）。
 */
export function buildQuery(params: Record<string, string | number | boolean | undefined | null>): string {
  const parts: string[] = []
  for (const key of Object.keys(params)) {
    const v = params[key]
    if (v == null || v === '') continue
    parts.push(encodeURIComponent(key) + '=' + encodeURIComponent(String(v)))
  }
  return parts.length > 0 ? '?' + parts.join('&') : ''
}

/** 业务字典选项 GET /dict-options?type_code=（登录即可；报告类型下拉等） */
export type DictOption = { label: string; value: string; sort: number }

export function apiDictOptions(typeCode: string): Promise<DictOption[]> {
  return new Promise<DictOption[]>((resolve, reject) => {
    httpGet<DictOption[]>('/dict-options' + buildQuery({ type_code: typeCode }))
      .then((d) => resolve(d ?? []))
      .catch(reject)
  })
}
