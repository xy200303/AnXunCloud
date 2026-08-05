// Package logger 初始化全局 zap 日志。
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// L 全局日志实例。
var L *zap.Logger

// Init 按级别初始化生产格式（JSON）日志。
func Init(level string) error {
	lv := zapcore.InfoLevel
	if err := lv.UnmarshalText([]byte(level)); err != nil {
		lv = zapcore.InfoLevel
	}
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(lv)
	cfg.Encoding = "console"
	cfg.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
	l, err := cfg.Build()
	if err != nil {
		return err
	}
	L = l
	return nil
}
