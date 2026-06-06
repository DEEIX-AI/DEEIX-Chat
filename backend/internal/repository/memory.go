package repository

import (
	"context"

	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
)

// MemoryRepository 定义记忆流程依赖的持久化能力。
type MemoryRepository interface {
	UpsertUserMemory(ctx context.Context, item *domainmemory.UserMemory) error
	DeleteUserMemory(ctx context.Context, userID uint, memoryKey string) error
	ListUserMemories(ctx context.Context, userID uint) ([]domainmemory.UserMemory, error)
	// SearchUserMemoriesByEmbedding 按查询向量语义检索最相关的用户记忆。
	// 需要向量存储支持且记忆已生成 embedding，否则返回空列表（非 error）。
	SearchUserMemoriesByEmbedding(ctx context.Context, userID uint, queryEmbedding []float32, topK int, minSimilarity float64) ([]domainmemory.UserMemory, error)
	// UpsertUserMemoryEmbedding 更新指定记忆条目的向量（异步写入，失败静默）。
	UpsertUserMemoryEmbedding(ctx context.Context, userID uint, memoryKey string, expectedValue string, embedding []float32) error
}
