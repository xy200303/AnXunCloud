// 字典管理接口（接口文档 §2.6）
import { request, type PageResult } from '@/utils/request'
import type { DictType, DictData } from './types'

export function listDictTypes(params: { page?: number; page_size?: number; code?: string; name?: string }) {
  return request<PageResult<DictType>>({ url: '/system/dict-types', method: 'get', params })
}

export function createDictType(data: { code: string; name: string }) {
  return request<{ id: string }>({ url: '/system/dict-types', method: 'post', data })
}

export function updateDictType(id: string, data: { name: string }) {
  return request<null>({ url: `/system/dict-types/${id}`, method: 'put', data })
}

export function deleteDictType(id: string) {
  return request<null>({ url: `/system/dict-types/${id}`, method: 'delete' })
}

export function listDictData(params: { type_code: string; label?: string; status?: number | ''; page?: number; page_size?: number }) {
  return request<PageResult<DictData>>({ url: '/system/dict-data', method: 'get', params })
}

export function createDictData(data: Omit<DictData, 'id'>) {
  return request<{ id: string }>({ url: '/system/dict-data', method: 'post', data })
}

export function updateDictData(id: string, data: Partial<DictData>) {
  return request<null>({ url: `/system/dict-data/${id}`, method: 'put', data })
}

export function deleteDictData(id: string) {
  return request<null>({ url: `/system/dict-data/${id}`, method: 'delete' })
}
