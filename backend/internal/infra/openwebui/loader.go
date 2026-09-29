package openwebui

import (
	"context"
	"net/url"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// RowLoader 从外部 OpenWebUI 数据库读取用户。
type RowLoader struct{}

// NewRowLoader 创建 OpenWebUI 行加载器。
func NewRowLoader() RowLoader {
	return RowLoader{}
}

type userRow struct {
	PublicID    string  `gorm:"column:public_id"`
	Username    string  `gorm:"column:username"`
	DisplayName string  `gorm:"column:display_name"`
	Email       string  `gorm:"column:email"`
	Balance     float64 `gorm:"column:balance"`
}

// LoadOpenWebUIRows 仅在单次导入期间打开外部数据库。
func (RowLoader) LoadOpenWebUIRows(ctx context.Context, dsn string) ([]repository.OpenWebUIUserRow, error) {
	db, err := openDB(dsn)
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	rows := make([]userRow, 0)
	query := importQuery(db.Migrator().HasTable("credit"))
	if err = db.WithContext(ctx).Raw(query).Scan(&rows).Error; err != nil {
		return nil, err
	}

	results := make([]repository.OpenWebUIUserRow, 0, len(rows))
	for _, row := range rows {
		results = append(results, repository.OpenWebUIUserRow{
			PublicID:    row.PublicID,
			Username:    row.Username,
			DisplayName: row.DisplayName,
			Email:       row.Email,
			Balance:     row.Balance,
		})
	}
	return results, nil
}

func openDB(dsn string) (*gorm.DB, error) {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return nil, repository.ErrInvalidInput
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		if _, err := url.Parse(trimmed); err != nil {
			return nil, repository.ErrInvalidInput
		}
		return gorm.Open(gormpostgres.Open(trimmed), &gorm.Config{})
	}
	const sqliteScheme = "sqlite://"
	if strings.HasPrefix(lower, sqliteScheme) {
		trimmed = trimmed[len(sqliteScheme):]
		if strings.TrimSpace(trimmed) == "" {
			return nil, repository.ErrInvalidInput
		}
	}
	return gorm.Open(sqlite.Open(trimmed), &gorm.Config{})
}

func importQuery(hasCredit bool) string {
	if hasCredit {
		return `select u.id as "public_id",
       u.id as "username",
       u.name as "display_name",
       u.email as "email",
       coalesce(c.credit, 0) as "balance"
from "user" as u
         left join "credit" as c
              on u.id = c.user_id`
	}
	return `select u.id as "public_id",
       u.id as "username",
       u.name as "display_name",
       u.email as "email",
       0 as "balance"
from "user" as u`
}
