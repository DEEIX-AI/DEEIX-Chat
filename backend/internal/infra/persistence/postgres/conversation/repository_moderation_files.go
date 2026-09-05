package conversation

import (
	"context"
	"strings"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/gorm"
)

// GetFileObjectByFileIDAnyStatus loads a file row regardless of ownership/status.
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

// ListModerationBlockedFileIDsForCleanup returns revoked files whose physical objects
// still need deletion. storage_path is cleared only after object-store deletion succeeds.
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

// RevokeGeneratedFileForModeration marks a generated file inaccessible and unlinks user ownership.
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

// DeleteGeneratedFileArtifactsForModeration marks attachments deleted and returns storage path
// for physical deletion. storage_path is NOT cleared here — callers clear it only after a
// successful object-store delete so failed deletes remain retryable.
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
	// Soft-delete attachments that still reference this file.
	if err := r.db.WithContext(ctx).Model(&models.Attachment{}).
		Where("file_id = ? AND status <> ?", fileID, "deleted").
		Update("status", "deleted").Error; err != nil {
		return dberror.Translate(err)
	}
	// Keep status blocked; leave storage_path intact for retryable physical cleanup.
	if err := r.db.WithContext(ctx).Model(&models.FileObject{}).
		Where("id = ?", file.ID).
		Updates(map[string]any{
			"status": "moderation_blocked",
		}).Error; err != nil {
		return dberror.Translate(err)
	}
	return nil
}

// ClearGeneratedFileStoragePath clears the storage path only after physical delete succeeds.
func (r *Repo) ClearGeneratedFileStoragePath(ctx context.Context, fileID string) error {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil
	}
	return dberror.Translate(r.db.WithContext(ctx).Model(&models.FileObject{}).
		Where("file_id = ?", fileID).
		Update("storage_path", "").Error)
}
