package memory

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type generationStream struct {
	ownerID         uint
	ownerExpiresAt  time.Time
	activeExpiresAt time.Time
	cancelExpiresAt time.Time
	eventsExpiresAt time.Time
	seq             int64
	events          []repository.GenerationStreamMessage
	notify          chan struct{}
}

func (c *Cache) RegisterGenerationStream(ctx context.Context, runID string, userID uint, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.ensureStreamLocked(runID)
	stream.ownerID = userID
	stream.ownerExpiresAt = ttlFromNow(ttl)
	stream.cancelExpiresAt = time.Time{}
	c.maybeSweepLocked(time.Now())
	return nil
}

func (c *Cache) GetGenerationStreamOwner(ctx context.Context, runID string) (uint, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(runID)]
	if stream == nil || stream.ownerID == 0 || expired(stream.ownerExpiresAt) {
		return 0, false, nil
	}
	return stream.ownerID, true, nil
}

func (c *Cache) TouchGenerationStreamActive(ctx context.Context, runID string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.ensureStreamLocked(runID)
	stream.activeExpiresAt = ttlFromNow(ttl)
	c.maybeSweepLocked(time.Now())
	return nil
}

func (c *Cache) ClearGenerationStreamActive(ctx context.Context, runID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if stream := c.streams[strings.TrimSpace(runID)]; stream != nil {
		stream.activeExpiresAt = time.Time{}
	}
	return nil
}

func (c *Cache) IsGenerationStreamActive(ctx context.Context, runID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(runID)]
	return stream != nil && !expired(stream.activeExpiresAt), nil
}

func (c *Cache) RequestGenerationStreamCancel(ctx context.Context, runID string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.ensureStreamLocked(runID)
	stream.cancelExpiresAt = ttlFromNow(ttl)
	stream.notifyLocked()
	c.maybeSweepLocked(time.Now())
	return nil
}

func (c *Cache) IsGenerationStreamCanceled(ctx context.Context, runID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(runID)]
	return stream != nil && !expired(stream.cancelExpiresAt), nil
}

func (c *Cache) AppendGenerationStreamEvent(ctx context.Context, runID string, payloadJSON string, maxEvents int64, ttl time.Duration) (repository.GenerationStreamMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.ensureStreamLocked(runID)
	stream.seq++
	record := repository.GenerationStreamMessage{
		ID:          strconv.FormatInt(stream.seq, 10),
		Seq:         stream.seq,
		PayloadJSON: payloadJSON,
	}
	stream.events = append(stream.events, record)
	if maxEvents <= 0 {
		maxEvents = 1024
	}
	if excess := len(stream.events) - int(maxEvents); excess > 0 {
		stream.events = stream.events[excess:]
	}
	stream.eventsExpiresAt = ttlFromNow(ttl)
	stream.notifyLocked()
	c.maybeSweepLocked(time.Now())
	return record, nil
}

func (c *Cache) ListGenerationStreamEvents(ctx context.Context, runID string, limit int64) ([]repository.GenerationStreamMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(runID)]
	if stream == nil || expired(stream.eventsExpiresAt) {
		return nil, nil
	}
	if limit <= 0 || int(limit) >= len(stream.events) {
		return append([]repository.GenerationStreamMessage(nil), stream.events...), nil
	}
	return append([]repository.GenerationStreamMessage(nil), stream.events[len(stream.events)-int(limit):]...), nil
}

func (c *Cache) ReadGenerationStreamEvents(ctx context.Context, runID string, afterID string, block time.Duration, limit int64) ([]repository.GenerationStreamMessage, error) {
	if c == nil {
		return nil, nil
	}
	if block <= 0 {
		block = 5 * time.Second
	}
	deadline := time.Now().Add(block)
	afterSeq := parseStreamID(afterID)
	for {
		c.mu.Lock()
		stream := c.streams[strings.TrimSpace(runID)]
		if stream == nil || expired(stream.eventsExpiresAt) {
			c.mu.Unlock()
			return nil, nil
		}
		records := generationEventsAfter(stream.events, afterSeq, limit)
		if len(records) > 0 {
			c.mu.Unlock()
			return records, nil
		}
		notify := stream.notify
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, nil
		case <-time.After(time.Until(deadline)):
			return nil, nil
		case <-notify:
		}
	}
}

func (c *Cache) ExpireGenerationStream(ctx context.Context, runID string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if stream := c.streams[strings.TrimSpace(runID)]; stream != nil {
		stream.eventsExpiresAt = ttlFromNow(ttl)
		stream.ownerExpiresAt = ttlFromNow(ttl)
		if !stream.cancelExpiresAt.IsZero() && !expired(stream.cancelExpiresAt) {
			stream.cancelExpiresAt = ttlFromNow(ttl)
		}
		stream.notifyLocked()
		c.maybeSweepLocked(time.Now())
	}
	return nil
}

func (c *Cache) ensureStreamLocked(runID string) *generationStream {
	runID = strings.TrimSpace(runID)
	stream := c.streams[runID]
	if stream == nil {
		stream = &generationStream{notify: make(chan struct{})}
		c.streams[runID] = stream
	}
	return stream
}

func (s *generationStream) notifyLocked() {
	close(s.notify)
	s.notify = make(chan struct{})
}

func (s *generationStream) ownerExpired(now time.Time) bool {
	return s.ownerExpiresAt.IsZero() || now.After(s.ownerExpiresAt)
}

func (s *generationStream) activeExpired(now time.Time) bool {
	return s.activeExpiresAt.IsZero() || now.After(s.activeExpiresAt)
}

func (s *generationStream) cancelExpired(now time.Time) bool {
	return s.cancelExpiresAt.IsZero() || now.After(s.cancelExpiresAt)
}

func (s *generationStream) eventsExpired(now time.Time) bool {
	return s.eventsExpiresAt.IsZero() || now.After(s.eventsExpiresAt)
}

func generationEventsAfter(events []repository.GenerationStreamMessage, afterSeq int64, limit int64) []repository.GenerationStreamMessage {
	results := make([]repository.GenerationStreamMessage, 0)
	for _, item := range events {
		if item.Seq <= afterSeq {
			continue
		}
		results = append(results, item)
		if limit > 0 && int64(len(results)) >= limit {
			break
		}
	}
	return results
}

func parseStreamID(raw string) int64 {
	value := strings.TrimSpace(raw)
	if value == "" || value == "0-0" {
		return 0
	}
	if idx := strings.Index(value, "-"); idx >= 0 {
		value = value[:idx]
	}
	n, _ := strconv.ParseInt(value, 10, 64)
	return n
}
