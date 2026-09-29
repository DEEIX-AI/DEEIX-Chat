package objectstorage

import (
	"context"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	portobjectstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/objectstorage"
)

const (
	BackendLocal = "local"
	BackendS3    = "s3"
)

// 数据契约定义在 ports/objectstorage，此处保留同名引用供实现使用。
var (
	ErrInvalidKey = portobjectstorage.ErrInvalidKey
	ErrNotFound   = portobjectstorage.ErrNotFound
)

type (
	PutOptions = portobjectstorage.PutOptions
	ObjectInfo = portobjectstorage.ObjectInfo
	Store      = portobjectstorage.Store
)

// S3Config 定义在 s3.go 之外，以便在 -tags nos3 构建中仍然存在。
type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
}

func New(ctx context.Context, cfg config.Config) (Store, error) {
	switch normalizeBackend(cfg.StorageBackend) {
	case BackendS3:
		return newS3(ctx, S3Config{
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
