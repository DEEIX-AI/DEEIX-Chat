//go:build !cgo

package admin

import (
	"context"
	"errors"
)

func (m *databaseBackupManager) restoreSQLite(ctx context.Context, sourcePath string) error {
	_ = ctx
	_ = sourcePath
	return errors.New("sqlite restore requires a CGO-enabled build with a C compiler")
}
