package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
)

const (
	fileContextModeDirectImage = "direct_image"
	fileContextModeFull        = "full_context"
	fileContextModeRAG         = "rag"
	fileContextModeRAGFallback = "rag_fallback_full_context"
	fileContextModeSkipped     = "skipped"
)

type attachmentSnapshotRef struct {
	FileID       string `json:"file_id"`
	Kind         string `json:"kind"`
	MimeType     string `json:"mime_type"`
	DetectedMIME string `json:"detected_mime"`
}

type conversationFileContextPlan struct {
	Attachments     []AttachmentInput
	FullAttachments []AttachmentInput
	RAGAttachments  []AttachmentInput
	Skipped         []AttachmentInput
}

func collectConversationFileIDs(messages []model.Message, currentFileIDs []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(currentFileIDs))
	add := func(raw string) {
		fileID := strings.TrimSpace(raw)
		if fileID == "" {
			return
		}
		if _, ok := seen[fileID]; ok {
			return
		}
		seen[fileID] = struct{}{}
		result = append(result, fileID)
	}

	for _, item := range messages {
		if !strings.EqualFold(strings.TrimSpace(item.Status), "success") {
			continue
		}
		for _, fileID := range parseAttachmentSnapshotFileIDs(item.Attachments) {
			add(fileID)
		}
	}
	for _, fileID := range currentFileIDs {
		add(fileID)
	}
	return result
}

func parseAttachmentSnapshotFileIDs(raw string) []string {
	items := parseAttachmentSnapshotRefs(raw)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if fileID := strings.TrimSpace(item.FileID); fileID != "" {
			result = append(result, fileID)
		}
	}
	return result
}

func parseAttachmentSnapshotRefs(raw string) []attachmentSnapshotRef {
	payload := strings.TrimSpace(raw)
	if payload == "" || payload == "[]" {
		return nil
	}
	var items []attachmentSnapshotRef
	if err := json.Unmarshal([]byte(payload), &items); err != nil {
		return nil
	}
	return items
}

func filterCurrentAttachments(items []AttachmentInput) []AttachmentInput {
	result := make([]AttachmentInput, 0)
	for _, item := range items {
		if item.Current {
			result = append(result, item)
		}
	}
	return result
}

func bindAttachmentMessageRoles(items []AttachmentInput, messages []model.Message) []AttachmentInput {
	if len(items) == 0 || len(messages) == 0 {
		return items
	}
	roles := make(map[string]string)
	for _, message := range messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		for _, fileID := range parseAttachmentSnapshotFileIDs(message.Attachments) {
			if role == "user" || roles[fileID] == "" {
				roles[fileID] = role
			}
		}
	}
	result := append([]AttachmentInput(nil), items...)
	for index := range result {
		result[index].MessageRole = roles[strings.TrimSpace(result[index].FileID)]
	}
	return result
}

func filterAttachmentsByContextMode(items []AttachmentInput, contextMode string) []AttachmentInput {
	result := make([]AttachmentInput, 0)
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.ContextMode), strings.TrimSpace(contextMode)) {
			result = append(result, item)
		}
	}
	return result
}

func isStableTextAttachment(item AttachmentInput) bool {
	if strings.EqualFold(strings.TrimSpace(item.ContextMode), fileContextModeDirectImage) {
		return false
	}
	return strings.TrimSpace(item.ExtractedText) != ""
}

func shouldShowAttachmentProcessTrace(items []AttachmentInput) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.ContextMode), fileContextModeDirectImage) && !item.Current {
			continue
		}
		if item.Current {
			return true
		}
		if !strings.EqualFold(strings.TrimSpace(item.ContextMode), fileContextModeSkipped) {
			return true
		}
	}
	return false
}

func attachmentProcessTraceItems(items []AttachmentInput) []AttachmentInput {
	result := make([]AttachmentInput, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.ContextMode), fileContextModeDirectImage) && !item.Current {
			continue
		}
		result = append(result, item)
	}
	return result
}

func buildConversationFileContextPlan(
	attachments []AttachmentInput,
	fileMode string,
	cfg config.Config,
	capabilityModelName string,
	capabilitiesJSON string,
	ragAvailable bool,
) conversationFileContextPlan {
	plan := conversationFileContextPlan{
		Attachments: make([]AttachmentInput, 0, len(attachments)),
	}
	for _, item := range attachments {
		kind := normalizeAttachmentKind(item.Kind, item.DetectedMIME)
		if kind == "image" && (item.Current || strings.EqualFold(strings.TrimSpace(item.MessageRole), "user")) {
			item.ContextMode = fileContextModeDirectImage
			plan.Attachments = append(plan.Attachments, item)
			plan.FullAttachments = append(plan.FullAttachments, item)
			continue
		}

		useRAG := shouldUseRAGForAttachment(item, fileMode, cfg, capabilityModelName, capabilitiesJSON, ragAvailable)
		if useRAG {
			item.ContextMode = fileContextModeRAG
			plan.Attachments = append(plan.Attachments, item)
			plan.RAGAttachments = append(plan.RAGAttachments, item)
			continue
		}

		canUseFullContext := canUseAttachmentFullContext(item, cfg)
		if fileMode == "rag" && !canRetrieveAttachment(item, ragAvailable) {
			if canUseFullContext {
				item.ContextMode = fileContextModeRAGFallback
				plan.Attachments = append(plan.Attachments, item)
				plan.FullAttachments = append(plan.FullAttachments, item)
				continue
			}
			item.ContextMode = fileContextModeSkipped
			plan.Attachments = append(plan.Attachments, item)
			plan.Skipped = append(plan.Skipped, item)
			continue
		}
		if !canUseFullContext {
			item.ContextMode = fileContextModeSkipped
			plan.Attachments = append(plan.Attachments, item)
			plan.Skipped = append(plan.Skipped, item)
			continue
		}
		item.ContextMode = fileContextModeFull
		plan.Attachments = append(plan.Attachments, item)
		plan.FullAttachments = append(plan.FullAttachments, item)
	}
	return plan
}

func shouldUseRAGForAttachment(item AttachmentInput, fileMode string, cfg config.Config, capabilityModelName string, capabilitiesJSON string, ragAvailable bool) bool {
	if !cfg.RAGEnabled || !cfg.EmbeddingEnabled {
		return false
	}
	if !canRetrieveAttachment(item, ragAvailable) {
		return false
	}
	switch fileMode {
	case "rag":
		return true
	case "full_context":
		return false
	default:
		if !canUseAttachmentFullContext(item, cfg) {
			return true
		}
		if cfg.ContextTokenBudgetEnabled {
			budget := domainchannel.EffectiveContextBudgetFromCapabilitiesWithFallback(capabilityModelName, capabilitiesJSON, cfg.ContextWindowFallbackTokens)
			fileTokens := int(tokenestimate.Estimate(item.ExtractedText))
			return budget > 0 && fileTokens > budget*2/5
		}
		return false
	}
}

func canRetrieveAttachment(item AttachmentInput, ragAvailable bool) bool {
	return ragAvailable &&
		strings.TrimSpace(item.FileID) != "" &&
		!item.RAGOptOut &&
		strings.EqualFold(strings.TrimSpace(item.EmbedStatus), "ready")
}

func fileContextPlanRAGObjects(items []AttachmentInput) []model.FileObject {
	result := make([]model.FileObject, 0, len(items))
	for _, item := range items {
		result = append(result, model.FileObject{
			ID:          item.FileObjID,
			FileID:      item.FileID,
			FileName:    item.FileName,
			EmbedStatus: item.EmbedStatus,
			ChunkCount:  item.ChunkCount,
			UpdatedAt:   item.FileUpdatedAt,
		})
	}
	return result
}

func (s *Service) resolveKnowledgeBaseRAGFiles(
	ctx context.Context,
	userID uint,
	publicIDs []string,
	ragAvailable bool,
) ([]model.FileObject, error) {
	if len(publicIDs) == 0 {
		return nil, nil
	}
	if !s.cfg.Snapshot().KnowledgeBaseEnabled {
		// 知识库功能已被后台关闭：静默忽略引用，保证存量会话仍可正常发送。
		return nil, nil
	}
	if !ragAvailable || s.knowledgeBaseResolver == nil || s.ragSvc == nil {
		return nil, ErrKnowledgeBaseUnavailable
	}
	bases, files, err := s.knowledgeBaseResolver.ResolveFiles(ctx, userID, publicIDs)
	if err != nil {
		if errors.Is(err, domainknowledgebase.ErrReferenceUnavailable) {
			return nil, ErrInvalidKnowledgeBaseReference
		}
		return nil, err
	}
	for _, base := range bases {
		if base.ReadyFileCount == 0 {
			return nil, ErrKnowledgeBaseNotReady
		}
	}
	ready := make([]model.FileObject, 0, len(files))
	seen := make(map[uint]struct{}, len(files))
	for _, file := range files {
		if file.ID == 0 || !file.ProcessingReady || file.RAGOptOut || !strings.EqualFold(strings.TrimSpace(file.EmbedStatus), "ready") || file.ChunkCount <= 0 {
			continue
		}
		if _, exists := seen[file.ID]; exists {
			continue
		}
		seen[file.ID] = struct{}{}
		ready = append(ready, file)
	}
	if len(ready) == 0 {
		return nil, ErrKnowledgeBaseNotReady
	}
	return ready, nil
}

func mergeRAGFileObjects(groups ...[]model.FileObject) []model.FileObject {
	count := 0
	for _, group := range groups {
		count += len(group)
	}
	result := make([]model.FileObject, 0, count)
	seen := make(map[uint]struct{}, count)
	for _, group := range groups {
		for _, item := range group {
			if item.ID == 0 {
				continue
			}
			if _, exists := seen[item.ID]; exists {
				continue
			}
			seen[item.ID] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

func splitRetrievalFallbackAttachments(items []AttachmentInput, cfg config.Config) ([]AttachmentInput, []AttachmentInput) {
	fallbacks := make([]AttachmentInput, 0, len(items))
	skipped := make([]AttachmentInput, 0)
	for _, item := range items {
		if canUseAttachmentFullContext(item, cfg) {
			item.ContextMode = fileContextModeRAGFallback
			fallbacks = append(fallbacks, item)
			continue
		}
		item.ContextMode = fileContextModeSkipped
		skipped = append(skipped, item)
	}
	return fallbacks, skipped
}

func appendRAGFallbackSkippedTrace(traceRecorder *messageTraceRecorder, skipped []AttachmentInput, reason string) {
	if traceRecorder == nil || len(skipped) == 0 {
		return
	}
	names := make([]string, 0, len(skipped))
	for _, item := range skipped {
		name := strings.TrimSpace(item.FileName)
		if name == "" {
			name = strings.TrimSpace(item.FileID)
		}
		if name != "" {
			names = append(names, name)
		}
	}
	traceRecorder.appendProcessSection(
		"部分文件未纳入",
		formatTraceStep(
			"内容检索",
			fmt.Sprintf("%s，文件超出预算或没有可用提取文本，暂未纳入%s。", ragFallbackReasonLabel(reason), traceNameScope(names)),
		),
		&tracePayload{Reason: strings.TrimSpace(reason), FileNames: names, Stages: []traceStage{{Kind: processTraceKindRetrieval, Status: processTraceStatusSkipped, FileCount: len(names)}}},
		messageTraceStatusStreaming,
	)
}

func ragFallbackReasonLabel(reason string) string {
	switch strings.TrimSpace(reason) {
	case "rag_empty":
		return "检索未命中"
	case "rag_low_score":
		return "检索结果低于相似度阈值"
	case "rag_timeout":
		return "检索超时"
	case "rag_unavailable":
		return "检索不可用"
	case "rag_error":
		return "检索失败"
	default:
		return strings.TrimSpace(reason)
	}
}
