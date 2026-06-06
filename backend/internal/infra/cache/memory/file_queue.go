package memory

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func (c *Cache) InitFileProcessingStream(ctx context.Context) error {
	return nil
}

func (c *Cache) EnqueueFileProcessing(ctx context.Context, userID uint, fileID string, retry int, lastError string) error {
	if c == nil || userID == 0 || strings.TrimSpace(fileID) == "" {
		return nil
	}
	c.mu.Lock()
	now := time.Now()
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
	return nil, nil
}

func (c *Cache) ReadFileProcessingMessages(ctx context.Context, consumerName string) ([]repository.FileProcessingMessage, error) {
	if c == nil {
		return nil, nil
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		c.mu.Lock()
		if len(c.fileQueue) > 0 {
			msg := c.fileQueue[0]
			c.fileQueue = c.fileQueue[1:]
			c.fileInflight[msg.ID] = msg
			c.mu.Unlock()
			return []repository.FileProcessingMessage{msg}, nil
		}
		notify := c.fileNotify
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, nil
		case <-timer.C:
			return nil, nil
		case <-notify:
		}
	}
}

func (c *Cache) AckFileProcessingMessage(ctx context.Context, messageID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.fileInflight, strings.TrimSpace(messageID))
	c.maybeSweepLocked(time.Now())
	return nil
}

func (c *Cache) DeleteFileProcessingMessage(ctx context.Context, messageID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.fileInflight, strings.TrimSpace(messageID))
	c.maybeSweepLocked(time.Now())
	return nil
}

func (c *Cache) SendFileProcessingToDLQ(ctx context.Context, userID uint, fileID string, retry int, lastError string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fileSeq++
	c.fileDLQ = append(c.fileDLQ, repository.FileProcessingMessage{
		ID:        "dlq-" + strconv.FormatInt(c.fileSeq, 10),
		UserID:    userID,
		FileID:    strings.TrimSpace(fileID),
		Retry:     retry,
		LastError: strings.TrimSpace(lastError),
	})
	c.maybeSweepLocked(time.Now())
	return nil
}

func (c *Cache) notifyFileQueueLocked() {
	close(c.fileNotify)
	c.fileNotify = make(chan struct{})
}
