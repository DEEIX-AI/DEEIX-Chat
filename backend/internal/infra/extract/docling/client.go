package docling

import (
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
)

const (
	errDoclingEmptyContent = "docling_empty_content"
	DefaultBaseURL         = "http://127.0.0.1:8005/ocr"
	healthEndpoint         = "/healthz"
)

// ClientConfig 表示 Docling 服务接入配置。
type ClientConfig struct {
	BaseURL        string
	AuthToken      string
	TimeoutSeconds int
}

// Request 表示一次 Docling 文本提取请求。
type Request struct {
	AbsolutePath string
	FileName     string
	MimeType     string
}

// Client 提供 Docling HTTP 文本提取能力。
type Client struct {
	baseURL    string
	authToken  string
	httpClient *http.Client
}

// New 创建 Docling 客户端；未配置地址时返回 nil。
func New(cfg ClientConfig) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil
	}
	return &Client{
		baseURL:    baseURL,
		authToken:  strings.TrimSpace(cfg.AuthToken),
		httpClient: platformtracing.NewHTTPClient(resolveHTTPTimeout(cfg.TimeoutSeconds, 60*time.Second)),
	}
}

// ProbeEndpoint 检测指定 Docling 服务地址是否可用。
func ProbeEndpoint(ctx context.Context, baseURL string, authToken string) (bool, string) {
	return probeEndpoint(ctx, baseURL, authToken, platformtracing.NewHTTPClient(800*time.Millisecond))
}

func probeEndpoint(ctx context.Context, baseURL string, authToken string, httpClient *http.Client) (bool, string) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return false, "服务地址为空。"
	}
	healthURL := resolveHealthURL(baseURL)

	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(requestCtx, 800*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, healthURL, nil)
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
		return false, "docling_unauthorized"
	case resp.StatusCode == http.StatusForbidden:
		return false, "docling_forbidden"
	default:
		return false, fmt.Sprintf("服务响应异常: %d", resp.StatusCode)
	}
}

// ExtractText 调用 Docling 提取文本。
func (c *Client) ExtractText(ctx context.Context, req Request) (string, error) {
	if strings.TrimSpace(req.AbsolutePath) == "" {
		return "", fmt.Errorf("docling_invalid_file_path")
	}
	if c == nil || c.baseURL == "" {
		return "", fmt.Errorf("docling_unavailable")
	}

	file, err := os.Open(strings.TrimSpace(req.AbsolutePath))
	if err != nil {
		return "", err
	}

	fileName := strings.TrimSpace(req.FileName)
	if fileName == "" {
		fileName = filepath.Base(strings.TrimSpace(req.AbsolutePath))
	}

	bodyReader, contentType, writeErrCh := buildMultipartBody(file, fileName, strings.TrimSpace(req.MimeType))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bodyReader)
	if err != nil {
		_ = bodyReader.Close()
		return "", err
	}
	httpReq.Header.Set("Content-Type", contentType)
	httpReq.Header.Set("Accept", "application/json, text/plain")
	applyAuthHeaders(httpReq, c.authToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		_ = bodyReader.Close()
		if writeErr := awaitMultipartWriteError(writeErrCh); writeErr != nil {
			return "", writeErr
		}
		return "", err
	}
	defer resp.Body.Close()
	if writeErr := awaitMultipartWriteError(writeErrCh); writeErr != nil {
		return "", writeErr
	}

	if resp.StatusCode == http.StatusNoContent {
		return "", fmt.Errorf(errDoclingEmptyContent)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(bodyBytes))
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return "", fmt.Errorf("docling_unauthorized")
		case http.StatusForbidden:
			return "", fmt.Errorf("docling_forbidden")
		case http.StatusUnprocessableEntity:
			if detail == "" {
				return "", fmt.Errorf("docling_unprocessable")
			}
			return "", fmt.Errorf("docling_unprocessable: %s", detail)
		default:
			if detail == "" {
				return "", fmt.Errorf("docling_http_%d", resp.StatusCode)
			}
			return "", fmt.Errorf("docling_http_%d: %s", resp.StatusCode, detail)
		}
	}

	text, err := parseResponse(io.LimitReader(resp.Body, 50*1024*1024), resp.Header.Get("Content-Type"))
	if err != nil {
		return "", err
	}
	if text == "" {
		return "", fmt.Errorf(errDoclingEmptyContent)
	}
	return text, nil
}

func buildMultipartBody(file *os.File, fileName string, mimeType string) (io.ReadCloser, string, <-chan error) {
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

		part, err := writer.CreateFormFile("file", fileName)
		if err != nil {
			fail(err)
			return
		}
		if _, err = io.Copy(part, file); err != nil {
			fail(err)
			return
		}
		if fileName != "" {
			if err = writer.WriteField("file_name", fileName); err != nil {
				fail(err)
				return
			}
		}
		if mimeType != "" {
			if err = writer.WriteField("mime_type", mimeType); err != nil {
				fail(err)
				return
			}
		}
		if err = writer.Close(); err != nil {
			fail(err)
			return
		}
		_ = bodyWriter.Close()
	}()

	return bodyReader, writer.FormDataContentType(), errCh
}

func parseResponse(body io.Reader, contentType string) (string, error) {
	if strings.Contains(strings.ToLower(contentType), "application/json") {
		var parsed payload
		decoder := json.NewDecoder(body)
		if err := decoder.Decode(&parsed); err == nil {
			text := normalizeText(parsed.ExtractedText())
			if text == "" {
				return "", fmt.Errorf(errDoclingEmptyContent)
			}
			return text, nil
		}
	}

	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	text := normalizeText(string(bodyBytes))
	if text == "" {
		return "", fmt.Errorf(errDoclingEmptyContent)
	}
	return text, nil
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

func resolveHealthURL(baseURL string) string {
	if strings.HasSuffix(baseURL, "/ocr") {
		return strings.TrimSuffix(baseURL, "/ocr") + healthEndpoint
	}
	return baseURL + healthEndpoint
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type payload struct {
	Text     string      `json:"text"`
	FullText string      `json:"full_text"`
	Content  string      `json:"content"`
	Markdown string      `json:"markdown"`
	Data     payloadNode `json:"data"`
	Result   payloadNode `json:"result"`
}

type payloadNode struct {
	Text     string `json:"text"`
	FullText string `json:"full_text"`
	Content  string `json:"content"`
	Markdown string `json:"markdown"`
}

func (p payload) ExtractedText() string {
	return firstNonEmpty(
		p.Text,
		p.FullText,
		p.Content,
		p.Markdown,
		p.Data.Text,
		p.Data.FullText,
		p.Data.Content,
		p.Data.Markdown,
		p.Result.Text,
		p.Result.FullText,
		p.Result.Content,
		p.Result.Markdown,
	)
}
