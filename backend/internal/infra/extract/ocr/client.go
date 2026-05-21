package ocr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/pdfrender"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
)

const (
	errOCREmptyContent      = "ocr_empty_content"
	DefaultTesseractBaseURL = "http://127.0.0.1:8004/ocr"
	DefaultRapidOCRBaseURL  = "http://127.0.0.1:8002/ocr"
	ManagedRapidOCRBaseURL  = "http://deeix-chat-rapidocr:8002/ocr"
	managedRapidOCRHost     = "deeix-chat-rapidocr"
	managedRapidOCRSource   = "managed"
	rapidOCRHealthEndpoint  = "/healthz"
	defaultLLMOCRPrompt     = "You are an OCR engine for document images. Extract all visible text in reading order. Return plain text only. Preserve the original language of the image text. Do not summarize, translate, explain, or add markdown. Preserve line breaks when they help readability."
)

// ClientConfig 表示 OCR 服务接入配置。
type ClientConfig struct {
	BaseURL        string
	AuthToken      string
	Model          string
	TimeoutSeconds int
	Prompt         string
	Env            string
}

// Request 表示一次 PDF OCR 请求。
type Request struct {
	AbsolutePath string
	FileName     string
	MimeType     string
	PageRanges   []PageRange
}

// Response 表示 OCR 返回结果。
type Response struct {
	Text          string
	RenderedPages int
	Pages         []PageText
}

// PageRange 表示 OCR 需要处理的连续页区间。
type PageRange struct {
	Start int
	End   int
}

// PageText 表示单页 OCR 结果。
type PageText struct {
	PageNumber int
	Text       string
}

// Client 封装 PDF OCR 回退能力。
type Client struct {
	baseURL        string
	authToken      string
	model          string
	prompt         string
	timeoutSeconds int
	httpClient     *http.Client
	llmClient      *llm.Client
	pdfRenderer    *pdfrender.Renderer
}

// NewRapidOCR 创建 RapidOCR client。
func NewRapidOCR(cfg ClientConfig) *Client {
	return newUploadTextClient(cfg)
}

// NewTesseract 创建 Tesseract OCR client。
func NewTesseract(cfg ClientConfig) *Client {
	return newUploadTextClient(cfg)
}

// NewPaddle 创建 Paddle OCR client。
func NewPaddle(cfg ClientConfig) *Client {
	return newUploadTextClient(cfg)
}

// NewLLM 创建当前 LLM OCR client。
func NewLLM(cfg ClientConfig) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil
	}
	return &Client{
		baseURL:        baseURL,
		authToken:      strings.TrimSpace(cfg.AuthToken),
		model:          strings.TrimSpace(cfg.Model),
		prompt:         strings.TrimSpace(cfg.Prompt),
		timeoutSeconds: cfg.TimeoutSeconds,
		llmClient:      llm.NewClientWithEnv(cfg.Env),
		pdfRenderer:    pdfrender.New(),
	}
}

func newUploadTextClient(cfg ClientConfig) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil
	}
	return &Client{
		baseURL:        baseURL,
		authToken:      strings.TrimSpace(cfg.AuthToken),
		model:          strings.TrimSpace(cfg.Model),
		prompt:         strings.TrimSpace(cfg.Prompt),
		timeoutSeconds: cfg.TimeoutSeconds,
		httpClient:     platformtracing.NewHTTPClient(resolveHTTPTimeout(cfg.TimeoutSeconds, 60*time.Second)),
	}
}

// ResolveRapidOCRBaseURL 为 RapidOCR 解析最终访问地址。
func ResolveRapidOCRBaseURL(cfg config.Config) string {
	if strings.EqualFold(strings.TrimSpace(cfg.ExtractRapidOCRSource), managedRapidOCRSource) {
		return ResolveManagedRapidOCRBaseURL(context.Background())
	}
	return strings.TrimSpace(cfg.ExtractRapidOCRBaseURL)
}

// ResolveManagedRapidOCRBaseURL 优先返回容器网络内地址，不可达时回退宿主机地址。
func ResolveManagedRapidOCRBaseURL(ctx context.Context) string {
	if ok, _ := ProbeRapidOCREndpoint(ctx, ManagedRapidOCRBaseURL, ""); ok {
		return ManagedRapidOCRBaseURL
	}
	if ok, _ := ProbeRapidOCREndpoint(ctx, DefaultRapidOCRBaseURL, ""); ok {
		return DefaultRapidOCRBaseURL
	}
	if managedRapidOCRHostResolvable(ctx) {
		return ManagedRapidOCRBaseURL
	}
	return DefaultRapidOCRBaseURL
}

func managedRapidOCRHostResolvable(ctx context.Context) bool {
	lookupCtx := ctx
	if lookupCtx == nil {
		lookupCtx = context.Background()
	}
	lookupCtx, cancel := context.WithTimeout(lookupCtx, 300*time.Millisecond)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(lookupCtx, managedRapidOCRHost)
	return err == nil && len(addrs) > 0
}

func ProbeRapidOCREndpoint(ctx context.Context, baseURL string, authToken string) (bool, string) {
	return ProbeOCREndpoint(ctx, baseURL, authToken)
}

func ProbeOCREndpoint(ctx context.Context, baseURL string, authToken string) (bool, string) {
	return probeOCREndpoint(ctx, baseURL, authToken, platformtracing.NewHTTPClient(800*time.Millisecond))
}

func probeOCREndpoint(ctx context.Context, baseURL string, authToken string, httpClient *http.Client) (bool, string) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return false, "服务地址为空。"
	}
	healthURL := resolveOCRHealthURL(baseURL)

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
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, ""
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return false, "ocr_unauthorized"
	}
	if resp.StatusCode == http.StatusForbidden {
		return false, "ocr_forbidden"
	}
	return false, fmt.Sprintf("服务响应异常: %d", resp.StatusCode)
}

func resolveOCRHealthURL(baseURL string) string {
	if strings.HasSuffix(baseURL, "/ocr") {
		return strings.TrimSuffix(baseURL, "/ocr") + rapidOCRHealthEndpoint
	}
	return baseURL + rapidOCRHealthEndpoint
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

// ExtractText 对文档或图片做 OCR，返回识别文本。
func (c *Client) ExtractText(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.AbsolutePath) == "" {
		return Response{}, fmt.Errorf("ocr_invalid_file_path")
	}
	if c == nil || c.baseURL == "" {
		return Response{}, fmt.Errorf("ocr_unavailable")
	}
	if c.llmClient != nil {
		return c.extractTextWithLLM(ctx, req)
	}
	return c.extractTextRemote(ctx, req)
}

func (c *Client) extractTextWithLLM(ctx context.Context, req Request) (Response, error) {
	if c == nil || c.llmClient == nil || strings.TrimSpace(c.baseURL) == "" {
		return Response{}, fmt.Errorf("ocr_unavailable")
	}
	if strings.TrimSpace(c.model) == "" {
		return Response{}, fmt.Errorf("ocr_unprocessable: missing model")
	}
	if isImageRequest(req) {
		imageData, err := os.ReadFile(strings.TrimSpace(req.AbsolutePath))
		if err != nil {
			return Response{}, err
		}
		text, err := c.extractImageTextWithLLM(ctx, imageData, strings.TrimSpace(req.MimeType), "Perform image OCR strictly. Return only the recognized body text. Preserve the original language of the image text. Do not explain, summarize, or add Markdown.")
		if err != nil {
			return Response{}, err
		}
		if text == "" {
			return Response{}, fmt.Errorf(errOCREmptyContent)
		}
		return Response{Text: text}, nil
	}

	pageNumbers := resolveOCRPageNumbers(req.PageRanges)
	if len(pageNumbers) == 0 {
		return Response{}, fmt.Errorf("ocr_unprocessable: no target pages")
	}

	pageTexts := make([]PageText, 0, len(pageNumbers))
	var pageErrors []string
	foundAnyImage := false
	tempDir, err := os.MkdirTemp("", "deeix-chat-llm-ocr-*")
	if err != nil {
		return Response{}, err
	}
	defer os.RemoveAll(tempDir)

	for _, pageNumber := range pageNumbers {
		imageData, renderErr := c.renderPageForLLM(ctx, req.AbsolutePath, pageNumber, tempDir)
		if renderErr != nil {
			pageErrors = append(pageErrors, fmt.Sprintf("page %d render_failed", pageNumber))
			continue
		}
		foundAnyImage = true

		text, callErr := c.extractImageTextWithLLM(ctx, imageData, "image/jpeg", fmt.Sprintf("This image is page %d of a PDF. Perform OCR strictly. Return only the recognized body text. Preserve the original language of the image text. Do not explain, summarize, or add Markdown.", pageNumber))
		if callErr != nil {
			pageErrors = append(pageErrors, fmt.Sprintf("page %d %s", pageNumber, strings.TrimSpace(callErr.Error())))
			continue
		}
		if text == "" {
			continue
		}
		pageTexts = append(pageTexts, PageText{
			PageNumber: pageNumber,
			Text:       text,
		})
	}

	if len(pageTexts) == 0 {
		if !foundAnyImage {
			return Response{}, fmt.Errorf("ocr_unprocessable: no page images found")
		}
		if len(pageErrors) > 0 {
			return Response{}, fmt.Errorf("ocr_failed: %s", strings.Join(pageErrors, "; "))
		}
		return Response{}, fmt.Errorf(errOCREmptyContent)
	}

	parts := make([]string, 0, len(pageTexts))
	for _, page := range pageTexts {
		if value := normalizeOCRText(page.Text); value != "" {
			parts = append(parts, value)
		}
	}
	return Response{
		Text:          strings.Join(parts, "\n\n"),
		RenderedPages: len(pageTexts),
		Pages:         pageTexts,
	}, nil
}

func (c *Client) extractImageTextWithLLM(ctx context.Context, imageData []byte, mimeType string, instruction string) (string, error) {
	prompt := strings.TrimSpace(c.prompt)
	if prompt == "" {
		prompt = defaultLLMOCRPrompt
	}
	mimeType = strings.TrimSpace(mimeType)
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		instruction = "Perform OCR strictly. Return only the recognized body text. Preserve the original language of the image text. Do not explain, summarize, or add Markdown."
	}

	parts := make([]llm.ContentPart, 0, 2)
	parts = append(parts, llm.ContentPart{
		Kind: llm.ContentPartText,
		Text: instruction,
	})
	parts = append(parts, llm.ContentPart{
		Kind:     llm.ContentPartImage,
		MimeType: mimeType,
		Data:     imageData,
	})

	output, err := c.llmClient.Generate(ctx, llm.RouteConfig{
		Protocol:         llm.AdapterOpenAIChatCompletions,
		BaseURL:          c.baseURL,
		APIKey:           c.authToken,
		ReadTimeoutMS:    max(c.timeoutSeconds, 60) * 1000,
		ConnectTimeoutMS: 10000,
		Endpoint:         llm.EndpointChatCompletions,
		UpstreamModel:    c.model,
	}, llm.GenerateInput{
		Messages: []llm.Message{
			{Role: "system", Content: prompt},
			{Role: "user", Parts: parts},
		},
	})
	if err != nil {
		return "", mapLLMOCRError(err)
	}
	return normalizeOCRText(output.Text), nil
}

func isImageRequest(req Request) bool {
	mimeType := strings.ToLower(strings.TrimSpace(req.MimeType))
	if strings.HasPrefix(mimeType, "image/") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(req.FileName)))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

func (c *Client) extractTextRemote(ctx context.Context, req Request) (Response, error) {
	if c == nil || strings.TrimSpace(c.baseURL) == "" {
		return Response{}, fmt.Errorf("ocr_unavailable")
	}
	file, err := os.Open(strings.TrimSpace(req.AbsolutePath))
	if err != nil {
		return Response{}, err
	}
	fileName := strings.TrimSpace(req.FileName)
	if fileName == "" {
		fileName = filepath.Base(strings.TrimSpace(req.AbsolutePath))
	}

	bodyReader, contentType, writeErrCh := buildMultipartOCRBody(file, fileName, strings.TrimSpace(req.MimeType), encodePageRanges(req.PageRanges), strings.TrimSpace(c.model), strings.TrimSpace(c.prompt))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bodyReader)
	if err != nil {
		_ = bodyReader.Close()
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", contentType)
	httpReq.Header.Set("Accept", "application/json, text/plain")
	applyAuthHeaders(httpReq, c.authToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		_ = bodyReader.Close()
		if writeErr := awaitMultipartWriteError(writeErrCh); writeErr != nil {
			return Response{}, writeErr
		}
		return Response{}, err
	}
	defer resp.Body.Close()
	if writeErr := awaitMultipartWriteError(writeErrCh); writeErr != nil {
		return Response{}, writeErr
	}

	if resp.StatusCode == http.StatusNoContent {
		return Response{}, fmt.Errorf(errOCREmptyContent)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		detail := strings.TrimSpace(string(bodyBytes))
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return Response{}, fmt.Errorf("ocr_unauthorized")
		case http.StatusForbidden:
			return Response{}, fmt.Errorf("ocr_forbidden")
		case http.StatusUnprocessableEntity:
			if detail == "" {
				return Response{}, fmt.Errorf("ocr_unprocessable")
			}
			return Response{}, fmt.Errorf("ocr_unprocessable: %s", detail)
		default:
			if detail == "" {
				return Response{}, fmt.Errorf("ocr_http_%d", resp.StatusCode)
			}
			return Response{}, fmt.Errorf("ocr_http_%d: %s", resp.StatusCode, detail)
		}
	}

	limitedBody := io.LimitReader(resp.Body, 50*1024*1024)
	return parseRemoteOCRResponse(limitedBody, resp.Header.Get("Content-Type"))
}

func normalizeOCRText(raw string) string {
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

func parseRemoteOCRResponse(body io.Reader, contentType string) (Response, error) {
	if strings.Contains(strings.ToLower(contentType), "application/json") {
		var parsed traditionalOCRPayload
		decoder := json.NewDecoder(body)
		if err := decoder.Decode(&parsed); err == nil {
			text := normalizeOCRText(parsed.ExtractedText())
			renderedPages := parsed.PageCount()
			pageTexts := parsed.ExtractedPageTexts()
			if text == "" && len(pageTexts) > 0 {
				pageParts := make([]string, 0, len(pageTexts))
				for _, page := range pageTexts {
					if value := normalizeOCRText(page.Text); value != "" {
						pageParts = append(pageParts, value)
					}
				}
				text = strings.Join(pageParts, "\n\n")
			}
			if text == "" {
				return Response{}, fmt.Errorf(errOCREmptyContent)
			}
			return Response{
				Text:          text,
				RenderedPages: renderedPages,
				Pages:         pageTexts,
			}, nil
		}
	}
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return Response{}, err
	}
	textBody := strings.TrimSpace(string(bodyBytes))
	text := normalizeOCRText(textBody)
	if text == "" {
		return Response{}, fmt.Errorf(errOCREmptyContent)
	}
	return Response{Text: text}, nil
}

func buildMultipartOCRBody(file *os.File, fileName string, mimeType string, pageRanges string, model string, prompt string) (io.ReadCloser, string, <-chan error) {
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
		if pageRanges != "" {
			if err = writer.WriteField("page_ranges", pageRanges); err != nil {
				fail(err)
				return
			}
		}
		if model != "" {
			if err = writer.WriteField("model", model); err != nil {
				fail(err)
				return
			}
		}
		if prompt != "" {
			if err = writer.WriteField("prompt", prompt); err != nil {
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

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func resolveOCRPageNumbers(ranges []PageRange) []int {
	result := make([]int, 0, len(ranges))
	seen := make(map[int]struct{}, len(ranges))
	for _, pageRange := range ranges {
		start := pageRange.Start
		end := pageRange.End
		if start <= 0 {
			continue
		}
		if end < start {
			end = start
		}
		for pageNumber := start; pageNumber <= end; pageNumber++ {
			if _, ok := seen[pageNumber]; ok {
				continue
			}
			seen[pageNumber] = struct{}{}
			result = append(result, pageNumber)
		}
	}
	return result
}

func (c *Client) renderPageForLLM(ctx context.Context, pdfPath string, pageNumber int, tempDir string) ([]byte, error) {
	if c == nil || c.pdfRenderer == nil {
		return nil, fmt.Errorf("pdf_page_renderer_unavailable")
	}
	return c.pdfRenderer.RenderPageJPEG(ctx, pdfrender.Request{
		SourcePath: pdfPath,
		PageNumber: pageNumber,
		TempDir:    tempDir,
	})
}

func mapLLMOCRError(err error) error {
	if err == nil {
		return nil
	}
	var upstreamErr *llm.UpstreamError
	if errors.As(err, &upstreamErr) {
		switch upstreamErr.StatusCode {
		case http.StatusUnauthorized:
			return fmt.Errorf("ocr_unauthorized")
		case http.StatusForbidden:
			return fmt.Errorf("ocr_forbidden")
		case http.StatusUnprocessableEntity:
			if strings.TrimSpace(upstreamErr.Message) == "" {
				return fmt.Errorf("ocr_unprocessable")
			}
			return fmt.Errorf("ocr_unprocessable: %s", strings.TrimSpace(upstreamErr.Message))
		default:
			if strings.TrimSpace(upstreamErr.Message) == "" {
				return fmt.Errorf("ocr_http_%d", upstreamErr.StatusCode)
			}
			return fmt.Errorf("ocr_http_%d: %s", upstreamErr.StatusCode, strings.TrimSpace(upstreamErr.Message))
		}
	}
	return fmt.Errorf("ocr_unavailable: %w", err)
}

func encodePageRanges(ranges []PageRange) string {
	if len(ranges) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ranges))
	for _, pageRange := range ranges {
		start := pageRange.Start
		end := pageRange.End
		if start <= 0 {
			continue
		}
		if end < start {
			end = start
		}
		if start == end {
			parts = append(parts, fmt.Sprintf("%d", start))
			continue
		}
		parts = append(parts, fmt.Sprintf("%d-%d", start, end))
	}
	return strings.Join(parts, ",")
}

type traditionalOCRPayload struct {
	Text          string                    `json:"text"`
	FullText      string                    `json:"full_text"`
	Content       string                    `json:"content"`
	Markdown      string                    `json:"markdown"`
	RenderedPages int                       `json:"rendered_pages"`
	Pages         traditionalOCRPagesValue  `json:"pages"`
	PageItems     []traditionalOCRPage      `json:"page_items"`
	PageResults   []traditionalOCRPage      `json:"page_results"`
	PageList      []traditionalOCRPage      `json:"page_list"`
	PageData      []traditionalOCRPage      `json:"page_data"`
	PageBlocks    []traditionalOCRPage      `json:"pages_data"`
	Results       []traditionalOCRPage      `json:"results"`
	Data          traditionalOCRPayloadNode `json:"data"`
	Result        traditionalOCRPayloadNode `json:"result"`
}

type traditionalOCRPayloadNode struct {
	Text          string                   `json:"text"`
	FullText      string                   `json:"full_text"`
	Content       string                   `json:"content"`
	Markdown      string                   `json:"markdown"`
	RenderedPages int                      `json:"rendered_pages"`
	Pages         traditionalOCRPagesValue `json:"pages"`
	PageItems     []traditionalOCRPage     `json:"page_items"`
	PageResults   []traditionalOCRPage     `json:"page_results"`
	PageList      []traditionalOCRPage     `json:"page_list"`
	PageData      []traditionalOCRPage     `json:"page_data"`
	PageBlocks    []traditionalOCRPage     `json:"pages_data"`
	Results       []traditionalOCRPage     `json:"results"`
}

type traditionalOCRPage struct {
	Page       int    `json:"page"`
	PageNumber int    `json:"page_number"`
	Index      int    `json:"index"`
	Text       string `json:"text"`
	Content    string `json:"content"`
	Markdown   string `json:"markdown"`
}

type traditionalOCRPagesValue struct {
	Count int
	Items []traditionalOCRPage
}

func (v *traditionalOCRPagesValue) UnmarshalJSON(data []byte) error {
	var count int
	if err := json.Unmarshal(data, &count); err == nil {
		v.Count = count
		v.Items = nil
		return nil
	}

	var items []traditionalOCRPage
	if err := json.Unmarshal(data, &items); err == nil {
		v.Count = len(items)
		v.Items = items
		return nil
	}

	return nil
}

func (p traditionalOCRPayload) ExtractedText() string {
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
		joinTraditionalOCRPages(
			p.PageItems,
			p.PageResults,
			p.PageList,
			p.PageData,
			p.PageBlocks,
			p.Results,
			p.Pages.Items,
			p.Data.PageItems,
			p.Data.PageResults,
			p.Data.PageList,
			p.Data.PageData,
			p.Data.PageBlocks,
			p.Data.Results,
			p.Data.Pages.Items,
			p.Result.PageItems,
			p.Result.PageResults,
			p.Result.PageList,
			p.Result.PageData,
			p.Result.PageBlocks,
			p.Result.Results,
			p.Result.Pages.Items,
		),
	)
}

func (p traditionalOCRPayload) PageCount() int {
	count := firstPositive(
		p.RenderedPages,
		p.Pages.Count,
		p.Data.RenderedPages,
		p.Data.Pages.Count,
		p.Result.RenderedPages,
		p.Result.Pages.Count,
	)
	if count > 0 {
		return count
	}
	return firstPositive(
		len(p.PageItems),
		len(p.PageResults),
		len(p.PageList),
		len(p.PageData),
		len(p.PageBlocks),
		len(p.Results),
		len(p.Pages.Items),
		len(p.Data.PageItems),
		len(p.Data.PageResults),
		len(p.Data.PageList),
		len(p.Data.PageData),
		len(p.Data.PageBlocks),
		len(p.Data.Results),
		len(p.Data.Pages.Items),
		len(p.Result.PageItems),
		len(p.Result.PageResults),
		len(p.Result.PageList),
		len(p.Result.PageData),
		len(p.Result.PageBlocks),
		len(p.Result.Results),
		len(p.Result.Pages.Items),
	)
}

func (p traditionalOCRPayload) ExtractedPageTexts() []PageText {
	return firstNonEmptyPageTexts(
		convertTraditionalOCRPages(p.Pages.Items),
		convertTraditionalOCRPages(p.PageItems),
		convertTraditionalOCRPages(p.PageResults),
		convertTraditionalOCRPages(p.PageList),
		convertTraditionalOCRPages(p.PageData),
		convertTraditionalOCRPages(p.PageBlocks),
		convertTraditionalOCRPages(p.Results),
		convertTraditionalOCRPages(p.Data.Pages.Items),
		convertTraditionalOCRPages(p.Data.PageItems),
		convertTraditionalOCRPages(p.Data.PageResults),
		convertTraditionalOCRPages(p.Data.PageList),
		convertTraditionalOCRPages(p.Data.PageData),
		convertTraditionalOCRPages(p.Data.PageBlocks),
		convertTraditionalOCRPages(p.Data.Results),
		convertTraditionalOCRPages(p.Result.Pages.Items),
		convertTraditionalOCRPages(p.Result.PageItems),
		convertTraditionalOCRPages(p.Result.PageResults),
		convertTraditionalOCRPages(p.Result.PageList),
		convertTraditionalOCRPages(p.Result.PageData),
		convertTraditionalOCRPages(p.Result.PageBlocks),
		convertTraditionalOCRPages(p.Result.Results),
	)
}

func joinTraditionalOCRPages(groups ...[]traditionalOCRPage) string {
	parts := make([]string, 0)
	for _, group := range groups {
		for _, page := range group {
			value := normalizeOCRText(firstNonEmpty(page.Text, page.Content, page.Markdown))
			if value == "" {
				continue
			}
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "\n\n")
}

func convertTraditionalOCRPages(pages []traditionalOCRPage) []PageText {
	result := make([]PageText, 0, len(pages))
	for idx, page := range pages {
		text := normalizeOCRText(firstNonEmpty(page.Text, page.Content, page.Markdown))
		if text == "" {
			continue
		}
		pageNumber := firstPositive(page.PageNumber, page.Page, page.Index)
		if pageNumber <= 0 {
			pageNumber = idx + 1
		}
		result = append(result, PageText{
			PageNumber: pageNumber,
			Text:       text,
		})
	}
	return result
}

func firstNonEmptyPageTexts(groups ...[]PageText) []PageText {
	for _, group := range groups {
		if len(group) > 0 {
			return group
		}
	}
	return nil
}
