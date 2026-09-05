package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/go-redis/redis/v8"
)

const consumeProviderAuthRecordScript = `
local value = redis.call("GET", KEYS[1])
if not value then
  return nil
end
redis.call("DEL", KEYS[1])
return value
`

type providerAuthBridge struct {
	client *redis.Client
}

// NewProviderAuthBridge creates the Redis-backed provider auth bridge store.
func NewProviderAuthBridge(client *redis.Client) repository.ProviderAuthBridgeRepository {
	return &providerAuthBridge{client: client}
}

func (s *providerAuthBridge) PutProviderAuthTransaction(ctx context.Context, id string, item repository.ProviderAuthTransaction, ttl time.Duration) error {
	return s.put(ctx, providerAuthTransactionKey(id), item, ttl)
}

func (s *providerAuthBridge) ConsumeProviderAuthTransaction(ctx context.Context, id string) (*repository.ProviderAuthTransaction, error) {
	var item repository.ProviderAuthTransaction
	if err := s.consume(ctx, providerAuthTransactionKey(id), &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *providerAuthBridge) PutProviderAuthGrant(ctx context.Context, key string, item repository.ProviderAuthGrant, ttl time.Duration) error {
	return s.put(ctx, providerAuthGrantKey(key), item, ttl)
}

func (s *providerAuthBridge) ConsumeProviderAuthGrant(ctx context.Context, key string) (*repository.ProviderAuthGrant, error) {
	var item repository.ProviderAuthGrant
	if err := s.consume(ctx, providerAuthGrantKey(key), &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *providerAuthBridge) put(ctx context.Context, key string, item any, ttl time.Duration) error {
	if s == nil || s.client == nil || key == "" || ttl <= 0 {
		return repository.ErrInvalidInput
	}
	payload, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, payload, ttl).Err()
}

func (s *providerAuthBridge) consume(ctx context.Context, key string, destination any) error {
	if s == nil || s.client == nil || key == "" {
		return repository.ErrNotFound
	}
	value, err := s.client.Eval(ctx, consumeProviderAuthRecordScript, []string{key}).Text()
	if errors.Is(err, redis.Nil) {
		return repository.ErrNotFound
	}
	if err != nil {
		return err
	}
	if err = json.Unmarshal([]byte(value), destination); err != nil {
		return fmt.Errorf("decode provider auth bridge record: %w", err)
	}
	return nil
}

func providerAuthTransactionKey(id string) string {
	return "auth:provider:transaction:" + id
}

func providerAuthGrantKey(key string) string {
	return "auth:provider:grant:" + key
}
