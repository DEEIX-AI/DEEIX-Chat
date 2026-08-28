package conversation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	appcm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/contentmoderation"
	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/objectstore"
	"go.uber.org/zap"
)

// MessageModerationOutcome is the soft-moderation end state for a turn.
// Nil means moderation was not required (policy off / no coordinator).
type MessageModerationOutcome struct {
	Blocked    bool
	EventID    string
	Direction  string
	Categories []string
	// State is the moderation target: passed | failed_open | blocked. Known blocks may
	// converge durably through the moderation compensation loop.
	State string
	// TerminalEmitted is true when moderation_blocked was already pushed to the stream.
	TerminalEmitted bool
}

// IsModerationBlocked reports whether the turn was blocked after a safety check.
func (r *SendMessageResult) IsModerationBlocked() bool {
	return r != nil && r.Moderation != nil && r.Moderation.Blocked
}

// ModerationTerminalEmitted reports whether moderation_blocked already went out on the stream.
func (r *SendMessageResult) ModerationTerminalEmitted() bool {
	return r != nil && r.Moderation != nil && r.Moderation.TerminalEmitted
}

// SetModerationService injects the optional content moderation orchestrator.
func (s *Service) SetModerationService(svc *appcm.Service) {
	s.moderationSvc = svc
	if svc == nil {
		return
	}
	svc.SetEventEmitter(func(runID string, eventType string, payload map[string]interface{}) {
		if payload == nil {
			payload = map[string]interface{}{"type": eventType}
		} else if _, ok := payload["type"]; !ok {
			payload["type"] = eventType
		}
		s.PublishMessageGenerationEvent(runID, payload)
	})
	svc.SetCancelRun(func(runID string) {
		if s.generationStreams != nil {
			s.generationStreams.cancelForced(context.Background(), normalizeRunID(runID))
		}
	})
	svc.SetOnBlocked(func(runID string, _ appcm.BlockInfo) {
		// Drop retained deltas/media so reconnect cannot replay withdrawn content.
		// The following emit of moderation_blocked re-seeds a safe terminal event.
		s.resetGenerationStreamEvents(runID)
	})
	svc.SetImageLoader(s.loadImageForModeration)
	svc.SetObjectStore(&moderationObjectStoreAdapter{service: s})
	svc.SetFileAccessController(&moderationFileAccessAdapter{service: s})
}

// startModerationRun begins per-turn moderation when policy is enabled.
// Live events use the existing OnEvent path (set by HTTP handlers) — no side channel.
// seedRun, when non-nil, is ensured in DB so mid-flight moderation_state updates have a row.
func (s *Service) startModerationRun(
	ctx context.Context,
	input SendMessageInput,
	runID string,
	userMessage *model.Message,
	assistantMessage *model.Message,
	seedRun *model.Run,
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
	// Durable run row must exist before UpdateRunModeration / ApplyRunBlock.
	s.ensureConversationRunForModeration(ctx, seedRun, input, runID)
	// BeginRun may have updated before the row existed; re-apply pending now.
	s.moderationSvc.SyncRunPending(ctx, runID)
	if input.OnEvent != nil {
		coord.SetLiveEmitter(func(eventType string, payload map[string]interface{}) {
			_ = input.OnEvent(eventType, payload)
		})
	}
	coord.EnqueueInputText(input.Content)
	if len(input.FileIDs) > 0 {
		coord.EnqueueInputImages(ctx, input.FileIDs)
	}
	return coord
}

// ensureConversationRunForModeration inserts a mid-flight run so barrier state updates are not no-ops.
func (s *Service) ensureConversationRunForModeration(
	ctx context.Context,
	seedRun *model.Run,
	input SendMessageInput,
	runID string,
) {
	if s == nil || s.repo == nil {
		return
	}
	var run model.Run
	if seedRun != nil {
		run = *seedRun
	} else {
		run = model.Run{
			RunID:          runID,
			RequestID:      strings.TrimSpace(input.RequestID),
			UserID:         input.UserID,
			ConversationID: input.ConversationID,
			TaskType:       "chat",
			Status:         "running",
			StartedAt:      time.Now(),
		}
	}
	if strings.TrimSpace(run.RunID) == "" {
		run.RunID = runID
	}
	if strings.TrimSpace(run.Status) == "" || run.Status == "error" {
		run.Status = "running"
	}
	run.ModerationState = "pending"
	run.EndedAt = nil
	if err := s.repo.EnsureConversationRun(ctx, &run); err != nil && s.logger != nil {
		s.logger.Warn("ensure_conversation_run_for_moderation_failed",
			zap.String("run_id", run.RunID),
			zap.Error(err),
		)
	}
}

// completeModerationAfterSuccess runs the post-generation barrier.
// On block it mutates result into a blocked snapshot and sets result.Moderation.
// Callers branch on result.IsModerationBlocked(); embed only runs on pass/fail-open.
func (s *Service) completeModerationAfterSuccess(
	ctx context.Context,
	coord *appcm.RunCoordinator,
	result *SendMessageResult,
	outputText string,
	outputImages []appcm.OutputImageSource,
	embedInput SendMessageInput,
	reuseUserMessage bool,
) {
	if coord == nil || result == nil {
		return
	}
	barrier := coord.AfterGeneration(ctx, outputText, outputImages)
	applyBarrierOutcome(result, barrier)
	if result.IsModerationBlocked() {
		return
	}
	// Pass / fail-open: embed now (persist path skipped embed while barrier was active).
	if reuseUserMessage {
		s.embedMessagePairAsync(embedInput, nil, &result.AssistantMessage)
	} else {
		s.embedMessagePairAsync(embedInput, &result.UserMessage, &result.AssistantMessage)
	}
}

// completeModerationAfterInterruption moderates content that was already visible
// and retained after a cancel or upstream failure, without embedding a partial reply.
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

// completeModerationAfterFailure continues input-only checks (no output moderation).
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
			State:   firstNonEmptyString(barrier.State, "passed"),
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

// applyBlockedRunFields copies soft-block outcome onto a conversation run for finalize/upsert.
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

// applyModerationRunState copies non-blocked barrier state onto the run for upsert.
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
	declaredMIME := firstNonEmptyString(file.DetectedMIME, file.MimeType)
	if normalizeAttachmentKind("", declaredMIME) != "image" {
		return empty, appcm.ErrNonImageAttachment
	}
	cfg := s.cfg.Snapshot()
	storeProvider := s.storeProvider
	if storeProvider == nil {
		storeProvider = appstorage.NewRuntimeProvider(config.NewRuntime(cfg), nil)
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

// loadOutputImagesForModeration loads final assistant image attachments for output checks.
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
		kind := normalizeAttachmentKind(ref.Kind, firstNonEmptyString(ref.DetectedMIME, ref.MimeType))
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
			MimeType: firstNonEmptyString(file.DetectedMIME, file.MimeType, "image/png"),
			SHA256:   file.SHA256,
		})
	}
	return out
}

func (s *Service) resetGenerationStreamEvents(runID string) {
	runID = normalizeRunID(runID)
	if runID == "" || s == nil || s.generationStreams == nil {
		return
	}
	s.generationStreams.resetEvents(context.Background(), runID)
}

type moderationObjectStoreAdapter struct {
	service *Service
}

func (a *moderationObjectStoreAdapter) Put(ctx context.Context, path string, data []byte, contentType string) error {
	store, err := a.open(ctx)
	if err != nil {
		return err
	}
	_, err = store.Put(ctx, path, bytes.NewReader(data), objectstore.PutOptions{ContentType: contentType})
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

func (a *moderationObjectStoreAdapter) open(ctx context.Context) (objectstore.Store, error) {
	provider := a.service.storeProvider
	if provider == nil {
		provider = appstorage.NewRuntimeProvider(a.service.cfg, nil)
	}
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
		storeProvider = appstorage.NewRuntimeProvider(a.service.cfg, nil)
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

// filterBlockedMessages excludes blocked messages from model context.
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
