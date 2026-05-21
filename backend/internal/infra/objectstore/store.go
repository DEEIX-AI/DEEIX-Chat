package objectstore

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

const (
	BackendLocal = "local"
	BackendS3    = "s3"
)

var (
	ErrInvalidKey = errors.New("invalid object key")
	ErrNotFound   = errors.New("object not found")
)

type PutOptions struct {
	SizeBytes   int64
	ContentType string
}

type ObjectInfo struct {
	Key         string
	SizeBytes   int64
	ContentType string
	ModTime     time.Time
}

type Store interface {
	Put(ctx context.Context, key string, body io.Reader, opts PutOptions) (ObjectInfo, error)
	Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, key string) error
	Materialize(ctx context.Context, key string) (string, func(), error)
}

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
