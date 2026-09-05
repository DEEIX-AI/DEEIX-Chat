package contentmoderation

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"go.uber.org/zap"
)

// RunMeta identifies the moderated conversation turn.
type RunMeta struct {
	UserID             uint
	ConversationID     uint
	RunID              string
	MessageID          uint
	MessagePublicID    string
	AssistantMessageID uint
	UserMessageID      uint
	// Ephemeral 表示该运行没有 conversation/message/run 持久化记录。
	// 审核事件仍按安全保留策略记录，但协调器不得写会话域状态。
	Ephemeral bool
}

// BlockInfo is returned when a round is blocked.
type BlockInfo struct {
	EventID    string
	Direction  string
	Categories []string
}

// BarrierResult is the post-generation / input-only barrier outcome.
type BarrierResult struct {
	// Block is non-nil whenever a known hit must be hidden from the caller. Durable
	// persistence may converge asynchronously when the primary transaction is unavailable.
	Block *BlockInfo
	// State is the caller-visible moderation target (passed|failed_open|blocked).
	State string
	// TerminalEmitted is true when moderation_blocked was published to live/recovery streams.
	TerminalEmitted bool
}

// LiveEmitter delivers events to the active HTTP stream (in addition to recovery storage).
type LiveEmitter func(eventType string, payload map[string]any)

// RunCoordinator tracks moderation tasks for a single chat/media run.
type RunCoordinator struct {
	service  *Service
	ctx      context.Context
	meta     RunMeta
	cfg      runtimeConfig
	liveEmit LiveEmitter

	mu             sync.Mutex
	pending        int
	blocked        bool
	blockInfo      BlockInfo
	failedOpen     bool
	allDone        chan struct{}
	allClosed      bool
	cancelOnce     sync.Once
	finished       bool
	settled        bool
	blockHandled   bool
	outputEnqueued bool
}

func newRunCoordinator(ctx context.Context, service *Service, meta RunMeta, cfg runtimeConfig) *RunCoordinator {
	return &RunCoordinator{
		service: service,
		ctx:     background.Detach(ctx),
		meta:    meta,
		cfg:     cfg,
		allDone: make(chan struct{}),
	}
}

// SetLiveEmitter wires the active stream sink for moderation_checking / moderation_blocked.
func (c *RunCoordinator) SetLiveEmitter(emit LiveEmitter) {
	if c == nil {
		return
	}
	c.liveEmit = emit
}

// EnqueueInputText queues input text moderation if policy requires it.
func (c *RunCoordinator) EnqueueInputText(text string) {
	if c == nil {
		return
	}
	selected := c.cfg.Policy.CategoriesFor(domaincm.DirectionInput, domaincm.ModalityText)
	if len(selected) == 0 || strings.TrimSpace(text) == "" {
		return
	}
	c.startTask(&moderationTask{
		Coord:     c,
		Direction: domaincm.DirectionInput,
		Modality:  domaincm.ModalityText,
		Text:      text,
		Selected:  selected,
		Location:  domaincm.ContentLocation{Field: "user_message"},
	})
}

// EnqueueInputImages queues input image moderation for used attachments.
func (c *RunCoordinator) EnqueueInputImages(ctx context.Context, fileIDs []string) {
	if c == nil {
		return
	}
	selected := c.cfg.Policy.CategoriesFor(domaincm.DirectionInput, domaincm.ModalityImage)
	if len(selected) == 0 || len(fileIDs) == 0 {
		return
	}
	if c.service == nil || c.service.imageLoader == nil {
		c.recordSurfaceFailure(domaincm.DirectionInput, domaincm.ModalityImage, "", ErrModerationService)
		return
	}
	seenSHA := make(map[string]struct{})
	raw := make([]OutputImageSource, 0, len(fileIDs))
	keptFiles := make([]string, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		fileID = strings.TrimSpace(fileID)
		if fileID == "" {
			continue
		}
		prepared, err := c.service.imageLoader(ctx, c.meta.UserID, fileID)
		if errors.Is(err, ErrNonImageAttachment) {
			continue
		}
		if err != nil || len(prepared.Data) == 0 {
			if err == nil {
				err = ErrModerationInvalidResp
			}
			c.recordSurfaceFailure(domaincm.DirectionInput, domaincm.ModalityImage, fileID, err)
			continue
		}
		sha := prepared.SHA256
		if sha != "" {
			if _, ok := seenSHA[sha]; ok {
				continue
			}
			seenSHA[sha] = struct{}{}
		}
		// Isolated copy for review only; user originals are not deleted.
		raw = append(raw, OutputImageSource{
			FileID:   fileID,
			Data:     prepared.Data,
			MimeType: prepared.Mime,
			SHA256:   sha,
		})
		keptFiles = append(keptFiles, fileID)
	}
	if len(raw) == 0 {
		return
	}
	c.enqueueInputImageSources(raw, keptFiles, selected)
}

// EnqueueInputImageSources queues request-scoped image bytes without requiring
// persisted file records. Ephemeral chat uses this path so its images follow the
// same moderation policy while remaining outside the user file library.
func (c *RunCoordinator) EnqueueInputImageSources(images []OutputImageSource) {
	if c == nil {
		return
	}
	selected := c.cfg.Policy.CategoriesFor(domaincm.DirectionInput, domaincm.ModalityImage)
	if len(selected) == 0 || len(images) == 0 {
		return
	}
	seenSHA := make(map[string]struct{})
	raw := make([]OutputImageSource, 0, len(images))
	fileIDs := make([]string, 0, len(images))
	for _, image := range images {
		if len(image.Data) == 0 {
			c.recordSurfaceFailure(domaincm.DirectionInput, domaincm.ModalityImage, image.FileID, ErrModerationInvalidResp)
			continue
		}
		sha := strings.TrimSpace(image.SHA256)
		if sha != "" {
			if _, exists := seenSHA[sha]; exists {
				continue
			}
			seenSHA[sha] = struct{}{}
		}
		fileID := strings.TrimSpace(image.FileID)
		raw = append(raw, OutputImageSource{
			FileID:   fileID,
			Data:     append([]byte(nil), image.Data...),
			MimeType: strings.TrimSpace(image.MimeType),
			SHA256:   sha,
		})
		fileIDs = append(fileIDs, fileID)
	}
	if len(raw) == 0 {
		return
	}
	c.enqueueInputImageSources(raw, fileIDs, selected)
}

func (c *RunCoordinator) enqueueInputImageSources(raw []OutputImageSource, fileIDs []string, selected []string) {
	c.startTask(&moderationTask{
		Coord:     c,
		Direction: domaincm.DirectionInput,
		Modality:  domaincm.ModalityImage,
		RawImages: raw,
		FileIDs:   fileIDs,
		Selected:  selected,
		Location:  domaincm.ContentLocation{Field: "user_attachments"},
		// Input hits must isolate but not revoke user library files.
		IsolateOnly: true,
	})
}

// AfterGeneration runs the post-generation barrier.
func (c *RunCoordinator) AfterGeneration(ctx context.Context, outputText string, outputImages []OutputImageSource) BarrierResult {
	if c == nil {
		return BarrierResult{State: domaincm.ModerationStatePassed}
	}
	if !c.meta.Ephemeral {
		if err := c.service.repo.UpdateRunModeration(ctx, c.meta.RunID, domaincm.ModerationStateModerating, "", "[]"); err != nil {
			c.service.logWarn("content_moderation_mark_moderating_failed", zap.String("run_id", c.meta.RunID), zap.Error(err))
		}
	}
	c.emit("moderation_checking", map[string]any{
		"type": "moderation_checking",
	})

	c.enqueueOutputText(outputText)
	c.enqueueOutputImages(outputImages)
	c.markOutputsEnqueued()
	c.waitAll(ctx)

	blocked, info, failedOpen := c.settle()

	if blocked {
		emitted, err := c.applyBlock(info)
		if err != nil {
			c.service.registerPendingBlock(c.meta, info)
			emitted = c.notifyBlocked(info)
		}
		c.finish()
		return BarrierResult{
			Block:           &info,
			State:           domaincm.ModerationStateBlocked,
			TerminalEmitted: emitted,
		}
	}
	state := domaincm.ModerationStatePassed
	if failedOpen {
		state = domaincm.ModerationStateFailedOpen
	}
	c.updateRunState(state, "", "[]")
	c.finish()
	return BarrierResult{State: state}
}

// WaitInputOnly continues input checks after generation errors/cancels.
func (c *RunCoordinator) WaitInputOnly(ctx context.Context) BarrierResult {
	if c == nil {
		return BarrierResult{State: domaincm.ModerationStatePassed}
	}
	c.markOutputsEnqueued()
	c.waitAll(ctx)
	blocked, info, failedOpen := c.settle()
	if blocked {
		emitted, err := c.applyBlock(info)
		if err != nil {
			c.service.registerPendingBlock(c.meta, info)
			emitted = c.notifyBlocked(info)
		}
		c.finish()
		return BarrierResult{
			Block:           &info,
			State:           domaincm.ModerationStateBlocked,
			TerminalEmitted: emitted,
		}
	}
	state := domaincm.ModerationStatePassed
	if failedOpen {
		state = domaincm.ModerationStateFailedOpen
	}
	c.updateRunState(state, "", "[]")
	c.finish()
	return BarrierResult{State: state}
}

func (c *RunCoordinator) settle() (bool, BlockInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.settled = true
	if c.blocked {
		c.blockHandled = true
	}
	return c.blocked, c.blockInfo, c.failedOpen
}

func (c *RunCoordinator) updateRunState(state, eventID, categoriesJSON string) {
	if c == nil || c.meta.Ephemeral || c.service == nil || c.service.repo == nil {
		return
	}
	ctx, cancel := background.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()
	if err := c.service.repo.UpdateRunModeration(ctx, c.meta.RunID, state, eventID, categoriesJSON); err != nil {
		c.service.logWarn("content_moderation_update_run_state_failed", zap.String("run_id", c.meta.RunID), zap.String("state", state), zap.Error(err))
	}
}

// RecordOutputImageFailure records an expected model-output image that could not be loaded.
// A missing image must never be treated as a clean moderation pass.
func (c *RunCoordinator) RecordOutputImageFailure(fileID string, loadErr error) {
	c.recordSurfaceFailure(domaincm.DirectionOutput, domaincm.ModalityImage, fileID, loadErr)
}

func (c *RunCoordinator) recordSurfaceFailure(direction, modality, fileID string, surfaceErr error) {
	if c == nil || c.service == nil || len(c.cfg.Policy.CategoriesFor(direction, modality)) == 0 {
		return
	}
	c.mu.Lock()
	if c.settled || c.finished {
		c.mu.Unlock()
		return
	}
	c.failedOpen = true
	c.mu.Unlock()

	if surfaceErr != nil {
		c.service.logWarn("content_moderation_surface_unavailable",
			zap.String("run_id", c.meta.RunID),
			zap.String("direction", direction),
			zap.String("modality", modality),
			zap.String("file_id", strings.TrimSpace(fileID)),
			zap.Error(surfaceErr),
		)
	}
	ctx, cancel := background.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()
	c.service.recordFailedOpen(ctx, c.meta, direction, modality, domaincm.ErrorCodeServiceError, 0)
	c.service.bumpDailyStat(ctx, repository.DailyStatIncrement{
		Direction:    direction,
		Modality:     modality,
		Result:       domaincm.ResultFailedOpen,
		CheckCount:   1,
		ContentItems: 1,
		FailureCount: 1,
	})
}

func (c *RunCoordinator) enqueueOutputText(text string) {
	selected := c.cfg.Policy.CategoriesFor(domaincm.DirectionOutput, domaincm.ModalityText)
	if len(selected) == 0 || strings.TrimSpace(text) == "" {
		return
	}
	c.startTask(&moderationTask{
		Coord:     c,
		Direction: domaincm.DirectionOutput,
		Modality:  domaincm.ModalityText,
		Text:      text,
		Selected:  selected,
		Location:  domaincm.ContentLocation{Field: "assistant_message"},
	})
}

func (c *RunCoordinator) enqueueOutputImages(images []OutputImageSource) {
	selected := c.cfg.Policy.CategoriesFor(domaincm.DirectionOutput, domaincm.ModalityImage)
	if len(selected) == 0 || len(images) == 0 {
		return
	}
	seen := make(map[string]struct{})
	raw := make([]OutputImageSource, 0, len(images))
	for _, img := range images {
		if len(img.Data) == 0 {
			continue
		}
		sha := img.SHA256
		if sha == "" {
			sha = sha256Hex(img.Data)
			img.SHA256 = sha
		}
		if _, ok := seen[sha]; ok {
			continue
		}
		seen[sha] = struct{}{}
		img.MimeType = textutil.FirstNonEmpty(img.MimeType, "image/png")
		raw = append(raw, img)
	}
	if len(raw) == 0 {
		return
	}
	c.startTask(&moderationTask{
		Coord:     c,
		Direction: domaincm.DirectionOutput,
		Modality:  domaincm.ModalityImage,
		RawImages: raw,
		Selected:  selected,
		Location:  domaincm.ContentLocation{Field: "assistant_images"},
	})
}

func (c *RunCoordinator) startTask(task *moderationTask) {
	c.mu.Lock()
	if c.finished {
		c.mu.Unlock()
		return
	}
	c.pending++
	c.mu.Unlock()

	// Worker (or enqueue-full path) calls onTaskResult directly — no Done-wait goroutine.
	if err := c.service.enqueue(task); err != nil {
		c.onTaskResult(task, taskResult{Err: err, ErrorCode: domaincm.ErrorCodeQueueFull})
	}
}

func (c *RunCoordinator) onTaskResult(task *moderationTask, result taskResult) *BlockInfo {
	c.mu.Lock()
	direction := ""
	if task != nil {
		direction = task.Direction
	}
	if c.pending > 0 {
		c.pending--
	}
	// Clear task payloads after processing to reduce retained sensitive memory.
	if task != nil {
		task.Text = ""
		task.RawImages = nil
	}
	if c.settled || c.finished {
		var lateBlock *BlockInfo
		if result.Hit && (!c.blockHandled || preferBlockDirection(direction, c.blockInfo.Direction)) {
			c.blockHandled = true
			info := BlockInfo{
				EventID:    result.EventID,
				Direction:  direction,
				Categories: append([]string(nil), result.Categories...),
			}
			c.blocked = true
			c.blockInfo = info
			lateBlock = &info
		}
		if c.pending == 0 && c.outputEnqueued {
			c.closeAllLocked()
		}
		c.mu.Unlock()
		return lateBlock
	}
	cancelInput := false
	if result.Hit {
		if !c.blocked || preferBlockDirection(direction, c.blockInfo.Direction) {
			c.blocked = true
			c.blockInfo = BlockInfo{
				EventID:    result.EventID,
				Direction:  direction,
				Categories: append([]string(nil), result.Categories...),
			}
		}
		if direction == domaincm.DirectionInput {
			cancelInput = true
		}
	} else if result.Err != nil {
		c.failedOpen = true
	}
	if c.pending == 0 && c.outputEnqueued {
		c.closeAllLocked()
	}
	c.mu.Unlock()
	if cancelInput {
		c.cancelOnce.Do(func() {
			if c.service.cancelRun != nil {
				c.service.cancelRun(c.ctx, c.meta.RunID)
			}
		})
	}
	return nil
}

func (c *RunCoordinator) markOutputsEnqueued() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.outputEnqueued = true
	if c.pending == 0 {
		c.closeAllLocked()
	}
}

func (c *RunCoordinator) closeAllLocked() {
	if !c.allClosed {
		close(c.allDone)
		c.allClosed = true
	}
}

func preferBlockDirection(candidate, current string) bool {
	return candidate == domaincm.DirectionInput && current != domaincm.DirectionInput
}

func (c *RunCoordinator) waitAll(ctx context.Context) {
	// Bound wait to remaining policy timeout so stream cannot hang forever.
	timeout := c.cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	// Allow queued + multi-surface work up to 2x single-check budget, capped at 60s.
	deadline := timeout * 2
	if deadline > 60*time.Second {
		deadline = 60 * time.Second
	}
	timer := time.NewTimer(deadline)
	defer timer.Stop()
	select {
	case <-c.allDone:
	case <-ctx.Done():
		select {
		case <-c.allDone:
		case <-time.After(100 * time.Millisecond):
			c.mu.Lock()
			if !c.blocked {
				c.failedOpen = true
			}
			c.mu.Unlock()
		}
	case <-timer.C:
		c.mu.Lock()
		if !c.blocked {
			c.failedOpen = true
		}
		c.mu.Unlock()
	}
}

// applyBlock persists withdrawal then emits the terminal stream event.
func (c *RunCoordinator) applyBlock(info BlockInfo) (bool, error) {
	if c.meta.Ephemeral {
		return c.notifyBlocked(info), nil
	}
	// Client disconnect cancels the request context; persistence must survive that.
	persistCtx, cancel := background.WithTimeout(c.ctx, 15*time.Second)
	defer cancel()
	includeUser := info.Direction == domaincm.DirectionInput
	categoriesJSON := mustJSON(info.Categories)

	// Single transactional write path — no sequential fallback.
	fileIDs, err := c.service.repo.ApplyRunBlock(persistCtx, c.meta.RunID, includeUser, info.EventID, categoriesJSON)
	if err != nil {
		c.service.logWarn("content_moderation_apply_block_failed",
			zap.String("run_id", c.meta.RunID),
			zap.Error(err),
		)
		return false, err
	}
	c.service.removePendingBlock(c.meta.RunID)
	c.service.deleteBlockedOutputFiles(c.ctx, fileIDs)
	return c.notifyBlocked(info), nil
}

func (c *RunCoordinator) notifyBlocked(info BlockInfo) bool {
	if c.service.onBlocked != nil {
		c.service.onBlocked(c.ctx, c.meta.RunID, info)
	}
	c.emit("moderation_blocked", map[string]any{
		"type":       "moderation_blocked",
		"eventID":    info.EventID,
		"direction":  info.Direction,
		"categories": info.Categories,
	})
	return true
}

func (c *RunCoordinator) emit(eventType string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{"type": eventType}
	} else if _, ok := payload["type"]; !ok {
		payload["type"] = eventType
	}
	// Prefer live sink (handler flushStreamEvent already persists + writes NDJSON).
	// Fall back to recovery-only emitter when no live connection is bound.
	if c.liveEmit != nil {
		c.liveEmit(eventType, payload)
		return
	}
	if c.service != nil && c.service.emitEvent != nil {
		c.service.emitEvent(c.ctx, c.meta.RunID, eventType, payload)
	}
}

func (c *RunCoordinator) finish() {
	c.mu.Lock()
	c.finished = true
	c.mu.Unlock()
	if c.service != nil {
		c.service.releaseCoordinator(c.meta.RunID)
	}
}

// IsBlocked returns whether a hit has already been recorded.
func (c *RunCoordinator) IsBlocked() (bool, BlockInfo) {
	if c == nil {
		return false, BlockInfo{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.blocked, c.blockInfo
}
