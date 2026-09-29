// Package cache 负责选择缓存后端。Memory 始终可用；Redis
// 可通过 -tags noredis 排除。
package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// RateLimiter 与 transport/http/middleware 所用的方法集一致，
// infra 不得导入该包。
type RateLimiter interface {
	AllowSlidingWindow(ctx context.Context, key string, limit int, window time.Duration, ttl time.Duration) (bool, error)
	AllowFixedWindow(ctx context.Context, keys []string, limit int, ttl time.Duration) (bool, error)
}

// Backend 为单个驱动提供全部基于缓存的仓储。
type Backend interface {
	// Name 是健康检查上报的驱动名称。
	Name() string
	Settings() repository.SettingsCacheRepository
	Channel() repository.ChannelCacheRepository
	Conversation() repository.ConversationCacheRepository
	RateLimiter() RateLimiter
	ProviderAuthBridge() repository.ProviderAuthBridgeRepository
	Ping(ctx context.Context) error
	Close() error
}

// Open 返回由 cfg.CacheDriver 选定的后端。
func Open(cfg config.Config) (Backend, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.CacheDriver)) {
	case "", "redis":
		return openRedis(cfg)
	case "memory":
		return memoryBackend{cache: memory.New()}, nil
	default:
		return nil, fmt.Errorf("unsupported cache driver %q", cfg.CacheDriver)
	}
}

type memoryBackend struct {
	cache *memory.Cache
}

func (memoryBackend) Name() string { return "memory" }

func (b memoryBackend) Settings() repository.SettingsCacheRepository {
	return memory.NewSettingsCache(b.cache)
}

func (b memoryBackend) Channel() repository.ChannelCacheRepository {
	return memory.NewChannelCache(b.cache)
}

func (b memoryBackend) Conversation() repository.ConversationCacheRepository {
	return memory.NewConversationCache(b.cache)
}

func (b memoryBackend) RateLimiter() RateLimiter {
	return memory.NewRateLimiter(b.cache)
}

func (b memoryBackend) ProviderAuthBridge() repository.ProviderAuthBridgeRepository {
	return memory.NewProviderAuthBridge(b.cache)
}

func (memoryBackend) Ping(context.Context) error { return nil }

func (memoryBackend) Close() error { return nil }
