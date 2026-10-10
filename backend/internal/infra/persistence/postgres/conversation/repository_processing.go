package conversation

import (
	"context"
	"strings"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
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
		return dberror.Translate(result.Error)
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
		return false, dberror.Translate(result.Error)
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
		Updates(map[string]any{
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
		return false, dberror.Translate(result.Error)
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
		Updates(map[string]any{
			"processing_status":       "queued",
			"processing_ready":        false,
			"extract_status":          "none",
			"processing_attempt_id":   "",
			"processing_completed_at": nil,
			"updated_at":              now,
		})
	if result.Error != nil {
		return false, dberror.Translate(result.Error)
	}
	return result.RowsAffected > 0, nil
}

// reprocessableFileCondition 匹配没有可用文本、可以重新走处理流水线的文件：处理失败；提取完成但无文本
// （includeEmpty 时）；排队早于停滞时刻仍未开始（队列消息丢失）。正在提取的文件不在其中，重置不会打断执行。
func reprocessableFileCondition(includeEmpty bool, stalledBefore time.Time) (string, []any) {
	clauses := []string{"processing_status = ?"}
	args := []any{"failed"}
	if includeEmpty {
		clauses = append(clauses, "(processing_status = ? AND extract_status = ?)")
		args = append(args, "ready", domainconversation.FileSubprocessStatusEmpty)
	}
	if !stalledBefore.IsZero() {
		clauses = append(clauses, "(processing_status IN ? AND updated_at < ?)")
		args = append(args, []string{"uploaded", domainconversation.FileProcessingStatusQueued}, stalledBefore)
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

// ResetFileForReprocessing 把没有可用文本的文件原子重置为待处理，并登记处理完成后的向量化请求。
// 条件写在同一条 UPDATE 中：并发提交只有一次成功，正在提取的文件不会被打断。
func (r *Repo) ResetFileForReprocessing(
	ctx context.Context,
	userID uint,
	fileID string,
	embeddingSignature string,
	stalledBefore time.Time,
) (bool, error) {
	fileID = strings.TrimSpace(fileID)
	embeddingSignature = strings.TrimSpace(embeddingSignature)
	if fileID == "" || embeddingSignature == "" {
		return false, repository.ErrInvalidInput
	}
	condition, args := reprocessableFileCondition(true, stalledBefore)
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("user_id = ? AND file_id = ? AND status = ? AND storage_path <> ''", userID, fileID, "active").
		Where(condition, args...).
		Updates(map[string]any{
			"processing_status":        domainconversation.FileProcessingStatusQueued,
			"processing_ready":         false,
			"processing_error_code":    "",
			"processing_error_message": "",
			"processing_attempt_id":    "",
			"processing_started_at":    now,
			"processing_completed_at":  nil,
			"extract_status":           "none",
			"rag_ready":                false,
			"rag_reason":               "",
			// queued + 签名即向量化请求：流水线提取完成后据此为该向量空间建立索引，不受上传自动向量化开关影响。
			"embed_status":    domainconversation.FileSubprocessStatusQueued,
			"embed_signature": embeddingSignature,
			"embed_error":     "",
			"updated_at":      now,
		})
	if result.Error != nil {
		return false, dberror.Translate(result.Error)
	}
	return result.RowsAffected > 0, nil
}

// ListFilesForReprocessing 按 ID 升序分页返回全平台需要重新走处理流水线的文件。
func (r *Repo) ListFilesForReprocessing(
	ctx context.Context,
	input repository.ListFilesForReprocessingInput,
) ([]domainconversation.FileObject, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	condition, args := reprocessableFileCondition(input.IncludeEmpty, input.StalledBefore)
	var entities []models.FileObject
	if err := r.db.WithContext(ctx).
		Where("id > ? AND status = ? AND storage_path <> ''", input.AfterID, "active").
		Where(condition, args...).
		Order("id ASC").
		Limit(limit).
		Find(&entities).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return toFileObjectDomains(entities), nil
}
