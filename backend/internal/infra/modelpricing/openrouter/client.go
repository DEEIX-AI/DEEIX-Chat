package openrouter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

const (
	modelsURL            = "https://openrouter.ai/api/v1/models"
	maxModelsResponse    = 8 << 20
	modelsRequestTimeout = 15 * time.Second
)

// Client 提供 OpenRouter 公共模型目录读取能力。
type Client struct {
	httpClient *http.Client
}

// New 创建使用严格出站策略的 OpenRouter 模型目录客户端。
func New(outboundPolicy security.OutboundPolicy) *Client {
	httpClient := security.NewOutboundHTTPClient(outboundPolicy, modelsRequestTimeout)
	httpClient.Transport = platformtracing.NewHTTPTransport(httpClient.Transport)
	return &Client{httpClient: httpClient}
}

// FetchModels 获取 OpenRouter 官方模型目录原始 JSON，协议映射由调用侧边界完成。
func (c *Client) FetchModels(ctx context.Context) ([]byte, error) {
	if c == nil || c.httpClient == nil {
		return nil, fmt.Errorf("openrouter pricing client is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("openrouter models request failed: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxModelsResponse+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxModelsResponse {
		return nil, fmt.Errorf("openrouter models response exceeds %d bytes", maxModelsResponse)
	}
	return body, nil
}
