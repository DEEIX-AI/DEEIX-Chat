package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	cachememory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestGenerationStreamRegistryReplayAndTerminal(t *testing.T) {
	registry := newGenerationStreamRegistry(newTestGenerationStreamStore(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	registry.register(ctx, runID, 7, "conv_test", func() {})
	defer registry.finish(ctx, runID)

	first := registry.publish(ctx, runID, map[string]interface{}{"type": "delta", "delta": "a"})
	second := registry.publish(ctx, runID, map[string]interface{}{"type": "completed"})
	if first["seq"] != int64(1) || second["seq"] != int64(2) {
		t.Fatalf("unexpected seq values: first=%v second=%v", first["seq"], second["seq"])
	}

	replay, events, unsubscribe, ok := registry.subscribe(ctx, 7, runID, 1, true)
	if !ok {
		t.Fatal("expected subscription to existing run")
	}
	defer unsubscribe()
	if len(replay) != 2 || replay[0].Seq != 1 || replay[0].Payload["type"] != "delta" || replay[0].Payload["delta"] != "a" || replay[0].Payload["replace"] != true || replay[1].Seq != 2 {
		t.Fatalf("unexpected replay events: %+v", replay)
	}
	if _, ok := <-events; ok {
		t.Fatal("terminal replay should close live event channel")
	}

	replay, events, unsubscribe, ok = registry.subscribe(ctx, 7, runID, 2, true)
	if !ok {
		t.Fatal("expected subscription after terminal seq to existing run")
	}
	defer unsubscribe()
	if len(replay) != 1 || replay[0].Payload["type"] != "delta" || replay[0].Payload["delta"] != "a" || replay[0].Payload["replace"] != true {
		t.Fatalf("unexpected replay after terminal seq: %+v", replay)
	}
	if _, ok := <-events; ok {
		t.Fatal("terminal state should close live event channel after last seq")
	}
}

func TestGenerationStreamRegistryReplayUsesFullTextSnapshotBeyondWindow(t *testing.T) {
	registry := newGenerationStreamRegistry(newTestGenerationStreamStore(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        3,
		SubscriberBuffer: 4,
	})
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	registry.register(ctx, runID, 7, "conv_test", func() {})
	defer registry.finish(ctx, runID)

	for _, delta := range []string{"a", "b", "c", "d", "e", "f"} {
		registry.publish(ctx, runID, map[string]interface{}{"type": "delta", "delta": delta})
	}

	replay, _, unsubscribe, ok := registry.subscribe(ctx, 7, runID, 0, true)
	if !ok {
		t.Fatal("expected long stream to remain resumable")
	}
	defer unsubscribe()
	if len(replay) != 1 || replay[0].Payload["type"] != "delta" || replay[0].Payload["delta"] != "abcdef" || replay[0].Payload["replace"] != true || replay[0].Seq != 6 {
		t.Fatalf("expected one complete text snapshot, got %+v", replay)
	}
}

func TestGenerationStreamRegistryLegacyReplayKeepsOriginalDeltaProtocol(t *testing.T) {
	registry := newGenerationStreamRegistry(newTestGenerationStreamStore(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	registry.register(ctx, runID, 7, "conv_test", func() {})
	defer registry.finish(ctx, runID)
	registry.publish(ctx, runID, map[string]interface{}{"type": "delta", "delta": "legacy"})

	replay, _, unsubscribe, ok := registry.subscribe(ctx, 7, runID, 0, false)
	if !ok {
		t.Fatal("expected legacy subscription")
	}
	defer unsubscribe()
	if len(replay) != 1 || replay[0].Payload["delta"] != "legacy" {
		t.Fatalf("unexpected legacy replay: %+v", replay)
	}
	if _, exists := replay[0].Payload["replace"]; exists {
		t.Fatalf("legacy replay unexpectedly received snapshot semantics: %+v", replay)
	}
}

func TestGenerationStreamRegistrySnapshotThenLiveDeltaExactlyOnce(t *testing.T) {
	registry := newGenerationStreamRegistry(newTestGenerationStreamStore(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        3,
		SubscriberBuffer: 4,
	})
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	registry.register(ctx, runID, 7, "conv_test", func() {})
	defer registry.finish(ctx, runID)
	registry.publish(ctx, runID, map[string]interface{}{"type": "delta", "delta": "a"})
	registry.publish(ctx, runID, map[string]interface{}{"type": "delta", "delta": "b"})

	replay, events, unsubscribe, ok := registry.subscribe(ctx, 7, runID, 0, true)
	if !ok {
		t.Fatal("expected subscription")
	}
	defer unsubscribe()
	if len(replay) != 1 || replay[0].Payload["delta"] != "ab" || replay[0].Payload["replace"] != true {
		t.Fatalf("unexpected snapshot replay: %+v", replay)
	}

	registry.publish(ctx, runID, map[string]interface{}{"type": "delta", "delta": "c"})
	select {
	case event := <-events:
		if event.Payload["type"] != "delta" || event.Payload["delta"] != "c" || event.Seq != 3 {
			t.Fatalf("unexpected live event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live delta")
	}
}

func TestGenerationStreamRegistryKeepsNonTextReplayInSequenceOrder(t *testing.T) {
	registry := newGenerationStreamRegistry(newTestGenerationStreamStore(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	registry.register(ctx, runID, 7, "conv_test", func() {})
	defer registry.finish(ctx, runID)
	registry.publish(ctx, runID, map[string]interface{}{"type": "file_proc", "message": "preparing"})
	registry.publish(ctx, runID, map[string]interface{}{"type": "delta", "delta": "answer"})
	registry.publish(ctx, runID, map[string]interface{}{"type": "usage", "output_tokens": 1})

	replay, _, unsubscribe, ok := registry.subscribe(ctx, 7, runID, 0, true)
	if !ok {
		t.Fatal("expected subscription")
	}
	defer unsubscribe()
	if len(replay) != 3 ||
		replay[0].Seq != 1 || replay[0].Payload["type"] != "file_proc" ||
		replay[1].Seq != 2 || replay[1].Payload["replace"] != true ||
		replay[2].Seq != 3 || replay[2].Payload["type"] != "usage" {
		t.Fatalf("unexpected ordered replay: %+v", replay)
	}
}

func TestGenerationStreamRegistryRejectsTextReplayWithoutSnapshot(t *testing.T) {
	store := newTestGenerationStreamStore()
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        3,
		SubscriberBuffer: 4,
	})
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	registry.register(ctx, runID, 7, "conv_test", func() {})
	defer registry.finish(ctx, runID)
	if _, err := store.AppendGenerationStreamEvent(ctx, runID, repository.GenerationStreamAppend{
		PayloadJSON: `{"type":"delta","delta":"unsafe"}`,
	}, 3, time.Minute); err != nil {
		t.Fatal(err)
	}

	if _, _, _, ok := registry.subscribe(ctx, 7, runID, 0, true); ok {
		t.Fatal("expected replay without an authoritative text snapshot to be rejected")
	}
}

func TestGenerationStreamRegistryResetClearsTextSnapshot(t *testing.T) {
	store := newTestGenerationStreamStore()
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        3,
		SubscriberBuffer: 4,
	})
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	registry.register(ctx, runID, 7, "conv_test", func() {})
	defer registry.finish(ctx, runID)
	registry.publish(ctx, runID, map[string]interface{}{"type": "delta", "delta": "blocked text"})
	registry.resetEvents(ctx, runID)
	registry.publish(ctx, runID, map[string]interface{}{"type": "moderation_blocked"})

	replay, events, unsubscribe, ok := registry.subscribe(ctx, 7, runID, 0, true)
	if !ok {
		t.Fatal("expected blocked stream to remain subscribable")
	}
	defer unsubscribe()
	if len(replay) != 1 || replay[0].Payload["type"] != "moderation_blocked" {
		t.Fatalf("blocked content was retained after reset: %+v", replay)
	}
	if _, ok := <-events; ok {
		t.Fatal("expected moderation block to terminate replay")
	}
}

func TestGenerationStreamRegistryCancelUsesSharedMarker(t *testing.T) {
	registry := newGenerationStreamRegistry(newTestGenerationStreamStore(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	canceled := false
	registry.register(ctx, runID, 9, "conv_test", func() { canceled = true })
	defer registry.finish(ctx, runID)

	if !registry.cancel(ctx, 9, runID) {
		t.Fatal("expected cancel to be accepted for run owner")
	}
	if !canceled {
		t.Fatal("expected local cancel function to be called")
	}
	if !registry.isCanceled(ctx, runID) {
		t.Fatal("expected shared cancel marker to be set")
	}
	if registry.hasActive(ctx, runID) {
		t.Fatal("expected active lease to be cleared after cancel")
	}
	registry.mu.Lock()
	_, stillTracked := registry.active[runID]
	registry.mu.Unlock()
	if stillTracked {
		t.Fatal("expected local active generation to be removed after cancel")
	}
	if registry.cancel(ctx, 8, runID) {
		t.Fatal("expected cancel to reject non-owner")
	}
}

func TestGenerationStreamRegistryActiveLeaseLifecycle(t *testing.T) {
	registry := newGenerationStreamRegistry(newTestGenerationStreamStore(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		LeaseTTL:         time.Second,
		LeaseRefresh:     100 * time.Millisecond,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	registry.register(ctx, runID, 7, "conv_test", func() {})

	if !registry.hasActive(ctx, runID) {
		t.Fatal("expected active lease after register")
	}
	items, err := registry.listActive(ctx, 7)
	if err != nil || len(items) != 1 || items[0].RunID != runID || items[0].ConversationPublicID != "conv_test" {
		t.Fatalf("active generation snapshot=%+v err=%v, want registered run", items, err)
	}
	if otherItems, otherErr := registry.listActive(ctx, 8); otherErr != nil || len(otherItems) != 0 {
		t.Fatalf("other user snapshot=%+v err=%v, want empty", otherItems, otherErr)
	}

	registry.finish(ctx, runID)
	if registry.hasActive(ctx, runID) {
		t.Fatal("expected active lease to be cleared after finish")
	}
	registry.mu.Lock()
	_, stillTracked := registry.active[runID]
	registry.mu.Unlock()
	if stillTracked {
		t.Fatal("expected local active generation to be removed after finish")
	}
	if items, err = registry.listActive(ctx, 7); err != nil || len(items) != 0 {
		t.Fatalf("active generation snapshot after finish=%+v err=%v, want empty", items, err)
	}
}

func TestGenerationStreamRegistryActiveSubscriptionStartsWithSnapshotAndStreamsEvents(t *testing.T) {
	store := cachememory.New()
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		LeaseTTL:         time.Second,
		LeaseRefresh:     100 * time.Millisecond,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	ctx := context.Background()
	registry.register(ctx, "run_snapshot", 7, "conv_snapshot", func() {})
	defer registry.finish(ctx, "run_snapshot")

	snapshot, events, cancel, err := registry.subscribeActive(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if len(snapshot) != 1 || snapshot[0].RunID != "run_snapshot" || snapshot[0].ConversationPublicID != "conv_snapshot" {
		t.Fatalf("snapshot = %+v, want the active run", snapshot)
	}
	otherSnapshot, otherEvents, cancelOther, err := registry.subscribeActive(ctx, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelOther()
	if len(otherSnapshot) != 0 {
		t.Fatalf("other user snapshot = %+v, want empty", otherSnapshot)
	}

	registry.register(ctx, "run_live", 7, "conv_live", func() {})
	select {
	case event := <-events:
		if event.Type != "started" || event.RunID != "run_live" || event.ConversationPublicID != "conv_live" {
			t.Fatalf("started event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for started event")
	}
	select {
	case event := <-otherEvents:
		t.Fatalf("other user received event = %+v", event)
	case <-time.After(25 * time.Millisecond):
	}

	registry.finish(ctx, "run_live")
	select {
	case event := <-events:
		if event.Type != "finished" || event.RunID != "run_live" {
			t.Fatalf("finished event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for finished event")
	}
}

func TestServiceListActiveMessageGenerationsDelegatesToRegistry(t *testing.T) {
	registry := newGenerationStreamRegistry(newTestGenerationStreamStore(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		LeaseTTL:         time.Second,
		LeaseRefresh:     100 * time.Millisecond,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	ctx := context.Background()
	registry.register(ctx, "run_reconcile", 7, "conv_reconcile", func() {})
	defer registry.finish(ctx, "run_reconcile")

	svc := &Service{generationStreams: registry}
	items, err := svc.ListActiveMessageGenerations(ctx, 7)
	if err != nil || len(items) != 1 || items[0].RunID != "run_reconcile" || items[0].ConversationPublicID != "conv_reconcile" {
		t.Fatalf("active snapshot = %+v err = %v, want registered run", items, err)
	}
	if items, err = svc.ListActiveMessageGenerations(ctx, 0); err != nil || len(items) != 0 {
		t.Fatalf("zero user snapshot = %+v err = %v, want empty", items, err)
	}

	var nilService *Service
	if items, err = nilService.ListActiveMessageGenerations(ctx, 7); err != nil || len(items) != 0 {
		t.Fatalf("nil service snapshot = %+v err = %v, want empty", items, err)
	}
}

func TestGenerationStreamStoreActiveLeaseExpires(t *testing.T) {
	store := newTestGenerationStreamStore()
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	if err := store.RegisterGenerationStream(ctx, runID, 7, "conv_test", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.TouchGenerationStreamActive(ctx, runID, 7, 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if active, err := store.IsGenerationStreamActive(ctx, runID); err != nil || !active {
		t.Fatalf("expected active lease, active=%v err=%v", active, err)
	}
	time.Sleep(20 * time.Millisecond)
	if active, err := store.IsGenerationStreamActive(ctx, runID); err != nil || active {
		t.Fatalf("expected expired active lease, active=%v err=%v", active, err)
	}
}

func TestGenerationStreamStoreReturnsLatestWindow(t *testing.T) {
	store := newTestGenerationStreamStore()
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	if err := store.RegisterGenerationStream(ctx, runID, 11, "conv_test", time.Minute); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := store.AppendGenerationStreamEvent(ctx, runID, repository.GenerationStreamAppend{PayloadJSON: `{"type":"delta"}`}, 3, time.Minute); err != nil {
			t.Fatal(err)
		}
	}

	events, err := store.ListGenerationStreamEvents(ctx, runID, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("expected latest 3 events, got %d", len(events))
	}
	if events[0].Seq != 3 || events[1].Seq != 4 || events[2].Seq != 5 {
		t.Fatalf("unexpected event window: %+v", events)
	}
}

func TestGenerationStreamSanitizesOversizedTracePayload(t *testing.T) {
	store := newTestGenerationStreamStore()
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	registry.register(ctx, runID, 7, "conv_test", func() {})
	defer registry.finish(ctx, runID)

	largeOutput := strings.Repeat("x", generationStreamMaxPayloadBytes)
	tracePayload, err := json.Marshal(map[string]interface{}{
		"tool_calls": []map[string]interface{}{{
			"tool_call_id":   "call_1",
			"name":           "fetch",
			"status":         "success",
			"output":         largeOutput,
			"output_detail":  largeOutput,
			"output_text":    largeOutput,
			"output_preview": "short result",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	published := registry.publish(ctx, runID, map[string]interface{}{
		"type":   "process_update",
		"status": "streaming",
		"trace": map[string]interface{}{
			"tools": map[string]interface{}{
				"payloadJSON": string(tracePayload),
			},
		},
	})
	if published["payloadTruncated"] != true {
		t.Fatalf("expected published payload to be marked truncated, got %#v", published)
	}

	records, err := store.ListGenerationStreamEvents(ctx, runID, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one stream record, got %d", len(records))
	}
	record := records[0].PayloadJSON
	if len(record) > generationStreamMaxPayloadBytes {
		t.Fatalf("expected sanitized stream payload below hard limit, got %d bytes", len(record))
	}
	if strings.Contains(record, largeOutput[:1024]) {
		t.Fatal("sanitized stream payload still contains full tool output")
	}
	var parsed struct {
		Trace struct {
			Tools struct {
				PayloadJSON string `json:"payloadJSON"`
			} `json:"tools"`
		} `json:"trace"`
	}
	if err := json.Unmarshal([]byte(record), &parsed); err != nil {
		t.Fatal(err)
	}
	var parsedTrace struct {
		ToolCalls []map[string]interface{} `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(parsed.Trace.Tools.PayloadJSON), &parsedTrace); err != nil {
		t.Fatal(err)
	}
	if len(parsedTrace.ToolCalls) != 1 {
		t.Fatalf("expected one sanitized tool call, got %#v", parsedTrace.ToolCalls)
	}
	call := parsedTrace.ToolCalls[0]
	if traceInt64(call["output_size"]) != int64(len(largeOutput)) ||
		traceInt64(call["output_detail_size"]) != int64(len(largeOutput)) ||
		traceInt64(call["output_text_size"]) != int64(len(largeOutput)) {
		t.Fatalf("expected output size metadata in sanitized payload, got %#v", call)
	}
	if _, ok := call["output_detail"]; ok {
		t.Fatalf("expected oversized output detail to be removed, got %#v", call)
	}
}

func TestGenerationStreamDoesNotCompactOversizedCompletedPayload(t *testing.T) {
	store := newTestGenerationStreamStore()
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	registry.register(ctx, runID, 7, "conv_test", func() {})
	defer registry.finish(ctx, runID)

	largeContent := strings.Repeat("a", generationStreamMaxPayloadBytes)
	published := registry.publish(ctx, runID, map[string]interface{}{
		"type": "completed",
		"data": map[string]interface{}{
			"assistantMessage": map[string]interface{}{
				"content": largeContent,
			},
		},
	})

	if published["payloadTruncated"] == true {
		t.Fatalf("completed payload must not be compacted for active clients, got %#v", published)
	}
	data, ok := published["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected completed data to be preserved, got %#v", published)
	}
	assistant, ok := data["assistantMessage"].(map[string]interface{})
	if !ok || assistant["content"] != largeContent {
		t.Fatal("expected completed assistant content to be preserved")
	}
	records, err := store.ListGenerationStreamEvents(ctx, runID, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || !strings.Contains(records[0].PayloadJSON, largeContent[:1024]) {
		t.Fatalf("expected stored terminal event to preserve completed data, got %+v", records)
	}
}

type testGenerationStreamStore struct {
	mu    sync.Mutex
	items map[string]*testGenerationStream
}

type testGenerationStream struct {
	userID         uint
	conversationID string
	canceled       bool
	activeUntil    time.Time
	nextSeq        int64
	events         []repository.GenerationStreamMessage
	textContent    strings.Builder
	textSeq        int64
	expiresAt      time.Time
}

func newTestGenerationStreamStore() *testGenerationStreamStore {
	return &testGenerationStreamStore{items: map[string]*testGenerationStream{}}
}

func (s *testGenerationStreamStore) RegisterGenerationStream(_ context.Context, runID string, userID uint, conversationPublicID string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.ensureLocked(runID)
	item.userID = userID
	item.conversationID = conversationPublicID
	item.canceled = false
	item.expiresAt = time.Now().Add(ttl)
	return nil
}

func (s *testGenerationStreamStore) GetGenerationStreamOwner(_ context.Context, runID string) (uint, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	item, ok := s.items[runID]
	if !ok || item.userID == 0 {
		return 0, false, nil
	}
	return item.userID, true, nil
}

func (s *testGenerationStreamStore) TouchGenerationStreamActive(_ context.Context, runID string, userID uint, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.ensureLocked(runID)
	if item.userID == userID {
		item.activeUntil = time.Now().Add(ttl)
	}
	return nil
}

func (s *testGenerationStreamStore) ClearGenerationStreamActive(_ context.Context, runID string, userID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item, ok := s.items[runID]; ok && item.userID == userID {
		item.activeUntil = time.Time{}
	}
	return nil
}

func (s *testGenerationStreamStore) IsGenerationStreamActive(_ context.Context, runID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	item, ok := s.items[runID]
	return ok && !item.activeUntil.IsZero() && time.Now().Before(item.activeUntil), nil
}

func (s *testGenerationStreamStore) ListActiveGenerationStreams(_ context.Context, userID uint) ([]repository.ActiveGenerationStream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	items := make([]repository.ActiveGenerationStream, 0)
	for runID, item := range s.items {
		if item.userID != userID || item.activeUntil.IsZero() || !time.Now().Before(item.activeUntil) || item.conversationID == "" {
			continue
		}
		items = append(items, repository.ActiveGenerationStream{RunID: runID, ConversationPublicID: item.conversationID})
	}
	return items, nil
}

func (s *testGenerationStreamStore) RequestGenerationStreamCancel(_ context.Context, runID string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.ensureLocked(runID)
	item.canceled = true
	item.expiresAt = time.Now().Add(ttl)
	return nil
}

func (s *testGenerationStreamStore) IsGenerationStreamCanceled(_ context.Context, runID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	item, ok := s.items[runID]
	return ok && item.canceled, nil
}

func (s *testGenerationStreamStore) AppendGenerationStreamEvent(_ context.Context, runID string, input repository.GenerationStreamAppend, maxEvents int64, ttl time.Duration) (repository.GenerationStreamMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.ensureLocked(runID)
	item.nextSeq++
	record := repository.GenerationStreamMessage{
		ID:          fmt.Sprintf("%d-0", item.nextSeq),
		Seq:         item.nextSeq,
		PayloadJSON: input.PayloadJSON,
	}
	item.events = append(item.events, record)
	if input.TextDelta != "" {
		_, _ = item.textContent.WriteString(input.TextDelta)
		item.textSeq = item.nextSeq
	}
	if maxEvents > 0 && int64(len(item.events)) > maxEvents {
		item.events = append([]repository.GenerationStreamMessage(nil), item.events[len(item.events)-int(maxEvents):]...)
	}
	item.expiresAt = time.Now().Add(ttl)
	return record, nil
}

func (s *testGenerationStreamStore) GetGenerationStreamTextSnapshot(_ context.Context, runID string) (repository.GenerationStreamTextSnapshot, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	item, ok := s.items[runID]
	if !ok || item.textSeq <= 0 {
		return repository.GenerationStreamTextSnapshot{}, false, nil
	}
	return repository.GenerationStreamTextSnapshot{
		Seq:     item.textSeq,
		Content: item.textContent.String(),
	}, true, nil
}

func (s *testGenerationStreamStore) ListGenerationStreamEvents(_ context.Context, runID string, limit int64) ([]repository.GenerationStreamMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	item, ok := s.items[runID]
	if !ok {
		return nil, nil
	}
	events := item.events
	if limit > 0 && int64(len(events)) > limit {
		events = events[len(events)-int(limit):]
	}
	return append([]repository.GenerationStreamMessage(nil), events...), nil
}

func (s *testGenerationStreamStore) ReadGenerationStreamEvents(ctx context.Context, runID string, afterID string, _ time.Duration, limit int64) ([]repository.GenerationStreamMessage, error) {
	afterSeq := testStreamIDSeq(afterID)
	events, err := s.ListGenerationStreamEvents(ctx, runID, limit)
	if err != nil {
		return nil, err
	}
	results := make([]repository.GenerationStreamMessage, 0)
	for _, event := range events {
		if event.Seq > afterSeq {
			results = append(results, event)
		}
	}
	return results, nil
}

func (s *testGenerationStreamStore) ResetGenerationStreamEvents(_ context.Context, runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item, ok := s.items[runID]; ok {
		item.events = nil
		item.textContent.Reset()
		item.textSeq = 0
	}
	return nil
}

func (s *testGenerationStreamStore) ExpireGenerationStream(_ context.Context, runID string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLocked(runID).expiresAt = time.Now().Add(ttl)
	return nil
}

func (s *testGenerationStreamStore) ensureLocked(runID string) *testGenerationStream {
	item, ok := s.items[runID]
	if ok {
		return item
	}
	item = &testGenerationStream{}
	s.items[runID] = item
	return item
}

func (s *testGenerationStreamStore) cleanupLocked() {
	now := time.Now()
	for runID, item := range s.items {
		if item.expiresAt.IsZero() || now.Before(item.expiresAt) {
			continue
		}
		delete(s.items, runID)
	}
}

func testStreamIDSeq(raw string) int64 {
	head, _, found := strings.Cut(strings.TrimSpace(raw), "-")
	if !found {
		return 0
	}
	value, err := strconv.ParseInt(head, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

var _ repository.GenerationStreamCacheRepository = (*testGenerationStreamStore)(nil)
