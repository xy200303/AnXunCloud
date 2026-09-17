// Package safe 后台 goroutine 统一收口：裸 go 起的任务 panic 会崩整个进程（gin Recovery 管不到），一律走 safe.Go。
package safe

import (
	"go.uber.org/zap"

	"anxuncloud/internal/pkg/logger"
)

// Go 带 recover 的 goroutine 启动：panic 记录日志后不扩散。
func Go(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.L.Error("后台任务 panic", zap.Any("err", r), zap.Stack("stack"))
			}
		}()
		fn()
	}()
}
