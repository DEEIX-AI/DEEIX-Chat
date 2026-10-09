package errorlog

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	domainerrorlog "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/errorlog"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

type errorLogRepositoryStub struct {
	mu      sync.Mutex
	created []domainerrorlog.Log
	block   chan struct{}
	done    chan struct{}
}

func (stub *errorLogRepositoryStub) Create(ctx context.Context, item *domainerrorlog.Log) error {
	if stub.block != nil {
		<-stub.block
	}
	stub.mu.Lock()
	stub.created = append(stub.created, *item)
	stub.mu.Unlock()
	if stub.done != nil {
		stub.done <- struct{}{}
	}
	return ctx.Err()
}

func (*errorLogRepositoryStub) List(context.Context, int, int, repository.ErrorLogListFilter) ([]domainerrorlog.Log, int64, error) {
	return nil, 0, nil
}

func TestRecordPersistsNormalizedEntryAfterRequestEnds(t *testing.T) {
	repo := &errorLogRepositoryStub{done: make(chan struct{}, 1)}
	service := NewService(repo, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	service.Record(ctx, RecordInput{
		RequestID:  " req-1 ",
		StatusCode: 500,
		Cause:      strings.Repeat("x", maxCauseBytes+100),
		UserAgent:  strings.Repeat("u", 400),
	})
	// 请求结束会取消上下文，写入必须不受影响。
	cancel()

	select {
	case <-repo.done:
	case <-time.After(2 * time.Second):
		t.Fatal("error log was not written")
	}
	got := repo.created[0]
	if got.RequestID != "req-1" || len(got.Cause) != maxCauseBytes || len(got.UserAgent) != maxUserAgentBytes || got.CreatedAt.IsZero() {
		t.Fatalf("unexpected normalized entry: id=%q cause=%d ua=%d at=%v", got.RequestID, len(got.Cause), len(got.UserAgent), got.CreatedAt)
	}
}

func TestRecordSkipsEntriesWithoutRequestID(t *testing.T) {
	repo := &errorLogRepositoryStub{}
	NewService(repo, zap.NewNop()).Record(context.Background(), RecordInput{StatusCode: 500})
	time.Sleep(20 * time.Millisecond)
	if len(repo.created) != 0 {
		t.Fatalf("entries without request id must be skipped, got %d", len(repo.created))
	}
}

func TestRecordDropsWritesBeyondInflightLimit(t *testing.T) {
	repo := &errorLogRepositoryStub{block: make(chan struct{})}
	service := NewService(repo, zap.NewNop())
	for i := 0; i < maxInflightWrites+5; i++ {
		service.Record(context.Background(), RecordInput{RequestID: "req", StatusCode: 500})
	}
	if dropped := service.dropped.Load(); dropped != 5 {
		t.Fatalf("expected 5 dropped writes while the database is stuck, got %d", dropped)
	}
	close(repo.block)
}
