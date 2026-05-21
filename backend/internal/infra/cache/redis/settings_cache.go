package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/go-redis/redis/v8"
)

const settingsCacheTTL = 60 * time.Second

// settingsCache 实现 repository.SettingsCacheRepository，封装配置项的 Redis 缓存操作。
type settingsCache struct {
	client *redis.Client
}

// NewSettingsCache 创建配置缓存实现。
func NewSettingsCache(client *redis.Client) repository.SettingsCacheRepository {
	return &settingsCache{client: client}
}

// Set 将 namespace/key 配置值写入 Redis，TTL 固定 60 秒。
func (s *settingsCache) Set(ctx context.Context, namespace, key, value string) error {
	cacheKey := s.buildKey(namespace, key)
	return s.client.Set(ctx, cacheKey, value, settingsCacheTTL).Err()
}

// Del 删除 namespace/key 对应的 Redis 缓存项。
func (s *settingsCache) Del(ctx context.Context, namespace, key string) error {
	cacheKey := s.buildKey(namespace, key)
	return s.client.Del(ctx, cacheKey).Err()
}

// buildKey 返回统一格式的缓存键：settings:<namespace>:<key>。
func (s *settingsCache) buildKey(namespace, key string) string {
	return fmt.Sprintf("settings:%s:%s", namespace, key)
}
