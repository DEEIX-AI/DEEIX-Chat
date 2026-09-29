// Package embeddingutil 定义 embedding 生产方与消费方共享的稳定标识。
package embeddingutil

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// ModelSignature 通过规范化的模型名与输出维度标识一个 embedding 向量空间。
func ModelSignature(model string, outputDimensions int) string {
	raw := strings.TrimSpace(model) + "@" + strconv.Itoa(outputDimensions)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:4]) + "@" + strconv.Itoa(outputDimensions)
}

// SpaceSignature 标识包含提供方端点在内的 embedding 向量空间。
// 端点会被规范化，使末尾斜杠不会产生不同的空间；
// 同时切换提供方时，也不会误用其他服务在相同模型名下生成的向量。
func SpaceSignature(model string, outputDimensions int, endpoint string) string {
	normalizedEndpoint := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	raw := strings.TrimSpace(model) + "@" + strconv.Itoa(outputDimensions) + "@" + normalizedEndpoint
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:8]) + "@" + strconv.Itoa(outputDimensions)
}
