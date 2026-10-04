// Package modelsdev 读取 models.dev 社区模型目录，并提供随二进制发布的模型目录内置快照。
package modelsdev

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

const (
	// DefaultURL 是 models.dev 官方目录地址。
	DefaultURL = "https://models.dev/api.json"

	maxCatalogResponse    = 64 << 20
	catalogRequestTimeout = 30 * time.Second
)

// Client 按出站安全策略拉取 models.dev api.json，并映射为领域模型目录条目。
type Client struct {
	httpClient *http.Client
}

// New 创建使用指定出站策略的 models.dev 目录客户端。
func New(outboundPolicy security.OutboundPolicy) *Client {
	httpClient := security.NewOutboundHTTPClient(outboundPolicy, catalogRequestTimeout)
	httpClient.Transport = platformtracing.NewHTTPTransport(httpClient.Transport)
	return &Client{httpClient: httpClient}
}

// Fetch 获取目录原始 JSON；url 为空时使用官方地址。
func (c *Client) Fetch(ctx context.Context, url string) ([]byte, error) {
	if c == nil || c.httpClient == nil {
		return nil, fmt.Errorf("model catalog client is not configured")
	}
	url = strings.TrimSpace(url)
	if url == "" {
		url = DefaultURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
		return nil, fmt.Errorf("model catalog request failed: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogResponse+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxCatalogResponse {
		return nil, fmt.Errorf("model catalog response exceeds %d bytes", maxCatalogResponse)
	}
	return body, nil
}

// FetchCatalog 从官方地址拉取目录并精简为模型目录条目；目录为空或结构异常时返回错误。
func (c *Client) FetchCatalog(ctx context.Context) ([]domainchannel.ModelCatalogEntry, error) {
	raw, err := c.Fetch(ctx, DefaultURL)
	if err != nil {
		return nil, err
	}
	return ParseCatalog(raw)
}
