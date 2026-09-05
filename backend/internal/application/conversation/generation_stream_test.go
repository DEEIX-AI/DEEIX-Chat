package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	cachememory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func registerTestGeneration(
	t *testing.T,
	registry *generationStreamRegistry,
	runID string,
	userID uint,
	conversationPublicID string,
	cancel context.CancelFunc,
) (context.Context, func()) {
	t.Helper()
	ctx, release, ok := registry.acquireRun(context.Background())
	if !ok {
		t.Fatal("expected generation lifecycle acquisition")
	}
	if err := registry.register(ctx, runID, userID, conversationPublicID, cancel); err != nil {
		release()
		t.Fatalf("register generation: %v", err)
	}
	return ctx, func() {
		registry.finish(ctx, runID)
		release()
	}
}

func publishTestGeneration(
	t *testing.T,
	registry *generationStreamRegistry,
	ctx context.Context,
	runID string,
	payload map[string]any,
) map[string]any {
	t.Helper()
	published, accepted := registry.publish(ctx, runID, payload)
	if !accepted {
		t.Fatal("expected generation event to be accepted")
	}
	return published
}

func claimTestGeneration(
	t *testing.T,
	store repository.GenerationStreamCacheRepository,
	runID string,
	executionID string,
	userID uint,
	conversationPublicID string,
	ttl time.Duration,
) repository.GenerationStreamLease {
	t.Helper()
	lease := repository.GenerationStreamLease{
		RunID:                runID,
		ExecutionID:          executionID,
		UserID:               userID,
		ConversationPublicID: conversationPublicID,
	}
	claimed, err := store.ClaimGenerationStream(context.Background(), lease, ttl, ttl)
	if err != nil || !claimed {
		t.Fatalf("claim generation: claimed=%v err=%v", claimed, err)
	}
	return lease
}

func TestGenerationStreamRegistryReplayAndTerminal(t *testing.T) {
	registry := newGenerationStreamRegistry(cachememory.New(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()

	first := publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "delta", "delta": "a"})
	second := publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "completed"})
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
	registry := newGenerationStreamRegistry(cachememory.New(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        3,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()

	for _, delta := range []string{"a", "b", "c", "d", "e", "f"} {
		publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "delta", "delta": delta})
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
	registry := newGenerationStreamRegistry(cachememory.New(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()
	publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "delta", "delta": "legacy"})

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
	registry := newGenerationStreamRegistry(cachememory.New(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        3,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()
	publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "delta", "delta": "a"})
	publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "delta", "delta": "b"})

	replay, events, unsubscribe, ok := registry.subscribe(ctx, 7, runID, 0, true)
	if !ok {
		t.Fatal("expected subscription")
	}
	defer unsubscribe()
	if len(replay) != 1 || replay[0].Payload["delta"] != "ab" || replay[0].Payload["replace"] != true {
		t.Fatalf("unexpected snapshot replay: %+v", replay)
	}

	publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "delta", "delta": "c"})
	select {
	case event := <-events:
		if event.Payload["type"] != "delta" || event.Payload["delta"] != "c" || event.Seq != 3 {
			t.Fatalf("unexpected live event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live delta")
	}
}

func TestGenerationStreamRegistrySubscriptionStopsWhenOwnerChanges(t *testing.T) {
	store := &generationStreamSubscriptionOwnershipStore{
		GenerationStreamCacheRepository: cachememory.New(),
		ownerID:                         7,
		readStarted:                     make(chan struct{}),
		releaseRead:                     make(chan struct{}),
	}
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})

	replay, events, unsubscribe, ok := registry.subscribe(context.Background(), 7, "run_owner_change", 0, true)
	if !ok {
		t.Fatal("expected initial owner to subscribe")
	}
	defer unsubscribe()
	if len(replay) != 0 {
		t.Fatalf("unexpected replay events: %+v", replay)
	}
	select {
	case <-store.readStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stream reader")
	}

	store.setOwner(8)
	close(store.releaseRead)
	select {
	case event, open := <-events:
		if open {
			t.Fatalf("old subscriber received replacement owner's event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("subscription did not stop after ownership changed")
	}
}

func TestGenerationStreamRegistryKeepsNonTextReplayInSequenceOrder(t *testing.T) {
	registry := newGenerationStreamRegistry(cachememory.New(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()
	publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "file_proc", "message": "preparing"})
	publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "delta", "delta": "answer"})
	publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "usage", "output_tokens": 1})

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

func TestGenerationStreamRegistryOrdersTextAndUpstreamThinkingSnapshots(t *testing.T) {
	registry := newGenerationStreamRegistry(cachememory.New(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()
	publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "file_proc", "message": "preparing"})
	publishTestGeneration(t, registry, ctx, runID, map[string]any{
		"type":    "upstream_think_delta",
		"status":  "streaming",
		"roundID": " round_1 ",
		"delta":   "thought",
	})
	publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "delta", "delta": "answer"})
	publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "usage", "output_tokens": 1})

	replay, _, unsubscribe, ok := registry.subscribe(ctx, 7, runID, 0, true)
	if !ok {
		t.Fatal("expected subscription")
	}
	defer unsubscribe()
	if len(replay) != 4 ||
		replay[0].Seq != 1 || replay[0].Payload["type"] != "file_proc" ||
		replay[1].Seq != 2 || replay[1].Payload["type"] != "upstream_think_delta" ||
		replay[1].Payload["contentMarkdown"] != "thought" || replay[1].Payload["roundID"] != "round_1" ||
		replay[2].Seq != 3 || replay[2].Payload["type"] != "delta" || replay[2].Payload["replace"] != true ||
		replay[3].Seq != 4 || replay[3].Payload["type"] != "usage" {
		t.Fatalf("unexpected ordered snapshot replay: %+v", replay)
	}
}

func TestGenerationStreamRegistryUpstreamThinkingSnapshotTracksTerminalMetadata(t *testing.T) {
	registry := newGenerationStreamRegistry(cachememory.New(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()
	publishTestGeneration(t, registry, ctx, runID, map[string]any{
		"type":    "upstream_think_delta",
		"status":  "streaming",
		"roundID": "round_1",
		"delta":   "thought",
	})
	publishTestGeneration(t, registry, ctx, runID, map[string]any{
		"type":    "upstream_think_delta",
		"status":  "completed",
		"roundID": "round_1",
	})

	replay, _, unsubscribe, ok := registry.subscribe(ctx, 7, runID, 0, true)
	if !ok {
		t.Fatal("expected subscription")
	}
	defer unsubscribe()
	if len(replay) != 1 || replay[0].Seq != 2 ||
		replay[0].Payload["type"] != "upstream_think_delta" ||
		replay[0].Payload["contentMarkdown"] != "thought" ||
		replay[0].Payload["status"] != "completed" {
		t.Fatalf("unexpected terminal upstream-thinking snapshot: %+v", replay)
	}
}

func TestGenerationStreamRegistryRestoresCompleteUpstreamThinkingBeyondReplayWindow(t *testing.T) {
	store := cachememory.New()
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        generationStreamMaxEvents,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()

	var expected strings.Builder
	for i := 0; i < generationStreamMaxEvents+100; i++ {
		delta := fmt.Sprintf("thought-%04d\n", i)
		expected.WriteString(delta)
		publishTestGeneration(t, registry, ctx, runID, map[string]any{
			"type":      "upstream_think_delta",
			"status":    "streaming",
			"stage":     "think",
			"roundID":   "round_1",
			"eventID":   "event_1",
			"startedAt": "2026-08-31T00:00:00Z",
			"delta":     delta,
		})
	}

	replay, events, unsubscribe, ok := registry.subscribe(ctx, 7, runID, 0, true)
	if !ok {
		t.Fatal("expected subscription")
	}
	defer unsubscribe()
	if len(replay) != 1 {
		t.Fatalf("expected one upstream-thinking checkpoint, got %d events", len(replay))
	}
	checkpoint := replay[0]
	if checkpoint.Payload["type"] != "upstream_think_delta" ||
		checkpoint.Payload["contentMarkdown"] != expected.String() ||
		checkpoint.Payload["roundID"] != "round_1" ||
		checkpoint.Seq != generationStreamMaxEvents+100 {
		t.Fatalf("unexpected upstream-thinking checkpoint: %+v", checkpoint)
	}

	publishTestGeneration(t, registry, ctx, runID, map[string]any{
		"type":    "upstream_think_delta",
		"status":  "streaming",
		"roundID": "round_1",
		"eventID": "event_1",
		"delta":   "continued",
	})
	select {
	case event := <-events:
		if event.Payload["type"] != "upstream_think_delta" || event.Payload["delta"] != "continued" {
			t.Fatalf("unexpected live upstream-thinking event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live upstream-thinking event")
	}
}

func TestGenerationStreamRegistryRejectsTextReplayWithoutSnapshot(t *testing.T) {
	store := cachememory.New()
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        3,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()
	lease, ok := registry.leaseForContext(ctx, runID)
	if !ok {
		t.Fatal("expected registered generation lease")
	}
	if _, accepted, err := store.AppendGenerationStreamEvent(ctx, lease, repository.GenerationStreamAppend{
		PayloadJSON: `{"type":"delta","delta":"unsafe"}`,
	}, 3, time.Minute); err != nil || !accepted {
		t.Fatalf("append text event: accepted=%v err=%v", accepted, err)
	}

	if _, _, _, ok := registry.subscribe(ctx, 7, runID, 0, true); ok {
		t.Fatal("expected replay without an authoritative text snapshot to be rejected")
	}
}

func TestGenerationStreamRegistryResetClearsTextSnapshot(t *testing.T) {
	store := cachememory.New()
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        3,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()
	publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "delta", "delta": "blocked text"})
	lease, ok := registry.leaseForContext(ctx, runID)
	if !ok {
		t.Fatal("expected registered generation lease")
	}
	if reset, err := store.ResetGenerationStreamEvents(ctx, lease); err != nil || !reset {
		t.Fatalf("reset generation events: reset=%v err=%v", reset, err)
	}
	publishTestGeneration(t, registry, ctx, runID, map[string]any{"type": "moderation_blocked"})

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
	registry := newGenerationStreamRegistry(cachememory.New(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	canceled := false
	ctx, cleanup := registerTestGeneration(t, registry, runID, 9, "conv_test", func() { canceled = true })
	defer cleanup()

	if !registry.cancel(ctx, 9, runID) {
		t.Fatal("expected cancel to be accepted for run owner")
	}
	if !canceled {
		t.Fatal("expected local cancel function to be called")
	}
	if !registry.isCanceled(ctx, runID) {
		t.Fatal("expected shared cancel marker to be set")
	}
	if !registry.hasActive(ctx, runID) {
		t.Fatal("cancel must retain the lease until the owner publishes its terminal event")
	}
	registry.mu.Lock()
	_, stillTracked := registry.active[runID]
	registry.mu.Unlock()
	if !stillTracked {
		t.Fatal("cancel must retain local ownership until finish")
	}
	if registry.cancel(ctx, 8, runID) {
		t.Fatal("expected cancel to reject non-owner")
	}
}

func TestGenerationStreamRegistryCancelWithoutStoreRequiresLocalOwner(t *testing.T) {
	registry := newGenerationStreamRegistry(nil, generationStreamOptions{})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 9, "conv_test", func() {})
	defer cleanup()

	if registry.cancel(ctx, 8, runID) {
		t.Fatal("expected cancel to reject a non-owner without shared storage")
	}
	if !registry.cancel(ctx, 9, runID) {
		t.Fatal("expected cancel to accept the local owner")
	}
}

type flakyGenerationStreamCompletionStore struct {
	repository.GenerationStreamCacheRepository
	mu       sync.Mutex
	failures int
	calls    int
}

func (s *flakyGenerationStreamCompletionStore) CompleteGenerationStream(
	ctx context.Context,
	lease repository.GenerationStreamLease,
	retention time.Duration,
) (bool, error) {
	s.mu.Lock()
	s.calls++
	fail := s.failures > 0
	if fail {
		s.failures--
	}
	s.mu.Unlock()
	if fail {
		return false, errors.New("temporary cache failure")
	}
	return s.GenerationStreamCacheRepository.CompleteGenerationStream(ctx, lease, retention)
}

func TestGenerationStreamRegistryRetriesCompletionAfterTransientStoreFailure(t *testing.T) {
	store := &flakyGenerationStreamCompletionStore{
		GenerationStreamCacheRepository: cachememory.New(),
		failures:                        1,
	}
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		Retention:    time.Minute,
		ActiveTTL:    time.Minute,
		LeaseTTL:     time.Second,
		LeaseRefresh: 100 * time.Millisecond,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 9, "conv_test", func() {})
	registry.finish(ctx, runID)
	cleanup()

	store.mu.Lock()
	calls := store.calls
	store.mu.Unlock()
	if calls != 2 {
		t.Fatalf("completion calls = %d, want 2 after one transient failure", calls)
	}
	if registry.hasActive(context.Background(), runID) {
		t.Fatal("completed generation remained active after retry")
	}
}

func TestGenerationStreamRegistryRetriesCompletionAfterProlongedStoreFailure(t *testing.T) {
	store := &flakyGenerationStreamCompletionStore{
		GenerationStreamCacheRepository: cachememory.New(),
		failures:                        generationStreamCompletionAttempts,
	}
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		Retention:    time.Minute,
		ActiveTTL:    time.Minute,
		LeaseTTL:     time.Second,
		LeaseRefresh: 100 * time.Millisecond,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 9, "conv_test", func() {})
	registry.finish(ctx, runID)
	cleanup()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		store.mu.Lock()
		calls := store.calls
		store.mu.Unlock()
		if calls >= generationStreamCompletionAttempts+1 && !registry.hasActive(context.Background(), runID) {
			registry.close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	registry.close()
	t.Fatal("completion retry did not recover after prolonged store failure")
}

func TestGenerationStreamRegistryActiveLeaseLifecycle(t *testing.T) {
	registry := newGenerationStreamRegistry(cachememory.New(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		LeaseTTL:         time.Second,
		LeaseRefresh:     100 * time.Millisecond,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()

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
	defer registry.close()
	ctx, cleanupSnapshot := registerTestGeneration(t, registry, "run_snapshot", 7, "conv_snapshot", func() {})
	defer cleanupSnapshot()

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

	_, cleanupLive := registerTestGeneration(t, registry, "run_live", 7, "conv_live", func() {})
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

	cleanupLive()
	select {
	case event := <-events:
		if event.Type != "finished" || event.RunID != "run_live" {
			t.Fatalf("finished event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for finished event")
	}
}

func TestGenerationStreamRegistryCloseCancelsActiveEventReaderInitialization(t *testing.T) {
	initializationStarted := make(chan struct{})
	initializationCanceled := make(chan struct{})
	store := &activeEventReaderTestStore{
		list: func(ctx context.Context) ([]repository.GenerationStreamMessage, error) {
			close(initializationStarted)
			<-ctx.Done()
			close(initializationCanceled)
			return nil, ctx.Err()
		},
	}
	registry := newGenerationStreamRegistry(store, generationStreamOptions{})
	subscribeDone := make(chan error, 1)
	go func() {
		_, _, _, err := registry.subscribeActive(context.Background(), 7)
		subscribeDone <- err
	}()

	select {
	case <-initializationStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for active event reader initialization")
	}
	closeDone := make(chan struct{})
	go func() {
		registry.close()
		close(closeDone)
	}()
	select {
	case <-initializationCanceled:
	case <-time.After(time.Second):
		t.Fatal("registry close did not cancel active event reader initialization")
	}
	select {
	case err := <-subscribeDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("subscribe error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("active event reader initialization did not return after close")
	}
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("registry close did not wait for active event reader initialization")
	}
}

func TestGenerationStreamRegistryCloseStopsActiveEventReader(t *testing.T) {
	readerContexts := make(chan context.Context, 1)
	readCanceled := make(chan struct{})
	store := &activeEventReaderTestStore{
		read: func(ctx context.Context) ([]repository.GenerationStreamMessage, error) {
			readerContexts <- ctx
			<-ctx.Done()
			close(readCanceled)
			return nil, ctx.Err()
		},
	}
	registry := newGenerationStreamRegistry(store, generationStreamOptions{})
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	_, events, unsubscribe, err := registry.subscribeActive(requestCtx, 7)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()

	var readerCtx context.Context
	select {
	case readerCtx = <-readerContexts:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for active event reader to start")
	}
	cancelRequest()
	select {
	case <-readerCtx.Done():
		t.Fatal("request cancellation stopped the process-scoped active event reader")
	default:
	}

	registry.close()
	select {
	case <-readCanceled:
	default:
		t.Fatal("registry close returned before the active event reader stopped")
	}
	if _, ok := <-events; ok {
		t.Fatal("registry close did not close active subscribers")
	}
	registry.close()
}

func TestGenerationStreamRegistryCloseCancelsActiveGenerations(t *testing.T) {
	store := cachememory.New()
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		ActiveTTL:    time.Hour,
		LeaseTTL:     time.Hour,
		LeaseRefresh: 30 * time.Minute,
	})
	lifecycleCtx, release, ok := registry.acquireRun(context.Background())
	if !ok {
		t.Fatal("expected lifecycle acquisition before close")
	}
	canceled := make(chan struct{}, 1)
	cancelCount := 0
	if err := registry.register(lifecycleCtx, "run_close_active", 7, "conv_close_active", func() {
		cancelCount++
		canceled <- struct{}{}
	}); err != nil {
		t.Fatal(err)
	}

	registry.mu.Lock()
	active := registry.active["run_close_active"]
	registry.mu.Unlock()
	if active == nil {
		t.Fatal("expected active generation before close")
	}

	closeDone := make(chan struct{})
	go func() {
		registry.close()
		close(closeDone)
	}()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("registry close did not cancel the active generation")
	}
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("registry close waited for the canceled request handler")
	}
	registry.finish(context.WithoutCancel(lifecycleCtx), "run_close_active")
	release()
	if cancelCount != 1 {
		t.Fatalf("registry close canceled the active generation %d times, want exactly once", cancelCount)
	}
	release()
	registry.mu.Lock()
	activeCount := len(registry.active)
	closed := registry.closed
	registry.mu.Unlock()
	if !closed || activeCount != 0 {
		t.Fatalf("registry close left lifecycle state behind: closed=%v active=%d", closed, activeCount)
	}
	select {
	case <-active.workerDone:
	default:
		t.Fatal("registry close did not stop the active lifecycle worker")
	}
	lateRegistrationCanceled := false
	if err := registry.register(context.Background(), "run_after_close", 7, "conv_after_close", func() {
		lateRegistrationCanceled = true
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("registration after close error = %v, want context canceled", err)
	}
	if !lateRegistrationCanceled {
		t.Fatal("registration after close was not canceled")
	}
	if _, persistedAfterClose, err := store.GetGenerationStreamOwner(context.Background(), "run_after_close"); err != nil || persistedAfterClose {
		t.Fatal("registration after close touched the generation stream store")
	}
}

func TestGenerationStreamRegistryAcquireRejectedAfterClose(t *testing.T) {
	registry := newGenerationStreamRegistry(nil, generationStreamOptions{})
	registry.close()

	lifecycleCtx, release, ok := registry.acquireRun(context.Background())
	if ok || lifecycleCtx != nil || release != nil {
		t.Fatal("expected lifecycle acquisition after close to be rejected")
	}
}

func TestGenerationStreamRegistryCloseCancelsLifecycleBeforeRegistration(t *testing.T) {
	registry := newGenerationStreamRegistry(nil, generationStreamOptions{})
	lifecycleCtx, release, ok := registry.acquireRun(context.Background())
	if !ok {
		t.Fatal("expected lifecycle acquisition before close")
	}

	closeDone := make(chan struct{})
	go func() {
		registry.close()
		close(closeDone)
	}()

	select {
	case <-lifecycleCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("registry close did not cancel an unregistered lifecycle")
	}
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("registry close waited for an unregistered request lifecycle")
	}
	release()
}

func TestGenerationStreamRegistryRejectsDuplicateClaimAcrossRegistries(t *testing.T) {
	store := cachememory.New()
	firstRegistry := newGenerationStreamRegistry(store, generationStreamOptions{
		ActiveTTL:    time.Hour,
		LeaseTTL:     time.Hour,
		LeaseRefresh: 30 * time.Minute,
	})
	secondRegistry := newGenerationStreamRegistry(store, generationStreamOptions{
		ActiveTTL:    time.Hour,
		LeaseTTL:     time.Hour,
		LeaseRefresh: 30 * time.Minute,
	})
	firstCtx, releaseFirst, ok := firstRegistry.acquireRun(context.Background())
	if !ok {
		t.Fatal("expected first lifecycle acquisition")
	}
	defer releaseFirst()
	secondCtx, releaseSecond, ok := secondRegistry.acquireRun(context.Background())
	if !ok {
		t.Fatal("expected second lifecycle acquisition")
	}
	defer releaseSecond()

	runID := "run_duplicate_claim"
	firstCanceled := false
	if err := firstRegistry.register(firstCtx, runID, 7, "conv_duplicate", func() {
		firstCanceled = true
	}); err != nil {
		t.Fatal(err)
	}
	secondCanceled := false
	if err := secondRegistry.register(secondCtx, runID, 7, "conv_duplicate", func() {
		secondCanceled = true
	}); !errors.Is(err, ErrDuplicateMessageGenerationRun) {
		t.Fatalf("duplicate registration error = %v, want duplicate run", err)
	}
	if firstCanceled {
		t.Fatal("duplicate claim canceled the existing owner")
	}
	if !secondCanceled {
		t.Fatal("rejected duplicate execution was not canceled")
	}

	secondRegistry.finish(secondCtx, runID)
	if !firstRegistry.hasActive(context.Background(), runID) {
		t.Fatal("finish without local ownership cleared the active owner")
	}
	if _, accepted := firstRegistry.publish(firstCtx, runID, map[string]any{"type": "delta", "delta": "owned"}); !accepted {
		t.Fatal("duplicate claim prevented the original owner from publishing")
	}
	firstRegistry.finish(firstCtx, runID)
}

func TestGenerationStreamRegistryRejectsReplacementAfterLeaseExpires(t *testing.T) {
	store := cachememory.New()
	options := generationStreamOptions{
		ActiveTTL:    time.Hour,
		LeaseTTL:     20 * time.Millisecond,
		LeaseRefresh: 5 * time.Millisecond,
		MaxEvents:    8,
	}
	firstRegistry := newGenerationStreamRegistry(store, options)
	secondRegistry := newGenerationStreamRegistry(store, options)
	firstCtx, releaseFirst, ok := firstRegistry.acquireRun(context.Background())
	if !ok {
		t.Fatal("expected first lifecycle acquisition")
	}
	defer releaseFirst()
	secondCtx, releaseSecond, ok := secondRegistry.acquireRun(context.Background())
	if !ok {
		t.Fatal("expected second lifecycle acquisition")
	}
	defer releaseSecond()

	runID := "run_stale_execution"
	firstCanceled := false
	if err := firstRegistry.register(firstCtx, runID, 7, "conv_stale", func() {
		firstCanceled = true
	}); err != nil {
		t.Fatal(err)
	}
	firstActive := firstRegistry.localActive(runID)
	if firstActive == nil {
		t.Fatal("expected first local execution")
	}
	stopActiveWorker(firstActive)
	time.Sleep(40 * time.Millisecond)

	if err := secondRegistry.register(secondCtx, runID, 7, "conv_stale", func() {}); !errors.Is(err, ErrDuplicateMessageGenerationRun) {
		t.Fatalf("claim after expired lease error = %v, want duplicate run", err)
	}
	if renewed, err := firstRegistry.renewActiveLease(context.Background(), runID, firstActive); err != nil || renewed {
		t.Fatalf("stale lease renewal = %v, err=%v; want rejected", renewed, err)
	}
	if reset, err := store.ResetGenerationStreamEvents(context.Background(), firstActive.lease(runID)); err != nil || reset {
		t.Fatalf("stale stream reset = %v, err=%v; want rejected", reset, err)
	}
	if _, accepted := firstRegistry.publish(firstCtx, runID, map[string]any{"type": "error", "error": "stale"}); accepted {
		t.Fatal("stale execution appended a terminal event")
	}
	if !firstCanceled {
		t.Fatal("lost ownership did not cancel the stale local execution")
	}
	firstRegistry.finish(firstCtx, runID)
	if secondRegistry.hasActive(context.Background(), runID) {
		t.Fatal("expired execution unexpectedly became active again")
	}
}

func TestGenerationStreamRegistryCloseCancelsConcurrentRegistration(t *testing.T) {
	store := &blockingGenerationStreamRegistrationStore{
		GenerationStreamCacheRepository: cachememory.New(),
		started:                         make(chan struct{}),
	}
	registry := newGenerationStreamRegistry(store, generationStreamOptions{})
	lifecycleCtx, release, ok := registry.acquireRun(context.Background())
	if !ok {
		t.Fatal("expected lifecycle acquisition")
	}
	registrationCanceled := make(chan struct{})
	registrationDone := make(chan struct{})
	go func() {
		defer close(registrationDone)
		_ = registry.register(lifecycleCtx, "run_close_race", 7, "conv_close_race", func() {
			close(registrationCanceled)
		})
	}()
	select {
	case <-store.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for registration store call")
	}

	closeDone := make(chan struct{})
	go func() {
		registry.close()
		close(closeDone)
	}()
	select {
	case <-registrationCanceled:
	case <-time.After(time.Second):
		t.Fatal("registry close did not cancel concurrent registration")
	}
	select {
	case <-registrationDone:
	case <-time.After(time.Second):
		t.Fatal("concurrent registration did not stop after registry close")
	}
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("registry close waited for the request lifecycle after registration stopped")
	}
	release()
	registry.mu.Lock()
	activeCount := len(registry.active)
	registry.mu.Unlock()
	if activeCount != 0 {
		t.Fatalf("concurrent registration created %d active generations after close", activeCount)
	}
	if _, persisted, err := store.GetGenerationStreamOwner(context.Background(), "run_close_race"); err != nil || persisted {
		t.Fatalf("concurrent registration persisted stream after close: persisted=%v err=%v", persisted, err)
	}
}

func TestServiceListActiveMessageGenerationsDelegatesToRegistry(t *testing.T) {
	registry := newGenerationStreamRegistry(cachememory.New(), generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		LeaseTTL:         time.Second,
		LeaseRefresh:     100 * time.Millisecond,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	ctx, cleanup := registerTestGeneration(t, registry, "run_reconcile", 7, "conv_reconcile", func() {})
	defer cleanup()

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

type interruptedGenerationRepository struct {
	repository.ConversationRepository
	errorCode    string
	errorMessage string
}

func (r *interruptedGenerationRepository) InterruptPendingAssistantMessageByRunID(
	_ context.Context,
	_ uint,
	_ string,
	errorCode string,
	errorMessage string,
) (bool, error) {
	r.errorCode = errorCode
	r.errorMessage = errorMessage
	return true, nil
}

func TestMarkMessageGenerationInterruptedPersistsTypedContract(t *testing.T) {
	repo := &interruptedGenerationRepository{}
	service := &Service{repo: repo}

	service.MarkMessageGenerationInterrupted(t.Context(), 7, "run_interrupted")

	if repo.errorCode != ErrMessageGenerationInterrupted.Code() || repo.errorMessage != ErrMessageGenerationInterrupted.Message() {
		t.Fatalf("persisted interruption contract = (%q, %q)", repo.errorCode, repo.errorMessage)
	}
}

func TestGenerationStreamStoreActiveLeaseExpires(t *testing.T) {
	store := cachememory.New()
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	claimTestGeneration(t, store, runID, "execution_expiry", 7, "conv_test", 10*time.Millisecond)
	if active, err := store.IsGenerationStreamActive(ctx, runID); err != nil || !active {
		t.Fatalf("expected active lease, active=%v err=%v", active, err)
	}
	time.Sleep(25 * time.Millisecond)
	if active, err := store.IsGenerationStreamActive(ctx, runID); err != nil || active {
		t.Fatalf("expected expired active lease, active=%v err=%v", active, err)
	}
}

func TestGenerationStreamStoreReturnsLatestWindow(t *testing.T) {
	store := cachememory.New()
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	lease := claimTestGeneration(t, store, runID, "execution_window", 11, "conv_test", time.Minute)
	for i := 0; i < 5; i++ {
		if _, accepted, err := store.AppendGenerationStreamEvent(ctx, lease, repository.GenerationStreamAppend{PayloadJSON: `{"type":"delta"}`}, 3, time.Minute); err != nil || !accepted {
			t.Fatalf("append event: accepted=%v err=%v", accepted, err)
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
	store := cachememory.New()
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()

	largeOutput := strings.Repeat("x", generationStreamMaxPayloadBytes)
	tracePayload, err := json.Marshal(map[string]any{
		"tool_calls": []map[string]any{{
			"tool_call_id":   "call_1",
			"name":           "fetch",
			"status":         "success",
			"output":         largeOutput,
			"output_detail":  largeOutput,
			"output_text":    largeOutput,
			"output_preview": "short result",
			"output_presentation": map[string]any{
				"text": "## Structured result\n\n- first item",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	published := publishTestGeneration(t, registry, ctx, runID, map[string]any{
		"type":   "process_update",
		"status": "streaming",
		"trace": map[string]any{
			"tools": map[string]any{
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
		ToolCalls []map[string]any `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(parsed.Trace.Tools.PayloadJSON), &parsedTrace); err != nil {
		t.Fatal(err)
	}
	if len(parsedTrace.ToolCalls) != 1 {
		t.Fatalf("expected one sanitized tool call, got %#v", parsedTrace.ToolCalls)
	}
	call := parsedTrace.ToolCalls[0]
	if traceInt64(call["output_detail_size"]) != int64(len(largeOutput)) ||
		traceInt64(call["output_text_size"]) != int64(len(largeOutput)) {
		t.Fatalf("expected output size metadata in sanitized payload, got %#v", call)
	}
	if _, ok := call["output_detail"]; ok {
		t.Fatalf("expected oversized output detail to be removed, got %#v", call)
	}
	presentation, ok := call["output_presentation"].(map[string]any)
	if !ok || getTraceString(presentation["text"]) != "## Structured result\n\n- first item" {
		t.Fatalf("expected semantic output presentation to survive stream sanitization, got %#v", call)
	}
}

func TestGenerationStreamDoesNotCompactOversizedCompletedPayload(t *testing.T) {
	store := cachememory.New()
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        8,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()

	largeContent := strings.Repeat("a", generationStreamMaxPayloadBytes)
	published := publishTestGeneration(t, registry, ctx, runID, map[string]any{
		"type": "completed",
		"data": map[string]any{
			"assistantMessage": map[string]any{
				"content": largeContent,
			},
		},
	})

	if published["payloadTruncated"] == true {
		t.Fatalf("completed payload must not be compacted for active clients, got %#v", published)
	}
	data, ok := published["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected completed data to be preserved, got %#v", published)
	}
	assistant, ok := data["assistantMessage"].(map[string]any)
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

type activeEventReaderTestStore struct {
	repository.GenerationStreamCacheRepository
	list func(context.Context) ([]repository.GenerationStreamMessage, error)
	read func(context.Context) ([]repository.GenerationStreamMessage, error)
}

type generationStreamSubscriptionOwnershipStore struct {
	repository.GenerationStreamCacheRepository
	mu          sync.Mutex
	ownerID     uint
	readStarted chan struct{}
	releaseRead chan struct{}
	readOnce    sync.Once
	readCalls   int
}

func (s *generationStreamSubscriptionOwnershipStore) setOwner(ownerID uint) {
	s.mu.Lock()
	s.ownerID = ownerID
	s.mu.Unlock()
}

func (s *generationStreamSubscriptionOwnershipStore) GetGenerationStreamOwner(_ context.Context, _ string) (uint, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ownerID, s.ownerID != 0, nil
}

func (s *generationStreamSubscriptionOwnershipStore) ReadGenerationStreamEvents(
	ctx context.Context,
	_ string,
	_ string,
	_ time.Duration,
	_ int64,
) ([]repository.GenerationStreamMessage, error) {
	s.mu.Lock()
	s.readCalls++
	readCall := s.readCalls
	s.mu.Unlock()
	if readCall > 1 {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	s.readOnce.Do(func() { close(s.readStarted) })
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.releaseRead:
		return []repository.GenerationStreamMessage{{
			ID:          "1-0",
			Seq:         1,
			PayloadJSON: `{"type":"delta","delta":"replacement"}`,
		}}, nil
	}
}

type blockingGenerationStreamRegistrationStore struct {
	repository.GenerationStreamCacheRepository
	started chan struct{}
	once    sync.Once
}

func (s *blockingGenerationStreamRegistrationStore) ClaimGenerationStream(
	ctx context.Context,
	_ repository.GenerationStreamLease,
	_ time.Duration,
	_ time.Duration,
) (bool, error) {
	s.once.Do(func() { close(s.started) })
	<-ctx.Done()
	return false, ctx.Err()
}

func (s *activeEventReaderTestStore) ListGenerationStreamEvents(ctx context.Context, _ string, _ int64) ([]repository.GenerationStreamMessage, error) {
	if s.list != nil {
		return s.list(ctx)
	}
	return nil, nil
}

func (s *activeEventReaderTestStore) ListActiveGenerationStreams(_ context.Context, _ uint) ([]repository.ActiveGenerationStream, error) {
	return nil, nil
}

func (s *activeEventReaderTestStore) ReadGenerationStreamEvents(ctx context.Context, _ string, _ string, _ time.Duration, _ int64) ([]repository.GenerationStreamMessage, error) {
	return s.read(ctx)
}
