// Package embedding 封装 OpenAI 兼容 embedding API 的 HTTP 客户端能力。
// application 层不直接依赖本包，而是通过 ports/embedding 契约调用。
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
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/outboundhttp"
	portembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

// ---------------------------------------------------------------------------
// 私有 JSON 协议类型（仅 infra 层使用）
// ---------------------------------------------------------------------------

type requestPayload struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
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
	httpClients *outboundhttp.Pool
}

// New 创建带出站安全策略的 Client。
func New(outboundPolicy security.OutboundPolicy) *Client {
	return &Client{
		httpClients: outboundhttp.NewPool(outboundPolicy, outboundhttp.DefaultCacheLimit, func(policy security.OutboundPolicy, trustedOrigin string, variant string) (outboundhttp.ManagedClient, error) {
			return newEmbeddingHTTPClient(policy, outboundPolicy, trustedOrigin, variant)
		}),
	}
}

func newEmbeddingHTTPClient(policy security.OutboundPolicy, redirectPolicy security.OutboundPolicy, trustedOrigin string, _ string) (outboundhttp.ManagedClient, error) {
	transport := security.NewOutboundHTTPTransport(policy, 10*time.Second)
	client := &http.Client{Transport: platformtracing.NewHTTPTransport(transport)}
	if trustedOrigin != "" {
		client.CheckRedirect = outboundhttp.NewRedirectPolicy(redirectPolicy, trustedOrigin, "embedding request")
	}
	return outboundhttp.ManagedClient{Client: client, CloseIdleConnections: transport.CloseIdleConnections}, nil
}

// CallAPI 向指定服务发起 embedding 请求，返回各文本对应的向量列表。
// Request.TimeoutSeconds ≤ 0 时默认 60 秒。
func (c *Client) CallAPI(ctx context.Context, input portembedding.Request) ([][]float32, error) {
	if len(input.Texts) == 0 {
		return nil, nil
	}

	body, err := json.Marshal(requestPayload{Model: input.Model, Input: input.Texts, Dimensions: input.Dimensions})
	if err != nil {
		return nil, fmt.Errorf("embedding: marshal request: %w", err)
	}

	if input.TimeoutSeconds <= 0 {
		input.TimeoutSeconds = 60
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(input.TimeoutSeconds)*time.Second)
	defer cancel()

	url := strings.TrimRight(input.APIBase, "/") + "/embeddings"
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedding: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(input.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+input.APIKey)
	}

	resp, err := c.httpClients.Do(req, input.APIBase, "")
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

	result := make([][]float32, len(input.Texts))
	seen := make([]bool, len(input.Texts))
	for _, item := range payload.Data {
		if item.Index < 0 || item.Index >= len(result) {
			return nil, fmt.Errorf("embedding: response index %d out of range", item.Index)
		}
		if seen[item.Index] {
			return nil, fmt.Errorf("embedding: duplicate response index %d", item.Index)
		}
		if len(item.Embedding) == 0 {
			return nil, fmt.Errorf("embedding: response vector %d is empty", item.Index)
		}
		if input.Dimensions > 0 && len(item.Embedding) != input.Dimensions {
			return nil, fmt.Errorf("embedding: response vector %d has %d dimensions, expected %d", item.Index, len(item.Embedding), input.Dimensions)
		}
		result[item.Index] = item.Embedding
		seen[item.Index] = true
	}
	for index, present := range seen {
		if !present {
			return nil, fmt.Errorf("embedding: response vector %d is missing", index)
		}
	}
	return result, nil
}

// CloseIdleConnections 释放所有 Embedding origin 客户端的空闲连接。
func (c *Client) CloseIdleConnections() {
	if c != nil && c.httpClients != nil {
		c.httpClients.CloseIdleConnections()
	}
}
