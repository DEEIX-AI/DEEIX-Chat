package logcleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// Repo 封装管理员日志物理清理。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建日志清理仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// DeleteBefore 按固定日志类型映射物理删除截止时间之前的数据。
func (r *Repo) DeleteBefore(ctx context.Context, logType string, before time.Time) (int64, error) {
	query := r.db.WithContext(ctx).Unscoped()
	var result *gorm.DB

	switch logType {
	case repository.LogCleanupTypeAudit:
		result = query.Where("created_at < ?", before).Delete(&models.AuditLog{})
	case repository.LogCleanupTypeAuth:
		result = query.Where("occurred_at < ?", before).Delete(&models.UserAuthEvent{})
	case repository.LogCleanupTypeUsage:
		result = query.Where("created_at < ?", before).Delete(&models.UsageLedger{})
	case repository.LogCleanupTypeOrders:
		result = query.Where("created_at < ?", before).Delete(&models.PaymentOrder{})
	case repository.LogCleanupTypeConversation:
		result = query.Where("created_at < ?", before).Delete(&models.ChatRunEvent{})
	case repository.LogCleanupTypeModeration:
		result = query.Where("created_at < ?", before).Delete(&models.ContentModerationEvent{})
	case repository.LogCleanupTypeErrors:
		result = query.Where("created_at < ?", before).Delete(&models.ErrorLog{})
	default:
		return 0, fmt.Errorf("unsupported log cleanup type: %s", logType)
	}

	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
