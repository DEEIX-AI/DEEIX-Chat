// Package background 提供分离型后台 goroutine 的统一启动入口。
package background

import (
	"context"
	"runtime/debug"
	"sync"
	"time"

	"go.uber.org/zap"
)

// inflight 跟踪所有通过 Go 启动的任务，以便关闭时排空。
var inflight sync.WaitGroup

// Detach 保留父上下文中的值，同时让收尾工作不受请求取消或原截止时间影响。
func Detach(parent context.Context) context.Context {
	if parent == nil {
		return context.Background()
	}
	return context.WithoutCancel(parent)
}

// WithTimeout 为请求派生的后台或收尾工作创建独立、有限的执行窗口。
func WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(Detach(parent), timeout)
}

// Go 启动分离型后台任务：捕获 panic 并记日志。同步等待结果的 goroutine 不要用本函数。
func Go(logger *zap.Logger, name string, fn func()) {
	if logger == nil {
		logger = zap.NewNop()
	}
	inflight.Add(1)
	go func() {
		defer inflight.Done()
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

// Wait 阻塞直到所有通过 Go 启动的任务返回或 ctx 到期。
// 调用方应先取消任务的 context；Wait 仅限定排空的时长上限。
func Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
