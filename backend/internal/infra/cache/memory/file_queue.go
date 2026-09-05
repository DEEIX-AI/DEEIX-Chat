package memory

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	fileProcessingMinIdle = 45 * time.Second
	maxFileQueueErrorSize = 255
	// maxFileQueueLength 限制内存主队列长度，避免积压时内存无界增长；
	// 达到上限后新任务入队返回 ErrFileProcessingQueueFull，
	// 由 processing.InitializeUploadedFile 将文件标为 failed，避免永远停在 queued。
	maxFileQueueLength = 10_000
)

type fileProcessingLease struct {
	consumerName string
	leasedAt     time.Time
	message      repository.FileProcessingMessage
}

type fileQueueState struct {
	queue    []repository.FileProcessingMessage
	inflight map[string]fileProcessingLease
	dlq      []repository.FileProcessingMessage
	notify   chan struct{}
}

func newFileQueueState() fileQueueState {
	return fileQueueState{
		inflight: map[string]fileProcessingLease{},
		notify:   make(chan struct{}),
	}
}

// InitFileProcessingStream initializes the file-processing queue backend.
func (c *Cache) InitFileProcessingStream(ctx context.Context) error {
	return ctx.Err()
}

// EnqueueFileProcessing queues a file for extraction and processing.
func (c *Cache) EnqueueFileProcessing(ctx context.Context, userID uint, fileID string, retry int, lastError string) error {
	return c.enqueueFileMessage(ctx, repository.FileProcessingMessage{
		UserID:    userID,
		FileID:    fileID,
		Retry:     retry,
		LastError: lastError,
		Queue:     repository.FileProcessingQueueDefault,
	})
}

// EnqueueFileEmbedding queues a file for embedding with the requested service configuration.
func (c *Cache) EnqueueFileEmbedding(
	ctx context.Context,
	userID uint,
	fileID string,
	embeddingSignature string,
	embeddingHost string,
) error {
	fileID = strings.TrimSpace(fileID)
	embeddingSignature = strings.TrimSpace(embeddingSignature)
	embeddingHost = strings.TrimRight(strings.TrimSpace(embeddingHost), "/")
	if fileID == "" || embeddingSignature == "" || embeddingHost == "" {
		return repository.ErrInvalidInput
	}
	return c.enqueueFileMessage(ctx, repository.FileProcessingMessage{
		UserID:             userID,
		FileID:             fileID,
		Kind:               repository.FileProcessingKindEmbedding,
		Queue:              repository.FileProcessingQueueEmbedding,
		EmbeddingSignature: embeddingSignature,
		EmbeddingHost:      embeddingHost,
	})
}

func (c *Cache) enqueueFileMessage(ctx context.Context, message repository.FileProcessingMessage) error {
	if c == nil || strings.TrimSpace(message.FileID) == "" {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	now := time.Now()
	state := c.fileQueueState(message.Queue)
	if len(state.queue) >= maxFileQueueLength {
		c.maybeSweepLocked(now)
		c.mu.Unlock()
		return repository.ErrFileProcessingQueueFull
	}
	c.fileSeq++
	message.ID = strconv.FormatInt(c.fileSeq, 10)
	message.FileID = strings.TrimSpace(message.FileID)
	message.LastError = truncateFileQueueError(message.LastError)
	msg := message
	state.queue = append(state.queue, msg)
	notifyFileQueueLocked(state)
	c.maybeSweepLocked(now)
	c.mu.Unlock()
	return nil
}

// ClaimTimedOutFileProcessingMessages reclaims one timed-out extraction lease.
func (c *Cache) ClaimTimedOutFileProcessingMessages(ctx context.Context, consumerName string) ([]repository.FileProcessingMessage, error) {
	return c.claimTimedOutFileMessages(ctx, consumerName, repository.FileProcessingQueueDefault)
}

// ClaimTimedOutFileEmbeddingMessages reclaims one timed-out embedding lease.
func (c *Cache) ClaimTimedOutFileEmbeddingMessages(ctx context.Context, consumerName string) ([]repository.FileProcessingMessage, error) {
	return c.claimTimedOutFileMessages(ctx, consumerName, repository.FileProcessingQueueEmbedding)
}

func (c *Cache) claimTimedOutFileMessages(ctx context.Context, consumerName string, queue repository.FileProcessingQueue) ([]repository.FileProcessingMessage, error) {
	if c == nil || strings.TrimSpace(consumerName) == "" {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.fileQueueState(queue)
	now := time.Now()
	var candidateID string
	var candidate fileProcessingLease
	for messageID, lease := range state.inflight {
		if now.Sub(lease.leasedAt) < fileProcessingMinIdle {
			continue
		}
		if candidateID == "" || lease.leasedAt.Before(candidate.leasedAt) {
			candidateID = messageID
			candidate = lease
		}
	}
	if candidateID == "" {
		return nil, nil
	}
	candidate.consumerName = strings.TrimSpace(consumerName)
	candidate.leasedAt = now
	candidate.message.Reclaimed = true
	state.inflight[candidateID] = candidate
	return []repository.FileProcessingMessage{candidate.message}, nil
}

// ReadFileProcessingMessages reads one extraction message and leases it to a consumer.
func (c *Cache) ReadFileProcessingMessages(ctx context.Context, consumerName string) ([]repository.FileProcessingMessage, error) {
	return c.readFileMessages(ctx, consumerName, repository.FileProcessingQueueDefault)
}

// ReadFileEmbeddingMessages reads one embedding message and leases it to a consumer.
func (c *Cache) ReadFileEmbeddingMessages(ctx context.Context, consumerName string) ([]repository.FileProcessingMessage, error) {
	return c.readFileMessages(ctx, consumerName, repository.FileProcessingQueueEmbedding)
}

func (c *Cache) readFileMessages(ctx context.Context, consumerName string, queue repository.FileProcessingQueue) ([]repository.FileProcessingMessage, error) {
	if c == nil || strings.TrimSpace(consumerName) == "" {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.mu.Lock()
		state := c.fileQueueState(queue)
		if len(state.queue) > 0 {
			msg := state.queue[0]
			state.queue = state.queue[1:]
			state.inflight[msg.ID] = fileProcessingLease{
				consumerName: strings.TrimSpace(consumerName),
				leasedAt:     time.Now(),
				message:      msg,
			}
			c.mu.Unlock()
			return []repository.FileProcessingMessage{msg}, nil
		}
		notify := state.notify
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, nil
		case <-notify:
		}
	}
}

// RenewFileProcessingMessageLease extends a consumer's lease for a file message.
func (c *Cache) RenewFileProcessingMessageLease(ctx context.Context, consumerName string, message repository.FileProcessingMessage) (bool, error) {
	if c == nil {
		return false, nil
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.fileQueueState(queueForMessage(message))
	messageID := strings.TrimSpace(message.ID)
	lease, exists := state.inflight[messageID]
	if !exists || lease.consumerName != strings.TrimSpace(consumerName) {
		return false, nil
	}
	lease.leasedAt = time.Now()
	state.inflight[messageID] = lease
	return true, nil
}

// SettleFileProcessingMessage acknowledges and removes a leased file message.
func (c *Cache) SettleFileProcessingMessage(ctx context.Context, consumerName string, message repository.FileProcessingMessage) (bool, error) {
	if c == nil {
		return false, nil
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.fileQueueState(queueForMessage(message))
	messageID := strings.TrimSpace(message.ID)
	lease, exists := state.inflight[messageID]
	if !exists || lease.consumerName != strings.TrimSpace(consumerName) {
		return false, nil
	}
	delete(state.inflight, messageID)
	c.maybeSweepLocked(time.Now())
	return true, nil
}

// RequeueFileProcessingMessage returns a leased file message to its queue.
func (c *Cache) RequeueFileProcessingMessage(
	ctx context.Context,
	consumerName string,
	message repository.FileProcessingMessage,
	retry int,
	lastError string,
) (bool, error) {
	if c == nil {
		return false, nil
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.fileQueueState(queueForMessage(message))
	messageID := strings.TrimSpace(message.ID)
	lease, exists := state.inflight[messageID]
	if !exists || lease.consumerName != strings.TrimSpace(consumerName) {
		return false, nil
	}
	// 重入队不受 maxFileQueueLength 限制：消息只是从 inflight 移回队列，总量无净增长。
	c.fileSeq++
	state.queue = append(state.queue, repository.FileProcessingMessage{
		ID:                 strconv.FormatInt(c.fileSeq, 10),
		UserID:             message.UserID,
		FileID:             strings.TrimSpace(message.FileID),
		Retry:              retry,
		LastError:          truncateFileQueueError(lastError),
		Kind:               message.Kind,
		Queue:              queueForMessage(message),
		EmbeddingSignature: message.EmbeddingSignature,
		EmbeddingHost:      message.EmbeddingHost,
	})
	delete(state.inflight, messageID)
	notifyFileQueueLocked(state)
	c.maybeSweepLocked(time.Now())
	return true, nil
}

// DeadLetterFileProcessingMessage moves a leased file message to the dead-letter queue.
func (c *Cache) DeadLetterFileProcessingMessage(
	ctx context.Context,
	consumerName string,
	message repository.FileProcessingMessage,
	lastError string,
) (bool, error) {
	if c == nil {
		return false, nil
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.fileQueueState(queueForMessage(message))
	messageID := strings.TrimSpace(message.ID)
	lease, exists := state.inflight[messageID]
	if !exists || lease.consumerName != strings.TrimSpace(consumerName) {
		return false, nil
	}
	c.fileSeq++
	state.dlq = append(state.dlq, repository.FileProcessingMessage{
		ID:                 "dlq-" + strconv.FormatInt(c.fileSeq, 10),
		UserID:             message.UserID,
		FileID:             strings.TrimSpace(message.FileID),
		Retry:              message.Retry,
		LastError:          truncateFileQueueError(lastError),
		Kind:               message.Kind,
		Queue:              queueForMessage(message),
		EmbeddingSignature: message.EmbeddingSignature,
		EmbeddingHost:      message.EmbeddingHost,
	})
	if len(state.dlq) > 10_000 {
		state.dlq = append([]repository.FileProcessingMessage(nil), state.dlq[len(state.dlq)-10_000:]...)
	}
	delete(state.inflight, messageID)
	c.maybeSweepLocked(time.Now())
	return true, nil
}

func (c *Cache) fileQueueState(queue repository.FileProcessingQueue) *fileQueueState {
	if queue == repository.FileProcessingQueueEmbedding {
		return &c.fileEmbeddingQueue
	}
	return &c.fileProcessingQueue
}

func queueForMessage(message repository.FileProcessingMessage) repository.FileProcessingQueue {
	if message.Queue != "" {
		return message.Queue
	}
	if message.Kind == repository.FileProcessingKindEmbedding {
		return repository.FileProcessingQueueEmbedding
	}
	return repository.FileProcessingQueueDefault
}

func notifyFileQueueLocked(state *fileQueueState) {
	close(state.notify)
	state.notify = make(chan struct{})
}

func truncateFileQueueError(message string) string {
	value := strings.TrimSpace(message)
	runes := []rune(value)
	if len(runes) > maxFileQueueErrorSize {
		return string(runes[:maxFileQueueErrorSize])
	}
	return value
}
