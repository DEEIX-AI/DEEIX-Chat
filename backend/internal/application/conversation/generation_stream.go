package conversation

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/google/uuid"
)

const (
	generationStreamRetention        = 15 * time.Minute
	generationStreamActiveTTL        = 2 * time.Hour
	generationStreamLeaseTTL         = 30 * time.Second
	generationStreamLeaseRefresh     = 10 * time.Second
	generationStreamMaxEvents        = 1024
	generationStreamSubscriberBuffer = 128
	generationStreamReadBlock        = 5 * time.Second
	generationStreamMaxPayloadBytes  = 128 * 1024
)

type generationStreamOptions struct {
	Retention        time.Duration
	ActiveTTL        time.Duration
	LeaseTTL         time.Duration
	LeaseRefresh     time.Duration
	MaxEvents        int
	SubscriberBuffer int
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
		markCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
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

// PublishMessageGenerationEvent 发布生成流事件，并返回带 seq 的实际载荷。
func (s *Service) PublishMessageGenerationEvent(runID string, payload map[string]interface{}) map[string]interface{} {
	return s.generationStreams.publish(context.Background(), normalizeRunID(runID), payload)
}

// SubscribeMessageGeneration 订阅用户所属 run 的生成流，返回可回放事件和后续事件通道。
func (s *Service) SubscribeMessageGeneration(
	ctx context.Context,
	userID uint,
	runID string,
	afterSeq int64,
	includeTextSnapshot bool,
) ([]GenerationStreamEvent, <-chan GenerationStreamEvent, func(), bool) {
	return s.generationStreams.subscribe(ctx, userID, normalizeRunID(runID), afterSeq, includeTextSnapshot)
}

// FinishMessageGeneration 标记生成流结束，并在短期恢复窗口后释放事件缓存。
func (s *Service) FinishMessageGeneration(runID string) {
	s.generationStreams.finish(context.Background(), normalizeRunID(runID))
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
		markCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	_, _ = s.repo.InterruptPendingAssistantMessageByRunID(
		markCtx,
		userID,
		normalizeRunID(runID),
		"stream_interrupted",
		"generation stream was interrupted; retry this message",
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
	Payload map[string]interface{}
}

type activeGeneration struct {
	userID          uint
	conversationID  string
	cancel          context.CancelFunc
	leaseCancel     context.CancelFunc
	maxRuntimeTimer *time.Timer
}

type generationStreamRegistry struct {
	mu                       sync.Mutex
	active                   map[string]*activeGeneration
	store                    repository.GenerationStreamCacheRepository
	options                  generationStreamOptions
	activeEventReaderStarted bool
	activeSubscriberSeq      uint64
	activeSubscribers        map[uint]map[uint64]chan ActiveMessageGenerationEvent
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
	return &generationStreamRegistry{
		active:            map[string]*activeGeneration{},
		store:             store,
		options:           options,
		activeSubscribers: map[uint]map[uint64]chan ActiveMessageGenerationEvent{},
	}
}

func (r *generationStreamRegistry) register(ctx context.Context, runID string, userID uint, conversationPublicID string, cancel context.CancelFunc) {
	if runID == "" {
		if cancel != nil {
			cancel()
		}
		return
	}
	conversationPublicID = normalizePublicID(conversationPublicID)
	if userID == 0 || conversationPublicID == "" {
		if cancel != nil {
			cancel()
		}
		return
	}
	if r.store != nil {
		_ = r.store.RegisterGenerationStream(ctx, runID, userID, conversationPublicID, r.options.ActiveTTL)
	}

	var replaced *activeGeneration
	r.mu.Lock()
	replaced = r.active[runID]
	active := &activeGeneration{userID: userID, conversationID: conversationPublicID, cancel: cancel}
	active.leaseCancel = r.startActiveLease(runID, userID)
	r.active[runID] = active
	r.scheduleActiveExpiryLocked(runID, active)
	r.mu.Unlock()

	if replaced != nil {
		stopActiveGeneration(replaced)
		if replaced.cancel != nil {
			replaced.cancel()
		}
		if replaced.userID != userID || replaced.conversationID != conversationPublicID {
			r.publishActiveEvent(context.Background(), replaced.userID, "finished", runID, replaced.conversationID)
		}
	}
	r.publishActiveEvent(context.Background(), userID, "started", runID, conversationPublicID)
}

func (r *generationStreamRegistry) cancel(ctx context.Context, userID uint, runID string) bool {
	if runID == "" {
		return false
	}
	if !r.authorized(ctx, r.store, runID, userID) {
		return false
	}
	return r.cancelActive(ctx, userID, runID, false)
}

// cancelForced cancels a run without owner checks (internal system paths such as moderation).
func (r *generationStreamRegistry) cancelForced(ctx context.Context, runID string) bool {
	if runID == "" {
		return false
	}
	return r.cancelActive(ctx, 0, runID, true)
}

func (r *generationStreamRegistry) cancelActive(ctx context.Context, userID uint, runID string, force bool) bool {
	if r.store != nil {
		_ = r.store.RequestGenerationStreamCancel(ctx, runID, r.options.Retention)
	}

	var active *activeGeneration
	var ok bool
	if force {
		active, ok = r.deleteActiveAny(runID)
	} else {
		active, ok = r.deleteActive(userID, runID)
	}
	if ok {
		stopActiveGeneration(active)
	}
	if ok && active.cancel != nil {
		active.cancel()
	}
	clearUserID := userID
	if active != nil && active.userID != 0 {
		clearUserID = active.userID
	}
	r.clearActive(context.Background(), runID, clearUserID)
	conversationID := ""
	if active != nil {
		conversationID = active.conversationID
	}
	r.publishActiveEvent(context.Background(), clearUserID, "finished", runID, conversationID)
	return true
}

func (r *generationStreamRegistry) deleteActiveAny(runID string) (*activeGeneration, bool) {
	if runID == "" {
		return nil, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	active, ok := r.active[runID]
	if !ok {
		return nil, false
	}
	delete(r.active, runID)
	return active, true
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

func (r *generationStreamRegistry) publish(ctx context.Context, runID string, payload map[string]interface{}) map[string]interface{} {
	if runID == "" {
		return payload
	}
	r.touchActive(ctx, runID)
	actual := cloneStreamPayload(payload)
	persisted, sanitized := generationStreamPayloadForStore(actual)
	payloadJSON, err := marshalStreamPayload(persisted)
	if err != nil {
		return actual
	}
	appendInput := repository.GenerationStreamAppend{PayloadJSON: payloadJSON}
	if streamString(persisted["type"]) == "delta" {
		appendInput.TextDelta = streamString(persisted["delta"])
	}
	record, err := r.append(ctx, r.store, runID, appendInput)
	if err == nil && record.Seq > 0 {
		actual["seq"] = record.Seq
		if sanitized {
			persisted["seq"] = record.Seq
		}
	}
	if shouldReturnSanitizedGenerationStreamPayload(actual, sanitized) {
		return persisted
	}
	return actual
}

// resetEvents clears retained stream events so blocked content cannot be replayed.
func (r *generationStreamRegistry) resetEvents(ctx context.Context, runID string) {
	if runID == "" || r.store == nil {
		return
	}
	_ = r.store.ResetGenerationStreamEvents(ctx, runID)
}

func (r *generationStreamRegistry) append(ctx context.Context, store repository.GenerationStreamCacheRepository, runID string, input repository.GenerationStreamAppend) (repository.GenerationStreamMessage, error) {
	if store == nil {
		return repository.GenerationStreamMessage{}, nil
	}
	return store.AppendGenerationStreamEvent(ctx, runID, input, int64(r.options.MaxEvents), r.options.ActiveTTL)
}

func (r *generationStreamRegistry) subscribe(
	ctx context.Context,
	userID uint,
	runID string,
	afterSeq int64,
	includeTextSnapshot bool,
) ([]GenerationStreamEvent, <-chan GenerationStreamEvent, func(), bool) {
	if runID == "" {
		return nil, nil, nil, false
	}
	return r.subscribeStore(ctx, r.store, userID, runID, afterSeq, includeTextSnapshot)
}

func (r *generationStreamRegistry) subscribeStore(
	ctx context.Context,
	store repository.GenerationStreamCacheRepository,
	userID uint,
	runID string,
	afterSeq int64,
	includeTextSnapshot bool,
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
	if includeTextSnapshot {
		// Read the snapshot after the retained window. Events appended in between
		// are read again from cursor; visible deltas already covered by the snapshot
		// are filtered by snapshot seq while non-text events remain replayable.
		textSnapshot, hasTextSnapshot, err = store.GetGenerationStreamTextSnapshot(ctx, runID)
		if err != nil {
			return nil, nil, nil, false
		}
	}
	replay, cursor, terminal, safe := retainedStreamEvents(retained, afterSeq, textSnapshot, hasTextSnapshot, includeTextSnapshot)
	if !safe {
		return nil, nil, nil, false
	}
	events := make(chan GenerationStreamEvent, r.options.SubscriberBuffer)
	if terminal {
		close(events)
		return replay, events, func() {}, true
	}

	readCtx, cancel := context.WithCancel(ctx)
	go r.readStoreEvents(readCtx, store, runID, cursor, afterSeq, textSnapshot.Seq, events)
	return replay, events, cancel, true
}

func (r *generationStreamRegistry) readStoreEvents(
	ctx context.Context,
	store repository.GenerationStreamCacheRepository,
	runID string,
	cursor string,
	afterSeq int64,
	textSnapshotSeq int64,
	out chan<- GenerationStreamEvent,
) {
	defer close(out)
	if strings.TrimSpace(cursor) == "" {
		cursor = "0-0"
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		records, err := store.ReadGenerationStreamEvents(ctx, runID, cursor, generationStreamReadBlock, int64(r.options.SubscriberBuffer))
		if err != nil {
			return
		}
		for _, record := range records {
			if strings.TrimSpace(record.ID) != "" {
				cursor = record.ID
			}
			if record.Seq <= afterSeq {
				continue
			}
			event, ok := decodeStreamRecord(record)
			if !ok {
				continue
			}
			if streamString(event.Payload["type"]) == "delta" && event.Seq <= textSnapshotSeq {
				continue
			}
			afterSeq = event.Seq
			select {
			case <-ctx.Done():
				return
			case out <- event:
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
	r.mu.Lock()
	active, ok := r.active[runID]
	if ok {
		delete(r.active, runID)
	}
	r.mu.Unlock()
	userID := uint(0)
	if active != nil {
		userID = active.userID
	}
	if userID == 0 && r.store != nil {
		if ownerID, found, err := r.store.GetGenerationStreamOwner(ctx, runID); err == nil && found {
			userID = ownerID
		}
	}
	r.clearActive(ctx, runID, userID)
	conversationID := ""
	if active != nil {
		conversationID = active.conversationID
	}
	r.publishActiveEvent(context.Background(), userID, "finished", runID, conversationID)
	if r.store != nil {
		_ = r.store.ExpireGenerationStream(ctx, runID, r.options.Retention)
	}
	if ok {
		stopActiveGeneration(active)
	}
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

func (r *generationStreamRegistry) scheduleActiveExpiryLocked(runID string, active *activeGeneration) {
	if active.maxRuntimeTimer != nil {
		active.maxRuntimeTimer.Stop()
	}
	activeTTL := r.options.ActiveTTL
	if activeTTL <= 0 {
		activeTTL = generationStreamActiveTTL
	}
	active.maxRuntimeTimer = time.AfterFunc(activeTTL, func() {
		var cancel context.CancelFunc
		var leaseCancel context.CancelFunc
		expired := false
		r.mu.Lock()
		current, ok := r.active[runID]
		if ok && current == active {
			delete(r.active, runID)
			cancel = current.cancel
			leaseCancel = current.leaseCancel
			expired = true
		}
		r.mu.Unlock()
		if leaseCancel != nil {
			leaseCancel()
		}
		if expired {
			r.clearActive(context.Background(), runID, active.userID)
			r.publishActiveEvent(context.Background(), active.userID, "finished", runID, active.conversationID)
		}
		if cancel != nil {
			cancel()
		}
	})
}

func (r *generationStreamRegistry) deleteActive(userID uint, runID string) (*activeGeneration, bool) {
	if runID == "" {
		return nil, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	active, ok := r.active[runID]
	if !ok || active.userID != userID {
		return nil, false
	}
	delete(r.active, runID)
	return active, true
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

func (r *generationStreamRegistry) startActiveLease(runID string, userID uint) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	r.touchActiveForOwner(ctx, runID, userID)
	go func() {
		ticker := time.NewTicker(r.options.LeaseRefresh)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.touchActiveForOwner(ctx, runID, userID)
			}
		}
	}()
	return cancel
}

func (r *generationStreamRegistry) touchActive(ctx context.Context, runID string) {
	if runID == "" {
		return
	}
	r.mu.Lock()
	active := r.active[runID]
	r.mu.Unlock()
	if active != nil {
		r.touchActiveForOwner(ctx, runID, active.userID)
	}
}

func (r *generationStreamRegistry) touchActiveForOwner(ctx context.Context, runID string, userID uint) {
	if runID == "" || userID == 0 || r.store == nil {
		return
	}
	_ = r.store.TouchGenerationStreamActive(ctx, runID, userID, r.options.LeaseTTL)
}

func (r *generationStreamRegistry) clearActive(ctx context.Context, runID string, userID uint) {
	if runID == "" {
		return
	}
	if r.store != nil {
		_ = r.store.ClearGenerationStreamActive(ctx, runID, userID)
	}
}

func stopActiveGeneration(active *activeGeneration) {
	if active == nil {
		return
	}
	if active.maxRuntimeTimer != nil {
		active.maxRuntimeTimer.Stop()
		active.maxRuntimeTimer = nil
	}
	if active.leaseCancel != nil {
		active.leaseCancel()
		active.leaseCancel = nil
	}
}

func retainedStreamEvents(
	records []repository.GenerationStreamMessage,
	afterSeq int64,
	textSnapshot repository.GenerationStreamTextSnapshot,
	hasTextSnapshot bool,
	includeTextSnapshot bool,
) ([]GenerationStreamEvent, string, bool, bool) {
	replay := make([]GenerationStreamEvent, 0)
	cursor := "0-0"
	terminal := false
	snapshotPending := includeTextSnapshot && hasTextSnapshot
	appendTextSnapshot := func() {
		if !snapshotPending {
			return
		}
		replay = append(replay, GenerationStreamEvent{
			Seq: textSnapshot.Seq,
			Payload: map[string]interface{}{
				"type":    "delta",
				"seq":     textSnapshot.Seq,
				"delta":   textSnapshot.Content,
				"replace": true,
			},
		})
		snapshotPending = false
	}
	for _, record := range records {
		if strings.TrimSpace(record.ID) != "" {
			cursor = record.ID
		}
		event, ok := decodeStreamRecord(record)
		if !ok {
			continue
		}
		if snapshotPending && event.Seq > textSnapshot.Seq {
			appendTextSnapshot()
		}
		if isTerminalStreamPayload(event.Payload) {
			terminal = true
		}
		if streamString(event.Payload["type"]) == "delta" {
			// A text delta without a cumulative checkpoint cannot be replayed
			// safely once the bounded event window has trimmed older chunks.
			if includeTextSnapshot && !hasTextSnapshot {
				return nil, cursor, terminal, false
			}
			if includeTextSnapshot && event.Seq <= textSnapshot.Seq {
				continue
			}
		}
		if event.Seq > afterSeq {
			replay = append(replay, event)
		}
	}
	appendTextSnapshot()
	return replay, cursor, terminal, true
}

func decodeStreamRecord(record repository.GenerationStreamMessage) (GenerationStreamEvent, bool) {
	payload := map[string]interface{}{}
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

func marshalStreamPayload(payload map[string]interface{}) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func generationStreamPayloadForStore(payload map[string]interface{}) (map[string]interface{}, bool) {
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

func shouldReturnSanitizedGenerationStreamPayload(actual map[string]interface{}, sanitized bool) bool {
	return sanitized && isTraceUpdateStreamPayload(actual)
}

func isTraceUpdateStreamPayload(payload map[string]interface{}) bool {
	switch strings.TrimSpace(streamString(payload["type"])) {
	case "process_update", "upstream_think_delta":
		return true
	default:
		return false
	}
}

func sanitizeGenerationStreamPayload(payload map[string]interface{}) map[string]interface{} {
	raw, err := json.Marshal(payload)
	if err != nil {
		next := cloneStreamPayload(payload)
		next["payloadTruncated"] = true
		return next
	}
	var normalized interface{}
	if err := json.Unmarshal(raw, &normalized); err != nil {
		next := cloneStreamPayload(payload)
		next["payloadTruncated"] = true
		return next
	}
	sanitized, _ := sanitizeGenerationStreamValue(normalized).(map[string]interface{})
	if sanitized == nil {
		sanitized = map[string]interface{}{}
	}
	sanitized["payloadTruncated"] = true
	return sanitized
}

func sanitizeGenerationStreamValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		next := make(map[string]interface{}, len(typed))
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
	case []interface{}:
		next := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			next = append(next, sanitizeGenerationStreamValue(item))
		}
		return next
	default:
		return value
	}
}

func sanitizeStreamToolCalls(value interface{}) interface{} {
	items, ok := value.([]interface{})
	if !ok {
		return sanitizeGenerationStreamValue(value)
	}
	next := make([]interface{}, 0, len(items))
	for _, item := range items {
		record, ok := item.(map[string]interface{})
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
	var payload interface{}
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

func compactOversizedGenerationStreamPayload(payload map[string]interface{}) map[string]interface{} {
	eventType := strings.TrimSpace(streamString(payload["type"]))
	next := map[string]interface{}{
		"type":             eventType,
		"payloadTruncated": true,
	}
	for _, key := range []string{"status", "message", "errorCode", "code"} {
		if value := strings.TrimSpace(streamString(payload[key])); value != "" {
			next[key] = compactSnippet(value, 512)
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

func streamString(value interface{}) string {
	text, _ := value.(string)
	return text
}

func cloneStreamPayload(payload map[string]interface{}) map[string]interface{} {
	next := make(map[string]interface{}, len(payload)+1)
	for key, value := range payload {
		next[key] = value
	}
	return next
}

func isTerminalStreamPayload(payload map[string]interface{}) bool {
	eventType, _ := payload["type"].(string)
	return eventType == "completed" || eventType == "error" || eventType == "moderation_blocked"
}

func int64FromPayload(raw interface{}) int64 {
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
