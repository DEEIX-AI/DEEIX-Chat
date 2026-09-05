package contentmoderation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type coordinatorTestRepo struct {
	mu          sync.Mutex
	createErr   error
	applyErr    error
	applyCalls  int
	applyInputs []bool
	events      []domaincm.Event
	stats       []repository.DailyStatIncrement
	latestHit   *domaincm.Event
	getEvent    *domaincm.Event
	getEventErr error
	runState    string
	staleRunIDs []string
}

func (r *coordinatorTestRepo) CreateEvent(_ context.Context, event *domaincm.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	if event != nil {
		r.events = append(r.events, *event)
	}
	return nil
}

type coordinatorTestObjectStore struct {
	mu      sync.Mutex
	put     []string
	deleted []string
}

func (s *coordinatorTestObjectStore) Put(_ context.Context, path string, _ []byte, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.put = append(s.put, path)
	return nil
}

func (s *coordinatorTestObjectStore) Open(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (s *coordinatorTestObjectStore) Delete(_ context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, path)
	return nil
}

func (r *coordinatorTestRepo) GetEventByPublicID(context.Context, string) (*domaincm.Event, error) {
	return r.getEvent, r.getEventErr
}

func (r *coordinatorTestRepo) GetLatestHitEventByRunID(context.Context, string) (*domaincm.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.latestHit, nil
}

func (r *coordinatorTestRepo) ListEvents(context.Context, domaincm.EventListFilter) ([]domaincm.Event, int64, error) {
	return nil, 0, nil
}

func (r *coordinatorTestRepo) ClearExpiredContentByPublicIDs(context.Context, []string) (int64, error) {
	return 0, nil
}

func (r *coordinatorTestRepo) ListExpiredContentEvents(context.Context, time.Time, int) ([]domaincm.Event, error) {
	return nil, nil
}

func (r *coordinatorTestRepo) DeleteExpiredMetadata(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (r *coordinatorTestRepo) IncrementDailyStat(_ context.Context, input repository.DailyStatIncrement) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats = append(r.stats, input)
	return nil
}

func (r *coordinatorTestRepo) ListDailyStats(context.Context, time.Time, time.Time) ([]domaincm.DailyStat, error) {
	return nil, nil
}

func (r *coordinatorTestRepo) DeleteDailyStatsBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (r *coordinatorTestRepo) UpdateRunModeration(_ context.Context, _ string, state string, _ string, _ string) error {
	r.mu.Lock()
	r.runState = state
	r.mu.Unlock()
	return nil
}

func (r *coordinatorTestRepo) ApplyRunBlock(_ context.Context, _ string, includeUser bool, _ string, _ string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.applyCalls++
	r.applyInputs = append(r.applyInputs, includeUser)
	return nil, r.applyErr
}

func (r *coordinatorTestRepo) GetRunModerationState(context.Context, string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runState, nil
}

func (r *coordinatorTestRepo) ListStaleModeratingRuns(context.Context, time.Time, int) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.staleRunIDs...), nil
}

func TestEphemeralCoordinatorBlockDoesNotMutateConversationRun(t *testing.T) {
	repo := &coordinatorTestRepo{}
	service := &Service{repo: repo}
	coord := newRunCoordinator(context.Background(), service, RunMeta{RunID: "temporary-run", Ephemeral: true}, runtimeConfig{})
	emitted := false
	coord.SetLiveEmitter(func(eventType string, _ map[string]any) {
		emitted = eventType == "moderation_blocked"
	})

	notified, err := coord.applyBlock(BlockInfo{EventID: "event", Direction: DirectionInput, Categories: []string{"unsafe"}})
	if err != nil {
		t.Fatalf("apply ephemeral block: %v", err)
	}
	if !notified || !emitted {
		t.Fatal("ephemeral block must still emit the terminal moderation event")
	}
	if repo.applyCalls != 0 {
		t.Fatalf("ApplyRunBlock calls = %d, want 0", repo.applyCalls)
	}
}

func TestKnownHitRemainsBlockedWhenDurableApplyFails(t *testing.T) {
	repo := &coordinatorTestRepo{applyErr: errors.New("database unavailable")}
	service := NewService(nil, repo, "", nil)
	coord := newRunCoordinator(context.Background(), service, RunMeta{RunID: "run_known_hit"}, runtimeConfig{Timeout: time.Second})
	coord.blocked = true
	coord.blockInfo = BlockInfo{EventID: "cme_hit", Direction: domaincm.DirectionOutput, Categories: []string{"violence"}}

	var emitted string
	coord.SetLiveEmitter(func(eventType string, _ map[string]any) {
		emitted = eventType
	})

	result := coord.AfterGeneration(context.Background(), "", nil)
	if result.Block == nil || result.State != domaincm.ModerationStateBlocked {
		t.Fatalf("known hit must remain blocked, got %#v", result)
	}
	if !result.TerminalEmitted || emitted != "moderation_blocked" {
		t.Fatalf("expected moderation_blocked terminal event, emitted=%q result=%#v", emitted, result)
	}
	if !service.hasPendingBlock("run_known_hit") {
		t.Fatal("failed durable apply must be registered for compensation")
	}
	repo.mu.Lock()
	repo.applyErr = nil
	repo.mu.Unlock()
	service.recoverPendingBlocks(context.Background())
	if service.hasPendingBlock("run_known_hit") {
		t.Fatal("successful compensation must remove the pending block")
	}
}

func TestLateHitRunsFullBlockCompensation(t *testing.T) {
	repo := &coordinatorTestRepo{}
	service := NewService(nil, repo, "", nil)
	coord := newRunCoordinator(context.Background(), service, RunMeta{RunID: "run_late_hit"}, runtimeConfig{})
	coord.pending = 1
	coord.outputEnqueued = true
	coord.settled = true

	var emitted string
	service.SetEventEmitter(func(_ context.Context, _ string, eventType string, _ map[string]any) {
		emitted = eventType
	})
	task := &moderationTask{Coord: coord, Direction: domaincm.DirectionOutput, Modality: domaincm.ModalityText}
	lateBlock := coord.onTaskResult(task, taskResult{Hit: true, EventID: "cme_late", Categories: []string{"violence"}})
	if lateBlock == nil {
		t.Fatal("late hit must request background block compensation")
	}
	service.handleLateBlock(coord.ctx, coord.meta, *lateBlock)

	repo.mu.Lock()
	applyCalls := repo.applyCalls
	repo.mu.Unlock()
	if applyCalls != 1 {
		t.Fatalf("expected one durable block apply, got %d", applyCalls)
	}
	if emitted != "moderation_blocked" {
		t.Fatalf("expected recovery moderation_blocked event, got %q", emitted)
	}
}

func TestInputHitTakesPrecedenceOverOutputHit(t *testing.T) {
	repo := &coordinatorTestRepo{}
	service := NewService(nil, repo, "", nil)
	cancelCalls := 0
	service.SetCancelRun(func(context.Context, string) { cancelCalls++ })
	coord := newRunCoordinator(context.Background(), service, RunMeta{RunID: "run_input_priority"}, runtimeConfig{})
	coord.pending = 2
	coord.outputEnqueued = true

	outputTask := &moderationTask{Coord: coord, Direction: domaincm.DirectionOutput, Modality: domaincm.ModalityText}
	coord.onTaskResult(outputTask, taskResult{Hit: true, EventID: "cme_output", Categories: []string{"violence"}})
	inputTask := &moderationTask{Coord: coord, Direction: domaincm.DirectionInput, Modality: domaincm.ModalityText}
	coord.onTaskResult(inputTask, taskResult{Hit: true, EventID: "cme_input", Categories: []string{"hate"}})

	blocked, info, _ := coord.settle()
	if !blocked || info.Direction != domaincm.DirectionInput || info.EventID != "cme_input" {
		t.Fatalf("input hit must win, got blocked=%v info=%#v", blocked, info)
	}
	if cancelCalls != 1 {
		t.Fatalf("input hit must cancel generation once, got %d", cancelCalls)
	}
}

func TestLateInputHitUpgradesHandledOutputBlock(t *testing.T) {
	repo := &coordinatorTestRepo{}
	service := NewService(nil, repo, "", nil)
	coord := newRunCoordinator(context.Background(), service, RunMeta{RunID: "run_late_input"}, runtimeConfig{})
	coord.pending = 1
	coord.outputEnqueued = true
	coord.settled = true
	coord.blocked = true
	coord.blockHandled = true
	coord.blockInfo = BlockInfo{EventID: "cme_output", Direction: domaincm.DirectionOutput}

	inputTask := &moderationTask{Coord: coord, Direction: domaincm.DirectionInput, Modality: domaincm.ModalityText}
	lateBlock := coord.onTaskResult(inputTask, taskResult{Hit: true, EventID: "cme_input", Categories: []string{"hate"}})
	if lateBlock == nil || lateBlock.Direction != domaincm.DirectionInput {
		t.Fatalf("late input hit must request an upgraded block, got %#v", lateBlock)
	}
	service.handleLateBlock(coord.ctx, coord.meta, *lateBlock)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.applyInputs) != 1 || !repo.applyInputs[0] {
		t.Fatalf("late input block must include the user message, calls=%#v", repo.applyInputs)
	}
}

func TestWorkerPrefetchDoesNotBypassQueueCapacity(t *testing.T) {
	repo := &coordinatorTestRepo{}
	service := NewService(nil, repo, "", nil)
	service.maxConcurrency = 1
	service.queueCapacity = 1
	service.activeWorkers = 1 // keep the worker waiting for a logical slot

	ctx, cancel := context.WithCancel(context.Background())
	service.wg.Add(1)
	go service.workerLoop(ctx)

	coord := newRunCoordinator(context.Background(), service, RunMeta{RunID: "run_queue"}, runtimeConfig{})
	first := &moderationTask{Coord: coord, Direction: domaincm.DirectionOutput, Modality: domaincm.ModalityText}
	if err := service.enqueue(first); err != nil {
		t.Fatalf("enqueue first task: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for len(service.taskQueue) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(service.taskQueue) != 0 {
		t.Fatal("worker did not prefetch first task")
	}

	second := &moderationTask{Direction: domaincm.DirectionOutput, Modality: domaincm.ModalityText}
	if err := service.enqueue(second); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("prefetched waiting task must still consume queue capacity, got %v", err)
	}
	cancel()
	service.wg.Wait()
}

func TestOutputImageLoadFailureIsAuditedAsFailedOpen(t *testing.T) {
	repo := &coordinatorTestRepo{}
	service := NewService(nil, repo, "", nil)
	cfg := runtimeConfig{
		Policy: Policy{OutputImageCategories: []string{"violence"}},
	}
	service.cachedConfig = &cfg
	service.cachedAt = time.Now()
	coord := newRunCoordinator(context.Background(), service, RunMeta{RunID: "run_image_load"}, cfg)

	coord.RecordOutputImageFailure("file_123", errors.New("object missing"))

	coord.mu.Lock()
	failedOpen := coord.failedOpen
	coord.mu.Unlock()
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if !failedOpen {
		t.Fatal("missing output image must mark the run failed-open")
	}
	if len(repo.events) != 1 || repo.events[0].Result != domaincm.ResultFailedOpen {
		t.Fatalf("expected one failed-open audit event, got %#v", repo.events)
	}
	if len(repo.stats) != 1 || repo.stats[0].FailureCount != 1 {
		t.Fatalf("expected failed-open statistics, got %#v", repo.stats)
	}
}

func TestInputImageModerationSkipsNonImageAttachments(t *testing.T) {
	repo := &coordinatorTestRepo{}
	service := NewService(nil, repo, "", nil)
	service.SetImageLoader(func(context.Context, uint, string) (PreparedImage, error) {
		return PreparedImage{}, ErrNonImageAttachment
	})
	cfg := runtimeConfig{Policy: Policy{InputImageCategories: []string{"violence"}}}
	coord := newRunCoordinator(context.Background(), service, RunMeta{RunID: "run_document_attachment"}, cfg)

	coord.EnqueueInputImages(context.Background(), []string{"file_pdf"})

	coord.mu.Lock()
	failedOpen := coord.failedOpen
	pending := coord.pending
	coord.mu.Unlock()
	repo.mu.Lock()
	eventCount := len(repo.events)
	repo.mu.Unlock()
	if failedOpen || pending != 0 || eventCount != 0 {
		t.Fatalf("non-image attachment must be ignored: failedOpen=%v pending=%d events=%d", failedOpen, pending, eventCount)
	}
}

func TestEphemeralInputImageModerationQueuesRequestScopedBytes(t *testing.T) {
	service := NewService(nil, &coordinatorTestRepo{}, "", nil)
	cfg := runtimeConfig{Policy: Policy{InputImageCategories: []string{"violence"}}}
	coord := newRunCoordinator(context.Background(), service, RunMeta{RunID: "run_ephemeral_image", Ephemeral: true}, cfg)

	coord.EnqueueInputImageSources([]OutputImageSource{
		{FileID: "temporary_image", Data: []byte("image-bytes"), MimeType: "image/png", SHA256: "sha"},
		{FileID: "temporary_duplicate", Data: []byte("duplicate"), MimeType: "image/png", SHA256: "sha"},
	})

	select {
	case task := <-service.taskQueue:
		if task == nil || task.Modality != domaincm.ModalityImage || !task.IsolateOnly {
			t.Fatalf("unexpected request-scoped moderation task: %#v", task)
		}
		if len(task.RawImages) != 1 || string(task.RawImages[0].Data) != "image-bytes" {
			t.Fatalf("request-scoped image bytes were not queued safely: %#v", task.RawImages)
		}
	default:
		t.Fatal("request-scoped image moderation task was not queued")
	}
}

func TestRecordHitRollsBackIsolatedImagesWhenEventCreateFails(t *testing.T) {
	repo := &coordinatorTestRepo{createErr: errors.New("database unavailable")}
	store := &coordinatorTestObjectStore{}
	service := NewService(nil, repo, "test-encryption-key", nil)
	service.SetObjectStore(store)
	coord := newRunCoordinator(context.Background(), service, RunMeta{RunID: "run_rollback", UserID: 42}, runtimeConfig{})
	task := &moderationTask{
		Coord:     coord,
		Direction: domaincm.DirectionOutput,
		Modality:  domaincm.ModalityImage,
		RawImages: []OutputImageSource{{FileID: "file_1", Data: []byte("image-bytes"), MimeType: "image/png"}},
	}

	eventID, err := service.recordHit(context.Background(), task, HitEvaluation{
		Hit:        true,
		Categories: []string{"violence"},
		Scores:     map[string]float64{"violence": 0.99},
	}, 10, nil)
	if !errors.Is(err, repo.createErr) {
		t.Fatalf("record hit error=%v, want %v", err, repo.createErr)
	}
	if eventID != "" {
		t.Fatalf("failed event persistence exposed dangling event ID %q", eventID)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.put) != 1 || len(store.deleted) != 1 || store.put[0] != store.deleted[0] {
		t.Fatalf("isolated image was not compensated: put=%#v deleted=%#v", store.put, store.deleted)
	}
}
