//go:build cgo

package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mattn/go-sqlite3"
)

func (m *databaseBackupManager) restoreSQLite(ctx context.Context, sourcePath string) error {
	source, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(sourcePath)+"?mode=ro")
	if err != nil {
		return err
	}
	defer source.Close()

	var integrity string
	if err := source.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("validate sqlite backup: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("invalid sqlite backup: integrity check returned %q", integrity)
	}

	destination, err := m.db.DB()
	if err != nil {
		return err
	}
	sourceConn, err := source.Conn(ctx)
	if err != nil {
		return err
	}
	defer sourceConn.Close()
	destinationConn, err := destination.Conn(ctx)
	if err != nil {
		return err
	}
	defer destinationConn.Close()

	return destinationConn.Raw(func(destinationDriver any) error {
		dst, ok := destinationDriver.(*sqlite3.SQLiteConn)
		if !ok {
			return errors.New("unexpected sqlite destination driver")
		}
		return sourceConn.Raw(func(sourceDriver any) error {
			src, ok := sourceDriver.(*sqlite3.SQLiteConn)
			if !ok {
				return errors.New("unexpected sqlite source driver")
			}
			backup, err := dst.Backup("main", src, "main")
			if err != nil {
				return err
			}
			defer backup.Close()
			for {
				done, err := backup.Step(256)
				if err != nil {
					return err
				}
				if done {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(10 * time.Millisecond):
				}
			}
		})
	})
}
