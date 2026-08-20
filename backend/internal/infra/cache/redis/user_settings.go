package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

var getUserSettingCacheVersionScript = redis.NewScript(`
local value = redis.call("GET", KEYS[1])
if value then
  redis.call("PEXPIRE", KEYS[1], ARGV[2])
  return value
end
redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
return ARGV[1]
`)

func (c *conversationCache) GetUserSettingCacheVersion(ctx context.Context, userID uint, key string, ttl time.Duration) (string, error) {
	version := uuid.NewString()
	return getUserSettingCacheVersionScript.Run(
		ctx,
		c.client,
		[]string{userSettingVersionKey(userID, key)},
		version,
		ttl.Milliseconds(),
	).Text()
}

func (c *conversationCache) AdvanceUserSettingCacheVersion(ctx context.Context, userID uint, key string, ttl time.Duration) (string, error) {
	version := uuid.NewString()
	if err := c.client.Set(ctx, userSettingVersionKey(userID, key), version, ttl).Err(); err != nil {
		return "", err
	}
	return version, nil
}

func (c *conversationCache) GetUserSettingCache(ctx context.Context, userID uint, key, version string) (string, bool, error) {
	value, err := c.client.Get(ctx, userSettingValueKey(userID, key, version)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (c *conversationCache) SetUserSettingCache(ctx context.Context, userID uint, key, version, value string, ttl time.Duration) error {
	return c.client.Set(ctx, userSettingValueKey(userID, key, version), value, ttl).Err()
}

func userSettingVersionKey(userID uint, key string) string {
	return fmt.Sprintf("conversation:user-setting:{%d}:%s:version", userID, key)
}

func userSettingValueKey(userID uint, key, version string) string {
	return fmt.Sprintf("conversation:user-setting:{%d}:%s:value:%s", userID, key, version)
}
