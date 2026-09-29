package vectorutil

import (
	"fmt"

	"gorm.io/gorm"
)

// PostgresPaddedExpression 返回可信的 SQL 表达式，将向量填充
// 至 MaxDimensions 而不改写存储值。调用方只能传入
// 内部列表达式，绝不能传入用户可控的输入。
func PostgresPaddedExpression(expression string) string {
	return fmt.Sprintf(
		`(CASE WHEN vector_dims(%[1]s) = %[2]d THEN %[1]s::vector(%[2]d) ELSE ((%[1]s)::real[] || array_fill(0::real, ARRAY[%[2]d - vector_dims(%[1]s)]))::vector(%[2]d) END)`,
		expression,
		MaxDimensions,
	)
}

// PostgresIndexExpression 返回 HNSW 索引定义与候选查询
// 共用的定宽 halfvec 表达式。
func PostgresIndexExpression(expression string) string {
	return fmt.Sprintf(
		`subvector(%s, 1, %d)::halfvec(%d)`,
		PostgresPaddedExpression(expression),
		IndexDimensions,
		IndexDimensions,
	)
}

// ConfigurePostgresCandidateSearch 为当前事务启用过滤式 HNSW 扫描。
func ConfigurePostgresCandidateSearch(tx *gorm.DB) error {
	if err := tx.Exec(`SET LOCAL hnsw.iterative_scan = strict_order`).Error; err != nil {
		return err
	}
	return tx.Exec(`SET LOCAL hnsw.ef_search = 100`).Error
}
