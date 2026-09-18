// Package configread 直读 sys_config 的系统参数读取统一入口（无缓存，缺失/非法回退默认值）。
// 适用低频/批处理场景；高频且带 config:all 缓存的场景仍走注入式 configSvc.Get。
package configread

import (
	"strconv"
	"strings"

	"gorm.io/gorm"

	sysmodel "anxuncloud/internal/module/system/model"
)

// Lookup 读取原始值（缺失/查询失败 → ok=false）。
func Lookup(db *gorm.DB, key string) (string, bool) {
	var cfg sysmodel.SysConfig
	if err := db.Select("value").Where("key = ?", key).First(&cfg).Error; err != nil {
		return "", false
	}
	return cfg.Value, true
}

// Getter 返回 ai.NewClient 等注入点所需的 func(key) (string, bool) 闭包（每次调用实时直读，不缓存）。
func Getter(db *gorm.DB) func(key string) (string, bool) {
	return func(key string) (string, bool) { return Lookup(db, key) }
}

// GetInt 读取整数型系统参数（TrimSpace 后解析；缺失/非法回退默认值）。
func GetInt(db *gorm.DB, key string, def int) int {
	if v, ok := Lookup(db, key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

// GetBool 读取布尔型系统参数（"true"/"false"；缺失/非法回退默认值）。
func GetBool(db *gorm.DB, key string, def bool) bool {
	if v, ok := Lookup(db, key); ok {
		switch strings.TrimSpace(v) {
		case "true":
			return true
		case "false":
			return false
		}
	}
	return def
}

// GetString 读取字符串型系统参数（缺失/空串回退默认值）。
func GetString(db *gorm.DB, key, def string) string {
	if v, ok := Lookup(db, key); ok && v != "" {
		return v
	}
	return def
}
