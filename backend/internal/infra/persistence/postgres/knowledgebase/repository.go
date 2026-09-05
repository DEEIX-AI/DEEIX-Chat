package knowledgebase

import (
	"context"
	"strings"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repo 封装知识库数据访问。
type Repo struct {
	db *gorm.DB
}

const knowledgeBaseFileSelectColumns = `
	fo.id, fo.file_id, fo.user_id, fo.purpose, fo.file_name, fo.mime_type, fo.detected_mime,
	fo.file_category, fo.size_bytes, fo.sha256, fo.storage_path, fo.status, fo.processing_status,
	fo.processing_ready, fo.extract_status, fo.embed_status, fo.embed_signature, fo.embed_error, fo.rag_opt_out,
	fo.chunk_count, fo.page_count, fo.created_at, fo.updated_at`

// NewRepo 创建知识库仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// ListKnowledgeBases 分页查询知识库。
func (r *Repo) ListKnowledgeBases(ctx context.Context, filter repository.KnowledgeBaseListFilter, offset int, limit int) ([]domainknowledgebase.KnowledgeBase, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	// Do not use the request-derived limit as a slice capacity. The SQL query
	// still enforces the bounded limit, and Gorm grows this slice only as rows return.
	items := make([]model.KnowledgeBase, 0)
	query := applyListFilter(r.db.WithContext(ctx).Model(&model.KnowledgeBase{}), filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	if strings.TrimSpace(filter.Sort) == "files" {
		query = query.
			Select("knowledge_bases.*").
			Joins("LEFT JOIN knowledge_base_files AS sort_kbf ON sort_kbf.knowledge_base_id = knowledge_bases.id").
			Joins("LEFT JOIN file_objects AS sort_fo ON sort_fo.id = sort_kbf.file_object_id AND sort_fo.status = ? AND sort_fo.deleted_at IS NULL", "active").
			Group("knowledge_bases.id")
	}
	if err := query.Order(listOrder(filter)).Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	results := toDomains(items)
	if err := r.hydrateCounts(ctx, results); err != nil {
		return nil, 0, err
	}
	return results, total, nil
}

// GetKnowledgeBaseByPublicID 按公开 ID 查询知识库。
func (r *Repo) GetKnowledgeBaseByPublicID(ctx context.Context, publicID string) (*domainknowledgebase.KnowledgeBase, error) {
	result, err := r.GetKnowledgeBaseAccessByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	items := []domainknowledgebase.KnowledgeBase{*result}
	if err := r.hydrateCounts(ctx, items); err != nil {
		return nil, err
	}
	return &items[0], nil
}

// GetKnowledgeBaseAccessByPublicID 查询知识库访问控制所需的元数据，不聚合文件计数。
func (r *Repo) GetKnowledgeBaseAccessByPublicID(ctx context.Context, publicID string) (*domainknowledgebase.KnowledgeBase, error) {
	var item model.KnowledgeBase
	if err := r.db.WithContext(ctx).Where("public_id = ?", strings.TrimSpace(publicID)).First(&item).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	result := toDomain(item)
	return &result, nil
}

// CreateKnowledgeBase 创建知识库。
func (r *Repo) CreateKnowledgeBase(ctx context.Context, item *domainknowledgebase.KnowledgeBase) (*domainknowledgebase.KnowledgeBase, error) {
	if item == nil {
		return nil, repository.ErrInvalidInput
	}
	record := toModel(item)
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if record.SortOrder <= 0 {
			var maxSortOrder int
			if err := tx.Model(&model.KnowledgeBase{}).
				Where("scope = ? AND owner_user_id = ?", record.Scope, record.OwnerUserID).
				Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSortOrder).Error; err != nil {
				return err
			}
			record.SortOrder = maxSortOrder + 1
		}
		enabled := record.Enabled
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		// Enabled=false is a meaningful control-plane value. Gorm applies the
		// model's default:true to a zero-value bool during Create, so restore the
		// explicitly requested state in the same transaction.
		if !enabled {
			if err := tx.Model(&record).UpdateColumn("enabled", false).Error; err != nil {
				return err
			}
			record.Enabled = false
		}
		return nil
	}); err != nil {
		return nil, dberror.Translate(err)
	}
	result := toDomain(record)
	return &result, nil
}

// PatchKnowledgeBase 更新知识库。
func (r *Repo) PatchKnowledgeBase(ctx context.Context, id uint, patch repository.KnowledgeBasePatch) (*domainknowledgebase.KnowledgeBase, error) {
	if id == 0 {
		return nil, repository.ErrInvalidInput
	}
	var result domainknowledgebase.KnowledgeBase
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.KnowledgeBase
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&item).Error; err != nil {
			return err
		}
		updates := make(map[string]any)
		if patch.Name != nil {
			updates["name"] = strings.TrimSpace(*patch.Name)
		}
		if patch.Description != nil {
			updates["description"] = strings.TrimSpace(*patch.Description)
		}
		if patch.Enabled != nil {
			updates["enabled"] = *patch.Enabled
		}
		if patch.SortOrder != nil {
			updates["sort_order"] = *patch.SortOrder
		}
		if patch.UpdatedByUserIDSet {
			updates["updated_by_user_id"] = patch.UpdatedByUserID
		}
		if len(updates) > 0 {
			if err := tx.Model(&item).Updates(updates).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("id = ?", id).First(&item).Error; err != nil {
			return err
		}
		result = toDomain(item)
		return nil
	}); err != nil {
		return nil, dberror.Translate(err)
	}
	items := []domainknowledgebase.KnowledgeBase{result}
	if err := r.hydrateCounts(ctx, items); err != nil {
		return nil, err
	}
	return &items[0], nil
}

// DeleteKnowledgeBase 删除知识库及其关联，并返回可供后续安全清理的文件。
// 文件对象不在该事务中删除；调用方必须通过文件服务重新校验其引用关系。
func (r *Repo) DeleteKnowledgeBase(ctx context.Context, id uint) ([]repository.KnowledgeBaseFileCleanupCandidate, error) {
	if id == 0 {
		return nil, repository.ErrInvalidInput
	}
	candidates := make([]repository.KnowledgeBaseFileCleanupCandidate, 0)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.KnowledgeBase
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&item).Error; err != nil {
			return err
		}
		if err := tx.Table("knowledge_base_files AS kbf").
			Select("fo.user_id, fo.file_id").
			Joins("JOIN file_objects AS fo ON fo.id = kbf.file_object_id AND fo.status = ? AND fo.deleted_at IS NULL", "active").
			Where("kbf.knowledge_base_id = ?", id).
			Scan(&candidates).Error; err != nil {
			return err
		}
		if err := tx.Where("knowledge_base_id = ?", id).Delete(&model.ConversationProjectKnowledgeBase{}).Error; err != nil {
			return err
		}
		if err := tx.Where("knowledge_base_id = ?", id).Delete(&model.KnowledgeBaseFile{}).Error; err != nil {
			return err
		}
		result := tx.Delete(&item)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return repository.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return nil, dberror.Translate(err)
	}
	return candidates, nil
}

// ListKnowledgeBaseFiles 分页查询知识库文件。
func (r *Repo) ListKnowledgeBaseFiles(ctx context.Context, knowledgeBaseID uint, offset int, limit int) ([]domainconversation.FileObject, int64, error) {
	if knowledgeBaseID == 0 {
		return nil, 0, repository.ErrInvalidInput
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	base := r.db.WithContext(ctx).Table("knowledge_base_files AS kbf").
		Joins("JOIN file_objects AS fo ON fo.id = kbf.file_object_id AND fo.status = ? AND fo.deleted_at IS NULL", "active").
		Where("kbf.knowledge_base_id = ?", knowledgeBaseID)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	items := make([]model.FileObject, 0)
	if err := base.Select(knowledgeBaseFileSelectColumns).Order("kbf.sort_order ASC, kbf.created_at ASC").Offset(offset).Limit(limit).Scan(&items).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	return toFileDomains(items), total, nil
}

// GetKnowledgeBaseFileProcessingStatuses 批量查询知识库内文件的处理状态。
func (r *Repo) GetKnowledgeBaseFileProcessingStatuses(ctx context.Context, knowledgeBaseID uint, fileIDs []string) ([]domainconversation.FileObject, error) {
	if knowledgeBaseID == 0 || len(fileIDs) == 0 {
		return nil, repository.ErrInvalidInput
	}
	return r.listKnowledgeBaseFileProcessingStatuses(ctx, knowledgeBaseID, fileIDs)
}

func (r *Repo) listKnowledgeBaseFileProcessingStatuses(ctx context.Context, knowledgeBaseID uint, fileIDs []string) ([]domainconversation.FileObject, error) {
	items := make([]model.FileObject, 0)
	if len(fileIDs) > 0 {
		if err := r.db.WithContext(ctx).Table("knowledge_base_files AS kbf").
			Select(`
				fo.file_id, fo.file_name, fo.mime_type, fo.detected_mime, fo.file_category, fo.storage_path, fo.status,
				fo.processing_status, fo.processing_ready, fo.extract_status, fo.embed_status, fo.embed_signature,
				fo.embed_error, fo.rag_opt_out, fo.chunk_count, fo.updated_at`).
			Joins("JOIN file_objects AS fo ON fo.id = kbf.file_object_id AND fo.status = ? AND fo.deleted_at IS NULL", "active").
			Where("kbf.knowledge_base_id = ? AND fo.file_id IN ?", knowledgeBaseID, fileIDs).
			Scan(&items).Error; err != nil {
			return nil, dberror.Translate(err)
		}
	}
	return toFileDomains(items), nil
}

// GetKnowledgeBaseFileProcessingSnapshot 查询知识库文件处理状态及聚合计数。
func (r *Repo) GetKnowledgeBaseFileProcessingSnapshot(ctx context.Context, knowledgeBaseID uint, fileIDs []string) (*repository.KnowledgeBaseFileProcessingSnapshot, error) {
	if knowledgeBaseID == 0 {
		return nil, repository.ErrInvalidInput
	}
	items, err := r.listKnowledgeBaseFileProcessingStatuses(ctx, knowledgeBaseID, fileIDs)
	if err != nil {
		return nil, err
	}
	counts, err := r.loadKnowledgeBaseCounts(ctx, []uint{knowledgeBaseID})
	if err != nil {
		return nil, err
	}
	count := counts[knowledgeBaseID]
	return &repository.KnowledgeBaseFileProcessingSnapshot{
		Files:               items,
		FileCount:           count.FileCount,
		ReadyFileCount:      count.ReadyFileCount,
		ProcessingFileCount: count.ProcessingFileCount,
	}, nil
}

// ListKnowledgeBaseSourceFiles 分页查询指定所有者的有效知识库资料。
// ownerUserID=0 表示平台资料池；非零值表示用户个人文件。
func (r *Repo) ListKnowledgeBaseSourceFiles(
	ctx context.Context,
	ownerUserID uint,
	searchQuery string,
	offset int,
	limit int,
) ([]domainconversation.FileObject, int64, error) {
	return r.listKnowledgeBaseSourceFiles(ctx, ownerUserID, searchQuery, offset, limit, 0)
}

// ListAvailableKnowledgeBaseFiles 分页查询指定所有者尚未加入知识库的有效文件。
// ownerUserID=0 表示平台资料池；非零值表示用户个人文件。
func (r *Repo) ListAvailableKnowledgeBaseFiles(
	ctx context.Context,
	knowledgeBaseID uint,
	ownerUserID uint,
	searchQuery string,
	offset int,
	limit int,
) ([]domainconversation.FileObject, int64, error) {
	if knowledgeBaseID == 0 {
		return nil, 0, repository.ErrInvalidInput
	}
	return r.listKnowledgeBaseSourceFiles(ctx, ownerUserID, searchQuery, offset, limit, knowledgeBaseID)
}

func (r *Repo) listKnowledgeBaseSourceFiles(
	ctx context.Context,
	ownerUserID uint,
	searchQuery string,
	offset int,
	limit int,
	excludedKnowledgeBaseID uint,
) ([]domainconversation.FileObject, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	query := r.db.WithContext(ctx).Table("file_objects AS fo").
		Where("fo.user_id = ? AND fo.status = ? AND fo.deleted_at IS NULL", ownerUserID, "active")
	if excludedKnowledgeBaseID > 0 {
		query = query.Where("NOT EXISTS (SELECT 1 FROM knowledge_base_files AS kbf WHERE kbf.knowledge_base_id = ? AND kbf.file_object_id = fo.id)", excludedKnowledgeBaseID)
	}
	if normalizedQuery := strings.TrimSpace(searchQuery); normalizedQuery != "" {
		pattern := "%" + strings.ToLower(normalizedQuery) + "%"
		query = query.Where("LOWER(fo.file_name) LIKE ?", pattern)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	items := make([]model.FileObject, 0)
	if err := query.Select(knowledgeBaseFileSelectColumns).
		Order("fo.created_at DESC, fo.id DESC").
		Offset(offset).
		Limit(limit).
		Scan(&items).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	return toFileDomains(items), total, nil
}

// GetKnowledgeBaseFile 查询知识库中仍然有效的指定文件。
func (r *Repo) GetKnowledgeBaseFile(ctx context.Context, knowledgeBaseID uint, fileID string) (*domainconversation.FileObject, error) {
	fileID = strings.TrimSpace(fileID)
	if knowledgeBaseID == 0 || fileID == "" {
		return nil, repository.ErrInvalidInput
	}

	var item model.FileObject
	result := r.db.WithContext(ctx).Table("knowledge_base_files AS kbf").
		Select(knowledgeBaseFileSelectColumns).
		Joins("JOIN file_objects AS fo ON fo.id = kbf.file_object_id AND fo.status = ? AND fo.deleted_at IS NULL", "active").
		Where("kbf.knowledge_base_id = ? AND fo.file_id = ?", knowledgeBaseID, fileID).
		Limit(1).
		Scan(&item)
	if result.Error != nil {
		return nil, dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, repository.ErrNotFound
	}
	resultItem := toFileDomain(item)
	return &resultItem, nil
}

// AddKnowledgeBaseFiles 将当前作用域内的文件加入知识库。
func (r *Repo) AddKnowledgeBaseFiles(ctx context.Context, knowledgeBaseID uint, scope string, ownerUserID uint, actorUserID uint, fileIDs []string) error {
	if knowledgeBaseID == 0 || actorUserID == 0 || len(fileIDs) == 0 {
		return repository.ErrInvalidInput
	}
	return dberror.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var base model.KnowledgeBase
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", knowledgeBaseID).First(&base).Error; err != nil {
			return err
		}
		if base.Scope != scope || base.OwnerUserID != ownerUserID {
			return repository.ErrNotFound
		}
		files := make([]model.FileObject, 0, len(fileIDs))
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("file_id IN ? AND status = ?", fileIDs, "active")
		if scope == domainknowledgebase.ScopeUser {
			query = query.Where("user_id = ?", ownerUserID)
		} else {
			query = query.Where("user_id = ?", 0)
		}
		if err := query.Find(&files).Error; err != nil {
			return err
		}
		if len(files) != len(fileIDs) {
			return repository.ErrNotFound
		}
		filesByPublicID := make(map[string]model.FileObject, len(files))
		for _, file := range files {
			filesByPublicID[file.FileID] = file
		}
		var maxSortOrder int
		if err := tx.Model(&model.KnowledgeBaseFile{}).Where("knowledge_base_id = ?", knowledgeBaseID).
			Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSortOrder).Error; err != nil {
			return err
		}
		rows := make([]model.KnowledgeBaseFile, 0, len(fileIDs))
		for index, fileID := range fileIDs {
			file, exists := filesByPublicID[fileID]
			if !exists {
				return repository.ErrNotFound
			}
			rows = append(rows, model.KnowledgeBaseFile{
				KnowledgeBaseID: knowledgeBaseID,
				FileObjectID:    file.ID,
				SortOrder:       maxSortOrder + index + 1,
				AddedByUserID:   actorUserID,
			})
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return tx.Model(&base).Update("revision", gorm.Expr("revision + 1")).Error
	}))
}

// RemoveKnowledgeBaseFile 将文件移出知识库，不删除文件对象。
func (r *Repo) RemoveKnowledgeBaseFile(ctx context.Context, knowledgeBaseID uint, fileID string) error {
	return dberror.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where(
			"knowledge_base_id = ? AND file_object_id IN (?)",
			knowledgeBaseID,
			tx.Model(&model.FileObject{}).Select("id").Where("file_id = ? AND status = ?", strings.TrimSpace(fileID), "active"),
		).Delete(&model.KnowledgeBaseFile{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return repository.ErrNotFound
		}
		return tx.Model(&model.KnowledgeBase{}).Where("id = ?", knowledgeBaseID).
			Update("revision", gorm.Expr("revision + 1")).Error
	}))
}

// ResolveVisibleKnowledgeBaseFiles 解析当前用户可使用的知识库与文件。
func (r *Repo) ResolveVisibleKnowledgeBaseFiles(ctx context.Context, userID uint, publicIDs []string) ([]domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error) {
	if userID == 0 || len(publicIDs) == 0 {
		return nil, nil, repository.ErrInvalidInput
	}
	bases := make([]model.KnowledgeBase, 0, len(publicIDs))
	if err := r.db.WithContext(ctx).
		Where("public_id IN ? AND enabled = ?", publicIDs, true).
		Where("scope = ? OR (scope = ? AND owner_user_id = ?)", domainknowledgebase.ScopeBuiltin, domainknowledgebase.ScopeUser, userID).
		Find(&bases).Error; err != nil {
		return nil, nil, dberror.Translate(err)
	}
	if len(bases) != len(publicIDs) {
		return nil, nil, repository.ErrNotFound
	}
	domainBases := toDomains(bases)
	if err := r.hydrateCounts(ctx, domainBases); err != nil {
		return nil, nil, err
	}
	baseIDs := make([]uint, 0, len(bases))
	for _, base := range bases {
		baseIDs = append(baseIDs, base.ID)
	}
	files := make([]model.FileObject, 0)
	if err := r.db.WithContext(ctx).Table("file_objects AS fo").Distinct(knowledgeBaseFileSelectColumns).
		Joins("JOIN knowledge_base_files AS kbf ON kbf.file_object_id = fo.id").
		Where("kbf.knowledge_base_id IN ? AND fo.status = ? AND fo.deleted_at IS NULL", baseIDs, "active").
		Where("fo.processing_ready = ? AND fo.embed_status = ? AND fo.rag_opt_out = ? AND fo.chunk_count > 0", true, "ready", false).
		Find(&files).Error; err != nil {
		return nil, nil, dberror.Translate(err)
	}
	return domainBases, toFileDomains(files), nil
}

type knowledgeBaseCountRow struct {
	KnowledgeBaseID     uint
	FileCount           int64
	ReadyFileCount      int64
	ProcessingFileCount int64
}

func (r *Repo) hydrateCounts(ctx context.Context, items []domainknowledgebase.KnowledgeBase) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	counts, err := r.loadKnowledgeBaseCounts(ctx, ids)
	if err != nil {
		return err
	}
	for index := range items {
		items[index].FileCount = counts[items[index].ID].FileCount
		items[index].ReadyFileCount = counts[items[index].ID].ReadyFileCount
		items[index].ProcessingFileCount = counts[items[index].ID].ProcessingFileCount
	}
	return nil
}

func (r *Repo) loadKnowledgeBaseCounts(ctx context.Context, ids []uint) (map[uint]knowledgeBaseCountRow, error) {
	rows := make([]knowledgeBaseCountRow, 0, len(ids))
	if err := r.db.WithContext(ctx).Table("knowledge_base_files AS kbf").
		Select(
			"kbf.knowledge_base_id, COUNT(fo.id) AS file_count, SUM(CASE WHEN fo.processing_ready = ? AND fo.embed_status = ? AND fo.rag_opt_out = ? AND fo.chunk_count > 0 THEN 1 ELSE 0 END) AS ready_file_count, SUM(CASE WHEN fo.processing_status IN ? OR fo.extract_status = ? OR fo.embed_status = ? THEN 1 ELSE 0 END) AS processing_file_count",
			true,
			"ready",
			false,
			[]string{
				domainconversation.FileProcessingStatusUploaded,
				domainconversation.FileProcessingStatusQueued,
				domainconversation.FileProcessingStatusExtracting,
				domainconversation.FileProcessingStatusEmbedding,
			},
			domainconversation.FileSubprocessStatusProcessing,
			domainconversation.FileSubprocessStatusProcessing,
		).
		Joins("JOIN file_objects AS fo ON fo.id = kbf.file_object_id AND fo.status = ? AND fo.deleted_at IS NULL", "active").
		Where("kbf.knowledge_base_id IN ?", ids).
		Group("kbf.knowledge_base_id").Scan(&rows).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	counts := make(map[uint]knowledgeBaseCountRow, len(rows))
	for _, row := range rows {
		counts[row.KnowledgeBaseID] = row
	}
	return counts, nil
}

func applyListFilter(query *gorm.DB, filter repository.KnowledgeBaseListFilter) *gorm.DB {
	if len(filter.PublicIDs) > 0 {
		query = query.Where("public_id IN ?", filter.PublicIDs)
	}
	if filter.VisibleUserID != nil {
		query = query.Where("(scope = ? AND enabled = ?) OR (scope = ? AND owner_user_id = ? AND enabled = ?)",
			domainknowledgebase.ScopeBuiltin, true, domainknowledgebase.ScopeUser, *filter.VisibleUserID, true)
	} else {
		if scope := strings.TrimSpace(filter.Scope); scope != "" {
			query = query.Where("scope = ?", scope)
		}
		if filter.OwnerUserID != nil {
			query = query.Where("owner_user_id = ?", *filter.OwnerUserID)
		}
		if filter.Enabled != nil {
			query = query.Where("enabled = ?", *filter.Enabled)
		}
	}
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", like, like)
	}
	return query
}

func listOrder(filter repository.KnowledgeBaseListFilter) string {
	switch strings.TrimSpace(filter.Sort) {
	case "name":
		return "LOWER(name) ASC, id ASC"
	case "created":
		return "created_at DESC, id DESC"
	case "updated":
		return "updated_at DESC, id DESC"
	case "files":
		return "COUNT(sort_fo.id) DESC, knowledge_bases.id DESC"
	}
	if filter.VisibleUserID != nil {
		return "CASE WHEN scope = 'builtin' THEN 0 ELSE 1 END ASC, sort_order ASC, updated_at DESC, id DESC"
	}
	return "CASE WHEN enabled THEN 0 ELSE 1 END ASC, sort_order ASC, updated_at DESC, id DESC"
}

func toDomain(item model.KnowledgeBase) domainknowledgebase.KnowledgeBase {
	return domainknowledgebase.KnowledgeBase{
		ID: item.ID, PublicID: item.PublicID, Scope: item.Scope, OwnerUserID: item.OwnerUserID,
		Name: item.Name, Description: item.Description, Enabled: item.Enabled, SortOrder: item.SortOrder,
		Revision: item.Revision, CreatedByUserID: item.CreatedByUserID, UpdatedByUserID: item.UpdatedByUserID,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toDomains(items []model.KnowledgeBase) []domainknowledgebase.KnowledgeBase {
	results := make([]domainknowledgebase.KnowledgeBase, 0, len(items))
	for _, item := range items {
		results = append(results, toDomain(item))
	}
	return results
}

func toModel(item *domainknowledgebase.KnowledgeBase) model.KnowledgeBase {
	return model.KnowledgeBase{
		PublicID: item.PublicID, Scope: item.Scope, OwnerUserID: item.OwnerUserID, Name: item.Name,
		Description: item.Description, Enabled: item.Enabled, SortOrder: item.SortOrder, Revision: item.Revision,
		CreatedByUserID: item.CreatedByUserID, UpdatedByUserID: item.UpdatedByUserID,
	}
}

func toFileDomains(items []model.FileObject) []domainconversation.FileObject {
	results := make([]domainconversation.FileObject, 0, len(items))
	for _, item := range items {
		results = append(results, toFileDomain(item))
	}
	return results
}

func toFileDomain(item model.FileObject) domainconversation.FileObject {
	return domainconversation.FileObject{
		ID: item.ID, FileID: item.FileID, UserID: item.UserID, Purpose: item.Purpose, FileName: item.FileName,
		MimeType: item.MimeType, DetectedMIME: item.DetectedMIME, FileCategory: item.FileCategory,
		SizeBytes: item.SizeBytes, SHA256: item.SHA256, StoragePath: item.StoragePath, Status: item.Status,
		ProcessingStatus: item.ProcessingStatus, ProcessingReady: item.ProcessingReady,
		ProcessingErrorCode: item.ProcessingErrorCode, ProcessingErrorMessage: item.ProcessingErrorMessage,
		ExtractStatus: item.ExtractStatus, EmbedStatus: item.EmbedStatus, EmbedSignature: item.EmbedSignature,
		EmbedError: item.EmbedError, RAGOptOut: item.RAGOptOut,
		ChunkCount: item.ChunkCount, PageCount: item.PageCount, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}
