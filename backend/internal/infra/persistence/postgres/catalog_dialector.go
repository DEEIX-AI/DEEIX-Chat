package db

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/migrator"
)

// catalogDialector keeps GORM's PostgreSQL migrator, but reports the catalog
// name information_schema actually uses. On Nile, current_database() is the
// logical database name while information_schema.table_catalog is current_catalog.
type catalogDialector struct {
	*postgres.Dialector
}

func newPostgresDialector(pool gorm.ConnPool) gorm.Dialector {
	base := postgres.New(postgres.Config{Conn: pool}).(*postgres.Dialector)
	return catalogDialector{Dialector: base}
}

func (d catalogDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return catalogMigrator{Migrator: postgres.Migrator{Migrator: migrator.Migrator{Config: migrator.Config{
		DB:                          db,
		Dialector:                   d,
		CreateIndexAfterCreateTable: true,
	}}}}
}

type catalogMigrator struct {
	postgres.Migrator
}

func (m catalogMigrator) CurrentDatabase() (name string) {
	m.DB.Raw("SELECT current_catalog").Scan(&name)
	return
}
