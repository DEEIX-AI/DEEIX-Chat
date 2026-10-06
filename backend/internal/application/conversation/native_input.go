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
//  2. 路由协议能编码该类型与格式（见 nativeProtocolSupport）；
//  3. 上游可信：原生协议（Anthropic Messages、Gemini）按原样透传；
//     OpenAI 兼容协议只在官方地址或显式声明时启用，第三方中转站可能不认识这些内容块；
//  4. 大小、页数与数量在保守上限内。
//
// 文件一律以 base64 内嵌在请求体中，不使用各家 Files API。PDF 不满足条件时回退到既有的文本提取；
// 音频与视频没有文本可回退，所属轮次改为留「不支持」说明。
// 策略按路由计算：故障转移到另一条路由时会重新判断，不会把原生内容块发给不支持它的协议。

// 原生文件的保守上限。Anthropic 单次请求上限 32MB 且上下文窗口不足 1M 时最多 100 页；Gemini 内联数据的
// 请求上限为 20MB（base64 后）。所有原生文件共用一份总预算，base64 膨胀约 4/3 后仍在 20MB 以内。
const (
	maxNativeInputCount      = 5
	maxNativeInputTotalBytes = 14 * 1024 * 1024
	maxNativePDFTotalBytes   = 8 * 1024 * 1024
	maxNativePDFTotalPages   = 100
	nativeDocumentMIMEPDF    = "application/pdf"
	officialOpenRouterHost   = "openrouter.ai"
)

// nativeInputKind 是可以原生发送的文件类型。
type nativeInputKind string

const (
	nativeInputPDF   nativeInputKind = "pdf"
	nativeInputAudio nativeInputKind = "audio"
	nativeInputVideo nativeInputKind = "video"
)

// 各协议能编码的音视频格式（MIME 已归一化，见 nativeInputMIME）。
var (
	// Gemini inlineData：https://ai.google.dev/gemini-api/docs/audio 与 /video-understanding。
	geminiNativeAudioMIMEs = mimeSet("audio/wav", "audio/mp3", "audio/aiff", "audio/aac", "audio/ogg", "audio/flac")
	geminiNativeVideoMIMEs = mimeSet("video/mp4", "video/mpeg", "video/mov", "video/avi", "video/x-flv", "video/mpg", "video/webm", "video/wmv", "video/3gpp")
	// OpenAI Chat Completions input_audio 只接受 wav 与 mp3。
	openAINativeAudioMIMEs = mimeSet("audio/wav", "audio/mp3")
	// OpenRouter Chat Completions：input_audio 与 video_url（data URL）。
	openRouterNativeAudioMIMEs = mimeSet("audio/wav", "audio/mp3", "audio/aiff", "audio/aac", "audio/ogg", "audio/flac", "audio/m4a")
	openRouterNativeVideoMIMEs = mimeSet("video/mp4", "video/mpeg", "video/mov", "video/webm")
)

func mimeSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

// nativeInputMIME 把附件 MIME 归一化为协议文档使用的写法（如 audio/mpeg → audio/mp3、video/quicktime → video/mov）。
func nativeInputMIME(att AttachmentInput) string {
	mime := strings.ToLower(strings.TrimSpace(textutil.FirstNonEmpty(att.DetectedMIME, att.MimeType)))
	if index := strings.Index(mime, ";"); index > 0 {
		mime = strings.TrimSpace(mime[:index])
	}
	switch mime {
	case "audio/mpeg", "audio/mpeg3", "audio/x-mpeg-3":
		return "audio/mp3"
	case "audio/x-wav", "audio/wave", "audio/vnd.wave":
		return "audio/wav"
	case "audio/x-aiff":
		return "audio/aiff"
	case "audio/x-flac":
		return "audio/flac"
	case "audio/mp4", "audio/x-m4a":
		return "audio/m4a"
	case "video/quicktime":
		return "video/mov"
	case "video/x-msvideo":
		return "video/avi"
	case "video/x-ms-wmv":
		return "video/wmv"
	case "video/3gpp2":
		return "video/3gpp"
	}
	return mime
}

// nativeInputPolicy 是某条路由上可以原生发送的输入类型。
type nativeInputPolicy struct {
	// ImageUnsupported 为 true 时模型声明不支持图片，图片不以图片内容块发送；能力未知时保持既有行为（发送）。
	// 用否定形式让零值表示「照常发送」。
	ImageUnsupported bool
	// PDF 为 true 时满足条件的 PDF 以原生文档内容块发送。
	PDF bool
	// AudioMIMEs / VideoMIMEs 为该路由可原生发送的音视频格式；为空表示不支持。
	AudioMIMEs map[string]struct{}
	VideoMIMEs map[string]struct{}
}

// accepts 判断附件能否在该路由上原生发送（不含预算）。
func (p nativeInputPolicy) accepts(att AttachmentInput) bool {
	switch nativeInputKindOf(att) {
	case nativeInputPDF:
		return p.PDF
	case nativeInputAudio:
		_, ok := p.AudioMIMEs[nativeInputMIME(att)]
		return ok
	case nativeInputVideo:
		_, ok := p.VideoMIMEs[nativeInputMIME(att)]
		return ok
	}
	return false
}

// nativeProtocolSupport 是路由能原生发送的内容块，已按上游可信度收敛（见文件开头第 3 条）。
type nativeProtocolSupport struct {
	pdf        bool
	audioMIMEs map[string]struct{}
	videoMIMEs map[string]struct{}
}

// resolveNativeInputPolicy 计算路由的原生输入策略。
func resolveNativeInputPolicy(route *channel.ResolvedRoute) nativeInputPolicy {
	if route == nil {
		return nativeInputPolicy{}
	}
	modalities := domainchannel.ResolveInputModalities(route.ModelCapabilitiesJSON, route.CatalogInputModalities)
	support := nativeProtocolSupportFor(route, modalities.Source == domainchannel.InputModalitiesSourceExplicit)
	policy := nativeInputPolicy{
		ImageUnsupported: modalities.Known() && !modalities.Supports(domainchannel.InputModalityImage),
		PDF:              support.pdf && modalities.Supports(domainchannel.InputModalityPDF),
	}
	if modalities.Supports(domainchannel.InputModalityAudio) {
		policy.AudioMIMEs = support.audioMIMEs
	}
	if modalities.Supports(domainchannel.InputModalityVideo) {
		policy.VideoMIMEs = support.videoMIMEs
	}
	return policy
}

// nativeProtocolSupportFor 返回路由协议能以原生内容块发送的类型，并按上游可信度收敛。
func nativeProtocolSupportFor(route *channel.ResolvedRoute, explicit bool) nativeProtocolSupport {
	switch llm.NormalizeAdapter(route.Protocol) {
	case llm.AdapterAnthropicMessages:
		return nativeProtocolSupport{pdf: true}
	case llm.AdapterGoogleGenerateContent:
		return nativeProtocolSupport{pdf: true, audioMIMEs: geminiNativeAudioMIMEs, videoMIMEs: geminiNativeVideoMIMEs}
	case llm.AdapterOpenAIResponses:
		return nativeProtocolSupport{pdf: explicit || isOfficialOpenAIBaseURL(route.BaseURL)}
	case llm.AdapterOpenAIChatCompletions:
		if !explicit && !isOfficialOpenAIBaseURL(route.BaseURL) {
			return nativeProtocolSupport{}
		}
		return nativeProtocolSupport{pdf: true, audioMIMEs: openAINativeAudioMIMEs}
	case llm.AdapterOpenRouterChat:
		if !explicit && !isOfficialOpenRouterBaseURL(route.BaseURL) {
			return nativeProtocolSupport{}
		}
		return nativeProtocolSupport{pdf: true, audioMIMEs: openRouterNativeAudioMIMEs, videoMIMEs: openRouterNativeVideoMIMEs}
	case llm.AdapterOpenRouterResponses:
		return nativeProtocolSupport{pdf: explicit || isOfficialOpenRouterBaseURL(route.BaseURL)}
	default:
		return nativeProtocolSupport{}
	}
}

func isOfficialOpenRouterBaseURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return strings.ToLower(parsed.Hostname()) == officialOpenRouterHost
}

// nativeInputKindOf 返回附件的原生输入类型；不是 PDF、音频或视频时返回空。
func nativeInputKindOf(att AttachmentInput) nativeInputKind {
	category := strings.ToLower(strings.TrimSpace(att.FileCategory))
	mime := nativeInputMIME(att)
	switch {
	case category == fileCategoryPDF || mime == nativeDocumentMIMEPDF:
		return nativeInputPDF
	case category == fileCategoryAudio || strings.HasPrefix(mime, "audio/"):
		return nativeInputAudio
	case category == fileCategoryVideo || strings.HasPrefix(mime, "video/"):
		return nativeInputVideo
	}
	return ""
}

// isUserMediaAttachment 判断附件是用户提交的音频或视频（助手生成的视频不回传给模型）。
func isUserMediaAttachment(att AttachmentInput) bool {
	kind := nativeInputKindOf(att)
	return (kind == nativeInputAudio || kind == nativeInputVideo) && isUserSubmittedAttachment(att)
}

func isUserSubmittedAttachment(att AttachmentInput) bool {
	return att.Current || strings.EqualFold(strings.TrimSpace(att.MessageRole), "user")
}

// isNativeInputCandidate 判断附件是否可以原生发送（不含路由策略与整体预算）。
//
// PDF 只替换系统本来就会提供全文的文件，以及没有可用提取文本的文件（如扫描件）；
// 走检索或因过长被跳过的 PDF 保持原有处理，避免原生发送绕开上下文预算。音频与视频没有提取文本，用户提交即可。
func isNativeInputCandidate(att AttachmentInput) bool {
	if !isUserSubmittedAttachment(att) || strings.TrimSpace(att.StoragePath) == "" {
		return false
	}
	if att.FileSize <= 0 || att.FileSize > maxNativeInputTotalBytes {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(att.ProcessingStatus), "failed") {
		return false
	}
	switch nativeInputKindOf(att) {
	case nativeInputPDF:
		if att.FileSize > maxNativePDFTotalBytes || att.PageCount > maxNativePDFTotalPages {
			return false
		}
		switch strings.TrimSpace(att.ContextMode) {
		case fileContextModeFull, fileContextModeRAGFallback:
			return true
		case fileContextModeSkipped:
			return strings.TrimSpace(att.ExtractedText) == ""
		}
		return false
	case nativeInputAudio, nativeInputVideo:
		return true
	}
	return false
}

// selectNativeInputs 在预算内挑出原生发送的文件，越新的文件越优先。
// attachments 按会话中出现的先后排列，本轮文件在最后。
func selectNativeInputs(attachments []AttachmentInput, policy nativeInputPolicy) []AttachmentInput {
	selected := make([]AttachmentInput, 0, maxNativeInputCount)
	seen := make(map[string]struct{})
	totalBytes, pdfBytes, pdfPages := int64(0), int64(0), 0
	for index := len(attachments) - 1; index >= 0 && len(selected) < maxNativeInputCount; index-- {
		att := attachments[index]
		fileID := strings.TrimSpace(att.FileID)
		if _, duplicated := seen[fileID]; duplicated || fileID == "" || !policy.accepts(att) || !isNativeInputCandidate(att) {
			continue
		}
		if totalBytes+att.FileSize > maxNativeInputTotalBytes {
			continue
		}
		isPDF := nativeInputKindOf(att) == nativeInputPDF
		if isPDF && (pdfBytes+att.FileSize > maxNativePDFTotalBytes || pdfPages+att.PageCount > maxNativePDFTotalPages) {
			continue
		}
		seen[fileID] = struct{}{}
		totalBytes += att.FileSize
		if isPDF {
			pdfBytes += att.FileSize
			pdfPages += att.PageCount
		}
		selected = append(selected, att)
	}
	return selected
}

// nativeInputCache 缓存一次发送内已读取的原生文件（按 fileID），读取失败也会记录。
// 同一次发送中，处理过程展示、首条路由与故障转移路由共用它，三者看到的原生决定一致，也不会重复读取存储。
type nativeInputCache struct {
	parts map[string]*llm.ContentPart
}

func newNativeInputCache() *nativeInputCache {
	return &nativeInputCache{parts: make(map[string]*llm.ContentPart)}
}

// nativeInputs 是某条路由上的原生输入结果。
type nativeInputs struct {
	Policy nativeInputPolicy
	// Parts 为实际原生发送的内容块（按 fileID）；读取失败的文件不在其中。
	Parts map[string]llm.ContentPart
}

// unsupported 判断附件在该路由上无法提供：模型不支持的图片，以及无法原生发送的用户音视频
// （没有文本可回退）。超出预算或读取失败的音视频不算，按「无法读取」处理。
func (n nativeInputs) unsupported(att AttachmentInput) bool {
	if _, native := n.Parts[strings.TrimSpace(att.FileID)]; native {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(att.ContextMode), fileContextModeDirectImage) {
		return n.Policy.ImageUnsupported
	}
	return isUserMediaAttachment(att) && !n.Policy.accepts(att)
}

// resolveNativeInputs 计算路由的原生输入策略，并返回该路由上实际原生发送的内容块。
// 读取失败的文件不在结果中：PDF 由文本提取兜底，音视频留「无法读取」说明。
func (s *Service) resolveNativeInputs(ctx context.Context, route *channel.ResolvedRoute, attachments []AttachmentInput, cache *nativeInputCache) nativeInputs {
	result := nativeInputs{Policy: resolveNativeInputPolicy(route)}
	selected := selectNativeInputs(attachments, result.Policy)
	if len(selected) == 0 {
		return result
	}
	if cache == nil {
		cache = newNativeInputCache()
	}
	missing := make([]AttachmentInput, 0, len(selected))
	for _, att := range selected {
		if _, loaded := cache.parts[strings.TrimSpace(att.FileID)]; !loaded {
			missing = append(missing, att)
		}
	}
	if len(missing) > 0 {
		loaded := s.loadNativeInputParts(ctx, missing)
		for _, att := range missing {
			fileID := strings.TrimSpace(att.FileID)
			if part, ok := loaded[fileID]; ok {
				cache.parts[fileID] = &part
			} else {
				cache.parts[fileID] = nil
			}
		}
	}
	result.Parts = make(map[string]llm.ContentPart, len(selected))
	for _, att := range selected {
		if part := cache.parts[strings.TrimSpace(att.FileID)]; part != nil {
			result.Parts[strings.TrimSpace(att.FileID)] = *part
		}
	}
	return result
}

// routeAttachmentTraceItems 把与路由相关的决定写入处理过程展示用的附件副本：
// 原生发送的文件标记为 native_document，无法提供的图片与音视频标记为 unsupported。
func routeAttachmentTraceItems(items []AttachmentInput, native nativeInputs, skipImages bool) []AttachmentInput {
	result := make([]AttachmentInput, len(items))
	copy(result, items)
	for index := range result {
		fileID := strings.TrimSpace(result[index].FileID)
		if _, ok := native.Parts[fileID]; ok {
			result[index].ContextMode = fileContextModeNativeDocument
			continue
		}
		isImage := result[index].ContextMode == fileContextModeDirectImage
		if (isImage && skipImages) || !native.unsupported(result[index]) {
			continue
		}
		result[index].ContextMode = fileContextModeUnsupported
	}
	return result
}

// loadNativeInputParts 读取选中文件的原始字节。单个文件读取失败时跳过它。
func (s *Service) loadNativeInputParts(ctx context.Context, attachments []AttachmentInput) map[string]llm.ContentPart {
	if len(attachments) == 0 || s == nil || s.storeProvider == nil {
		return nil
	}
	store, err := s.storeProvider.Open(ctx)
	if err != nil {
		s.warnNativeInput("native_input_store_open_failed", "", err)
		return nil
	}
	parts := make(map[string]llm.ContentPart, len(attachments))
	for _, att := range attachments {
		data, err := readNativeInput(ctx, store, att)
		if err != nil {
			s.warnNativeInput("native_input_read_failed", att.FileID, err)
			continue
		}
		parts[strings.TrimSpace(att.FileID)] = newNativeInputPart(att, data)
	}
	return parts
}

func newNativeInputPart(att AttachmentInput, data []byte) llm.ContentPart {
	part := llm.ContentPart{Data: data, FileName: strings.TrimSpace(att.FileName), MimeType: nativeInputMIME(att)}
	switch nativeInputKindOf(att) {
	case nativeInputAudio:
		part.Kind = llm.ContentPartAudio
		part.DurationSeconds = att.DurationSeconds
	case nativeInputVideo:
		part.Kind = llm.ContentPartVideo
		part.DurationSeconds = att.DurationSeconds
	default:
		part.Kind = llm.ContentPartDocument
		part.MimeType = nativeDocumentMIMEPDF
		part.PageCount = att.PageCount
	}
	return part
}

func readNativeInput(ctx context.Context, store objectstorage.Store, att AttachmentInput) ([]byte, error) {
	reader, _, err := store.Open(ctx, strings.TrimSpace(att.StoragePath))
	if err != nil {
		return nil, err
	}
	defer reader.Close() //nolint:errcheck
	data, err := io.ReadAll(io.LimitReader(reader, maxNativeInputTotalBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > maxNativeInputTotalBytes {
		return nil, fmt.Errorf("native input size %d outside limit", len(data))
	}
	return data, nil
}

func (s *Service) warnNativeInput(message string, fileID string, err error) {
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

func (t *attachmentRouteTrace) resolve(ctx context.Context, s *Service, route *channel.ResolvedRoute, attachments []AttachmentInput, cache *nativeInputCache) {
	native := s.resolveNativeInputs(ctx, route, attachments, cache)
	items := routeAttachmentTraceItems(t.items, native, t.skipImages)
	t.summary, t.markdown, t.payload = buildAttachmentProcessTrace(t.fileMode, items)
}

func (t *attachmentRouteTrace) visible() bool {
	return shouldShowAttachmentProcessTrace(t.items)
}
