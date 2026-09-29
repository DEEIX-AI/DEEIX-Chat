//go:build nos3

package objectstorage

import (
	"context"
	"errors"
)

// ErrS3Unavailable 在使用 -tags nos3 构建时返回。
var ErrS3Unavailable = errors.New("objectstorage: s3 backend not compiled into this binary")

func newS3(context.Context, S3Config) (Store, error) {
	return nil, ErrS3Unavailable
}
