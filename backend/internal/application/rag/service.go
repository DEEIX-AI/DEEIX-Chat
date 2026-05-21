package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// Service 封装 RAG 检索能力。
type Service struct {
	cfg         *config.Runtime
	repo        repository.RAGRepository
	cache       repository.RAGCacheRepository
	embedClient *embedding.Client
}

// RetrieveInput 定义 RAG 检索输入。
type RetrieveInput struct {
	UserID   uint
	Query    string
	FileObjs []domainconversation.FileObject
}

// RetrieveStatus 表示一次文件 RAG 检索的稳定结果状态。
type RetrieveStatus string

const (
	RetrieveStatusHit         RetrieveStatus = "rag_hit"
	RetrieveStatusEmpty       RetrieveStatus = "rag_empty"
	RetrieveStatusLowScore    RetrieveStatus = "rag_low_score"
	RetrieveStatusTimeout     RetrieveStatus = "rag_timeout"
	RetrieveStatusError       RetrieveStatus = "rag_error"
	RetrieveStatusUnavailable RetrieveStatus = "rag_unavailable"
)

// RetrieveResult 汇总 RAG 检索结果和诊断信息。
type RetrieveResult struct {
	Chunks         []domainconversation.RAGChunk
	Status         RetrieveStatus
	Reason         string
	CandidateCount int
	FilteredCount  int
	MaxScore       float32
	Cached         bool
}

// NewService 创建服务。
func NewService(cfg config.Config, repo repository.RAGRepository, cache repository.RAGCacheRepository, embedClient *embedding.Client) *Service {
	return NewServiceWithRuntime(config.NewRuntime(cfg), repo, cache, embedClient)
}

// NewServiceWithRuntime 创建使用运行时配置容器的服务。
func NewServiceWithRuntime(cfg *config.Runtime, repo repository.RAGRepository, cache repository.RAGCacheRepository, embedClient *embedding.Client) *Service {
	return &Service{
		cfg:         cfg,
		repo:        repo,
		cache:       cache,
		embedClient: embedClient,
	}
}

// Retrieve 对查询文本做向量检索，返回按文档顺序排列的最相关文本片段。
func (s *Service) Retrieve(ctx context.Context, input RetrieveInput) ([]domainconversation.RAGChunk, error) {
	result, err := s.RetrieveWithStatus(ctx, input)
	return result.Chunks, err
}

// RetrieveWithStatus 对查询文本做向量检索，并返回可观测的稳定状态。
func (s *Service) RetrieveWithStatus(ctx context.Context, input RetrieveInput) (RetrieveResult, error) {
	cfg := s.snapshot()
	if !cfg.RAGEnabled || !cfg.EmbeddingEnabled || cfg.RAGModel == "" {
		return RetrieveResult{Status: RetrieveStatusUnavailable, Reason: "rag_or_embedding_disabled"}, nil
	}
	if len(input.FileObjs) == 0 || strings.TrimSpace(input.Query) == "" {
		return RetrieveResult{Status: RetrieveStatusEmpty, Reason: "empty_query_or_files"}, nil
	}
	if cached, ok := s.loadRAGCache(ctx, input.UserID, input.Query, input.FileObjs, cfg); ok {
		return RetrieveResult{
			Chunks:         cached,
			Status:         RetrieveStatusHit,
			Reason:         "cache_hit",
			CandidateCount: len(cached),
			FilteredCount:  len(cached),
			MaxScore:       maxRAGChunkScore(cached),
			Cached:         true,
		}, nil
	}

	fileObjIDs := make([]uint, 0, len(input.FileObjs))
	idToName := make(map[uint]string, len(input.FileObjs))
	idToFileID := make(map[uint]string, len(input.FileObjs))
	for _, fo := range input.FileObjs {
		fileObjIDs = append(fileObjIDs, fo.ID)
		idToName[fo.ID] = fo.FileName
		idToFileID[fo.ID] = fo.FileID
	}

	embeddings, err := s.embedTexts(ctx, []string{input.Query}, cfg)
	if err != nil {
		return ragErrorResult(ctx, err), fmt.Errorf("embedding query failed: %w", err)
	}
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		err := fmt.Errorf("embedding provider returned empty result")
		return ragErrorResult(ctx, err), err
	}

	topK := cfg.RAGTopK
	if topK <= 0 {
		topK = 5
	}
	fetchMultiplier := cfg.RAGFetchMultiplier
	if fetchMultiplier <= 0 {
		fetchMultiplier = 3
	}
	fetchK := topK * fetchMultiplier

	var chunks []domainconversation.FileChunkSearchResult
	var searchErr error
	if cfg.RAGHybridEnabled {
		chunks, searchErr = s.hybridRetrieve(ctx, input.UserID, fileObjIDs, input.Query, embeddings[0], fetchK)
	} else {
		chunks, searchErr = s.repo.SearchFileChunks(ctx, input.UserID, fileObjIDs, embeddings[0], fetchK)
	}
	if searchErr != nil {
		return ragErrorResult(ctx, searchErr), fmt.Errorf("search file chunks: %w", searchErr)
	}
	if len(chunks) == 0 {
		return RetrieveResult{Status: RetrieveStatusEmpty, Reason: "no_candidates"}, nil
	}

	minSimilarity := cfg.RAGMinSimilarity
	if minSimilarity <= 0 {
		minSimilarity = 0.45
	}
	filtered := chunks[:0]
	for _, c := range chunks {
		if c.Similarity >= float32(minSimilarity) {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return RetrieveResult{
			Status:         RetrieveStatusLowScore,
			Reason:         "below_min_similarity",
			CandidateCount: len(chunks),
			FilteredCount:  0,
			MaxScore:       maxFileChunkScore(chunks),
		}, nil
	}

	sortChunksByDocOrder(filtered)

	tokenBudget := cfg.RAGTokenBudget
	if tokenBudget <= 0 {
		tokenBudget = 2000
	}
	var totalTokens int64
	results := make([]domainconversation.RAGChunk, 0, len(filtered))
	for _, c := range filtered {
		chunkTokens := estimateTokens(c.Content)
		if totalTokens+chunkTokens > int64(tokenBudget) {
			break
		}
		totalTokens += chunkTokens
		results = append(results, domainconversation.RAGChunk{
			Content:    c.Content,
			FileName:   idToName[c.FileObjID],
			FileID:     idToFileID[c.FileObjID],
			ChunkIndex: c.ChunkIndex,
			Score:      c.Similarity,
		})
	}
	if len(results) == 0 {
		return RetrieveResult{
			Status:         RetrieveStatusEmpty,
			Reason:         "token_budget_exhausted",
			CandidateCount: len(chunks),
			FilteredCount:  len(filtered),
			MaxScore:       maxFileChunkScore(filtered),
		}, nil
	}
	s.storeRAGCache(ctx, input.UserID, input.Query, input.FileObjs, cfg, results)
	return RetrieveResult{
		Chunks:         results,
		Status:         RetrieveStatusHit,
		Reason:         "matched",
		CandidateCount: len(chunks),
		FilteredCount:  len(filtered),
		MaxScore:       maxRAGChunkScore(results),
	}, nil
}

func (s *Service) snapshot() config.Config {
	if s == nil || s.cfg == nil {
		return config.Config{}
	}
	return s.cfg.Snapshot()
}

func normalizeRAGQuery(query string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
}

func buildRAGCacheKey(userID uint, query string, ragFileObjs []domainconversation.FileObject, cfg config.Config) string {
	fileSignatures := make([]string, 0, len(ragFileObjs))
	for _, fo := range ragFileObjs {
		fileSignatures = append(fileSignatures, fmt.Sprintf("%d:%s:%d:%s",
			fo.ID,
			strings.TrimSpace(fo.FileID),
			fo.ChunkCount,
			strings.TrimSpace(fo.EmbedStatus),
		))
	}
	sort.Strings(fileSignatures)

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("u=%d|q=%s|provider=%s|m=%s|k=%d|sim=%.4f|budget=%d|fm=%d|dim=%d|files=",
		userID,
		normalizeRAGQuery(query),
		strings.TrimSpace(cfg.EmbeddingHost),
		strings.TrimSpace(cfg.RAGModel),
		cfg.RAGTopK,
		cfg.RAGMinSimilarity,
		cfg.RAGTokenBudget,
		cfg.RAGFetchMultiplier,
		cfg.EmbeddingOutputDimensions,
	))
	for _, signature := range fileSignatures {
		builder.WriteString(signature)
		builder.WriteString("|")
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return "rag:cache:" + hex.EncodeToString(sum[:])
}

func (s *Service) loadRAGCache(ctx context.Context, userID uint, query string, ragFileObjs []domainconversation.FileObject, cfg config.Config) ([]domainconversation.RAGChunk, bool) {
	if s == nil || s.cache == nil || cfg.RAGRetrievalCacheTTL <= 0 {
		return nil, false
	}
	cacheKey := buildRAGCacheKey(userID, query, ragFileObjs, cfg)
	return s.cache.GetRAGCache(ctx, cacheKey)
}

func (s *Service) storeRAGCache(ctx context.Context, userID uint, query string, ragFileObjs []domainconversation.FileObject, cfg config.Config, chunks []domainconversation.RAGChunk) {
	if s == nil || s.cache == nil || cfg.RAGRetrievalCacheTTL <= 0 {
		return
	}
	if len(chunks) == 0 {
		return
	}
	cacheKey := buildRAGCacheKey(userID, query, ragFileObjs, cfg)
	s.cache.SetRAGCache(ctx, cacheKey, chunks, time.Duration(cfg.RAGRetrievalCacheTTL)*time.Second)
}

func ragErrorResult(ctx context.Context, err error) RetrieveResult {
	if isRAGTimeout(ctx, err) {
		return RetrieveResult{Status: RetrieveStatusTimeout, Reason: "deadline_exceeded"}
	}
	return RetrieveResult{Status: RetrieveStatusError, Reason: "retrieval_error"}
}

func isRAGTimeout(ctx context.Context, err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return ctx != nil && errors.Is(ctx.Err(), context.DeadlineExceeded)
}

func maxFileChunkScore(chunks []domainconversation.FileChunkSearchResult) float32 {
	var maxScore float32
	for _, chunk := range chunks {
		if chunk.Similarity > maxScore {
			maxScore = chunk.Similarity
		}
	}
	return maxScore
}

func maxRAGChunkScore(chunks []domainconversation.RAGChunk) float32 {
	var maxScore float32
	for _, chunk := range chunks {
		if chunk.Score > maxScore {
			maxScore = chunk.Score
		}
	}
	return maxScore
}

func sortChunksByDocOrder(chunks []domainconversation.FileChunkSearchResult) {
	n := len(chunks)
	for i := 1; i < n; i++ {
		for j := i; j > 0; j-- {
			a, b := chunks[j-1], chunks[j]
			if a.FileObjID > b.FileObjID || (a.FileObjID == b.FileObjID && a.ChunkIndex > b.ChunkIndex) {
				chunks[j-1], chunks[j] = chunks[j], chunks[j-1]
			} else {
				break
			}
		}
	}
}

func (s *Service) embedTexts(ctx context.Context, texts []string, cfg config.Config) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	model := strings.TrimSpace(cfg.RAGModel)
	host := strings.TrimSpace(cfg.EmbeddingHost)
	if !cfg.EmbeddingEnabled || model == "" || host == "" {
		return nil, fmt.Errorf("embedding unavailable: embedding model or host missing")
	}
	if s == nil || s.embedClient == nil {
		return nil, fmt.Errorf("embedding unavailable: client missing")
	}

	apiBase, apiKey, err := resolveEmbeddingUpstream(cfg)
	if err != nil || apiBase == "" {
		return nil, fmt.Errorf("no embedding endpoint available: %w", err)
	}

	batchSize := cfg.EmbedBatchSize
	if batchSize <= 0 {
		batchSize = 20
	}
	var allEmbeddings [][]float32
	for start := 0; start < len(texts); start += batchSize {
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batchEmbeddings, batchErr := s.embedClient.CallAPI(ctx, apiBase, apiKey, model, texts[start:end], cfg.EmbeddingTimeoutSeconds)
		if batchErr != nil {
			return nil, batchErr
		}
		allEmbeddings = append(allEmbeddings, batchEmbeddings...)
	}
	return normalizeEmbeddingBatchDimensions(allEmbeddings, cfg.EmbeddingOutputDimensions), nil
}

func resolveEmbeddingUpstream(cfg config.Config) (string, string, error) {
	if strings.TrimSpace(cfg.RAGModel) == "" {
		return "", "", fmt.Errorf("file.rag_model is required")
	}
	if strings.TrimSpace(cfg.EmbeddingHost) == "" {
		return "", "", fmt.Errorf("file.embedding_host is required")
	}
	return strings.TrimRight(strings.TrimSpace(cfg.EmbeddingHost), "/"), strings.TrimSpace(cfg.EmbeddingKey), nil
}

func normalizeEmbeddingBatchDimensions(embeddings [][]float32, outputDimensions int) [][]float32 {
	if outputDimensions <= 0 {
		return embeddings
	}
	result := make([][]float32, 0, len(embeddings))
	for _, vector := range embeddings {
		result = append(result, normalizeEmbeddingDimensions(vector, outputDimensions))
	}
	return result
}

func normalizeEmbeddingDimensions(vector []float32, outputDimensions int) []float32 {
	if outputDimensions <= 0 || len(vector) == outputDimensions {
		return vector
	}
	if len(vector) > outputDimensions {
		return append([]float32(nil), vector[:outputDimensions]...)
	}
	result := make([]float32, outputDimensions)
	copy(result, vector)
	return result
}

func estimateTokens(content string) int64 {
	if len(content) == 0 {
		return 0
	}
	var cjk, other int64
	for _, r := range content {
		if isCJKRune(r) {
			cjk++
		} else {
			other++
		}
	}
	tokens := (cjk*2+2)/3 + (other+3)/4
	if tokens == 0 {
		return 1
	}
	return tokens
}

func isCJKRune(r rune) bool {
	return (r >= 0x2E80 && r <= 0x9FFF) ||
		(r >= 0xAC00 && r <= 0xD7AF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0x20000 && r <= 0x2A6DF)
}

// hybridRetrieve 并行执行向量检索与 BM25 全文检索，使用 RRF（Reciprocal Rank Fusion）合并结果。
// k=60 为 RRF 平滑系数，参考 Cormack et al. 2009 推荐值。
func (s *Service) hybridRetrieve(ctx context.Context, userID uint, fileObjIDs []uint, query string, embedding []float32, topK int) ([]domainconversation.FileChunkSearchResult, error) {
	type result struct {
		chunks []domainconversation.FileChunkSearchResult
		err    error
	}
	vecCh := make(chan result, 1)
	bm25Ch := make(chan result, 1)

	go func() {
		chunks, err := s.repo.SearchFileChunks(ctx, userID, fileObjIDs, embedding, topK)
		vecCh <- result{chunks, err}
	}()
	go func() {
		chunks, err := s.repo.BM25SearchFileChunks(ctx, userID, fileObjIDs, query, topK)
		bm25Ch <- result{chunks, err}
	}()

	vecResult := <-vecCh
	bm25Result := <-bm25Ch

	if vecResult.err != nil {
		return nil, vecResult.err
	}

	// RRF 合并
	const rrfK = 60.0
	scores := make(map[uint]float32)
	bestChunk := make(map[uint]domainconversation.FileChunkSearchResult)

	for rank, c := range vecResult.chunks {
		scores[c.ID] += 1.0 / float32(rrfK+rank+1)
		bestChunk[c.ID] = c
	}
	if bm25Result.err == nil {
		for rank, c := range bm25Result.chunks {
			scores[c.ID] += 1.0 / float32(rrfK+rank+1)
			if _, exists := bestChunk[c.ID]; !exists {
				bestChunk[c.ID] = c
			}
		}
	}

	merged := make([]domainconversation.FileChunkSearchResult, 0, len(scores))
	for id, chunk := range bestChunk {
		chunk.Similarity = scores[id]
		merged = append(merged, chunk)
	}
	// 按 RRF 得分降序排序
	for i := 1; i < len(merged); i++ {
		key := merged[i]
		j := i - 1
		for j >= 0 && merged[j].Similarity < key.Similarity {
			merged[j+1] = merged[j]
			j--
		}
		merged[j+1] = key
	}
	if len(merged) > topK {
		merged = merged[:topK]
	}
	return merged, nil
}
