import { apiPointByCode } from '@/services/api'
import { isNfcSupported, readCardOnce, toastNfcUnavailable } from '@/utils/nfc'

let scanning = false

/**
 * 从新版点位二维码短链接（{站点源}/p/{code}）提取点位编号。
 */
export function extractPointCode(raw: string): string {
  const text = String(raw || '').trim()
  const m = text.match(/\/p\/([A-Za-z0-9]+)\/?(?:[?#].*)?$/)
  return m != null ? m[1] : ''
}

/**
 * 按编号解析点位并跳转连续巡检向导（扫码与 NFC 共用的「任务定位器」，技术方案 §5.3）。
 * 扫码传入二维码编号，NFC 传入卡片 UID；后端分别按 qrcode_no / nfc_id 精确匹配。
 * 跳转携带编号：向导内按 qrcode_no/nfc_id 匹配到点位即视为已核验凭证。
 */
export function resolvePointCode(code: string) {
  apiPointByCode(code)
    .then((p) => {
      uni.vibrateShort({})
      if (p.tasks.length == 0) {
        uni.showToast({ title: '该点位今日无巡检任务', icon: 'none' })
        return
      }
      // 任务定位器：找第一个未打卡任务直接跳进连续巡检向导并定位到该点位
      const t = p.tasks.find((x) => !x.checked)
      if (t == null) {
        // 今日已打卡：进记录卡（先看后改）
        uni.navigateTo({
          url:
            '/pages/checkin/record?task_id=' + encodeURIComponent(p.tasks[0].task_id) +
            '&point_id=' + encodeURIComponent(p.point_id)
        })
        return
      }
      uni.navigateTo({
        url:
          '/pages/checkin/quick?task_id=' + encodeURIComponent(t.task_id) +
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
      const code = extractPointCode(res.result)
      if (code == '') {
        uni.showToast({ title: '请扫描新版点位二维码', icon: 'none' })
        return
      }
      resolvePointCode(code)
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
 * 导出供「+」菜单的 NFC 识别项复用（iOS 手动触发入口）。
 */
export function doNfc() {
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
