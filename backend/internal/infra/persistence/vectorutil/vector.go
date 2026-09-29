// Package vectorutil 定义 PostgreSQL 与 SQLite 持久化实现
// 共享的物理向量表示。
package vectorutil

import (
	"fmt"
	"strconv"
	"strings"
)

// MaxDimensions 是持久化层支持的最大 embedding 宽度。
// PostgreSQL 保留模型原生宽度，仅在索引/检索表达式中填充；
// SQLite 需要定宽的 vec0 表，在写入时填充。
const (
	MaxDimensions = 4096
	// IndexDimensions 满足 pgvector halfvec HNSW 的上限，同时保留最大宽度向量
	// 除最后 96 个分量外的全部分量，用于候选召回。
	IndexDimensions = 4000
)

// CandidateLimit 返回有界的 ANN 候选集，用于全向量精确重排。
func CandidateLimit(topK int) int {
	const (
		minimum    = 100
		maximum    = 1000
		multiplier = 10
	)
	limit := topK * multiplier
	if limit < minimum {
		return minimum
	}
	if limit > maximum {
		if topK > maximum {
			return topK
		}
		return maximum
	}
	return limit
}

// AlignForStorage 返回 SQLite 所需的定宽表示。
func AlignForStorage(input []float32) ([]float32, error) {
	if len(input) == 0 || len(input) == MaxDimensions {
		return input, nil
	}
	if len(input) > MaxDimensions {
		return nil, fmt.Errorf("embedding dimensions %d exceed supported maximum %d", len(input), MaxDimensions)
	}
	result := make([]float32, MaxDimensions)
	copy(result, input)
	return result, nil
}

// PostgresLiteral 将原生宽度向量序列化以供 PostgreSQL 存储。
func PostgresLiteral(input []float32) (string, error) {
	if len(input) > MaxDimensions {
		return "", fmt.Errorf("embedding dimensions %d exceed supported maximum %d", len(input), MaxDimensions)
	}
	return postgresLiteral(input), nil
}

// PostgresPaddedLiteral 序列化最大宽度的查询向量。PostgreSQL
// 检索表达式会在精确重排前将存储的原生宽度向量填充至相同宽度，
// 从而在不扩展每一行的情况下保持余弦相似度。
func PostgresPaddedLiteral(input []float32) (string, error) {
	aligned, err := AlignForStorage(input)
	if err != nil {
		return "", err
	}
	return postgresLiteral(aligned), nil
}

func postgresLiteral(input []float32) string {
	if len(input) == 0 {
		return "[]"
	}
	var builder strings.Builder
	builder.Grow(len(input) * 4)
	builder.WriteByte('[')
	for index, value := range input {
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.FormatFloat(float64(value), 'f', -1, 32))
	}
	builder.WriteByte(']')
	return builder.String()
}
