package vectorutil

import (
	"fmt"

	"gorm.io/gorm"
)

// PostgresPaddedExpression returns a trusted SQL expression that pads a vector
// to MaxDimensions without rewriting the stored value. Callers must only pass
// internal column expressions, never user-controlled input.
func PostgresPaddedExpression(expression string) string {
	return fmt.Sprintf(
		`(CASE WHEN vector_dims(%[1]s) = %[2]d THEN %[1]s::vector(%[2]d) ELSE ((%[1]s)::real[] || array_fill(0::real, ARRAY[%[2]d - vector_dims(%[1]s)]))::vector(%[2]d) END)`,
		expression,
		MaxDimensions,
	)
}

// PostgresIndexExpression returns the fixed-width halfvec expression used by
// both the HNSW index definition and candidate queries.
func PostgresIndexExpression(expression string) string {
	return fmt.Sprintf(
		`subvector(%s, 1, %d)::halfvec(%d)`,
		PostgresPaddedExpression(expression),
		IndexDimensions,
		IndexDimensions,
	)
}

// ConfigurePostgresCandidateSearch enables filtered HNSW scans for the current transaction.
func ConfigurePostgresCandidateSearch(tx *gorm.DB) error {
	if err := tx.Exec(`SET LOCAL hnsw.iterative_scan = strict_order`).Error; err != nil {
		return err
	}
	return tx.Exec(`SET LOCAL hnsw.ef_search = 100`).Error
}
