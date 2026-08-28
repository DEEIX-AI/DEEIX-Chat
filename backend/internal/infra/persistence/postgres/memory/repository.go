package memory

import (
	"context"
	"fmt"
	"strings"

	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/sqlitevec"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/vectorutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// maxUserMemoriesPerUser 单用户长期记忆条目上限；列表接口为全量加载，需要写入侧兜底有界。
const maxUserMemoriesPerUser = 500

// float32SliceToVec 按模型原始维度序列化 PostgreSQL 向量。
func float32SliceToVec(v []float32) (string, error) {
	return vectorutil.PostgresLiteral(v)
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
		// 新增前检查单用户记忆条数上限。
		var count int64
		if err := r.db.WithContext(ctx).
			Model(&model.UserMemory{}).
			Where("user_id = ?", item.UserID).
			Count(&count).Error; err != nil {
			return translateError(err)
		}
		if count >= maxUserMemoriesPerUser {
			return repository.ErrUserMemoryLimitExceeded
		}
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
		if err := tx.Exec(
			fmt.Sprintf(`DELETE FROM %s WHERE memory_id = ?`, sqlitevec.UserMemoryVectorTable),
			memoryID,
		).Error; err != nil {
			return translateError(err)
		}
		return translateError(tx.Model(&model.UserMemory{}).Where("id = ?", memoryID).Update("embedding_signature", "").Error)
	}
	if !r.postgresUserMemoryEmbeddingColumnAvailable(ctx, tx) {
		return nil
	}
	return translateError(tx.Exec(`UPDATE "user_memories" SET embedding = NULL, embedding_signature = '' WHERE id = ?`, memoryID).Error)
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
func (r *Repo) SearchUserMemoriesByEmbedding(ctx context.Context, userID uint, queryEmbedding []float32, embeddingSignature string, topK int, minSimilarity float64) ([]domainmemory.UserMemory, error) {
	if len(queryEmbedding) == 0 || strings.TrimSpace(embeddingSignature) == "" || topK <= 0 {
		return nil, nil
	}
	if r.sqliteDialect() {
		return r.searchSQLiteUserMemoriesByEmbedding(ctx, userID, queryEmbedding, embeddingSignature, topK, minSimilarity)
	}
	vec, err := vectorutil.PostgresPaddedLiteral(queryEmbedding)
	if err != nil {
		return nil, err
	}
	candidateLimit := vectorutil.CandidateLimit(topK)
	indexExpression := vectorutil.PostgresIndexExpression("embedding")
	exactExpression := vectorutil.PostgresPaddedExpression("memories.embedding")
	query := fmt.Sprintf(`
		WITH vector_candidates AS MATERIALIZED (
			SELECT id
			FROM user_memories
			WHERE user_id = ? AND embedding_signature = ? AND embedding IS NOT NULL
			ORDER BY %s
				<=> subvector(?::vector, 1, %d)::halfvec(%d)
			LIMIT ?
		)
		SELECT memories.id, memories.user_id, memories.memory_key, memories.value, memories.scope, memories.updated_by,
		       (1 - (%s <=> ?::vector(%d))) AS similarity
		FROM user_memories AS memories
		JOIN vector_candidates AS candidates ON candidates.id = memories.id
		ORDER BY similarity DESC
		LIMIT ?`,
		indexExpression,
		vectorutil.IndexDimensions,
		vectorutil.IndexDimensions,
		exactExpression,
		vectorutil.MaxDimensions,
	)
	var rows []userMemorySearchRow
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := vectorutil.ConfigurePostgresCandidateSearch(tx); err != nil {
			return err
		}
		return tx.Raw(query, userID, embeddingSignature, vec, candidateLimit, vec, topK).Scan(&rows).Error
	}); err != nil {
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

func (r *Repo) searchSQLiteUserMemoriesByEmbedding(ctx context.Context, userID uint, queryEmbedding []float32, embeddingSignature string, topK int, minSimilarity float64) ([]domainmemory.UserMemory, error) {
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
			AND vectors.embedding_signature = ?
			AND memories.embedding_signature = ?
		ORDER BY vectors.distance ASC`, sqlitevec.UserMemoryVectorTable)
	var rows []userMemorySearchRow
	if err := r.db.WithContext(ctx).Raw(query, vector, topK, userID, embeddingSignature, embeddingSignature).Scan(&rows).Error; err != nil {
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
func (r *Repo) UpsertUserMemoryEmbedding(ctx context.Context, userID uint, memoryKey string, expectedValue string, embedding []float32, embeddingSignature string) error {
	if len(embedding) == 0 || strings.TrimSpace(embeddingSignature) == "" {
		return nil
	}
	if r.sqliteDialect() {
		return r.upsertSQLiteUserMemoryEmbedding(ctx, userID, memoryKey, expectedValue, embedding, embeddingSignature)
	}
	vec, err := float32SliceToVec(embedding)
	if err != nil {
		return err
	}
	query := `UPDATE "user_memories" SET embedding = ?::vector, embedding_signature = ? WHERE user_id = ? AND memory_key = ?`
	args := []interface{}{vec, embeddingSignature, userID, memoryKey}
	if strings.TrimSpace(expectedValue) != "" {
		query += ` AND value = ?`
		args = append(args, strings.TrimSpace(expectedValue))
	}
	return r.db.WithContext(ctx).Exec(
		query,
		args...,
	).Error
}

func (r *Repo) upsertSQLiteUserMemoryEmbedding(ctx context.Context, userID uint, memoryKey string, expectedValue string, embedding []float32, embeddingSignature string) error {
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
		update := tx.Model(&model.UserMemory{}).Where("id = ?", item.ID)
		if strings.TrimSpace(expectedValue) != "" {
			update = update.Where("value = ?", strings.TrimSpace(expectedValue))
		}
		result := update.Update("embedding_signature", embeddingSignature)
		if result.Error != nil {
			return translateError(result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Exec(fmt.Sprintf(`DELETE FROM %s WHERE memory_id = ?`, sqlitevec.UserMemoryVectorTable), item.ID).Error; err != nil {
			return translateError(err)
		}
		return translateError(tx.Exec(
			fmt.Sprintf(`INSERT INTO %s (memory_id, user_id, embedding_signature, embedding) VALUES (?, ?, ?, ?)`, sqlitevec.UserMemoryVectorTable),
			item.ID,
			item.UserID,
			embeddingSignature,
			vector,
		).Error)
	})
}
