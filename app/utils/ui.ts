/**
 * UI 反馈小工具（toast 样板收敛）。
 */

/** 错误 toast：title 取 e?.message，兜底「操作失败」 */
export function toastErr(e: any): void {
  uni.showToast({ title: (e != null && e.message) || '操作失败', icon: 'none' })
}
