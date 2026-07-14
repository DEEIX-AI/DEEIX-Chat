package admin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"gorm.io/gorm"
)

const maxDatabaseRestoreBytes int64 = 1024 * 1024 * 1024

var errDatabaseOperationBusy = errors.New("database backup or restore is already running")

type databaseBackupManager struct {
	db  *gorm.DB
	cfg config.Config
	mu  sync.Mutex
}

type databaseBackupFile struct {
	Path string
	Name string
}

func NewDatabaseBackupManager(db *gorm.DB, cfg config.Config) *databaseBackupManager {
	return &databaseBackupManager{db: db, cfg: cfg}
}

func (m *databaseBackupManager) driver() string {
	driver := strings.ToLower(strings.TrimSpace(m.cfg.DatabaseDriver))
	if driver == "" {
		return "postgres"
	}
	return driver
}

func (m *databaseBackupManager) withLock(fn func() error) error {
	if !m.mu.TryLock() {
		return errDatabaseOperationBusy
	}
	defer m.mu.Unlock()
	return fn()
}

func (m *databaseBackupManager) Backup(ctx context.Context) (databaseBackupFile, error) {
	var result databaseBackupFile
	err := m.withLock(func() error {
		timestamp := time.Now().UTC().Format("20060102T150405Z")
		switch m.driver() {
		case "sqlite":
			path, err := tempBackupPath("deeix-chat-"+timestamp+"-", ".sqlite3")
			if err != nil {
				return err
			}
			if err := m.backupSQLite(ctx, path); err != nil {
				_ = os.Remove(path)
				return err
			}
			result = databaseBackupFile{Path: path, Name: "deeix-chat-" + timestamp + ".sqlite3"}
			return nil
		case "postgres":
			path, err := tempBackupPath("deeix-chat-"+timestamp+"-", ".dump")
			if err != nil {
				return err
			}
			cmd := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-privileges", "--file", path, m.cfg.PostgresDSN)
			if output, err := cmd.CombinedOutput(); err != nil {
				_ = os.Remove(path)
				return fmt.Errorf("pg_dump failed: %w: %s", err, strings.TrimSpace(string(output)))
			}
			result = databaseBackupFile{Path: path, Name: "deeix-chat-" + timestamp + ".dump"}
			return nil
		default:
			return fmt.Errorf("unsupported database driver %q", m.driver())
		}
	})
	return result, err
}

func (m *databaseBackupManager) Restore(ctx context.Context, sourcePath string) error {
	return m.withLock(func() error {
		switch m.driver() {
		case "sqlite":
			return m.restoreSQLite(ctx, sourcePath)
		case "postgres":
			cmd := exec.CommandContext(ctx, "pg_restore", "--clean", "--if-exists", "--no-owner", "--no-privileges", "--single-transaction", "--dbname", m.cfg.PostgresDSN, sourcePath)
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("pg_restore failed: %w: %s", err, strings.TrimSpace(string(output)))
			}
			return nil
		default:
			return fmt.Errorf("unsupported database driver %q", m.driver())
		}
	})
}

func tempBackupPath(pattern, suffix string) (string, error) {
	file, err := os.CreateTemp("", pattern+"*"+suffix)
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func (m *databaseBackupManager) backupSQLite(ctx context.Context, path string) error {
	escaped := strings.ReplaceAll(filepath.ToSlash(path), "'", "''")
	return m.db.WithContext(ctx).Exec("VACUUM INTO '" + escaped + "'").Error
}

func copyRestoreUpload(dst *os.File, src io.Reader) (int64, error) {
	return io.Copy(dst, io.LimitReader(src, maxDatabaseRestoreBytes+1))
}
