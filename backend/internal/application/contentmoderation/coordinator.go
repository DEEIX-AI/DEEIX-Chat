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

// RunMeta 标识被审核的会话轮次。
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

// BlockInfo 在轮次被拦截时返回。
type BlockInfo struct {
	EventID    string
	Direction  string
	Categories []string
}

// BarrierResult 是生成后 / 仅输入屏障的结果。
type BarrierResult struct {
	// 只要已知命中必须对调用方隐藏，Block 即非 nil。主事务不可用时，持久化落库可能由异步补偿收敛。
	Block *BlockInfo
	// State 是调用方可见的审核目标状态（passed|failed_open|blocked）。
	State string
	// 当 moderation_blocked 已发布到实时/恢复流时，TerminalEmitted 为 true。
	TerminalEmitted bool
}

// LiveEmitter 将事件投递到活动的 HTTP 流（恢复存储之外）。
type LiveEmitter func(eventType string, payload map[string]any)

// RunCoordinator 跟踪单次聊天/媒体运行的审核任务。
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

// SetLiveEmitter 为 moderation_checking / moderation_blocked 接入活动流输出端。
func (c *RunCoordinator) SetLiveEmitter(emit LiveEmitter) {
	if c == nil {
		return
	}
	c.liveEmit = emit
}

// EnqueueInputText 在策略要求时将输入文本审核入队。
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

// EnqueueInputImages 为已使用的附件将输入图片审核入队。
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
		// 仅用于审核的隔离副本；不删除用户原始文件。
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

// EnqueueInputImageSources 将请求作用域内的图片字节入队，无需
// 持久化的文件记录。临时聊天使用此路径，使其图片遵循
// 相同的审核策略，同时保持在用户文件库之外。
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
		// 输入命中必须隔离，但不撤销用户文件库中的文件。
		IsolateOnly: true,
	})
}

// AfterGeneration 执行生成后屏障。
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

// WaitInputOnly 在生成出错/取消后继续执行输入检查。
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

// RecordOutputImageFailure 记录预期存在但无法加载的模型输出图片。
// 缺失的图片绝不能被视为审核通过。
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

	// Worker（或入队已满路径）直接调用 onTaskResult——无需等待 Done 的 goroutine。
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
	// 处理后清空任务载荷，以减少驻留的敏感内存。
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
	// 将等待限制在剩余策略超时内，避免流永久挂起。
	timeout := c.cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	// 允许排队 + 多检查面的工作最多占用单次检查预算的 2 倍，上限 60s。
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

// applyBlock 持久化撤回后发出终态流事件。
func (c *RunCoordinator) applyBlock(info BlockInfo) (bool, error) {
	if c.meta.Ephemeral {
		return c.notifyBlocked(info), nil
	}
	// 客户端断开会取消请求上下文；持久化必须不受其影响。
	persistCtx, cancel := background.WithTimeout(c.ctx, 15*time.Second)
	defer cancel()
	includeUser := info.Direction == domaincm.DirectionInput
	categoriesJSON := mustJSON(info.Categories)

	// 单一事务写入路径——无顺序回退。
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
	// 优先使用实时输出端（handler 的 flushStreamEvent 已完成持久化并写入 NDJSON）。
	// 未绑定实时连接时回退到仅恢复的发送器。
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

// IsBlocked 返回是否已记录命中。
func (c *RunCoordinator) IsBlocked() (bool, BlockInfo) {
	if c == nil {
		return false, BlockInfo{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.blocked, c.blockInfo
}
