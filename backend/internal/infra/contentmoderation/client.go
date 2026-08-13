// Package contentmoderation provides the managed outbound HTTP boundary for
// administrator-configured moderation services.
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

// Client reuses origin-scoped HTTP transports for moderation endpoints.
type Client struct {
	pool      *outboundhttp.Pool
	doRequest func(request *http.Request, configuredEndpoint string) (*http.Response, error)
}

// New creates a managed client under the injected outbound policy.
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

// Do executes a request only against the exact origin configured by an administrator.
func (c *Client) Do(request *http.Request, configuredEndpoint string) (*http.Response, error) {
	if c != nil && c.doRequest != nil {
		return c.doRequest(request, configuredEndpoint)
	}
	if c == nil || c.pool == nil {
		return nil, fmt.Errorf("content moderation HTTP client is not configured")
	}
	return c.pool.Do(request, configuredEndpoint, "")
}

// CloseIdleConnections releases pooled transports during application shutdown.
func (c *Client) CloseIdleConnections() {
	if c != nil && c.pool != nil {
		c.pool.CloseIdleConnections()
	}
}
