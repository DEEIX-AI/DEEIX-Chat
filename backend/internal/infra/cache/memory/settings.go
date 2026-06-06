package memory

import (
	"context"
	"time"
)

func (c *Cache) Set(ctx context.Context, namespace, key, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.settings[settingsKey(namespace, key)] = expiringString{value: value, expiresAt: now.Add(time.Minute)}
	c.maybeSweepLocked(now)
	return nil
}

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
