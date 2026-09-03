/**
 * App 检查更新（技术方案：发布物 force_update 标记）。
 * - 启动自动检查（today 页挂载时调一次，无更新静默）；
 * - 关于页手动检查（无更新 toast「已是最新」）；
 * - 有更新 → UpdateDialog 自定义弹窗（强制更新不可跳过）。
 * 数据源：GET /api/public/app/latest?platform=android|harmony|ios（公开接口，免登录）。
 */
import { getPublicOrigin } from '@/services/request'
import { platformOf } from '@/utils/nfc'

export type LatestRelease = {
  version: string
  force_update: boolean
  note: string
  size: number
  download_url: string
}

/** 当前安装包版本（非 App 端回落 manifest 默认） */
export function currentVersion(): string {
  // #ifdef APP-PLUS
  const rt: any = plus.runtime
  if (rt != null && rt.version != null && rt.version != '') return String(rt.version)
  // #endif
  return '1.0.0'
}

/** 版本比较：a>b 返回 1，a<b 返回 -1，相等 0（按数字段比较，1.0.10 > 1.0.9；非数字段按 0 计） */
export function compareVersion(a: string, b: string): number {
  const pa = String(a || '').replace(/^v/i, '').split('.')
  const pb = String(b || '').replace(/^v/i, '').split('.')
  const n = Math.max(pa.length, pb.length)
  for (let i = 0; i < n; i++) {
    const x = i < pa.length ? parseInt(pa[i], 10) || 0 : 0
    const y = i < pb.length ? parseInt(pb[i], 10) || 0 : 0
    if (x != y) return x > y ? 1 : -1
  }
  return 0
}

/** 拉取本平台最新发布物；未发布/无更新/网络失败均 resolve(null)（检查更新不打扰主流程） */
export function fetchLatestRelease(): Promise<LatestRelease | null> {
  return new Promise((resolve) => {
    // #ifndef APP-PLUS
    resolve(null)
    // #endif
    // #ifdef APP-PLUS
    const plat = platformOf()
    if (plat == 'other') {
      resolve(null)
      return
    }
    uni.request({
      url: getPublicOrigin() + '/api/public/app/latest?platform=' + plat,
      method: 'GET',
      timeout: 10000,
      success: (res) => {
        const env: any = res.data
        if (res.statusCode != 200 || env == null || env.code != 0 || env.data == null) {
          resolve(null)
          return
        }
        const d = env.data
        const ver = String(d.version || '')
        if (ver == '' || compareVersion(ver, currentVersion()) <= 0) {
          resolve(null)
          return
        }
        resolve({
          version: ver,
          force_update: d.force_update === true,
          note: String(d.note || ''),
          size: typeof d.size == 'number' ? d.size : 0,
          download_url: String(d.download_url || '')
        })
      },
      fail: () => resolve(null)
    })
    // #endif
  })
}
