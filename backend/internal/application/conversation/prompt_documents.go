package conversation

import (
	"sort"
	"strings"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// 对话文件的放置策略
//
// 文件内容跟随提交它的那条用户消息，以 <documents> 内容块放在该轮，而不是集中放在提示词开头：
//   - 模型能直接看出每份文件属于哪一轮；用户只发「继续」「看看这个」时也知道本轮新交的文件。
//   - 历史只在末尾追加：某一轮的渲染只取决于该轮附件快照与文件状态，跨轮字节一致。
//     新增文件只改变最新一轮，系统提示与此前全部历史仍可命中前缀缓存。
//
// 用户消息内的内容块按固定顺序排列（见 addUserTurnParts）：文件 → 图片 → 本轮动态上下文 → 用户原文。
// 资料在前、问题在后有利于回答质量；动态上下文（检索片段、检索回退全文、记忆、召回）只在本轮存在，
// 排在文件与图片之后，缓存断点可以落在它之前，本轮新交的文件在提交当轮即写入缓存。

type turnDocumentAccess string

const (
	// turnDocumentFull 全文随该轮提供。
	turnDocumentFull turnDocumentAccess = "full"
	// turnDocumentRetrieval 文件较大，只按问题检索片段。
	turnDocumentRetrieval turnDocumentAccess = "retrieval"
	// turnDocumentUnavailable 没有可用的提取文本，或超出全文上限且无法检索。
	turnDocumentUnavailable turnDocumentAccess = "unavailable"
	// turnDocumentNative 原始文件（PDF、音频、视频）以原生内容块随该轮发送（见 native_input.go）。
	turnDocumentNative turnDocumentAccess = "native"
	// turnDocumentUnsupported 当前模型或路由不支持该类型的输入（如纯文本模型收到图片或音频）。
	turnDocumentUnsupported turnDocumentAccess = "unsupported"
)

const (
	turnDocumentRetrievalNote   = "文件较大，未提供全文；需要时系统会按当前问题检索相关片段。"
	turnDocumentUnavailableNote = "无法读取该文件的内容。"
	turnDocumentNativeNote      = "原始文件已随此消息附上。"
	turnDocumentUnsupportedNote = "当前模型不支持该类型的输入，未提供该文件。"
)

// turnDocumentOptions 是与路由相关的放置选项：同一组文件在不同路由上可能以不同方式提供。
type turnDocumentOptions struct {
	// Native 为该路由的原生输入结果。命中的文件以内容块随所属轮次发送，并在 <documents> 中留一句说明，
	// 便于不携带文件名的协议（如 Gemini inlineData）也能对应文件；无法提供的图片与音视频留「不支持」说明。
	Native nativeInputs
	// SkipImages 为 true 时本次请求不发送图片（如媒体任务），图片不留说明。
	SkipImages bool
}

type turnDocument struct {
	Attachment AttachmentInput
	Access     turnDocumentAccess
	// Native 为 Access=native 时随轮次发送的原生内容块。
	Native llm.ContentPart
	// FromAssistant 表示文件由上一条助手回复产生，随其后的第一条用户消息提供。
	FromAssistant bool
}

// turnDocumentPlacement 是一次请求中文件在对话里的落位结果。
type turnDocumentPlacement struct {
	Messages []llm.Message
	// FullAttachments 是以全文（提取文本）形式进入对话的附件，用于上下文规划 trace。
	FullAttachments []AttachmentInput
	// NativeAttachments 是以原生内容块（PDF、音频、视频）进入对话的附件。
	NativeAttachments []AttachmentInput
	TokenEstimate     int64
}

// isPromptHistoryMessage 与 historyMessagesFromDomain 的过滤条件保持一致，
// 保证领域消息下标与发送给模型的历史下标一一对应。
func isPromptHistoryMessage(item model.Message) bool {
	if item.Role != "user" && item.Role != "assistant" && item.Role != "system" {
		return false
	}
	return !stringsEqualFold(item.Status, "blocked")
}

// placeTurnDocuments 把文件内容写入所属用户轮次。history 必须由 historyMessagesFromDomain(domainMessages)
// 生成且尚未合并同角色消息；attachments 为本轮文件规划结果（已带 ContextMode）。
// 当前轮的附件快照要等文件处理完成才写回消息，这里以 Current 标记归属最后一条用户消息。
func placeTurnDocuments(history []llm.Message, domainMessages []model.Message, attachments []AttachmentInput, options turnDocumentOptions) turnDocumentPlacement {
	placement := turnDocumentPlacement{Messages: history}
	if len(history) == 0 || len(attachments) == 0 {
		return placement
	}
	byFileID := make(map[string]AttachmentInput, len(attachments))
	currentIDs := make([]string, 0)
	for _, att := range attachments {
		fileID := strings.TrimSpace(att.FileID)
		if fileID == "" {
			continue
		}
		byFileID[fileID] = att
		if att.Current {
			currentIDs = append(currentIDs, fileID)
		}
	}

	lastUserIndex := -1
	historyIndex := 0
	for _, item := range domainMessages {
		if !isPromptHistoryMessage(item) {
			continue
		}
		if item.Role == "user" {
			lastUserIndex = historyIndex
		}
		historyIndex++
	}
	if historyIndex != len(history) {
		// 下标无法对齐时宁可不注入，也不能把文件挂到错误的轮次。
		return placement
	}

	result := cloneLLMMessages(history)
	pendingFromAssistant := make([]turnDocument, 0)
	historyIndex = -1
	for _, item := range domainMessages {
		if !isPromptHistoryMessage(item) {
			continue
		}
		historyIndex++
		fileIDs := parseAttachmentSnapshotFileIDs(item.Attachments)
		switch item.Role {
		case "assistant":
			for _, fileID := range fileIDs {
				if document, ok := newTurnDocument(byFileID[fileID], true, options); ok {
					pendingFromAssistant = append(pendingFromAssistant, document)
				}
			}
		case "user":
			if historyIndex == lastUserIndex {
				fileIDs = appendMissingFileIDs(fileIDs, currentIDs)
			}
			documents := pendingFromAssistant
			pendingFromAssistant = make([]turnDocument, 0)
			seen := make(map[string]struct{}, len(fileIDs))
			for _, fileID := range fileIDs {
				if _, duplicated := seen[fileID]; duplicated {
					continue
				}
				seen[fileID] = struct{}{}
				if document, ok := newTurnDocument(byFileID[fileID], false, options); ok {
					documents = append(documents, document)
				}
			}
			if len(documents) == 0 {
				continue
			}
			parts := []llm.ContentPart{turnDocumentsPart(documents)}
			for _, document := range documents {
				if document.Access == turnDocumentNative {
					parts = append(parts, document.Native)
				}
				switch document.Access {
				case turnDocumentFull:
					placement.FullAttachments = append(placement.FullAttachments, document.Attachment)
				case turnDocumentNative:
					placement.NativeAttachments = append(placement.NativeAttachments, document.Attachment)
				}
			}
			result[historyIndex] = addUserTurnParts(result[historyIndex], parts...)
			for _, part := range parts {
				placement.TokenEstimate += estimateContentPartTokens(part)
			}
		}
	}
	placement.Messages = result
	return placement
}

func appendMissingFileIDs(fileIDs []string, extra []string) []string {
	result := append([]string(nil), fileIDs...)
	seen := make(map[string]struct{}, len(result))
	for _, fileID := range result {
		seen[fileID] = struct{}{}
	}
	for _, fileID := range extra {
		if _, ok := seen[fileID]; ok {
			continue
		}
		seen[fileID] = struct{}{}
		result = append(result, fileID)
	}
	return result
}

// newTurnDocument 按文件规划结果与路由选项决定该轮如何呈现文件。
// 图片以图片内容块随轮次发送，不在此渲染；模型不支持图片时改为说明。
func newTurnDocument(att AttachmentInput, fromAssistant bool, options turnDocumentOptions) (turnDocument, bool) {
	fileID := strings.TrimSpace(att.FileID)
	if fileID == "" {
		return turnDocument{}, false
	}
	mode := strings.TrimSpace(att.ContextMode)
	document := turnDocument{Attachment: att, FromAssistant: fromAssistant}
	if strings.EqualFold(mode, fileContextModeDirectImage) {
		if options.SkipImages || fromAssistant || !options.Native.unsupported(att) {
			return turnDocument{}, false
		}
		document.Access = turnDocumentUnsupported
		return document, true
	}
	if native, ok := options.Native.Parts[fileID]; ok && !fromAssistant {
		document.Access = turnDocumentNative
		document.Native = native
		return document, true
	}
	if !fromAssistant && options.Native.unsupported(att) {
		document.Access = turnDocumentUnsupported
		return document, true
	}
	switch {
	case (strings.EqualFold(mode, fileContextModeFull) || strings.EqualFold(mode, fileContextModeRAGFallback)) &&
		strings.TrimSpace(att.ExtractedText) != "":
		document.Access = turnDocumentFull
	case strings.EqualFold(mode, fileContextModeRAG):
		document.Access = turnDocumentRetrieval
	default:
		kind := normalizeAttachmentKind(att.Kind, textutil.FirstNonEmpty(att.DetectedMIME, att.MimeType))
		if kind == "image" {
			// 未进入上下文的图片（如助手生成且未开启 OCR）不占位，避免无意义的提示。
			return turnDocument{}, false
		}
		document.Access = turnDocumentUnavailable
	}
	return document, true
}

func renderTurnDocuments(documents []turnDocument) string {
	var builder strings.Builder
	builder.WriteString("<documents>")
	for _, document := range documents {
		name := strings.TrimSpace(document.Attachment.FileName)
		if name == "" {
			name = "未命名文件"
		}
		builder.WriteString("\n<document source=\"")
		builder.WriteString(xmlEscapeAttr(name))
		builder.WriteString("\"")
		if document.FromAssistant {
			builder.WriteString(" origin=\"assistant\"")
		}
		switch document.Access {
		case turnDocumentFull:
			builder.WriteString(">\n")
			builder.WriteString(xmlEscapeText(strings.TrimSpace(document.Attachment.ExtractedText)))
			builder.WriteString("\n</document>")
		case turnDocumentRetrieval:
			builder.WriteString(" access=\"retrieval\">")
			builder.WriteString(turnDocumentRetrievalNote)
			builder.WriteString("</document>")
		case turnDocumentNative:
			builder.WriteString(" access=\"native\">")
			builder.WriteString(turnDocumentNativeNote)
			builder.WriteString("</document>")
		case turnDocumentUnsupported:
			builder.WriteString(" access=\"unsupported\">")
			builder.WriteString(turnDocumentUnsupportedNote)
			builder.WriteString("</document>")
		default:
			builder.WriteString(" access=\"unavailable\">")
			builder.WriteString(turnDocumentUnavailableNote)
			builder.WriteString("</document>")
		}
	}
	builder.WriteString("\n</documents>")
	return builder.String()
}

func turnDocumentsPart(documents []turnDocument) llm.ContentPart {
	return llm.ContentPart{Kind: llm.ContentPartFile, Text: renderTurnDocuments(documents)}
}

// userTurnPartRank 规定用户消息内容块的固定顺序：文件说明 → 原生文件/图片 → 本轮动态上下文 → 用户原文。
func userTurnPartRank(part llm.ContentPart) int {
	switch {
	case part.Kind == llm.ContentPartFile:
		return 0
	case isBinaryContentPart(part):
		return 1
	case part.Dynamic:
		return 2
	default:
		return 3
	}
}

// addUserTurnParts 按固定顺序把内容块并入用户消息，同类内容保持加入的先后次序。
// 历史重建与本轮组装都经由这里，保证同一轮在两种路径下的内容块顺序完全一致。
func addUserTurnParts(message llm.Message, parts ...llm.ContentPart) llm.Message {
	if len(parts) == 0 {
		return message
	}
	merged := append(messageContentParts(message), parts...)
	sort.SliceStable(merged, func(i, j int) bool {
		return userTurnPartRank(merged[i]) < userTurnPartRank(merged[j])
	})
	message.Content = ""
	message.Parts = merged
	return message
}

// markConversationCacheBoundary 在本轮能被下一轮原样重建的最长前缀末尾放置缓存断点。
//
// 本轮用户消息中，动态上下文之前的文件与图片在下一轮会作为历史原样重现；没有动态上下文时整条
// 消息都可重现。断点落在可重现部分的末尾，本轮写入的缓存覆盖系统提示、全部历史和本轮文件，
// 下一轮可整段命中，动态内容不进入缓存键。本轮没有可重现内容时退到上一条消息末尾。
// 同一轮内的工具循环在其后追加消息，断点同样有效。
func markConversationCacheBoundary(messages []llm.Message) []llm.Message {
	result, marked := markCurrentTurnCacheBoundary(messages)
	if marked {
		return result
	}
	lastUserIndex := lastUserMessageIndex(messages)
	if lastUserIndex <= 0 || !promptMessageHasContent(messages[lastUserIndex-1]) {
		return messages
	}
	result = cloneLLMMessages(messages)
	result[lastUserIndex-1].CacheControl = &llm.CacheControl{Type: "ephemeral"}
	return result
}

// markCurrentTurnCacheBoundary 只在本轮用户消息内部放置断点，返回是否放置成功。
func markCurrentTurnCacheBoundary(messages []llm.Message) ([]llm.Message, bool) {
	lastUserIndex := lastUserMessageIndex(messages)
	if lastUserIndex < 0 {
		return messages, false
	}
	message := messages[lastUserIndex]
	if len(message.Parts) == 0 {
		if strings.TrimSpace(message.Content) == "" {
			return messages, false
		}
		result := cloneLLMMessages(messages)
		result[lastUserIndex].CacheControl = &llm.CacheControl{Type: "ephemeral"}
		return result, true
	}
	boundary := -1
	for index, part := range message.Parts {
		if part.Dynamic {
			break
		}
		if contentPartHasContent(part) {
			boundary = index
		}
	}
	if boundary < 0 {
		return messages, false
	}
	result := cloneLLMMessages(messages)
	parts := append([]llm.ContentPart(nil), message.Parts...)
	parts[boundary].CacheControl = &llm.CacheControl{Type: "ephemeral"}
	result[lastUserIndex].Parts = parts
	return result, true
}

func lastUserMessageIndex(messages []llm.Message) int {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role == "user" {
			return index
		}
	}
	return -1
}

func promptMessageHasContent(message llm.Message) bool {
	if strings.TrimSpace(message.Content) != "" {
		return true
	}
	for _, part := range message.Parts {
		if contentPartHasContent(part) {
			return true
		}
	}
	return false
}

func contentPartHasContent(part llm.ContentPart) bool {
	if isBinaryContentPart(part) {
		return len(part.Data) > 0
	}
	return strings.TrimSpace(part.Text) != ""
}

func isBinaryContentPart(part llm.ContentPart) bool {
	switch part.Kind {
	case llm.ContentPartImage, llm.ContentPartAudio, llm.ContentPartVideo, llm.ContentPartDocument:
		return true
	default:
		return false
	}
}
