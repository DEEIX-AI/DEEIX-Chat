// Package embeddingutil defines stable identifiers shared by embedding producers and consumers.
package embeddingutil

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// ModelSignature identifies one embedding vector space by normalized model name and output dimensions.
func ModelSignature(model string, outputDimensions int) string {
	raw := strings.TrimSpace(model) + "@" + strconv.Itoa(outputDimensions)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:4]) + "@" + strconv.Itoa(outputDimensions)
}

// SpaceSignature identifies an embedding vector space including the provider
// endpoint. The endpoint is normalized so a trailing slash does not create a
// different space, while switching providers cannot accidentally reuse vectors
// produced by a different service under the same model name.
func SpaceSignature(model string, outputDimensions int, endpoint string) string {
	normalizedEndpoint := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	raw := strings.TrimSpace(model) + "@" + strconv.Itoa(outputDimensions) + "@" + normalizedEndpoint
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:8]) + "@" + strconv.Itoa(outputDimensions)
}
