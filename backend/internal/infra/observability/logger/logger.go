package logger

import (
	base "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/logger"
	"go.uber.org/zap"
)

// New 创建平台日志实例。
func New(env string) (*zap.Logger, error) {
	return base.New(env, "deeix-chat")
}
