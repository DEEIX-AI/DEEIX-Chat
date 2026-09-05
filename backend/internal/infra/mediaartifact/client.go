// Package mediaartifact 负责下载模型提供商返回的临时媒体制品。
package mediaartifact

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/outboundhttp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

const (
	defaultConnectTimeout   = 10 * time.Second
	imageDownloadTimeout    = 60 * time.Second
	videoDownloadTimeout    = 120 * time.Second
	metadataRequestTimeout  = 30 * time.Second
	metadataResponseLimit   = 1 << 20
	errorResponseDrainLimit = 64 << 10
	geminiFilePollAttempts  = 12
	geminiFilePollInterval  = 5 * time.Second
	maxRedirects            = 10
	geminiAPIKeyHeader      = "x-goog-api-key"
	geminiFilesHostname     = "generativelanguage.googleapis.com"
)

var (
	// ErrResponseTooLarge 表示上游媒体制品超过调用方指定的大小上限。
	ErrResponseTooLarge error = responseTooLargeError{}
	errMetadataTooLarge       = errors.New("media artifact metadata response is too large")
)

type responseTooLargeError struct{}

func (responseTooLargeError) Error() string {
	return "media artifact response is too large"
}

// MediaArtifactResponseTooLarge 为调用方提供稳定的大小超限错误分类能力。
func (responseTooLargeError) MediaArtifactResponseTooLarge() {}

type downloadResult struct {
	Data     []byte
	MIMEType string
}

// Client 维护提供商返回媒体 URL 所使用的隔离出站客户端。
// 仅当制品 URL 与管理员配置的模型 endpoint 同 origin 时继承局部信任；其他 URL 仍按不可信输入处理。
type Client struct {
	basePolicy   security.OutboundPolicy
	httpClients  *outboundhttp.Pool
	pollAttempts int
	pollInterval time.Duration
}

// New 使用注入的严格出站策略创建可复用的媒体制品客户端。
func New(policy security.OutboundPolicy) *Client {
	return &Client{
		basePolicy: policy,
		httpClients: outboundhttp.NewPool(policy, outboundhttp.DefaultCacheLimit, func(trustedPolicy security.OutboundPolicy, trustedOrigin string, _ string) (outboundhttp.ManagedClient, error) {
			return newMediaArtifactHTTPClient(trustedPolicy, policy, trustedOrigin), nil
		}),
		pollAttempts: geminiFilePollAttempts,
		pollInterval: geminiFilePollInterval,
	}
}

func newMediaArtifactHTTPClient(policy security.OutboundPolicy, strictPolicy security.OutboundPolicy, trustedOrigin string) outboundhttp.ManagedClient {
	transport := security.NewOutboundHTTPTransport(policy, defaultConnectTimeout)
	// 媒体制品 URL 经常携带签名查询参数，不能使用会记录 url.full 的通用 HTTP tracing。
	// 请求级诊断由 application 的脱敏结构化日志承担。
	httpClient := &http.Client{Transport: transport, CheckRedirect: mediaArtifactRedirectPolicy(strictPolicy, trustedOrigin)}
	return outboundhttp.ManagedClient{Client: httpClient, CloseIdleConnections: transport.CloseIdleConnections}
}

// DownloadImage 从不可信的提供商返回 URL 下载生成图片。
func (c *Client) DownloadImage(ctx context.Context, sourceURL string, trustedProviderEndpoint string, maxBytes int64) ([]byte, string, error) {
	if _, _, ok := geminiGeneratedFileURLs(sourceURL); ok {
		return nil, "", errors.New("gemini files generated image URI is not supported")
	}
	result, err := c.download(ctx, downloadRequest{
		url:                sourceURL,
		trustedEndpoint:    trustedProviderEndpoint,
		maxBytes:           maxBytes,
		timeout:            imageDownloadTimeout,
		expectedMIMEPrefix: "image/",
		failureLabel:       "download generated image",
	})
	return result.Data, result.MIMEType, err
}

// DownloadVideo 下载生成视频，并按需等待和解析 Gemini Files 制品。
func (c *Client) DownloadVideo(ctx context.Context, sourceURL string, trustedProviderEndpoint string, apiKey string, maxBytes int64) ([]byte, string, error) {
	resolvedMIME := ""
	headers := map[string]string(nil)
	providerBearerToken := strings.TrimSpace(apiKey)
	downloadURL := strings.TrimSpace(sourceURL)
	metadataURL, geminiDownloadURL, geminiFile := geminiGeneratedFileURLs(downloadURL)
	if geminiFile {
		if strings.TrimSpace(apiKey) == "" {
			return nil, "", errors.New("gemini files generated video URI requires an API key")
		}
		var err error
		resolvedMIME, err = c.waitGeminiFileReady(ctx, metadataURL, trustedProviderEndpoint, apiKey)
		if err != nil {
			return nil, "", err
		}
		downloadURL = geminiDownloadURL
		headers = map[string]string{geminiAPIKeyHeader: strings.TrimSpace(apiKey)}
		providerBearerToken = ""
	}

	result, err := c.download(ctx, downloadRequest{
		url:                 downloadURL,
		trustedEndpoint:     trustedProviderEndpoint,
		headers:             headers,
		providerBearerToken: providerBearerToken,
		maxBytes:            maxBytes,
		timeout:             videoDownloadTimeout,
		expectedMIMEPrefix:  "video/",
		failureLabel:        "download generated video",
	})
	if err != nil {
		return nil, "", err
	}
	if result.MIMEType == "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(resolvedMIME)), "video/") {
		result.MIMEType = strings.TrimSpace(resolvedMIME)
	}
	return result.Data, result.MIMEType, nil
}

type downloadRequest struct {
	url                 string
	trustedEndpoint     string
	headers             map[string]string
	providerBearerToken string
	maxBytes            int64
	timeout             time.Duration
	expectedMIMEPrefix  string
	failureLabel        string
}

// download 统一执行带超时、状态码和响应大小边界的媒体下载。
func (c *Client) download(ctx context.Context, input downloadRequest) (downloadResult, error) {
	if c == nil || c.httpClients == nil {
		return downloadResult{}, fmt.Errorf("media artifact client is not configured")
	}
	if input.maxBytes <= 0 {
		return downloadResult{}, fmt.Errorf("media artifact size limit must be positive")
	}
	trustedEndpoint, policy, err := c.artifactPolicy(input.url, input.trustedEndpoint)
	if err != nil {
		return downloadResult{}, fmt.Errorf("%s failed: %w", input.failureLabel, err)
	}
	if err := security.ValidateOutboundHTTPURL(input.url, policy); err != nil {
		return downloadResult{}, fmt.Errorf("%s failed: %w", input.failureLabel, err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, input.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, strings.TrimSpace(input.url), nil)
	if err != nil {
		return downloadResult{}, err
	}
	for key, value := range input.headers {
		request.Header.Set(key, value)
	}
	// 仅在制品 URL 与管理员配置的 Provider endpoint 明确同源时携带 Key。
	// 跨 origin 制品和后续跨 origin 重定向都不得获得 Provider 凭据。
	if trustedEndpoint != "" && sameArtifactOrigin(input.url, input.trustedEndpoint) && strings.TrimSpace(input.providerBearerToken) != "" {
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(input.providerBearerToken))
	}
	response, err := c.httpClients.Do(request, trustedEndpoint, "")
	if err != nil {
		return downloadResult{}, sanitizeRequestError(input.failureLabel, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		drainErrorResponse(response.Body)
		return downloadResult{}, fmt.Errorf("%s failed: HTTP %d", input.failureLabel, response.StatusCode)
	}
	data, err := readLimited(response.Body, input.maxBytes, ErrResponseTooLarge)
	if err != nil {
		return downloadResult{}, err
	}
	return downloadResult{
		Data:     data,
		MIMEType: responseMIMEType(response.Header.Get("Content-Type"), input.expectedMIMEPrefix),
	}, nil
}

func (c *Client) artifactPolicy(sourceURL string, trustedProviderEndpoint string) (string, security.OutboundPolicy, error) {
	trustedEndpoint := ""
	providerEndpoint := strings.TrimSpace(trustedProviderEndpoint)
	if providerEndpoint != "" {
		providerOrigin, err := security.HTTPOrigin(providerEndpoint)
		if err != nil {
			return "", security.OutboundPolicy{}, fmt.Errorf("invalid configured model endpoint: %w", err)
		}
		sourceOrigin, sourceErr := security.HTTPOrigin(sourceURL)
		if sourceErr == nil && sourceOrigin == providerOrigin {
			trustedEndpoint = providerEndpoint
		}
	}
	policy := c.basePolicy
	if trustedEndpoint != "" {
		var err error
		policy, err = c.basePolicy.WithTrustedHTTPURLs(trustedEndpoint)
		if err != nil {
			return "", security.OutboundPolicy{}, err
		}
	}
	return trustedEndpoint, policy, nil
}

// waitGeminiFileReady 按 Gemini Files 状态协议轮询，且始终服从调用方取消信号。
func (c *Client) waitGeminiFileReady(ctx context.Context, metadataURL string, trustedProviderEndpoint string, apiKey string) (string, error) {
	lastState := ""
	for attempt := 0; attempt < c.pollAttempts; attempt++ {
		state, mimeType, err := c.fetchGeminiFileState(ctx, metadataURL, trustedProviderEndpoint, apiKey)
		if err != nil {
			return "", err
		}
		if geminiFileReady(state) {
			return mimeType, nil
		}
		lastState = state
		if geminiFileFailed(state) {
			return "", fmt.Errorf("generated video file failed: %s", state)
		}
		if attempt == c.pollAttempts-1 {
			break
		}
		timer := time.NewTimer(c.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
	return "", fmt.Errorf("generated video file is not ready: %s", strings.TrimSpace(lastState))
}

// fetchGeminiFileState 在独立请求超时和元数据大小边界内读取一次文件状态。
func (c *Client) fetchGeminiFileState(ctx context.Context, metadataURL string, trustedProviderEndpoint string, apiKey string) (string, string, error) {
	trustedEndpoint, policy, err := c.artifactPolicy(metadataURL, trustedProviderEndpoint)
	if err != nil {
		return "", "", fmt.Errorf("poll generated video file failed: %w", err)
	}
	if err := security.ValidateOutboundHTTPURL(metadataURL, policy); err != nil {
		return "", "", fmt.Errorf("poll generated video file failed: %w", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, metadataRequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return "", "", err
	}
	request.Header.Set(geminiAPIKeyHeader, strings.TrimSpace(apiKey))
	response, err := c.httpClients.Do(request, trustedEndpoint, "")
	if err != nil {
		return "", "", sanitizeRequestError("poll generated video file", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		drainErrorResponse(response.Body)
		return "", "", fmt.Errorf("poll generated video file failed: HTTP %d", response.StatusCode)
	}
	body, err := readLimited(response.Body, metadataResponseLimit, errMetadataTooLarge)
	if err != nil {
		return "", "", fmt.Errorf("read generated video file metadata: %w", err)
	}
	var payload struct {
		State       string `json:"state"`
		MIMEType    string `json:"mimeType"`
		MIMETypeAlt string `json:"mime_type"`
		File        *struct {
			State       string `json:"state"`
			MIMEType    string `json:"mimeType"`
			MIMETypeAlt string `json:"mime_type"`
		} `json:"file"`
	}
	if err = json.Unmarshal(body, &payload); err != nil {
		return "", "", err
	}
	state := textutil.FirstNonEmpty(payload.State)
	mimeType := textutil.FirstNonEmpty(payload.MIMEType, payload.MIMETypeAlt)
	if payload.File != nil {
		state = textutil.FirstNonEmpty(state, payload.File.State)
		mimeType = textutil.FirstNonEmpty(mimeType, payload.File.MIMEType, payload.File.MIMETypeAlt)
	}
	return state, mimeType, nil
}

// readLimited 多读取一个字节以可靠识别刚好越界的响应，避免无上限占用内存。
func readLimited(reader io.Reader, maxBytes int64, limitError error) ([]byte, error) {
	readLimit := maxBytes
	if maxBytes < int64(^uint64(0)>>1) {
		readLimit++
	}
	data, err := io.ReadAll(io.LimitReader(reader, readLimit))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: limit is %d bytes", limitError, maxBytes)
	}
	return data, nil
}

func responseMIMEType(contentType string, expectedPrefix string) string {
	normalized := strings.TrimSpace(contentType)
	if !strings.HasPrefix(strings.ToLower(normalized), strings.ToLower(expectedPrefix)) {
		return ""
	}
	return strings.TrimSpace(strings.Split(normalized, ";")[0])
}

// drainErrorResponse 有界丢弃错误响应，避免把第三方正文带入错误与日志。
func drainErrorResponse(body io.Reader) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(body, errorResponseDrainLimit))
}

// sanitizeRequestError 移除 url.Error 中可能含签名参数的完整 URL，同时保留底层错误链。
func sanitizeRequestError(operation string, err error) error {
	var requestError *url.Error
	if errors.As(err, &requestError) && requestError.Err != nil {
		err = requestError.Err
	}
	return fmt.Errorf("%s failed: %w", operation, err)
}

// geminiGeneratedFileURLs 仅识别官方 Gemini Files 主机，并移除上游 URL 中的查询凭据。
func geminiGeneratedFileURLs(rawURL string) (string, string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || !strings.EqualFold(parsed.Hostname(), geminiFilesHostname) {
		return "", "", false
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for index, segment := range segments {
		if !strings.EqualFold(segment, "files") || index+1 >= len(segments) {
			continue
		}
		fileID := strings.TrimSpace(segments[index+1])
		if colon := strings.Index(fileID, ":"); colon >= 0 {
			fileID = fileID[:colon]
		}
		if fileID == "" {
			return "", "", false
		}
		fileSegments := append([]string(nil), segments[:index+2]...)
		fileSegments[index+1] = fileID

		metadata := *parsed
		metadata.Path = "/" + strings.Join(fileSegments, "/")
		metadata.RawPath = ""
		metadata.RawQuery = ""
		metadata.Fragment = ""

		download := metadata
		download.Path = metadata.Path + ":download"
		download.RawQuery = "alt=media"
		return metadata.String(), download.String(), true
	}
	return "", "", false
}

func geminiFileReady(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), "ACTIVE")
}

func geminiFileFailed(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), "FAILED")
}

// stripCredentialOnCrossOriginRedirect 允许严格策略内的重定向，但禁止把 Gemini 密钥带到其他 origin。
func stripCredentialOnCrossOriginRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	if len(via) == 0 || sameHTTPOrigin(request.URL, via[0].URL) {
		return nil
	}
	request.Header.Del(geminiAPIKeyHeader)
	request.Header.Del("Authorization")
	return nil
}

func sameArtifactOrigin(sourceURL string, providerEndpoint string) bool {
	sourceOrigin, sourceErr := security.HTTPOrigin(strings.TrimSpace(sourceURL))
	providerOrigin, providerErr := security.HTTPOrigin(strings.TrimSpace(providerEndpoint))
	return sourceErr == nil && providerErr == nil && sourceOrigin == providerOrigin
}

func mediaArtifactRedirectPolicy(strictPolicy security.OutboundPolicy, trustedOrigin string) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if err := stripCredentialOnCrossOriginRedirect(request, via); err != nil {
			return err
		}
		if trustedOrigin != "" {
			redirectOrigin, err := security.HTTPOrigin(request.URL.String())
			if err == nil && redirectOrigin == trustedOrigin {
				return nil
			}
			trustedURL, trustedErr := url.Parse(trustedOrigin)
			if err != nil || trustedErr != nil || strings.EqualFold(request.URL.Hostname(), trustedURL.Hostname()) {
				return fmt.Errorf("media artifact redirect changed trusted origin: %w", security.ErrUnsafeOutboundURL)
			}
		}
		if err := security.ValidateOutboundHTTPURL(request.URL.String(), strictPolicy); err != nil {
			return fmt.Errorf("media artifact redirect target is not allowed: %w", err)
		}
		return nil
	}
}

func sameHTTPOrigin(left *url.URL, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}
	leftOrigin, leftErr := security.HTTPOrigin(left.String())
	rightOrigin, rightErr := security.HTTPOrigin(right.String())
	return leftErr == nil && rightErr == nil && leftOrigin == rightOrigin
}

// CloseIdleConnections 释放可复用传输层持有的空闲连接。
func (c *Client) CloseIdleConnections() {
	if c != nil && c.httpClients != nil {
		c.httpClients.CloseIdleConnections()
	}
}
