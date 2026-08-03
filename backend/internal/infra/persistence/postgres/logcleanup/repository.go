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
		result = query.Where("created_at < ?", before).Delete(&model.AuditLog{})
	case repository.LogCleanupTypeAuth:
		result = query.Where("occurred_at < ?", before).Delete(&model.UserAuthEvent{})
	case repository.LogCleanupTypeUsage:
		result = query.Where("created_at < ?", before).Delete(&model.UsageLedger{})
	case repository.LogCleanupTypeOrders:
		result = query.Where("created_at < ?", before).Delete(&model.PaymentOrder{})
	case repository.LogCleanupTypeConversation:
		result = query.Where("created_at < ?", before).Delete(&model.ChatRunEvent{})
	case repository.LogCleanupTypeSystem:
		result = query.Where("created_at < ?", before).Delete(&model.SystemEvent{})
	default:
		return 0, fmt.Errorf("unsupported log cleanup type: %s", logType)
	}

	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// DeleteConversationRuns 物理删除指定运行的全部对话事件。
func (r *Repo) DeleteConversationRuns(ctx context.Context, runIDs []string) (int64, error) {
	if len(runIDs) == 0 {
		return 0, nil
	}
	var deletedCount int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Unscoped().Where("run_id IN ?", runIDs).Delete(&model.ChatRunEvent{})
		if result.Error != nil {
			return result.Error
		}
		deletedCount = result.RowsAffected
		return nil
	})
	return deletedCount, err
}
