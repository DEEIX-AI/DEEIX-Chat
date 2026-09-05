package memory

import (
	"context"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestGenerationStreamClaimCancelAndComplete(t *testing.T) {
	cache := New()
	ctx := context.Background()
	lease := testGenerationStreamLease("run_memory_lifecycle", "execution_1")

	if claimed, err := cache.ClaimGenerationStream(ctx, lease, time.Minute, time.Minute); err != nil || !claimed {
		t.Fatalf("claim=%v err=%v, want true nil", claimed, err)
	}
	if canceled, err := cache.IsGenerationStreamCanceled(ctx, lease.RunID); err != nil || canceled {
		t.Fatalf("new claim canceled=%v err=%v, want false nil", canceled, err)
	}
	if active, err := cache.IsGenerationStreamActive(ctx, lease.RunID); err != nil || !active {
		t.Fatalf("new claim active=%v err=%v, want true nil", active, err)
	}
	if ownerID, ok, err := cache.GetGenerationStreamOwner(ctx, lease.RunID); err != nil || !ok || ownerID != lease.UserID {
		t.Fatalf("owner=(%d,%v) err=%v, want (%d,true) nil", ownerID, ok, err, lease.UserID)
	}
	if requested, err := cache.RequestGenerationStreamCancel(ctx, lease.RunID, lease.UserID, time.Minute); err != nil || !requested {
		t.Fatalf("cancel requested=%v err=%v, want true nil", requested, err)
	}
	if canceled, err := cache.IsGenerationStreamCanceled(ctx, lease.RunID); err != nil || !canceled {
		t.Fatalf("canceled=%v err=%v, want true nil", canceled, err)
	}
	if claimed, err := cache.ClaimGenerationStream(ctx, testGenerationStreamLease(lease.RunID, "execution_2"), time.Minute, time.Minute); err != nil || claimed {
		t.Fatalf("duplicate claim=%v err=%v, want false nil", claimed, err)
	}
	if completed, err := cache.CompleteGenerationStream(ctx, lease, time.Minute); err != nil || !completed {
		t.Fatalf("complete=%v err=%v, want true nil", completed, err)
	}
	if active, err := cache.IsGenerationStreamActive(ctx, lease.RunID); err != nil || active {
		t.Fatalf("completed stream active=%v err=%v, want false nil", active, err)
	}
	if requested, err := cache.RequestGenerationStreamCancel(ctx, lease.RunID, lease.UserID, time.Minute); err != nil || requested {
		t.Fatalf("completed cancel requested=%v err=%v, want false nil", requested, err)
	}
}

func TestGenerationStreamClaimIsIdempotentForSameExecution(t *testing.T) {
	cache := New()
	ctx := context.Background()
	lease := testGenerationStreamLease("run_memory_idempotent_claim", "execution_1")
	mustClaimGenerationStream(t, cache, lease, time.Minute)
	if _, accepted, err := cache.AppendGenerationStreamEvent(ctx, lease, repository.GenerationStreamAppend{
		PayloadJSON: `{"type":"delta"}`,
		TextDelta:   "kept",
	}, 8, time.Minute); err != nil || !accepted {
		t.Fatalf("append accepted=%v err=%v, want true nil", accepted, err)
	}

	if claimed, err := cache.ClaimGenerationStream(ctx, lease, time.Minute, time.Minute); err != nil || !claimed {
		t.Fatalf("repeated claim=%v err=%v, want true nil", claimed, err)
	}
	events, err := cache.ListGenerationStreamEvents(ctx, lease.RunID, 8)
	if err != nil || len(events) != 1 {
		t.Fatalf("repeated claim changed retained events: events=%+v err=%v", events, err)
	}
}

func TestGenerationStreamTextSnapshotLifecycle(t *testing.T) {
	cache := New()
	ctx := context.Background()
	lease := testGenerationStreamLease("run_memory_text_snapshot", "execution_text")
	mustClaimGenerationStream(t, cache, lease, time.Minute)

	for _, delta := range []string{"完整", "恢复", "文本"} {
		if _, accepted, err := cache.AppendGenerationStreamEvent(ctx, lease, repository.GenerationStreamAppend{
			PayloadJSON: `{"type":"delta"}`,
			TextDelta:   delta,
		}, 2, time.Minute); err != nil || !accepted {
			t.Fatalf("append accepted=%v err=%v", accepted, err)
		}
	}

	snapshot, ok, err := cache.GetGenerationStreamTextSnapshot(ctx, lease.RunID)
	if err != nil || !ok || snapshot.Content != "完整恢复文本" || snapshot.Seq != 3 {
		t.Fatalf("unexpected snapshot=%+v ok=%v err=%v", snapshot, ok, err)
	}
	events, err := cache.ListGenerationStreamEvents(ctx, lease.RunID, 2)
	if err != nil || len(events) != 2 {
		t.Fatalf("expected bounded event window, events=%+v err=%v", events, err)
	}
	if reset, err := cache.ResetGenerationStreamEvents(ctx, lease); err != nil || !reset {
		t.Fatalf("reset=%v err=%v, want true nil", reset, err)
	}
	if snapshot, ok, err = cache.GetGenerationStreamTextSnapshot(ctx, lease.RunID); err != nil || ok {
		t.Fatalf("snapshot survived reset: snapshot=%+v ok=%v err=%v", snapshot, ok, err)
	}
	if _, accepted, err := cache.AppendGenerationStreamEvent(ctx, lease, repository.GenerationStreamAppend{
		PayloadJSON: `{"type":"delta"}`,
		TextDelta:   "安全内容",
	}, 2, time.Minute); err != nil || !accepted {
		t.Fatalf("append after reset accepted=%v err=%v", accepted, err)
	}
	snapshot, ok, err = cache.GetGenerationStreamTextSnapshot(ctx, lease.RunID)
	if err != nil || !ok || snapshot.Content != "安全内容" || snapshot.Seq != 4 {
		t.Fatalf("unexpected snapshot after reset: snapshot=%+v ok=%v err=%v", snapshot, ok, err)
	}
}

func TestGenerationStreamUpstreamThinkSnapshotLifecycle(t *testing.T) {
	cache := New()
	ctx := context.Background()
	lease := testGenerationStreamLease("run_memory_upstream_think_snapshot", "execution_think")
	mustClaimGenerationStream(t, cache, lease, time.Minute)

	updates := []*repository.GenerationStreamUpstreamThinkAppend{
		{RoundID: "round_1", Delta: "思考", MetadataJSON: `{"type":"upstream_think_delta","roundID":"round_1"}`},
		{RoundID: "round_1", ContentMarkdown: "完整思考", Replace: true, MetadataJSON: `{"type":"upstream_think_delta","roundID":"round_1"}`},
		{RoundID: "round_1", Delta: "继续", MetadataJSON: `{"type":"upstream_think_delta","roundID":"round_1"}`},
	}
	for _, update := range updates {
		if _, accepted, err := cache.AppendGenerationStreamEvent(ctx, lease, repository.GenerationStreamAppend{
			PayloadJSON:   `{"type":"upstream_think_delta"}`,
			UpstreamThink: update,
		}, 2, time.Minute); err != nil || !accepted {
			t.Fatalf("append accepted=%v err=%v", accepted, err)
		}
	}

	snapshot, ok, err := cache.GetGenerationStreamUpstreamThinkSnapshot(ctx, lease.RunID)
	if err != nil || !ok || snapshot.ContentMarkdown != "完整思考继续" || snapshot.RoundID != "round_1" || snapshot.Seq != 3 {
		t.Fatalf("unexpected snapshot=%+v ok=%v err=%v", snapshot, ok, err)
	}
	if _, accepted, err := cache.AppendGenerationStreamEvent(ctx, lease, repository.GenerationStreamAppend{
		PayloadJSON: `{"type":"upstream_think_delta"}`,
		UpstreamThink: &repository.GenerationStreamUpstreamThinkAppend{
			RoundID:      "round_2",
			Delta:        "下一轮",
			MetadataJSON: `{"type":"upstream_think_delta","roundID":"round_2"}`,
		},
	}, 2, time.Minute); err != nil || !accepted {
		t.Fatalf("next round accepted=%v err=%v", accepted, err)
	}
	snapshot, ok, err = cache.GetGenerationStreamUpstreamThinkSnapshot(ctx, lease.RunID)
	if err != nil || !ok || snapshot.ContentMarkdown != "下一轮" || snapshot.RoundID != "round_2" || snapshot.Seq != 4 {
		t.Fatalf("unexpected next-round snapshot=%+v ok=%v err=%v", snapshot, ok, err)
	}
}

func TestGenerationStreamRejectsReuseUntilOwnershipExpires(t *testing.T) {
	cache := New()
	ctx := context.Background()
	first := testGenerationStreamLease("run_memory_takeover", "execution_1")
	second := testGenerationStreamLease(first.RunID, "execution_2")
	claimed, err := cache.ClaimGenerationStream(ctx, first, 10*time.Millisecond, 80*time.Millisecond)
	if err != nil || !claimed {
		t.Fatalf("first claim=%v err=%v, want true nil", claimed, err)
	}
	time.Sleep(20 * time.Millisecond)
	if claimed, err = cache.ClaimGenerationStream(ctx, second, time.Minute, time.Minute); err != nil || claimed {
		t.Fatalf("claim before ownership expiry=%v err=%v, want false nil", claimed, err)
	}
	time.Sleep(70 * time.Millisecond)
	mustClaimGenerationStream(t, cache, second, time.Minute)

	if _, accepted, err := cache.AppendGenerationStreamEvent(ctx, first, repository.GenerationStreamAppend{PayloadJSON: `{"type":"delta"}`}, 8, time.Minute); err != nil || accepted {
		t.Fatalf("stale append accepted=%v err=%v, want false nil", accepted, err)
	}
	if reset, err := cache.ResetGenerationStreamEvents(ctx, first); err != nil || reset {
		t.Fatalf("stale reset=%v err=%v, want false nil", reset, err)
	}
	if renewed, err := cache.RenewGenerationStreamLease(ctx, first, time.Minute, time.Minute); err != nil || renewed {
		t.Fatalf("stale renew=%v err=%v, want false nil", renewed, err)
	}
	if completed, err := cache.CompleteGenerationStream(ctx, first, time.Minute); err != nil || completed {
		t.Fatalf("stale complete=%v err=%v, want false nil", completed, err)
	}
	if abandoned, err := cache.AbandonGenerationStream(ctx, first); err != nil || abandoned {
		t.Fatalf("stale abandon=%v err=%v, want false nil", abandoned, err)
	}
	if _, accepted, err := cache.AppendGenerationStreamEvent(ctx, second, repository.GenerationStreamAppend{PayloadJSON: `{"type":"delta"}`}, 8, time.Minute); err != nil || !accepted {
		t.Fatalf("current append accepted=%v err=%v, want true nil", accepted, err)
	}
}

func TestGenerationStreamCompletesAfterActiveLeaseExpires(t *testing.T) {
	cache := New()
	ctx := context.Background()
	lease := testGenerationStreamLease("run_memory_expired_active", "execution_1")
	if claimed, err := cache.ClaimGenerationStream(ctx, lease, 10*time.Millisecond, 200*time.Millisecond); err != nil || !claimed {
		t.Fatalf("claim=%v err=%v, want true nil", claimed, err)
	}
	time.Sleep(20 * time.Millisecond)
	if completed, err := cache.CompleteGenerationStream(ctx, lease, time.Minute); err != nil || !completed {
		t.Fatalf("complete after active expiry=%v err=%v, want true nil", completed, err)
	}
	if active, err := cache.IsGenerationStreamActive(ctx, lease.RunID); err != nil || active {
		t.Fatalf("completed stream active=%v err=%v, want false nil", active, err)
	}
}

func testGenerationStreamLease(runID string, executionID string) repository.GenerationStreamLease {
	return repository.GenerationStreamLease{
		RunID:                runID,
		ExecutionID:          executionID,
		UserID:               7,
		ConversationPublicID: "conv_test",
	}
}

func mustClaimGenerationStream(t *testing.T, cache *Cache, lease repository.GenerationStreamLease, ttl time.Duration) {
	t.Helper()
	claimed, err := cache.ClaimGenerationStream(context.Background(), lease, ttl, ttl)
	if err != nil || !claimed {
		t.Fatalf("claim=%v err=%v, want true nil", claimed, err)
	}
}
