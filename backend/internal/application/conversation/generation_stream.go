package conversation

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/google/uuid"
)

const (
	generationStreamRetention            = 15 * time.Minute
	generationStreamActiveTTL            = 2 * time.Hour
	generationStreamLeaseTTL             = 30 * time.Second
	generationStreamLeaseRefresh         = 10 * time.Second
	generationStreamMaxEvents            = 1024
	generationStreamSubscriberBuffer     = 128
	generationStreamReadBlock            = 5 * time.Second
	generationStreamMaxPayloadBytes      = 128 * 1024
	generationStreamCleanupTimeout       = 5 * time.Second
	generationStreamCompletionAttempts   = 3
	generationStreamCompletionRetryDelay = 100 * time.Millisecond
	generationStreamCompletionMaxDelay   = 5 * time.Second
)

type generationStreamOptions struct {
	Retention        time.Duration
	ActiveTTL        time.Duration
	LeaseTTL         time.Duration
	LeaseRefresh     time.Duration
	MaxEvents        int
	SubscriberBuffer int
}

type readStoreEventsInput struct {
	Store                    repository.GenerationStreamCacheRepository
	UserID                   uint
	RunID                    string
	Cursor                   string
	AfterSeq                 int64
	TextSnapshotSeq          int64
	UpstreamThinkSnapshotSeq int64
	Output                   chan<- GenerationStreamEvent
}

type retainedStreamEventsInput struct {
	Records                  []repository.GenerationStreamMessage
	AfterSeq                 int64
	TextSnapshot             repository.GenerationStreamTextSnapshot
	HasTextSnapshot          bool
	UpstreamThinkSnapshot    repository.GenerationStreamUpstreamThinkSnapshot
	HasUpstreamThinkSnapshot bool
	IncludeSnapshots         bool
}

type pendingGenerationCompletion struct {
	lease       repository.GenerationStreamLease
	nextAttempt time.Time
	retryDelay  time.Duration
	expiresAt   time.Time
}

func defaultGenerationStreamOptions() generationStreamOptions {
	return generationStreamOptions{
		Retention:        generationStreamRetention,
		ActiveTTL:        generationStreamActiveTTL,
		LeaseTTL:         generationStreamLeaseTTL,
		LeaseRefresh:     generationStreamLeaseRefresh,
		MaxEvents:        generationStreamMaxEvents,
		SubscriberBuffer: generationStreamSubscriberBuffer,
	}
}

// EnsureMessageGenerationRunID 规范化客户端 run ID；为空时生成新的公开 ID。
func EnsureMessageGenerationRunID(raw string) string {
	runID := normalizeRunID(raw)
	if runID != "" {
		return runID
	}
	return "run_" + normalizePublicID(uuid.NewString())
}

// CancelMessageGeneration 取消用户显式停止的流式生成；浏览器刷新不会走这个路径。
func (s *Service) CancelMessageGeneration(ctx context.Context, userID uint, runID string) bool {
	normalizedRunID := normalizeRunID(runID)
	canceled := s.generationStreams.cancel(ctx, userID, normalizedRunID)
	if !canceled || s == nil || s.repo == nil {
		return canceled
	}
	markCtx := ctx
	var cancel context.CancelFunc
	if markCtx == nil || markCtx.Err() != nil {
		markCtx, cancel = background.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	_, _ = s.repo.CancelPendingGenerationMessagesByRunID(
		markCtx,
		userID,
		normalizedRunID,
		classifyRunErrorCode(ErrMessageGenerationCanceled),
		ErrMessageGenerationCanceled.Error(),
	)
	return true
}

// PublishMessageGenerationEvent 发布由当前请求生命周期持有的生成事件。
func (s *Service) PublishMessageGenerationEvent(ctx context.Context, runID string, payload map[string]any) (map[string]any, bool) {
	return s.generationStreams.publish(ctx, normalizeRunID(runID), payload)
}

// SubscribeMessageGeneration 订阅用户所属 run 的生成流，返回可回放事件和后续事件通道。
func (s *Service) SubscribeMessageGeneration(
	ctx context.Context,
	userID uint,
	runID string,
	afterSeq int64,
	includeSnapshots bool,
) ([]GenerationStreamEvent, <-chan GenerationStreamEvent, func(), bool) {
	return s.generationStreams.subscribe(ctx, userID, normalizeRunID(runID), afterSeq, includeSnapshots)
}

// FinishMessageGeneration 标记当前请求持有的生成流结束，并在短期恢复窗口后释放事件缓存。
func (s *Service) FinishMessageGeneration(ctx context.Context, runID string) {
	if s == nil || s.generationStreams == nil {
		return
	}
	cleanupCtx, cancel := background.WithTimeout(ctx, generationStreamCleanupTimeout)
	defer cancel()
	s.generationStreams.finish(cleanupCtx, normalizeRunID(runID))
}

// AcquireMessageGenerationLifecycle 获取流式消息请求的服务生命周期上下文与租约。
// 调用方必须把返回上下文传入生成链路，并在请求完全收尾后释放租约。
func (s *Service) AcquireMessageGenerationLifecycle(parent context.Context) (context.Context, func(), bool) {
	if s == nil || s.generationStreams == nil {
		return nil, nil, false
	}
	return s.generationStreams.acquireRun(parent)
}

// HasActiveMessageGeneration 判断该 run 是否仍持有活跃生成租约。
func (s *Service) HasActiveMessageGeneration(ctx context.Context, runID string) bool {
	if s == nil || s.generationStreams == nil {
		return false
	}
	return s.generationStreams.hasActive(ctx, normalizeRunID(runID))
}

// MarkMessageGenerationInterrupted 将无法继续恢复的 pending 生成标记为中断。
func (s *Service) MarkMessageGenerationInterrupted(ctx context.Context, userID uint, runID string) {
	if s == nil || s.repo == nil {
		return
	}
	markCtx := ctx
	var cancel context.CancelFunc
	if markCtx == nil || markCtx.Err() != nil {
		markCtx, cancel = background.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	_, _ = s.repo.InterruptPendingAssistantMessageByRunID(
		markCtx,
		userID,
		normalizeRunID(runID),
		ErrMessageGenerationInterrupted.Code(),
		ErrMessageGenerationInterrupted.Message(),
	)
}

func (s *Service) isMessageGenerationCanceled(ctx context.Context, runID string) bool {
	return s.generationStreams.isCanceled(ctx, normalizeRunID(runID))
}

func normalizeRunID(raw string) string {
	value := normalizePublicID(raw)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "run_") {
		value = "run_" + value
	}
	if len(value) > 64 {
		return ""
	}
	return value
}

// GenerationStreamEvent 表示可恢复生成流中的一条有序事件。
type GenerationStreamEvent struct {
	ID      string
	Seq     int64
	Payload map[string]any
}

type activeGeneration struct {
	userID         uint
	conversationID string
	owner          *generationLifecycleOwner
	baseCtx        context.Context
	cancel         context.CancelFunc
	workerCancel   context.CancelFunc
	workerStart    chan struct{}
	workerReady    chan struct{}
	workerDone     chan struct{}
}

func (a *activeGeneration) lease(runID string) repository.GenerationStreamLease {
	if a == nil || a.owner == nil {
		return repository.GenerationStreamLease{}
	}
	return repository.GenerationStreamLease{
		RunID:                runID,
		ExecutionID:          a.owner.executionID,
		UserID:               a.userID,
		ConversationPublicID: a.conversationID,
	}
}

type generationLifecycleOwner struct {
	executionID string
}

type generationLifecycleOwnerContextKey struct{}

type generationStreamRegistry struct {
	mu                       sync.Mutex
	active                   map[string]*activeGeneration
	store                    repository.GenerationStreamCacheRepository
	options                  generationStreamOptions
	lifecycleCtx             context.Context
	lifecycleCancel          context.CancelFunc
	closeOnce                sync.Once
	closed                   bool
	registrationWG           sync.WaitGroup
	activeWorkerWG           sync.WaitGroup
	activeEventReaderStarted bool
	activeEventReaderWG      sync.WaitGroup
	activeSubscriberSeq      uint64
	activeSubscribers        map[uint]map[uint64]chan ActiveMessageGenerationEvent
	completionMu             sync.Mutex
	pendingCompletions       map[string]pendingGenerationCompletion
	completionWake           chan struct{}
	completionRunning        bool
	completionClosed         bool
	completionWG             sync.WaitGroup
}

func newGenerationStreamRegistry(store repository.GenerationStreamCacheRepository, options generationStreamOptions) *generationStreamRegistry {
	if options.Retention <= 0 {
		options.Retention = generationStreamRetention
	}
	if options.ActiveTTL <= 0 {
		options.ActiveTTL = generationStreamActiveTTL
	}
	if options.LeaseTTL <= 0 {
		options.LeaseTTL = generationStreamLeaseTTL
	}
	if options.LeaseRefresh <= 0 || options.LeaseRefresh >= options.LeaseTTL {
		options.LeaseRefresh = options.LeaseTTL / 3
	}
	if options.MaxEvents <= 0 {
		options.MaxEvents = generationStreamMaxEvents
	}
	if options.SubscriberBuffer <= 0 {
		options.SubscriberBuffer = generationStreamSubscriberBuffer
	}
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	return &generationStreamRegistry{
		active:             map[string]*activeGeneration{},
		store:              store,
		options:            options,
		lifecycleCtx:       lifecycleCtx,
		lifecycleCancel:    lifecycleCancel,
		activeSubscribers:  map[uint]map[uint64]chan ActiveMessageGenerationEvent{},
		pendingCompletions: map[string]pendingGenerationCompletion{},
		completionWake:     make(chan struct{}, 1),
	}
}

func (r *generationStreamRegistry) acquireRun(parent context.Context) (context.Context, func(), bool) {
	if r == nil {
		return nil, nil, false
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, nil, false
	}
	lifecycleCtx := r.lifecycleCtx
	r.mu.Unlock()

	if parent == nil {
		parent = context.Background()
	}
	owner := &generationLifecycleOwner{executionID: normalizePublicID(uuid.NewString())}
	ownedCtx := context.WithValue(parent, generationLifecycleOwnerContextKey{}, owner)
	runCtx, cancelRun := context.WithCancel(ownedCtx)
	stopLifecycleCancellation := context.AfterFunc(lifecycleCtx, cancelRun)
	var once sync.Once
	release := func() {
		once.Do(func() {
			stopLifecycleCancellation()
			cancelRun()
		})
	}
	return runCtx, release, true
}

func (r *generationStreamRegistry) close() {
	if r == nil {
		return
	}
	r.closeOnce.Do(func() {
		var activeGenerations []*activeGeneration
		r.mu.Lock()
		r.closed = true
		r.lifecycleCancel()
		activeGenerations = make([]*activeGeneration, 0, len(r.active))
		for _, active := range r.active {
			activeGenerations = append(activeGenerations, active)
		}
		r.mu.Unlock()
		r.closeActiveSubscribers()
		for _, active := range activeGenerations {
			stopActiveWorker(active)
			if active.cancel != nil {
				active.cancel()
			}
		}
		r.registrationWG.Wait()

		r.mu.Lock()
		orphanedGenerations := make(map[string]*activeGeneration, len(r.active))
		for runID, active := range r.active {
			orphanedGenerations[runID] = active
			delete(r.active, runID)
		}
		r.mu.Unlock()
		for runID, active := range orphanedGenerations {
			stopActiveWorker(active)
			cleanupCtx, cleanupCancel := background.WithTimeout(active.baseCtx, generationStreamCleanupTimeout)
			r.finalizeGeneration(cleanupCtx, runID, active)
			cleanupCancel()
		}
		r.activeWorkerWG.Wait()
		r.activeEventReaderWG.Wait()
		r.completionMu.Lock()
		r.completionClosed = true
		r.completionMu.Unlock()
		select {
		case r.completionWake <- struct{}{}:
		default:
		}
		r.completionWG.Wait()
	})
}

func (r *generationStreamRegistry) register(ctx context.Context, runID string, userID uint, conversationPublicID string, cancel context.CancelFunc) error {
	fail := func(err error) error {
		if cancel != nil {
			cancel()
		}
		return err
	}
	if r == nil || runID == "" {
		return fail(context.Canceled)
	}
	conversationPublicID = normalizePublicID(conversationPublicID)
	if userID == 0 || conversationPublicID == "" {
		return fail(context.Canceled)
	}
	owner := generationLifecycleOwnerFromContext(ctx)
	if owner == nil || strings.TrimSpace(owner.executionID) == "" {
		return fail(context.Canceled)
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return fail(context.Canceled)
	}
	r.registrationWG.Add(1)
	r.mu.Unlock()

	registrationCtx, cancelRegistration := context.WithCancel(ctx)
	stopLifecycleCancellation := context.AfterFunc(r.lifecycleCtx, cancelRegistration)
	defer func() {
		stopLifecycleCancellation()
		cancelRegistration()
		r.registrationWG.Done()
	}()
	lease := repository.GenerationStreamLease{
		RunID:                runID,
		ExecutionID:          owner.executionID,
		UserID:               userID,
		ConversationPublicID: conversationPublicID,
	}
	if r.store != nil {
		claimed, err := r.store.ClaimGenerationStream(
			registrationCtx,
			lease,
			r.options.LeaseTTL,
			r.options.ActiveTTL,
		)
		if err != nil {
			cleanupCtx, cleanupCancel := background.WithTimeout(ctx, generationStreamCleanupTimeout)
			_, _ = r.store.AbandonGenerationStream(cleanupCtx, lease)
			cleanupCancel()
			return fail(err)
		}
		if !claimed {
			return fail(ErrDuplicateMessageGenerationRun)
		}
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		if r.store != nil {
			cleanupCtx, cleanupCancel := background.WithTimeout(ctx, generationStreamCleanupTimeout)
			_, _ = r.store.AbandonGenerationStream(cleanupCtx, lease)
			cleanupCancel()
		}
		return fail(context.Canceled)
	}
	if existing := r.active[runID]; existing != nil {
		r.mu.Unlock()
		if r.store != nil && existing.owner != owner {
			cleanupCtx, cleanupCancel := background.WithTimeout(ctx, generationStreamCleanupTimeout)
			_, _ = r.store.AbandonGenerationStream(cleanupCtx, lease)
			cleanupCancel()
		}
		return fail(ErrDuplicateMessageGenerationRun)
	}
	baseCtx := background.Detach(ctx)
	workerCtx, workerCancel := context.WithCancel(baseCtx)
	active := &activeGeneration{
		userID:         userID,
		conversationID: conversationPublicID,
		owner:          owner,
		baseCtx:        baseCtx,
		cancel:         cancel,
		workerCancel:   workerCancel,
		workerStart:    make(chan struct{}),
		workerReady:    make(chan struct{}),
		workerDone:     make(chan struct{}),
	}
	r.activeWorkerWG.Add(1)
	r.active[runID] = active
	go r.runActiveWorker(workerCtx, runID, active)
	r.mu.Unlock()

	close(active.workerStart)
	<-active.workerReady
	r.mu.Lock()
	current := !r.closed && r.active[runID] == active
	r.mu.Unlock()
	if !current {
		return fail(context.Canceled)
	}
	r.publishActiveEvent(registrationCtx, userID, "started", runID, conversationPublicID)
	return nil
}

func (r *generationStreamRegistry) cancel(ctx context.Context, userID uint, runID string) bool {
	if r == nil || runID == "" || userID == 0 {
		return false
	}
	if r.store != nil {
		requested, err := r.store.RequestGenerationStreamCancel(ctx, runID, userID, r.options.Retention)
		if err != nil || !requested {
			return false
		}
		if active := r.localActive(runID); active != nil && active.userID == userID && active.cancel != nil {
			active.cancel()
		}
		return true
	}
	active := r.localActive(runID)
	if active == nil || active.userID != userID {
		return false
	}
	if active.cancel != nil {
		active.cancel()
	}
	return true
}

// cancelForced cancels a run without owner checks (internal system paths such as moderation).
func (r *generationStreamRegistry) cancelForced(ctx context.Context, runID string) bool {
	if r == nil || runID == "" {
		return false
	}
	active := r.localActive(runID)
	if active == nil {
		return false
	}
	if r.store != nil {
		requested, err := r.store.RequestGenerationStreamCancel(ctx, runID, active.userID, r.options.Retention)
		if err != nil || !requested {
			return false
		}
	}
	if active.cancel != nil {
		active.cancel()
	}
	return true
}

func (r *generationStreamRegistry) isCanceled(ctx context.Context, runID string) bool {
	if runID == "" {
		return false
	}
	if r.store != nil {
		if canceled, err := r.store.IsGenerationStreamCanceled(ctx, runID); err == nil && canceled {
			return true
		}
	}
	return false
}

func (r *generationStreamRegistry) publish(ctx context.Context, runID string, payload map[string]any) (map[string]any, bool) {
	if runID == "" {
		return payload, false
	}
	lease, ok := r.leaseForContext(ctx, runID)
	if !ok {
		return payload, false
	}
	return r.publishWithLease(ctx, lease, payload)
}

func (r *generationStreamRegistry) publishCurrent(ctx context.Context, runID string, payload map[string]any) {
	active := r.localActive(runID)
	if active == nil {
		return
	}
	_, _ = r.publishWithLease(ctx, active.lease(runID), payload)
}

func (r *generationStreamRegistry) publishWithLease(
	ctx context.Context,
	lease repository.GenerationStreamLease,
	payload map[string]any,
) (map[string]any, bool) {
	actual := cloneStreamPayload(payload)
	persisted, sanitized := generationStreamPayloadForStore(actual)
	payloadJSON, err := marshalStreamPayload(persisted)
	if err != nil {
		return actual, true
	}
	appendInput := repository.GenerationStreamAppend{PayloadJSON: payloadJSON}
	if streamString(persisted["type"]) == "delta" {
		appendInput.TextDelta = streamString(persisted["delta"])
	}
	appendInput.UpstreamThink = generationStreamUpstreamThinkAppend(actual)
	record, accepted, err := r.append(ctx, r.store, lease, appendInput)
	if err != nil {
		return actual, true
	}
	if !accepted {
		r.cancelLocalExecution(lease.RunID, lease.ExecutionID)
		return actual, false
	}
	if record.Seq > 0 {
		actual["seq"] = record.Seq
		if sanitized {
			persisted["seq"] = record.Seq
		}
	}
	if shouldReturnSanitizedGenerationStreamPayload(actual, sanitized) {
		return persisted, true
	}
	return actual, true
}

func (r *generationStreamRegistry) resetCurrentEvents(ctx context.Context, runID string) {
	if runID == "" || r.store == nil {
		return
	}
	active := r.localActive(runID)
	if active == nil {
		return
	}
	_, _ = r.store.ResetGenerationStreamEvents(ctx, active.lease(runID))
}

func (r *generationStreamRegistry) append(
	ctx context.Context,
	store repository.GenerationStreamCacheRepository,
	lease repository.GenerationStreamLease,
	input repository.GenerationStreamAppend,
) (repository.GenerationStreamMessage, bool, error) {
	if store == nil {
		return repository.GenerationStreamMessage{}, true, nil
	}
	return store.AppendGenerationStreamEvent(ctx, lease, input, int64(r.options.MaxEvents), r.options.ActiveTTL)
}

func (r *generationStreamRegistry) subscribe(
	ctx context.Context,
	userID uint,
	runID string,
	afterSeq int64,
	includeSnapshots bool,
) ([]GenerationStreamEvent, <-chan GenerationStreamEvent, func(), bool) {
	if runID == "" {
		return nil, nil, nil, false
	}
	return r.subscribeStore(ctx, r.store, userID, runID, afterSeq, includeSnapshots)
}

func (r *generationStreamRegistry) subscribeStore(
	ctx context.Context,
	store repository.GenerationStreamCacheRepository,
	userID uint,
	runID string,
	afterSeq int64,
	includeSnapshots bool,
) ([]GenerationStreamEvent, <-chan GenerationStreamEvent, func(), bool) {
	if store == nil || !r.authorized(ctx, store, runID, userID) {
		return nil, nil, nil, false
	}
	retained, err := store.ListGenerationStreamEvents(ctx, runID, int64(r.options.MaxEvents))
	if err != nil {
		return nil, nil, nil, false
	}
	textSnapshot := repository.GenerationStreamTextSnapshot{}
	hasTextSnapshot := false
	upstreamThinkSnapshot := repository.GenerationStreamUpstreamThinkSnapshot{}
	hasUpstreamThinkSnapshot := false
	if includeSnapshots {
		// Read checkpoints after the retained window. Events appended in between
		// are read again from cursor; deltas already covered by a checkpoint are
		// filtered by its seq while non-content events remain replayable.
		textSnapshot, hasTextSnapshot, err = store.GetGenerationStreamTextSnapshot(ctx, runID)
		if err != nil {
			return nil, nil, nil, false
		}
		upstreamThinkSnapshot, hasUpstreamThinkSnapshot, err = store.GetGenerationStreamUpstreamThinkSnapshot(ctx, runID)
		if err != nil {
			return nil, nil, nil, false
		}
	}
	if !r.authorized(ctx, store, runID, userID) {
		return nil, nil, nil, false
	}
	replay, cursor, terminal, safe := retainedStreamEvents(retainedStreamEventsInput{
		Records:                  retained,
		AfterSeq:                 afterSeq,
		TextSnapshot:             textSnapshot,
		HasTextSnapshot:          hasTextSnapshot,
		UpstreamThinkSnapshot:    upstreamThinkSnapshot,
		HasUpstreamThinkSnapshot: hasUpstreamThinkSnapshot,
		IncludeSnapshots:         includeSnapshots,
	})
	if !safe {
		return nil, nil, nil, false
	}
	events := make(chan GenerationStreamEvent, r.options.SubscriberBuffer)
	if terminal {
		close(events)
		return replay, events, func() {}, true
	}

	readCtx, cancel := context.WithCancel(ctx)
	go r.readStoreEvents(readCtx, readStoreEventsInput{
		Store:                    store,
		UserID:                   userID,
		RunID:                    runID,
		Cursor:                   cursor,
		AfterSeq:                 afterSeq,
		TextSnapshotSeq:          textSnapshot.Seq,
		UpstreamThinkSnapshotSeq: upstreamThinkSnapshot.Seq,
		Output:                   events,
	})
	return replay, events, cancel, true
}

func (r *generationStreamRegistry) readStoreEvents(ctx context.Context, input readStoreEventsInput) {
	defer close(input.Output)
	if strings.TrimSpace(input.Cursor) == "" {
		input.Cursor = "0-0"
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		records, err := input.Store.ReadGenerationStreamEvents(ctx, input.RunID, input.Cursor, generationStreamReadBlock, int64(r.options.SubscriberBuffer))
		if err != nil {
			return
		}
		if !r.authorized(ctx, input.Store, input.RunID, input.UserID) {
			return
		}
		for _, record := range records {
			if strings.TrimSpace(record.ID) != "" {
				input.Cursor = record.ID
			}
			if record.Seq <= input.AfterSeq {
				continue
			}
			event, ok := decodeStreamRecord(record)
			if !ok {
				continue
			}
			if streamString(event.Payload["type"]) == "delta" && event.Seq <= input.TextSnapshotSeq {
				continue
			}
			if streamString(event.Payload["type"]) == "upstream_think_delta" && event.Seq <= input.UpstreamThinkSnapshotSeq {
				continue
			}
			input.AfterSeq = event.Seq
			select {
			case <-ctx.Done():
				return
			case input.Output <- event:
			}
			if isTerminalStreamPayload(event.Payload) {
				return
			}
		}
	}
}

func (r *generationStreamRegistry) finish(ctx context.Context, runID string) {
	if runID == "" {
		return
	}
	owner := generationLifecycleOwnerFromContext(ctx)
	r.mu.Lock()
	active, ok := r.active[runID]
	if ok && active.owner != owner {
		r.mu.Unlock()
		return
	}
	if ok {
		delete(r.active, runID)
	}
	r.mu.Unlock()
	if ok {
		stopActiveWorker(active)
	}
	if active == nil {
		return
	}
	r.finalizeGeneration(ctx, runID, active)
}

func (r *generationStreamRegistry) finalizeGeneration(ctx context.Context, runID string, active *activeGeneration) {
	if active == nil {
		return
	}
	lease := active.lease(runID)
	completed, err := r.completeGenerationStream(ctx, lease)
	if err != nil {
		r.enqueueCompletion(lease)
		return
	}
	if completed {
		r.publishActiveEvent(ctx, active.userID, "finished", runID, active.conversationID)
	}
}

func (r *generationStreamRegistry) completeGenerationStream(ctx context.Context, lease repository.GenerationStreamLease) (bool, error) {
	if r.store == nil {
		return true, nil
	}
	var lastErr error
	for attempt := 0; attempt < generationStreamCompletionAttempts; attempt++ {
		completed, err := r.store.CompleteGenerationStream(ctx, lease, r.options.Retention)
		if err == nil {
			return completed, nil
		}
		lastErr = err
		if attempt == generationStreamCompletionAttempts-1 {
			return false, lastErr
		}
		timer := time.NewTimer(generationStreamCompletionRetryDelay << attempt)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false, ctx.Err()
		case <-timer.C:
		}
	}
	return false, lastErr
}

func (r *generationStreamRegistry) enqueueCompletion(lease repository.GenerationStreamLease) {
	if r == nil || r.store == nil || strings.TrimSpace(lease.RunID) == "" {
		return
	}
	r.completionMu.Lock()
	if r.completionClosed {
		r.completionMu.Unlock()
		return
	}
	key := generationCompletionKey(lease)
	if _, exists := r.pendingCompletions[key]; !exists {
		r.pendingCompletions[key] = pendingGenerationCompletion{
			lease:       lease,
			nextAttempt: time.Now(),
			retryDelay:  generationStreamCompletionRetryDelay,
			expiresAt:   time.Now().Add(r.options.ActiveTTL),
		}
	}
	if !r.completionRunning {
		r.completionRunning = true
		r.completionWG.Add(1)
		go r.runCompletionWorker()
	}
	r.completionMu.Unlock()
	select {
	case r.completionWake <- struct{}{}:
	default:
	}
}

func (r *generationStreamRegistry) runCompletionWorker() {
	defer r.completionWG.Done()
	for {
		if r.isClosed() {
			r.completionMu.Lock()
			r.completionRunning = false
			r.completionMu.Unlock()
			return
		}
		pending, wait, ok := r.nextPendingCompletion()
		if !ok {
			r.completionMu.Lock()
			if len(r.pendingCompletions) == 0 {
				r.completionRunning = false
				r.completionMu.Unlock()
				return
			}
			r.completionMu.Unlock()
			continue
		}
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-r.completionWake:
				timer.Stop()
				continue
			case <-timer.C:
			}
		}
		if !pending.expiresAt.IsZero() && time.Now().After(pending.expiresAt) {
			r.completionMu.Lock()
			delete(r.pendingCompletions, generationCompletionKey(pending.lease))
			r.completionMu.Unlock()
			continue
		}
		cleanupCtx, cleanupCancel := background.WithTimeout(context.TODO(), generationStreamCleanupTimeout)
		completed, err := r.completeGenerationStream(cleanupCtx, pending.lease)
		cleanupCancel()
		if err != nil {
			pending.nextAttempt = time.Now().Add(pending.retryDelay)
			pending.retryDelay *= 2
			if pending.retryDelay > generationStreamCompletionMaxDelay {
				pending.retryDelay = generationStreamCompletionMaxDelay
			}
			r.completionMu.Lock()
			if !r.completionClosed {
				r.pendingCompletions[generationCompletionKey(pending.lease)] = pending
			}
			r.completionMu.Unlock()
			continue
		}
		r.completionMu.Lock()
		delete(r.pendingCompletions, generationCompletionKey(pending.lease))
		r.completionMu.Unlock()
		if completed {
			r.publishActiveEvent(background.Detach(context.TODO()), pending.lease.UserID, "finished", pending.lease.RunID, pending.lease.ConversationPublicID)
		}
	}
}

func generationCompletionKey(lease repository.GenerationStreamLease) string {
	return strings.TrimSpace(lease.RunID) + "\x00" + strings.TrimSpace(lease.ExecutionID)
}

func (r *generationStreamRegistry) nextPendingCompletion() (pendingGenerationCompletion, time.Duration, bool) {
	r.completionMu.Lock()
	defer r.completionMu.Unlock()
	var selected pendingGenerationCompletion
	found := false
	for _, pending := range r.pendingCompletions {
		if !found || pending.nextAttempt.Before(selected.nextAttempt) {
			selected = pending
			found = true
		}
	}
	if !found {
		return pendingGenerationCompletion{}, 0, false
	}
	wait := time.Until(selected.nextAttempt)
	if wait < 0 {
		wait = 0
	}
	return selected, wait, true
}

func (r *generationStreamRegistry) isClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

func generationLifecycleOwnerFromContext(ctx context.Context) *generationLifecycleOwner {
	if ctx == nil {
		return nil
	}
	owner, _ := ctx.Value(generationLifecycleOwnerContextKey{}).(*generationLifecycleOwner)
	return owner
}

func (r *generationStreamRegistry) authorized(ctx context.Context, store repository.GenerationStreamCacheRepository, runID string, userID uint) bool {
	if store == nil || userID == 0 {
		return false
	}
	ownerID, ok, err := store.GetGenerationStreamOwner(ctx, runID)
	if err != nil || !ok {
		return false
	}
	return ownerID == userID
}

func (r *generationStreamRegistry) runActiveWorker(ctx context.Context, runID string, active *activeGeneration) {
	defer r.activeWorkerWG.Done()
	defer close(active.workerDone)
	select {
	case <-ctx.Done():
		close(active.workerReady)
		return
	case <-active.workerStart:
	}
	close(active.workerReady)
	if ctx.Err() != nil {
		return
	}
	leaseValidUntil := time.Now().Add(r.options.LeaseTTL)

	activeTTL := r.options.ActiveTTL
	if activeTTL <= 0 {
		activeTTL = generationStreamActiveTTL
	}
	expiryTimer := time.NewTimer(activeTTL)
	defer expiryTimer.Stop()
	leaseTicker := time.NewTicker(r.options.LeaseRefresh)
	defer leaseTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-leaseTicker.C:
			renewed, err := r.renewActiveLease(ctx, runID, active)
			if err == nil && renewed {
				leaseValidUntil = time.Now().Add(r.options.LeaseTTL)
				continue
			}
			if err != nil && time.Now().Before(leaseValidUntil) {
				continue
			}
			if r.removeActiveExecution(runID, active) {
				if active.cancel != nil {
					active.cancel()
				}
				cleanupCtx, cleanupCancel := background.WithTimeout(active.baseCtx, generationStreamCleanupTimeout)
				r.finalizeGeneration(cleanupCtx, runID, active)
				cleanupCancel()
			}
			return
		case <-expiryTimer.C:
			expired := r.removeActiveExecution(runID, active)
			if !expired {
				return
			}
			if active.cancel != nil {
				active.cancel()
			}
			cleanupCtx, cleanupCancel := background.WithTimeout(active.baseCtx, generationStreamCleanupTimeout)
			r.finalizeGeneration(cleanupCtx, runID, active)
			cleanupCancel()
			return
		}
	}
}

func (r *generationStreamRegistry) hasActive(ctx context.Context, runID string) bool {
	if runID == "" {
		return false
	}
	if r.store != nil {
		if active, err := r.store.IsGenerationStreamActive(ctx, runID); err == nil && active {
			return true
		}
	}
	return false
}

func (r *generationStreamRegistry) renewActiveLease(ctx context.Context, runID string, active *activeGeneration) (bool, error) {
	if runID == "" || active == nil || r.store == nil {
		return true, nil
	}
	return r.store.RenewGenerationStreamLease(
		ctx,
		active.lease(runID),
		r.options.LeaseTTL,
		r.options.ActiveTTL,
	)
}

func (r *generationStreamRegistry) localActive(runID string) *activeGeneration {
	if r == nil || runID == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.active[runID]
}

func (r *generationStreamRegistry) leaseForContext(ctx context.Context, runID string) (repository.GenerationStreamLease, bool) {
	owner := generationLifecycleOwnerFromContext(ctx)
	if owner == nil {
		return repository.GenerationStreamLease{}, false
	}
	active := r.localActive(runID)
	if active == nil || active.owner != owner {
		return repository.GenerationStreamLease{}, false
	}
	return active.lease(runID), true
}

func (r *generationStreamRegistry) cancelLocalExecution(runID string, executionID string) {
	active := r.localActive(runID)
	if active == nil || active.owner == nil || active.owner.executionID != executionID {
		return
	}
	if active.cancel != nil {
		active.cancel()
	}
}

func (r *generationStreamRegistry) removeActiveExecution(runID string, active *activeGeneration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active[runID] != active {
		return false
	}
	delete(r.active, runID)
	return true
}

func stopActiveWorker(active *activeGeneration) {
	if active == nil {
		return
	}
	if active.workerCancel != nil {
		active.workerCancel()
	}
	if active.workerDone != nil {
		<-active.workerDone
	}
}

func retainedStreamEvents(input retainedStreamEventsInput) ([]GenerationStreamEvent, string, bool, bool) {
	replay := make([]GenerationStreamEvent, 0, len(input.Records)+2)
	cursor := "0-0"
	terminal := false
	textSnapshotPending := input.IncludeSnapshots && input.HasTextSnapshot
	upstreamThinkSnapshotPending := input.IncludeSnapshots && input.HasUpstreamThinkSnapshot
	appendTextSnapshot := func() {
		if !textSnapshotPending {
			return
		}
		replay = append(replay, GenerationStreamEvent{
			Seq: input.TextSnapshot.Seq,
			Payload: map[string]any{
				"type":    "delta",
				"seq":     input.TextSnapshot.Seq,
				"delta":   input.TextSnapshot.Content,
				"replace": true,
			},
		})
		textSnapshotPending = false
	}
	appendUpstreamThinkSnapshot := func() {
		if !upstreamThinkSnapshotPending {
			return
		}
		replay = append(replay, GenerationStreamEvent{
			Seq:     input.UpstreamThinkSnapshot.Seq,
			Payload: generationStreamUpstreamThinkSnapshotPayload(input.UpstreamThinkSnapshot),
		})
		upstreamThinkSnapshotPending = false
	}
	appendSnapshotsBefore := func(seq int64) {
		for (textSnapshotPending && input.TextSnapshot.Seq < seq) ||
			(upstreamThinkSnapshotPending && input.UpstreamThinkSnapshot.Seq < seq) {
			if textSnapshotPending && (!upstreamThinkSnapshotPending || input.TextSnapshot.Seq <= input.UpstreamThinkSnapshot.Seq) {
				appendTextSnapshot()
				continue
			}
			appendUpstreamThinkSnapshot()
		}
	}
	for _, record := range input.Records {
		if strings.TrimSpace(record.ID) != "" {
			cursor = record.ID
		}
		event, ok := decodeStreamRecord(record)
		if !ok {
			continue
		}
		appendSnapshotsBefore(event.Seq)
		if isTerminalStreamPayload(event.Payload) {
			terminal = true
		}
		if streamString(event.Payload["type"]) == "delta" {
			// A text delta without a cumulative checkpoint cannot be replayed
			// safely once the bounded event window has trimmed older chunks.
			if input.IncludeSnapshots && !input.HasTextSnapshot {
				return nil, cursor, terminal, false
			}
			if input.IncludeSnapshots && event.Seq <= input.TextSnapshot.Seq {
				continue
			}
		}
		if streamString(event.Payload["type"]) == "upstream_think_delta" {
			if input.IncludeSnapshots && !input.HasUpstreamThinkSnapshot && upstreamThinkPayloadHasContent(event.Payload) {
				return nil, cursor, terminal, false
			}
			if input.IncludeSnapshots && event.Seq <= input.UpstreamThinkSnapshot.Seq {
				continue
			}
		}
		if event.Seq > input.AfterSeq {
			replay = append(replay, event)
		}
	}
	if textSnapshotPending && upstreamThinkSnapshotPending {
		if input.TextSnapshot.Seq <= input.UpstreamThinkSnapshot.Seq {
			appendTextSnapshot()
		} else {
			appendUpstreamThinkSnapshot()
		}
	}
	appendTextSnapshot()
	appendUpstreamThinkSnapshot()
	return replay, cursor, terminal, true
}

func generationStreamUpstreamThinkAppend(payload map[string]any) *repository.GenerationStreamUpstreamThinkAppend {
	if streamString(payload["type"]) != "upstream_think_delta" {
		return nil
	}
	delta, _ := payload["delta"].(string)
	contentMarkdown, hasContent := payload["contentMarkdown"].(string)
	roundID := strings.TrimSpace(streamString(payload["roundID"]))
	metadata := map[string]any{"type": "upstream_think_delta"}
	for _, key := range []string{"status", "title", "summary", "stage", "eventID", "startedAt", "endedAt", "kind"} {
		if value, ok := payload[key]; ok {
			metadata[key] = value
		}
	}
	if roundID != "" {
		metadata["roundID"] = roundID
	}
	metadataJSON, err := marshalStreamPayload(metadata)
	if err != nil {
		return nil
	}
	return &repository.GenerationStreamUpstreamThinkAppend{
		RoundID:         roundID,
		Delta:           delta,
		ContentMarkdown: contentMarkdown,
		Replace:         hasContent,
		MetadataJSON:    metadataJSON,
	}
}

func generationStreamUpstreamThinkSnapshotPayload(snapshot repository.GenerationStreamUpstreamThinkSnapshot) map[string]any {
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(snapshot.MetadataJSON), &payload); err != nil || payload == nil {
		payload = map[string]any{}
	}
	payload["type"] = "upstream_think_delta"
	payload["seq"] = snapshot.Seq
	payload["contentMarkdown"] = snapshot.ContentMarkdown
	if strings.TrimSpace(streamString(payload["roundID"])) == "" && snapshot.RoundID != "" {
		payload["roundID"] = snapshot.RoundID
	}
	delete(payload, "delta")
	return payload
}

func upstreamThinkPayloadHasContent(payload map[string]any) bool {
	_, hasDelta := payload["delta"].(string)
	_, hasContent := payload["contentMarkdown"].(string)
	return hasDelta || hasContent
}

func decodeStreamRecord(record repository.GenerationStreamMessage) (GenerationStreamEvent, bool) {
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(record.PayloadJSON), &payload); err != nil {
		return GenerationStreamEvent{}, false
	}
	seq := record.Seq
	if seq <= 0 {
		seq = int64FromPayload(payload["seq"])
	}
	if seq <= 0 {
		return GenerationStreamEvent{}, false
	}
	payload["seq"] = seq
	return GenerationStreamEvent{ID: record.ID, Seq: seq, Payload: payload}, true
}

func marshalStreamPayload(payload map[string]any) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func generationStreamPayloadForStore(payload map[string]any) (map[string]any, bool) {
	if !isTraceUpdateStreamPayload(payload) {
		return payload, false
	}
	payloadJSON, err := marshalStreamPayload(payload)
	if err != nil || len(payloadJSON) <= generationStreamMaxPayloadBytes {
		return payload, false
	}
	sanitized := sanitizeGenerationStreamPayload(payload)
	payloadJSON, err = marshalStreamPayload(sanitized)
	if err != nil || len(payloadJSON) <= generationStreamMaxPayloadBytes {
		return sanitized, true
	}
	return compactOversizedGenerationStreamPayload(sanitized), true
}

func shouldReturnSanitizedGenerationStreamPayload(actual map[string]any, sanitized bool) bool {
	return sanitized && isTraceUpdateStreamPayload(actual)
}

func isTraceUpdateStreamPayload(payload map[string]any) bool {
	switch strings.TrimSpace(streamString(payload["type"])) {
	case "process_update", "upstream_think_delta":
		return true
	default:
		return false
	}
}

func sanitizeGenerationStreamPayload(payload map[string]any) map[string]any {
	raw, err := json.Marshal(payload)
	if err != nil {
		next := cloneStreamPayload(payload)
		next["payloadTruncated"] = true
		return next
	}
	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		next := cloneStreamPayload(payload)
		next["payloadTruncated"] = true
		return next
	}
	sanitized, _ := sanitizeGenerationStreamValue(normalized).(map[string]any)
	if sanitized == nil {
		sanitized = map[string]any{}
	}
	sanitized["payloadTruncated"] = true
	return sanitized
}

func sanitizeGenerationStreamValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		next := make(map[string]any, len(typed))
		for key, item := range typed {
			if shouldDropStreamTraceField(key) {
				next[key+"_size"] = len(strings.TrimSpace(streamString(item)))
				next[key+"_truncated"] = true
				continue
			}
			if isTracePayloadJSONField(key) {
				if sanitized := sanitizeStreamTracePayloadJSON(streamString(item)); sanitized != "" {
					next[key] = sanitized
				}
				continue
			}
			if key == "tool_calls" {
				next[key] = sanitizeStreamToolCalls(item)
				continue
			}
			next[key] = sanitizeGenerationStreamValue(item)
		}
		return next
	case []any:
		next := make([]any, 0, len(typed))
		for _, item := range typed {
			next = append(next, sanitizeGenerationStreamValue(item))
		}
		return next
	default:
		return value
	}
}

func sanitizeStreamToolCalls(value any) any {
	items, ok := value.([]any)
	if !ok {
		return sanitizeGenerationStreamValue(value)
	}
	next := make([]any, 0, len(items))
	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			next = append(next, sanitizeGenerationStreamValue(item))
			continue
		}
		next = append(next, sanitizeGenerationStreamValue(record))
	}
	return next
}

func sanitizeStreamTracePayloadJSON(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	var payload any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return ""
	}
	sanitized := sanitizeGenerationStreamValue(payload)
	data, err := json.Marshal(sanitized)
	if err != nil || string(data) == "{}" {
		return ""
	}
	return string(data)
}

func compactOversizedGenerationStreamPayload(payload map[string]any) map[string]any {
	eventType := strings.TrimSpace(streamString(payload["type"]))
	next := map[string]any{
		"type":             eventType,
		"payloadTruncated": true,
	}
	for _, key := range []string{"status", "message", "errorCode", "code"} {
		if value := strings.TrimSpace(streamString(payload[key])); value != "" {
			next[key] = textutil.CompactSnippet(value, 512)
		}
	}
	if eventType == "" {
		next["type"] = "stream_update"
	}
	return next
}

func shouldDropStreamTraceField(key string) bool {
	switch key {
	case "input", "input_detail", "output", "output_text", "output_detail":
		return true
	default:
		return false
	}
}

func isTracePayloadJSONField(key string) bool {
	return key == "payloadJSON" || key == "PayloadJSON" || key == "payloadJson"
}

func streamString(value any) string {
	text, _ := value.(string)
	return text
}

func cloneStreamPayload(payload map[string]any) map[string]any {
	next := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		next[key] = value
	}
	return next
}

func isTerminalStreamPayload(payload map[string]any) bool {
	eventType, _ := payload["type"].(string)
	return eventType == "completed" || eventType == "error" || eventType == "moderation_blocked"
}

func int64FromPayload(raw any) int64 {
	switch value := raw.(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	case json.Number:
		n, _ := value.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return n
	default:
		return 0
	}
}
