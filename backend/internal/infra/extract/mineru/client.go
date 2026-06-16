package mineru

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

const (
	errMinerUEmptyContent = "mineru_empty_content"
	DefaultBaseURL        = "https://mineru.net/api/v4"
)

const (
	SourceCloud      = "cloud"
	SourceSelfHosted = "self_hosted"
)

// ClientConfig 表示 MinerU 服务接入配置。
type ClientConfig struct {
	Source                string
	BaseURL               string
	AuthToken             string
	TimeoutSeconds        int
	Env                   string
	SSRFProtectionEnabled bool
}

// Request 表示一次 MinerU 文本提取请求。
type Request struct {
	AbsolutePath string
	FileName     string
}

// Client 提供 MinerU HTTP 文本提取能力。
type Client struct {
	source             string
	baseURL            string
	authToken          string
	httpClient         *http.Client
	artifactHTTPClient *http.Client
}

// New 创建 MinerU 客户端；未配置地址时返回 nil。
func New(cfg ClientConfig) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil
	}
	source := normalizeSource(cfg.Source)
	timeout := resolveHTTPTimeout(cfg.TimeoutSeconds, 180*time.Second)
	return &Client{
		source:             source,
		baseURL:            baseURL,
		authToken:          strings.TrimSpace(cfg.AuthToken),
		httpClient:         newServiceHTTPClient(timeout, source, cfg.Env, cfg.SSRFProtectionEnabled),
		artifactHTTPClient: newSafeHTTPClient(timeout, cfg.Env, cfg.SSRFProtectionEnabled),
	}
}

func newServiceHTTPClient(timeout time.Duration, source string, env string, ssrfProtectionEnabled bool) *http.Client {
	if normalizeSource(source) == SourceSelfHosted {
		return platformtracing.NewHTTPClient(timeout)
	}
	return newSafeHTTPClient(timeout, env, ssrfProtectionEnabled)
}

func newSafeHTTPClient(timeout time.Duration, env string, ssrfProtectionEnabled bool) *http.Client {
	transport := security.NewOutboundHTTPTransport(env, ssrfProtectionEnabled, 10*time.Second)
	return &http.Client{
		Timeout:   timeout,
		Transport: platformtracing.NewHTTPTransport(transport),
	}
}

// ProbeEndpoint 检测指定 MinerU 服务地址是否可用。
func ProbeEndpoint(ctx context.Context, baseURL string, authToken string) (bool, string) {
	return probeEndpoint(ctx, baseURL, authToken, platformtracing.NewHTTPClient(1200*time.Millisecond))
}

func probeEndpoint(ctx context.Context, baseURL string, authToken string, httpClient *http.Client) (bool, string) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return false, "服务地址为空。"
	}

	if ok, message := probeCloudEndpoint(ctx, baseURL, authToken, httpClient); ok {
		return true, ""
	} else if message == "mineru_unauthorized" || message == "mineru_forbidden" {
		return false, message
	}

	if ok, message := probeSelfHostedEndpoint(ctx, baseURL, authToken, httpClient); ok {
		return true, ""
	} else if message == "mineru_unauthorized" || message == "mineru_forbidden" {
		return false, message
	}

	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(requestCtx, 1200*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, baseURL, nil)
	if err != nil {
		return false, "服务地址格式不正确。"
	}
	applyAuthHeaders(req, authToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))

	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return true, fmt.Sprintf("服务已响应: %d", resp.StatusCode)
	}
	return false, fmt.Sprintf("服务响应异常: %d", resp.StatusCode)
}

func probeCloudEndpoint(ctx context.Context, baseURL string, authToken string, httpClient *http.Client) (bool, string) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return false, "服务地址为空。"
	}

	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(requestCtx, 1200*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, baseURL+"/extract-results/batch/ping", nil)
	if err != nil {
		return false, "服务地址格式不正确。"
	}
	applyAuthHeaders(req, authToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return true, ""
	case resp.StatusCode == http.StatusUnauthorized:
		return false, "mineru_unauthorized"
	case resp.StatusCode == http.StatusForbidden:
		return false, "mineru_forbidden"
	case resp.StatusCode < 500:
		return true, fmt.Sprintf("服务已响应: %d", resp.StatusCode)
	default:
		return false, fmt.Sprintf("服务响应异常: %d", resp.StatusCode)
	}
}

func probeSelfHostedEndpoint(ctx context.Context, baseURL string, authToken string, httpClient *http.Client) (bool, string) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return false, "服务地址为空。"
	}

	for _, endpoint := range []string{"/openapi.json", "/docs"} {
		requestCtx := ctx
		if requestCtx == nil {
			requestCtx = context.Background()
		}
		requestCtx, cancel := context.WithTimeout(requestCtx, 1200*time.Millisecond)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, baseURL+endpoint, nil)
		if err != nil {
			cancel()
			return false, "服务地址格式不正确。"
		}
		applyAuthHeaders(req, authToken)

		resp, err := httpClient.Do(req)
		cancel()
		if err != nil {
			continue
		}
		func() {
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
		}()
		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			return true, ""
		case resp.StatusCode == http.StatusUnauthorized:
			return false, "mineru_unauthorized"
		case resp.StatusCode == http.StatusForbidden:
			return false, "mineru_forbidden"
		}
	}

	return false, "未检测到 MinerU 自部署 API 或云端批处理接口。"
}

// ExtractText 调用 MinerU 提取文本。
func (c *Client) ExtractText(ctx context.Context, req Request) (string, error) {
	if strings.TrimSpace(req.AbsolutePath) == "" {
		return "", fmt.Errorf("mineru_invalid_file_path")
	}
	if c == nil || c.baseURL == "" {
		return "", fmt.Errorf("mineru_unavailable")
	}

	if c.source == SourceSelfHosted {
		text, err := c.extractTextSelfHosted(ctx, req)
		if err == nil {
			return text, nil
		}
		if !shouldFallbackToCloud(err) {
			return "", err
		}
	}

	batchID, uploadURL, err := c.createBatch(ctx, req)
	if err != nil {
		if shouldFallbackToSelfHosted(err) {
			return c.extractTextSelfHosted(ctx, req)
		}
		return "", err
	}
	if err := c.uploadFile(ctx, uploadURL, req.AbsolutePath); err != nil {
		return "", err
	}
	zipURL, err := c.pollBatch(ctx, batchID)
	if err != nil {
		return "", err
	}
	return c.downloadResult(ctx, zipURL)
}

func (c *Client) extractTextSelfHosted(ctx context.Context, req Request) (string, error) {
	file, err := os.Open(strings.TrimSpace(req.AbsolutePath))
	if err != nil {
		return "", err
	}

	fileName := strings.TrimSpace(req.FileName)
	if fileName == "" {
		fileName = filepath.Base(strings.TrimSpace(req.AbsolutePath))
	}

	bodyReader, contentType, writeErrCh := buildSelfHostedMultipartBody(file, fileName)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/file_parse", bodyReader)
	if err != nil {
		_ = bodyReader.Close()
		return "", err
	}
	httpReq.Header.Set("Content-Type", contentType)
	httpReq.Header.Set("Accept", "application/json")
	applyAuthHeaders(httpReq, c.authToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		_ = bodyReader.Close()
		if writeErr := awaitMultipartWriteError(writeErrCh); writeErr != nil {
			return "", writeErr
		}
		return "", fmt.Errorf("mineru_unavailable")
	}
	defer resp.Body.Close()
	if writeErr := awaitMultipartWriteError(writeErrCh); writeErr != nil {
		return "", writeErr
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("mineru_unauthorized")
	}
	if resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("mineru_forbidden")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			return "", fmt.Errorf("mineru_http_%d", resp.StatusCode)
		}
		return "", fmt.Errorf("mineru_http_%d: %s", resp.StatusCode, detail)
	}

	var parsed selfHostedResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 20*1024*1024)).Decode(&parsed); err != nil {
		return "", fmt.Errorf("mineru_invalid_response")
	}

	text := normalizeText(parsed.firstMarkdown())
	if text == "" {
		return "", fmt.Errorf(errMinerUEmptyContent)
	}
	return text, nil
}

func (c *Client) createBatch(ctx context.Context, req Request) (string, string, error) {
	fileName := strings.TrimSpace(req.FileName)
	if fileName == "" {
		fileName = filepath.Base(strings.TrimSpace(req.AbsolutePath))
	}

	payload := map[string]any{
		"language":       "ch",
		"enable_formula": true,
		"enable_table":   true,
		"files": []map[string]any{
			{
				"name":    fileName,
				"data_id": fileName,
				"is_ocr":  true,
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/file-urls/batch", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	applyAuthHeaders(httpReq, c.authToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", "", fmt.Errorf("mineru_unavailable")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", fmt.Errorf("mineru_unauthorized")
	}
	if resp.StatusCode == http.StatusForbidden {
		return "", "", fmt.Errorf("mineru_forbidden")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			return "", "", fmt.Errorf("mineru_http_%d", resp.StatusCode)
		}
		return "", "", fmt.Errorf("mineru_http_%d: %s", resp.StatusCode, detail)
	}

	var parsed batchCreateResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&parsed); err != nil {
		return "", "", fmt.Errorf("mineru_invalid_response")
	}
	if strings.TrimSpace(parsed.Data.BatchID) == "" || len(parsed.Data.FileURLs) == 0 || strings.TrimSpace(parsed.Data.FileURLs[0]) == "" {
		return "", "", fmt.Errorf("mineru_invalid_response")
	}
	return strings.TrimSpace(parsed.Data.BatchID), strings.TrimSpace(parsed.Data.FileURLs[0]), nil
}

func (c *Client) uploadFile(ctx context.Context, uploadURL string, absolutePath string) error {
	file, err := os.Open(strings.TrimSpace(absolutePath))
	if err != nil {
		return err
	}
	defer file.Close()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, file)
	if err != nil {
		return err
	}
	resp, err := c.artifactHTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("mineru_unavailable")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			return fmt.Errorf("mineru_http_%d", resp.StatusCode)
		}
		return fmt.Errorf("mineru_http_%d: %s", resp.StatusCode, detail)
	}
	return nil
}

func (c *Client) pollBatch(ctx context.Context, batchID string) (string, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	endpoint := c.baseURL + "/extract-results/batch/" + batchID
	for {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", err
		}
		httpReq.Header.Set("Accept", "application/json")
		applyAuthHeaders(httpReq, c.authToken)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			return "", fmt.Errorf("mineru_unavailable")
		}
		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			return "", fmt.Errorf("mineru_unauthorized")
		}
		if resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			return "", fmt.Errorf("mineru_forbidden")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			detail := strings.TrimSpace(string(body))
			if detail == "" {
				return "", fmt.Errorf("mineru_http_%d", resp.StatusCode)
			}
			return "", fmt.Errorf("mineru_http_%d: %s", resp.StatusCode, detail)
		}

		var parsed batchResultResponse
		err = json.NewDecoder(io.LimitReader(resp.Body, 4*1024*1024)).Decode(&parsed)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("mineru_invalid_response")
		}

		state := strings.ToLower(strings.TrimSpace(parsed.Data.State))
		var item batchResultItem
		if len(parsed.Data.ExtractResult) > 0 {
			item = parsed.Data.ExtractResult[0]
			if itemState := strings.ToLower(strings.TrimSpace(item.State)); itemState != "" {
				state = itemState
			}
		}
		switch state {
		case "done", "success", "completed":
			if len(parsed.Data.ExtractResult) == 0 {
				return "", fmt.Errorf("mineru_invalid_response")
			}
			zipURL := strings.TrimSpace(item.FullZipURL)
			if zipURL == "" {
				return "", fmt.Errorf("mineru_invalid_response")
			}
			return zipURL, nil
		case "failed", "error":
			detail := strings.TrimSpace(parsed.Data.ErrMsg)
			if detail == "" {
				detail = strings.TrimSpace(item.ErrMsg)
			}
			if detail == "" {
				return "", fmt.Errorf("mineru_failed")
			}
			return "", fmt.Errorf("mineru_unprocessable: %s", detail)
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *Client) downloadResult(ctx context.Context, zipURL string) (string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, zipURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.artifactHTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("mineru_unavailable")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			return "", fmt.Errorf("mineru_http_%d", resp.StatusCode)
		}
		return "", fmt.Errorf("mineru_http_%d: %s", resp.StatusCode, detail)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024*1024))
	if err != nil {
		return "", err
	}
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", fmt.Errorf("mineru_invalid_response")
	}

	for _, file := range reader.File {
		name := strings.ToLower(strings.TrimSpace(file.Name))
		if !strings.HasSuffix(name, "full.md") && !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".markdown") {
			continue
		}
		rc, openErr := file.Open()
		if openErr != nil {
			continue
		}
		content, readErr := io.ReadAll(io.LimitReader(rc, 20*1024*1024))
		rc.Close()
		if readErr != nil {
			continue
		}
		text := normalizeText(string(content))
		if text != "" {
			return text, nil
		}
	}

	return "", fmt.Errorf(errMinerUEmptyContent)
}

func normalizeText(raw string) string {
	lines := strings.Split(raw, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return strings.Join(result, "\n")
}

func normalizeSource(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case SourceSelfHosted:
		return SourceSelfHosted
	default:
		return SourceCloud
	}
}

func shouldFallbackToSelfHosted(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "mineru_http_404") || strings.Contains(message, "mineru_invalid_response")
}

func shouldFallbackToCloud(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "mineru_http_404") || strings.Contains(message, "mineru_invalid_response")
}

func applyAuthHeaders(req *http.Request, authToken string) {
	if req == nil {
		return
	}
	token := strings.TrimSpace(authToken)
	if token == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-API-Key", token)
	req.Header.Set("token", token)
}

func resolveHTTPTimeout(raw int, fallback time.Duration) time.Duration {
	timeout := time.Duration(raw) * time.Second
	if timeout <= 0 {
		return fallback
	}
	return timeout
}

func buildSelfHostedMultipartBody(file *os.File, fileName string) (io.ReadCloser, string, <-chan error) {
	bodyReader, bodyWriter := io.Pipe()
	writer := multipart.NewWriter(bodyWriter)
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)
		defer file.Close() //nolint:errcheck

		fail := func(err error) {
			errCh <- err
			_ = bodyWriter.CloseWithError(err)
		}

		part, err := writer.CreateFormFile("files", fileName)
		if err != nil {
			fail(err)
			return
		}
		if _, err = io.Copy(part, file); err != nil {
			fail(err)
			return
		}

		if err = writer.WriteField("lang_list", "ch"); err != nil {
			fail(err)
			return
		}
		if err = writer.WriteField("backend", "pipeline"); err != nil {
			fail(err)
			return
		}
		if err = writer.WriteField("parse_method", "auto"); err != nil {
			fail(err)
			return
		}
		if err = writer.WriteField("formula_enable", "true"); err != nil {
			fail(err)
			return
		}
		if err = writer.WriteField("table_enable", "true"); err != nil {
			fail(err)
			return
		}
		if err = writer.WriteField("return_md", "true"); err != nil {
			fail(err)
			return
		}
		if err = writer.WriteField("response_format_zip", "false"); err != nil {
			fail(err)
			return
		}

		if err = writer.Close(); err != nil {
			fail(err)
			return
		}
		_ = bodyWriter.Close()
	}()

	return bodyReader, writer.FormDataContentType(), errCh
}

func awaitMultipartWriteError(errCh <-chan error) error {
	if errCh == nil {
		return nil
	}
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

type batchCreateResponse struct {
	Data struct {
		BatchID  string   `json:"batch_id"`
		FileURLs []string `json:"file_urls"`
	} `json:"data"`
}

type selfHostedResponse struct {
	Results map[string]selfHostedResult `json:"results"`
}

type selfHostedResult struct {
	MarkdownContent string `json:"md_content"`
}

func (r selfHostedResponse) firstMarkdown() string {
	for _, item := range r.Results {
		if strings.TrimSpace(item.MarkdownContent) != "" {
			return item.MarkdownContent
		}
	}
	return ""
}

type batchResultResponse struct {
	Data struct {
		State         string            `json:"state"`
		ErrMsg        string            `json:"err_msg"`
		ExtractResult []batchResultItem `json:"extract_result"`
	} `json:"data"`
}

type batchResultItem struct {
	State      string `json:"state"`
	ErrMsg     string `json:"err_msg"`
	FullZipURL string `json:"full_zip_url"`
}
