package conversation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"io"
	"strings"
	"time"

	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/objectstore"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/toolresult"
)

const (
	maxImageAttachmentAnalysisChars      = 6000
	maxImageAttachmentAnalysisTotalChars = 16000
)

type imageAttachmentAnalysis struct {
	FileID   string
	FileName string
	ToolName string
	Content  string
}

type imageAttachmentProcessingInput struct {
	UserID         uint
	ConversationID uint
	MessageID      uint
	RequestID      string
	RunID          string
	UserPrompt     string
	Attachments    []AttachmentInput
	Runtime        selectedToolRuntime
	TraceRecorder  *messageTraceRecorder
}

type imageAttachmentProcessingResult struct {
	Routed                bool
	Analyses              []imageAttachmentAnalysis
	Rows                  []domainconversation.ToolCall
	PersistedToolCallKeys map[string]struct{}
	// MCPToolUsage 聚合附件处理器成功的 MCP 调用，与工具循环的计量口径一致。
	MCPToolUsage []MCPToolUsageItem
}

func (s *Service) processImageAttachments(
	ctx context.Context,
	input imageAttachmentProcessingInput,
) (imageAttachmentProcessingResult, error) {
	processor := input.Runtime.attachmentProcessor
	images := currentImageAttachments(input.Attachments)
	if processor == nil || len(images) == 0 {
		return imageAttachmentProcessingResult{}, nil
	}
	result := imageAttachmentProcessingResult{
		Routed:                true,
		Analyses:              make([]imageAttachmentAnalysis, 0, len(images)),
		Rows:                  make([]domainconversation.ToolCall, 0, len(images)),
		PersistedToolCallKeys: make(map[string]struct{}, len(images)),
	}
	if len(images) > s.resolveMaxToolCallsPerRun() {
		return result, fmt.Errorf("%w: image count exceeds the tool call limit", ErrImageAttachmentProcessingFailed)
	}
	if processor.argument == "" ||
		(processor.encoding != domainmcp.AttachmentEncodingBase64 && processor.encoding != domainmcp.AttachmentEncodingDataURL) {
		return result, fmt.Errorf("%w: processor configuration is invalid", ErrImageAttachmentProcessingFailed)
	}

	cfg := s.cfg.Snapshot()
	storeProvider := s.storeProvider
	if storeProvider == nil {
		return result, fmt.Errorf("%w: open object storage: %w", ErrImageAttachmentProcessingFailed, appstorage.ErrProviderNotConfigured)
	}
	store, err := storeProvider.Open(ctx)
	if err != nil {
		return result, fmt.Errorf("%w: open object storage: %w", ErrImageAttachmentProcessingFailed, err)
	}

	totalImageBytes := 0
	analysisCharLimit := min(maxImageAttachmentAnalysisChars, maxImageAttachmentAnalysisTotalChars/len(images))
	for index, attachment := range images {
		prepared, prepareErr := prepareImageAttachmentForProcessor(ctx, store, attachment, cfg.ImageMaxDimension)
		if prepareErr != nil {
			return result, prepareErr
		}
		totalImageBytes += len(prepared.data)
		if totalImageBytes > maxConversationImageContextBytes {
			return result, fmt.Errorf("%w: image attachment context exceeds %d bytes", ErrFileTooLarge, maxConversationImageContextBytes)
		}

		encodedImage := base64.StdEncoding.EncodeToString(prepared.data)
		if processor.encoding == domainmcp.AttachmentEncodingDataURL {
			encodedImage = "data:" + prepared.mimeType + ";base64," + encodedImage
		}
		arguments := map[string]any{processor.argument: encodedImage}
		if processor.promptArgument != "" {
			arguments[processor.promptArgument] = strings.TrimSpace(input.UserPrompt)
		}
		argumentsJSON, marshalErr := json.Marshal(arguments)
		if marshalErr != nil {
			return result, fmt.Errorf("%w: encode processor arguments: %w", ErrImageAttachmentProcessingFailed, marshalErr)
		}
		normalizedArguments, validationErr := normalizeToolArguments(string(argumentsJSON), input.Runtime.schemas[processor.modelName])
		row := domainconversation.ToolCall{
			MessageID:      input.MessageID,
			ConversationID: input.ConversationID,
			UserID:         input.UserID,
			RunID:          input.RunID,
			ToolCallID:     fmt.Sprintf("attachment_%s_%d", input.RunID, index+1),
			ToolType:       "mcp_attachment",
			ToolName:       processor.toolName,
			Status:         "requested",
			InputJSON:      imageAttachmentAuditInput(attachment, prepared.mimeType, processor.encoding, len(prepared.data)),
		}
		if validationErr != nil {
			row.Status = "error"
			row.ErrorJSON = validationErr.Error()
			s.persistImageAttachmentToolRow(ctx, &row, &result)
			return result, fmt.Errorf("%w: %w", ErrImageAttachmentProcessingFailed, validationErr)
		}

		binding, ok := input.Runtime.mcpBindings[processor.modelName]
		if !ok {
			row.Status = "error"
			row.ErrorJSON = "processor is not enabled for this run"
			s.persistImageAttachmentToolRow(ctx, &row, &result)
			return result, fmt.Errorf("%w: processor is not enabled", ErrImageAttachmentProcessingFailed)
		}
		row.MCPServerID = binding.ServerID
		row.MCPServerName = binding.ServerName
		startedAt := time.Now()
		output, executeErr := s.executeToolCall(ctx, ExecuteToolInput{
			UserID:         input.UserID,
			ConversationID: input.ConversationID,
			RequestID:      input.RequestID,
			ToolName:       processor.toolName,
			ArgumentsJSON:  normalizedArguments,
			MCPConfig:      &binding.Config,
		})
		row.LatencyMS = max(time.Since(startedAt).Milliseconds(), 0)
		if executeErr != nil {
			row.Status = "error"
			row.ErrorJSON = toolresult.SanitizeOpaque(executeErr.Error())
			s.persistImageAttachmentToolRow(ctx, &row, &result)
			return result, fmt.Errorf("%w: %w", ErrImageAttachmentProcessingFailed, executeErr)
		}
		row.OutputJSON = toolresult.SanitizeOpaque(output)
		if row.OutputJSON == "" {
			row.OutputJSON = "{}"
		}
		// 上游调用已成功并产生费用，即使后续解析失败也应计量。
		result.MCPToolUsage = mergeMCPToolUsage(result.MCPToolUsage, []MCPToolUsageItem{{
			ServerID:     binding.ServerID,
			ServerName:   binding.ServerName,
			ToolName:     binding.ToolName,
			CallCount:    1,
			PriceNanousd: binding.PriceNanousd,
		}})
		analysis := imageAttachmentAnalysisText(output)
		if analysis == "" {
			row.Status = "error"
			row.ErrorJSON = "processor returned no textual analysis"
			s.persistImageAttachmentToolRow(ctx, &row, &result)
			return result, fmt.Errorf("%w: processor returned no textual analysis", ErrImageAttachmentProcessingFailed)
		}
		row.Status = "success"
		s.persistImageAttachmentToolRow(ctx, &row, &result)
		analysis = contextArtifactExcerpt(analysis, analysisCharLimit)
		result.Analyses = append(result.Analyses, imageAttachmentAnalysis{
			FileID:   strings.TrimSpace(attachment.FileID),
			FileName: textutil.FirstNonEmpty(attachment.FileName, attachment.FileID),
			ToolName: processor.displayName,
			Content:  analysis,
		})
	}

	if input.TraceRecorder != nil {
		fileNames := make([]string, 0, len(result.Analyses))
		for _, analysis := range result.Analyses {
			fileNames = append(fileNames, analysis.FileName)
		}
		input.TraceRecorder.appendProcessSection(
			fmt.Sprintf("已通过 %s 处理 %d 张图片", processor.displayName, len(result.Analyses)),
			formatTraceStep("图片附件", fmt.Sprintf("图片已交由 %s 分析，主模型仅接收分析结果。", processor.displayName)),
			&tracePayload{ToolID: processor.toolID, ToolName: processor.toolName, FileNames: fileNames, Stages: []traceStage{{Kind: "mcp_attachment_processor", Status: messageTraceStatusCompleted, FileCount: len(result.Analyses)}}},
			messageTraceStatusCompleted,
		)
	}
	return result, nil
}

type preparedImageAttachment struct {
	data     []byte
	mimeType string
}

func prepareImageAttachmentForProcessor(
	ctx context.Context,
	store objectstore.Store,
	attachment AttachmentInput,
	maxDimension int,
) (preparedImageAttachment, error) {
	storagePath := strings.TrimSpace(attachment.StoragePath)
	if storagePath == "" {
		return preparedImageAttachment{}, fmt.Errorf("%w: image storage path is empty", ErrInvalidFileReference)
	}
	reader, _, err := store.Open(ctx, storagePath)
	if err != nil {
		return preparedImageAttachment{}, fmt.Errorf("%w: open image %s: %w", ErrFileNotFound, attachment.FileID, err)
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, maxConversationImageSourceBytes+1))
	closeErr := reader.Close()
	if readErr != nil {
		return preparedImageAttachment{}, fmt.Errorf("%w: read image %s: %w", ErrFileNotFound, attachment.FileID, readErr)
	}
	if closeErr != nil {
		return preparedImageAttachment{}, fmt.Errorf("%w: close image %s: %w", ErrFileNotFound, attachment.FileID, closeErr)
	}
	if len(data) == 0 {
		return preparedImageAttachment{}, fmt.Errorf("%w: image %s is empty", ErrInvalidFileReference, attachment.FileID)
	}
	if len(data) > maxConversationImageSourceBytes {
		return preparedImageAttachment{}, fmt.Errorf("%w: image %s exceeds source limit", ErrFileTooLarge, attachment.FileID)
	}
	if maxDimension <= 0 {
		maxDimension = 1024
	}
	mimeType := resolveImageMimeType(textutil.FirstNonEmpty(attachment.DetectedMIME, attachment.MimeType))
	resized, actualMIME := resizeImageIfNeeded(data, mimeType, maxDimension)
	return preparedImageAttachment{data: resized, mimeType: actualMIME}, nil
}

func currentImageAttachments(attachments []AttachmentInput) []AttachmentInput {
	result := make([]AttachmentInput, 0)
	for _, attachment := range attachments {
		mimeType := textutil.FirstNonEmpty(attachment.DetectedMIME, attachment.MimeType)
		if attachment.Current && normalizeAttachmentKind(attachment.Kind, mimeType) == "image" {
			result = append(result, attachment)
		}
	}
	return result
}

func imageAttachmentAuditInput(attachment AttachmentInput, mimeType string, encoding string, byteSize int) string {
	payload, _ := json.Marshal(map[string]any{
		"file_id":   strings.TrimSpace(attachment.FileID),
		"file_name": strings.TrimSpace(attachment.FileName),
		"mime_type": strings.TrimSpace(mimeType),
		"encoding":  strings.TrimSpace(encoding),
		"byte_size": byteSize,
	})
	return string(payload)
}

func (s *Service) persistImageAttachmentToolRow(
	ctx context.Context,
	row *domainconversation.ToolCall,
	result *imageAttachmentProcessingResult,
) {
	if row == nil || result == nil {
		return
	}
	persisted := s.persistToolCallResult(ctx, row)
	result.Rows = append(result.Rows, *row)
	if persisted {
		result.PersistedToolCallKeys[toolCallPersistenceKey(*row)] = struct{}{}
	}
}

func imageAttachmentAnalysisText(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	var payload struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent any `json:"structuredContent"`
	}
	if err := json.Unmarshal([]byte(value), &payload); err == nil {
		parts := make([]string, 0, len(payload.Content))
		for _, item := range payload.Content {
			if text := strings.TrimSpace(item.Text); text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
		if payload.StructuredContent != nil {
			if encoded, err := json.Marshal(payload.StructuredContent); err == nil {
				return strings.TrimSpace(modelToolOutputForModel(string(encoded)))
			}
		}
		return ""
	}
	return strings.TrimSpace(modelToolOutputForModel(value))
}

func withoutCurrentImageAttachments(plan conversationFileContextPlan) conversationFileContextPlan {
	filter := func(items []AttachmentInput) []AttachmentInput {
		result := make([]AttachmentInput, 0, len(items))
		for _, item := range items {
			mimeType := textutil.FirstNonEmpty(item.DetectedMIME, item.MimeType)
			if item.Current && normalizeAttachmentKind(item.Kind, mimeType) == "image" {
				continue
			}
			result = append(result, item)
		}
		return result
	}
	plan.Attachments = filter(plan.Attachments)
	plan.FullAttachments = filter(plan.FullAttachments)
	return plan
}
