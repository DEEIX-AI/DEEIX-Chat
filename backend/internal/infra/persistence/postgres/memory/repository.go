package memory

import (
	"context"
	"errors"
	"strconv"
	"strings"

	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// float32SliceToVec 将 []float32 转为 pgvector 文本格式 "[1.0,2.0,...]"。
func float32SliceToVec(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.FormatFloat(float64(f), 'f', -1, 32))
	}
	sb.WriteByte(']')
	return sb.String()
}

// translateError 将 gorm 底层错误统一映射为仓储语义错误。
func translateError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repository.ErrNotFound
	}
	return err
}

// Repo 聚合记忆域数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) sqliteDialect() bool {
	return r != nil && r.db != nil && r.db.Dialector != nil && r.db.Dialector.Name() == "sqlite"
}

// UpsertUserMemory 更新或插入用户长期记忆。
func (r *Repo) UpsertUserMemory(ctx context.Context, item *domainmemory.UserMemory) error {
	if item == nil {
		return nil
	}
	var existing model.UserMemory
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND memory_key = ?", item.UserID, item.MemoryKey).
		First(&existing).Error
	if err == nil {
		existing.Value = item.Value
		existing.Scope = item.Scope
		existing.UpdatedBy = item.UpdatedBy
		return translateError(r.db.WithContext(ctx).Save(&existing).Error)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record := model.UserMemory{
			UserID:    item.UserID,
			MemoryKey: item.MemoryKey,
			Value:     item.Value,
			Scope:     item.Scope,
			UpdatedBy: item.UpdatedBy,
		}
		return translateError(r.db.WithContext(ctx).Create(&record).Error)
	}
	return translateError(err)
}

// DeleteUserMemory 删除用户长期记忆（按 key 匹配，物理删除）。
func (r *Repo) DeleteUserMemory(ctx context.Context, userID uint, memoryKey string) error {
	return translateError(r.db.WithContext(ctx).
		Where("user_id = ? AND memory_key = ?", userID, memoryKey).
		Delete(&model.UserMemory{}).Error)
}

// ListUserMemories 查询用户长期记忆。
func (r *Repo) ListUserMemories(ctx context.Context, userID uint) ([]domainmemory.UserMemory, error) {
	items := make([]model.UserMemory, 0)
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("updated_at DESC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainmemory.UserMemory, 0, len(items))
	for _, item := range items {
		results = append(results, domainmemory.UserMemory{
			ID:        item.ID,
			UserID:    item.UserID,
			MemoryKey: item.MemoryKey,
			Value:     item.Value,
			Scope:     item.Scope,
			UpdatedBy: item.UpdatedBy,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}
	return results, nil
}

type userMemorySearchRow struct {
	ID         uint
	UserID     uint
	MemoryKey  string
	Value      string
	Scope      string
	UpdatedBy  string
	Similarity float64
}

// SearchUserMemoriesByEmbedding 按查询向量语义检索最相关的用户记忆（需 pgvector 支持）。
func (r *Repo) SearchUserMemoriesByEmbedding(ctx context.Context, userID uint, queryEmbedding []float32, topK int, minSimilarity float64) ([]domainmemory.UserMemory, error) {
	if len(queryEmbedding) == 0 || topK <= 0 {
		return nil, nil
	}
	if r.sqliteDialect() {
		return nil, nil
	}
	vec := float32SliceToVec(queryEmbedding)
	query := `
		SELECT id, user_id, memory_key, value, scope, updated_by,
		       (1 - (embedding <=> ?::vector)) AS similarity
		FROM user_memories
		WHERE user_id = ? AND embedding IS NOT NULL
		ORDER BY similarity DESC
		LIMIT ?`
	var rows []userMemorySearchRow
	if err := r.db.WithContext(ctx).Raw(query, vec, userID, topK).Scan(&rows).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainmemory.UserMemory, 0, len(rows))
	for _, row := range rows {
		if row.Similarity < minSimilarity {
			continue
		}
		results = append(results, domainmemory.UserMemory{
			ID:        row.ID,
			UserID:    row.UserID,
			MemoryKey: row.MemoryKey,
			Value:     row.Value,
			Scope:     row.Scope,
			UpdatedBy: row.UpdatedBy,
		})
	}
	return results, nil
}

// UpsertUserMemoryEmbedding 更新指定记忆条目的向量（异步写入，失败静默）。
func (r *Repo) UpsertUserMemoryEmbedding(ctx context.Context, userID uint, memoryKey string, expectedValue string, embedding []float32) error {
	if len(embedding) == 0 {
		return nil
	}
	if r.sqliteDialect() {
		return nil
	}
	vec := float32SliceToVec(embedding)
	query := `UPDATE "user_memories" SET embedding = ?::vector WHERE user_id = ? AND memory_key = ?`
	args := []interface{}{vec, userID, memoryKey}
	if strings.TrimSpace(expectedValue) != "" {
		query += ` AND value = ?`
		args = append(args, strings.TrimSpace(expectedValue))
	}
	return r.db.WithContext(ctx).Exec(
		query,
		args...,
	).Error
}
