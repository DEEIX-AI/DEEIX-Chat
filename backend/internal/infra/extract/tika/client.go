package tika

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
)

// Request 表示 Tika 文本提取请求。
type Request struct {
	AbsolutePath string
	FileName     string
	MimeType     string
}

// Client 提供 Apache Tika HTTP 提取能力。
type Client struct {
	baseURL    string
	authToken  string
	httpClient *http.Client
}

const (
	errTikaEmptyContent        = "tika_empty_content"
	errTikaUnprocessable       = "tika_unprocessable"
	errTikaUnauthorized        = "tika_unauthorized"
	errTikaForbidden           = "tika_forbidden"
	errTikaUnsupportedMimeType = "tika_unsupported_media_type"
	DefaultTikaBaseURL         = "http://127.0.0.1:9998"
	ManagedTikaBaseURL         = "http://deeix-chat-tika:9998"
	managedTikaHost            = "deeix-chat-tika"
	tikaSourceManaged          = "managed"
)

// New 创建 Tika 客户端；未配置地址时返回 nil。
func New(cfg config.Config) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(resolveBaseURL(cfg)), "/")
	if baseURL == "" {
		return nil
	}
	timeout := time.Duration(cfg.ExtractTikaTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		baseURL:    baseURL,
		authToken:  strings.TrimSpace(cfg.ExtractTikaAuthToken),
		httpClient: platformtracing.NewHTTPClient(timeout),
	}
}

func resolveBaseURL(cfg config.Config) string {
	if strings.EqualFold(strings.TrimSpace(cfg.ExtractTikaSource), tikaSourceManaged) {
		return ResolveManagedBaseURL(context.Background())
	}
	baseURL := strings.TrimSpace(cfg.ExtractTikaBaseURL)
	return baseURL
}

// ResolveManagedBaseURL 为系统托管的 Tika 服务解析可用访问地址。
// 优先使用容器网络内地址，宿主机运行时回退到回环地址。
func ResolveManagedBaseURL(ctx context.Context) string {
	if ok, _ := ProbeEndpoint(ctx, ManagedTikaBaseURL, ""); ok {
		return ManagedTikaBaseURL
	}
	if ok, _ := ProbeEndpoint(ctx, DefaultTikaBaseURL, ""); ok {
		return DefaultTikaBaseURL
	}
	if managedTikaHostResolvable(ctx) {
		return ManagedTikaBaseURL
	}
	return DefaultTikaBaseURL
}

func managedTikaHostResolvable(ctx context.Context) bool {
	lookupCtx := ctx
	if lookupCtx == nil {
		lookupCtx = context.Background()
	}
	lookupCtx, cancel := context.WithTimeout(lookupCtx, 300*time.Millisecond)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupHost(lookupCtx, managedTikaHost)
	return err == nil && len(addrs) > 0
}

// ProbeEndpoint 检测指定 Tika 服务地址是否可用。
// 探活使用官方文档明确支持的 GET /tika，而不是根路径。
func ProbeEndpoint(ctx context.Context, baseURL string, authToken string) (bool, string) {
	return probeEndpoint(ctx, baseURL, authToken, platformtracing.NewHTTPClient(800*time.Millisecond))
}

func probeEndpoint(ctx context.Context, baseURL string, authToken string, httpClient *http.Client) (bool, string) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return false, "服务地址为空。"
	}

	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(requestCtx, 800*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, baseURL+"/tika", nil)
	if err != nil {
		return false, "服务地址格式不正确。"
	}
	if token := strings.TrimSpace(authToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, ""
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return false, errTikaUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return false, errTikaForbidden
	}
	return false, fmt.Sprintf("服务响应异常: %d", resp.StatusCode)
}

// ExtractText 调用 Tika 提取文本。
func (c *Client) ExtractText(ctx context.Context, input Request) (string, error) {
	if c == nil {
		return "", fmt.Errorf("tika_disabled")
	}

	file, err := os.Open(strings.TrimSpace(input.AbsolutePath))
	if err != nil {
		return "", err
	}
	defer file.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/tika", file)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain")
	if value := strings.TrimSpace(input.MimeType); value != "" {
		req.Header.Set("Content-Type", value)
	}
	if value := strings.TrimSpace(input.FileName); value != "" {
		req.Header.Set("X-Tika-Filename", value)
	}
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return "", fmt.Errorf(errTikaEmptyContent)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		detail := strings.TrimSpace(string(body))
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return "", fmt.Errorf(errTikaUnauthorized)
		case http.StatusForbidden:
			return "", fmt.Errorf(errTikaForbidden)
		case http.StatusUnsupportedMediaType:
			return "", fmt.Errorf(errTikaUnsupportedMimeType)
		case http.StatusUnprocessableEntity:
			if detail == "" {
				return "", fmt.Errorf(errTikaUnprocessable)
			}
			return "", fmt.Errorf("%s: %s", errTikaUnprocessable, detail)
		default:
			if detail == "" {
				return "", fmt.Errorf("tika_http_%d", resp.StatusCode)
			}
			return "", fmt.Errorf("tika_http_%d: %s", resp.StatusCode, detail)
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "", fmt.Errorf(errTikaEmptyContent)
	}
	return text, nil
}
