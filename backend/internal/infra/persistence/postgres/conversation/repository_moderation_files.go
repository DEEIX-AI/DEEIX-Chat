package conversation

import (
	"context"
	"strings"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/gorm"
)

// GetFileObjectByFileIDAnyStatus 加载文件行，不限归属/状态。
func (r *Repo) GetFileObjectByFileIDAnyStatus(ctx context.Context, fileID string) (*domainconversation.FileObject, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, nil
	}
	var file models.FileObject
	if err := r.db.WithContext(ctx).Where("file_id = ?", fileID).First(&file).Error; err != nil {
		if dberror.IsRecordNotFound(err) || err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, dberror.Translate(err)
	}
	item := toFileObjectDomain(file)
	return &item, nil
}

// ListModerationBlockedFileIDsForCleanup 返回物理对象仍需删除的已撤销文件。
// storage_path 仅在对象存储删除成功后才清空。
func (r *Repo) ListModerationBlockedFileIDsForCleanup(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}
	var fileIDs []string
	err := r.db.WithContext(ctx).Model(&models.FileObject{}).
		Where("status = ? AND storage_path <> ''", "moderation_blocked").
		Order("id ASC").
		Limit(limit).
		Pluck("file_id", &fileIDs).Error
	return fileIDs, dberror.Translate(err)
}

// RevokeGeneratedFileForModeration 将生成文件标记为不可访问，并解除用户归属。
func (r *Repo) RevokeGeneratedFileForModeration(ctx context.Context, fileID string) error {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil
	}
	err := r.db.WithContext(ctx).Model(&models.FileObject{}).
		Where("file_id = ? AND status = ?", fileID, "active").
		Updates(map[string]any{
			"status":  "moderation_blocked",
			"user_id": 0,
		}).Error
	if dberror.IsRecordNotFound(err) {
		return nil
	}
	return dberror.Translate(err)
}

// DeleteGeneratedFileArtifactsForModeration 将附件标记为已删除，并返回用于物理删除的
// 存储路径。此处不清空 storage_path——调用方仅在对象存储删除
// 成功后才清空，以便失败的删除可以重试。
func (r *Repo) DeleteGeneratedFileArtifactsForModeration(ctx context.Context, fileID string) error {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil
	}
	var file models.FileObject
	err := r.db.WithContext(ctx).
		Where("file_id = ?", fileID).
		First(&file).Error
	if err != nil {
		if dberror.IsRecordNotFound(err) || err == gorm.ErrRecordNotFound {
			return nil
		}
		return dberror.Translate(err)
	}
	// 软删除仍引用该文件的附件。
	if err := r.db.WithContext(ctx).Model(&models.Attachment{}).
		Where("file_id = ? AND status <> ?", fileID, "deleted").
		Update("status", "deleted").Error; err != nil {
		return dberror.Translate(err)
	}
	// 保持 blocked 状态；保留 storage_path 以便可重试的物理清理。
	if err := r.db.WithContext(ctx).Model(&models.FileObject{}).
		Where("id = ?", file.ID).
		Updates(map[string]any{
			"status": "moderation_blocked",
		}).Error; err != nil {
		return dberror.Translate(err)
	}
	return nil
}

// ClearGeneratedFileStoragePath 仅在物理删除成功后清空存储路径。
func (r *Repo) ClearGeneratedFileStoragePath(ctx context.Context, fileID string) error {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil
	}
	return dberror.Translate(r.db.WithContext(ctx).Model(&models.FileObject{}).
		Where("file_id = ?", fileID).
		Update("storage_path", "").Error)
}
