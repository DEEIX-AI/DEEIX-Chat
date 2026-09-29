// Package contentmoderation 为管理员配置的审核服务
// 提供受管控的出站 HTTP 边界。
package contentmoderation

import (
	"fmt"
	"net/http"
	"time"

	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/outboundhttp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

const moderationRequestTimeout = 30 * time.Second

// Client 为审核端点复用按 origin 划分的 HTTP transport。
type Client struct {
	pool      *outboundhttp.Pool
	doRequest func(request *http.Request, configuredEndpoint string) (*http.Response, error)
}

// New 在注入的出站策略下创建受管控的客户端。
func New(outboundPolicy security.OutboundPolicy) *Client {
	return &Client{pool: outboundhttp.NewPool(
		outboundPolicy,
		outboundhttp.DefaultCacheLimit,
		func(policy security.OutboundPolicy, trustedOrigin string, _ string) (outboundhttp.ManagedClient, error) {
			client := security.NewOutboundHTTPClient(policy, moderationRequestTimeout)
			transport, ok := client.Transport.(*http.Transport)
			if !ok {
				return outboundhttp.ManagedClient{}, fmt.Errorf("moderation HTTP transport is not reusable")
			}
			client.Transport = platformtracing.NewHTTPTransport(transport)
			if trustedOrigin != "" {
				client.CheckRedirect = outboundhttp.NewRedirectPolicy(outboundPolicy, trustedOrigin, "content moderation request")
			}
			return outboundhttp.ManagedClient{
				Client:               client,
				CloseIdleConnections: transport.CloseIdleConnections,
			}, nil
		},
	)}
}

// Do 仅针对管理员配置的精确 origin 执行请求。
func (c *Client) Do(request *http.Request, configuredEndpoint string) (*http.Response, error) {
	if c != nil && c.doRequest != nil {
		return c.doRequest(request, configuredEndpoint)
	}
	if c == nil || c.pool == nil {
		return nil, fmt.Errorf("content moderation HTTP client is not configured")
	}
	return c.pool.Do(request, configuredEndpoint, "")
}

// CloseIdleConnections 在应用关闭时释放池化的 transport。
func (c *Client) CloseIdleConnections() {
	if c != nil && c.pool != nil {
		c.pool.CloseIdleConnections()
	}
}
