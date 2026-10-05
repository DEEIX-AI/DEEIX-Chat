//go:build !nopostgres

package openwebui

import (
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openPostgres(dsn string) (*gorm.DB, error) {
	return gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
}
