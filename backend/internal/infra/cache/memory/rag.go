package memory

import (
	"context"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func (c *Cache) GetRAGCache(ctx context.Context, key string) ([]domainconversation.RAGChunk, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.rag[key]
	if !ok || expired(item.expiresAt) {
		delete(c.rag, key)
		return nil, false
	}
	return append([]domainconversation.RAGChunk(nil), item.chunks...), true
}

func (c *Cache) SetRAGCache(ctx context.Context, key string, chunks []domainconversation.RAGChunk, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.rag[key] = expiringRAG{chunks: append([]domainconversation.RAGChunk(nil), chunks...), expiresAt: ttlFromNow(ttl)}
	c.maybeSweepLocked(now)
}
