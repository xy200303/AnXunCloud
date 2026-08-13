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
 * 标签约定：全局识别与打卡校验统一按卡片 UID（uidHex）——后台点位档案「NFC 卡号」录的就是它，
 * 后端 by-code 与 checkin 均按 nfc_id 比对。NDEF 文本（点位编号）仍保留写入能力，
 * 作为卡面人类可读信息与外部 NFC 工具识别用，不再参与 App 内定位/校验逻辑。
 *
 * 管理端点位维护扩展：
 * - readCardOnce：只读卡片 UID（录 nfc_id 用，不要求 NDEF 文本）；
 * - writePointCode：写入双 NDEF 记录（文本=点位编号 + URI=短链接 /p/{code}，未装 App 的手机贴卡可打开点位信息页）；
 *   写后重读校验；iOS 需 keepSessionAlive，写完恢复默认配置。
 * 读/写共用 onTagDiscovered 单次分发（插件无 off 接口），任一操作进行中另一通道立即失败回调。
 *
 * 不离卡重读（readActiveTagSnapshot）：ReaderMode 只在卡片进入磁场瞬间触发一次事件，
 * 卡一直贴着不会重复触发；按钮操作先查活动标签快照（activeTagTtlMs=8s 窗内），
 * 贴着卡重复点按钮可直接响应，无需「拿开再靠近」。
 */

// #ifdef APP-PLUS
import * as HlNfc from '@/uni_modules/hl-nfccase-uts'
// #endif
import { getPublicOrigin } from '@/services/request'

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

/** 单次读取等待：needCode=true 要求 NDEF 文本（打卡校验）；false 只要卡片 UID（点位录卡号） */
type OneShotWait = {
  needCode: boolean
  cb: (res: NfcReadResult | null, errMsg?: string) => void
}
/** 单次读取回调（iOS 按钮触发 / 表单内 NFC 校验 / 录卡号），优先级高于 globalDispatch */
let oneShot: OneShotWait | null = null

/** 单次写卡等待（点位写编号）：与 oneShot 互斥，onTagDiscovered 内执行 ndefWrite */
type WriteWait = {
  code: string
  cb: (ok: boolean, errMsg?: string) => void
}
let writeReq: WriteWait | null = null
/** iOS 写卡前临时改过 configure（keepSessionAlive），写完需恢复默认，避免影响打卡读取 */
let writeConfigured = false

/** 从 NDEF URI（短链接，如 https://pi.hbuer.com/p/P000015）中提取点位编号 */
function codeFromUri(uri: string): string {
  const m = uri.match(/\/p\/([A-Za-z0-9]+)\/?$/)
  return m != null ? m[1] : ''
}

function parseEvent(event: any): NfcReadResult {
  let code = event != null && event.ndefText != null ? String(event.ndefText).trim() : ''
  // NDEF URI 记录（短链接跳转）兜底：无文本记录时从链接提取点位编号
  if (code == '' && event != null && event.ndefUri != null) {
    code = codeFromUri(String(event.ndefUri).trim())
  }
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

/** 恢复 ensureRegistered 的默认 configure（iOS 写卡后调用，避免 keepSessionAlive 影响打卡读取） */
function restoreDefaultConfigure() {
  // #ifdef APP-PLUS
  if (!writeConfigured) return
  writeConfigured = false
  try {
    HlNfc.configure({
      useReaderMode: true,
      dedupeMs: 3000,
      pendingMax: 10,
      activeTagTtlMs: 8000,
      readSuccessAlertMessage: '读取成功'
    })
  } catch (_e) {}
  // #endif
}

/**
 * 不离卡重读：Android/鸿蒙 ReaderMode 只在卡片进入磁场瞬间触发一次 onTagDiscovered，
 * 卡一直贴着不会重复触发（表现为「要拿开再靠近才识别」）。按钮触发操作前先查
 * 当前磁场中是否已有贴着的卡（插件活动标签时间窗 activeTagTtlMs 内重读快照），
 * 有则直接使用，无需拿开再靠近。无活动标签/超窗返回 null，走正常贴卡等待。
 */
function readActiveTagSnapshot(): NfcReadResult | null {
  // #ifdef APP-PLUS
  try {
    const r: any = (HlNfc as any).refreshActiveTagSync()
    if (r != null && r.code == 0 && r.data != null) {
      const res = parseEvent(r.data)
      if (res.code != null || res.cardId != null) return res
    }
  } catch (_e) {}
  // #endif
  return null
}

/** 插件返回对象中提取错误文案 */
function pluginErrMsg(res: any, fallback: string): string {
  if (res != null && res.data != null && res.data.error != null && res.data.error != '') {
    return String(res.data.error)
  }
  if (res != null && res.message != null && res.message != '') {
    return String(res.message)
  }
  return fallback
}

/** 写卡执行体（onTagDiscovered 内调用）：ndefWrite → 重读校验 → 清活动标签 → 恢复配置 */
function doWrite(code: string, cb: (ok: boolean, errMsg?: string) => void) {
  // #ifdef APP-PLUS
  const run = async () => {
    const anyNfc = HlNfc as any
    try {
      // 双记录：文本（点位编号，App 内识别）+ URI（短链接，未装 App 的手机贴卡可打开点位信息页）
      const uri = getPublicOrigin() + '/p/' + code
      // await 兼容三端：Android/iOS 返回同步对象（await 立即解包），鸿蒙可能返回 Promise
      const res: any = await anyNfc.ndefWrite([
        { type: 'text', data: code, lang: 'zh' },
        { type: 'uri', data: uri }
      ])
      if (res == null || res.code != 0) {
        cb(false, pluginErrMsg(res, '写入失败，请保持贴卡重试'))
        return
      }
      // 写后不离卡重读校验：NDEF 文本应与写入编号一致
      try {
        const r: any = await anyNfc.refreshActiveTagSync()
        const text = r != null && r.data != null && r.data.ndefText != null ? String(r.data.ndefText).trim() : ''
        if (r == null || r.code != 0 || text != code) {
          cb(false, '写入后校验未通过，请重试')
          return
        }
      } catch (_e) {
        cb(false, '写入后校验失败，请重试')
        return
      }
      cb(true)
    } catch (e: any) {
      cb(false, 'NFC 写入异常：' + (e != null && e.message != null ? e.message : ''))
    } finally {
      try { HlNfc.clearActiveTag() } catch (_e) {}
      restoreDefaultConfigure()
    }
  }
  run()
  // #endif
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
    activeTagTtlMs: 8000, // 活动标签时间窗：贴着卡重复点按钮可直接重读（见 readActiveTagSnapshot）
    readSuccessAlertMessage: '读取成功' // iOS 系统面板完成文案
  })
  HlNfc.onTagDiscovered((event: any) => {
    const r = parseEvent(event)
    // 写卡通道优先：贴卡即执行写入（与读取互斥，见 writePointCode）
    if (writeReq != null) {
      const w = writeReq
      writeReq = null
      doWrite(w.code, w.cb)
      return
    }
    if (oneShot != null) {
      const wait = oneShot
      oneShot = null
      // 重置去重状态，允许用户紧接着再贴同一张卡（如表单内重新校验）
      try { HlNfc.clearActiveTag() } catch (_e) {}
      if (wait.needCode && r.code == null) {
        wait.cb(null, '标签内未找到点位编号文本记录')
        return
      }
      if (!wait.needCode && r.cardId == null) {
        wait.cb(null, '未读取到卡片 UID')
        return
      }
      wait.cb(r)
      return
    }
    // 全局识别双通道：NDEF 编号优先（短链接/文本记录），无编号按卡片 UID 兜底
    const key = r.code != null ? r.code : r.cardId
    if (globalDispatch != null && key != null) {
      globalDispatch(key)
    }
  })
  HlNfc.onSessionInvalidated((event: any) => {
    // iOS 用户取消/超时：若在等待单次读取/写卡结果，按失败回调
    if (writeReq != null) {
      const w = writeReq
      writeReq = null
      restoreDefaultConfigure()
      w.cb(false, sessionReasonText(event))
      return
    }
    if (oneShot != null) {
      const wait = oneShot
      oneShot = null
      wait.cb(null, sessionReasonText(event))
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
 * NFC 不可用原因：'no-plugin' 当前安装包/基座未集成插件（标准基座运行开发包常见）；
 * 'unsupported' 设备不支持或未开启；'ok' 可用。
 */
export function nfcUnavailReason(): string {
  // #ifdef APP-PLUS
  try {
    return HlNfc.isNfcSupported() ? 'ok' : 'unsupported'
  } catch (_e) {
    return 'no-plugin'
  }
  // #endif
  // #ifndef APP-PLUS
  return 'unsupported'
  // #endif
}

/** NFC 不可用时给出准确提示（替代笼统的「接入中」） */
export function toastNfcUnavailable() {
  const r = nfcUnavailReason()
  if (r == 'no-plugin') {
    uni.showToast({ title: '当前安装包未集成 NFC 插件，请使用自定义基座运行', icon: 'none' })
    return
  }
  uni.showToast({ title: '设备不支持 NFC 或未开启', icon: 'none' })
}

/**
 * 单次读取 NDEF 文本（保留能力；主流程已统一按卡片 UID，见 readCardOnce/readCardInfoOnce）。
 * cb(res, errMsg)：res.code 为 NDEF 文本（点位编号），res.cardId 为卡片 UID。
 * 用户取消/超时也会通过 cb(null, 原因) 回调。
 */
export function readNdefOnce(cb: (res: NfcReadResult | null, errMsg?: string) => void) {
  // #ifdef APP-PLUS
  if (!isNfcSupported()) {
    cb(null, '设备不支持 NFC 或未开启')
    return
  }
  if (oneShot != null || writeReq != null) {
    cb(null, 'NFC 操作中，请稍候')
    return
  }
  ensureRegistered()
  // 卡已贴着（未离开活动标签时间窗）：直接用快照，无需拿开再靠近
  const snap = readActiveTagSnapshot()
  if (snap != null && snap.code != null) {
    cb(snap)
    return
  }
  oneShot = { needCode: true, cb: cb }
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
 * 单次读卡信息（点位管理「读取 NFC 卡号」）：返回卡片 UID + 卡内 NDEF 文本（点位编号），
 * 空白卡也能读（code 为 null）。用于展示「卡号 + 卡内编号」对应关系。
 */
export function readCardInfoOnce(cb: (res: NfcReadResult | null, errMsg?: string) => void) {
  // #ifdef APP-PLUS
  if (!isNfcSupported()) {
    cb(null, '设备不支持 NFC 或未开启')
    return
  }
  if (oneShot != null || writeReq != null) {
    cb(null, 'NFC 操作中，请稍候')
    return
  }
  ensureRegistered()
  // 卡已贴着（未离开活动标签时间窗）：直接用快照，无需拿开再靠近
  const snapCard = readActiveTagSnapshot()
  if (snapCard != null && snapCard.cardId != null) {
    cb(snapCard)
    return
  }
  oneShot = { needCode: false, cb: cb }
  try {
    HlNfc.startNFCSession('请贴近 NFC 标签读取卡号')
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
 * 单次读卡号（点位管理「读取 NFC 卡号」）：只要卡片 UID（uidHex），不要求 NDEF 文本，
 * 空白卡/未写编号的卡也能读出卡号。回调签名与 readNdefOnce 的简化版对齐。
 */
export function readCardOnce(cb: (cardId: string | null, errMsg?: string) => void) {
  readCardInfoOnce((res, errMsg) => {
    if (res == null) {
      cb(null, errMsg)
      return
    }
    cb(res.cardId, res.cardId == null ? '未读取到卡片 UID' : undefined)
  })
}

/**
 * 写入点位信息到 NFC 标签（双 NDEF 记录）：
 * - text = 点位编号（App 内识别/外部 NFC 工具可读）；
 * - uri  = 短链接 {站点源}/p/{code}（未装 App 的手机贴卡弹系统通知，浏览器打开点位信息公开页）。
 * 流程：登记一次性写等待 → 提示贴卡 → onTagDiscovered 内 ndefWrite → 重读校验 → 恢复配置。
 * - Android：前台 ReaderMode 本就在监听，贴卡即触发；
 * - iOS：先 configure({keepSessionAlive:true}) 再 startNFCSession 弹系统面板，写完恢复默认配置；
 * - 鸿蒙：ndefWrite 可能返回 Promise，doWrite 内统一 await 兼容；
 * - 与读取通道互斥：读/写进行中再发起会立即回调失败；
 * - 非 App 端（小程序）直接回调不支持。
 */
export function writePointCode(code: string, cb: (ok: boolean, errMsg?: string) => void) {
  // #ifdef APP-PLUS
  if (!isNfcSupported()) {
    cb(false, '设备不支持 NFC 或未开启')
    return
  }
  if (oneShot != null || writeReq != null) {
    cb(false, 'NFC 操作中，请稍候')
    return
  }
  ensureRegistered()
  writeReq = { code: code, cb: cb }
  if (platformOf() == 'ios') {
    // iOS 写卡必须 keepSessionAlive，否则读卡后会话自动关闭、写入必失败（插件 readme）
    try {
      HlNfc.configure({
        keepSessionAlive: true,
        activeTagTtlMs: 10000,
        readSuccessKeepAliveAlertMessage: '识别成功，请保持贴卡'
      })
      writeConfigured = true
    } catch (_e) {}
  }
  try {
    HlNfc.startNFCSession('请贴近要写入的 NFC 标签')
  } catch (e: any) {
    writeReq = null
    restoreDefaultConfigure()
    cb(false, 'NFC 调用异常：' + (e && e.message ? e.message : ''))
    return
  }
  // Android/鸿蒙：卡已贴着（活动标签在插件内存中，满足 ndefWrite 前提），直接写入
  if (platformOf() != 'ios' && writeReq != null && readActiveTagSnapshot() != null) {
    const w = writeReq
    writeReq = null
    doWrite(w.code, w.cb)
  }
  // #endif
  // #ifndef APP-PLUS
  cb(false, '当前端不支持 NFC 写卡')
  // #endif
}

/**
 * Android/鸿蒙 全局前台识别：App 打开任何页面贴标签即 dispatch(卡片 UID)。
 * iOS 内部直接忽略（CoreNFC 不支持常驻监听，走按钮触发的单次读取）。
 * 插件无 off 接口，stopGlobalListener 仅摘除内部分发。
 */
export function startGlobalListener(dispatch: (cardId: string) => void) {
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
        const key = r.code != null ? r.code : r.cardId
        if (key != null) {
          dispatch(key)
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
