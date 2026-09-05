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
	portembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/embeddingutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
)

// Service 封装 RAG 检索能力。
type Service struct {
	cfg         *config.Runtime
	repo        repository.RAGRepository
	cache       repository.RAGCacheRepository
	embedClient EmbeddingClient
}

// EmbeddingClient 调用外部服务将文本批量转换为向量。
type EmbeddingClient interface {
	CallAPI(ctx context.Context, input portembedding.Request) ([][]float32, error)
}

// RetrieveInput 定义 RAG 检索输入。
type RetrieveInput struct {
	UserID    uint
	Query     string
	FileObjs  []domainconversation.FileObject
	Ephemeral bool
}

type hybridRetrieveInput struct {
	UserID              uint
	FileObjectIDs       []uint
	Query               string
	Embedding           []float32
	EmbeddingSignature  string
	TopK                int
	MinVectorSimilarity float32
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

const ragCacheVersion = "v3"

const ragInitialPerFileLimit = 2

const ragDiversityMinScoreRatio float32 = 0.75

// NewServiceWithRuntime 创建使用运行时配置容器的服务。
func NewServiceWithRuntime(cfg *config.Runtime, repo repository.RAGRepository, cache repository.RAGCacheRepository, embedClient EmbeddingClient) *Service {
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
	if !input.Ephemeral {
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
	minSimilarity := cfg.RAGMinSimilarity
	if minSimilarity <= 0 {
		minSimilarity = 0.45
	}
	fetchMultiplier := cfg.RAGFetchMultiplier
	if fetchMultiplier <= 0 {
		fetchMultiplier = 3
	}
	fetchK := topK * fetchMultiplier
	embeddingSignature := strings.TrimSpace(cfg.EmbeddingModelSignature)
	if embeddingSignature == "" {
		embeddingSignature = embeddingutil.ModelSignature(cfg.RAGModel, cfg.EmbeddingOutputDimensions)
	}

	var chunks []domainconversation.FileChunkSearchResult
	var searchErr error
	if cfg.RAGHybridEnabled {
		chunks, searchErr = s.hybridRetrieve(ctx, hybridRetrieveInput{
			UserID:              input.UserID,
			FileObjectIDs:       fileObjIDs,
			Query:               input.Query,
			Embedding:           embeddings[0],
			EmbeddingSignature:  embeddingSignature,
			TopK:                fetchK,
			MinVectorSimilarity: float32(minSimilarity),
		})
	} else {
		chunks, searchErr = s.repo.SearchFileChunks(ctx, input.UserID, fileObjIDs, embeddings[0], embeddingSignature, fetchK)
	}
	if searchErr != nil {
		return ragErrorResult(ctx, searchErr), fmt.Errorf("search file chunks: %w", searchErr)
	}
	if len(chunks) == 0 {
		return RetrieveResult{Status: RetrieveStatusEmpty, Reason: "no_candidates"}, nil
	}

	filtered := filterRAGCandidates(chunks, float32(minSimilarity), cfg.RAGHybridEnabled)
	if len(filtered) == 0 {
		return RetrieveResult{
			Status:         RetrieveStatusLowScore,
			Reason:         "below_min_similarity",
			CandidateCount: len(chunks),
			FilteredCount:  0,
			MaxScore:       maxRetrievalScore(chunks, cfg.RAGHybridEnabled),
		}, nil
	}

	tokenBudget := cfg.RAGTokenBudget
	if tokenBudget <= 0 {
		tokenBudget = 2000
	}
	selected := selectRAGCandidatesForContext(filtered, topK, tokenBudget, cfg.RAGHybridEnabled)
	if len(selected) == 0 {
		return RetrieveResult{
			Status:         RetrieveStatusEmpty,
			Reason:         "token_budget_exhausted",
			CandidateCount: len(chunks),
			FilteredCount:  len(filtered),
			MaxScore:       maxRetrievalScore(filtered, cfg.RAGHybridEnabled),
		}, nil
	}

	results := make([]domainconversation.RAGChunk, 0, len(selected))
	for _, c := range selected {
		results = append(results, domainconversation.RAGChunk{
			Content:    c.Content,
			FileName:   idToName[c.FileObjID],
			FileID:     idToFileID[c.FileObjID],
			ChunkIndex: c.ChunkIndex,
			Score:      retrievalScore(c, cfg.RAGHybridEnabled),
		})
	}
	if !input.Ephemeral {
		s.storeRAGCache(ctx, input.UserID, input.Query, input.FileObjs, cfg, results)
	}
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
		fileSignatures = append(fileSignatures, fmt.Sprintf("%d:%s:%d:%s:%d",
			fo.ID,
			strings.TrimSpace(fo.FileID),
			fo.ChunkCount,
			strings.TrimSpace(fo.EmbedStatus),
			fo.UpdatedAt.UnixNano(),
		))
	}
	sort.Strings(fileSignatures)

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("v=%s|u=%d|q=%s|provider=%s|m=%s|hybrid=%t|k=%d|sim=%.4f|budget=%d|fm=%d|dim=%d|files=",
		ragCacheVersion,
		userID,
		normalizeRAGQuery(query),
		strings.TrimSpace(cfg.EmbeddingHost),
		strings.TrimSpace(cfg.RAGModel),
		cfg.RAGHybridEnabled,
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

func maxRetrievalScore(chunks []domainconversation.FileChunkSearchResult, hybrid bool) float32 {
	var maxScore float32
	for _, chunk := range chunks {
		score := retrievalScore(chunk, hybrid)
		if score > maxScore {
			maxScore = score
		}
	}
	return maxScore
}

func retrievalScore(chunk domainconversation.FileChunkSearchResult, hybrid bool) float32 {
	if hybrid {
		return chunk.RankScore
	}
	return chunk.Similarity
}

func filterRAGCandidates(
	chunks []domainconversation.FileChunkSearchResult,
	minSimilarity float32,
	hybrid bool,
) []domainconversation.FileChunkSearchResult {
	if hybrid {
		return append([]domainconversation.FileChunkSearchResult(nil), chunks...)
	}
	filtered := make([]domainconversation.FileChunkSearchResult, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk.Similarity >= minSimilarity {
			filtered = append(filtered, chunk)
		}
	}
	return filtered
}

func selectRAGCandidatesForContext(
	candidates []domainconversation.FileChunkSearchResult,
	topK int,
	tokenBudget int,
	hybrid bool,
) []domainconversation.FileChunkSearchResult {
	if len(candidates) == 0 || topK <= 0 || tokenBudget <= 0 {
		return nil
	}
	selected := make([]domainconversation.FileChunkSearchResult, 0, min(topK, len(candidates)))
	selectedIndexes := make(map[int]struct{}, min(topK, len(candidates)))
	fileCounts := make(map[uint]int)
	var totalTokens int64
	bestScore := retrievalScore(candidates[0], hybrid)
	selectCandidates := func(enforceFileLimit bool) {
		for index, candidate := range candidates {
			if len(selected) >= topK {
				return
			}
			if _, exists := selectedIndexes[index]; exists {
				continue
			}
			if enforceFileLimit && fileCounts[candidate.FileObjID] >= ragInitialPerFileLimit {
				continue
			}
			if enforceFileLimit && bestScore > 0 && retrievalScore(candidate, hybrid) < bestScore*ragDiversityMinScoreRatio {
				continue
			}
			chunkTokens := ragChunkTokenEstimate(candidate)
			if totalTokens+chunkTokens > int64(tokenBudget) {
				continue
			}
			totalTokens += chunkTokens
			selectedIndexes[index] = struct{}{}
			fileCounts[candidate.FileObjID]++
			selected = append(selected, candidate)
		}
	}

	// Prefer evidence across files only while it remains reasonably close to the
	// strongest hit. The second pass fills remaining capacity by relevance, so a
	// weak document can never displace materially better evidence merely to add
	// variety.
	selectCandidates(true)
	selectCandidates(false)

	// Select by retrieval relevance first, then restore document order only for
	// the final prompt so adjacent chunks remain readable without displacing
	// higher-ranked evidence under a tight token budget.
	sortChunksByDocOrder(selected)
	return selected
}

func ragChunkTokenEstimate(candidate domainconversation.FileChunkSearchResult) int64 {
	if candidate.TokenCount > 0 {
		return int64(candidate.TokenCount)
	}
	return tokenestimate.Estimate(candidate.Content)
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
		batchEmbeddings, batchErr := s.embedClient.CallAPI(ctx, portembedding.Request{
			APIBase:        apiBase,
			APIKey:         apiKey,
			Model:          model,
			Texts:          texts[start:end],
			Dimensions:     cfg.EmbeddingOutputDimensions,
			TimeoutSeconds: cfg.EmbeddingTimeoutSeconds,
		})
		if batchErr != nil {
			return nil, batchErr
		}
		allEmbeddings = append(allEmbeddings, batchEmbeddings...)
	}
	return allEmbeddings, nil
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

// hybridRetrieve 并行执行向量检索与 BM25 全文检索，使用 RRF（Reciprocal Rank Fusion）合并结果。
// k=60 为 RRF 平滑系数，参考 Cormack et al. 2009 推荐值。
func (s *Service) hybridRetrieve(ctx context.Context, input hybridRetrieveInput) ([]domainconversation.FileChunkSearchResult, error) {
	type result struct {
		chunks []domainconversation.FileChunkSearchResult
		err    error
	}
	vecCh := make(chan result, 1)
	bm25Ch := make(chan result, 1)

	go func() {
		chunks, err := s.repo.SearchFileChunks(ctx, input.UserID, input.FileObjectIDs, input.Embedding, input.EmbeddingSignature, input.TopK)
		vecCh <- result{chunks, err}
	}()
	go func() {
		chunks, err := s.repo.BM25SearchFileChunks(ctx, input.UserID, input.FileObjectIDs, input.Query, input.TopK)
		bm25Ch <- result{chunks, err}
	}()

	vecResult := <-vecCh
	bm25Result := <-bm25Ch

	if vecResult.err != nil {
		return nil, vecResult.err
	}

	// RRF 合并
	const rrfK = 60.0
	const maxRRFScore = 2.0 / (rrfK + 1.0)
	scores := make(map[uint]float32)
	bestChunk := make(map[uint]domainconversation.FileChunkSearchResult)

	for rank, c := range vecResult.chunks {
		if c.Similarity < input.MinVectorSimilarity {
			continue
		}
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
		chunk.RankScore = min(1, scores[id]/float32(maxRRFScore))
		merged = append(merged, chunk)
	}
	// 按 RRF 得分降序排序
	for i := 1; i < len(merged); i++ {
		key := merged[i]
		j := i - 1
		for j >= 0 && (merged[j].RankScore < key.RankScore ||
			(merged[j].RankScore == key.RankScore && merged[j].ID > key.ID)) {
			merged[j+1] = merged[j]
			j--
		}
		merged[j+1] = key
	}
	if len(merged) > input.TopK {
		merged = merged[:input.TopK]
	}
	return merged, nil
}
