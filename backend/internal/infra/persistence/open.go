// Package persistence 负责选择 SQL 驱动。SQLite 始终可用；
// Postgres 可通过 -tags nopostgres 排除。
package persistence

import (
	"fmt"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	sqlitedb "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/sqlite"
	"gorm.io/gorm"
)

// Open 返回 cfg.DatabaseDriver 对应的连接。
func Open(cfg config.Config) (*gorm.DB, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.DatabaseDriver)) {
	case "", "postgres":
		return openPostgres(cfg)
	case "sqlite":
		return sqlitedb.New(cfg)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.DatabaseDriver)
	}
}
