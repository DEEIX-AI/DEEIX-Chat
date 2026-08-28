// Package background 提供分离型后台 goroutine 的统一启动入口。
package background

import (
	"runtime/debug"

	"go.uber.org/zap"
)

// Go 启动分离型后台任务：捕获 panic 并记日志。同步等待结果的 goroutine 不要用本函数。
func Go(logger *zap.Logger, name string, fn func()) {
	if logger == nil {
		logger = zap.NewNop()
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("background_task_panic",
					zap.String("task", name),
					zap.Any("panic", r),
					zap.ByteString("stack", debug.Stack()),
				)
			}
		}()
		fn()
	}()
}
