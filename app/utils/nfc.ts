/**
 * NFC 集成层（技术方案 §5.4），插件：hl-nfccase-uts（uni_modules/hl-nfccase-uts）。
 *
 * 插件能力（见插件 interface.uts / readme）：
 * - onTagDiscovered(cb)：持续监听碰卡事件（无 off 接口，全局只注册一次，内部路由分发）；
 * - Android：注册监听即前台自动识别（ReaderMode），无需启动会话；
 * - iOS：必须 startNFCSession() 由用户触发系统扫描面板；取消/超时走 onSessionInvalidated；
 * - 鸿蒙：需 startNFCSession() 但无系统弹窗（全局监听时启动一次即可）；
 * - 事件字段：uidHex（卡片 UID）、ndefText（NDEF 文本=点位编号）、ndefUri、techs、timestamp。
 *
 * 经典 uni-app 条件编译注意（插件 readme Q1）：APP-HARMONY 是 uni-app x 专属标识，
 * 经典版用 APP-PLUS（覆盖 Android/iOS/鸿蒙三端）+ 运行时 platformOf() 区分平台。
 *
 * 标签约定：NDEF 文本记录，内容 = 点位编号（如 P000015）。
 * 提交打卡的 nfc_id 用卡片 UID（uidHex）——后端 checkMode 比对的是点位档案的「NFC 卡号」。
 */

// #ifdef APP-PLUS
import * as HlNfc from '@/uni_modules/hl-nfccase-uts'
// #endif

/** 一次读取的结果：code=NDEF 文本（点位编号）；cardId=卡片 UID（十六进制，提交打卡时作 nfc_id） */
export type NfcReadResult = {
  code: string | null
  cardId: string | null
}

// ---- 运行时平台识别（经典 uni-app 不能用 APP-HARMONY 条件编译） -------------------

/** 'android' | 'ios' | 'harmony' | 'other'（非 App 端恒 'other'） */
export function platformOf(): string {
  // #ifdef APP-PLUS
  const info: any = uni.getSystemInfoSync()
  const p = String(info.platform || info.osName || '').toLowerCase()
  if (p == 'harmony' || p == 'harmonyos' || p == 'ohos') return 'harmony'
  if (p == 'ios') return 'ios'
  if (p == 'android') return 'android'
  // #endif
  return 'other'
}

// ---- 事件解析与全局路由 ----------------------------------------------------------

let registered = false
/** Android/鸿蒙全局前台识别的分发（App.vue 挂载） */
let globalDispatch: ((code: string) => void) | null = null
/** 单次读取回调（iOS 按钮触发 / 表单内 NFC 校验），优先级高于 globalDispatch */
let oneShot: ((res: NfcReadResult | null, errMsg?: string) => void) | null = null

function parseEvent(event: any): NfcReadResult {
  const code = event != null && event.ndefText != null ? String(event.ndefText).trim() : ''
  const cardId = event != null && event.uidHex != null ? String(event.uidHex).trim() : ''
  return { code: code == '' ? null : code, cardId: cardId == '' ? null : cardId }
}

function sessionReasonText(event: any): string {
  const reason = event != null ? String(event.reason || '') : ''
  if (reason == 'userCanceled') return '已取消'
  if (reason == 'sessionTimeout') return '读取超时，请重新贴近标签'
  if (reason == 'systemBusy') return '系统 NFC 繁忙，请稍后再试'
  return event != null && event.message != null ? String(event.message) : 'NFC 会话已结束'
}

/** 全局只注册一次的插件监听（插件无 off 接口） */
function ensureRegistered() {
  // #ifdef APP-PLUS
  if (registered) return
  registered = true
  HlNfc.configure({
    useReaderMode: true, // Android 前台 ReaderMode，更丝滑
    dedupeMs: 3000,      // 同一卡片 3s 去重，防贴着标签连续触发
    pendingMax: 10,
    readSuccessAlertMessage: '读取成功' // iOS 系统面板完成文案
  })
  HlNfc.onTagDiscovered((event: any) => {
    const r = parseEvent(event)
    if (oneShot != null) {
      const cb = oneShot
      oneShot = null
      // 重置去重状态，允许用户紧接着再贴同一张卡（如表单内重新校验）
      try { HlNfc.clearActiveTag() } catch (_e) {}
      if (r.code == null) {
        cb(null, '标签内未找到点位编号文本记录')
        return
      }
      cb(r)
      return
    }
    if (globalDispatch != null && r.code != null) {
      globalDispatch(r.code)
    }
  })
  HlNfc.onSessionInvalidated((event: any) => {
    // iOS 用户取消/超时：若在等待单次读取结果，按失败回调
    if (oneShot != null) {
      const cb = oneShot
      oneShot = null
      cb(null, sessionReasonText(event))
    }
  })
  // #endif
}

// ---- 导出能力 --------------------------------------------------------------------

/** 当前设备是否支持 NFC（App 端且硬件支持；小程序端恒 false） */
export function isNfcSupported(): boolean {
  // #ifdef APP-PLUS
  try {
    return HlNfc.isNfcSupported()
  } catch (_e) {
    return false
  }
  // #endif
  // #ifndef APP-PLUS
  return false // 小程序端一期不支持
  // #endif
}

/**
 * 单次读取（iOS 主路径：用户点按钮 → 系统扫描面板；Android/鸿蒙用于表单内手动校验）。
 * cb(res, errMsg)：res.code 为 NDEF 文本（点位编号），res.cardId 为卡片 UID。
 * 用户取消/超时也会通过 cb(null, 原因) 回调。
 */
export function readNdefOnce(cb: (res: NfcReadResult | null, errMsg?: string) => void) {
  // #ifdef APP-PLUS
  if (!isNfcSupported()) {
    cb(null, '设备不支持 NFC 或未开启')
    return
  }
  if (oneShot != null) {
    cb(null, 'NFC 读取中，请稍候')
    return
  }
  ensureRegistered()
  oneShot = cb
  try {
    // Android 为空实现（前台监听本就在跑，事件会路由到 oneShot）；
    // iOS 弹系统扫描面板；鸿蒙启动会话（无弹窗）。
    HlNfc.startNFCSession('请贴近巡检点位 NFC 标签')
  } catch (e: any) {
    oneShot = null
    cb(null, 'NFC 调用异常：' + (e && e.message ? e.message : ''))
  }
  // #endif
  // #ifndef APP-PLUS
  cb(null, '当前端不支持 NFC')
  // #endif
}

/**
 * Android/鸿蒙 全局前台识别：App 打开任何页面贴标签即 dispatch(点位编号)。
 * iOS 内部直接忽略（CoreNFC 不支持常驻监听，走 readNdefOnce）。
 * 插件无 off 接口，stopGlobalListener 仅摘除内部分发。
 */
export function startGlobalListener(dispatch: (code: string) => void) {
  // #ifdef APP-PLUS
  if (!isNfcSupported()) return
  const plat = platformOf()
  if (plat == 'ios') return
  ensureRegistered()
  globalDispatch = dispatch
  if (plat == 'harmony') {
    // 鸿蒙必须启动会话才开始监听（无系统弹窗，对用户无感）
    try { HlNfc.startNFCSession() } catch (_e) {}
  }
  // 后台/冷启动贴卡的缓存事件兜底（App 未打开时贴标签唤起场景）
  try {
    const pending: any = HlNfc.getPendingTagsSync(true, 10)
    if (Array.isArray(pending)) {
      for (let i = 0; i < pending.length; i++) {
        const r = parseEvent(pending[i])
        if (r.code != null) {
          dispatch(r.code)
          break // 只取最新一条有效点位，避免连跳
        }
      }
    }
  } catch (_e) {}
  // #endif
}

export function stopGlobalListener() {
  globalDispatch = null
}
