package conversation

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	activeGenerationEventRetention = 2 * time.Hour
	activeGenerationEventMaxEvents = 8192
	activeGenerationEventReadBlock = 5 * time.Minute
	activeGenerationEventBuffer    = 32
	activeGenerationEventStreamID  = "active_events_v1"
)

// ActiveMessageGeneration is the authoritative, user-scoped runtime snapshot.
type ActiveMessageGeneration struct {
	RunID                string
	ConversationPublicID string
}

// ActiveMessageGenerationEvent is one user-scoped navigation state change.
type ActiveMessageGenerationEvent struct {
	Type                 string
	RunID                string
	ConversationPublicID string
	UserID               uint
}

type activeMessageGenerationEventPayload struct {
	Type                 string `json:"type"`
	RunID                string `json:"runID"`
	ConversationPublicID string `json:"conversationPublicID,omitempty"`
	UserID               uint   `json:"userID"`
}

// SubscribeActiveMessageGenerations returns an authoritative snapshot followed
// by live user-scoped run state events. Redis Streams bridge multiple API nodes.
func (s *Service) SubscribeActiveMessageGenerations(
	ctx context.Context,
	userID uint,
) ([]ActiveMessageGeneration, <-chan ActiveMessageGenerationEvent, context.CancelFunc, error) {
	if s == nil || s.generationStreams == nil || userID == 0 {
		return []ActiveMessageGeneration{}, nil, func() {}, nil
	}
	return s.generationStreams.subscribeActive(ctx, userID)
}

// ListActiveMessageGenerations 返回用户当前活跃生成的权威快照。
// 供长连接周期对账使用：增量事件丢失时客户端可据此自愈失效的运行状态。
func (s *Service) ListActiveMessageGenerations(
	ctx context.Context,
	userID uint,
) ([]ActiveMessageGeneration, error) {
	if s == nil || s.generationStreams == nil || userID == 0 {
		return []ActiveMessageGeneration{}, nil
	}
	return s.generationStreams.listActive(ctx, userID)
}

func (r *generationStreamRegistry) listActive(ctx context.Context, userID uint) ([]ActiveMessageGeneration, error) {
	if r == nil || r.store == nil || userID == 0 {
		return []ActiveMessageGeneration{}, nil
	}
	stored, err := r.store.ListActiveGenerationStreams(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]ActiveMessageGeneration, 0, len(stored))
	seen := make(map[string]struct{}, len(stored))
	for _, item := range stored {
		runID := normalizeRunID(item.RunID)
		conversationPublicID := normalizePublicID(item.ConversationPublicID)
		if runID == "" || conversationPublicID == "" {
			continue
		}
		if _, exists := seen[runID]; exists {
			continue
		}
		seen[runID] = struct{}{}
		items = append(items, ActiveMessageGeneration{
			RunID:                runID,
			ConversationPublicID: conversationPublicID,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RunID < items[j].RunID })
	return items, nil
}

func (r *generationStreamRegistry) publishActiveEvent(
	ctx context.Context,
	userID uint,
	eventType string,
	runID string,
	conversationPublicID string,
) {
	if r == nil || r.store == nil || userID == 0 || (eventType != "started" && eventType != "finished") {
		return
	}
	payload, err := json.Marshal(activeMessageGenerationEventPayload{
		Type:                 eventType,
		RunID:                normalizeRunID(runID),
		ConversationPublicID: normalizePublicID(conversationPublicID),
		UserID:               userID,
	})
	if err != nil {
		return
	}
	_, _ = r.store.AppendActiveGenerationEvent(ctx, repository.GenerationStreamAppend{
		PayloadJSON: string(payload),
	}, activeGenerationEventMaxEvents, activeGenerationEventRetention)
}

func (r *generationStreamRegistry) subscribeActive(
	ctx context.Context,
	userID uint,
) ([]ActiveMessageGeneration, <-chan ActiveMessageGenerationEvent, context.CancelFunc, error) {
	if r == nil || r.store == nil || userID == 0 {
		return []ActiveMessageGeneration{}, nil, func() {}, nil
	}
	if err := r.ensureActiveEventReader(ctx); err != nil {
		return nil, nil, nil, err
	}
	events, cancel, ok := r.addActiveSubscriber(userID)
	if !ok {
		return nil, nil, nil, context.Canceled
	}
	snapshot, err := r.listActive(ctx, userID)
	if err != nil {
		cancel()
		return nil, nil, nil, err
	}
	return snapshot, events, cancel, nil
}

func (r *generationStreamRegistry) ensureActiveEventReader(ctx context.Context) error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return context.Canceled
	}
	if r.activeEventReaderStarted {
		r.mu.Unlock()
		return nil
	}
	r.activeEventReaderWG.Add(1)
	lifecycleCtx := r.lifecycleCtx
	r.mu.Unlock()

	initializationCtx, cancelInitialization := context.WithCancel(ctx)
	stopLifecycleCancellation := context.AfterFunc(lifecycleCtx, cancelInitialization)
	defer func() {
		stopLifecycleCancellation()
		cancelInitialization()
		r.activeEventReaderWG.Done()
	}()

	latest, err := r.store.ListGenerationStreamEvents(initializationCtx, activeGenerationEventStreamID, 1)
	if err != nil {
		return err
	}
	cursor := "0-0"
	if len(latest) > 0 && strings.TrimSpace(latest[len(latest)-1].ID) != "" {
		cursor = latest[len(latest)-1].ID
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return context.Canceled
	}
	if r.activeEventReaderStarted {
		r.mu.Unlock()
		return nil
	}
	r.activeEventReaderStarted = true
	r.activeEventReaderWG.Add(1)
	r.mu.Unlock()
	go func() {
		defer r.activeEventReaderWG.Done()
		r.readActiveEventBus(lifecycleCtx, cursor)
	}()
	return nil
}

func (r *generationStreamRegistry) addActiveSubscriber(
	userID uint,
) (<-chan ActiveMessageGenerationEvent, context.CancelFunc, bool) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, func() {}, false
	}
	r.activeSubscriberSeq++
	subscriberID := r.activeSubscriberSeq
	events := make(chan ActiveMessageGenerationEvent, activeGenerationEventBuffer)
	if r.activeSubscribers[userID] == nil {
		r.activeSubscribers[userID] = make(map[uint64]chan ActiveMessageGenerationEvent)
	}
	r.activeSubscribers[userID][subscriberID] = events
	r.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			r.mu.Lock()
			subscribers := r.activeSubscribers[userID]
			if current, ok := subscribers[subscriberID]; ok {
				delete(subscribers, subscriberID)
				close(current)
			}
			if len(subscribers) == 0 {
				delete(r.activeSubscribers, userID)
			}
			r.mu.Unlock()
		})
	}
	return events, cancel, true
}

func (r *generationStreamRegistry) readActiveEventBus(ctx context.Context, cursor string) {
	retryDelay := time.Second
	for {
		records, err := r.store.ReadGenerationStreamEvents(
			ctx,
			activeGenerationEventStreamID,
			cursor,
			activeGenerationEventReadBlock,
			activeGenerationEventBuffer,
		)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			r.closeActiveSubscribers()
			timer := time.NewTimer(retryDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			if retryDelay < 30*time.Second {
				retryDelay *= 2
			}
			continue
		}
		retryDelay = time.Second
		for _, record := range records {
			if strings.TrimSpace(record.ID) != "" {
				cursor = record.ID
			}
			var payload activeMessageGenerationEventPayload
			if json.Unmarshal([]byte(record.PayloadJSON), &payload) != nil {
				continue
			}
			event := ActiveMessageGenerationEvent(payload)
			event.RunID = normalizeRunID(event.RunID)
			event.ConversationPublicID = normalizePublicID(event.ConversationPublicID)
			if event.UserID == 0 || event.RunID == "" || (event.Type != "started" && event.Type != "finished") {
				continue
			}
			r.fanOutActiveEvent(event)
		}
	}
}

func (r *generationStreamRegistry) fanOutActiveEvent(event ActiveMessageGenerationEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	subscribers := r.activeSubscribers[event.UserID]
	for subscriberID, events := range subscribers {
		select {
		case events <- event:
		default:
			close(events)
			delete(subscribers, subscriberID)
		}
	}
	if len(subscribers) == 0 {
		delete(r.activeSubscribers, event.UserID)
	}
}

func (r *generationStreamRegistry) closeActiveSubscribers() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for userID, subscribers := range r.activeSubscribers {
		for subscriberID, events := range subscribers {
			close(events)
			delete(subscribers, subscriberID)
		}
		delete(r.activeSubscribers, userID)
	}
}
