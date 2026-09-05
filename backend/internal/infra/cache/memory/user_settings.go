package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GetUserSettingCacheVersion returns the current version for a user's setting key.
func (c *Cache) GetUserSettingCacheVersion(_ context.Context, userID uint, key string, ttl time.Duration) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	cacheKey := userSettingVersionKey(userID, key)
	item, ok := c.userSettingVersions[cacheKey]
	if ok && now.Before(item.expiresAt) {
		item.expiresAt = ttlFromNow(ttl)
		c.userSettingVersions[cacheKey] = item
		return item.value, nil
	}
	version := uuid.NewString()
	c.userSettingVersions[cacheKey] = expiringString{value: version, expiresAt: ttlFromNow(ttl)}
	c.maybeSweepLocked(now)
	return version, nil
}

// AdvanceUserSettingCacheVersion creates and stores a new version for a user's setting key.
func (c *Cache) AdvanceUserSettingCacheVersion(_ context.Context, userID uint, key string, ttl time.Duration) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cacheKey := userSettingVersionKey(userID, key)
	version := uuid.NewString()
	c.userSettingVersions[cacheKey] = expiringString{value: version, expiresAt: ttlFromNow(ttl)}
	c.maybeSweepLocked(time.Now())
	return version, nil
}

// GetUserSettingCache returns a versioned value for a user's setting key.
func (c *Cache) GetUserSettingCache(_ context.Context, userID uint, key, version string) (string, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	cacheKey := userSettingValueKey(userID, key, version)
	item, ok := c.userSettings[cacheKey]
	if !ok {
		return "", false, nil
	}
	if !now.Before(item.expiresAt) {
		delete(c.userSettings, cacheKey)
		return "", false, nil
	}
	return item.value, true, nil
}

// SetUserSettingCache stores a versioned value for a user's setting key.
func (c *Cache) SetUserSettingCache(_ context.Context, userID uint, key, version, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.userSettings[userSettingValueKey(userID, key, version)] = expiringString{
		value:     value,
		expiresAt: ttlFromNow(ttl),
	}
	c.maybeSweepLocked(now)
	return nil
}

func userSettingVersionKey(userID uint, key string) string {
	return fmt.Sprintf("%d:%s", userID, key)
}

func userSettingValueKey(userID uint, key, version string) string {
	return fmt.Sprintf("%d:%s:%s", userID, key, version)
}
