// 地图服务接口（接口文档 §2.x：GET /map/config、GET /map/search）
import type { AxiosRequestConfig } from 'axios'
import { request } from '@/utils/request'

export interface MapConfig {
  provider: string
  key: string
}

export interface MapPlace {
  title: string
  address: string
  lng: number
  lat: number
}

// 地图服务公开配置：key 为空表示未配置，前端据此禁用地图选点
export function getMapConfig() {
  return request<MapConfig>({ url: '/map/config', method: 'get' })
}

// 地点搜索（腾讯地点提示代理）；location 传当前地图中心可让结果偏向附近
// silent：错误在选点面板内展示，不走全局 toast
export function searchMapPlaces(keyword: string, location?: string) {
  return request<MapPlace[]>({ url: '/map/search', method: 'get', params: { keyword, location }, silent: true } as AxiosRequestConfig & { silent?: boolean })
}
