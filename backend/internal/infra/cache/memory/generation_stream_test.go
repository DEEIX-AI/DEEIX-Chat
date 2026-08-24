package memory

import (
	"context"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestGenerationStreamRegisterDoesNotMarkCanceled(t *testing.T) {
	cache := New()
	ctx := context.Background()
	runID := "run_memory_cancel_state"

	if err := cache.RegisterGenerationStream(ctx, runID, 7, "conv_test", time.Minute); err != nil {
		t.Fatalf("register generation stream: %v", err)
	}

	if canceled, err := cache.IsGenerationStreamCanceled(ctx, runID); err != nil || canceled {
		t.Fatalf("newly registered stream canceled=%v err=%v, want false nil", canceled, err)
	}
	if active, err := cache.IsGenerationStreamActive(ctx, runID); err != nil || active {
		t.Fatalf("newly registered stream active=%v err=%v, want false nil", active, err)
	}
	if ownerID, ok, err := cache.GetGenerationStreamOwner(ctx, runID); err != nil || !ok || ownerID != 7 {
		t.Fatalf("owner=(%d,%v) err=%v, want (7,true) nil", ownerID, ok, err)
	}

	if err := cache.RequestGenerationStreamCancel(ctx, runID, time.Minute); err != nil {
		t.Fatalf("request cancel: %v", err)
	}
	if canceled, err := cache.IsGenerationStreamCanceled(ctx, runID); err != nil || !canceled {
		t.Fatalf("requested stream canceled=%v err=%v, want true nil", canceled, err)
	}

	if err := cache.RegisterGenerationStream(ctx, runID, 7, "conv_test", time.Minute); err != nil {
		t.Fatalf("register generation stream after cancel: %v", err)
	}
	if canceled, err := cache.IsGenerationStreamCanceled(ctx, runID); err != nil || canceled {
		t.Fatalf("re-registered stream canceled=%v err=%v, want false nil", canceled, err)
	}
}

func TestGenerationStreamTextSnapshotLifecycle(t *testing.T) {
	cache := New()
	ctx := context.Background()
	runID := "run_memory_text_snapshot"
	if err := cache.RegisterGenerationStream(ctx, runID, 7, "conv_test", time.Minute); err != nil {
		t.Fatal(err)
	}

	for _, delta := range []string{"完整", "恢复", "文本"} {
		if _, err := cache.AppendGenerationStreamEvent(ctx, runID, repository.GenerationStreamAppend{
			PayloadJSON: `{"type":"delta"}`,
			TextDelta:   delta,
		}, 2, time.Minute); err != nil {
			t.Fatal(err)
		}
	}

	snapshot, ok, err := cache.GetGenerationStreamTextSnapshot(ctx, runID)
	if err != nil || !ok {
		t.Fatalf("snapshot ok=%v err=%v", ok, err)
	}
	if snapshot.Content != "完整恢复文本" || snapshot.Seq != 3 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	events, err := cache.ListGenerationStreamEvents(ctx, runID, 2)
	if err != nil || len(events) != 2 {
		t.Fatalf("expected bounded event window, events=%+v err=%v", events, err)
	}

	if err := cache.ResetGenerationStreamEvents(ctx, runID); err != nil {
		t.Fatal(err)
	}
	if snapshot, ok, err = cache.GetGenerationStreamTextSnapshot(ctx, runID); err != nil || ok {
		t.Fatalf("snapshot survived reset: snapshot=%+v ok=%v err=%v", snapshot, ok, err)
	}
	if _, err := cache.AppendGenerationStreamEvent(ctx, runID, repository.GenerationStreamAppend{
		PayloadJSON: `{"type":"delta"}`,
		TextDelta:   "安全内容",
	}, 2, time.Minute); err != nil {
		t.Fatal(err)
	}
	snapshot, ok, err = cache.GetGenerationStreamTextSnapshot(ctx, runID)
	if err != nil || !ok || snapshot.Content != "安全内容" || snapshot.Seq != 4 {
		t.Fatalf("unexpected snapshot after reset: snapshot=%+v ok=%v err=%v", snapshot, ok, err)
	}
}

func TestGenerationStreamClearActiveMarksInactive(t *testing.T) {
	cache := New()
	ctx := context.Background()
	runID := "run_memory_active_state"

	if err := cache.RegisterGenerationStream(ctx, runID, 7, "conv_test", time.Minute); err != nil {
		t.Fatalf("register generation stream: %v", err)
	}
	if err := cache.TouchGenerationStreamActive(ctx, runID, 7, time.Minute); err != nil {
		t.Fatalf("touch active stream: %v", err)
	}
	if active, err := cache.IsGenerationStreamActive(ctx, runID); err != nil || !active {
		t.Fatalf("touched stream active=%v err=%v, want true nil", active, err)
	}
	items, err := cache.ListActiveGenerationStreams(ctx, 7)
	if err != nil || len(items) != 1 || items[0].RunID != runID || items[0].ConversationPublicID != "conv_test" {
		t.Fatalf("active streams=%+v err=%v, want registered run", items, err)
	}
	if otherItems, otherErr := cache.ListActiveGenerationStreams(ctx, 8); otherErr != nil || len(otherItems) != 0 {
		t.Fatalf("other user active streams=%+v err=%v, want empty", otherItems, otherErr)
	}

	if err := cache.ClearGenerationStreamActive(ctx, runID, 7); err != nil {
		t.Fatalf("clear active stream: %v", err)
	}
	if active, err := cache.IsGenerationStreamActive(ctx, runID); err != nil || active {
		t.Fatalf("cleared stream active=%v err=%v, want false nil", active, err)
	}
	if items, err = cache.ListActiveGenerationStreams(ctx, 7); err != nil || len(items) != 0 {
		t.Fatalf("active streams after clear=%+v err=%v, want empty", items, err)
	}
}
