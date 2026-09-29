package memory

import (
	"context"
	"time"
)

// Set 以缓存默认过期时间保存带命名空间的设置。
func (c *Cache) Set(ctx context.Context, namespace, key, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.settings[settingsKey(namespace, key)] = expiringString{value: value, expiresAt: now.Add(time.Minute)}
	c.maybeSweepLocked(now)
	return nil
}

// Del 从缓存中移除带命名空间的设置。
func (c *Cache) Del(ctx context.Context, namespace, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.settings, settingsKey(namespace, key))
	c.maybeSweepLocked(time.Now())
	return nil
}

func settingsKey(namespace, key string) string {
	return namespace + ":" + key
}
