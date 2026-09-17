/**
 * 业务 API 层——与后端 /api/app（小程序 /api/mp）接口一一对应，类型对齐响应结构。
 * 参照 docs/接口文档.md §1.3 信封、§3 移动端接口；/api/app 组与 /api/mp 同形（技术方案 §4）。
 *
 * 本文件仅为 re-export 聚合层：实现按业务域拆分在 services/modules/ 下，
 * 页面 import 路径保持 '@/services/api' 不变。
 */

export * from './modules/auth'
export * from './modules/task'
export * from './modules/checkin'
export * from './modules/reports'
export * from './modules/message'
export * from './modules/admin'
export * from './modules/equipment'

// 通用工具中仅字典接口对外（toId/buildQuery 为模块内部实现细节）
export { apiDictOptions } from './modules/common'
export type { DictOption } from './modules/common'
