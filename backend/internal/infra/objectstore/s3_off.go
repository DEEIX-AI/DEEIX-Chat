//go:build nos3

package objectstore

import (
	"context"
	"errors"
)

// ErrS3Unavailable is returned by builds made with -tags nos3.
var ErrS3Unavailable = errors.New("objectstore: s3 backend not compiled into this binary")

func newS3(context.Context, S3Config) (Store, error) {
	return nil, ErrS3Unavailable
}
