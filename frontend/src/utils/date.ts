// 日期工具：本地时区 YYYY-MM-DD（筛选条件默认区间等场景共用）
export function fmtDate(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}
