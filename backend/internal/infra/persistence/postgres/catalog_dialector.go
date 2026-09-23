package db

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/migrator"
)

// catalogDialector keeps GORM's PostgreSQL migrator. On Nile, information_schema
// filters by the catalog name "nile", not current_database(). Ordinary Postgres
// is unchanged.
type catalogDialector struct {
	*postgres.Dialector
	nile bool
}

func newPostgresDialector(pool gorm.ConnPool, nile bool) gorm.Dialector {
	base := postgres.New(postgres.Config{Conn: pool}).(*postgres.Dialector)
	return &catalogDialector{Dialector: base, nile: nile}
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
	if d, ok := m.Dialector.(*catalogDialector); ok && d.nile {
		return nileInformationSchemaCatalog
	}
	return m.Migrator.CurrentDatabase()
}

// StoresVectorIndexColumn reports whether candidate vectors live in their own
// column. Nile rejects functions inside an index expression, so the HNSW index
// is a plain halfvec column written with the row.
func StoresVectorIndexColumn(db *gorm.DB) bool {
	if db == nil || db.Dialector == nil {
		return false
	}
	dialector, ok := db.Dialector.(*catalogDialector)
	return ok && dialector.nile
}
