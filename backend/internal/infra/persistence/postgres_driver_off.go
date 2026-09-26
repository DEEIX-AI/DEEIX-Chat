//go:build nopostgres

package persistence

import (
	"errors"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"gorm.io/gorm"
)

// ErrPostgresUnavailable is returned by builds made with -tags nopostgres.
var ErrPostgresUnavailable = errors.New("persistence: postgres driver not compiled into this binary; use database_driver: sqlite")

func openPostgres(config.Config) (*gorm.DB, error) {
	return nil, ErrPostgresUnavailable
}
