/**
 * uniPush 2.0 集成层（App 端专属；全部原生调用包在 #ifdef APP-PLUS 内，H5/小程序零影响）。
 *
 * 能力：
 * - CID 获取与绑定上报：登录成功（stores/auth.login）与 App 启动已登录（App.vue onLaunch）
 *   时调 POST /push/device；登出前（stores/auth.logout）调 DELETE /push/device 解绑。
 *   拿不到 cid / 非 android|ios / 接口失败均静默跳过，不阻塞主流程。
 * - 通知点击跳转：App.vue onLaunch 注册 plus.push 'click' 监听，解析 payload
 *   （click_type=payload_custom 的自定义 JSON：{type, biz_id}，与 sys_message 同源），
 *   按 type 路由，路由口径与 pages/messages/index.vue onTap 保持一致。
 * - 图标角标同步：setAppBadge(n) 为底层封装（plus.runtime.setBadgeNumber，n=0 也调用清零）；
 *   syncBadge() 轻量拉取未读数（GET /messages?page=1&page_size=1 只取 unread_count）后设置。
 *   调用点：App 启动/登录后（跟随 bindPushDevice）、消息中心标记已读后、推送点击处理完；
 *   未登录不调用，登出时清零。iOS 服务端推送也带绝对角标（push_channel.ios.auto_badge），
 *   端内同步用于 Android（launcher 支持才显示）与已读后的即时收敛。
 *
 * 已知限制（冷启动覆盖度）：
 * - 在线推送（应用存活 / 个推通道）点击必触发 click 事件；
 * - 厂商通道离线推送点击冷启动时，个推 SDK 一般也会回调 click 事件，但个别机型/系统上
 *   事件可能早于 JS 监听注册到达而丢失。未做 plus.push.getAllNotifications 兜底——
 *   该 API 只是 Android 通知栏快照，拿不到完整 payload，强行做意义有限，注释说明于此。
 */

// #ifdef APP-PLUS
declare const plus: any
// #endif

import { apiBindPushDevice, apiUnbindPushDevice, apiMessages } from '@/services/api'
import { getAccessToken } from '@/utils/storage'

// ---- CID 获取与绑定 ---------------------------------------------------------------

/** 取 uniPush CID；非 App 端 / 拿不到时 resolve('')（调用方静默跳过） */
export function getPushCid(): Promise<string> {
  return new Promise<string>((resolve) => {
    // #ifdef APP-PLUS
    try {
      ;(uni as any).getPushClientId({
        success: (res: any) => {
          resolve(res != null && res.cid != null && res.cid != '' ? String(res.cid) : '')
        },
        fail: () => {
          resolve('')
        }
      })
    } catch (e) {
      resolve('')
    }
    // #endif
    // #ifndef APP-PLUS
    resolve('')
    // #endif
  })
}

/** 推送平台：'android' | 'ios'，其他端返回 ''（跳过绑定） */
function pushPlatform(): string {
  // #ifdef APP-PLUS
  const p = String(uni.getSystemInfoSync().platform || '').toLowerCase()
  if (p == 'android' || p == 'ios') return p
  // #endif
  return ''
}

/** 绑定上报（登录成功 / 启动时已登录调用）；无 token、拿不到 cid、接口失败均静默跳过 */
export function bindPushDevice(): void {
  // #ifdef APP-PLUS
  const platform = pushPlatform()
  if (platform == '' || getAccessToken() == '') return
  getPushCid().then((cid) => {
    if (cid == '') return
    apiBindPushDevice(cid, platform).catch((_e: any) => {})
  })
  // #endif
}

/** 解绑（登出清 token 之前调用）；无论成败都 resolve，不阻塞登出 */
export function unbindPushDevice(): Promise<void> {
  return new Promise<void>((resolve) => {
    // #ifdef APP-PLUS
    if (pushPlatform() == '' || getAccessToken() == '') {
      resolve()
      return
    }
    getPushCid().then((cid) => {
      if (cid == '') {
        resolve()
        return
      }
      apiUnbindPushDevice(cid)
        .then(() => resolve())
        .catch((_e: any) => resolve())
    })
    // #endif
    // #ifndef APP-PLUS
    resolve()
    // #endif
  })
}

// ---- 图标角标同步 ------------------------------------------------------------------

/**
 * 底层角标设置（plus.runtime.setBadgeNumber；n<=0 也调用以清零）。
 * iOS 与「支持角标的 Android launcher」生效，不支持的机型静默无效；非 App 端静默跳过。
 */
export function setAppBadge(n: number): void {
  // #ifdef APP-PLUS
  try {
    plus.runtime.setBadgeNumber(n > 0 ? n : 0)
  } catch (e) {}
  // #endif
}

/** 拉取当前未读数并同步图标角标（失败静默）；未登录不调用（登出场景由调用方 setAppBadge(0) 清零） */
export function syncBadge(): void {
  // #ifdef APP-PLUS
  if (getAccessToken() == '') return
  apiMessages(1, 1)
    .then((res) => {
      setAppBadge(res.unread_count)
    })
    .catch((_e: any) => {})
  // #endif
}

// ---- 通知点击跳转 ------------------------------------------------------------------

/** 推送 payload（后端 click_type=payload_custom 的自定义 JSON，与 sys_message type/biz_id 同源） */
export type PushPayload = {
  type?: string
  biz_id?: string
}

/** 解析 msg.payload：可能是 JSON 字符串或已解析对象；解析失败返回 null（走兜底路由） */
function parsePayload(raw: any): PushPayload | null {
  if (raw == null) return null
  if (typeof raw == 'object') return raw as PushPayload
  if (typeof raw == 'string') {
    try {
      const obj = JSON.parse(raw)
      return typeof obj == 'object' && obj != null ? (obj as PushPayload) : null
    } catch (e) {
      return null
    }
  }
  return null
}

/**
 * 按 type/biz_id 路由（口径与消息列表 onTap 一致）。
 * 未登录先回登录页（登录后不再自动续跳，用户可从消息中心进入）；
 * 缺 type / 未知类型 / 解析失败兜底消息列表。
 */
export function routePushMessage(payload: PushPayload | null): void {
  if (getAccessToken() == '') {
    uni.reLaunch({ url: '/pages/login/index' })
    return
  }
  const type = payload != null && payload.type != null ? payload.type : ''
  const biz = payload != null && payload.biz_id != null ? String(payload.biz_id) : ''
  // inspection：biz_id 为巡检任务 id → 任务明细
  if (type == 'inspection' && biz != '') {
    uni.navigateTo({ url: '/pages/tasks/detail?id=' + encodeURIComponent(biz) })
    return
  }
  if (type == 'report') {
    if (biz != '') {
      uni.navigateTo({ url: '/pages/reports/detail?id=' + encodeURIComponent(biz) })
    } else {
      // 无 biz_id：落报告待办列表
      uni.navigateTo({ url: '/pages/reports/pending' })
    }
    return
  }
  if (type == 'checkin_audit') {
    // 打卡打回/审核提醒：带去今日任务（tabBar 页用 switchTab）
    uni.switchTab({ url: '/pages/tasks/today' })
    return
  }
  if (type == 'announcement' && biz != '') {
    uni.navigateTo({ url: '/pages/messages/notice-detail?id=' + encodeURIComponent(biz) })
    return
  }
  // notice / 缺 type / 未知类型 / payload 解析失败：兜底消息列表
  uni.switchTab({ url: '/pages/messages/index' })
}

/** 注册通知点击监听（App.vue onLaunch 调用一次；重复调用安全） */
let clickListening = false
export function initPushClickListener(): void {
  // #ifdef APP-PLUS
  if (clickListening) return
  clickListening = true
  try {
    plus.push.addEventListener('click', (msg: any) => {
      routePushMessage(parsePayload(msg != null ? msg.payload : null))
      // 点击处理完补一次角标同步（通知到达时服务端已按最新未读数下发 iOS 角标，此处兜底收敛）
      syncBadge()
    })
  } catch (e) {}
  // #endif
}
