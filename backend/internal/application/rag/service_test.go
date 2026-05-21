package rag

import (
	"context"
	"testing"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

type testRAGCache struct {
	setCalls int
}

func (c *testRAGCache) GetRAGCache(ctx context.Context, key string) ([]domainconversation.RAGChunk, bool) {
	return nil, false
}

func (c *testRAGCache) SetRAGCache(ctx context.Context, key string, chunks []domainconversation.RAGChunk, ttl time.Duration) {
	c.setCalls++
}

func TestRetrieveWithStatusReportsUnavailable(t *testing.T) {
	svc := NewServiceWithRuntime(config.NewRuntime(config.Config{}), nil, nil, nil)

	result, err := svc.RetrieveWithStatus(t.Context(), RetrieveInput{UserID: 1, Query: "hello"})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != RetrieveStatusUnavailable {
		t.Fatalf("expected unavailable, got %#v", result)
	}
}

func TestRetrieveWithStatusReportsEmptyInput(t *testing.T) {
	svc := NewServiceWithRuntime(config.NewRuntime(config.Config{
		RAGEnabled:       true,
		EmbeddingEnabled: true,
		RAGModel:         "embed",
	}), nil, nil, nil)

	result, err := svc.RetrieveWithStatus(t.Context(), RetrieveInput{UserID: 1, Query: " "})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != RetrieveStatusEmpty {
		t.Fatalf("expected empty, got %#v", result)
	}
}

func TestStoreRAGCacheSkipsEmptyResults(t *testing.T) {
	cache := &testRAGCache{}
	svc := NewServiceWithRuntime(config.NewRuntime(config.Config{}), nil, cache, nil)

	svc.storeRAGCache(t.Context(), 1, "query", nil, config.Config{RAGRetrievalCacheTTL: 60}, nil)

	if cache.setCalls != 0 {
		t.Fatalf("expected empty result not to be cached, got %d calls", cache.setCalls)
	}
}
