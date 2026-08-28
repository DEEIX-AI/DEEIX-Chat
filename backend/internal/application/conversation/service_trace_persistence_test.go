package conversation

import (
	"context"
	"sync"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type orderedTraceRepository struct {
	repository.ConversationRepository

	mu           sync.Mutex
	traceRows    []model.MessageTrace
	eventRows    []model.MessageTraceEventRow
	firstStarted chan struct{}
	releaseFirst chan struct{}
	startOnce    sync.Once
}

func (r *orderedTraceRepository) UpsertConversationMessageTrace(_ context.Context, item *model.MessageTrace) error {
	r.startOnce.Do(func() {
		close(r.firstStarted)
		<-r.releaseFirst
	})
	r.mu.Lock()
	defer r.mu.Unlock()
	r.traceRows = append(r.traceRows, *item)
	return nil
}

func (r *orderedTraceRepository) UpsertConversationMessageTraceEvent(_ context.Context, item *model.MessageTraceEventRow) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eventRows = append(r.eventRows, *item)
	return nil
}

func TestTerminalTracePersistencePreservesReconciliationOrder(t *testing.T) {
	repo := &orderedTraceRepository{
		firstStarted: make(chan struct{}),
		releaseFirst: make(chan struct{}),
	}
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		ctx:       context.Background(),
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_ordered_trace"},
		service:   &Service{repo: repo},
	}

	payload := map[string]interface{}{"item_id": "reasoning_1", "status": "in_progress"}
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "stream snapshot", payload)
	recorder.completeUpstreamThink()

	select {
	case <-repo.firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first terminal trace persistence did not start")
	}

	recorder.reconcileStructuredThink(
		"final snapshot",
		"",
		map[string]interface{}{"item_id": "reasoning_1", "status": "completed"},
	)
	close(repo.releaseFirst)

	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	recorder.waitForPendingPersistence(waitCtx)
	if err := waitCtx.Err(); err != nil {
		t.Fatalf("wait for terminal trace persistence: %v", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.traceRows) != 2 || len(repo.eventRows) != 2 {
		t.Fatalf("expected two ordered trace snapshots, got rows=%d events=%d", len(repo.traceRows), len(repo.eventRows))
	}
	if got := repo.traceRows[0].ContentMarkdown; got != "stream snapshot" {
		t.Fatalf("first persisted trace = %q, want stream snapshot", got)
	}
	if got := repo.traceRows[1].ContentMarkdown; got != "final snapshot" {
		t.Fatalf("last persisted trace = %q, want final snapshot", got)
	}
	if got := repo.eventRows[1].ContentMarkdown; got != "final snapshot" {
		t.Fatalf("last persisted event = %q, want final snapshot", got)
	}
}
