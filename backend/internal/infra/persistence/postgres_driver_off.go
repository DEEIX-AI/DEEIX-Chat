//go:build nopostgres

package persistence

import (
	"errors"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"gorm.io/gorm"
)

// ErrPostgresUnavailable 在使用 -tags nopostgres 构建时返回。
var ErrPostgresUnavailable = errors.New("persistence: postgres driver not compiled into this binary; use database_driver: sqlite")

func openPostgres(config.Config) (*gorm.DB, error) {
	return nil, ErrPostgresUnavailable
}
