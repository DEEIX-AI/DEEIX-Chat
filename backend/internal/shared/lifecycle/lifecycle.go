// Package lifecycle 提供进程级优雅关停的信号原语。
package lifecycle

import "sync"

// Shutdown 是进程关停排空信号。
// BeginDrain 触发后 Draining 返回 true 且 Done 通道关闭，供订阅型长连接与就绪探针感知排空。
type Shutdown struct {
	once sync.Once
	done chan struct{}
}

// NewShutdown 创建未触发的关停信号。
func NewShutdown() *Shutdown {
	return &Shutdown{done: make(chan struct{})}
}

// BeginDrain 进入关停排空阶段，幂等且并发安全。
func (s *Shutdown) BeginDrain() {
	if s == nil {
		return
	}
	s.once.Do(func() { close(s.done) })
}

// Draining 报告是否已进入关停排空阶段。
func (s *Shutdown) Draining() bool {
	if s == nil {
		return false
	}
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// Done 返回排空信号通道。nil 接收者返回 nil 通道，select 分支永不就绪。
func (s *Shutdown) Done() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.done
}
