//go:build !nopostgres

package persistence

import (
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	postgresdb "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres"
	"gorm.io/gorm"
)

func openPostgres(cfg config.Config) (*gorm.DB, error) {
	return postgresdb.New(cfg)
}
