// Package persistence selects the SQL driver. SQLite is always available;
// Postgres is excluded with -tags nopostgres.
package persistence

import (
	"fmt"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	sqlitedb "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/sqlite"
	"gorm.io/gorm"
)

// Open returns a connection for cfg.DatabaseDriver.
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
