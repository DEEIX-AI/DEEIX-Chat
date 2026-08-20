// Package vectorutil defines the physical vector representation shared by
// PostgreSQL and SQLite persistence implementations.
package vectorutil

import (
	"fmt"
	"strconv"
	"strings"
)

// MaxDimensions is the largest embedding width supported by persistence.
// PostgreSQL keeps the model's native width and pads only inside index/search
// expressions. SQLite requires a fixed-width vec0 table and pads on write.
const (
	MaxDimensions = 4096
	// IndexDimensions fits pgvector's halfvec HNSW limit while retaining all but
	// the final 96 components of a maximum-width vector for candidate recall.
	IndexDimensions = 4000
)

// CandidateLimit returns a bounded ANN candidate set for exact full-vector reranking.
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

// AlignForStorage returns the fixed-width representation required by SQLite.
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

// PostgresLiteral serializes a native-width vector for PostgreSQL storage.
func PostgresLiteral(input []float32) (string, error) {
	if len(input) > MaxDimensions {
		return "", fmt.Errorf("embedding dimensions %d exceed supported maximum %d", len(input), MaxDimensions)
	}
	return postgresLiteral(input), nil
}

// PostgresPaddedLiteral serializes a maximum-width query vector. PostgreSQL
// search expressions pad stored native-width vectors to the same width before
// exact reranking, preserving cosine similarity without expanding every row.
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
