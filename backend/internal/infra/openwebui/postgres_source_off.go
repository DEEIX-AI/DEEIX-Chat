//go:build nopostgres

package openwebui

import (
	"fmt"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// openPostgres 在 -tags nopostgres 构建（桌面端本地模式）中不可用：只支持从 SQLite 导入，
// PostgreSQL 来源按无效 DSN 处理，避免为一次性导入链接整个 PostgreSQL 驱动。
func openPostgres(string) (*gorm.DB, error) {
	return nil, fmt.Errorf("%w: PostgreSQL import sources are not compiled into this binary", repository.ErrInvalidInput)
}
