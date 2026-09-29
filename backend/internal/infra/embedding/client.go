// Package embedding 封装 embedding 服务的 HTTP 客户端，按配置协议编码请求（见 protocol.go）。
// application 层不直接依赖本包，而是通过 ports/embedding 契约调用。
package embedding

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/outboundhttp"
	portembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

// maxResponseBytes 限制单次响应读取上限；4096 维 × 100 条 float32 文本约 8MB，留足余量。
const maxResponseBytes = 64 << 20

// Client 封装 embedding API 的 HTTP 调用能力。
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

// CallAPI 向指定服务发起 embedding 请求，返回与 Inputs 一一对应的向量列表。
// Request.TimeoutSeconds ≤ 0 时默认 60 秒。
func (c *Client) CallAPI(ctx context.Context, input portembedding.Request) ([][]float32, error) {
	if len(input.Inputs) == 0 {
		return nil, nil
	}
	adapter, err := resolveProtocol(input.Protocol)
	if err != nil {
		return nil, err
	}
	endpoint, body, headers, err := adapter.encode(input)
	if err != nil {
		return nil, err
	}

	if input.TimeoutSeconds <= 0 {
		input.TimeoutSeconds = 60
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(input.TimeoutSeconds)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedding: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
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
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("embedding: read response: %w", err)
	}
	result, err := adapter.decode(respBody, len(input.Inputs))
	if err != nil {
		return nil, err
	}
	for index, vector := range result {
		if len(vector) == 0 {
			return nil, fmt.Errorf("embedding: response vector %d is missing", index)
		}
		if input.Dimensions > 0 && len(vector) != input.Dimensions {
			return nil, fmt.Errorf("embedding: response vector %d has %d dimensions, expected %d", index, len(vector), input.Dimensions)
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
