// Package objectstore 定义对象存储端口的数据契约。
package objectstore

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	// ErrInvalidKey 表示对象键不合法。
	ErrInvalidKey = errors.New("invalid object key")
	// ErrNotFound 表示对象不存在。
	ErrNotFound = errors.New("object not found")
)

// PutOptions 描述写入对象时的元信息。
type PutOptions struct {
	SizeBytes   int64
	ContentType string
}

// ObjectInfo 描述对象的键、大小、类型与修改时间。
type ObjectInfo struct {
	Key         string
	SizeBytes   int64
	ContentType string
	ModTime     time.Time
}

// Store 定义对象存储的读写能力。
type Store interface {
	Put(ctx context.Context, key string, body io.Reader, opts PutOptions) (ObjectInfo, error)
	Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, key string) error
	Materialize(ctx context.Context, key string) (string, func(), error)
}
