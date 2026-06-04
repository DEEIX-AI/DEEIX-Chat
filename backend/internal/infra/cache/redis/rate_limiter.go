package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

// rateLimiter 提供基于 Redis 的 HTTP 限流存储能力。
type rateLimiter struct {
	client *redis.Client
}

// NewRateLimiter 创建 Redis 限流器。
func NewRateLimiter(client *redis.Client) *rateLimiter {
	if client == nil {
		return nil
	}
	return &rateLimiter{client: client}
}

// AllowSlidingWindow 使用有序集合实现滑动窗口限流。
func (r *rateLimiter) AllowSlidingWindow(ctx context.Context, key string, limit int, window time.Duration, ttl time.Duration) (bool, error) {
	if r == nil || r.client == nil || key == "" || limit <= 0 {
		return true, nil
	}
	if window <= 0 {
		window = time.Minute
	}
	if ttl <= 0 {
		ttl = window * 2
	}

	nowNanos := time.Now().UnixNano()
	now := nowNanos / int64(time.Millisecond)
	windowStart := now - window.Milliseconds()
	member := strconv.FormatInt(nowNanos, 10)

	pipe := r.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	countCmd := pipe.ZCard(ctx, key)
	pipe.ZAdd(ctx, key, &redis.Z{Score: float64(now), Member: member})
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return true, err
	}
	return countCmd.Val() < int64(limit), nil
}

// AllowFixedWindow 使用计数器实现固定窗口限流。
func (r *rateLimiter) AllowFixedWindow(ctx context.Context, keys []string, limit int, ttl time.Duration) (bool, error) {
	if r == nil || r.client == nil || len(keys) == 0 || limit <= 0 {
		return true, nil
	}
	if ttl <= 0 {
		ttl = time.Minute
	}

	pipe := r.client.Pipeline()
	incrCmds := make([]*redis.IntCmd, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		incrCmds = append(incrCmds, pipe.Incr(ctx, key))
		pipe.Expire(ctx, key, ttl)
	}
	if len(incrCmds) == 0 {
		return true, nil
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return true, err
	}

	for _, cmd := range incrCmds {
		if cmd.Val() > int64(limit) {
			return false, nil
		}
	}
	return true, nil
}
