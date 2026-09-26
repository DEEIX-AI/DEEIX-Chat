// Package cache selects the cache backend. Memory is always available; Redis
// is excluded with -tags noredis.
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

// RateLimiter matches the method set consumed by transport/http/middleware,
// which infra must not import.
type RateLimiter interface {
	AllowSlidingWindow(ctx context.Context, key string, limit int, window time.Duration, ttl time.Duration) (bool, error)
	AllowFixedWindow(ctx context.Context, keys []string, limit int, ttl time.Duration) (bool, error)
}

// Backend provides every cache-backed repository for one driver.
type Backend interface {
	// Name is the driver name reported by health checks.
	Name() string
	Settings() repository.SettingsCacheRepository
	Channel() repository.ChannelCacheRepository
	Conversation() repository.ConversationCacheRepository
	RateLimiter() RateLimiter
	ProviderAuthBridge() repository.ProviderAuthBridgeRepository
	Ping(ctx context.Context) error
	Close() error
}

// Open returns the backend selected by cfg.CacheDriver.
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
