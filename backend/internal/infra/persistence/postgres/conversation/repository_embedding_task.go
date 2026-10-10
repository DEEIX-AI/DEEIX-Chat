package conversation

import (
	"context"
	"strings"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"gorm.io/gorm"
)

const fileImageCategory = "image"

// embeddingTaskColumns 是任务列表需要的字段，避免整行读取提取预览等大字段。
var embeddingTaskColumns = []string{
	"id", "file_id", "user_id", "file_name", "mime_type", "detected_mime", "file_category", "size_bytes",
	"storage_path", "status", "processing_status", "processing_ready", "processing_error_code",
	"processing_error_message", "extract_status", "embed_status", "embed_signature", "embed_error",
	"chunk_count", "created_at", "updated_at",
}

// processedWithTextCondition 匹配处理流水线已成功结束且有可用文本或原图的文件，向量化服务可以直接处理它们。
const processedWithTextCondition = "processing_status = ? AND extract_status <> ?"

func processedWithTextArgs() []any {
	return []any{"ready", domainconversation.FileSubprocessStatusEmpty}
}

// MarkStalledFileEmbeddingsFailed 将全平台长时间停留在 processing 的文件标记为失败。
// 每次向量化执行都有超时上限，超过阈值仍未更新说明执行进程已退出；
// 同时把仍停在 embedding 阶段的处理状态收敛为就绪，避免文件一直显示处理中。
func (r *Repo) MarkStalledFileEmbeddingsFailed(ctx context.Context, cutoff time.Time, message string) (int64, error) {
	message = strings.TrimSpace(message)
	if cutoff.IsZero() || message == "" {
		return 0, repository.ErrInvalidInput
	}
	message = truncateText(message, 255)
	result := r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("status = ? AND embed_status = ? AND updated_at < ?", "active", "processing", cutoff).
		Updates(map[string]any{
			"embed_status":             "failed",
			"embed_error":              message,
			"processing_status":        gorm.Expr("CASE WHEN processing_status = ? THEN ? ELSE processing_status END", "embedding", "ready"),
			"processing_ready":         gorm.Expr("CASE WHEN processing_status = ? THEN ? ELSE processing_ready END", "embedding", true),
			"processing_error_code":    gorm.Expr("CASE WHEN processing_status = ? THEN ? ELSE processing_error_code END", "embedding", "embed_failed"),
			"processing_error_message": gorm.Expr("CASE WHEN processing_status = ? THEN ? ELSE processing_error_message END", "embedding", message),
		})
	return result.RowsAffected, dberror.Translate(result.Error)
}

// CountFileEmbeddingStates 用一次分组查询统计全平台文件的向量化状态分布。
func (r *Repo) CountFileEmbeddingStates(
	ctx context.Context,
	scope repository.EmbeddableFileScope,
	stalledBefore time.Time,
) ([]repository.FileEmbeddingStateCount, error) {
	embeddable, embeddableArgs := embeddableFileCondition(scope)
	args := make([]any, 0, len(embeddableArgs)+4)
	args = append(args, embeddableArgs...)
	args = append(args, "failed", []string{"queued", "processing"}, stalledBefore)
	var rows []struct {
		EmbedStatus      string
		Embeddable       int
		ProcessingFailed int
		Stalled          int
		Total            int64
	}
	err := r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Select(
			"embed_status, "+
				"CASE WHEN "+embeddable+" THEN 1 ELSE 0 END AS embeddable, "+
				"CASE WHEN processing_status = ? THEN 1 ELSE 0 END AS processing_failed, "+
				"CASE WHEN embed_status IN ? AND updated_at < ? THEN 1 ELSE 0 END AS stalled, "+
				"COUNT(*) AS total",
			args...,
		).
		Where("status = ?", "active").
		// 按位置分组：两种方言都支持，且不依赖输出别名的解析规则。
		Group("1, 2, 3, 4").
		Scan(&rows).Error
	if err != nil {
		return nil, dberror.Translate(err)
	}
	counts := make([]repository.FileEmbeddingStateCount, 0, len(rows))
	for _, row := range rows {
		counts = append(counts, repository.FileEmbeddingStateCount{
			Status:           row.EmbedStatus,
			Embeddable:       row.Embeddable == 1,
			ProcessingFailed: row.ProcessingFailed == 1,
			Stalled:          row.Stalled == 1,
			Count:            row.Total,
		})
	}
	return counts, nil
}

// ListFileEmbeddingTasks 分页返回全平台匹配的文件，按最近更新时间倒序。
func (r *Repo) ListFileEmbeddingTasks(
	ctx context.Context,
	input repository.ListFileEmbeddingTasksInput,
) ([]domainconversation.FileObject, int64, error) {
	if len(input.Matches) == 0 {
		return nil, 0, repository.ErrInvalidInput
	}
	embeddable, embeddableArgs := embeddableFileCondition(input.Scope)
	parts := make([]string, 0, len(input.Matches))
	args := make([]any, 0, len(input.Matches)*(2+len(embeddableArgs)))
	for _, match := range input.Matches {
		status := strings.TrimSpace(match.Status)
		if status == "" {
			return nil, 0, repository.ErrInvalidInput
		}
		clauses := []string{"embed_status = ?"}
		args = append(args, status)
		if match.Embeddable != nil {
			if *match.Embeddable {
				clauses = append(clauses, embeddable)
			} else {
				clauses = append(clauses, "NOT "+embeddable)
			}
			args = append(args, embeddableArgs...)
		}
		if match.ProcessingFailed != nil {
			if *match.ProcessingFailed {
				clauses = append(clauses, "processing_status = ?")
			} else {
				clauses = append(clauses, "processing_status <> ?")
			}
			args = append(args, "failed")
		}
		parts = append(parts, "("+strings.Join(clauses, " AND ")+")")
	}
	query := r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("status = ?", "active").
		Where("("+strings.Join(parts, " OR ")+")", args...)
	if search := strings.ToLower(strings.TrimSpace(input.Query)); search != "" {
		pattern := "%" + search + "%"
		query = query.Where("(LOWER(file_id) LIKE ? OR LOWER(file_name) LIKE ?)", pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	limit := input.Limit
	if limit <= 0 {
		limit = pagination.DefaultPageSize
	}
	// 仅在 pagination.MaxPageSize 处做防御性截断，与上层分页校验保持一致。
	if limit > pagination.MaxPageSize {
		limit = pagination.MaxPageSize
	}
	var entities []models.FileObject
	if err := query.
		Select(embeddingTaskColumns).
		Order("updated_at DESC").
		Order("id DESC").
		Offset(input.Offset).
		Limit(limit).
		Find(&entities).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	return toFileObjectDomains(entities), total, nil
}

// GetActiveFileObjectsByFileIDs 跨用户按文件 ID 批量读取活跃文件。
func (r *Repo) GetActiveFileObjectsByFileIDs(ctx context.Context, fileIDs []string) ([]domainconversation.FileObject, error) {
	if len(fileIDs) == 0 {
		return []domainconversation.FileObject{}, nil
	}
	var entities []models.FileObject
	if err := r.db.WithContext(ctx).
		Where("status = ? AND file_id IN ?", "active", fileIDs).
		Find(&entities).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return toFileObjectDomains(entities), nil
}

// ListFilesForReindex 分页返回需要重建向量的文件：none、stale、failed，以及停滞的 queued 任务。
// 只包含处理流水线已成功结束的文件；处理失败、无文本或仍在处理的文件由处理流水线负责，
// 交给向量化服务只会重复提取。
func (r *Repo) ListFilesForReindex(ctx context.Context, input repository.ListFilesForReindexInput) ([]domainconversation.FileObject, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	statusCondition := "embed_status IN ?"
	args := []any{[]string{"none", "stale", "failed"}}
	if !input.StalledBefore.IsZero() {
		// queued 只在队列消息丢失时才会停滞；即使原消息仍在队列中，重复执行也会被文件行锁串行化且结果幂等。
		statusCondition = "(embed_status IN ? OR (embed_status = ? AND updated_at < ?))"
		args = append(args, domainconversation.FileSubprocessStatusQueued, input.StalledBefore)
	}
	var entities []models.FileObject
	err := r.db.WithContext(ctx).
		Where("id > ? AND status = ?", input.AfterID, "active").
		Where(processedWithTextCondition, processedWithTextArgs()...).
		Where(statusCondition, args...).
		Order("id ASC").
		Limit(limit).
		Find(&entities).Error
	if err != nil {
		return nil, dberror.Translate(err)
	}
	return toFileObjectDomains(entities), nil
}

// embeddableFileCondition 把可向量化范围翻译为 SQL 条件。
// 没有文件实体的记录无法读取内容，不论类别都视为不可向量化。
func embeddableFileCondition(scope repository.EmbeddableFileScope) (string, []any) {
	categories := make([]string, 0, len(scope.Categories))
	includeImages := false
	for _, category := range scope.Categories {
		normalized := strings.ToLower(strings.TrimSpace(category))
		switch normalized {
		case "":
		case fileImageCategory:
			includeImages = true
		default:
			categories = append(categories, normalized)
		}
	}
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 3)
	if len(categories) > 0 {
		clauses = append(clauses, "file_category IN ?")
		args = append(args, categories)
	}
	if includeImages {
		imageMIMETypes := make([]string, 0, len(scope.ImageMIMETypes))
		for _, mimeType := range scope.ImageMIMETypes {
			if normalized := strings.ToLower(strings.TrimSpace(mimeType)); normalized != "" {
				imageMIMETypes = append(imageMIMETypes, normalized)
			}
		}
		if len(imageMIMETypes) > 0 {
			// 与应用层一致：优先使用后端探测的 MIME，缺失时回退到客户端声明的 MIME。
			clauses = append(clauses, "(file_category = ? AND LOWER(COALESCE(NULLIF(TRIM(detected_mime), ''), TRIM(mime_type))) IN ?)")
			args = append(args, fileImageCategory, imageMIMETypes)
		} else {
			clauses = append(clauses, "file_category = ?")
			args = append(args, fileImageCategory)
		}
	}
	if len(clauses) == 0 {
		return "1 = 0", nil
	}
	return "(storage_path <> '' AND (" + strings.Join(clauses, " OR ") + "))", args
}
