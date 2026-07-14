package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	infraembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

var ErrEmbeddingServiceNotConfigured = errors.New("embedding service not configured")

// Service 封装文件 embedding 执行与状态管理能力。
type Service struct {
	cfg         *config.Runtime
	repo        repository.EmbeddingRepository
	extractSvc  *extraction.Service
	embedClient *infraembedding.Client
	logger      *zap.Logger
}

// NewService 创建 embedding 服务。
func NewService(cfg config.Config, repo repository.EmbeddingRepository, extractSvc *extraction.Service, embedClient *infraembedding.Client, logger *zap.Logger) *Service {
	return NewServiceWithRuntime(config.NewRuntime(cfg), repo, extractSvc, embedClient, logger)
}

// NewServiceWithRuntime 创建使用运行时配置容器的 embedding 服务。
func NewServiceWithRuntime(cfg *config.Runtime, repo repository.EmbeddingRepository, extractSvc *extraction.Service, embedClient *infraembedding.Client, logger *zap.Logger) *Service {
	if extractSvc == nil {
		extractSvc = extraction.NewServiceWithRuntime(cfg)
	}
	return &Service{
		cfg:         cfg,
		repo:        repo,
		extractSvc:  extractSvc,
		embedClient: embedClient,
		logger:      logger,
	}
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
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.ProcessFile(ctx, fileObj); err != nil && s.logger != nil {
			s.logger.Warn("embedding_failed",
				zap.String("file_id", fileObj.FileID),
				zap.Error(err),
			)
		}
	}()
}

// ProcessFile 执行 embedding 完整流程。
func (s *Service) ProcessFile(ctx context.Context, fileObj domainconversation.FileObject) error {
	cfg := s.snapshot()
	if !cfg.EmbeddingEnabled || strings.TrimSpace(cfg.RAGModel) == "" || strings.TrimSpace(cfg.EmbeddingHost) == "" {
		return nil
	}
	if s.repo == nil {
		return nil
	}
	if !supportsEmbeddingSource(fileObj, cfg) {
		return nil
	}

	if err := s.repo.UpdateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, "processing", ""); err != nil {
		return err
	}

	text, err := s.loadSourceText(ctx, fileObj)
	if err != nil {
		_ = s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, "failed", "无法提取文本")
		return err
	}
	if strings.TrimSpace(text) == "" {
		_ = s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, "failed", "无法提取文本")
		return fmt.Errorf("no extractable text in file %s", fileObj.FileID)
	}

	chunks := infraembedding.ChunkText(text, cfg.EmbedChunkSizeTokens, cfg.EmbedChunkOverlapTokens)
	if len(chunks) == 0 {
		_ = s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, "failed", "分片结果为空")
		return nil
	}

	embeddings, err := s.embedTexts(ctx, chunks)
	if err != nil {
		_ = s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, "failed", truncateError(err.Error(), 255))
		return err
	}

	now := time.Now()
	fileChunks := make([]domainconversation.FileChunk, 0, len(chunks))
	for i, chunk := range chunks {
		fileChunks = append(fileChunks, domainconversation.FileChunk{
			FileObjID:  fileObj.ID,
			UserID:     fileObj.UserID,
			ChunkIndex: i,
			Content:    chunk,
			TokenCount: int(estimateTokens(chunk)),
			CreatedAt:  now,
		})
	}
	if err = s.repo.ReplaceFileChunks(ctx, fileObj.ID, fileChunks, embeddings); err != nil {
		_ = s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, "failed", err.Error())
		return err
	}

	_ = s.repo.UpdateFileObjectChunkCount(ctx, fileObj.ID, len(fileChunks))
	return s.repo.UpdateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, "ready", "")
}

func (s *Service) updateFileObjectEmbedStatus(ctx context.Context, userID uint, fileID string, status string, embedErr string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	writeCtx := ctx
	if writeCtx == nil || writeCtx.Err() != nil {
		var cancel context.CancelFunc
		writeCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	return s.repo.UpdateFileObjectEmbedStatus(writeCtx, userID, fileID, status, embedErr)
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
	return s.embedTexts(ctx, texts)
}

func (s *Service) embedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	cfg := s.snapshot()
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
		batchEmbeddings, batchErr := s.embedClient.CallAPI(ctx, apiBase, apiKey, model, texts[start:end], cfg.EmbeddingTimeoutSeconds)
		if batchErr != nil {
			return nil, batchErr
		}
		if len(batchEmbeddings) != end-start {
			return nil, fmt.Errorf("embedding batch returned %d vectors for %d texts", len(batchEmbeddings), end-start)
		}
		allEmbeddings = append(allEmbeddings, batchEmbeddings...)
	}
	return postProcessEmbeddings(allEmbeddings, cfg.EmbeddingOutputDimensions, cfg.EmbeddingNormalize), nil
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
	raw := model + "@" + strconv.Itoa(outputDimensions)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:4]) + "@" + strconv.Itoa(outputDimensions)
}

// GetIndexStatus 返回向量索引的健康状态快照。
func (s *Service) GetIndexStatus(ctx context.Context) (EmbeddingIndexStatus, error) {
	cfg := s.snapshot()
	signature := strings.TrimSpace(cfg.EmbeddingModelSignature)
	if signature == "" && strings.TrimSpace(cfg.RAGModel) != "" {
		signature = ComputeModelSignature(cfg.RAGModel, cfg.EmbeddingOutputDimensions)
	}
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

// MarkAllFilesStale 将所有已完成 embedding 的文件标记为失效，在模型变更时调用。
func (s *Service) MarkAllFilesStale(ctx context.Context) (int64, error) {
	if s.repo == nil {
		return 0, nil
	}
	return s.repo.MarkAllEmbeddedFilesStale(ctx)
}

// ReindexStaleFiles 异步触发所有可向量化的 none/stale/failed 文件，返回提交任务数。
// 实际 embedding 在 goroutine 中执行，调用方立即返回。
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
			if !canEmbedFile(cfg, f) {
				continue
			}
			s.Trigger(f)
			submitted++
		}
		if len(files) < pageSize {
			break
		}
		afterID = files[len(files)-1].ID
	}
	return submitted, nil
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

// postProcessEmbeddings 对批量向量做两步后处理：
//  1. 维度对齐（截断 or 零填充），使所有向量统一为 outputDimensions 维；
//     outputDimensions <= 0 时跳过。
//  2. L2 归一化（单位向量），使余弦相似度 = 点积，提升检索精度；
//     normalize=false 或向量模为 0 时跳过。
func postProcessEmbeddings(embeddings [][]float32, outputDimensions int, normalize bool) [][]float32 {
	result := make([][]float32, 0, len(embeddings))
	for _, vec := range embeddings {
		v := alignDimensions(vec, outputDimensions)
		if normalize {
			v = l2Normalize(v)
		}
		result = append(result, v)
	}
	return result
}

// alignDimensions 将向量截断或零填充到目标维度。
// outputDimensions <= 0 或维度已匹配时直接返回原向量。
func alignDimensions(vector []float32, outputDimensions int) []float32 {
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
