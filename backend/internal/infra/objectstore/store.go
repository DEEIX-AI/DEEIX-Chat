package objectstore

import (
	"context"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	portobjectstore "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/objectstore"
)

const (
	BackendLocal = "local"
	BackendS3    = "s3"
)

// 数据契约定义在 ports/objectstore，此处保留同名引用供实现使用。
var (
	ErrInvalidKey = portobjectstore.ErrInvalidKey
	ErrNotFound   = portobjectstore.ErrNotFound
)

type (
	PutOptions = portobjectstore.PutOptions
	ObjectInfo = portobjectstore.ObjectInfo
	Store      = portobjectstore.Store
)

func New(ctx context.Context, cfg config.Config) (Store, error) {
	switch normalizeBackend(cfg.StorageBackend) {
	case BackendS3:
		return NewS3(ctx, S3Config{
			Endpoint:        cfg.StorageS3Endpoint,
			Region:          cfg.StorageS3Region,
			Bucket:          cfg.StorageS3Bucket,
			Prefix:          cfg.StorageS3Prefix,
			AccessKeyID:     cfg.StorageS3AccessKeyID,
			SecretAccessKey: cfg.StorageS3SecretAccessKey,
			ForcePathStyle:  cfg.StorageS3ForcePathStyle,
		})
	default:
		return NewLocal(cfg.StorageRootDir), nil
	}
}

func normalizeBackend(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case BackendS3:
		return BackendS3
	default:
		return BackendLocal
	}
}
