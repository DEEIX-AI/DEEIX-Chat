package conversation

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
)

const MessageErrorCodeMediaImageStreamUnsupported = "media.image_stream_unsupported"

func normalizePublicID(raw string) string {
	return conv.NormalizePublicID(raw)
}

// isCJKRune 判断字符是否属于 CJK 字符范围（中文、日文、韩文）。
func isCJKRune(r rune) bool {
	return (r >= 0x2E80 && r <= 0x9FFF) || // CJK 部首、假名、统一表意文字
		(r >= 0xAC00 && r <= 0xD7AF) || // 韩文音节
		(r >= 0xF900 && r <= 0xFAFF) || // CJK 兼容汉字
		(r >= 0x20000 && r <= 0x2A6DF) // CJK 扩展 B
}

// estimateTokens 估算文本 token 数，区分 CJK 与其他字符权重。
// CJK 字符：约 1.5 chars/token；ASCII 及其他：约 4 chars/token。
func estimateTokens(content string) int64 {
	if len(content) == 0 {
		return 0
	}
	var cjk, other int64
	for _, r := range content {
		if isCJKRune(r) {
			cjk++
		} else {
			other++
		}
	}
	// CJK: tokens = ceil(cjk * 2/3)；other: tokens = ceil(other / 4)
	tokens := (cjk*2+2)/3 + (other+3)/4
	if tokens == 0 {
		return 1
	}
	return tokens
}

func estimateContentPartTokens(part llm.ContentPart) int64 {
	switch part.Kind {
	case llm.ContentPartImage:
		return 255
	case llm.ContentPartFile:
		return estimateTokens(part.FileName) + estimateTokens(part.Text) + 8
	default:
		return estimateTokens(part.Text)
	}
}

func estimateMessageTokens(message llm.Message) int64 {
	var tokens int64 = 4
	if message.Role != "" {
		tokens += 1
	}
	if len(message.Parts) > 0 {
		for _, part := range message.Parts {
			tokens += estimateContentPartTokens(part)
		}
		return tokens
	}
	return tokens + estimateTokens(message.Content)
}

func estimatePromptTokens(messages []llm.Message) int64 {
	var tokens int64 = 2
	for _, message := range messages {
		tokens += estimateMessageTokens(message)
	}
	if tokens < 0 {
		return 0
	}
	return tokens
}

func compactSnippet(content string, maxLen int) string {
	value := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if value == "" {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 120
	}
	runes := []rune(value)
	if len(runes) <= maxLen {
		return value
	}
	return string(runes[:maxLen]) + "..."
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if normalized := strings.TrimSpace(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

func buildContextPolicyJSON(cfg config.Config) string {
	policy := map[string]interface{}{
		"max_turns":                     cfg.ContextMaxTurns,
		"max_input_tokens":              cfg.ContextMaxInputTokens,
		"compact_enabled":               cfg.ContextCompactEnabled,
		"compact_trigger_tokens":        cfg.ContextCompactTrigger,
		"compact_preserve_recent_turns": cfg.ContextCompactPreserve,
	}
	raw, err := json.Marshal(policy)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func truncateError(message string, limit int) string {
	value := strings.TrimSpace(message)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func getStringFromAny(raw interface{}) string {
	return conv.GetStringFromAny(raw)
}

func getIntFromAny(raw interface{}) int {
	return conv.GetIntFromAny(raw)
}

func inferProvider(platformModelName string) string {
	code := strings.ToLower(strings.TrimSpace(platformModelName))
	switch {
	case strings.HasPrefix(code, "gpt-"):
		return "openai"
	case strings.HasPrefix(code, "claude-"):
		return "anthropic"
	default:
		return "internal"
	}
}

func classifyRunErrorCode(err error) string {
	var upstreamErr *llm.UpstreamError
	if errors.As(err, &upstreamErr) && isImageStreamConfigurationFailure(upstreamErr) {
		return MessageErrorCodeMediaImageStreamUnsupported
	}
	switch {
	case errors.Is(err, ErrConversationNotFound):
		return "conversation_not_found"
	case errors.Is(err, ErrInvalidFileReference):
		return "invalid_file_reference"
	case errors.Is(err, ErrFileNotFound):
		return "file_not_found"
	case errors.Is(err, ErrStorageQuotaExceeded):
		return "storage_quota_exceeded"
	case errors.Is(err, ErrFileTooLarge):
		return "file_too_large"
	case errors.Is(err, ErrModelRouteNotConfigured):
		return "model_route_not_configured"
	case errors.Is(err, ErrUpstreamEmptyResponse):
		return "upstream_empty_response"
	case errors.Is(err, ErrToolRunFinalAnswerMissing):
		return "tool_run_final_answer_missing"
	case errors.Is(err, ErrMessageGenerationCanceled):
		return "generation_canceled"
	case errors.Is(err, ErrMediaImagePromptRequired):
		return "media_image_prompt_required"
	case errors.Is(err, ErrMediaImageGenerationRejectsInputs):
		return "media_image_generation_rejects_inputs"
	case errors.Is(err, ErrMediaImageEditInputRequired):
		return "media_image_edit_input_required"
	case errors.Is(err, ErrMediaImageEditTooManyInputs):
		return "media_image_edit_too_many_inputs"
	case errors.Is(err, ErrMediaImageEditInputInvalid):
		return "media_image_edit_input_invalid"
	case errors.Is(err, ErrMediaVideoPromptRequired):
		return "media_video_prompt_required"
	case errors.Is(err, ErrMediaVideoInputInvalid):
		return "media_video_input_invalid"
	case errors.Is(err, ErrMediaVideoTooManyInputs):
		return "media_video_too_many_inputs"
	case errors.Is(err, ErrMediaRouteProtocolMismatch):
		return "media_route_protocol_mismatch"
	case errors.Is(err, ErrUpstreamRequestFailed):
		return "upstream_request_failed"
	default:
		return "internal_error"
	}
}

func messageErrorSummary(err error) string {
	if err == nil {
		return ""
	}
	var upstreamErr *llm.UpstreamError
	if errors.As(err, &upstreamErr) {
		return upstreamErrorSummary(upstreamErr)
	}
	value := strings.TrimSpace(err.Error())
	if value == "" {
		return ""
	}
	prefix := ErrUpstreamRequestFailed.Error() + ":"
	for strings.HasPrefix(value, prefix) {
		value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
	}
	return value
}

func isMessageGenerationCanceledError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrMessageGenerationCanceled) {
		return true
	}
	var upstreamErr *llm.UpstreamError
	if !errors.As(err, &upstreamErr) || !isSuccessfulUpstreamStatus(upstreamErr.StatusCode) {
		return false
	}
	return strings.TrimSpace(upstreamErr.Message) == ErrMessageGenerationCanceled.Error()
}

func messageErrorDebug(err error) *llm.UpstreamDebugSnapshot {
	if err == nil {
		return nil
	}
	var upstreamErr *llm.UpstreamError
	if errors.As(err, &upstreamErr) {
		return sanitizeUpstreamDebugSnapshot(upstreamErr.Debug)
	}
	return nil
}

func sanitizeUpstreamDebugSnapshot(debug *llm.UpstreamDebugSnapshot) *llm.UpstreamDebugSnapshot {
	if debug == nil {
		return nil
	}
	return &llm.UpstreamDebugSnapshot{
		Request: llm.UpstreamDebugRequest{
			Method: debug.Request.Method,
			Path:   debug.Request.Path,
			Body:   sanitizeUpstreamNameJSON(debug.Request.Body),
		},
		Response: llm.UpstreamDebugResponse{
			StatusCode: debug.Response.StatusCode,
			Body:       sanitizeUpstreamNameJSON(debug.Response.Body),
		},
	}
}

func sanitizeUpstreamNameJSON(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return raw
	}
	var payload interface{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return raw
	}
	deleteUpstreamNameValues(payload, "")
	data, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return string(data)
}

func deleteUpstreamNameValues(value interface{}, parentKey string) {
	switch current := value.(type) {
	case map[string]interface{}:
		for key, child := range current {
			if isUpstreamNameKey(key, parentKey) {
				delete(current, key)
				continue
			}
			deleteUpstreamNameValues(child, key)
		}
	case []interface{}:
		for _, child := range current {
			deleteUpstreamNameValues(child, parentKey)
		}
	}
}

func isUpstreamNameKey(key string, parentKey string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", ""))
	if normalized == "upstreamname" {
		return true
	}
	return strings.ToLower(strings.TrimSpace(parentKey)) == "upstream" && (normalized == "name" || normalized == "displayname")
}

func upstreamErrorSummary(err *llm.UpstreamError) string {
	if err == nil {
		return ""
	}
	lines := make([]string, 0, 3)
	hint := imageStreamConfigurationHint(err)
	if isSuccessfulUpstreamStatus(err.StatusCode) {
		lines = append(lines, fmt.Sprintf("模型响应格式不兼容（HTTP %d）", err.StatusCode))
		lines = append(lines, "错误：上游返回成功状态码，但响应格式与当前协议不兼容")
		if hint != "" {
			lines = append(lines, hint)
		}
		return strings.Join(lines, "\n")
	}
	if err.StatusCode > 0 {
		lines = append(lines, fmt.Sprintf("模型请求失败（HTTP %d）", err.StatusCode))
	} else {
		lines = append(lines, "模型请求失败")
	}
	if message := normalizeUpstreamErrorMessage(err.Message); message != "" {
		lines = append(lines, "错误："+message)
	}
	if hint != "" {
		lines = append(lines, hint)
	}
	return strings.Join(lines, "\n")
}

func isSuccessfulUpstreamStatus(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

func normalizeUpstreamErrorMessage(message string) string {
	value := strings.TrimSpace(message)
	if value == "" || looksLikeRawSSEBody(value) {
		return ""
	}
	return value
}

func looksLikeRawSSEBody(value string) bool {
	normalized := strings.TrimSpace(value)
	return strings.HasPrefix(normalized, "data:") ||
		strings.Contains(normalized, "\ndata:") ||
		strings.Contains(normalized, " data:")
}

func imageStreamConfigurationHint(err *llm.UpstreamError) string {
	if !isImageStreamConfigurationFailure(err) {
		return ""
	}
	return "Tips：当前上游可能不支持流式响应，请管理员在模型能力中关闭“图像流式调用”（设置 image.stream=false）后重试。"
}

func isImageStreamConfigurationFailure(err *llm.UpstreamError) bool {
	return err != nil && isImageStreamingUpstreamRequest(err.Debug) && isStreamingResponseFormatFailure(err)
}

func isImageStreamingUpstreamRequest(debug *llm.UpstreamDebugSnapshot) bool {
	if debug == nil {
		return false
	}
	path := strings.ToLower(strings.TrimSpace(debug.Request.Path))
	body := strings.TrimSpace(debug.Request.Body)
	if strings.Contains(path, "/images/") {
		return jsonObjectFieldIsTrue(body, "stream")
	}
	if strings.Contains(path, ":streamgeneratecontent") {
		return geminiImageResponseRequested(body)
	}
	return false
}

func jsonObjectFieldIsTrue(raw string, key string) bool {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil {
		return false
	}
	switch value := payload[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}

func geminiImageResponseRequested(raw string) bool {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil {
		return false
	}
	config, ok := payload["generationConfig"].(map[string]interface{})
	if !ok {
		return false
	}
	return containsImageModality(config["responseModalities"])
}

func containsImageModality(raw interface{}) bool {
	switch value := raw.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "image")
	case []interface{}:
		for _, item := range value {
			if containsImageModality(item) {
				return true
			}
		}
	case []string:
		for _, item := range value {
			if containsImageModality(item) {
				return true
			}
		}
	}
	return false
}

func isStreamingResponseFormatFailure(err *llm.UpstreamError) bool {
	detail := strings.ToLower(strings.TrimSpace(err.Message + "\n" + err.Body))
	if err.Debug != nil {
		detail = strings.TrimSpace(detail + "\n" + strings.ToLower(err.Debug.Response.Body))
	}
	if detail == "" {
		return false
	}
	if strings.Contains(detail, "invalid character") && strings.Contains(detail, "looking for beginning of value") {
		return true
	}
	return isStreamUnsupportedError(err)
}

func wrapUpstreamRequestError(cause error) error {
	if cause == nil {
		return ErrUpstreamRequestFailed
	}
	return fmt.Errorf("%w: %w", ErrUpstreamRequestFailed, cause)
}

// MessageErrorSummary 返回适合边界层展示的错误摘要。
func MessageErrorSummary(err error) string {
	return messageErrorSummary(err)
}

// MessageErrorCode 返回适合边界层和前端本地化使用的稳定错误码。
func MessageErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var upstreamErr *llm.UpstreamError
	if errors.As(err, &upstreamErr) && isImageStreamConfigurationFailure(upstreamErr) {
		return MessageErrorCodeMediaImageStreamUnsupported
	}
	return ""
}

// MessageErrorDebug 返回脱敏后的上游请求/响应快照，用于排查兼容性问题。
func MessageErrorDebug(err error) *llm.UpstreamDebugSnapshot {
	return messageErrorDebug(err)
}

func normalizeAttachmentKind(kind string, mimeType string) string {
	value := strings.TrimSpace(kind)
	if value != "" {
		return value
	}
	return inferAttachmentKind(mimeType)
}

// NormalizeAttachmentKind 规范化附件类型，供边界层复用。
func NormalizeAttachmentKind(kind string, mimeType string) string {
	return normalizeAttachmentKind(kind, mimeType)
}

func normalizeToolType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "function", "function_call", "tool_call":
		return "function"
	case "mcp", "mcp_call":
		return "mcp"
	case "":
		return "function"
	default:
		return value
	}
}

func inferAttachmentKind(mimeType string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "image/") {
		return "image"
	}
	return "file"
}

func normalizeBranchReason(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "retry", "edit":
		return value
	default:
		return "default"
	}
}

func normalizeMessageFeedback(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "up":
		return "up"
	case "down":
		return "down"
	default:
		return ""
	}
}

func fallbackContentType(contentType string) string {
	value := strings.TrimSpace(contentType)
	if value == "" {
		return "text"
	}
	return value
}

func appendAssistantText(base string, suffix string) string {
	if suffix == "" {
		return base
	}
	if strings.TrimSpace(base) == "" {
		return suffix
	}
	return base + "\n\n" + suffix
}

func shouldFallbackToNonStreaming(err error) bool {
	var upstreamErr *llm.UpstreamError
	if !errors.As(err, &upstreamErr) {
		return false
	}
	switch upstreamErr.StatusCode {
	case 405, 415, 501:
		return true
	default:
		return isStreamUnsupportedError(upstreamErr)
	}
}

func isStreamUnsupportedError(err *llm.UpstreamError) bool {
	detail := strings.ToLower(strings.TrimSpace(err.Message + " " + err.Body))
	if detail == "" || !strings.Contains(detail, "stream") {
		return false
	}
	for _, marker := range []string{
		"not support",
		"not_supported",
		"unsupported",
		"not available",
		"does not support",
		"doesn't support",
	} {
		if strings.Contains(detail, marker) {
			return true
		}
	}
	return false
}

func emitFallbackText(text string, onDelta func(string) error) error {
	if onDelta == nil {
		return nil
	}
	content := text
	if content == "" {
		return nil
	}

	runes := []rune(content)
	const chunkSize = 24
	for start := 0; start < len(runes); start += chunkSize {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		if err := onDelta(string(runes[start:end])); err != nil {
			return err
		}
	}
	return nil
}

// isDocxMIME 判断文件是否为 DOCX 格式。
func isDocxMIME(mimeType, fileName string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	ext := ""
	if idx := strings.LastIndex(fileName, "."); idx >= 0 {
		ext = strings.ToLower(fileName[idx+1:])
	}
	return strings.Contains(m, "wordprocessingml") || strings.Contains(m, "msword") ||
		ext == "docx" || ext == "doc"
}

func isPDFMIME(mimeType, fileName string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	if m == "application/pdf" {
		return true
	}
	if idx := strings.LastIndex(fileName, "."); idx >= 0 {
		return strings.ToLower(fileName[idx+1:]) == "pdf"
	}
	return false
}

func isTextMIMEForEmbed(mimeType, fileName string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(m, "text/") {
		return true
	}
	switch m {
	case "application/json", "application/xml", "application/javascript", "application/typescript",
		"application/yaml", "application/x-yaml", "application/toml":
		return true
	}
	if idx := strings.LastIndex(fileName, "."); idx >= 0 {
		ext := strings.ToLower(fileName[idx+1:])
		switch ext {
		case "txt", "md", "markdown", "csv", "json", "xml", "html", "htm",
			"css", "js", "ts", "jsx", "tsx", "py", "go", "rs", "java",
			"c", "cpp", "h", "hpp", "cs", "rb", "php", "swift", "kt",
			"sh", "bash", "zsh", "yaml", "yml", "toml", "ini", "conf", "sql":
			return true
		}
	}
	return false
}

type userContextInput struct {
	Attachments         []AttachmentInput
	RAGChunks           []domainconversation.RAGChunk
	HistoricalArtifacts []domainconversation.ContextArtifact
	CurrentArtifacts    []domainconversation.ContextArtifact
	Snapshot            *snapshotContext
	Memory              []domainmemory.UserMemory
	RecallChunks        []domainconversation.MessageChunk
}

type snapshotContext struct {
	Summary  string
	FromTurn int
	ToTurn   int
	Strategy string
}

// prependStableFileContext 将可全文注入的文本文件固定放在消息前缀，避免多轮对话中
// 同一份文件内容漂移到最新 user 消息，破坏上游前缀缓存。
func prependStableFileContext(messages []llm.Message, attachments []AttachmentInput) []llm.Message {
	contextXML := buildStableFileContextXML(attachments)
	if contextXML.empty() {
		return messages
	}
	content := buildUserContextPrompt("", contextXML)
	if strings.TrimSpace(content) == "" {
		return messages
	}
	result := make([]llm.Message, 0, len(messages)+1)
	result = append(result, llm.Message{
		Role:    "system",
		Content: content,
	})
	result = append(result, messages...)
	return result
}

func buildStableFileContextXML(attachments []AttachmentInput) userContextXML {
	if len(attachments) == 0 {
		return userContextXML{}
	}
	items := make([]AttachmentInput, 0, len(attachments))
	for _, att := range attachments {
		if !isStableTextAttachment(att) {
			continue
		}
		items = append(items, att)
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := stableAttachmentSortKey(items[i])
		right := stableAttachmentSortKey(items[j])
		return left < right
	})

	contextXML := userContextXML{files: make([]string, 0, len(items))}
	for _, att := range items {
		contextXML.files = append(contextXML.files, formatAttachmentFileContext(att.FileName, att.ExtractedText))
	}
	return contextXML
}

func stableAttachmentSortKey(att AttachmentInput) string {
	if value := strings.TrimSpace(att.FileID); value != "" {
		return "0:" + value
	}
	if value := strings.TrimSpace(att.SHA256); value != "" {
		return "1:" + value
	}
	if value := strings.TrimSpace(att.FileName); value != "" {
		return "2:" + value
	}
	return "3:"
}

func imageAttachmentsForCurrentUser(attachments []AttachmentInput) []AttachmentInput {
	if len(attachments) == 0 {
		return nil
	}
	result := make([]AttachmentInput, 0)
	for _, att := range attachments {
		if att.Current && normalizeAttachmentKind(att.Kind, att.MimeType) == "image" {
			result = append(result, att)
		}
	}
	return result
}

func injectUserContext(
	ctx context.Context,
	messages []llm.Message,
	input userContextInput,
	cfg config.Config,
	storeProvider appstorage.Provider,
) []llm.Message {
	if len(input.Attachments) == 0 &&
		len(input.RAGChunks) == 0 &&
		len(input.HistoricalArtifacts) == 0 &&
		input.Snapshot == nil &&
		len(input.Memory) == 0 &&
		len(input.RecallChunks) == 0 {
		return messages
	}

	maxDim := cfg.ImageMaxDimension
	if maxDim <= 0 {
		maxDim = 1024
	}

	// 找到最后一条用户消息，构建 ContentParts
	lastUserIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx < 0 {
		return messages
	}

	lastUserMsg := messages[lastUserIdx]
	imageParts := make([]llm.ContentPart, 0, len(input.Attachments))
	contextXML := buildUserContextXML(input)

	for _, att := range input.Attachments {
		kind := normalizeAttachmentKind(att.Kind, att.MimeType)
		if kind == "image" {
			// 图片：读取文件字节并缩放
			storagePath := strings.TrimSpace(att.StoragePath)
			if storagePath == "" {
				continue
			}
			if storeProvider == nil {
				storeProvider = appstorage.NewRuntimeProvider(config.NewRuntime(cfg), nil)
			}
			store, storeErr := storeProvider.Open(ctx)
			if storeErr != nil {
				continue
			}
			reader, _, readErr := store.Open(ctx, storagePath)
			if readErr != nil {
				continue
			}
			imgData, readErr := io.ReadAll(io.LimitReader(reader, 50*1024*1024))
			_ = reader.Close()
			if readErr != nil {
				continue
			}
			mime := resolveImageMimeType(att.MimeType)
			resized := resizeImageIfNeeded(imgData, mime, maxDim)
			imageParts = append(imageParts, llm.ContentPart{
				Kind:     llm.ContentPartImage,
				MimeType: mime,
				Data:     resized,
			})
		}
	}

	if len(imageParts) == 0 && contextXML.empty() {
		return messages
	}

	content := strings.TrimSpace(lastUserMsg.Content)
	if !contextXML.empty() {
		content = buildUserContextPrompt(content, contextXML)
	}

	result := make([]llm.Message, len(messages))
	copy(result, messages)
	if len(imageParts) == 0 {
		result[lastUserIdx] = llm.Message{
			Role:    lastUserMsg.Role,
			Content: content,
		}
		return result
	}

	parts := make([]llm.ContentPart, 0, 1+len(imageParts))
	if content != "" {
		parts = append(parts, llm.ContentPart{
			Kind: llm.ContentPartText,
			Text: content,
		})
	}
	parts = append(parts, imageParts...)
	result[lastUserIdx] = llm.Message{Role: lastUserMsg.Role, Parts: parts}
	return result
}

func formatAttachmentFileContext(fileName string, text string) string {
	name := strings.TrimSpace(fileName)
	if name == "" {
		name = "未命名文件"
	}
	return `<file name="` + xmlEscapeAttr(name) + `">` + xmlEscapeText(strings.TrimSpace(text)) + `</file>`
}

type userContextXML struct {
	summary  string
	memory   []string
	files    []string
	evidence []string
	rag      []string
	recall   []string
}

func (x userContextXML) empty() bool {
	return strings.TrimSpace(x.summary) == "" &&
		len(x.memory) == 0 &&
		len(x.files) == 0 &&
		len(x.evidence) == 0 &&
		len(x.rag) == 0 &&
		len(x.recall) == 0
}

func buildUserContextXML(input userContextInput) userContextXML {
	return userContextXML{
		summary:  formatSnapshotContext(input.Snapshot),
		memory:   formatMemoryContext(input.Memory),
		evidence: formatHistoricalEvidenceContext(input.HistoricalArtifacts),
		rag:      formatRAGFileContext(input.RAGChunks),
		recall:   formatRecallContext(input.RecallChunks),
	}
}

func formatSnapshotContext(snapshot *snapshotContext) string {
	if snapshot == nil || strings.TrimSpace(snapshot.Summary) == "" {
		return ""
	}
	attrs := ` from="` + xmlEscapeAttr(fmt.Sprintf("%d", snapshot.FromTurn)) + `" to="` + xmlEscapeAttr(fmt.Sprintf("%d", snapshot.ToTurn)) + `"`
	if strategy := strings.TrimSpace(snapshot.Strategy); strategy != "" {
		attrs += ` strategy="` + xmlEscapeAttr(strategy) + `"`
	}
	return "<sum" + attrs + ">" + xmlEscapeText(strings.TrimSpace(snapshot.Summary)) + "</sum>"
}

func formatMemoryContext(memories []domainmemory.UserMemory) []string {
	if len(memories) == 0 {
		return nil
	}
	items := make([]string, 0, len(memories))
	for _, memory := range memories {
		key := strings.TrimSpace(memory.MemoryKey)
		value := strings.TrimSpace(memory.Value)
		if key == "" || value == "" {
			continue
		}
		items = append(items, `<mem k="`+xmlEscapeAttr(key)+`">`+xmlEscapeText(value)+`</mem>`)
	}
	return items
}

func formatRAGFileContext(chunks []domainconversation.RAGChunk) []string {
	if len(chunks) == 0 {
		return nil
	}
	items := make([]string, 0, len(chunks))
	for index, chunk := range chunks {
		text := strings.TrimSpace(chunk.Content)
		if text == "" {
			continue
		}
		name := strings.TrimSpace(chunk.FileName)
		if name == "" {
			name = strings.TrimSpace(chunk.FileID)
		}
		if name == "" {
			name = "unknown"
		}
		chunkIndex := chunk.ChunkIndex
		if chunkIndex <= 0 {
			chunkIndex = index + 1
		}
		items = append(items, `<doc name="`+xmlEscapeAttr(name)+`" i="`+xmlEscapeAttr(fmt.Sprintf("%d", chunkIndex))+`">`+xmlEscapeText(text)+`</doc>`)
	}
	return items
}

func formatHistoricalEvidenceContext(artifacts []domainconversation.ContextArtifact) []string {
	if len(artifacts) == 0 {
		return nil
	}
	items := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		content := strings.TrimSpace(artifact.Content)
		if content == "" {
			continue
		}
		kind := strings.TrimSpace(string(artifact.Kind))
		if kind == "" {
			kind = "evidence"
		}
		source := strings.TrimSpace(artifact.SourceTitle)
		if source == "" {
			source = strings.TrimSpace(artifact.SourceID)
		}
		if source == "" {
			source = "unknown"
		}
		items = append(items, `<ev k="`+xmlEscapeAttr(kind)+`" src="`+xmlEscapeAttr(source)+`">`+xmlEscapeText(compactSnippet(content, 500))+`</ev>`)
	}
	return items
}

func formatRecallContext(chunks []domainconversation.MessageChunk) []string {
	if len(chunks) == 0 {
		return nil
	}
	items := make([]string, 0, len(chunks))
	for index, chunk := range chunks {
		content := strings.TrimSpace(chunk.Content)
		if content == "" {
			continue
		}
		role := strings.TrimSpace(chunk.Role)
		if role == "" {
			role = "unknown"
		}
		chunkIndex := chunk.ChunkIndex
		if chunkIndex <= 0 {
			chunkIndex = index + 1
		}
		items = append(items, `<msg role="`+xmlEscapeAttr(role)+`" i="`+xmlEscapeAttr(fmt.Sprintf("%d", chunkIndex))+`">`+xmlEscapeText(compactSnippet(content, 300))+`</msg>`)
	}
	return items
}

func buildUserContextPrompt(userRequest string, contextXML userContextXML) string {
	var builder strings.Builder
	builder.WriteString("<ctx>")
	if strings.TrimSpace(contextXML.summary) != "" {
		builder.WriteString("\n")
		builder.WriteString(contextXML.summary)
	}
	if len(contextXML.memory) > 0 {
		builder.WriteString("\n<mems>\n")
		builder.WriteString(strings.Join(contextXML.memory, "\n"))
		builder.WriteString("\n</mems>")
	}
	if len(contextXML.files) > 0 {
		builder.WriteString("\n<files>\n")
		builder.WriteString(strings.Join(contextXML.files, "\n"))
		builder.WriteString("\n</files>")
	}
	if len(contextXML.evidence) > 0 {
		builder.WriteString("\n<evs>\n")
		builder.WriteString(strings.Join(contextXML.evidence, "\n"))
		builder.WriteString("\n</evs>")
	}
	if len(contextXML.rag) > 0 {
		builder.WriteString("\n<rag>\n")
		builder.WriteString(strings.Join(contextXML.rag, "\n"))
		builder.WriteString("\n</rag>")
	}
	if len(contextXML.recall) > 0 {
		builder.WriteString("\n<recall>\n")
		builder.WriteString(strings.Join(contextXML.recall, "\n"))
		builder.WriteString("\n</recall>")
	}
	builder.WriteString("\n</ctx>")

	request := strings.TrimSpace(userRequest)
	if request != "" {
		builder.WriteString("\n\n<q>")
		builder.WriteString(xmlEscapeText(request))
		builder.WriteString("</q>")
	}
	return builder.String()
}

var xmlTextReplacer = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

func xmlEscapeAttr(value string) string {
	var builder strings.Builder
	if err := xml.EscapeText(&builder, []byte(value)); err != nil {
		return ""
	}
	return builder.String()
}

func xmlEscapeText(value string) string {
	return xmlTextReplacer.Replace(value)
}

// filterMemoriesByScope 按 scope 过滤记忆列表。
func filterMemoriesByScope(memories []domainmemory.UserMemory, scopes ...string) []domainmemory.UserMemory {
	scopeSet := make(map[string]struct{}, len(scopes))
	for _, s := range scopes {
		scopeSet[s] = struct{}{}
	}
	result := make([]domainmemory.UserMemory, 0, len(memories))
	for _, m := range memories {
		if _, ok := scopeSet[m.Scope]; ok {
			result = append(result, m)
		}
	}
	return result
}

// selectRelevantMemories 从记忆列表中按关键词相关性选出最多 topK 条。
// 无向量服务时使用关键词匹配作为后备策略：key 或 value 命中查询词即认为相关。
func selectRelevantMemories(memories []domainmemory.UserMemory, query string, topK int) []domainmemory.UserMemory {
	if len(memories) == 0 || topK <= 0 {
		return nil
	}
	if len(memories) <= topK {
		return memories
	}

	// 查询词命中的记忆优先注入上下文，降低无关长期记忆对回答的干扰。
	queryLower := strings.ToLower(strings.TrimSpace(query))
	words := strings.Fields(queryLower)

	type scored struct {
		m     domainmemory.UserMemory
		score int
	}
	items := make([]scored, 0, len(memories))
	for _, m := range memories {
		if queryLower == "" || len(words) == 0 {
			items = append(items, scored{m, 0})
			continue
		}
		combined := strings.ToLower(m.MemoryKey + " " + m.Value)
		score := 0
		for _, w := range words {
			if len(w) >= 2 && strings.Contains(combined, w) {
				score++
			}
		}
		items = append(items, scored{m, score})
	}

	// 按分数降序，保持同分时原始顺序（stable）
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].score > items[j-1].score; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}

	result := make([]domainmemory.UserMemory, 0, topK)
	for i := 0; i < topK && i < len(items); i++ {
		result = append(result, items[i].m)
	}
	return result
}

// selectRelevantUserMemories 优先使用记忆向量检索；不可用或超时后回退到关键词筛选。
func (s *Service) selectRelevantUserMemories(ctx context.Context, userID uint, query string, memories []domainmemory.UserMemory, topK int) []domainmemory.UserMemory {
	fallback := selectRelevantMemories(memories, query, topK)
	if s == nil || s.embeddingSvc == nil || s.memoryRecorder == nil || strings.TrimSpace(query) == "" {
		return fallback
	}
	cfg := s.cfg.Snapshot()
	if !cfg.EmbeddingEnabled {
		return fallback
	}
	searchCtx, cancel := context.WithTimeout(ctx, semanticRecallDeadline)
	defer cancel()
	embeddings, err := s.embeddingSvc.EmbedTexts(searchCtx, []string{query})
	if err != nil || len(embeddings) == 0 {
		return fallback
	}
	matches, err := s.memoryRecorder.SearchUserMemoriesByEmbedding(searchCtx, userID, embeddings[0], topK, 0.7)
	if err != nil || len(matches) == 0 {
		return fallback
	}

	allowed := make(map[string]domainmemory.UserMemory, len(memories))
	for _, memory := range memories {
		key := strings.TrimSpace(memory.MemoryKey)
		if key == "" {
			continue
		}
		allowed[key] = memory
	}
	result := make([]domainmemory.UserMemory, 0, topK)
	seen := make(map[string]struct{}, topK)
	for _, memory := range matches {
		key := strings.TrimSpace(memory.MemoryKey)
		item, ok := allowed[key]
		if !ok {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		result = append(result, item)
		seen[key] = struct{}{}
		if len(result) >= topK {
			break
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}

// buildPreferencePrompt 将 scope=preference 的记忆格式化为行为指令型 system 提示。
func buildPreferencePrompt(memories []domainmemory.UserMemory, maxTokens int) string {
	if len(memories) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("# prefs\n")
	tokenCount := estimateTokens(sb.String())
	for _, m := range memories {
		line := "- " + strings.TrimSpace(m.MemoryKey) + ": " + strings.TrimSpace(m.Value) + "\n"
		lineTokens := estimateTokens(line)
		if int(tokenCount)+int(lineTokens) > maxTokens {
			break
		}
		sb.WriteString(line)
		tokenCount += lineTokens
	}
	return strings.TrimRight(sb.String(), "\n")
}
