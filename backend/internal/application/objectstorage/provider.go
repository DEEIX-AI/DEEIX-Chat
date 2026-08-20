package objectstorage

import (
	"context"
	"sync"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstore"
)

// Provider 为应用服务提供对象存储能力，隔离具体存储实现的创建方式。
type Provider interface {
	Open(ctx context.Context) (objectstore.Store, error)
}

// Factory 创建对象存储实例。
type Factory func(ctx context.Context, cfg config.Config) (objectstore.Store, error)

// RuntimeProvider 基于运行时配置创建对象存储实例。
type RuntimeProvider struct {
	cfg     *config.Runtime
	factory Factory

	mu           sync.Mutex
	cachedKey    runtimeStorageKey
	cachedStore  objectstore.Store
	cachePresent bool
}

type runtimeStorageKey struct {
	backend           string
	rootDir           string
	s3Endpoint        string
	s3Region          string
	s3Bucket          string
	s3Prefix          string
	s3AccessKeyID     string
	s3SecretAccessKey string
	s3ForcePathStyle  bool
}

// NewRuntimeProvider 创建对象存储 provider。
func NewRuntimeProvider(cfg *config.Runtime, factory Factory) *RuntimeProvider {
	if factory == nil {
		factory = objectstore.New
	}
	return &RuntimeProvider{cfg: cfg, factory: factory}
}

// Open 打开当前配置对应的对象存储。
func (p *RuntimeProvider) Open(ctx context.Context) (objectstore.Store, error) {
	cfg := p.cfg.Snapshot()
	key := newRuntimeStorageKey(cfg)

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cachePresent && p.cachedKey == key {
		return p.cachedStore, nil
	}
	store, err := p.factory(ctx, cfg)
	if err != nil {
		return nil, err
	}
	p.cachedKey = key
	p.cachedStore = store
	p.cachePresent = true
	return store, nil
}

func newRuntimeStorageKey(cfg config.Config) runtimeStorageKey {
	return runtimeStorageKey{
		backend: cfg.StorageBackend, rootDir: cfg.StorageRootDir,
		s3Endpoint: cfg.StorageS3Endpoint, s3Region: cfg.StorageS3Region,
		s3Bucket: cfg.StorageS3Bucket, s3Prefix: cfg.StorageS3Prefix,
		s3AccessKeyID: cfg.StorageS3AccessKeyID, s3SecretAccessKey: cfg.StorageS3SecretAccessKey,
		s3ForcePathStyle: cfg.StorageS3ForcePathStyle,
	}
}
