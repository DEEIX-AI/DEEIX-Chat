package cache

import (
	"context"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/go-redis/redis/extra/redisotel/v8"
	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel/attribute"
)

// NewRedis 初始化 Redis 客户端并执行连通性校验。
func NewRedis(cfg config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Username: cfg.RedisUsername,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	client.AddHook(redisotel.NewTracingHook(
		redisotel.WithAttributes(
			attribute.String("db.system", "Redis"),
			attribute.String("server.address", cfg.RedisAddr),
			attribute.Int("db.redis.database_index", cfg.RedisDB),
		),
	))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}
