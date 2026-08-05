// axios 封装：baseURL /api/admin、Bearer token、统一错误提示、401 跳登录、文件流下载
import axios, { AxiosError, type AxiosRequestConfig, type AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'
import { getToken, clearToken } from '@/utils/auth'
import { mockAdapter } from '@/mock'

// 开发期 mock 开关：
// - 默认关闭（业务接口已全部对接真实后端 http://localhost:8090）
// - VITE_USE_MOCK=true 时启用完整 mock（系统管理旧页面可脱离后端点通）
const useMock = import.meta.env.DEV && import.meta.env.VITE_USE_MOCK === 'true'

// 统一响应结构（接口文档 §1.3）
export interface ApiResult<T = any> {
  code: number
  message: string
  data: T
}

// 分页响应结构（接口文档 §1.4）
export interface PageResult<T = any> {
  list: T[]
  total: number
  page: number
  page_size: number
}

// 是否需要提示"重新登录"防重复弹
let reloginPending = false

function toLogin() {
  if (reloginPending) return
  reloginPending = true
  clearToken()
  // 全量刷新，清掉已注入的动态路由与状态
  window.location.href = `/login?redirect=${encodeURIComponent(window.location.pathname)}`
}

const service = axios.create({
  baseURL: '/api/admin',
  timeout: 30000,
  adapter: async (config) => {
    if (useMock) {
      const mockResp = await mockAdapter(config)
      if (mockResp) return mockResp
    }
    // 未命中 mock 时透传真实后端（vite proxy → http://localhost:8090）
    // axios 1.x 的 defaults.adapter 是名称数组（['xhr','http','fetch']），需用 getAdapter 解析
    return axios.getAdapter(['xhr', 'fetch'])(config)
  }
})

// 请求拦截：注入 Bearer token
service.interceptors.request.use(
  (config) => {
    const token = getToken()
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

// 响应拦截：统一按 code 判断；blob 文件流直接放行
// 返回值实际为 ApiResult（request() 泛型承担类型收敛），故 fulfilled 用 any 收敛
service.interceptors.response.use(
  (response): any => {
    // 文件流（导出/模板下载）直接返回完整 response
    if (response.config.responseType === 'blob') {
      return response
    }
    const res = response.data as ApiResult
    if (res.code === 0) {
      // 统一收敛为 data 部分，api 层直接拿到业务数据
      return res.data
    }
    handleBizError(res.code, res.message)
    return Promise.reject(new Error(res.message || '请求失败'))
  },
  (error: AxiosError<ApiResult>) => {
    // blob 错误响应需转 JSON 读取错误信息
    if (error.response?.config.responseType === 'blob' && error.response.data instanceof Blob) {
      error.response.data.text().then((text) => {
        try {
          const res = JSON.parse(text) as ApiResult
          handleBizError(res.code, res.message)
        } catch {
          ElMessage.error('下载失败，请稍后重试')
        }
      })
      return Promise.reject(error)
    }
    const res = error.response?.data
    if (res && typeof res.code === 'number') {
      handleBizError(res.code, res.message)
    } else if (error.code === 'ECONNABORTED') {
      ElMessage.error('请求超时，请稍后重试')
    } else {
      ElMessage.error('网络异常，请检查网络后重试')
    }
    return Promise.reject(error)
  }
)

function handleBizError(code: number, message?: string) {
  // 认证类错误：跳登录
  if (code === 40101 || code === 40102 || code === 40103 || code === 40104) {
    ElMessage.error(message || '登录状态已失效，请重新登录')
    toLogin()
    return
  }
  ElMessage.error(message || '操作失败，请稍后重试')
}

// 泛型请求：返回 data 部分
export function request<T = any>(config: AxiosRequestConfig): Promise<T> {
  return service(config) as unknown as Promise<T>
}

export default service
