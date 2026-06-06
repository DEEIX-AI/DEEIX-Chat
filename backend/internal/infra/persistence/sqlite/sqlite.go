package sqlite

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/schema"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/sqlitevec"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// New initializes a SQLite connection for single-node deployments.
func New(cfg config.Config) (*gorm.DB, error) {
	dsn, err := sqliteDSN(cfg)
	if err != nil {
		return nil, err
	}
	sqlitevec.Register()
	db, err := gorm.Open(sqlite.Open(dsn), newGORMConfig(cfg))
	if err != nil {
		return nil, err
	}
	if err = configureSQLiteConnection(db, cfg); err != nil {
		return nil, err
	}
	if err = schema.Migrate(db); err != nil {
		return nil, err
	}
	if err = sqlitevec.Migrate(db); err != nil {
		return nil, err
	}
	if err = schema.SeedLLMSettings(db); err != nil {
		return nil, err
	}
	if err = schema.SeedBillingCatalog(db); err != nil {
		return nil, err
	}
	return db, nil
}

func sqliteDSN(cfg config.Config) (string, error) {
	if dsn := strings.TrimSpace(cfg.SQLiteDSN); dsn != "" {
		return dsn, nil
	}
	path := strings.TrimSpace(cfg.SQLitePath)
	if path == "" {
		return "", fmt.Errorf("sqlite path is empty")
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	query := url.Values{}
	query.Set("_busy_timeout", fmt.Sprintf("%d", cfg.SQLiteBusyTimeoutMS))
	query.Set("_cache_size", fmt.Sprintf("%d", -cfg.SQLiteCacheSizeKB))
	query.Set("_foreign_keys", "on")
	query.Set("_journal_mode", "WAL")
	query.Set("_synchronous", cfg.SQLiteSynchronous)
	return "file:" + path + "?" + query.Encode(), nil
}

func configureSQLiteConnection(db *gorm.DB, cfg config.Config) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	maxOpen := cfg.SQLiteMaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 1
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxOpen)
	sqlDB.SetConnMaxLifetime(0)
	if err := db.Exec(`PRAGMA foreign_keys = ON`).Error; err != nil {
		return err
	}
	if err := db.Exec(`PRAGMA journal_mode = WAL`).Error; err != nil {
		return err
	}
	if err := db.Exec(fmt.Sprintf(`PRAGMA synchronous = %s`, cfg.SQLiteSynchronous)).Error; err != nil {
		return err
	}
	if err := db.Exec(fmt.Sprintf(`PRAGMA busy_timeout = %d`, cfg.SQLiteBusyTimeoutMS)).Error; err != nil {
		return err
	}
	if err := db.Exec(fmt.Sprintf(`PRAGMA cache_size = %d`, -cfg.SQLiteCacheSizeKB)).Error; err != nil {
		return err
	}
	if err := db.Exec(fmt.Sprintf(`PRAGMA mmap_size = %d`, cfg.SQLiteMmapSizeBytes)).Error; err != nil {
		return err
	}
	if err := db.Exec(fmt.Sprintf(`PRAGMA temp_store = %s`, cfg.SQLiteTempStore)).Error; err != nil {
		return err
	}
	if err := db.Exec(`PRAGMA optimize`).Error; err != nil {
		return err
	}
	return nil
}

func newGORMConfig(cfg config.Config) *gorm.Config {
	gormConfig := &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	}
	if isProductionEnv(cfg.Env) {
		gormConfig.Logger = gormlogger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		})
	}
	return gormConfig
}

func isProductionEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}
