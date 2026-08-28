package embedding

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/embeddingutil"
	"go.uber.org/zap"
)

var ErrEmbeddingServiceNotConfigured = errors.New("embedding service not configured")

const embeddingWorkerConcurrency = 4

// Service 封装文件 embedding 执行与状态管理能力。
type Service struct {
	cfg         *config.Runtime
	repo        repository.EmbeddingRepository
	extractSvc  *extraction.Service
	embedClient EmbeddingClient
	logger      *zap.Logger
	workSlots   chan struct{}
	reindexJobs chan string
	reindexMu   sync.Mutex
	reindexing  bool
}

// EmbeddingClient 调用外部服务将文本批量转换为向量。
type EmbeddingClient interface {
	CallAPI(ctx context.Context, apiBase, apiKey, model string, texts []string, dimensions int, timeoutSeconds int) ([][]float32, error)
}

// NewService 创建 embedding 服务。
func NewService(cfg config.Config, repo repository.EmbeddingRepository, extractSvc *extraction.Service, embedClient EmbeddingClient, logger *zap.Logger) *Service {
	return NewServiceWithRuntime(config.NewRuntime(cfg), repo, extractSvc, embedClient, logger)
}

// NewServiceWithRuntime 创建使用运行时配置容器的 embedding 服务。
func NewServiceWithRuntime(cfg *config.Runtime, repo repository.EmbeddingRepository, extractSvc *extraction.Service, embedClient EmbeddingClient, logger *zap.Logger) *Service {
	if extractSvc == nil {
		extractSvc = extraction.NewServiceWithRuntime(cfg)
	}
	return &Service{
		cfg:         cfg,
		repo:        repo,
		extractSvc:  extractSvc,
		embedClient: embedClient,
		logger:      logger,
		workSlots:   make(chan struct{}, embeddingWorkerConcurrency),
		reindexJobs: make(chan string, 1),
	}
}

// StartBackgroundWorkers 启动后台重建任务的常驻执行协程；ctx 取消后不再领取新任务。
func (s *Service) StartBackgroundWorkers(ctx context.Context) {
	if s == nil || ctx == nil {
		return
	}
	background.Go(s.logger, "embedding_reindex_dispatch", func() {
		for {
			select {
			case <-ctx.Done():
				return
			case signature := <-s.reindexJobs:
				s.runReindex(ctx, signature)
			}
		}
	})
}

// Available 返回当前对话 RAG 检索能力是否可用及原因。
func (s *Service) Available(ctx context.Context) (bool, string) {
	cfg := s.snapshot()
	if !cfg.RAGEnabled {
		return false, "rag_disabled"
	}
	available, reason, _ := s.indexingAvailable(ctx, cfg)
	return available, reason
}

// IndexingAvailable 返回文件向量索引维护能力是否可用及原因。
func (s *Service) IndexingAvailable(ctx context.Context) (bool, string) {
	available, reason, _ := s.indexingAvailable(ctx, s.snapshot())
	return available, reason
}

func (s *Service) indexingAvailable(ctx context.Context, cfg config.Config) (bool, string, error) {
	if !cfg.EmbeddingEnabled {
		return false, "embedding_disabled", nil
	}
	if strings.TrimSpace(cfg.RAGModel) == "" {
		return false, "embedding_model_missing", nil
	}
	if strings.TrimSpace(cfg.EmbeddingHost) == "" {
		return false, "embedding_host_missing", nil
	}
	if s.embedClient == nil {
		return false, "embedding_client_missing", nil
	}
	if s.repo == nil {
		return false, "vector_store_unavailable", nil
	}
	available, err := s.repo.VectorStoreAvailable(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("embedding vector store availability check failed", zap.Error(err))
		}
		return false, "vector_store_error", err
	}
	if !available {
		return false, "vector_store_unavailable", nil
	}
	return true, "available", nil
}

// ShouldTrigger 判断当前文件是否应触发 embedding。
func (s *Service) ShouldTrigger(fileObj domainconversation.FileObject) bool {
	cfg := s.snapshot()
	if !cfg.EmbeddingEnabled || !cfg.EmbedTriggerOnUpload || strings.TrimSpace(cfg.RAGModel) == "" || strings.TrimSpace(cfg.EmbeddingHost) == "" {
		return false
	}
	return canEmbedFile(cfg, fileObj)
}

func canEmbedFile(cfg config.Config, fileObj domainconversation.FileObject) bool {
	if strings.TrimSpace(fileObj.StoragePath) == "" || strings.ToLower(strings.TrimSpace(fileObj.Status)) != "active" {
		return false
	}
	return supportsEmbeddingSource(fileObj, cfg)
}

// MaybeTrigger 在满足条件时异步触发 embedding。
func (s *Service) MaybeTrigger(fileObj domainconversation.FileObject) {
	if !s.ShouldTrigger(fileObj) {
		return
	}
	if available, _, _ := s.indexingAvailable(context.Background(), s.snapshot()); !available {
		return
	}
	s.Trigger(fileObj)
}

// Trigger 异步触发 embedding。
func (s *Service) Trigger(fileObj domainconversation.FileObject) {
	background.Go(s.logger, "embedding_process_file", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.ProcessFile(ctx, fileObj); err != nil && s.logger != nil {
			s.logger.Warn("embedding_failed",
				zap.String("file_id", fileObj.FileID),
				zap.Error(err),
			)
		}
	})
}

// ProcessFile 执行 embedding 完整流程。
func (s *Service) ProcessFile(ctx context.Context, fileObj domainconversation.FileObject) error {
	if s != nil && s.workSlots != nil {
		select {
		case s.workSlots <- struct{}{}:
			defer func() { <-s.workSlots }()
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	cfg := s.snapshot()
	embeddingSignature := configuredModelSignature(cfg)
	if !cfg.EmbeddingEnabled || strings.TrimSpace(cfg.RAGModel) == "" || strings.TrimSpace(cfg.EmbeddingHost) == "" {
		return nil
	}
	if s.repo == nil {
		return nil
	}
	if !supportsEmbeddingSource(fileObj, cfg) {
		return nil
	}

	claimed, err := s.repo.ClaimFileEmbedding(ctx, fileObj.UserID, fileObj.FileID, embeddingSignature)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}

	text, err := s.loadSourceText(ctx, fileObj)
	if err != nil {
		_ = s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, embeddingSignature, "failed", "无法提取文本")
		return err
	}
	if strings.TrimSpace(text) == "" {
		_ = s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, embeddingSignature, "failed", "无法提取文本")
		return fmt.Errorf("no extractable text in file %s", fileObj.FileID)
	}

	chunks := embeddingutil.ChunkText(text, cfg.EmbedChunkSizeTokens, cfg.EmbedChunkOverlapTokens)
	if len(chunks) == 0 {
		_ = s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, embeddingSignature, "failed", "分片结果为空")
		return nil
	}

	embeddings, err := s.embedTextsWithConfig(ctx, chunks, cfg)
	if err != nil {
		_ = s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, embeddingSignature, "failed", truncateError(err.Error(), 255))
		return err
	}

	now := time.Now()
	fileChunks := make([]domainconversation.FileChunk, 0, len(chunks))
	for i, chunk := range chunks {
		fileChunks = append(fileChunks, domainconversation.FileChunk{
			FileObjID:          fileObj.ID,
			UserID:             fileObj.UserID,
			ChunkIndex:         i,
			Content:            chunk,
			TokenCount:         int(estimateTokens(chunk)),
			EmbeddingSignature: embeddingSignature,
			CreatedAt:          now,
		})
	}
	published, err := s.repo.ReplaceFileChunks(ctx, fileObj.ID, embeddingSignature, fileChunks, embeddings)
	if err != nil {
		_ = s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, embeddingSignature, "failed", err.Error())
		return err
	}
	if !published {
		return nil
	}

	if current, countErr := s.repo.UpdateFileObjectChunkCount(ctx, fileObj.ID, embeddingSignature, len(fileChunks)); countErr != nil {
		return countErr
	} else if !current {
		return nil
	}
	return s.completeFileEmbedding(ctx, fileObj, embeddingSignature, cfg.EmbeddingHost)
}

func (s *Service) completeFileEmbedding(ctx context.Context, fileObj domainconversation.FileObject, expectedSignature string, expectedHost string) error {
	const configurationChanged = "embedding configuration changed during processing"
	if !s.embeddingConfigurationCurrent(expectedSignature, expectedHost) {
		_, err := s.repo.UpdateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, expectedSignature, "stale", configurationChanged)
		return err
	}
	current, err := s.repo.UpdateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, expectedSignature, "ready", "")
	if err != nil || !current {
		return err
	}
	// The second check closes the window where configuration changes between
	// the first check and publishing the ready state. A later change observes
	// a ready file and is handled by the normal global invalidation path.
	if !s.embeddingConfigurationCurrent(expectedSignature, expectedHost) {
		_, err = s.repo.UpdateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, expectedSignature, "stale", configurationChanged)
		return err
	}
	return nil
}

func (s *Service) embeddingConfigurationCurrent(expectedSignature string, expectedHost string) bool {
	cfg := s.snapshot()
	return configuredModelSignature(cfg) == expectedSignature &&
		strings.TrimRight(strings.TrimSpace(cfg.EmbeddingHost), "/") == strings.TrimRight(strings.TrimSpace(expectedHost), "/")
}

func (s *Service) updateFileObjectEmbedStatus(ctx context.Context, userID uint, fileID string, embeddingSignature string, status string, embedErr string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	writeCtx := ctx
	if writeCtx == nil || writeCtx.Err() != nil {
		var cancel context.CancelFunc
		writeCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	_, err := s.repo.UpdateFileObjectEmbedStatus(writeCtx, userID, fileID, embeddingSignature, status, embedErr)
	return err
}

// WaitReady 轮询等待文件 embedding 就绪。
func (s *Service) WaitReady(ctx context.Context, userID uint, fileID string, timeout time.Duration) bool {
	if s == nil || s.repo == nil {
		return false
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		fo, err := s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
		if err != nil || fo == nil {
			return false
		}
		if fo.EmbedStatus == "ready" {
			return true
		}
		if fo.EmbedStatus == "failed" {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(500 * time.Millisecond):
		}
	}
	return false
}

func (s *Service) loadSourceText(ctx context.Context, fileObj domainconversation.FileObject) (string, error) {
	if s != nil && s.repo != nil {
		if result, err := s.repo.GetFileObjectProcessingByObjectID(ctx, fileObj.ID); err == nil && result != nil {
			if path := strings.TrimSpace(result.ExtractStoragePath); path != "" && s.extractSvc != nil {
				text, readErr := s.extractSvc.ReadExtractedText(ctx, path)
				if readErr == nil && strings.TrimSpace(text) != "" {
					return text, nil
				}
			}
		}
	}

	cfg := s.snapshot()
	if s.extractSvc == nil {
		return "", fmt.Errorf("extract service not configured")
	}
	result, err := s.extractSvc.ExtractStoredFile(ctx, extraction.ExtractInput{
		File:                  fileObj,
		PDFMaxPages:           cfg.FileFullContextPDFMaxPages,
		OCREngine:             cfg.ExtractOCREngine,
		ImageOCREnabled:       cfg.ExtractImageOCREnabled,
		PDFOCRFallbackEnabled: cfg.ExtractPDFOCRFallbackEnabled,
	})
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// EmbedTexts 对外暴露向量化能力，供消息历史 embedding 等场景复用。
// 参数与返回值与内部 embedTexts 相同，失败时返回 error 而非 panic。
func (s *Service) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings, _, err := s.EmbedTextsWithSignature(ctx, texts)
	return embeddings, err
}

// EmbedTextsWithSignature 使用同一份配置快照生成向量和签名，避免配置切换期间错标向量空间。
func (s *Service) EmbedTextsWithSignature(ctx context.Context, texts []string) ([][]float32, string, error) {
	cfg := s.snapshot()
	embeddings, err := s.embedTextsWithConfig(ctx, texts, cfg)
	if err != nil {
		return nil, "", err
	}
	return embeddings, configuredModelSignature(cfg), nil
}

func (s *Service) embedTextsWithConfig(ctx context.Context, texts []string, cfg config.Config) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	model := strings.TrimSpace(cfg.RAGModel)
	host := strings.TrimSpace(cfg.EmbeddingHost)
	if !cfg.EmbeddingEnabled {
		return nil, fmt.Errorf("embedding disabled")
	}
	if model == "" || host == "" {
		return nil, fmt.Errorf("embedding model or host missing")
	}
	if s.embedClient == nil {
		return nil, fmt.Errorf("embedding client not configured")
	}

	apiBase := strings.TrimRight(host, "/")
	apiKey := strings.TrimSpace(cfg.EmbeddingKey)
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
		batchEmbeddings, batchErr := s.embedClient.CallAPI(ctx, apiBase, apiKey, model, texts[start:end], cfg.EmbeddingOutputDimensions, cfg.EmbeddingTimeoutSeconds)
		if batchErr != nil {
			return nil, batchErr
		}
		if len(batchEmbeddings) != end-start {
			return nil, fmt.Errorf("embedding batch returned %d vectors for %d texts", len(batchEmbeddings), end-start)
		}
		allEmbeddings = append(allEmbeddings, batchEmbeddings...)
	}
	if !cfg.EmbeddingNormalize {
		return allEmbeddings, nil
	}
	for index := range allEmbeddings {
		allEmbeddings[index] = l2Normalize(allEmbeddings[index])
	}
	return allEmbeddings, nil
}

func (s *Service) snapshot() config.Config {
	if s == nil || s.cfg == nil {
		return config.Config{}
	}
	return s.cfg.Snapshot()
}

// EmbeddingIndexStatus 表示向量索引的当前健康状态。
type EmbeddingIndexStatus struct {
	ModelSignature string
	ReadyCount     int64
	StaleCount     int64
	PendingCount   int64
	FailedCount    int64
	NeedsReindex   bool
}

// ComputeModelSignature 根据模型名和输出维度计算模型签名（格式: hex8@dims）。
// 相同模型/维度组合始终产生相同签名，用于检测配置变更。
func ComputeModelSignature(model string, outputDimensions int) string {
	return embeddingutil.ModelSignature(model, outputDimensions)
}

// ComputeSpaceSignature derives a new opaque vector-space identifier when an
// administrator changes the model, output dimensions, or provider endpoint.
func ComputeSpaceSignature(model string, outputDimensions int, endpoint string) string {
	return embeddingutil.SpaceSignature(model, outputDimensions, endpoint)
}

func configuredModelSignature(cfg config.Config) string {
	if signature := strings.TrimSpace(cfg.EmbeddingModelSignature); signature != "" {
		return signature
	}
	if strings.TrimSpace(cfg.RAGModel) == "" {
		return ""
	}
	return ComputeModelSignature(cfg.RAGModel, cfg.EmbeddingOutputDimensions)
}

// GetIndexStatus 返回向量索引的健康状态快照。
func (s *Service) GetIndexStatus(ctx context.Context) (EmbeddingIndexStatus, error) {
	cfg := s.snapshot()
	signature := configuredModelSignature(cfg)
	status := EmbeddingIndexStatus{
		ModelSignature: signature,
	}
	if s.repo == nil {
		return status, nil
	}
	var err error
	if status.ReadyCount, err = s.repo.CountFilesByEmbedStatus(ctx, "ready"); err != nil {
		return status, err
	}
	if status.StaleCount, err = s.repo.CountFilesByEmbedStatus(ctx, "stale"); err != nil {
		return status, err
	}
	if status.FailedCount, err = s.repo.CountFilesByEmbedStatus(ctx, "failed"); err != nil {
		return status, err
	}
	noneCount, _ := s.repo.CountFilesByEmbedStatus(ctx, "none")
	processingCount, _ := s.repo.CountFilesByEmbedStatus(ctx, "processing")
	status.PendingCount = noneCount + processingCount
	status.NeedsReindex = status.StaleCount > 0
	return status, nil
}

// MarkFilesStale 将不属于目标向量空间的文件标记为失效。
func (s *Service) MarkFilesStale(ctx context.Context, activeSignature string) (int64, error) {
	if s.repo == nil {
		return 0, nil
	}
	signature := strings.TrimSpace(activeSignature)
	if signature == "" {
		return 0, nil
	}
	return s.repo.MarkEmbeddedFilesStale(ctx, signature)
}

// ReconcileIndex 对账当前运行时配置与文件索引状态，用于启动恢复和失败补偿。
func (s *Service) ReconcileIndex(ctx context.Context) (int64, error) {
	return s.MarkFilesStale(ctx, configuredModelSignature(s.snapshot()))
}

// ReindexStaleFiles 提交一次去重的后台重建任务，返回本次纳入重建的文件数。
// 后台任务通过固定 worker 数执行，不会按文件数量无限创建 goroutine。
func (s *Service) ReindexStaleFiles(ctx context.Context) (int, error) {
	if s.repo == nil {
		return 0, nil
	}
	cfg := s.snapshot()
	available, _, err := s.indexingAvailable(ctx, cfg)
	if err != nil {
		return 0, err
	}
	if !available {
		return 0, ErrEmbeddingServiceNotConfigured
	}

	s.reindexMu.Lock()
	if s.reindexing {
		s.reindexMu.Unlock()
		return 0, nil
	}
	s.reindexing = true
	s.reindexMu.Unlock()
	started := false
	defer func() {
		if started {
			return
		}
		s.reindexMu.Lock()
		s.reindexing = false
		s.reindexMu.Unlock()
	}()

	const pageSize = 100
	submitted := 0
	var afterID uint
	for {
		files, err := s.repo.ListFilesForReindex(ctx, pageSize, afterID)
		if err != nil {
			return submitted, err
		}
		if len(files) == 0 {
			break
		}
		for _, f := range files {
			if canEmbedFile(cfg, f) {
				submitted++
			}
		}
		if len(files) < pageSize {
			break
		}
		afterID = files[len(files)-1].ID
	}
	if submitted == 0 {
		return 0, nil
	}

	started = true
	// reindexing 标记保证同一时刻至多一个待执行任务，缓冲为 1 的通道不会阻塞。
	s.reindexJobs <- configuredModelSignature(cfg)
	return submitted, nil
}

func (s *Service) runReindex(ctx context.Context, expectedSignature string) {
	defer func() {
		s.reindexMu.Lock()
		s.reindexing = false
		s.reindexMu.Unlock()
	}()

	jobs := make(chan domainconversation.FileObject, embeddingWorkerConcurrency)
	var workers sync.WaitGroup
	for range embeddingWorkerConcurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for fileObj := range jobs {
				if ctx.Err() != nil || configuredModelSignature(s.snapshot()) != expectedSignature {
					continue
				}
				jobCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
				err := s.ProcessFile(jobCtx, fileObj)
				cancel()
				if err != nil && !errors.Is(err, context.Canceled) && s.logger != nil {
					s.logger.Warn("embedding_reindex_failed", zap.String("file_id", fileObj.FileID), zap.Error(err))
				}
			}
		}()
	}

	cfg := s.snapshot()
	const pageSize = 100
	var afterID uint
scan:
	for ctx.Err() == nil && configuredModelSignature(s.snapshot()) == expectedSignature {
		files, err := s.repo.ListFilesForReindex(ctx, pageSize, afterID)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("embedding_reindex_list_failed", zap.Error(err))
			}
			break
		}
		if len(files) == 0 {
			break
		}
		for _, fileObj := range files {
			if !canEmbedFile(cfg, fileObj) {
				continue
			}
			select {
			case jobs <- fileObj:
			case <-ctx.Done():
				break scan
			}
		}
		if len(files) < pageSize {
			break
		}
		afterID = files[len(files)-1].ID
	}
	close(jobs)
	workers.Wait()
}

func supportsEmbeddingSource(fileObj domainconversation.FileObject, cfg config.Config) bool {
	switch strings.ToLower(strings.TrimSpace(fileObj.FileCategory)) {
	case "video":
		return false
	case "image":
		return cfg.ExtractImageOCREnabled
	}
	mime := strings.ToLower(strings.TrimSpace(fileObj.MimeType))
	name := strings.TrimSpace(fileObj.FileName)
	return isTextMIMEForEmbed(mime, name) || isPDFMIME(mime, name) || isWordMIME(mime, name) || isPresentationMIME(mime, name) || isExcelMIME(mime, name)
}

// l2Normalize 对向量做 L2 归一化（除以欧氏模长），返回单位向量。
// 零向量（模为 0）保持不变，避免除零。
func l2Normalize(vector []float32) []float32 {
	var sumSq float64
	for _, v := range vector {
		sumSq += float64(v) * float64(v)
	}
	if sumSq == 0 {
		return vector
	}
	norm := float32(1.0 / math.Sqrt(sumSq))
	result := make([]float32, len(vector))
	for i, v := range vector {
		result[i] = v * norm
	}
	return result
}

func truncateError(message string, limit int) string {
	value := strings.TrimSpace(message)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
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

func isPDFMIME(mimeType, fileName string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	if m == "application/pdf" {
		return true
	}
	if idx := strings.LastIndex(fileName, "."); idx >= 0 {
		return strings.ToLower(fileName[idx+1:]) == "pdf"
	}
	return false
}

func isTextMIMEForEmbed(mimeType, fileName string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(m, "text/") {
		return true
	}
	switch m {
	case "application/json", "application/xml", "application/javascript", "application/typescript",
		"application/yaml", "application/x-yaml", "application/toml":
		return true
	}
	if idx := strings.LastIndex(fileName, "."); idx >= 0 {
		ext := strings.ToLower(fileName[idx+1:])
		switch ext {
		case "txt", "md", "markdown", "csv", "json", "xml", "html", "htm",
			"css", "js", "ts", "jsx", "tsx", "py", "go", "rs", "java",
			"c", "cpp", "h", "hpp", "cs", "rb", "php", "swift", "kt",
			"sh", "bash", "zsh", "yaml", "yml", "toml", "ini", "conf", "sql":
			return true
		}
	}
	return false
}

func isWordMIME(mimeType, fileName string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	ext := ""
	if idx := strings.LastIndex(fileName, "."); idx >= 0 {
		ext = strings.ToLower(fileName[idx+1:])
	}
	return strings.Contains(m, "wordprocessingml") || strings.Contains(m, "msword") ||
		ext == "docx" || ext == "doc"
}

func isPresentationMIME(mimeType, fileName string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	ext := ""
	if idx := strings.LastIndex(fileName, "."); idx >= 0 {
		ext = strings.ToLower(fileName[idx+1:])
	}
	return strings.Contains(m, "presentationml") || strings.Contains(m, "ms-powerpoint") ||
		ext == "pptx" || ext == "ppt"
}

func isExcelMIME(mimeType, fileName string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	ext := ""
	if idx := strings.LastIndex(fileName, "."); idx >= 0 {
		ext = strings.ToLower(fileName[idx+1:])
	}
	return strings.Contains(m, "spreadsheetml") || strings.Contains(m, "ms-excel") ||
		m == "text/csv" || ext == "xlsx" || ext == "xls" || ext == "csv"
}
