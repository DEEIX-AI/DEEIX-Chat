package conversation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

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
	registry.register(ctx, runID, 7, func() {})
	defer registry.finish(ctx, runID)

	first := registry.publish(ctx, runID, map[string]interface{}{"type": "delta", "delta": "a"})
	second := registry.publish(ctx, runID, map[string]interface{}{"type": "completed"})
	if first["seq"] != int64(1) || second["seq"] != int64(2) {
		t.Fatalf("unexpected seq values: first=%v second=%v", first["seq"], second["seq"])
	}

	replay, events, unsubscribe, ok := registry.subscribe(ctx, 7, runID, 1)
	if !ok {
		t.Fatal("expected subscription to existing run")
	}
	defer unsubscribe()
	if len(replay) != 1 || replay[0].Seq != 2 {
		t.Fatalf("unexpected replay events: %+v", replay)
	}
	if _, ok := <-events; ok {
		t.Fatal("terminal replay should close live event channel")
	}

	replay, events, unsubscribe, ok = registry.subscribe(ctx, 7, runID, 2)
	if !ok {
		t.Fatal("expected subscription after terminal seq to existing run")
	}
	defer unsubscribe()
	if len(replay) != 0 {
		t.Fatalf("unexpected replay after terminal seq: %+v", replay)
	}
	if _, ok := <-events; ok {
		t.Fatal("terminal state should close live event channel after last seq")
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
	registry.register(ctx, runID, 9, func() { canceled = true })
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
	registry.register(ctx, runID, 7, func() {})

	if !registry.hasActive(ctx, runID) {
		t.Fatal("expected active lease after register")
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
}

func TestGenerationStreamStoreActiveLeaseExpires(t *testing.T) {
	store := newTestGenerationStreamStore()
	ctx := context.Background()
	runID := EnsureMessageGenerationRunID("")
	if err := store.TouchGenerationStreamActive(ctx, runID, 10*time.Millisecond); err != nil {
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
	if err := store.RegisterGenerationStream(ctx, runID, 11, time.Minute); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := store.AppendGenerationStreamEvent(ctx, runID, `{"type":"delta"}`, 3, time.Minute); err != nil {
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

type testGenerationStreamStore struct {
	mu    sync.Mutex
	items map[string]*testGenerationStream
}

type testGenerationStream struct {
	userID      uint
	canceled    bool
	activeUntil time.Time
	nextSeq     int64
	events      []repository.GenerationStreamMessage
	expiresAt   time.Time
}

func newTestGenerationStreamStore() *testGenerationStreamStore {
	return &testGenerationStreamStore{items: map[string]*testGenerationStream{}}
}

func (s *testGenerationStreamStore) RegisterGenerationStream(_ context.Context, runID string, userID uint, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.ensureLocked(runID)
	item.userID = userID
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

func (s *testGenerationStreamStore) TouchGenerationStreamActive(_ context.Context, runID string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLocked(runID).activeUntil = time.Now().Add(ttl)
	return nil
}

func (s *testGenerationStreamStore) ClearGenerationStreamActive(_ context.Context, runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item, ok := s.items[runID]; ok {
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

func (s *testGenerationStreamStore) AppendGenerationStreamEvent(_ context.Context, runID string, payloadJSON string, maxEvents int64, ttl time.Duration) (repository.GenerationStreamMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.ensureLocked(runID)
	item.nextSeq++
	record := repository.GenerationStreamMessage{
		ID:          fmt.Sprintf("%d-0", item.nextSeq),
		Seq:         item.nextSeq,
		PayloadJSON: payloadJSON,
	}
	item.events = append(item.events, record)
	if maxEvents > 0 && int64(len(item.events)) > maxEvents {
		item.events = append([]repository.GenerationStreamMessage(nil), item.events[len(item.events)-int(maxEvents):]...)
	}
	item.expiresAt = time.Now().Add(ttl)
	return record, nil
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
