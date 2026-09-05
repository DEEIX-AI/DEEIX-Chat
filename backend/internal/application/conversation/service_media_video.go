package conversation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appcm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/contentmoderation"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const maxMediaVideoInputImages = 1

// MediaVideoTaskType 区分普通视频生成与基于源视频的扩展。
type MediaVideoTaskType string

const (
	MediaVideoTaskGeneration MediaVideoTaskType = "video_generation"
	MediaVideoTaskExtension  MediaVideoTaskType = "video_extension"
)

// MediaVideoInput 定义视频生成任务的应用层入参。
type MediaVideoInput struct {
	UserID                uint
	ConversationID        uint
	RequestID             string
	TaskType              MediaVideoTaskType
	Prompt                string
	PlatformModelName     string
	Options               map[string]any
	ClientRunID           string
	FileIDs               []string
	ParentMessagePublicID string
	SourceMessagePublicID string
	BranchReason          string
	// UsageAuthorization 是请求级计费授权；路由确定后据此把预算预留抬高到按时长预估成本。
	UsageAuthorization *domainbilling.UsageAuthorization
	OnEvent            func(eventType string, payload map[string]any) error
}

// StreamMediaVideo 执行视频生成任务并把结果保存为文件对象。
func (s *Service) StreamMediaVideo(ctx context.Context, input MediaVideoInput) (result *SendMessageResult, retErr error) {
	if s.routeResolver == nil || s.llmClient == nil {
		return nil, ErrModelRouteNotConfigured
	}
	runID := normalizeRunID(input.ClientRunID)
	if runID == "" {
		runID = "run_" + normalizePublicID(uuid.NewString())
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
	taskType := normalizeMediaVideoTaskType(input.TaskType)
	routeTaskType := channel.TaskTypeVideoGeneration
	if taskType == MediaVideoTaskExtension {
		routeTaskType = channel.TaskTypeVideoExtension
	}

	platformModelName := strings.TrimSpace(input.PlatformModelName)
	if platformModelName == "" {
		platformModelName = strings.TrimSpace(conversation.Model)
	}
	if platformModelName == "" {
		return nil, ErrModelRouteNotConfigured
	}
	videoEndpoint := llm.EndpointVideoGenerations
	if taskType == MediaVideoTaskExtension {
		videoEndpoint = llm.EndpointVideoExtensions
	}
	run := &model.Run{
		RunID:              runID,
		RequestID:          strings.TrimSpace(input.RequestID),
		UserID:             input.UserID,
		ConversationID:     input.ConversationID,
		TaskType:           routeTaskType,
		Endpoint:           videoEndpoint,
		Provider:           strings.TrimSpace(conversation.Provider),
		RequestedModelName: platformModelName,
		Status:             "running",
		StartedAt:          startedAt,
	}
	if err = s.claimConversationRun(ctx, run); err != nil {
		return nil, err
	}
	var moderationCoord *appcm.RunCoordinator
	var userMessage *model.Message
	var assistantMessage *model.Message
	defer func() {
		if retErr != nil && moderationCoord != nil {
			if result == nil && userMessage != nil && assistantMessage != nil {
				result = &SendMessageResult{
					UserMessage:      *userMessage,
					AssistantMessage: *assistantMessage,
					Billable:         false,
					StartedAt:        startedAt,
				}
			}
			moderationCtx, cancelModeration := background.WithTimeout(ctx, moderationFinalizationTimeout)
			s.completeModerationAfterFailure(moderationCtx, moderationCoord, result)
			cancelModeration()
		}
		endedAt := time.Now()
		run.EndedAt = &endedAt
		run.TotalLatencyMS = endedAt.Sub(startedAt).Milliseconds()
		switch {
		case result != nil && result.IsModerationBlocked():
			applyBlockedRunFields(run, result)
		case retErr == nil:
			run.Status = "success"
			if result != nil {
				applyModerationRunState(run, result)
			}
		case errors.Is(retErr, ErrMessageGenerationCanceled):
			run.Status = "canceled"
			run.ErrorCode = classifyRunErrorCode(retErr)
			run.ErrorMessage = textutil.TruncateTrimmed(messageErrorSummary(retErr), 255)
			if result != nil {
				applyModerationRunState(run, result)
			}
		default:
			run.Status = "error"
			run.ErrorCode = classifyRunErrorCode(retErr)
			run.ErrorMessage = textutil.TruncateTrimmed(messageErrorSummary(retErr), 255)
			if result != nil {
				applyModerationRunState(run, result)
			}
		}
		persistCtx, cancelPersist := background.WithTimeout(ctx, 5*time.Second)
		defer cancelPersist()
		if err := s.repo.UpdateConversationRun(persistCtx, run); err != nil && s.logger != nil {
			s.logger.Error("update_video_conversation_run_failed",
				zap.String("trace_id", traceid.FromContext(ctx)),
				zap.String("run_id", run.RunID),
				zap.Error(err),
			)
		}
	}()
	cancelCtx, cancel := context.WithCancel(ctx)
	ctx = cancelCtx
	if err = s.generationStreams.register(ctx, runID, input.UserID, conversation.PublicID, cancel); err != nil {
		return nil, err
	}

	route, err := s.routeResolver.ResolveRoute(ctx, channel.ResolveRouteInput{
		PlatformModelName: platformModelName,
		TaskType:          routeTaskType,
		Scope:             channel.RouteScopeUser,
		UserID:            input.UserID,
		ConversationID:    input.ConversationID,
		RequestID:         strings.TrimSpace(input.RequestID),
	})
	if err != nil {
		return nil, mapRouteResolutionError(err)
	}
	if !llm.IsVideoGenerationAdapter(route.Protocol) {
		return nil, ErrMediaRouteProtocolMismatch
	}
	if taskType == MediaVideoTaskExtension && llm.NormalizeAdapter(route.Protocol) != llm.AdapterXAIVideoExtensions {
		return nil, ErrMediaRouteProtocolMismatch
	}
	videoEndpoint = llm.DefaultEndpointForAdapter(route.Protocol)
	run.Endpoint = videoEndpoint
	run.ProviderProtocol = route.Protocol
	run.UpstreamID = route.UpstreamID
	run.UpstreamModelID = route.UpstreamModelID
	run.UpstreamName = route.UpstreamName
	run.PlatformModelName = route.PlatformModelName
	run.RoutedBindingCode = route.BindingCode
	run.ModelVendor = route.ModelVendor
	run.ModelIcon = route.ModelIcon
	run.UpstreamModelName = route.UpstreamModel
	if err = s.ensureUsageBudgetCoversEstimate(ctx, input.UsageAuthorization, route, input.Options, usageBudgetEstimate{
		CallCount:       1,
		DurationSeconds: mediaDurationSecondsFromOptions(withDefaultMediaVideoDuration(input.Options, route.Protocol)),
	}); err != nil {
		return nil, err
	}
	if strings.TrimSpace(conversation.Model) != strings.TrimSpace(route.PlatformModelName) {
		conversation.Model = strings.TrimSpace(route.PlatformModelName)
		conversation.Provider = inferProvider(conversation.Model)
		run.Provider = conversation.Provider
		if err = s.repo.UpdateConversationModel(ctx, input.ConversationID, conversation.Model, conversation.Provider); err != nil {
			return nil, err
		}
	}
	resolvedAttachments, videoInputParts, videoExtensionSource, err := s.resolveMediaVideoInputs(ctx, input, taskType)
	if err != nil {
		return nil, err
	}
	attachmentsJSON := marshalAttachmentSnapshots(resolvedAttachments)

	assistantMessage = &model.Message{
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
			TokenUsage:      tokenestimate.Estimate(input.Prompt),
			InputTokens:     tokenestimate.Estimate(input.Prompt),
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
	}
	traceRecorder := newMessageTraceRecorder(s, ctx, assistantMessage, input.OnEvent)
	defer func() {
		if retErr != nil && traceRecorder != nil {
			traceRecorder.fail(retErr)
			traceRecorder.attachToMessage(assistantMessage)
		}
	}()
	moderationFileIDs := make([]string, 0, len(resolvedAttachments))
	if taskType != MediaVideoTaskExtension {
		for _, item := range resolvedAttachments {
			if fileID := strings.TrimSpace(item.FileID); fileID != "" {
				moderationFileIDs = append(moderationFileIDs, fileID)
			}
		}
	}
	moderationCoord = s.startModerationRun(ctx, SendMessageInput{
		UserID:             input.UserID,
		ConversationID:     input.ConversationID,
		RequestID:          input.RequestID,
		Content:            strings.TrimSpace(input.Prompt),
		FileIDs:            moderationFileIDs,
		ClientRunID:        runID,
		OnEvent:            input.OnEvent,
		UsageAuthorization: input.UsageAuthorization,
	}, runID, userMessage, assistantMessage)
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
		Endpoint:            videoEndpoint,
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
	if llm.NormalizeAdapter(route.Protocol) == llm.AdapterXAIVideoExtensions {
		llm.SanitizeXAIVideoExtensionOptions(filteredOptions)
	} else if llm.NormalizeAdapter(route.Protocol) == llm.AdapterXAIVideo {
		llm.SanitizeXAIVideoOptions(filteredOptions)
	}
	filteredOptions = withDefaultMediaVideoDuration(filteredOptions, route.Protocol)
	durationSeconds := mediaDurationSecondsFromOptions(filteredOptions)
	buildFailureResult := func(failure error, usage llm.Usage) *SendMessageResult {
		result := buildFailedMediaBillingResult(failedMediaBillingResultInput{
			UserMessage:      userMessage,
			AssistantMessage: assistantMessage,
			Route:            *route,
			EffectiveOptions: filteredOptions,
			Usage:            usage,
			StartedAt:        startedAt,
			DurationSeconds:  durationSeconds,
			Failure:          failure,
			Billable:         false,
		})
		applyMediaRunUsage(run, result)
		return result
	}

	emitMediaEvent(input.OnEvent, "running", "generating video", "video")
	generateInput := llm.GenerateInput{
		RequestID:      strings.TrimSpace(input.RequestID),
		ConversationID: input.ConversationID,
		Messages: []llm.Message{{
			Role:    "user",
			Content: strings.TrimSpace(input.Prompt),
		}},
		Options:              filteredOptions,
		VideoExtensionSource: videoExtensionSource,
	}
	if len(videoInputParts) > 0 {
		parts := make([]llm.ContentPart, 0, 1+len(videoInputParts))
		parts = append(parts, llm.ContentPart{Kind: llm.ContentPartText, Text: strings.TrimSpace(input.Prompt)})
		parts = append(parts, videoInputParts...)
		generateInput.Messages = []llm.Message{{Role: "user", Parts: parts}}
	}

	output, err := s.llmClient.Generate(ctx, routeConfig, generateInput)
	if err != nil {
		if s.isCanceledMediaGeneration(ctx, runID, err) {
			retErr = ErrMessageGenerationCanceled
			var cancelErr error
			result, cancelErr = s.completeCanceledMediaGeneration(canceledMediaGenerationInput{
				Context:          ctx,
				Conversation:     conversation,
				UserMessage:      userMessage,
				AssistantMessage: assistantMessage,
				ReuseUserMessage: reuseUserMessage,
				Route:            *route,
				EffectiveOptions: filteredOptions,
				GenerateInput:    generateInput,
				StartedAt:        startedAt,
				DurationSeconds:  durationSeconds,
				Billable:         false,
			})
			if cancelErr != nil {
				retErr = cancelErr
				return nil, cancelErr
			}
			applyMediaRunUsage(run, result)
			return result, nil
		}
		s.routeResolver.MarkRouteFailure(ctx, route, err)
		retErr = wrapUpstreamRequestError(err)
		_ = s.repo.UpdateMessageState(ctx, assistantMessage.ID, "error", classifyRunErrorCode(retErr), textutil.TruncateTrimmed(messageErrorSummary(retErr), 255))
		return nil, retErr
	}
	s.routeResolver.MarkRouteSuccess(ctx, route)
	if output == nil || len(output.GeneratedVideos) == 0 {
		retErr = ErrUpstreamEmptyResponse
		_ = s.repo.UpdateMessageState(ctx, assistantMessage.ID, "error", classifyRunErrorCode(retErr), textutil.TruncateTrimmed(messageErrorSummary(retErr), 255))
		return buildFailureResult(retErr, mediaOutputUsage(output)), retErr
	}
	videoDurations, generatedDurationSeconds := resolveGeneratedVideoDurations(output.GeneratedVideos, durationSeconds)
	if taskType != MediaVideoTaskExtension && generatedDurationSeconds > 0 {
		durationSeconds = generatedDurationSeconds
	}

	emitMediaEvent(input.OnEvent, "saving_artifact", "saving video", "video")
	uploaded := make([]model.FileObject, 0, len(output.GeneratedVideos))
	attachmentRows := make([]model.Attachment, 0, len(output.GeneratedVideos))
	now := time.Now()
	for i, video := range output.GeneratedVideos {
		data, mimeType, readErr := s.readGeneratedVideo(ctx, video, route.BaseURL, route.APIKey)
		if readErr != nil {
			retErr = s.finalizeGeneratedMediaArtifactFailure(ctx, run, assistantMessage.ID, i+1, len(output.GeneratedVideos), readErr)
			return buildFailureResult(retErr, output.Usage), retErr
		}
		fileName := generatedVideoFileName(route.PlatformModelName, now, i, len(output.GeneratedVideos), mimeType)
		uploadResult, uploadErr := s.uploadSvc.UploadFile(ctx, appupload.UploadFileInput{
			UserID:       input.UserID,
			Purpose:      "generated_video",
			FileName:     fileName,
			MimeType:     mimeType,
			DeclaredSize: int64(len(data)),
			Reader:       bytes.NewReader(data),
		})
		if uploadErr != nil {
			retErr = uploadErr
			_ = s.repo.UpdateMessageState(ctx, assistantMessage.ID, "error", classifyRunErrorCode(retErr), textutil.TruncateTrimmed(messageErrorSummary(retErr), 255))
			return buildFailureResult(uploadErr, output.Usage), uploadErr
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
			MetaJSON:       generatedVideoAttachmentMetaJSON(videoDurations[i]),
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
		return buildFailureResult(err, output.Usage), err
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
	assistantMessage.Attachments = string(marshalAttachmentSnapshots(videoAttachmentsFromFiles(uploaded, videoDurations)))
	run.InputTokens = usage.InputTokens
	run.OutputTokens = usage.OutputTokens
	run.CacheReadTokens = usage.CacheReadTokens
	run.CacheWriteTokens = usage.CacheWriteTokens
	run.ReasoningTokens = usage.ReasoningTokens

	result = &SendMessageResult{
		UserMessage:         *userMessage,
		AssistantMessage:    *assistantMessage,
		MetadataRefreshHint: s.resolveConversationMetadataRefreshHint(ctx, *conversation, *userMessage),
		Billable:            true,
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
		DurationSeconds:     durationSeconds,
	}
	if moderationCoord != nil {
		// Omni Moderation has no video modality. The prompt and optional input
		// image participate in the barrier; an extension source is intentionally excluded.
		s.completeModerationAfterSuccess(ctx, completeModerationAfterSuccessInput{
			Coordinator:      moderationCoord,
			Result:           result,
			EmbedInput:       SendMessageInput{UserID: input.UserID, ConversationID: input.ConversationID},
			ReuseUserMessage: reuseUserMessage,
		})
	}
	return result, nil
}

func normalizeMediaVideoTaskType(taskType MediaVideoTaskType) MediaVideoTaskType {
	if taskType == MediaVideoTaskExtension {
		return MediaVideoTaskExtension
	}
	return MediaVideoTaskGeneration
}

func mediaVideoUserContentType(hasInputs bool) string {
	if hasInputs {
		return "mixed"
	}
	return "text"
}

func (s *Service) resolveMediaVideoInputs(ctx context.Context, input MediaVideoInput, taskType MediaVideoTaskType) ([]AttachmentInput, []llm.ContentPart, *llm.ContentPart, error) {
	if len(input.FileIDs) == 0 {
		if taskType == MediaVideoTaskExtension {
			return nil, nil, nil, ErrMediaVideoInputInvalid
		}
		return nil, nil, nil, nil
	}
	attachments, err := s.resolveAttachments(ctx, input.UserID, input.FileIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(attachments) > maxMediaVideoInputImages {
		return nil, nil, nil, ErrMediaVideoTooManyInputs
	}
	if taskType == MediaVideoTaskExtension {
		if len(attachments) != 1 {
			return nil, nil, nil, ErrMediaVideoInputInvalid
		}
		part, readErr := s.readMediaVideoExtensionSource(ctx, input.UserID, attachments[0].FileID)
		if readErr != nil {
			return nil, nil, nil, readErr
		}
		return attachments, nil, &part, nil
	}
	parts := make([]llm.ContentPart, 0, len(attachments))
	for _, attachment := range attachments {
		if normalizeAttachmentKind(attachment.Kind, attachment.MimeType) != "image" {
			return nil, nil, nil, ErrMediaVideoInputInvalid
		}
		part, readErr := s.readMediaImageEditFile(ctx, input.UserID, attachment.FileID)
		if readErr != nil {
			return nil, nil, nil, readErr
		}
		part.FileName = mediaImageEditInputFileName(attachment.FileName, part.MimeType)
		parts = append(parts, part)
	}
	return attachments, parts, nil, nil
}

func (s *Service) readMediaVideoExtensionSource(ctx context.Context, userID uint, fileID string) (llm.ContentPart, error) {
	content, err := s.uploadSvc.OpenFileContent(ctx, userID, strings.TrimSpace(fileID))
	if err != nil {
		return llm.ContentPart{}, err
	}
	defer content.Reader.Close() //nolint:errcheck
	limit := s.cfg.Snapshot().MaxUploadFileBytes
	if limit <= 0 {
		limit = 20 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(content.Reader, limit+1))
	if err != nil {
		return llm.ContentPart{}, err
	}
	if int64(len(data)) > limit {
		return llm.ContentPart{}, ErrFileTooLarge
	}
	if detectGeneratedVideoMIME(data) != "video/mp4" {
		return llm.ContentPart{}, ErrMediaVideoInputInvalid
	}
	return llm.ContentPart{Kind: llm.ContentPartVideo, MimeType: "video/mp4", Data: data, FileName: content.File.FileName}, nil
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

func (s *Service) readGeneratedVideo(ctx context.Context, video llm.GeneratedVideo, trustedProviderEndpoint string, apiKey string) ([]byte, string, error) {
	mimeType := strings.TrimSpace(video.MIMEType)
	if mimeType == "" {
		mimeType = "video/mp4"
	}
	if b64 := strings.TrimSpace(video.B64JSON); b64 != "" {
		data, err := base64.StdEncoding.DecodeString(stripBase64DataURLPrefix(b64))
		if err != nil {
			return nil, mimeType, newGeneratedMediaArtifactError("video", "decode", err)
		}
		validated, detectedMIME, validationErr := validateGeneratedVideoBytes(data, mimeType)
		if validationErr != nil {
			return nil, mimeType, newGeneratedMediaArtifactError("video", "validation", validationErr)
		}
		return validated, detectedMIME, nil
	}
	url := strings.TrimSpace(video.URL)
	if url == "" {
		return nil, mimeType, ErrUpstreamEmptyResponse
	}
	if s.mediaDownloader == nil {
		return nil, mimeType, newGeneratedMediaArtifactError("video", "configuration", fmt.Errorf("generated media downloader is not configured"))
	}
	cfg := s.cfg.Snapshot()
	limit := cfg.MaxUploadFileBytes
	if limit <= 0 {
		limit = 20 * 1024 * 1024
	}
	data, downloadedMIME, err := s.mediaDownloader.DownloadVideo(ctx, url, trustedProviderEndpoint, apiKey, limit)
	if err != nil {
		if isMediaArtifactResponseTooLarge(err) {
			return nil, mimeType, ErrFileTooLarge
		}
		return nil, mimeType, newGeneratedMediaArtifactError("video", "download", err)
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(downloadedMIME)), "video/") {
		mimeType = strings.TrimSpace(downloadedMIME)
	}
	validated, detectedMIME, validationErr := validateGeneratedVideoBytes(data, mimeType)
	if validationErr != nil {
		return nil, mimeType, newGeneratedMediaArtifactError("video", "validation", validationErr)
	}
	return validated, detectedMIME, nil
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

func generatedVideoAttachmentMetaJSON(durationSeconds int64) string {
	if durationSeconds <= 0 {
		return ""
	}
	payload, err := json.Marshal(map[string]int64{"duration_seconds": durationSeconds})
	if err != nil {
		return ""
	}
	return string(payload)
}

func videoAttachmentsFromFiles(files []model.FileObject, durations []int64) []AttachmentInput {
	items := make([]AttachmentInput, 0, len(files))
	for index, file := range files {
		durationSeconds := int64(0)
		if index < len(durations) {
			durationSeconds = positiveSeconds(durations[index])
		}
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
			DurationSeconds:  durationSeconds,
		})
	}
	return items
}
