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
	getCalls int
	chunks   []domainconversation.RAGChunk
}

type testRAGRepository struct {
	vector []domainconversation.FileChunkSearchResult
	bm25   []domainconversation.FileChunkSearchResult
}

func (r *testRAGRepository) SearchFileChunks(context.Context, uint, []uint, []float32, string, int) ([]domainconversation.FileChunkSearchResult, error) {
	return append([]domainconversation.FileChunkSearchResult(nil), r.vector...), nil
}

func (r *testRAGRepository) BM25SearchFileChunks(context.Context, uint, []uint, string, int) ([]domainconversation.FileChunkSearchResult, error) {
	return append([]domainconversation.FileChunkSearchResult(nil), r.bm25...), nil
}

func (c *testRAGCache) GetRAGCache(ctx context.Context, key string) ([]domainconversation.RAGChunk, bool) {
	c.getCalls++
	return append([]domainconversation.RAGChunk(nil), c.chunks...), len(c.chunks) > 0
}

func TestRetrieveWithStatusEphemeralBypassesRetrievalCache(t *testing.T) {
	cache := &testRAGCache{chunks: []domainconversation.RAGChunk{{Content: "cached secret"}}}
	svc := NewServiceWithRuntime(config.NewRuntime(config.Config{
		RAGEnabled:       true,
		EmbeddingEnabled: true,
		RAGModel:         "embed",
	}), &testRAGRepository{}, cache, nil)

	_, err := svc.RetrieveWithStatus(t.Context(), RetrieveInput{
		UserID:    1,
		Query:     "private query",
		FileObjs:  []domainconversation.FileObject{{ID: 1, FileID: "file"}},
		Ephemeral: true,
	})
	if err == nil {
		t.Fatal("expected retrieval to continue past cache and fail without an embedding client")
	}
	if cache.getCalls != 0 || cache.setCalls != 0 {
		t.Fatalf("ephemeral retrieval touched cache: get=%d set=%d", cache.getCalls, cache.setCalls)
	}
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

func TestBuildRAGCacheKeyTracksFileRevisionAndIgnoresOrder(t *testing.T) {
	cfg := config.Config{RAGModel: "embed", EmbeddingOutputDimensions: 1536}
	updatedAt := time.Unix(100, 0)
	files := []domainconversation.FileObject{
		{ID: 2, FileID: "two", ChunkCount: 3, EmbedStatus: "ready", UpdatedAt: updatedAt},
		{ID: 1, FileID: "one", ChunkCount: 2, EmbedStatus: "ready", UpdatedAt: updatedAt},
	}
	reordered := []domainconversation.FileObject{files[1], files[0]}
	if buildRAGCacheKey(7, "query", files, cfg) != buildRAGCacheKey(7, "query", reordered, cfg) {
		t.Fatal("cache key changed when authorized files were only reordered")
	}

	revised := append([]domainconversation.FileObject(nil), files...)
	revised[0].UpdatedAt = updatedAt.Add(time.Second)
	if buildRAGCacheKey(7, "query", files, cfg) == buildRAGCacheKey(7, "query", revised, cfg) {
		t.Fatal("cache key did not change after file metadata revision")
	}

	hybrid := cfg
	hybrid.RAGHybridEnabled = true
	if buildRAGCacheKey(7, "query", files, cfg) == buildRAGCacheKey(7, "query", files, hybrid) {
		t.Fatal("cache key did not change when retrieval strategy changed")
	}
}

func TestHybridRetrieveKeepsVectorThresholdSeparateFromRRFScore(t *testing.T) {
	repo := &testRAGRepository{
		vector: []domainconversation.FileChunkSearchResult{
			{FileChunk: domainconversation.FileChunk{ID: 1}, Similarity: 0.9},
			{FileChunk: domainconversation.FileChunk{ID: 2}, Similarity: 0.2},
		},
		bm25: []domainconversation.FileChunkSearchResult{
			{FileChunk: domainconversation.FileChunk{ID: 2}},
			{FileChunk: domainconversation.FileChunk{ID: 3}},
		},
	}
	service := &Service{repo: repo}

	results, err := service.hybridRetrieve(t.Context(), hybridRetrieveInput{
		UserID:              7,
		FileObjectIDs:       []uint{10},
		Query:               "query",
		Embedding:           []float32{1},
		EmbeddingSignature:  "sig",
		TopK:                10,
		MinVectorSimilarity: 0.45,
	})
	if err != nil {
		t.Fatalf("hybridRetrieve() error = %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("hybridRetrieve() results = %#v, want vector hit plus two lexical hits", results)
	}
	byID := make(map[uint]domainconversation.FileChunkSearchResult, len(results))
	for _, result := range results {
		byID[result.ID] = result
		if result.RankScore <= 0 || result.RankScore > 1 {
			t.Fatalf("hybrid rank score = %f, want normalized score in (0,1]", result.RankScore)
		}
	}
	if byID[1].Similarity != 0.9 {
		t.Fatalf("vector similarity was overwritten by RRF score: %#v", byID[1])
	}
	if _, ok := byID[2]; !ok {
		t.Fatal("lexical hit was removed because its vector similarity was below threshold")
	}
	if filtered := filterRAGCandidates(results, 0.45, true); len(filtered) != len(results) {
		t.Fatalf("hybrid candidates were filtered using a vector threshold: %#v", filtered)
	}
}

func TestSelectRAGCandidatesUsesRelevanceBeforeDocumentOrderAndSkipsOversizedChunks(t *testing.T) {
	if selected := selectRAGCandidatesForContext(nil, 1, 1, false); len(selected) != 0 {
		t.Fatalf("empty candidate selection = %#v, want none", selected)
	}

	candidates := []domainconversation.FileChunkSearchResult{
		{FileChunk: domainconversation.FileChunk{ID: 20, FileObjID: 20, Content: "high"}, Similarity: 0.9},
		{FileChunk: domainconversation.FileChunk{ID: 1, FileObjID: 1, Content: "low"}, Similarity: 0.8},
	}
	selected := selectRAGCandidatesForContext(candidates, 1, 1, false)
	if len(selected) != 1 || selected[0].ID != 20 {
		t.Fatalf("selected = %#v, want highest-relevance chunk before document ordering", selected)
	}

	oversizedFirst := []domainconversation.FileChunkSearchResult{
		{FileChunk: domainconversation.FileChunk{ID: 30, FileObjID: 30, Content: "abcdefgh"}, Similarity: 0.95},
		{FileChunk: domainconversation.FileChunk{ID: 31, FileObjID: 31, Content: "a"}, Similarity: 0.9},
	}
	selected = selectRAGCandidatesForContext(oversizedFirst, 1, 1, false)
	if len(selected) != 1 || selected[0].ID != 31 {
		t.Fatalf("selected = %#v, want smaller relevant chunk after oversized candidate", selected)
	}
}

func TestSelectRAGCandidatesPrefersFileDiversityThenFillsByRelevance(t *testing.T) {
	candidates := []domainconversation.FileChunkSearchResult{
		{FileChunk: domainconversation.FileChunk{ID: 1, FileObjID: 10, ChunkIndex: 0, Content: "first"}, Similarity: 0.99},
		{FileChunk: domainconversation.FileChunk{ID: 2, FileObjID: 10, ChunkIndex: 1, Content: "second"}, Similarity: 0.98},
		{FileChunk: domainconversation.FileChunk{ID: 3, FileObjID: 10, ChunkIndex: 2, Content: "third"}, Similarity: 0.97},
		{FileChunk: domainconversation.FileChunk{ID: 4, FileObjID: 20, ChunkIndex: 0, Content: "other"}, Similarity: 0.80},
	}

	selected := selectRAGCandidatesForContext(candidates, 3, 100, false)
	if len(selected) != 3 {
		t.Fatalf("selected = %#v, want three chunks", selected)
	}
	selectedIDs := make(map[uint]struct{}, len(selected))
	for _, item := range selected {
		selectedIDs[item.ID] = struct{}{}
	}
	if _, ok := selectedIDs[4]; !ok {
		t.Fatalf("selected = %#v, want a relevant chunk from the second file", selected)
	}
	if _, ok := selectedIDs[3]; ok {
		t.Fatalf("selected = %#v, third same-file chunk should yield to source diversity", selected)
	}

	singleFile := selectRAGCandidatesForContext(candidates[:3], 3, 100, false)
	if len(singleFile) != 3 {
		t.Fatalf("single-file selection = %#v, want second pass to fill topK", singleFile)
	}
}

func TestSelectRAGCandidatesDoesNotTradeStrongEvidenceForWeakDiversity(t *testing.T) {
	candidates := []domainconversation.FileChunkSearchResult{
		{FileChunk: domainconversation.FileChunk{ID: 1, FileObjID: 10, Content: "first"}, Similarity: 0.99},
		{FileChunk: domainconversation.FileChunk{ID: 2, FileObjID: 10, Content: "second"}, Similarity: 0.98},
		{FileChunk: domainconversation.FileChunk{ID: 3, FileObjID: 10, Content: "third"}, Similarity: 0.97},
		{FileChunk: domainconversation.FileChunk{ID: 4, FileObjID: 20, Content: "weak"}, Similarity: 0.46},
	}

	selected := selectRAGCandidatesForContext(candidates, 3, 100, false)
	if len(selected) != 3 {
		t.Fatalf("selected = %#v, want three chunks", selected)
	}
	for _, item := range selected {
		if item.ID == 4 {
			t.Fatalf("selected = %#v, weak diversity candidate displaced stronger evidence", selected)
		}
	}
}

func TestSelectRAGCandidatesUsesStoredTokenCount(t *testing.T) {
	candidates := []domainconversation.FileChunkSearchResult{
		{FileChunk: domainconversation.FileChunk{ID: 1, FileObjID: 10, Content: "a", TokenCount: 3}, Similarity: 0.99},
		{FileChunk: domainconversation.FileChunk{ID: 2, FileObjID: 20, Content: "b", TokenCount: 1}, Similarity: 0.90},
	}

	selected := selectRAGCandidatesForContext(candidates, 1, 1, false)
	if len(selected) != 1 || selected[0].ID != 2 {
		t.Fatalf("selected = %#v, want token metadata to exclude the oversized first chunk", selected)
	}
}
