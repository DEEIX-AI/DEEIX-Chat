package db

import (
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/migrator"
)

// catalogDialector keeps GORM's PostgreSQL migrator. CurrentDatabase stays on
// current_database() unless that name disagrees with current_catalog and
// information_schema is using the latter, which is what Nile does.
type catalogDialector struct {
	*postgres.Dialector
	once       sync.Once
	catalog    string
	useCatalog bool
}

func newPostgresDialector(pool gorm.ConnPool) gorm.Dialector {
	base := postgres.New(postgres.Config{Conn: pool}).(*postgres.Dialector)
	return &catalogDialector{Dialector: base}
}

func (d *catalogDialector) Migrator(db *gorm.DB) gorm.Migrator {
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
	if d, ok := m.Dialector.(*catalogDialector); ok {
		d.once.Do(func() {
			d.catalog, d.useCatalog = divergentInformationSchemaCatalog(m.DB)
		})
		if d.useCatalog {
			return d.catalog
		}
	}
	return m.Migrator.CurrentDatabase()
}

func divergentInformationSchemaCatalog(db *gorm.DB) (string, bool) {
	var databaseName, catalogName string
	var informationSchemaUsesCatalog bool
	err := db.Raw(`
		SELECT current_database(), current_catalog,
		       EXISTS (
		           SELECT 1 FROM information_schema.tables
		           WHERE table_catalog = current_catalog
		             AND table_catalog <> current_database()
		       )
	`).Row().Scan(&databaseName, &catalogName, &informationSchemaUsesCatalog)
	if err != nil {
		return "", false
	}
	return informationSchemaCatalog(databaseName, catalogName, informationSchemaUsesCatalog)
}

func informationSchemaCatalog(databaseName string, catalogName string, informationSchemaUsesCatalog bool) (string, bool) {
	if databaseName == "" || catalogName == "" || catalogName == databaseName || !informationSchemaUsesCatalog {
		return "", false
	}
	return catalogName, true
}
