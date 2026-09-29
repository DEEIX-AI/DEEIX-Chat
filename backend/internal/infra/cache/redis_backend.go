//go:build !noredis

package cache

import (
	"context"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/redis"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	goredis "github.com/go-redis/redis/v8"
)

func openRedis(cfg config.Config) (Backend, error) {
	client, err := redis.NewRedis(cfg)
	if err != nil {
		return nil, err
	}
	return redisBackend{client: client}, nil
}

type redisBackend struct {
	client *goredis.Client
}

func (redisBackend) Name() string { return "redis" }

func (b redisBackend) Settings() repository.SettingsCacheRepository {
	return redis.NewSettingsCache(b.client)
}

func (b redisBackend) Channel() repository.ChannelCacheRepository {
	return redis.NewChannelCache(b.client)
}

func (b redisBackend) Conversation() repository.ConversationCacheRepository {
	return redis.NewConversationCache(b.client)
}

func (b redisBackend) RateLimiter() RateLimiter {
	return redis.NewRateLimiter(b.client)
}

func (b redisBackend) ProviderAuthBridge() repository.ProviderAuthBridgeRepository {
	return redis.NewProviderAuthBridge(b.client)
}

func (b redisBackend) Ping(ctx context.Context) error {
	return b.client.Ping(ctx).Err()
}

func (b redisBackend) Close() error {
	return b.client.Close()
}
