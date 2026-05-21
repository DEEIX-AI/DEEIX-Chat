package objectstorage

import (
	"context"

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
	return p.factory(ctx, p.cfg.Snapshot())
}
