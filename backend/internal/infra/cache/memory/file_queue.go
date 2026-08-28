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

func (c *Cache) InitFileProcessingStream(ctx context.Context) error {
	return ctx.Err()
}

func (c *Cache) EnqueueFileProcessing(ctx context.Context, userID uint, fileID string, retry int, lastError string) error {
	if c == nil || strings.TrimSpace(fileID) == "" {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	now := time.Now()
	if len(c.fileQueue) >= maxFileQueueLength {
		c.maybeSweepLocked(now)
		c.mu.Unlock()
		return repository.ErrFileProcessingQueueFull
	}
	c.fileSeq++
	msg := repository.FileProcessingMessage{
		ID:        strconv.FormatInt(c.fileSeq, 10),
		UserID:    userID,
		FileID:    strings.TrimSpace(fileID),
		Retry:     retry,
		LastError: strings.TrimSpace(lastError),
	}
	c.fileQueue = append(c.fileQueue, msg)
	c.notifyFileQueueLocked()
	c.maybeSweepLocked(now)
	c.mu.Unlock()
	return nil
}

func (c *Cache) ClaimTimedOutFileProcessingMessages(ctx context.Context, consumerName string) ([]repository.FileProcessingMessage, error) {
	if c == nil || strings.TrimSpace(consumerName) == "" {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	var candidateID string
	var candidate fileProcessingLease
	for messageID, lease := range c.fileInflight {
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
	c.fileInflight[candidateID] = candidate
	return []repository.FileProcessingMessage{candidate.message}, nil
}

func (c *Cache) ReadFileProcessingMessages(ctx context.Context, consumerName string) ([]repository.FileProcessingMessage, error) {
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
		if len(c.fileQueue) > 0 {
			msg := c.fileQueue[0]
			c.fileQueue = c.fileQueue[1:]
			c.fileInflight[msg.ID] = fileProcessingLease{
				consumerName: strings.TrimSpace(consumerName),
				leasedAt:     time.Now(),
				message:      msg,
			}
			c.mu.Unlock()
			return []repository.FileProcessingMessage{msg}, nil
		}
		notify := c.fileNotify
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

func (c *Cache) RenewFileProcessingMessageLease(ctx context.Context, consumerName, messageID string) (bool, error) {
	if c == nil {
		return false, nil
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	messageID = strings.TrimSpace(messageID)
	lease, exists := c.fileInflight[messageID]
	if !exists || lease.consumerName != strings.TrimSpace(consumerName) {
		return false, nil
	}
	lease.leasedAt = time.Now()
	c.fileInflight[messageID] = lease
	return true, nil
}

func (c *Cache) SettleFileProcessingMessage(ctx context.Context, consumerName, messageID string) (bool, error) {
	if c == nil {
		return false, nil
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	messageID = strings.TrimSpace(messageID)
	lease, exists := c.fileInflight[messageID]
	if !exists || lease.consumerName != strings.TrimSpace(consumerName) {
		return false, nil
	}
	delete(c.fileInflight, messageID)
	c.maybeSweepLocked(time.Now())
	return true, nil
}

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
	messageID := strings.TrimSpace(message.ID)
	lease, exists := c.fileInflight[messageID]
	if !exists || lease.consumerName != strings.TrimSpace(consumerName) {
		return false, nil
	}
	// 重入队不受 maxFileQueueLength 限制：消息只是从 inflight 移回队列，总量无净增长。
	c.fileSeq++
	c.fileQueue = append(c.fileQueue, repository.FileProcessingMessage{
		ID:        strconv.FormatInt(c.fileSeq, 10),
		UserID:    message.UserID,
		FileID:    strings.TrimSpace(message.FileID),
		Retry:     retry,
		LastError: strings.TrimSpace(lastError),
	})
	delete(c.fileInflight, messageID)
	c.notifyFileQueueLocked()
	c.maybeSweepLocked(time.Now())
	return true, nil
}

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
	messageID := strings.TrimSpace(message.ID)
	lease, exists := c.fileInflight[messageID]
	if !exists || lease.consumerName != strings.TrimSpace(consumerName) {
		return false, nil
	}
	c.fileSeq++
	c.fileDLQ = append(c.fileDLQ, repository.FileProcessingMessage{
		ID:        "dlq-" + strconv.FormatInt(c.fileSeq, 10),
		UserID:    message.UserID,
		FileID:    strings.TrimSpace(message.FileID),
		Retry:     message.Retry,
		LastError: strings.TrimSpace(lastError),
	})
	if len(c.fileDLQ) > 10_000 {
		c.fileDLQ = append([]repository.FileProcessingMessage(nil), c.fileDLQ[len(c.fileDLQ)-10_000:]...)
	}
	delete(c.fileInflight, messageID)
	c.maybeSweepLocked(time.Now())
	return true, nil
}

func (c *Cache) notifyFileQueueLocked() {
	close(c.fileNotify)
	c.fileNotify = make(chan struct{})
}
