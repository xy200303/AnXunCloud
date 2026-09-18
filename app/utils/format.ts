/** 打卡方式展示映射（checkin_type：qrcode/fence/nfc/offline） */
export function checkinTypeTextOf(t: string): string {
  if (t == 'qrcode') return '扫二维码'
  if (t == 'nfc') return '刷 NFC 卡'
  if (t == 'fence') return '到场确认'
  if (t == 'offline') return '离线补传'
  return t
}
