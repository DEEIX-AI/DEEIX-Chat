package conversation

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/objectstorage"
	"go.uber.org/zap"
)

// 原生输入策略
//
// 文件能否以原生内容块发送，由四个条件同时决定：
//  1. 模型输入模态包含该类型：管理员在能力 JSON 中显式声明 inputModalities，否则取 models.dev 目录；
//  2. 路由协议能编码该内容块（见 nativePDFProtocolAllowed）；
//  3. 上游可信：原生协议（Anthropic Messages、Gemini generateContent）按原样透传；
//     OpenAI 兼容协议只在官方地址或显式声明时启用，第三方中转站可能不认识文件内容块；
//  4. 大小、页数与数量在保守上限内。
//
// 文件一律以 base64 内嵌在请求体中，不使用各家 Files API。任一条件不满足时回退到既有的文本提取。
// 策略按路由计算：故障转移到另一条路由时会重新判断，不会把原生内容块发给不支持它的协议。

// 原生文档的保守上限。Anthropic 单次请求上限 32MB（base64 后）且上下文窗口不足 1M 时最多 100 页，
// 是三家中最严格的；文档与图片同在一个请求里，因此文档只占用其中一部分。
const (
	maxNativeDocumentCount      = 5
	maxNativeDocumentBytes      = 8 * 1024 * 1024
	maxNativeDocumentTotalBytes = 8 * 1024 * 1024
	maxNativeDocumentTotalPages = 100
	nativeDocumentMIMEPDF       = "application/pdf"
	officialOpenRouterHost      = "openrouter.ai"
)

// nativeInputPolicy 是某条路由上可以原生发送的输入类型。
type nativeInputPolicy struct {
	// Image 为 false 时图片不以图片内容块发送；能力未知时保持既有行为（发送）。
	Image bool
	// PDF 为 true 时满足条件的 PDF 以原生文档内容块发送。
	PDF bool
}

// resolveNativeInputPolicy 计算路由的原生输入策略。
func resolveNativeInputPolicy(route *channel.ResolvedRoute) nativeInputPolicy {
	if route == nil {
		return nativeInputPolicy{Image: true}
	}
	modalities := domainchannel.ResolveInputModalities(route.ModelCapabilitiesJSON, route.CatalogInputModalities)
	return nativeInputPolicy{
		Image: !modalities.Known() || modalities.Supports(domainchannel.InputModalityImage),
		PDF: modalities.Supports(domainchannel.InputModalityPDF) &&
			nativePDFProtocolAllowed(route, modalities.Source == domainchannel.InputModalitiesSourceExplicit),
	}
}

// nativePDFProtocolAllowed 判断路由协议能否以原生内容块发送 PDF，以及上游是否可信。
func nativePDFProtocolAllowed(route *channel.ResolvedRoute, explicit bool) bool {
	switch llm.NormalizeAdapter(route.Protocol) {
	case llm.AdapterAnthropicMessages, llm.AdapterGoogleGenerateContent:
		return true
	case llm.AdapterOpenAIResponses, llm.AdapterOpenAIChatCompletions:
		return explicit || isOfficialOpenAIBaseURL(route.BaseURL)
	case llm.AdapterOpenRouterChat, llm.AdapterOpenRouterResponses:
		return explicit || isOfficialOpenRouterBaseURL(route.BaseURL)
	default:
		return false
	}
}

func isOfficialOpenRouterBaseURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return strings.ToLower(parsed.Hostname()) == officialOpenRouterHost
}

// isNativeDocumentCandidate 判断附件是否可以作为原生 PDF 发送（不含整体预算）。
//
// 只替换系统本来就会提供全文的文件，以及没有可用提取文本的文件（如扫描件）；
// 走检索或因过长被跳过的文件保持原有处理，避免原生发送绕开上下文预算。
func isNativeDocumentCandidate(att AttachmentInput) bool {
	if !att.Current && !strings.EqualFold(strings.TrimSpace(att.MessageRole), "user") {
		return false
	}
	if !isPDFAttachment(att) || strings.TrimSpace(att.StoragePath) == "" {
		return false
	}
	if att.FileSize <= 0 || att.FileSize > maxNativeDocumentBytes || att.PageCount > maxNativeDocumentTotalPages {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(att.ProcessingStatus), "failed") {
		return false
	}
	switch strings.TrimSpace(att.ContextMode) {
	case fileContextModeFull, fileContextModeRAGFallback:
		return true
	case fileContextModeSkipped:
		return strings.TrimSpace(att.ExtractedText) == ""
	default:
		return false
	}
}

func isPDFAttachment(att AttachmentInput) bool {
	if strings.EqualFold(strings.TrimSpace(att.FileCategory), fileCategoryPDF) {
		return true
	}
	mime := strings.ToLower(strings.TrimSpace(textutil.FirstNonEmpty(att.DetectedMIME, att.MimeType)))
	return mime == nativeDocumentMIMEPDF
}

// selectNativeDocuments 在预算内挑出原生发送的 PDF，越新的文件越优先。
// attachments 按会话中出现的先后排列，本轮文件在最后。
func selectNativeDocuments(attachments []AttachmentInput, policy nativeInputPolicy) []AttachmentInput {
	if !policy.PDF {
		return nil
	}
	selected := make([]AttachmentInput, 0, maxNativeDocumentCount)
	seen := make(map[string]struct{})
	totalBytes, totalPages := int64(0), 0
	for index := len(attachments) - 1; index >= 0 && len(selected) < maxNativeDocumentCount; index-- {
		att := attachments[index]
		fileID := strings.TrimSpace(att.FileID)
		if _, duplicated := seen[fileID]; duplicated || fileID == "" || !isNativeDocumentCandidate(att) {
			continue
		}
		if totalBytes+att.FileSize > maxNativeDocumentTotalBytes || totalPages+att.PageCount > maxNativeDocumentTotalPages {
			continue
		}
		seen[fileID] = struct{}{}
		totalBytes += att.FileSize
		totalPages += att.PageCount
		selected = append(selected, att)
	}
	return selected
}

// nativeDocumentCache 缓存一次发送内已读取的原生文档（按 fileID），读取失败也会记录。
// 同一次发送中，处理过程展示、首条路由与故障转移路由共用它，三者看到的原生决定一致，也不会重复读取存储。
type nativeDocumentCache struct {
	parts map[string]*llm.ContentPart
}

func newNativeDocumentCache() *nativeDocumentCache {
	return &nativeDocumentCache{parts: make(map[string]*llm.ContentPart)}
}

// resolveNativeDocuments 计算路由的原生输入策略，并返回该路由上实际原生发送的文档内容块。
// 读取失败的文件不在结果中，由文本提取兜底。
func (s *Service) resolveNativeDocuments(ctx context.Context, route *channel.ResolvedRoute, attachments []AttachmentInput, cache *nativeDocumentCache) (nativeInputPolicy, map[string]llm.ContentPart) {
	policy := resolveNativeInputPolicy(route)
	selected := selectNativeDocuments(attachments, policy)
	if len(selected) == 0 {
		return policy, nil
	}
	if cache == nil {
		cache = newNativeDocumentCache()
	}
	missing := make([]AttachmentInput, 0, len(selected))
	for _, att := range selected {
		if _, loaded := cache.parts[strings.TrimSpace(att.FileID)]; !loaded {
			missing = append(missing, att)
		}
	}
	if len(missing) > 0 {
		loaded := s.loadNativeDocumentParts(ctx, missing)
		for _, att := range missing {
			fileID := strings.TrimSpace(att.FileID)
			if part, ok := loaded[fileID]; ok {
				cache.parts[fileID] = &part
			} else {
				cache.parts[fileID] = nil
			}
		}
	}
	result := make(map[string]llm.ContentPart, len(selected))
	for _, att := range selected {
		if part := cache.parts[strings.TrimSpace(att.FileID)]; part != nil {
			result[strings.TrimSpace(att.FileID)] = *part
		}
	}
	return policy, result
}

// routeAttachmentTraceItems 把与路由相关的决定写入处理过程展示用的附件副本：
// 原生发送的 PDF 标记为 native_document，模型不支持的图片标记为 unsupported。
func routeAttachmentTraceItems(items []AttachmentInput, policy nativeInputPolicy, nativeDocuments map[string]llm.ContentPart, skipImages bool) []AttachmentInput {
	result := make([]AttachmentInput, len(items))
	copy(result, items)
	for index := range result {
		fileID := strings.TrimSpace(result[index].FileID)
		if _, native := nativeDocuments[fileID]; native {
			result[index].ContextMode = fileContextModeNativeDocument
			continue
		}
		if result[index].ContextMode == fileContextModeDirectImage && !policy.Image && !skipImages {
			result[index].ContextMode = fileContextModeUnsupported
		}
	}
	return result
}

// loadNativeDocumentParts 读取选中的 PDF 原始字节。单个文件读取失败时跳过它，由文本提取兜底。
func (s *Service) loadNativeDocumentParts(ctx context.Context, attachments []AttachmentInput) map[string]llm.ContentPart {
	if len(attachments) == 0 || s == nil || s.storeProvider == nil {
		return nil
	}
	store, err := s.storeProvider.Open(ctx)
	if err != nil {
		s.warnNativeDocument("native_document_store_open_failed", "", err)
		return nil
	}
	parts := make(map[string]llm.ContentPart, len(attachments))
	for _, att := range attachments {
		data, err := readNativeDocument(ctx, store, att)
		if err != nil {
			s.warnNativeDocument("native_document_read_failed", att.FileID, err)
			continue
		}
		parts[strings.TrimSpace(att.FileID)] = llm.ContentPart{
			Kind:      llm.ContentPartDocument,
			MimeType:  nativeDocumentMIMEPDF,
			Data:      data,
			FileName:  strings.TrimSpace(att.FileName),
			PageCount: att.PageCount,
		}
	}
	return parts
}

func readNativeDocument(ctx context.Context, store objectstorage.Store, att AttachmentInput) ([]byte, error) {
	reader, _, err := store.Open(ctx, strings.TrimSpace(att.StoragePath))
	if err != nil {
		return nil, err
	}
	defer reader.Close() //nolint:errcheck
	data, err := io.ReadAll(io.LimitReader(reader, maxNativeDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > maxNativeDocumentBytes {
		return nil, fmt.Errorf("native document size %d outside limit", len(data))
	}
	return data, nil
}

func (s *Service) warnNativeDocument(message string, fileID string, err error) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.Warn(message, zap.String("file_id", strings.TrimSpace(fileID)), zap.Error(err))
}

// attachmentRouteTrace 是「文件上下文」处理阶段的展示内容，随路由重新计算。
type attachmentRouteTrace struct {
	fileMode   string
	items      []AttachmentInput
	skipImages bool

	summary  string
	markdown string
	payload  *tracePayload
}

func (t *attachmentRouteTrace) resolve(ctx context.Context, s *Service, route *channel.ResolvedRoute, attachments []AttachmentInput, cache *nativeDocumentCache) {
	policy, nativeDocuments := s.resolveNativeDocuments(ctx, route, attachments, cache)
	items := routeAttachmentTraceItems(t.items, policy, nativeDocuments, t.skipImages)
	t.summary, t.markdown, t.payload = buildAttachmentProcessTrace(t.fileMode, items)
}

func (t *attachmentRouteTrace) visible() bool {
	return shouldShowAttachmentProcessTrace(t.items)
}
