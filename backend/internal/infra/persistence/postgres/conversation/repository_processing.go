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

func (r *Repo) UpdateFileObjectProcessing(
	ctx context.Context,
	userID uint,
	fileID string,
	input repository.UpdateFileObjectProcessingInput,
) error {
	updates := fileObjectProcessingUpdates(input)
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now()
	result := r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("user_id = ? AND file_id = ?", userID, fileID).
		Updates(updates)
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func fileObjectProcessingUpdates(input repository.UpdateFileObjectProcessingInput) map[string]interface{} {
	updates := make(map[string]interface{})
	if input.ProcessingStatus != nil {
		updates["processing_status"] = *input.ProcessingStatus
	}
	if input.ProcessingReady != nil {
		updates["processing_ready"] = *input.ProcessingReady
	}
	if input.ProcessingErrorCode != nil {
		updates["processing_error_code"] = *input.ProcessingErrorCode
	}
	if input.ProcessingErrorMessage != nil {
		updates["processing_error_message"] = *input.ProcessingErrorMessage
	}
	if input.ExtractStatus != nil {
		updates["extract_status"] = *input.ExtractStatus
	}
	if input.PageCount != nil {
		updates["page_count"] = *input.PageCount
	}
	if input.ExtractorVersion != nil {
		updates["extractor_version"] = *input.ExtractorVersion
	}
	if input.ExtractedAt != nil {
		updates["extracted_at"] = *input.ExtractedAt
	}
	return updates
}
