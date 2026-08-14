import { apiPointByCode } from '@/services/api'
import { isNfcSupported, readCardOnce, toastNfcUnavailable } from '@/utils/nfc'

let scanning = false

/**
 * 从扫码内容提取点位编号：
 * - 新版二维码内容为短链接（{站点源}/p/{code}，外来人员扫码直接开公开页），提取路径中的编号；
 * - 旧版二维码为纯编号（如 P000015），原样返回。
 */
export function extractPointCode(raw: string): string {
  const text = String(raw || '').trim()
  const m = text.match(/\/p\/([A-Za-z0-9]+)\/?(?:[?#].*)?$/)
  return m != null ? m[1] : text
}

/**
 * 按编号解析点位并跳转打卡表单（扫码与 NFC 共用的「任务定位器」，技术方案 §5.3）。
 * code 为点位编号（如 P000015）；后端 by-code 接口同时按 qrcode_no / nfc_id 匹配。
 */
export function resolvePointCode(code: string) {
  apiPointByCode(code)
    .then((p) => {
      uni.vibrateShort({})
      if (p.tasks.length == 0) {
        uni.showToast({ title: '该点位今日无巡检任务', icon: 'none' })
        return
      }
      // 任务定位器：找第一个未打卡任务直接跳打卡表单（携带二维码编号视为已核验凭证）
      const t = p.tasks.find((x) => !x.checked)
      if (t == null) {
        uni.showToast({ title: '该点位今日已完成打卡', icon: 'none' })
        return
      }
      uni.navigateTo({
        url:
          '/pages/checkin/form?task_id=' + encodeURIComponent(t.task_id) +
          '&point_id=' + encodeURIComponent(p.point_id) +
          '&no=' + encodeURIComponent(code)
      })
    })
    .catch((e: Error) => {
      uni.showToast({ title: e.message, icon: 'none' })
    })
}

function doScan() {
  if (scanning) return
  scanning = true
  uni.scanCode({
    onlyFromCamera: true, // 禁相册选图（防二维码照片代扫作弊）
    success: (res) => {
      scanning = false
      resolvePointCode(extractPointCode(res.result))
    },
    fail: (err) => {
      scanning = false
      // 用户取消静默；其他失败（权限等）提示原因便于排查
      const msg = (err && err.errMsg) ? err.errMsg : ''
      if (msg.indexOf('cancel') < 0 && msg != '') {
        uni.showToast({ title: '扫码失败：' + msg, icon: 'none' })
      }
    }
  })
}

/**
 * NFC 打卡（iOS 唯一入口；Android/鸿蒙另有全局前台监听，见 App.vue）。
 * 用户点按钮 → 系统扫描面板 → 读卡片 UID → 任务定位器跳转（后端 by-code 按 nfc_id 匹配）。
 */
function doNfc() {
  if (!isNfcSupported()) {
    toastNfcUnavailable()
    return
  }
  uni.showLoading({ title: '请贴近 NFC 标签', mask: true })
  readCardOnce((cardId, errMsg) => {
    uni.hideLoading()
    if (cardId == null) {
      uni.showToast({ title: errMsg || 'NFC 读取失败', icon: 'none' })
      return
    }
    resolvePointCode(cardId)
  })
}

/**
 * 中央扫码按钮（midButton）入口：不经中间页，直接拉起系统扫码。
 * iOS 先弹方式选择（CoreNFC 必须用户明确触发）；其他端直接扫码。
 */
export function launchCheckinScan() {
  // #ifdef APP-IOS
  uni.showActionSheet({
    itemList: ['扫码打卡', 'NFC 打卡'],
    success: (res) => {
      if (res.tapIndex == 0) doScan()
      if (res.tapIndex == 1) doNfc()
    },
    fail: (_e) => {}
  })
  // #endif
  // #ifndef APP-IOS
  doScan()
  // #endif
}
