package conversation

import (
	"context"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func (r *Repo) UpdateFileObjectProcessingState(ctx context.Context, item *domainconversation.FileObjectProcessing) error {
	if item == nil {
		return nil
	}
	result := r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("id = ? AND user_id = ?", item.FileObjectID, item.UserID).
		Updates(fileObjectProcessingStateUpdates(item))
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *Repo) UpdateClaimedFileObjectProcessingState(
	ctx context.Context,
	item *domainconversation.FileObjectProcessing,
	attemptID string,
) (bool, error) {
	if item == nil || attemptID == "" {
		return false, nil
	}
	updates := fileObjectProcessingStateUpdates(item)
	if item.ProcessingStatus == "ready" || item.ProcessingStatus == "failed" {
		updates["processing_attempt_id"] = ""
	}
	result := r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("id = ? AND user_id = ? AND processing_attempt_id = ?", item.FileObjectID, item.UserID, attemptID).
		Updates(updates)
	if result.Error != nil {
		return false, translateError(result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (r *Repo) GetFileObjectProcessingByObjectID(ctx context.Context, fileObjID uint) (*domainconversation.FileObjectProcessing, error) {
	var item models.FileObject
	if err := r.db.WithContext(ctx).
		Where("id = ?", fileObjID).
		First(&item).Error; err != nil {
		return nil, err
	}
	result := toFileObjectProcessingStateDomain(item)
	return &result, nil
}

func (r *Repo) CloneFileObjectProcessingState(ctx context.Context, sourceFileObjID uint, targetFileObjID uint, userID uint) error {
	if sourceFileObjID == 0 || targetFileObjID == 0 {
		return nil
	}
	source, err := r.GetFileObjectProcessingByObjectID(ctx, sourceFileObjID)
	if err != nil {
		return nil
	}
	now := time.Now()
	copyItem := *source
	copyItem.ID = 0
	copyItem.FileObjectID = targetFileObjID
	copyItem.UserID = userID
	copyItem.CreatedAt = now
	copyItem.UpdatedAt = now
	return r.UpdateFileObjectProcessingState(ctx, &copyItem)
}

func (r *Repo) TryClaimFileObjectProcessing(
	ctx context.Context,
	userID uint,
	fileID string,
	allowRecovery bool,
	extractorVersion string,
	attemptID string,
) (bool, error) {
	if attemptID == "" {
		return false, nil
	}
	claimableStatuses := []string{"queued"}
	if allowRecovery {
		claimableStatuses = append(claimableStatuses, "extracting", "embedding")
	}
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("user_id = ? AND file_id = ? AND processing_status IN ?", userID, fileID, claimableStatuses).
		Updates(map[string]interface{}{
			"processing_status":        "extracting",
			"processing_ready":         false,
			"processing_error_code":    "",
			"processing_error_message": "",
			"extract_status":           "processing",
			"extractor_version":        extractorVersion,
			"processing_attempt_id":    attemptID,
			"processing_started_at":    now,
			"processing_completed_at":  nil,
			"updated_at":               now,
		})
	if result.Error != nil {
		return false, translateError(result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (r *Repo) ResetFileObjectProcessingForRetry(
	ctx context.Context,
	userID uint,
	fileID string,
	attemptID string,
) (bool, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where(
			"user_id = ? AND file_id = ? AND processing_attempt_id = ? AND processing_status IN ?",
			userID,
			fileID,
			attemptID,
			[]string{"extracting", "embedding"},
		).
		Updates(map[string]interface{}{
			"processing_status":       "queued",
			"processing_ready":        false,
			"extract_status":          "none",
			"processing_attempt_id":   "",
			"processing_completed_at": nil,
			"updated_at":              now,
		})
	if result.Error != nil {
		return false, translateError(result.Error)
	}
	return result.RowsAffected > 0, nil
}
