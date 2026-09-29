//go:build noredis

package cache

import (
	"errors"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

// ErrRedisUnavailable 在使用 -tags noredis 构建时返回。
var ErrRedisUnavailable = errors.New("cache: redis driver not compiled into this binary; use cache_driver: memory")

func openRedis(config.Config) (Backend, error) {
	return nil, ErrRedisUnavailable
}
