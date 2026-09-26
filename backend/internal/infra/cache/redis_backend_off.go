//go:build noredis

package cache

import (
	"errors"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

// ErrRedisUnavailable is returned by builds made with -tags noredis.
var ErrRedisUnavailable = errors.New("cache: redis driver not compiled into this binary; use cache_driver: memory")

func openRedis(config.Config) (Backend, error) {
	return nil, ErrRedisUnavailable
}
