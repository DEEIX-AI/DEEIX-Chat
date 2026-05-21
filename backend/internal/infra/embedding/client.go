// Package embedding 封装 OpenAI 兼容 embedding API 的 HTTP 客户端能力。
// application 层不直接依赖本包，而是通过 repository.EmbeddingClient 接口调用。
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

// ---------------------------------------------------------------------------
// 私有 JSON 协议类型（仅 infra 层使用）
// ---------------------------------------------------------------------------

type requestPayload struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type responsePayload struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// ---------------------------------------------------------------------------
// 客户端
// ---------------------------------------------------------------------------

// Client 封装 OpenAI 兼容 embedding API 的 HTTP 调用能力。
type Client struct {
	env string
}

// New 创建 Client。
func New() *Client {
	return NewWithEnv("")
}

// NewWithEnv 创建带运行环境的 Client。
func NewWithEnv(env string) *Client {
	return &Client{env: strings.TrimSpace(env)}
}

// CallAPI 向指定 apiBase 发起 embedding 请求，返回各文本对应的向量列表。
// timeoutSeconds ≤ 0 时默认 60 秒。
func (c *Client) CallAPI(
	ctx context.Context,
	apiBase, apiKey, model string,
	texts []string,
	timeoutSeconds int,
) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	body, err := json.Marshal(requestPayload{Model: model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("embedding: marshal request: %w", err)
	}

	url := strings.TrimRight(apiBase, "/") + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedding: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	transport := security.NewOutboundHTTPTransport(c.env, 10*time.Second)
	httpClient := &http.Client{
		Timeout:   time.Duration(timeoutSeconds) * time.Second,
		Transport: platformtracing.NewHTTPTransport(transport),
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("embedding: API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var payload responsePayload
	if err = json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("embedding: decode response: %w", err)
	}

	result := make([][]float32, len(texts))
	for _, item := range payload.Data {
		if item.Index < len(result) {
			result[item.Index] = item.Embedding
		}
	}
	return result, nil
}

// ChunkText 将文本按估算 token 数分片，使用段落优先截断策略。
// chunkSize 和 overlap 的单位为 token，按 2 bytes/token 估算。
func ChunkText(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 512
	}
	if overlap < 0 {
		overlap = 64
	}
	// 估算：CJK 约 1.5 chars/token，ASCII 约 4 chars/token，取折中 2 chars/token。
	// 这里按 rune 切分，不能使用字符串字节下标，否则中文文本会出现 slice 越界。
	chunkRunes := chunkSize * 2
	overlapRunes := overlap * 2
	if overlapRunes >= chunkRunes {
		overlapRunes = chunkRunes / 4
	}
	paragraphBreak := []rune("\n\n")
	lineBreak := []rune("\n")

	runes := []rune(text)
	if len(runes) <= chunkRunes {
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []string{text}
	}

	var chunks []string
	start := 0
	for start < len(runes) {
		end := start + chunkRunes
		if end > len(runes) {
			end = len(runes)
		}
		slice := string(runes[start:end])
		if end < len(runes) {
			window := runes[start:end]
			if idx := lastRuneSequenceIndex(window, paragraphBreak); idx > chunkRunes/2 {
				end = start + idx + 2
				slice = string(runes[start:end])
			} else if idx := lastRuneSequenceIndex(window, lineBreak); idx > chunkRunes/2 {
				end = start + idx + 1
				slice = string(runes[start:end])
			}
		}
		if strings.TrimSpace(slice) != "" {
			chunks = append(chunks, slice)
		}
		if end >= len(runes) {
			break
		}
		next := end - overlapRunes
		if next <= start {
			next = start + 1
		}
		start = next
	}
	return chunks
}

func lastRuneSequenceIndex(haystack []rune, needle []rune) int {
	if len(needle) == 0 || len(haystack) < len(needle) {
		return -1
	}
	for i := len(haystack) - len(needle); i >= 0; i-- {
		matched := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}
