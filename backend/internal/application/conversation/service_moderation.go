package conversation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appcm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/contentmoderation"
	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/objectstorage"
)

const moderationFinalizationTimeout = 65 * time.Second

// MessageModerationOutcome 是一轮对话的软审核最终状态。
// nil 表示无需审核（策略关闭 / 无协调器）。
type MessageModerationOutcome struct {
	Blocked    bool
	EventID    string
	Direction  string
	Categories []string
	// State 是审核目标状态：passed | failed_open | blocked。已知拦截可能
	// 通过审核补偿循环持久地收敛。
	State string
	// 当 moderation_blocked 已推送到流时，TerminalEmitted 为 true。
	TerminalEmitted bool
}

type completeModerationAfterSuccessInput struct {
	Coordinator      *appcm.RunCoordinator
	Result           *SendMessageResult
	OutputText       string
	OutputImages     []appcm.OutputImageSource
	EmbedInput       SendMessageInput
	ReuseUserMessage bool
}

// IsModerationBlocked 报告该轮次是否在安全检查后被拦截。
func (r *SendMessageResult) IsModerationBlocked() bool {
	return r != nil && r.Moderation != nil && r.Moderation.Blocked
}

// ModerationTerminalEmitted 报告 moderation_blocked 是否已在流上发出。
func (r *SendMessageResult) ModerationTerminalEmitted() bool {
	return r != nil && r.Moderation != nil && r.Moderation.TerminalEmitted
}

// ModerationBlockedBilledReason 返回被拦截运行仍要结算的原因，用于拦截事件与账单快照。
// 拦截只撤回内容，不撤回上游已产生的用量：持有预算预留的付费调用照常结算；self 模式与
// 免费模型本就不产生费用，未产生可计费用量的拦截也不结算，两者都不标注。
func ModerationBlockedBilledReason(result *SendMessageResult, authorization *domainbilling.UsageAuthorization) string {
	if !result.IsModerationBlocked() || !result.Billable {
		return ""
	}
	return paidUsageBilledReason(authorization)
}

// liveModerationBlockedBilledReason 在拦截事件实时推送时标注计费：输出侧拦截意味着上游已产出内容、
// 用量必然发生；输入侧拦截是否已产生用量取决于与生成的时序，留待结算后由账单快照标注。
func liveModerationBlockedBilledReason(direction string, authorization *domainbilling.UsageAuthorization) string {
	if direction != appcm.DirectionOutput {
		return ""
	}
	return paidUsageBilledReason(authorization)
}

func paidUsageBilledReason(authorization *domainbilling.UsageAuthorization) string {
	if authorization == nil || authorization.Reservation == nil {
		return ""
	}
	return appbilling.BilledReasonModerationBlockedUpstreamUsage
}

// moderationLiveEmitter 把实时事件回调包装为审核协调器的推送出口，并给拦截事件标注计费说明。
func moderationLiveEmitter(
	onEvent func(eventType string, payload map[string]any) error,
	authorization *domainbilling.UsageAuthorization,
) appcm.LiveEmitter {
	return func(eventType string, payload map[string]any) {
		if eventType == "moderation_blocked" && payload != nil {
			direction, _ := payload["direction"].(string)
			if reason := liveModerationBlockedBilledReason(direction, authorization); reason != "" {
				payload["billedReason"] = reason
			}
		}
		_ = onEvent(eventType, payload)
	}
}

// SetModerationService 注入可选的内容审核编排器。
func (s *Service) SetModerationService(svc *appcm.Service) {
	s.moderationSvc = svc
	if svc == nil {
		return
	}
	svc.SetEventEmitter(func(ctx context.Context, runID string, eventType string, payload map[string]any) {
		if payload == nil {
			payload = map[string]any{"type": eventType}
		} else if _, ok := payload["type"]; !ok {
			payload["type"] = eventType
		}
		if s.generationStreams != nil {
			s.generationStreams.publishCurrent(ctx, normalizeRunID(runID), payload)
		}
	})
	svc.SetCancelRun(func(ctx context.Context, runID string) {
		if s.generationStreams != nil {
			s.generationStreams.cancelForced(ctx, normalizeRunID(runID))
		}
	})
	svc.SetOnBlocked(func(ctx context.Context, runID string, _ appcm.BlockInfo) {
		// 丢弃保留的增量/媒体，使重连无法重放已撤回的内容。
		// 随后发出的 moderation_blocked 会重新写入一个安全的终态事件。
		s.resetGenerationStreamEvents(ctx, runID)
	})
	svc.SetImageLoader(s.loadImageForModeration)
	svc.SetObjectStore(&moderationObjectStoreAdapter{service: s})
	svc.SetFileAccessController(&moderationFileAccessAdapter{service: s})
}

// startModerationRun 在策略启用时开始单轮审核。
// 实时事件使用现有的 OnEvent 路径（由 HTTP handler 设置）——无旁路通道。
func (s *Service) startModerationRun(
	ctx context.Context,
	input SendMessageInput,
	runID string,
	userMessage *model.Message,
	assistantMessage *model.Message,
) *appcm.RunCoordinator {
	if s == nil || s.moderationSvc == nil || userMessage == nil {
		return nil
	}
	meta := appcm.RunMeta{
		UserID:             input.UserID,
		ConversationID:     input.ConversationID,
		RunID:              runID,
		MessageID:          userMessage.ID,
		MessagePublicID:    userMessage.PublicID,
		UserMessageID:      userMessage.ID,
		AssistantMessageID: 0,
	}
	if assistantMessage != nil {
		meta.AssistantMessageID = assistantMessage.ID
	}
	coord := s.moderationSvc.BeginRun(ctx, meta)
	if coord == nil {
		return nil
	}
	if input.OnEvent != nil {
		coord.SetLiveEmitter(moderationLiveEmitter(input.OnEvent, input.UsageAuthorization))
	}
	coord.EnqueueInputText(input.Content)
	if len(input.FileIDs) > 0 {
		coord.EnqueueInputImages(ctx, input.FileIDs)
	}
	return coord
}

// completeModerationAfterSuccess 执行生成后屏障。
// 拦截时将 result 改写为拦截快照并设置 result.Moderation。
// 调用方依据 result.IsModerationBlocked() 分支；仅在通过/fail-open 时执行向量化。
func (s *Service) completeModerationAfterSuccess(ctx context.Context, input completeModerationAfterSuccessInput) {
	if input.Coordinator == nil || input.Result == nil {
		return
	}
	barrier := input.Coordinator.AfterGeneration(ctx, input.OutputText, input.OutputImages)
	applyBarrierOutcome(input.Result, barrier)
	if input.Result.IsModerationBlocked() {
		return
	}
	// 通过 / fail-open：立即向量化（屏障活动期间持久化路径跳过了向量化）。
	if input.ReuseUserMessage {
		s.embedMessagePairAsync(ctx, input.EmbedInput, nil, &input.Result.AssistantMessage)
	} else {
		s.embedMessagePairAsync(ctx, input.EmbedInput, &input.Result.UserMessage, &input.Result.AssistantMessage)
	}
}

// completeModerationAfterInterruption 审核在取消或上游失败后
// 已可见并被保留的内容，且不对部分回复进行向量化。
func (s *Service) completeModerationAfterInterruption(
	ctx context.Context,
	coord *appcm.RunCoordinator,
	result *SendMessageResult,
	outputText string,
) {
	if coord == nil || result == nil {
		return
	}
	barrier := coord.AfterGeneration(ctx, outputText, nil)
	applyBarrierOutcome(result, barrier)
}

// completeModerationAfterFailure 继续执行仅输入检查（不审核输出）。
func (s *Service) completeModerationAfterFailure(
	ctx context.Context,
	coord *appcm.RunCoordinator,
	result *SendMessageResult,
) {
	if coord == nil {
		return
	}
	barrier := coord.WaitInputOnly(ctx)
	if result == nil {
		return
	}
	applyBarrierOutcome(result, barrier)
}

func applyBarrierOutcome(result *SendMessageResult, barrier appcm.BarrierResult) {
	if result == nil {
		return
	}
	if barrier.Block == nil {
		result.Moderation = &MessageModerationOutcome{
			Blocked: false,
			State:   textutil.FirstNonEmpty(barrier.State, "passed"),
		}
		return
	}
	result.postBillingCompaction = nil
	result.MetadataRefreshHint = conversationMetadataRefreshNotNeeded
	applyBlockedSnapshot(result, *barrier.Block, barrier.TerminalEmitted)
}

func applyBlockedSnapshot(result *SendMessageResult, block appcm.BlockInfo, terminalEmitted bool) {
	if result == nil {
		return
	}
	if block.Direction == appcm.DirectionInput {
		result.UserMessage.Status = "blocked"
		result.UserMessage.ModerationEventID = block.EventID
		result.UserMessage.ModerationCategoriesJSON = mustJSONArray(block.Categories)
		result.UserMessage.ErrorCode = "content_moderation.blocked"
		result.UserMessage.ErrorMessage = "content blocked by moderation"
	}
	result.AssistantMessage.Status = "blocked"
	result.AssistantMessage.Content = ""
	result.AssistantMessage.ReasoningContent = ""
	result.AssistantMessage.Attachments = "[]"
	result.AssistantMessage.ProcessTrace = nil
	result.AssistantMessage.ModerationEventID = block.EventID
	result.AssistantMessage.ModerationCategoriesJSON = mustJSONArray(block.Categories)
	result.AssistantMessage.ErrorCode = "content_moderation.blocked"
	result.AssistantMessage.ErrorMessage = "content blocked by moderation"
	result.Moderation = &MessageModerationOutcome{
		Blocked:         true,
		EventID:         block.EventID,
		Direction:       block.Direction,
		Categories:      append([]string(nil), block.Categories...),
		State:           "blocked",
		TerminalEmitted: terminalEmitted,
	}
}

// applyBlockedRunFields 将软拦截结果复制到会话运行上，用于 finalize/upsert。
func applyBlockedRunFields(run *model.Run, result *SendMessageResult) {
	if run == nil || result == nil || !result.IsModerationBlocked() {
		return
	}
	run.Status = "blocked"
	run.ErrorCode = "content_moderation.blocked"
	run.ErrorMessage = "content blocked by moderation"
	run.ModerationState = "blocked"
	if result.Moderation != nil {
		run.ModerationEventID = result.Moderation.EventID
		run.ModerationCategoriesJSON = mustJSONArray(result.Moderation.Categories)
	}
}

// applyModerationRunState 将非拦截的屏障状态复制到运行上，用于 upsert。
func applyModerationRunState(run *model.Run, result *SendMessageResult) {
	if run == nil || result == nil || result.Moderation == nil {
		return
	}
	if result.Moderation.Blocked {
		applyBlockedRunFields(run, result)
		return
	}
	if state := strings.TrimSpace(result.Moderation.State); state != "" {
		run.ModerationState = state
	}
}

func mustJSONArray(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func moderationOutputText(parts ...string) string {
	seen := make(map[string]struct{}, len(parts))
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		kept = append(kept, part)
	}
	return strings.Join(kept, "\n\n")
}

func (s *Service) loadImageForModeration(ctx context.Context, userID uint, fileID string) (appcm.PreparedImage, error) {
	empty := appcm.PreparedImage{}
	file, err := s.repo.GetActiveFileObjectByID(ctx, userID, strings.TrimSpace(fileID))
	if err != nil || file == nil {
		return empty, err
	}
	declaredMIME := textutil.FirstNonEmpty(file.DetectedMIME, file.MimeType)
	if normalizeAttachmentKind("", declaredMIME) != "image" {
		return empty, appcm.ErrNonImageAttachment
	}
	cfg := s.cfg.Snapshot()
	storeProvider := s.storeProvider
	if storeProvider == nil {
		return empty, appstorage.ErrProviderNotConfigured
	}
	store, err := storeProvider.Open(ctx)
	if err != nil {
		return empty, err
	}
	reader, _, err := store.Open(ctx, strings.TrimSpace(file.StoragePath))
	if err != nil {
		return empty, err
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, maxConversationImageSourceBytes+1))
	_ = reader.Close()
	if readErr != nil {
		return empty, readErr
	}
	if len(data) == 0 {
		return empty, errEmptyModerationImage
	}
	if len(data) > 20*1024*1024 {
		return empty, errModerationImageTooLarge
	}
	detectedMIME := detectGeneratedImageMIME(data)
	if detectedMIME == "" {
		return empty, errUnsupportedModerationImage
	}
	maxDim := cfg.ImageMaxDimension
	if maxDim <= 0 {
		maxDim = 1024
	}
	resized, actualMIME := resizeImageIfNeeded(data, detectedMIME, maxDim)
	return appcm.PreparedImage{
		Data:   resized,
		SHA256: file.SHA256,
		Mime:   actualMIME,
		Size:   int64(len(resized)),
		FileID: file.FileID,
	}, nil
}

var (
	errEmptyModerationImage       = errString("empty image")
	errModerationImageTooLarge    = errString("image exceeds 20MB")
	errUnsupportedModerationImage = errString("unsupported moderation image")
)

type stringError string

func (e stringError) Error() string { return string(e) }
func errString(s string) error      { return stringError(s) }

// loadOutputImagesForModeration 加载最终助手图片附件，用于输出检查。
func (s *Service) loadOutputImagesForModeration(ctx context.Context, coord *appcm.RunCoordinator, userID uint, attachmentsJSON string) []appcm.OutputImageSource {
	refs := parseAttachmentSnapshotRefs(attachmentsJSON)
	if len(refs) == 0 {
		return nil
	}
	out := make([]appcm.OutputImageSource, 0, len(refs))
	for _, ref := range refs {
		fileID := strings.TrimSpace(ref.FileID)
		if fileID == "" {
			continue
		}
		kind := normalizeAttachmentKind(ref.Kind, textutil.FirstNonEmpty(ref.DetectedMIME, ref.MimeType))
		if kind != "image" {
			continue
		}
		prepared, err := s.loadImageForModeration(ctx, userID, fileID)
		if err != nil || len(prepared.Data) == 0 {
			if err == nil {
				err = errEmptyModerationImage
			}
			coord.RecordOutputImageFailure(fileID, err)
			continue
		}
		out = append(out, appcm.OutputImageSource{
			FileID:   fileID,
			Data:     prepared.Data,
			MimeType: prepared.Mime,
			SHA256:   prepared.SHA256,
		})
	}
	return out
}

func loadOutputImagesFromFiles(coord *appcm.RunCoordinator, files []model.FileObject, dataByFileID map[string][]byte) []appcm.OutputImageSource {
	out := make([]appcm.OutputImageSource, 0, len(files))
	for _, file := range files {
		data := dataByFileID[file.FileID]
		if len(data) == 0 {
			coord.RecordOutputImageFailure(file.FileID, errEmptyModerationImage)
			continue
		}
		out = append(out, appcm.OutputImageSource{
			FileID:   file.FileID,
			Data:     data,
			MimeType: textutil.FirstNonEmpty(file.DetectedMIME, file.MimeType, "image/png"),
			SHA256:   file.SHA256,
		})
	}
	return out
}

func (s *Service) resetGenerationStreamEvents(ctx context.Context, runID string) {
	runID = normalizeRunID(runID)
	if runID == "" || s == nil || s.generationStreams == nil {
		return
	}
	s.generationStreams.resetCurrentEvents(ctx, runID)
}

type moderationObjectStoreAdapter struct {
	service *Service
}

func (a *moderationObjectStoreAdapter) Put(ctx context.Context, path string, data []byte, contentType string) error {
	store, err := a.open(ctx)
	if err != nil {
		return err
	}
	_, err = store.Put(ctx, path, bytes.NewReader(data), objectstorage.PutOptions{ContentType: contentType})
	return err
}

func (a *moderationObjectStoreAdapter) Open(ctx context.Context, path string) ([]byte, error) {
	store, err := a.open(ctx)
	if err != nil {
		return nil, err
	}
	reader, _, err := store.Open(ctx, path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (a *moderationObjectStoreAdapter) Delete(ctx context.Context, path string) error {
	store, err := a.open(ctx)
	if err != nil {
		return err
	}
	return store.Delete(ctx, path)
}

func (a *moderationObjectStoreAdapter) open(ctx context.Context) (objectstorage.Store, error) {
	if a == nil || a.service == nil || a.service.storeProvider == nil {
		return nil, appstorage.ErrProviderNotConfigured
	}
	provider := a.service.storeProvider
	return provider.Open(ctx)
}

type moderationFileAccessAdapter struct {
	service *Service
}

var _ appcm.FileAccessController = (*moderationFileAccessAdapter)(nil)

type moderationBlockedFileLister interface {
	ListModerationBlockedFileIDsForCleanup(ctx context.Context, limit int) ([]string, error)
}

func (a *moderationFileAccessAdapter) RevokeGeneratedFile(ctx context.Context, fileID string) error {
	if a.service == nil || a.service.repo == nil {
		return nil
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil
	}
	return a.service.repo.RevokeGeneratedFileForModeration(ctx, fileID)
}

func (a *moderationFileAccessAdapter) DeleteGeneratedFileArtifacts(ctx context.Context, fileID string) error {
	if a.service == nil || a.service.repo == nil {
		return nil
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil
	}
	var storagePath string
	if file, err := a.service.repo.GetFileObjectByFileIDAnyStatus(ctx, fileID); err == nil && file != nil {
		storagePath = strings.TrimSpace(file.StoragePath)
	}
	if err := a.service.repo.DeleteGeneratedFileArtifactsForModeration(ctx, fileID); err != nil {
		return err
	}
	if storagePath == "" {
		return nil
	}
	storeProvider := a.service.storeProvider
	if storeProvider == nil {
		return appstorage.ErrProviderNotConfigured
	}
	store, err := storeProvider.Open(ctx)
	if err != nil {
		return err
	}
	if err := store.Delete(ctx, storagePath); err != nil {
		return err
	}
	return a.service.repo.ClearGeneratedFileStoragePath(ctx, fileID)
}

func (a *moderationFileAccessAdapter) RetryBlockedGeneratedFileDeletes(ctx context.Context, limit int) (int, error) {
	if a.service == nil || a.service.repo == nil {
		return 0, nil
	}
	lister, ok := a.service.repo.(moderationBlockedFileLister)
	if !ok {
		return 0, errors.New("conversation repository does not support moderation file cleanup")
	}
	fileIDs, err := lister.ListModerationBlockedFileIDsForCleanup(ctx, limit)
	if err != nil {
		return 0, err
	}
	deleted := 0
	var cleanupErr error
	for _, fileID := range fileIDs {
		if err := a.DeleteGeneratedFileArtifacts(ctx, fileID); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		deleted++
	}
	return deleted, cleanupErr
}

// filterBlockedMessages 从模型上下文中排除被拦截的消息。
func filterBlockedMessages(messages []model.Message) []model.Message {
	if len(messages) == 0 {
		return messages
	}
	out := make([]model.Message, 0, len(messages))
	for _, item := range messages {
		if strings.EqualFold(strings.TrimSpace(item.Status), "blocked") {
			continue
		}
		out = append(out, item)
	}
	return out
}
