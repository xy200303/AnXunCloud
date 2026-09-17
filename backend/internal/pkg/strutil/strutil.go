// Package strutil 收编各 service 包内复制粘贴的字符串小工具。
// 统一语义：截断类函数一律先 TrimSpace 再按 rune 截断；拼接类函数忽略空白段。
package strutil

import "strings"

// Truncate 截断字符串至 n 个 rune。统一语义：先 TrimSpace 再截断。
func Truncate(s string, n int) string {
	rs := []rune(strings.TrimSpace(s))
	if len(rs) <= n {
		return string(rs)
	}
	return string(rs[:n])
}

// StrVal 可空文本快照取值（nil → 空串）。
func StrVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// AppendNote 追加备注/理由（全角分号连接；两段均 TrimSpace，忽略空段）。
func AppendNote(base, add string) string {
	base = strings.TrimSpace(base)
	add = strings.TrimSpace(add)
	if base == "" {
		return add
	}
	if add == "" {
		return base
	}
	return base + "；" + add
}
