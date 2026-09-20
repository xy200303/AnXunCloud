/** 打卡方式展示映射（checkin_type：qrcode/fence/nfc/offline） */
export function checkinTypeTextOf(t: string): string {
  if (t == 'qrcode') return '扫二维码'
  if (t == 'nfc') return '刷 NFC 卡'
  if (t == 'fence') return '到场确认'
  if (t == 'offline') return '离线补传'
  return t
}

/** 日期时间展示（兼容 ISO 字符串、普通日期字符串与毫秒时间戳） */
export function formatDateTime(value: string | number | Date | null | undefined): string {
  if (value == null || value === '') return ''

  let date: Date
  if (value instanceof Date || typeof value == 'number') {
    date = new Date(value)
  } else if (/^\d+$/.test(value)) {
    date = new Date(Number(value))
  } else {
    date = new Date(value.includes('T') ? value : value.replace(/-/g, '/'))
  }
  if (Number.isNaN(date.getTime())) return String(value)

  const pad = (part: number) => String(part).padStart(2, '0')
  return (
    date.getFullYear() + '-' +
    pad(date.getMonth() + 1) + '-' +
    pad(date.getDate()) + ' ' +
    pad(date.getHours()) + ':' +
    pad(date.getMinutes()) + ':' +
    pad(date.getSeconds())
  )
}

/** 无法检查原因展示映射（exception_type：device_missing/unable_to_capture/camera_broken/ai_failed/label_missing；空=无原因） */
export function itemExceptionTextOf(t: string): string {
  if (t == 'device_missing') return '设备不存在'
  if (t == 'unable_to_capture') return '无法拍摄'
  if (t == 'camera_broken') return '相机故障'
  if (t == 'ai_failed') return 'AI 识别失败（人工现场确认）'
  if (t == 'label_missing') return '标签磨损'
  return ''
}

/** 检查项三态结论文案（checkin_record_item.result：normal 正常 / abnormal 异常 / escaped 无法检查，escaped 按 exception_type 带原因） */
export function itemResultTextOf(result: string, exceptionType?: string): string {
  if (result == 'escaped') {
    const et = itemExceptionTextOf(exceptionType ?? '')
    return et != '' ? '⊘ 无法检查·' + et : '⊘ 无法检查'
  }
  if (result == 'abnormal') return '⚠ 异常'
  return '✓ 正常'
}

/** 检查项三态结论颜色键（normal=success 绿 / abnormal=danger 红 / escaped=warning 橙灰） */
export function itemResultColorKeyOf(result: string): 'success' | 'danger' | 'warning' {
  if (result == 'escaped') return 'warning'
  if (result == 'abnormal') return 'danger'
  return 'success'
}
