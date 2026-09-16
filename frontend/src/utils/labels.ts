// 打卡相关标签映射：打卡方式 / 打卡结果 / 审核状态（打卡记录页、报告页、详情抽屉共用）

// 打卡方式：qrcode 扫码 / fence 围栏 / offline 离线补传 / nfc NFC
export function checkinTypeLabel(t: string) {
  return { qrcode: '扫码', fence: '围栏', offline: '离线补传', nfc: 'NFC' }[t] || t
}

// 打卡结果 tag：疑似作弊-橙 / 异常-红 / 正常-绿
export function checkinResultTag(row: { result: string; is_suspect: boolean }): { label: string; type: 'success' | 'warning' | 'danger' } {
  if (row.is_suspect) return { label: '疑似作弊', type: 'warning' }
  if (row.result === 'abnormal') return { label: '异常', type: 'danger' }
  return { label: '正常', type: 'success' }
}

// 审核状态：auto_pass 默认通过-灰 / pending 待审核-橙 / pass 人工通过-绿 / rejected 已驳回-红
export function auditStatusTag(s: string): { label: string; type: 'info' | 'warning' | 'success' | 'danger' } {
  return (
    {
      auto_pass: { label: '默认通过', type: 'info' },
      pending: { label: '待审核', type: 'warning' },
      pass: { label: '人工通过', type: 'success' },
      rejected: { label: '已驳回', type: 'danger' }
    }[s] || { label: s || '--', type: 'info' }
  ) as { label: string; type: 'info' | 'warning' | 'success' | 'danger' }
}
