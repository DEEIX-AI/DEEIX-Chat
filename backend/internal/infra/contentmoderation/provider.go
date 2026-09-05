package contentmoderation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	cmport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/contentmoderation"
)

const (
	defaultModerationPath = "/moderations"
	maxTextChunkBytes     = 12 * 1024
	maxImageSourceBytes   = 20 * 1024 * 1024
	maxImageBatchBytes    = 20 * 1024 * 1024
	maxRetryAfter         = 30 * time.Second
	maxResponseBytes      = 4 << 20
)

type moderationInput struct {
	Type     string              `json:"type"`
	Text     string              `json:"text,omitempty"`
	ImageURL *moderationImageURL `json:"image_url,omitempty"`
}

type moderationImageURL struct {
	URL string `json:"url"`
}

type moderationRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

type moderationCategoryResult struct {
	Flagged                   bool                `json:"flagged"`
	Categories                map[string]bool     `json:"categories"`
	CategoryScores            map[string]float64  `json:"category_scores"`
	CategoryAppliedInputTypes map[string][]string `json:"category_applied_input_types"`
}

type moderationResponse struct {
	ID      string                     `json:"id"`
	Model   string                     `json:"model"`
	Results []moderationCategoryResult `json:"results"`
}

func (document moderationResponse) toDomainResponse() *domaincm.ProviderResponse {
	results := make([]domaincm.CategoryResult, 0, len(document.Results))
	for _, result := range document.Results {
		results = append(results, domaincm.CategoryResult{
			Flagged:                   result.Flagged,
			Categories:                result.Categories,
			CategoryScores:            result.CategoryScores,
			CategoryAppliedInputTypes: result.CategoryAppliedInputTypes,
		})
	}
	return &domaincm.ProviderResponse{ID: document.ID, Model: document.Model, Results: results}
}

// ValidateBaseURL validates and normalizes the supported OpenAI-compatible endpoint shape.
func (c *Client) ValidateBaseURL(raw string) error {
	_, err := normalizeBaseURL(raw)
	return err
}

// ModerateText chunks UTF-8 text and submits each chunk within one shared timeout.
func (c *Client) ModerateText(
	ctx context.Context,
	config domaincm.ProviderConfig,
	text string,
	selected []string,
	modality string,
) (*domaincm.ProviderResponse, error) {
	chunks := splitTextChunks(text)
	if len(chunks) == 0 {
		return emptyResponse(), nil
	}
	if modality == "" {
		modality = domaincm.ModalityText
	}
	deadline := providerDeadline(config.Timeout)
	var merged *domaincm.ProviderResponse
	for _, chunk := range chunks {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, cmport.ErrTimeout
		}
		requestCtx, cancel := context.WithTimeout(ctx, remaining)
		response, err := c.moderate(requestCtx, config, buildTextInput(chunk))
		cancel()
		if err != nil {
			return nil, err
		}
		merged = mergeResponses(merged, response)
		if domaincm.EvaluateHit(response, selected, modality).Hit {
			return merged, nil
		}
	}
	return merged, nil
}

// ModerateImages batches encoded image inputs within one shared timeout.
func (c *Client) ModerateImages(
	ctx context.Context,
	config domaincm.ProviderConfig,
	images []domaincm.ProviderImage,
	selected []string,
	modality string,
) (*domaincm.ProviderResponse, error) {
	dataURLs := buildImageDataURLs(images)
	if len(dataURLs) == 0 {
		return emptyResponse(), nil
	}
	if modality == "" {
		modality = domaincm.ModalityImage
	}
	deadline := providerDeadline(config.Timeout)
	var merged *domaincm.ProviderResponse
	for _, batch := range batchImageDataURLs(dataURLs) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, cmport.ErrTimeout
		}
		requestCtx, cancel := context.WithTimeout(ctx, remaining)
		response, err := c.moderate(requestCtx, config, buildImageInputs(batch))
		cancel()
		if err != nil {
			return nil, err
		}
		merged = mergeResponses(merged, response)
		if domaincm.EvaluateHit(response, selected, modality).Hit {
			return merged, nil
		}
	}
	return merged, nil
}

func (c *Client) moderate(ctx context.Context, config domaincm.ProviderConfig, input any) (*domaincm.ProviderResponse, error) {
	endpoint, err := normalizeBaseURL(config.BaseURL)
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = "omni-moderation-latest"
	}
	body, err := json.Marshal(moderationRequest{Model: model, Input: input})
	if err != nil {
		return nil, fmt.Errorf("marshal moderation request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, mapContextError(err)
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build moderation request: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")
		if key := strings.TrimSpace(config.APIKey); key != "" {
			request.Header.Set("Authorization", "Bearer "+key)
		}

		response, err := c.Do(request, config.BaseURL)
		if err != nil {
			lastErr = fmt.Errorf("%w: %w", cmport.ErrNetwork, err)
			if attempt == 0 && shouldRetryNetwork(err) {
				continue
			}
			return nil, lastErr
		}
		payload, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
		_ = response.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("%w: read response", cmport.ErrNetwork)
			if attempt == 0 && shouldRetryNetwork(readErr) {
				continue
			}
			return nil, lastErr
		}

		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			lastErr = mapHTTPStatus(response.StatusCode)
			if attempt == 0 {
				if err := waitForRetry(ctx, parseRetryAfter(response.Header.Get("Retry-After"))); err != nil {
					return nil, err
				}
				continue
			}
			return nil, lastErr
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, mapHTTPStatus(response.StatusCode)
		}

		var document moderationResponse
		if err := json.Unmarshal(payload, &document); err != nil {
			return nil, fmt.Errorf("%w: malformed JSON", cmport.ErrInvalidResponse)
		}
		if len(document.Results) == 0 {
			return nil, fmt.Errorf("%w: empty results", cmport.ErrInvalidResponse)
		}
		for _, result := range document.Results {
			if result.Categories == nil {
				return nil, fmt.Errorf("%w: missing categories", cmport.ErrInvalidResponse)
			}
		}
		return document.toDomainResponse(), nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, cmport.ErrService
}

func normalizeBaseURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", cmport.ErrInvalidBaseURL
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return "", cmport.ErrInvalidBaseURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", cmport.ErrInvalidBaseURL
	}
	path := strings.TrimRight(parsed.Path, "/")
	switch lower := strings.ToLower(path); {
	case strings.HasSuffix(lower, "/moderations"):
	case strings.HasSuffix(lower, "/v1"):
		path += defaultModerationPath
	case path == "" || path == "/":
		path = "/v1" + defaultModerationPath
	default:
		path += "/v1" + defaultModerationPath
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func splitTextChunks(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	raw := []byte(text)
	chunks := make([]string, 0, (len(raw)/maxTextChunkBytes)+1)
	for len(raw) > 0 {
		limit := min(maxTextChunkBytes, len(raw))
		for limit > 0 && !utf8.Valid(raw[:limit]) {
			limit--
		}
		if limit == 0 {
			_, size := utf8.DecodeRune(raw)
			limit = max(size, 1)
		}
		chunks = append(chunks, string(raw[:limit]))
		raw = raw[limit:]
	}
	return chunks
}

func buildTextInput(text string) []moderationInput {
	return []moderationInput{{Type: "text", Text: text}}
}

func buildImageDataURLs(images []domaincm.ProviderImage) []string {
	items := make([]string, 0, len(images))
	for _, image := range images {
		if len(image.Data) == 0 {
			continue
		}
		mimeType := strings.TrimSpace(image.MimeType)
		if mimeType == "" {
			mimeType = "image/png"
		}
		items = append(items, "data:"+mimeType+";base64,"+base64.StdEncoding.EncodeToString(image.Data))
	}
	return items
}

func buildImageInputs(dataURLs []string) []moderationInput {
	items := make([]moderationInput, 0, len(dataURLs))
	for _, raw := range dataURLs {
		if value := strings.TrimSpace(raw); value != "" {
			items = append(items, moderationInput{Type: "image_url", ImageURL: &moderationImageURL{URL: value}})
		}
	}
	return items
}

func batchImageDataURLs(dataURLs []string) [][]string {
	batches := make([][]string, 0)
	current := make([]string, 0)
	currentSize := 0
	for _, item := range dataURLs {
		size := len(item)
		if size > maxImageSourceBytes {
			if len(current) > 0 {
				batches = append(batches, current)
				current = nil
				currentSize = 0
			}
			batches = append(batches, []string{item})
			continue
		}
		if len(current) > 0 && currentSize+size > maxImageBatchBytes {
			batches = append(batches, current)
			current = nil
			currentSize = 0
		}
		current = append(current, item)
		currentSize += size
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

func mergeResponses(base, next *domaincm.ProviderResponse) *domaincm.ProviderResponse {
	if base == nil {
		return next
	}
	if next == nil || len(next.Results) == 0 {
		return base
	}
	base.Results = append(base.Results, next.Results...)
	if strings.TrimSpace(next.Model) != "" {
		base.Model = next.Model
	}
	return base
}

func emptyResponse() *domaincm.ProviderResponse {
	return &domaincm.ProviderResponse{Results: []domaincm.CategoryResult{{
		Categories:                map[string]bool{},
		CategoryScores:            map[string]float64{},
		CategoryAppliedInputTypes: map[string][]string{},
	}}}
}

func providerDeadline(timeout time.Duration) time.Time {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return time.Now().Add(timeout)
}

func mapHTTPStatus(status int) error {
	// Never include provider response bodies: compatible services may echo
	// moderated content or credentials and this error can be persisted.
	if status == http.StatusTooManyRequests {
		return fmt.Errorf("%w: status %d", cmport.ErrRateLimited, status)
	}
	return fmt.Errorf("%w: status %d", cmport.ErrService, status)
}

func mapContextError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return cmport.ErrTimeout
	}
	return fmt.Errorf("%w: %w", cmport.ErrNetwork, err)
}

func shouldRetryNetwork(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return mapContextError(ctx.Err())
	case <-timer.C:
		return nil
	}
}

func parseRetryAfter(raw string) time.Duration {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return min(time.Duration(seconds)*time.Second, maxRetryAfter)
	}
	if parsed, err := http.ParseTime(value); err == nil {
		delay := time.Until(parsed)
		if delay > 0 {
			return min(delay, maxRetryAfter)
		}
	}
	return 0
}
