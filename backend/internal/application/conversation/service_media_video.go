package conversation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	maxMediaVideoInputImages        = 1
	geminiGeneratedFilePollAttempts = 12
	geminiGeneratedFilePollInterval = 5 * time.Second
)

// MediaVideoInput 定义视频生成任务的应用层入参。
type MediaVideoInput struct {
	UserID                uint
	ConversationID        uint
	RequestID             string
	Prompt                string
	PlatformModelName     string
	Options               map[string]interface{}
	ClientRunID           string
	FileIDs               []string
	ParentMessagePublicID string
	SourceMessagePublicID string
	BranchReason          string
	OnEvent               func(eventType string, payload map[string]interface{}) error
}

// StreamMediaVideo 执行视频生成任务并把结果保存为文件对象。
func (s *Service) StreamMediaVideo(ctx context.Context, input MediaVideoInput) (*SendMessageResult, error) {
	if s.routeResolver == nil || s.llmClient == nil {
		return nil, ErrModelRouteNotConfigured
	}
	ctx = context.WithoutCancel(ctx)

	runID := normalizeRunID(input.ClientRunID)
	if runID == "" {
		runID = "run_" + normalizePublicID(uuid.NewString())
	}
	existingRuns, err := s.repo.ListConversationRunsByRunIDs(ctx, input.UserID, input.ConversationID, []string{runID})
	if err != nil {
		return nil, err
	}
	if len(existingRuns) > 0 {
		return nil, ErrDuplicateMessageGenerationRun
	}
	startedAt := time.Now()
	conversation, err := s.repo.GetConversationByUser(ctx, input.ConversationID, input.UserID)
	if err != nil {
		return nil, ErrConversationNotFound
	}

	normalizedBranchReason := normalizeBranchReason(input.BranchReason)
	branchState, err := s.resolveMessageBranch(ctx, input.ConversationID, input.UserID, input.ParentMessagePublicID, input.SourceMessagePublicID, normalizedBranchReason)
	if err != nil {
		return nil, err
	}
	reuseUserMessage := branchState.ReuseUserMessage != nil
	if reuseUserMessage {
		input.Prompt = branchState.ReuseUserMessage.Content
		input.FileIDs = parseAttachmentSnapshotFileIDs(branchState.ReuseUserMessage.Attachments)
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return nil, ErrMediaVideoPromptRequired
	}

	platformModelName := strings.TrimSpace(input.PlatformModelName)
	if platformModelName == "" {
		platformModelName = strings.TrimSpace(conversation.Model)
	}
	if platformModelName == "" {
		return nil, ErrModelRouteNotConfigured
	}
	route, err := s.routeResolver.ResolveRoute(ctx, channel.ResolveRouteInput{
		PlatformModelName: platformModelName,
		TaskType:          channel.TaskTypeVideoGeneration,
		Scope:             channel.RouteScopeUser,
		UserID:            input.UserID,
		ConversationID:    input.ConversationID,
		RequestID:         strings.TrimSpace(input.RequestID),
	})
	if err != nil {
		return nil, ErrModelRouteNotConfigured
	}
	if !llm.IsVideoGenerationAdapter(route.Protocol) {
		return nil, ErrMediaRouteProtocolMismatch
	}
	if strings.TrimSpace(conversation.Model) != strings.TrimSpace(route.PlatformModelName) {
		conversation.Model = strings.TrimSpace(route.PlatformModelName)
		conversation.Provider = inferProvider(conversation.Model)
		if err = s.repo.UpdateConversationModel(ctx, input.ConversationID, conversation.Model, conversation.Provider); err != nil {
			return nil, err
		}
	}
	resolvedAttachments, videoInputParts, err := s.resolveMediaVideoInputs(ctx, input)
	if err != nil {
		return nil, err
	}
	attachmentsJSON := marshalAttachmentSnapshots(resolvedAttachments)

	run := &model.Run{
		RunID:              runID,
		RequestID:          strings.TrimSpace(input.RequestID),
		UserID:             input.UserID,
		ConversationID:     input.ConversationID,
		TaskType:           channel.TaskTypeVideoGeneration,
		Endpoint:           llm.EndpointInteractions,
		Provider:           strings.TrimSpace(conversation.Provider),
		ProviderProtocol:   route.Protocol,
		UpstreamID:         route.UpstreamID,
		UpstreamModelID:    route.UpstreamModelID,
		UpstreamName:       route.UpstreamName,
		RequestedModelName: platformModelName,
		PlatformModelName:  route.PlatformModelName,
		RoutedBindingCode:  route.BindingCode,
		ModelVendor:        route.ModelVendor,
		ModelIcon:          route.ModelIcon,
		UpstreamModelName:  route.UpstreamModel,
		Status:             "error",
		StartedAt:          startedAt,
	}
	var retErr error
	defer func() {
		endedAt := time.Now()
		run.EndedAt = &endedAt
		run.TotalLatencyMS = endedAt.Sub(startedAt).Milliseconds()
		if retErr == nil {
			run.Status = "success"
		} else {
			run.Status = "error"
			run.ErrorCode = classifyRunErrorCode(retErr)
			run.ErrorMessage = truncateError(messageErrorSummary(retErr), 255)
		}
		if err := s.repo.CreateConversationRun(context.WithoutCancel(ctx), run); err != nil && s.logger != nil {
			s.logger.Error("create_video_conversation_run_failed",
				zap.String("trace_id", traceid.FromContext(ctx)),
				zap.String("run_id", run.RunID),
				zap.Error(err),
			)
		}
	}()
	cancelCtx, cancel := context.WithCancel(ctx)
	ctx = cancelCtx
	s.generationStreams.register(ctx, runID, input.UserID, cancel)

	assistantMessage := &model.Message{
		ConversationID: input.ConversationID,
		UserID:         input.UserID,
		PublicID:       normalizePublicID(uuid.NewString()),
		RunID:          runID,
		Role:           "assistant",
		ContentType:    "video",
		Content:        "",
		BranchReason:   normalizedBranchReason,
		Status:         "pending",
		Attachments:    "[]",
	}
	var userMessage *model.Message
	if reuseUserMessage {
		reused := *branchState.ReuseUserMessage
		userMessage = &reused
		assistantMessage.ParentMessageID = &userMessage.ID
		assistantMessage.SourceMessageID = branchState.SourceMessageID
		if err = s.repo.CreateAssistantBranchMessage(ctx, assistantMessage); err != nil {
			retErr = err
			return nil, err
		}
		assistantMessage.ParentPublicID = userMessage.PublicID
		assistantMessage.SourcePublicID = branchState.SourcePublicID
	} else {
		userMessage = &model.Message{
			ConversationID:  input.ConversationID,
			UserID:          input.UserID,
			PublicID:        normalizePublicID(uuid.NewString()),
			ParentMessageID: branchState.ParentMessageID,
			RunID:           runID,
			Role:            "user",
			ContentType:     mediaVideoUserContentType(len(resolvedAttachments) > 0),
			Content:         strings.TrimSpace(input.Prompt),
			BranchReason:    normalizedBranchReason,
			SourceMessageID: branchState.SourceMessageID,
			TokenUsage:      estimateTokens(input.Prompt),
			InputTokens:     estimateTokens(input.Prompt),
			Status:          "success",
			Attachments:     attachmentsJSON,
		}
		userAttachmentRows := mediaInputAttachmentRows(input.ConversationID, input.UserID, resolvedAttachments)
		if err = s.repo.CreateMessagePairWithUserAttachments(ctx, userMessage, assistantMessage, userAttachmentRows); err != nil {
			retErr = err
			return nil, err
		}
		userMessage.ParentPublicID = branchState.ParentPublicID
		userMessage.SourcePublicID = branchState.SourcePublicID
		assistantMessage.ParentPublicID = userMessage.PublicID
		s.maybeGenerateConversationMetadataAsync(*conversation, *userMessage)
	}
	traceRecorder := newMessageTraceRecorder(s, ctx, assistantMessage, input.OnEvent)
	defer func() {
		if retErr != nil && traceRecorder != nil {
			traceRecorder.fail(retErr)
			traceRecorder.attachToMessage(assistantMessage)
		}
	}()
	emitMediaEvent(input.OnEvent, "queued", "video task queued", "video")

	cfg := s.cfg.Snapshot()
	attributionReferer, attributionTitle := s.llmAttribution()
	routeConfig := llm.RouteConfig{
		Protocol:            route.Protocol,
		BaseURL:             route.BaseURL,
		APIKey:              route.APIKey,
		HeadersJSON:         route.HeadersJSON,
		ConnectTimeoutMS:    route.ConnectTimeoutMS,
		ReadTimeoutMS:       route.ReadTimeoutMS,
		StreamIdleTimeoutMS: route.StreamIdleTimeoutMS,
		Endpoint:            llm.EndpointInteractions,
		UpstreamModel:       route.UpstreamModel,
		AttributionReferer:  attributionReferer,
		AttributionTitle:    attributionTitle,
	}
	filteredOptions := filterModelOptions(input.Options, route.Protocol, modelOptionPolicyConfig{
		Mode:                  cfg.ModelOptionPolicyMode,
		AllowedPathsJSON:      cfg.ModelOptionAllowedPaths,
		DeniedPathsJSON:       cfg.ModelOptionDeniedPaths,
		ModelCapabilitiesJSON: route.ModelCapabilitiesJSON,
	})
	if llm.NormalizeAdapter(route.Protocol) == llm.AdapterGeminiInteractions {
		filteredOptions = withGeminiInteractionResponseType(filteredOptions, "video")
	}

	emitMediaEvent(input.OnEvent, "running", "generating video", "video")
	generateInput := llm.GenerateInput{
		RequestID:      strings.TrimSpace(input.RequestID),
		ConversationID: input.ConversationID,
		Messages: []llm.Message{{
			Role:    "user",
			Content: strings.TrimSpace(input.Prompt),
		}},
		Options: filteredOptions,
	}
	if len(videoInputParts) > 0 {
		parts := make([]llm.ContentPart, 0, 1+len(videoInputParts))
		parts = append(parts, llm.ContentPart{Kind: llm.ContentPartText, Text: strings.TrimSpace(input.Prompt)})
		parts = append(parts, videoInputParts...)
		generateInput.Messages = []llm.Message{{Role: "user", Parts: parts}}
	}

	output, err := s.llmClient.Generate(ctx, routeConfig, generateInput)
	if err != nil {
		s.routeResolver.MarkRouteFailure(ctx, route, err)
		retErr = wrapUpstreamRequestError(err)
		_ = s.repo.UpdateMessageState(ctx, assistantMessage.ID, "error", classifyRunErrorCode(retErr), truncateError(messageErrorSummary(retErr), 255))
		return nil, retErr
	}
	s.routeResolver.MarkRouteSuccess(ctx, route)
	if output == nil || len(output.GeneratedVideos) == 0 {
		retErr = ErrUpstreamEmptyResponse
		_ = s.repo.UpdateMessageState(ctx, assistantMessage.ID, "error", classifyRunErrorCode(retErr), truncateError(messageErrorSummary(retErr), 255))
		return nil, retErr
	}

	emitMediaEvent(input.OnEvent, "saving_artifact", "saving video", "video")
	uploaded := make([]model.FileObject, 0, len(output.GeneratedVideos))
	attachmentRows := make([]model.Attachment, 0, len(output.GeneratedVideos))
	now := time.Now()
	for i, video := range output.GeneratedVideos {
		data, mimeType, readErr := s.readGeneratedVideo(ctx, video, route.APIKey)
		if readErr != nil {
			retErr = readErr
			_ = s.repo.UpdateMessageState(ctx, assistantMessage.ID, "error", classifyRunErrorCode(retErr), truncateError(messageErrorSummary(retErr), 255))
			return nil, readErr
		}
		fileName := generatedVideoFileName(route.PlatformModelName, now, i, len(output.GeneratedVideos), mimeType)
		uploadResult, uploadErr := s.UploadFile(ctx, appupload.UploadFileInput{
			UserID:       input.UserID,
			Purpose:      "generated_video",
			FileName:     fileName,
			MimeType:     mimeType,
			DeclaredSize: int64(len(data)),
			Reader:       bytes.NewReader(data),
		})
		if uploadErr != nil {
			retErr = uploadErr
			_ = s.repo.UpdateMessageState(ctx, assistantMessage.ID, "error", classifyRunErrorCode(retErr), truncateError(messageErrorSummary(retErr), 255))
			return nil, uploadErr
		}
		file := uploadResult.File
		uploaded = append(uploaded, file)
		attachmentRows = append(attachmentRows, model.Attachment{
			ConversationID: input.ConversationID,
			MessageID:      assistantMessage.ID,
			UserID:         input.UserID,
			FileID:         file.FileID,
			Kind:           "file",
			FileName:       file.FileName,
			MimeType:       file.DetectedMIME,
			FileSize:       file.SizeBytes,
			SHA256:         file.SHA256,
			StoragePath:    file.StoragePath,
			Status:         "active",
			UploadedAt:     now,
		})
	}

	usage := output.Usage
	if reuseUserMessage {
		assistantMessage.InputTokens = usage.InputTokens
		assistantMessage.CacheReadTokens = usage.CacheReadTokens
		assistantMessage.CacheWriteTokens = usage.CacheWriteTokens
	} else {
		userMessage.InputTokens = usage.InputTokens
		userMessage.CacheReadTokens = usage.CacheReadTokens
		userMessage.CacheWriteTokens = usage.CacheWriteTokens
		userMessage.TokenUsage = usage.InputTokens + usage.CacheReadTokens + usage.CacheWriteTokens
	}

	content := generatedVideoMarkdown(uploaded)
	latencyMS := time.Since(startedAt).Milliseconds()
	if reuseUserMessage {
		err = s.repo.CompleteAssistantMessageWithGeneratedAttachments(ctx,
			assistantMessage.ID,
			repository.AssistantMessageCompletionUpdate{
				ContentType:      "video",
				Content:          content,
				InputTokens:      usage.InputTokens,
				OutputTokens:     usage.OutputTokens,
				CacheReadTokens:  usage.CacheReadTokens,
				CacheWriteTokens: usage.CacheWriteTokens,
				ReasoningTokens:  usage.ReasoningTokens,
				LatencyMS:        latencyMS,
				Status:           "success",
			},
			attachmentRows,
		)
	} else {
		err = s.repo.CompleteAssistantMessageWithAttachments(ctx,
			userMessage.ID,
			repository.MessageUsageUpdate{
				InputTokens:      usage.InputTokens,
				CacheReadTokens:  usage.CacheReadTokens,
				CacheWriteTokens: usage.CacheWriteTokens,
			},
			assistantMessage.ID,
			repository.AssistantMessageCompletionUpdate{
				ContentType:     "video",
				Content:         content,
				OutputTokens:    usage.OutputTokens,
				ReasoningTokens: usage.ReasoningTokens,
				LatencyMS:       latencyMS,
				Status:          "success",
			},
			attachmentRows,
		)
	}
	if err != nil {
		retErr = err
		return nil, err
	}

	assistantMessage.Content = content
	assistantMessage.OutputTokens = usage.OutputTokens
	assistantMessage.ReasoningTokens = usage.ReasoningTokens
	assistantMessage.TokenUsage = usage.OutputTokens + usage.ReasoningTokens
	if reuseUserMessage {
		assistantMessage.TokenUsage += usage.InputTokens + usage.CacheReadTokens + usage.CacheWriteTokens
	}
	assistantMessage.LatencyMS = latencyMS
	assistantMessage.Status = "success"
	assistantMessage.Attachments = string(marshalAttachmentSnapshots(videoAttachmentsFromFiles(uploaded)))
	run.InputTokens = usage.InputTokens
	run.OutputTokens = usage.OutputTokens
	run.CacheReadTokens = usage.CacheReadTokens
	run.CacheWriteTokens = usage.CacheWriteTokens
	run.ReasoningTokens = usage.ReasoningTokens

	return &SendMessageResult{
		UserMessage:         *userMessage,
		AssistantMessage:    *assistantMessage,
		MetadataRefreshHint: conversationMetadataRefreshHint(*conversation, *userMessage),
		UpstreamID:          route.UpstreamID,
		UpstreamName:        route.UpstreamName,
		PlatformModelName:   route.PlatformModelName,
		RoutedBindingCode:   route.BindingCode,
		UpstreamModelName:   route.UpstreamModel,
		UpstreamProtocol:    route.Protocol,
		EffectiveOptions:    filteredOptions,
		UsageSpeed:          usage.Speed,
		UsageServiceTier:    usage.ServiceTier,
		RawUsageJSON:        usage.RawUsageJSON,
		CacheWrite5mTokens:  usage.CacheWrite5mTokens,
		CacheWrite1hTokens:  usage.CacheWrite1hTokens,
		StartedAt:           startedAt,
		LatencyMS:           latencyMS,
	}, nil
}

func mediaVideoUserContentType(hasInputs bool) string {
	if hasInputs {
		return "mixed"
	}
	return "text"
}

func (s *Service) resolveMediaVideoInputs(ctx context.Context, input MediaVideoInput) ([]AttachmentInput, []llm.ContentPart, error) {
	if len(input.FileIDs) == 0 {
		return nil, nil, nil
	}
	attachments, err := s.resolveAttachments(ctx, input.UserID, input.FileIDs)
	if err != nil {
		return nil, nil, err
	}
	if len(attachments) > maxMediaVideoInputImages {
		return nil, nil, ErrMediaVideoTooManyInputs
	}
	parts := make([]llm.ContentPart, 0, len(attachments))
	for _, attachment := range attachments {
		if normalizeAttachmentKind(attachment.Kind, attachment.MimeType) != "image" {
			return nil, nil, ErrMediaVideoInputInvalid
		}
		part, readErr := s.readMediaImageEditFile(ctx, input.UserID, attachment.FileID)
		if readErr != nil {
			return nil, nil, readErr
		}
		part.FileName = mediaImageEditInputFileName(attachment.FileName, part.MimeType)
		parts = append(parts, part)
	}
	return attachments, parts, nil
}

func mediaInputAttachmentRows(conversationID uint, userID uint, attachments []AttachmentInput) []model.Attachment {
	rows := make([]model.Attachment, 0, len(attachments))
	now := time.Now()
	for _, item := range attachments {
		rows = append(rows, model.Attachment{
			ConversationID: conversationID,
			UserID:         userID,
			FileID:         strings.TrimSpace(item.FileID),
			Kind:           normalizeAttachmentKind(item.Kind, item.MimeType),
			FileName:       strings.TrimSpace(item.FileName),
			MimeType:       strings.TrimSpace(item.MimeType),
			FileSize:       item.FileSize,
			SHA256:         strings.TrimSpace(item.SHA256),
			StoragePath:    strings.TrimSpace(item.StoragePath),
			Status:         "active",
			MetaJSON:       strings.TrimSpace(item.MetaJSON),
			UploadedAt:     now,
		})
	}
	return rows
}

func (s *Service) readGeneratedVideo(ctx context.Context, video llm.GeneratedVideo, apiKey string) ([]byte, string, error) {
	mimeType := strings.TrimSpace(video.MIMEType)
	if mimeType == "" {
		mimeType = "video/mp4"
	}
	if b64 := strings.TrimSpace(video.B64JSON); b64 != "" {
		data, err := base64.StdEncoding.DecodeString(stripBase64DataURLPrefix(b64))
		if err != nil {
			return nil, mimeType, err
		}
		return validateGeneratedVideoBytes(data, mimeType)
	}
	url := strings.TrimSpace(video.URL)
	if url == "" {
		return nil, mimeType, ErrUpstreamEmptyResponse
	}
	metadataURL, downloadURL, geminiFileDownload := geminiGeneratedFileURLs(url)
	if geminiFileDownload {
		resolvedMIME, resolveErr := s.waitGeminiGeneratedVideoFileReady(ctx, metadataURL, apiKey)
		if resolveErr != nil {
			return nil, mimeType, resolveErr
		}
		url = downloadURL
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(resolvedMIME)), "video/") {
			mimeType = strings.TrimSpace(resolvedMIME)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, mimeType, err
	}
	if geminiFileDownload {
		req.Header.Set("x-goog-api-key", strings.TrimSpace(apiKey))
	}
	cfg := s.cfg.Snapshot()
	client := security.NewOutboundHTTPClient(cfg.Env, cfg.SSRFProtectionEnabled, 120*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, mimeType, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mimeType, fmt.Errorf("download generated video failed: HTTP %d", resp.StatusCode)
	}
	if contentType := strings.TrimSpace(resp.Header.Get("Content-Type")); strings.HasPrefix(strings.ToLower(contentType), "video/") {
		mimeType = strings.Split(contentType, ";")[0]
	}
	limit := cfg.MaxUploadFileBytes
	if limit <= 0 {
		limit = 20 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, mimeType, err
	}
	if int64(len(data)) > limit {
		return nil, mimeType, ErrFileTooLarge
	}
	return validateGeneratedVideoBytes(data, mimeType)
}

func (s *Service) waitGeminiGeneratedVideoFileReady(ctx context.Context, metadataURL string, apiKey string) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("Gemini Files generated video URI requires an API key")
	}
	cfg := s.cfg.Snapshot()
	client := security.NewOutboundHTTPClient(cfg.Env, cfg.SSRFProtectionEnabled, 30*time.Second)
	mimeType, err := pollGeminiGeneratedFileReady(ctx, client, metadataURL, apiKey)
	if err != nil {
		return "", err
	}
	return mimeType, nil
}

func isGeminiFilesURL(rawURL string) bool {
	_, _, ok := geminiGeneratedFileURLs(rawURL)
	return ok
}

func geminiGeneratedFileURLs(rawURL string) (string, string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || !isGeminiFilesHost(parsed.Hostname()) {
		return "", "", false
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for index, segment := range segments {
		if !strings.EqualFold(segment, "files") || index+1 >= len(segments) {
			continue
		}
		fileID := strings.TrimSpace(segments[index+1])
		if colon := strings.Index(fileID, ":"); colon >= 0 {
			fileID = fileID[:colon]
		}
		if fileID == "" {
			return "", "", false
		}
		fileSegments := append([]string(nil), segments[:index+2]...)
		fileSegments[index+1] = fileID

		metadata := *parsed
		metadata.Path = "/" + strings.Join(fileSegments, "/")
		metadata.RawPath = ""
		metadata.RawQuery = ""
		metadata.Fragment = ""

		download := metadata
		download.Path = metadata.Path + ":download"
		download.RawQuery = "alt=media"
		return metadata.String(), download.String(), true
	}
	return "", "", false
}

func isGeminiFilesHost(host string) bool {
	normalized := strings.ToLower(strings.TrimSpace(host))
	return normalized == "generativelanguage.googleapis.com"
}

func pollGeminiGeneratedFileReady(ctx context.Context, client *http.Client, metadataURL string, apiKey string) (string, error) {
	lastState := ""
	for attempt := 0; attempt < geminiGeneratedFilePollAttempts; attempt++ {
		state, mimeType, err := fetchGeminiGeneratedFileState(ctx, client, metadataURL, apiKey)
		if err != nil {
			return "", err
		}
		if geminiGeneratedFileReady(state) {
			return mimeType, nil
		}
		lastState = state
		if geminiGeneratedFileFailed(state) {
			return "", fmt.Errorf("generated video file failed: %s", state)
		}
		if attempt == geminiGeneratedFilePollAttempts-1 {
			break
		}
		timer := time.NewTimer(geminiGeneratedFilePollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
	return "", fmt.Errorf("generated video file is not ready: %s", strings.TrimSpace(lastState))
}

func fetchGeminiGeneratedFileState(ctx context.Context, client *http.Client, metadataURL string, apiKey string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("x-goog-api-key", strings.TrimSpace(apiKey))
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("poll generated video file failed: HTTP %d: %s", resp.StatusCode, truncateError(string(body), 512))
	}
	payload := map[string]interface{}{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", err
	}
	file := mapFromAny(payload["file"])
	state := firstNonEmptyString(
		getStringFromAny(payload["state"]),
		getStringFromAny(file["state"]),
	)
	mimeType := firstNonEmptyString(
		getStringFromAny(payload["mimeType"]),
		getStringFromAny(payload["mime_type"]),
		getStringFromAny(file["mimeType"]),
		getStringFromAny(file["mime_type"]),
	)
	return state, mimeType, nil
}

func mapFromAny(raw interface{}) map[string]interface{} {
	if payload, ok := raw.(map[string]interface{}); ok {
		return payload
	}
	return map[string]interface{}{}
}

func geminiGeneratedFileReady(state string) bool {
	return strings.ToUpper(strings.TrimSpace(state)) == "ACTIVE"
}

func geminiGeneratedFileFailed(state string) bool {
	return strings.ToUpper(strings.TrimSpace(state)) == "FAILED"
}

func validateGeneratedVideoBytes(data []byte, declaredMIME string) ([]byte, string, error) {
	detected := detectGeneratedVideoMIME(data)
	if detected == "" {
		return nil, strings.TrimSpace(declaredMIME), ErrMediaVideoInputInvalid
	}
	return data, detected, nil
}

func detectGeneratedVideoMIME(data []byte) string {
	if len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")) {
		return "video/mp4"
	}
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{0x1a, 0x45, 0xdf, 0xa3}) {
		return "video/webm"
	}
	return ""
}

func generatedVideoFileName(modelName string, capturedAt time.Time, index int, total int, mimeType string) string {
	base := sanitizeGeneratedImageFileBase(modelName)
	if base == "image" {
		base = "video"
	}
	timestamp := fmt.Sprintf("%s-%03d", capturedAt.Format("20060102-150405"), capturedAt.Nanosecond()/int(time.Millisecond))
	if total > 1 {
		return fmt.Sprintf("%s-%s-%02d%s", base, timestamp, index+1, videoFileExtension(mimeType))
	}
	return fmt.Sprintf("%s-%s%s", base, timestamp, videoFileExtension(mimeType))
}

func videoFileExtension(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "video/webm":
		return ".webm"
	default:
		return ".mp4"
	}
}

func generatedVideoMarkdown(files []model.FileObject) string {
	blocks := make([]string, 0, len(files))
	for i, file := range files {
		label := "Generated video"
		if len(files) > 1 {
			label = fmt.Sprintf("Generated video %d", i+1)
		}
		blocks = append(blocks, fmt.Sprintf("[%s](/api/v1/files/%s/content)", label, file.FileID))
	}
	return strings.Join(blocks, "\n\n")
}

func videoAttachmentsFromFiles(files []model.FileObject) []AttachmentInput {
	items := make([]AttachmentInput, 0, len(files))
	for _, file := range files {
		items = append(items, AttachmentInput{
			FileObjID:        file.ID,
			FileID:           file.FileID,
			Kind:             "file",
			FileName:         file.FileName,
			MimeType:         file.MimeType,
			DetectedMIME:     file.DetectedMIME,
			FileCategory:     file.FileCategory,
			FileSize:         file.SizeBytes,
			SHA256:           file.SHA256,
			StoragePath:      file.StoragePath,
			ProcessingStatus: file.ProcessingStatus,
			ProcessingReady:  file.ProcessingReady,
		})
	}
	return items
}
