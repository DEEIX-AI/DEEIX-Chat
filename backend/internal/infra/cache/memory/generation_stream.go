package memory

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type generationStream struct {
	ownerID               uint
	executionID           string
	conversationID        string
	ownerExpiresAt        time.Time
	activeExpiresAt       time.Time
	cancelExpiresAt       time.Time
	eventsExpiresAt       time.Time
	seq                   int64
	events                []repository.GenerationStreamMessage
	textContent           strings.Builder
	textSeq               int64
	upstreamThinkContent  strings.Builder
	upstreamThinkSeq      int64
	upstreamThinkRoundID  string
	upstreamThinkMetadata string
	notify                chan struct{}
}

// ClaimGenerationStream 认领生成流的所有权并续期其租约。
func (c *Cache) ClaimGenerationStream(
	_ context.Context,
	lease repository.GenerationStreamLease,
	leaseTTL time.Duration,
	ownershipTTL time.Duration,
) (bool, error) {
	lease.RunID = strings.TrimSpace(lease.RunID)
	lease.ExecutionID = strings.TrimSpace(lease.ExecutionID)
	lease.ConversationPublicID = strings.TrimSpace(lease.ConversationPublicID)
	if lease.RunID == "" || lease.ExecutionID == "" || lease.UserID == 0 || lease.ConversationPublicID == "" {
		return false, nil
	}
	if ownershipTTL < leaseTTL {
		ownershipTTL = leaseTTL
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	stream := c.streams[lease.RunID]
	if stream != nil {
		if !stream.activeExpired(now) &&
			stream.executionID == lease.ExecutionID &&
			stream.ownerID == lease.UserID &&
			stream.conversationID == lease.ConversationPublicID {
			stream.ownerExpiresAt = ttlFromNow(ownershipTTL)
			stream.activeExpiresAt = ttlFromNow(leaseTTL)
			c.notifyGenerationStreamsLocked()
			return true, nil
		}
		if !stream.activeExpired(now) || !stream.ownerExpired(now) {
			return false, nil
		}
		stream.resetEventsLocked()
	} else {
		stream = c.ensureStreamLocked(lease.RunID)
	}
	stream.ownerID = lease.UserID
	stream.executionID = lease.ExecutionID
	stream.conversationID = lease.ConversationPublicID
	stream.ownerExpiresAt = ttlFromNow(ownershipTTL)
	stream.activeExpiresAt = ttlFromNow(leaseTTL)
	stream.cancelExpiresAt = time.Time{}
	stream.notifyLocked()
	c.notifyGenerationStreamsLocked()
	c.maybeSweepLocked(now)
	return true, nil
}

// GetGenerationStreamOwner 返回生成流的当前活跃所有者。
func (c *Cache) GetGenerationStreamOwner(ctx context.Context, runID string) (uint, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(runID)]
	now := time.Now()
	if stream == nil || stream.ownerID == 0 || stream.ownerExpired(now) {
		return 0, false, nil
	}
	return stream.ownerID, true, nil
}

// RenewGenerationStreamLease 延长流的活跃租约与所有权租约。
func (c *Cache) RenewGenerationStreamLease(
	_ context.Context,
	lease repository.GenerationStreamLease,
	leaseTTL time.Duration,
	ownershipTTL time.Duration,
) (bool, error) {
	if ownershipTTL < leaseTTL {
		ownershipTTL = leaseTTL
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(lease.RunID)]
	now := time.Now()
	if stream == nil ||
		stream.executionID != strings.TrimSpace(lease.ExecutionID) ||
		stream.ownerID != lease.UserID ||
		stream.conversationID != strings.TrimSpace(lease.ConversationPublicID) ||
		stream.activeExpired(now) {
		return false, nil
	}
	stream.activeExpiresAt = ttlFromNow(leaseTTL)
	stream.ownerExpiresAt = ttlFromNow(ownershipTTL)
	c.maybeSweepLocked(now)
	return true, nil
}

// CompleteGenerationStream 将流标记为完成，并临时保留其事件。
func (c *Cache) CompleteGenerationStream(_ context.Context, lease repository.GenerationStreamLease, retention time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(lease.RunID)]
	now := time.Now()
	if stream == nil ||
		stream.executionID != strings.TrimSpace(lease.ExecutionID) ||
		stream.ownerID != lease.UserID ||
		stream.conversationID != strings.TrimSpace(lease.ConversationPublicID) ||
		stream.ownerExpired(now) {
		return false, nil
	}
	stream.executionID = ""
	stream.activeExpiresAt = time.Time{}
	stream.ownerExpiresAt = ttlFromNow(retention)
	stream.eventsExpiresAt = ttlFromNow(retention)
	if !stream.cancelExpired(now) {
		stream.cancelExpiresAt = ttlFromNow(retention)
	}
	stream.notifyLocked()
	c.notifyGenerationStreamsLocked()
	c.maybeSweepLocked(now)
	return true, nil
}

// AbandonGenerationStream 移除活跃流，不保留其事件。
func (c *Cache) AbandonGenerationStream(_ context.Context, lease repository.GenerationStreamLease) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	lease.RunID = strings.TrimSpace(lease.RunID)
	stream := c.streams[lease.RunID]
	if stream == nil ||
		stream.executionID != strings.TrimSpace(lease.ExecutionID) ||
		stream.ownerID != lease.UserID ||
		stream.activeExpired(time.Now()) {
		return false, nil
	}
	stream.notifyLocked()
	delete(c.streams, lease.RunID)
	c.notifyGenerationStreamsLocked()
	return true, nil
}

// ListActiveGenerationStreams 列出用户拥有的活跃流。
func (c *Cache) ListActiveGenerationStreams(ctx context.Context, userID uint) ([]repository.ActiveGenerationStream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.maybeSweepLocked(now)
	items := make([]repository.ActiveGenerationStream, 0)
	for runID, stream := range c.streams {
		if stream.ownerID != userID || stream.activeExpired(now) || strings.TrimSpace(stream.conversationID) == "" {
			continue
		}
		items = append(items, repository.ActiveGenerationStream{
			RunID:                runID,
			ConversationPublicID: stream.conversationID,
		})
	}
	return items, nil
}

// IsGenerationStreamActive 报告生成流是否处于活跃状态。
func (c *Cache) IsGenerationStreamActive(ctx context.Context, runID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(runID)]
	now := time.Now()
	return stream != nil && !stream.activeExpired(now), nil
}

// RequestGenerationStreamCancel 记录用户对某个流的取消请求。
func (c *Cache) RequestGenerationStreamCancel(_ context.Context, runID string, userID uint, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(runID)]
	if stream == nil || stream.ownerID != userID || stream.activeExpired(time.Now()) {
		return false, nil
	}
	stream.cancelExpiresAt = ttlFromNow(ttl)
	stream.notifyLocked()
	c.maybeSweepLocked(time.Now())
	return true, nil
}

// IsGenerationStreamCanceled 报告流是否存在未过期的取消请求。
func (c *Cache) IsGenerationStreamCanceled(ctx context.Context, runID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(runID)]
	now := time.Now()
	return stream != nil && !stream.cancelExpired(now), nil
}

// AppendGenerationStreamEvent 向已拥有的生成流追加事件。
func (c *Cache) AppendGenerationStreamEvent(
	_ context.Context,
	lease repository.GenerationStreamLease,
	input repository.GenerationStreamAppend,
	maxEvents int64,
	ttl time.Duration,
) (repository.GenerationStreamMessage, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(lease.RunID)]
	now := time.Now()
	if stream == nil || stream.executionID != strings.TrimSpace(lease.ExecutionID) || stream.activeExpired(now) {
		return repository.GenerationStreamMessage{}, false, nil
	}
	record := appendGenerationStreamEventLocked(stream, input, maxEvents, ttl)
	c.notifyGenerationStreamsLocked()
	c.maybeSweepLocked(now)
	return record, true, nil
}

// AppendActiveGenerationEvent 向共享的活跃流 feed 追加事件。
func (c *Cache) AppendActiveGenerationEvent(
	_ context.Context,
	input repository.GenerationStreamAppend,
	maxEvents int64,
	ttl time.Duration,
) (repository.GenerationStreamMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.ensureStreamLocked("active_events_v1")
	record := appendGenerationStreamEventLocked(stream, input, maxEvents, ttl)
	c.notifyGenerationStreamsLocked()
	c.maybeSweepLocked(time.Now())
	return record, nil
}

func appendGenerationStreamEventLocked(
	stream *generationStream,
	input repository.GenerationStreamAppend,
	maxEvents int64,
	ttl time.Duration,
) repository.GenerationStreamMessage {
	stream.seq++
	record := repository.GenerationStreamMessage{
		ID:          strconv.FormatInt(stream.seq, 10),
		Seq:         stream.seq,
		PayloadJSON: input.PayloadJSON,
	}
	stream.events = append(stream.events, record)
	if input.TextDelta != "" {
		_, _ = stream.textContent.WriteString(input.TextDelta)
		stream.textSeq = stream.seq
	}
	if input.UpstreamThink != nil {
		roundID := strings.TrimSpace(input.UpstreamThink.RoundID)
		if roundID == "" {
			roundID = stream.upstreamThinkRoundID
		}
		if roundID != "" && stream.upstreamThinkRoundID != "" && roundID != stream.upstreamThinkRoundID {
			stream.upstreamThinkContent.Reset()
		}
		if input.UpstreamThink.Replace {
			stream.upstreamThinkContent.Reset()
			_, _ = stream.upstreamThinkContent.WriteString(input.UpstreamThink.ContentMarkdown)
		} else {
			_, _ = stream.upstreamThinkContent.WriteString(input.UpstreamThink.Delta)
		}
		stream.upstreamThinkSeq = stream.seq
		stream.upstreamThinkRoundID = roundID
		stream.upstreamThinkMetadata = input.UpstreamThink.MetadataJSON
	}
	if maxEvents <= 0 {
		maxEvents = 1024
	}
	if excess := len(stream.events) - int(maxEvents); excess > 0 {
		stream.events = stream.events[excess:]
	}
	stream.eventsExpiresAt = ttlFromNow(ttl)
	stream.notifyLocked()
	return record
}

// GetGenerationStreamUpstreamThinkSnapshot 返回最新的上游思考快照。
func (c *Cache) GetGenerationStreamUpstreamThinkSnapshot(ctx context.Context, runID string) (repository.GenerationStreamUpstreamThinkSnapshot, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(runID)]
	now := time.Now()
	if stream == nil || stream.eventsExpired(now) || stream.upstreamThinkSeq <= 0 {
		return repository.GenerationStreamUpstreamThinkSnapshot{}, false, nil
	}
	return repository.GenerationStreamUpstreamThinkSnapshot{
		Seq:             stream.upstreamThinkSeq,
		RoundID:         stream.upstreamThinkRoundID,
		ContentMarkdown: stream.upstreamThinkContent.String(),
		MetadataJSON:    stream.upstreamThinkMetadata,
	}, true, nil
}

// GetGenerationStreamTextSnapshot 返回最新的累计文本快照。
func (c *Cache) GetGenerationStreamTextSnapshot(ctx context.Context, runID string) (repository.GenerationStreamTextSnapshot, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(runID)]
	now := time.Now()
	if stream == nil || stream.eventsExpired(now) || stream.textSeq <= 0 {
		return repository.GenerationStreamTextSnapshot{}, false, nil
	}
	return repository.GenerationStreamTextSnapshot{
		Seq:     stream.textSeq,
		Content: stream.textContent.String(),
	}, true, nil
}

// ListGenerationStreamEvents 返回生成流保留的事件。
func (c *Cache) ListGenerationStreamEvents(ctx context.Context, runID string, limit int64) ([]repository.GenerationStreamMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(runID)]
	now := time.Now()
	if stream == nil || stream.eventsExpired(now) {
		return nil, nil
	}
	if limit <= 0 || int(limit) >= len(stream.events) {
		return append([]repository.GenerationStreamMessage(nil), stream.events...), nil
	}
	return append([]repository.GenerationStreamMessage(nil), stream.events[len(stream.events)-int(limit):]...), nil
}

// ReadGenerationStreamEvents 等待并返回流游标之后的事件。
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
		now := time.Now()
		if stream == nil || stream.eventsExpired(now) {
			notify := c.streamNotify
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, nil
			case <-time.After(time.Until(deadline)):
				return nil, nil
			case <-notify:
				continue
			}
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

func (c *Cache) notifyGenerationStreamsLocked() {
	close(c.streamNotify)
	c.streamNotify = make(chan struct{})
}

// ResetGenerationStreamEvents 清除保留的事件，同时保留流的所有权。
func (c *Cache) ResetGenerationStreamEvents(_ context.Context, lease repository.GenerationStreamLease) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stream := c.streams[strings.TrimSpace(lease.RunID)]
	if stream == nil || stream.executionID != strings.TrimSpace(lease.ExecutionID) || stream.activeExpired(time.Now()) {
		return false, nil
	}
	stream.resetEventsLocked()
	// 保持 seq 单调递增，使进行中的 afterSeq 游标仍然有效。
	stream.notifyLocked()
	c.notifyGenerationStreamsLocked()
	return true, nil
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

func (s *generationStream) resetEventsLocked() {
	s.events = nil
	s.textContent.Reset()
	s.textSeq = 0
	s.upstreamThinkContent.Reset()
	s.upstreamThinkSeq = 0
	s.upstreamThinkRoundID = ""
	s.upstreamThinkMetadata = ""
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
