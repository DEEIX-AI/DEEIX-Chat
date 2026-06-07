package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	memorycache "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/memory"
	rediscache "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/redis"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	postgresdb "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres"
	sqlitedb "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/sqlite"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	platformhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

func openDatabase(cfg config.Config) (*gorm.DB, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.DatabaseDriver)) {
	case "", "postgres":
		return postgresdb.New(cfg)
	case "sqlite":
		return sqlitedb.New(cfg)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.DatabaseDriver)
	}
}

func openCache(cfg config.Config) (*redis.Client, *memorycache.Cache, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.CacheDriver)) {
	case "", "redis":
		client, err := rediscache.NewRedis(cfg)
		if err != nil {
			return nil, nil, err
		}
		return client, nil, nil
	case "memory":
		return nil, memorycache.New(), nil
	default:
		return nil, nil, fmt.Errorf("unsupported cache driver %q", cfg.CacheDriver)
	}
}

func buildSettingsCache(cfg config.Config, redisClient *redis.Client, memoryCache *memorycache.Cache) repository.SettingsCacheRepository {
	if useRedisCache(cfg, redisClient) {
		return rediscache.NewSettingsCache(redisClient)
	}
	if memoryCache != nil {
		return memorycache.NewSettingsCache(memoryCache)
	}
	return nil
}

func buildChannelCache(cfg config.Config, redisClient *redis.Client, memoryCache *memorycache.Cache) repository.ChannelCacheRepository {
	if useRedisCache(cfg, redisClient) {
		return rediscache.NewChannelCache(redisClient)
	}
	if memoryCache != nil {
		return memorycache.NewChannelCache(memoryCache)
	}
	return nil
}

func buildConversationCache(cfg config.Config, redisClient *redis.Client, memoryCache *memorycache.Cache) repository.ConversationCacheRepository {
	if useRedisCache(cfg, redisClient) {
		return rediscache.NewConversationCache(redisClient)
	}
	if memoryCache != nil {
		return memorycache.NewConversationCache(memoryCache)
	}
	return nil
}

func buildRateLimiter(cfg config.Config, redisClient *redis.Client, memoryCache *memorycache.Cache) middleware.RateLimiter {
	if useRedisCache(cfg, redisClient) {
		return rediscache.NewRateLimiter(redisClient)
	}
	if memoryCache != nil {
		return memorycache.NewRateLimiter(memoryCache)
	}
	return nil
}

func useRedisCache(cfg config.Config, redisClient *redis.Client) bool {
	return redisClient != nil && strings.EqualFold(strings.TrimSpace(cfg.CacheDriver), "redis")
}

type healthChecker struct {
	db          *gorm.DB
	cacheDriver string
	redis       *redis.Client
}

func newHealthChecker(db *gorm.DB, cacheDriver string, redisClient *redis.Client) platformhttp.HealthChecker {
	return &healthChecker{
		db:          db,
		cacheDriver: strings.ToLower(strings.TrimSpace(cacheDriver)),
		redis:       redisClient,
	}
}

func (h *healthChecker) CheckHealth(ctx context.Context) ([]platformhttp.HealthCheck, bool) {
	checks := make([]platformhttp.HealthCheck, 0, 2)
	healthy := true

	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err != nil {
			checks = append(checks, platformhttp.HealthCheck{Name: "db", Status: "error: " + err.Error()})
			healthy = false
		} else {
			dbCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if err = sqlDB.PingContext(dbCtx); err != nil {
				checks = append(checks, platformhttp.HealthCheck{Name: "db", Status: "error"})
				healthy = false
			} else {
				checks = append(checks, platformhttp.HealthCheck{Name: "db", Status: "ok"})
			}
		}
	} else {
		checks = append(checks, platformhttp.HealthCheck{Name: "db", Status: "not_configured"})
	}

	switch h.cacheDriver {
	case "redis", "":
		redisCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if h.redis == nil {
			checks = append(checks, platformhttp.HealthCheck{Name: "redis", Status: "not_configured"})
			healthy = false
		} else if err := h.redis.Ping(redisCtx).Err(); err != nil {
			checks = append(checks, platformhttp.HealthCheck{Name: "redis", Status: "error"})
			healthy = false
		} else {
			checks = append(checks, platformhttp.HealthCheck{Name: "redis", Status: "ok"})
		}
	case "memory":
		checks = append(checks, platformhttp.HealthCheck{Name: "cache", Status: "memory"})
	default:
		checks = append(checks, platformhttp.HealthCheck{Name: "cache", Status: "unsupported"})
		healthy = false
	}

	return checks, healthy
}
