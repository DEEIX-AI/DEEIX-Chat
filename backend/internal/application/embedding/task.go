package embedding

import (
	"context"
	"strings"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/imageutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"go.uber.org/zap"
)

// 管理员查看向量化任务时的分组。
const (
	// TaskBucketReady 是已完成向量化、仅供查看的文件。
	TaskBucketReady = "ready"
	// TaskBucketPending 是尚未索引的可向量化文件与排队、执行中的任务。
	TaskBucketPending = "pending"
	// TaskBucketFailed 是处理或向量化失败的文件。
	TaskBucketFailed = "failed"
	// TaskBucketStale 是向量空间变更后需要重建的文件。
	TaskBucketStale = "stale"
	// TaskBucketEmpty 是提取完成但没有文本的文件。
	TaskBucketEmpty = "empty"
	// TaskBucketUnsupported 是当前配置下类型无法向量化的文件。
	TaskBucketUnsupported = "unsupported"
)

// embeddableFileCategories 是按正文向量化的文件类别。上传时 file_category 由与 supportsEmbeddingSource
// 相同的 MIME 与扩展名规则推断，因此可以在查询中代替逐个文件判断。
var embeddableFileCategories = [...]string{"pdf", "word", "presentation", "excel", "text"}

// FileTask 是管理员任务列表中的一个文件及其当前可重试性。
type FileTask struct {
	File domainconversation.FileObject
	// Stalled 表示排队或执行超过停滞阈值，重试会重新投递任务。
	Stalled bool
	// Reprocess 表示重试会先重新提取文件文本（处理失败、无文本或处理排队停滞）。
	Reprocess bool
	// RetryBlockedReason 非空时说明该文件当前不能重试，取值与 TargetedFileSkip.Reason 一致。
	RetryBlockedReason string
}

// ListFileTasksInput 描述管理员任务列表的筛选与分页条件。
type ListFileTasksInput struct {
	Bucket   string
	Query    string
	Page     int
	PageSize int
}

// FileTaskPage 是管理员任务列表的一页结果。
type FileTaskPage struct {
	Items []FileTask
	Total int64
}

// IsTaskBucket 判断任务分组是否受支持。
func IsTaskBucket(bucket string) bool {
	_, ok := taskBucketMatches(bucket)
	return ok
}

// ListFileTasks 分页列出全平台指定分组的向量化任务，并给出每个文件当前能否重试。
func (s *Service) ListFileTasks(ctx context.Context, input ListFileTasksInput) (FileTaskPage, error) {
	matches, ok := taskBucketMatches(input.Bucket)
	if !ok {
		return FileTaskPage{}, ErrInvalidTaskBucket
	}
	page := FileTaskPage{Items: []FileTask{}}
	if s.repo == nil {
		return page, nil
	}
	cfg := s.snapshot()
	offset, limit := pagination.Offset(input.Page, input.PageSize)
	files, total, err := s.repo.ListFileEmbeddingTasks(ctx, repository.ListFileEmbeddingTasksInput{
		Matches: matches,
		Scope:   embeddableFileScope(cfg),
		Query:   strings.TrimSpace(input.Query),
		Offset:  offset,
		Limit:   limit,
	})
	if err != nil {
		return page, err
	}
	signature := configuredModelSignature(cfg)
	stalledBefore := queueStalledBefore(time.Now())
	page.Total = total
	page.Items = make([]FileTask, 0, len(files))
	for i := range files {
		blockedReason, reprocess := fileVectorizationPlan(cfg, files[i], signature, stalledBefore, true)
		page.Items = append(page.Items, FileTask{
			File:               files[i],
			Stalled:            fileEmbeddingStalled(files[i], stalledBefore) || fileProcessingStalled(files[i], stalledBefore),
			Reprocess:          reprocess,
			RetryBlockedReason: blockedReason,
		})
	}
	return page, nil
}

func taskBucketMatches(bucket string) ([]repository.FileEmbedStatusMatch, bool) {
	yes, no := true, false
	switch bucket {
	case TaskBucketReady:
		return []repository.FileEmbedStatusMatch{{Status: "ready"}}, true
	case TaskBucketPending:
		return []repository.FileEmbedStatusMatch{
			{Status: "none", Embeddable: &yes, ProcessingFailed: &no},
			{Status: domainconversation.FileSubprocessStatusQueued},
			{Status: domainconversation.FileSubprocessStatusProcessing},
		}, true
	case TaskBucketFailed:
		// 与 GetIndexStatus 口径一致：处理流水线失败、尚未进入索引的可向量化文件也算失败。
		return []repository.FileEmbedStatusMatch{
			{Status: "failed"},
			{Status: "none", Embeddable: &yes, ProcessingFailed: &yes},
		}, true
	case TaskBucketStale:
		return []repository.FileEmbedStatusMatch{{Status: "stale"}}, true
	case TaskBucketEmpty:
		return []repository.FileEmbedStatusMatch{{Status: domainconversation.FileSubprocessStatusEmpty}}, true
	case TaskBucketUnsupported:
		return []repository.FileEmbedStatusMatch{{Status: "none", Embeddable: &no}}, true
	default:
		return nil, false
	}
}

// embeddableFileScope 返回当前配置下可向量化的文件范围，与 canEmbedFile 的判断保持一致：
// 图片在启用 OCR 时全部可用，否则只有协议支持图片输入且格式受支持时可用。
func embeddableFileScope(cfg config.Config) repository.EmbeddableFileScope {
	scope := repository.EmbeddableFileScope{
		Categories: append(make([]string, 0, len(embeddableFileCategories)+1), embeddableFileCategories[:]...),
	}
	switch {
	case cfg.ExtractImageOCREnabled:
		scope.Categories = append(scope.Categories, "image")
	case protocolSupportsImage(cfg):
		scope.Categories = append(scope.Categories, "image")
		scope.ImageMIMETypes = imageutil.SupportedMimeTypes()
	}
	return scope
}

// queueStalledBefore 返回排队任务的停滞判定时刻：早于该时刻仍未完成的 queued/processing 任务视为停滞。
func queueStalledBefore(now time.Time) time.Time {
	return now.Add(-queueStallTimeout)
}

// fileEmbeddingStalled 判断排队或执行中的任务是否已停滞；缺少更新时间时保守地视为仍在进行。
func fileEmbeddingStalled(fileObj domainconversation.FileObject, stalledBefore time.Time) bool {
	switch strings.ToLower(strings.TrimSpace(fileObj.EmbedStatus)) {
	case domainconversation.FileSubprocessStatusQueued, domainconversation.FileSubprocessStatusProcessing:
		return !fileObj.UpdatedAt.IsZero() && fileObj.UpdatedAt.Before(stalledBefore)
	default:
		return false
	}
}

// fileProcessingStalled 判断文件是否排队等待处理超过停滞阈值仍未开始，通常是处理队列消息丢失。
func fileProcessingStalled(fileObj domainconversation.FileObject, stalledBefore time.Time) bool {
	switch strings.ToLower(strings.TrimSpace(fileObj.ProcessingStatus)) {
	case "uploaded", domainconversation.FileProcessingStatusQueued:
		return !fileObj.UpdatedAt.IsZero() && fileObj.UpdatedAt.Before(stalledBefore)
	default:
		return false
	}
}

// sweepStalledTasks 把超过执行上限仍停在 processing 的任务标记为失败，使其进入可重试状态。
func (s *Service) sweepStalledTasks(ctx context.Context) {
	if s.repo == nil {
		return
	}
	count, err := s.repo.MarkStalledFileEmbeddingsFailed(ctx, time.Now().Add(-processingStallTimeout), embeddingStalledMessage)
	if err != nil {
		if ctx.Err() == nil && s.logger != nil {
			s.logger.Warn("embedding_stall_sweep_failed", zap.Error(err))
		}
		return
	}
	if count > 0 && s.logger != nil {
		s.logger.Info("embedding_stalled_tasks_failed", zap.Int64("count", count))
	}
}
