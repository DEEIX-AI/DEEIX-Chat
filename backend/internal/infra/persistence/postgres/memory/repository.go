package memory

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/sqlitevec"
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
	if dberror.IsRecordNotFound(err) {
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
		return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			existing.Value = item.Value
			existing.Scope = item.Scope
			existing.UpdatedBy = item.UpdatedBy
			if err := tx.Save(&existing).Error; err != nil {
				return translateError(err)
			}
			return r.clearUserMemoryEmbedding(ctx, tx, existing.ID)
		})
	}
	if dberror.IsRecordNotFound(err) {
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

func (r *Repo) clearUserMemoryEmbedding(ctx context.Context, tx *gorm.DB, memoryID uint) error {
	if memoryID == 0 {
		return nil
	}
	if r.sqliteDialect() {
		return translateError(tx.Exec(
			fmt.Sprintf(`DELETE FROM %s WHERE memory_id = ?`, sqlitevec.UserMemoryVectorTable),
			memoryID,
		).Error)
	}
	if !r.postgresUserMemoryEmbeddingColumnAvailable(ctx, tx) {
		return nil
	}
	return translateError(tx.Exec(`UPDATE "user_memories" SET embedding = NULL WHERE id = ?`, memoryID).Error)
}

func (r *Repo) postgresUserMemoryEmbeddingColumnAvailable(ctx context.Context, tx *gorm.DB) bool {
	if tx == nil || tx.Dialector == nil || tx.Dialector.Name() != "postgres" {
		return false
	}
	available := false
	err := tx.WithContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema()
				AND table_name = 'user_memories'
				AND column_name = 'embedding'
				AND udt_name = 'vector'
		)`,
	).Scan(&available).Error
	return err == nil && available
}

// DeleteUserMemory 删除用户长期记忆（按 key 匹配，物理删除）。
func (r *Repo) DeleteUserMemory(ctx context.Context, userID uint, memoryKey string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if r.sqliteDialect() {
			if err := tx.Exec(
				fmt.Sprintf(`DELETE FROM %s WHERE memory_id IN (
					SELECT id FROM user_memories WHERE user_id = ? AND memory_key = ?
				)`, sqlitevec.UserMemoryVectorTable),
				userID,
				memoryKey,
			).Error; err != nil {
				return translateError(err)
			}
		}
		return translateError(tx.
			Where("user_id = ? AND memory_key = ?", userID, memoryKey).
			Delete(&model.UserMemory{}).Error)
	})
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

// SearchUserMemoriesByEmbedding 按查询向量语义检索最相关的用户记忆。
func (r *Repo) SearchUserMemoriesByEmbedding(ctx context.Context, userID uint, queryEmbedding []float32, topK int, minSimilarity float64) ([]domainmemory.UserMemory, error) {
	if len(queryEmbedding) == 0 || topK <= 0 {
		return nil, nil
	}
	if r.sqliteDialect() {
		return r.searchSQLiteUserMemoriesByEmbedding(ctx, userID, queryEmbedding, topK, minSimilarity)
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

func (r *Repo) searchSQLiteUserMemoriesByEmbedding(ctx context.Context, userID uint, queryEmbedding []float32, topK int, minSimilarity float64) ([]domainmemory.UserMemory, error) {
	vector, err := sqlitevec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT memories.id, memories.user_id, memories.memory_key, memories.value, memories.scope, memories.updated_by,
		       (1.0 - vectors.distance) AS similarity
		FROM %s AS vectors
		JOIN user_memories AS memories
			ON memories.id = vectors.memory_id
		WHERE vectors.embedding MATCH ?
			AND vectors.k = ?
			AND vectors.user_id = ?
		ORDER BY vectors.distance ASC`, sqlitevec.UserMemoryVectorTable)
	var rows []userMemorySearchRow
	if err := r.db.WithContext(ctx).Raw(query, vector, topK, userID).Scan(&rows).Error; err != nil {
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
		return r.upsertSQLiteUserMemoryEmbedding(ctx, userID, memoryKey, expectedValue, embedding)
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

func (r *Repo) upsertSQLiteUserMemoryEmbedding(ctx context.Context, userID uint, memoryKey string, expectedValue string, embedding []float32) error {
	var item model.UserMemory
	query := r.db.WithContext(ctx).Where("user_id = ? AND memory_key = ?", userID, memoryKey)
	if strings.TrimSpace(expectedValue) != "" {
		query = query.Where("value = ?", strings.TrimSpace(expectedValue))
	}
	if err := query.First(&item).Error; err != nil {
		if dberror.IsRecordNotFound(err) {
			return nil
		}
		return translateError(err)
	}
	vector, err := sqlitevec.SerializeFloat32(embedding)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(fmt.Sprintf(`DELETE FROM %s WHERE memory_id = ?`, sqlitevec.UserMemoryVectorTable), item.ID).Error; err != nil {
			return translateError(err)
		}
		return translateError(tx.Exec(
			fmt.Sprintf(`INSERT INTO %s (memory_id, user_id, embedding) VALUES (?, ?, ?)`, sqlitevec.UserMemoryVectorTable),
			item.ID,
			item.UserID,
			vector,
		).Error)
	})
}
