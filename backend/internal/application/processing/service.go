package processing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

const (
	// DefaultExtractorVersion 是当前文件处理流水线版本标识。
	DefaultExtractorVersion  = "file-pipeline-v1"
	fileProcessingMaxRetries = 3
	defaultProcessingPreview = 280
	defaultExtractTimeout    = 60 * time.Second
	fixedEmbeddingTimeout    = 5 * time.Minute
	failurePersistTimeout    = 5 * time.Second
)

var (
	// ErrFileProcessingFailed 表示文件处理失败。
	ErrFileProcessingFailed = errors.New("file processing failed")
)

// FileProcessingStatusDTO 文件处理状态响应数据。
type FileProcessingStatusDTO struct {
	FileID           string
	DetectedMIME     string
	FileCategory     string
	ProcessingStatus string
	ProcessingReady  bool
	ExtractStatus    string
	EmbedStatus      string
	PreviewText      string
	OCRUsed          bool
	RAGReady         bool
	RAGReason        string
	ErrorCode        string
	ErrorMessage     string
	ExtractChars     int
	ExtractPages     int
	StartedAt        *time.Time
	CompletedAt      *time.Time
}

// ReadyFileResult 表示等待文件处理完成后的可消费结果。
type ReadyFileResult struct {
	File          domainconversation.FileObject
	DetectedMIME  string
	FileCategory  string
	PageCount     int
	ExtractStatus string
	EmbedStatus   string
	ExtractedText string
}

// Service 封装文件处理后台流水线与状态查询能力。
type Service struct {
	cfg              *config.Runtime
	repo             repository.FileProcessingRepository
	cache            repository.FileProcessingQueueRepository
	extractSvc       *extraction.Service
	embeddingSvc     *appembedding.Service
	logger           *zap.Logger
	extractorVersion string
}

// NewService 创建文件处理服务。
func NewService(
	cfg config.Config,
	repo repository.FileProcessingRepository,
	cache repository.FileProcessingQueueRepository,
	extractSvc *extraction.Service,
	embeddingSvc *appembedding.Service,
	logger *zap.Logger,
	extractorVersion string,
) *Service {
	return NewServiceWithRuntime(config.NewRuntime(cfg), repo, cache, extractSvc, embeddingSvc, logger, extractorVersion)
}

// NewServiceWithRuntime 创建使用运行时配置容器的文件处理服务。
func NewServiceWithRuntime(
	cfg *config.Runtime,
	repo repository.FileProcessingRepository,
	cache repository.FileProcessingQueueRepository,
	extractSvc *extraction.Service,
	embeddingSvc *appembedding.Service,
	logger *zap.Logger,
	extractorVersion string,
) *Service {
	if extractSvc == nil {
		extractSvc = extraction.NewServiceWithRuntime(cfg)
	}
	return &Service{
		cfg:              cfg,
		repo:             repo,
		cache:            cache,
		extractSvc:       extractSvc,
		embeddingSvc:     embeddingSvc,
		logger:           logger,
		extractorVersion: strings.TrimSpace(extractorVersion),
	}
}

// StartBackgroundWorkers 启动文件处理后台 worker，ctx 取消时 worker 退出。
func (s *Service) StartBackgroundWorkers(ctx context.Context) {
	if s == nil || s.cache == nil {
		return
	}
	consumerName := "worker-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if err := s.cache.InitFileProcessingStream(ctx); err != nil && s.logger != nil {
		s.logger.Warn("create_file_processing_group_failed", zap.Error(err))
	}
	go s.runFileProcessingWorker(ctx, consumerName)
}

// InitializeUploadedFile 初始化新上传文件的处理状态。
func (s *Service) InitializeUploadedFile(ctx context.Context, fileObj *domainconversation.FileObject) error {
	if fileObj == nil {
		return nil
	}
	now := time.Now()
	if fileObj.FileCategory == "image" && !s.snapshot().ExtractImageOCREnabled {
		fileObj.ProcessingStatus = "ready"
		fileObj.ProcessingReady = true
		fileObj.ExtractStatus = "none"
		processingStatus := "ready"
		processingReady := true
		processingErrorCode := ""
		processingErrorMessage := ""
		extractStatus := "none"
		if err := s.repo.UpdateFileObjectProcessing(ctx, fileObj.UserID, fileObj.FileID, repository.UpdateFileObjectProcessingInput{
			ProcessingStatus:       &processingStatus,
			ProcessingReady:        &processingReady,
			ProcessingErrorCode:    &processingErrorCode,
			ProcessingErrorMessage: &processingErrorMessage,
			ExtractStatus:          &extractStatus,
		}); err != nil {
			return err
		}
		return s.repo.UpdateFileObjectProcessingState(ctx, &domainconversation.FileObjectProcessing{
			FileObjectID:     fileObj.ID,
			UserID:           fileObj.UserID,
			DetectedMIME:     fileObj.DetectedMIME,
			FileCategory:     fileObj.FileCategory,
			ProcessingStatus: "ready",
			ExtractStatus:    "none",
			RAGReady:         false,
			RAGReason:        "image_not_applicable",
			ExtractorVersion: s.version(),
			StartedAt:        &now,
			CompletedAt:      &now,
		})
	}

	if !supportsExtraction(fileObj.FileCategory) {
		return s.markFileProcessingFailed(ctx, fileObj, "mime_blocked", "unsupported file category")
	}

	processingStatus := "queued"
	processingReady := false
	extractStatus := "none"
	if err := s.repo.UpdateFileObjectProcessing(ctx, fileObj.UserID, fileObj.FileID, repository.UpdateFileObjectProcessingInput{
		ProcessingStatus: &processingStatus,
		ProcessingReady:  &processingReady,
		ExtractStatus:    &extractStatus,
	}); err != nil {
		return err
	}
	if err := s.repo.UpdateFileObjectProcessingState(ctx, &domainconversation.FileObjectProcessing{
		FileObjectID:     fileObj.ID,
		UserID:           fileObj.UserID,
		DetectedMIME:     fileObj.DetectedMIME,
		FileCategory:     fileObj.FileCategory,
		ProcessingStatus: "queued",
		ExtractStatus:    "none",
		ExtractorVersion: s.version(),
		StartedAt:        &now,
	}); err != nil {
		return err
	}
	return s.enqueueFileProcessing(ctx, fileObj.UserID, fileObj.FileID, 0, "")
}

// ProcessFile 执行单个文件处理任务。
func (s *Service) ProcessFile(ctx context.Context, userID uint, fileID string) error {
	fileObj, err := s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
	if err != nil || fileObj == nil {
		return err
	}
	if fileObj.FileCategory == "image" && !s.snapshot().ExtractImageOCREnabled {
		return nil
	}

	cfg := s.snapshot()
	extractTimeout := resolveProcessingExtractTimeout(cfg, fileObj.FileCategory)
	runCtx, cancel := context.WithTimeout(ctx, extractTimeout+fixedEmbeddingTimeout)
	defer cancel()

	startedAt := time.Now()
	processingStatus := "extracting"
	processingReady := false
	processingErrorCode := ""
	processingErrorMessage := ""
	extractStatus := "processing"
	extractorVersion := s.version()
	if err = s.repo.UpdateFileObjectProcessing(runCtx, userID, fileID, repository.UpdateFileObjectProcessingInput{
		ProcessingStatus:       &processingStatus,
		ProcessingReady:        &processingReady,
		ProcessingErrorCode:    &processingErrorCode,
		ProcessingErrorMessage: &processingErrorMessage,
		ExtractStatus:          &extractStatus,
		ExtractorVersion:       &extractorVersion,
	}); err != nil {
		return err
	}

	extractCtx, extractCancel := context.WithTimeout(runCtx, extractTimeout)
	extractResult, extractErr := s.extractTextForProcessing(extractCtx, *fileObj)
	extractCancel()
	if extractErr != nil {
		code, message := resolveProcessingFailure(fileObj, extractErr)
		return s.markFileProcessingFailed(runCtx, fileObj, code, message)
	}
	if strings.TrimSpace(extractResult.Text) == "" {
		return s.markFileProcessingFailed(runCtx, fileObj, "extract_failed", "无法提取文本")
	}

	extractPath, err := s.extractSvc.WriteExtractedText(runCtx, fileObj.UserID, fileObj.FileID, extractResult.Text)
	if err != nil {
		return s.markFileProcessingFailed(runCtx, fileObj, "extract_failed", err.Error())
	}
	now := time.Now()
	preview := compactSnippet(extractResult.Text, defaultProcessingPreview)
	ragAvailable, ragReason := s.embeddingSvc.Available(runCtx)
	indexingAvailable, _ := s.embeddingSvc.IndexingAvailable(runCtx)
	resultRAGReady := false
	resultRAGReason := "not_applicable"
	if supportsRAG(fileObj.FileCategory) {
		resultRAGReady = ragAvailable
		if ragAvailable {
			resultRAGReason = "embedding_pending"
		} else {
			resultRAGReason = ragReason
		}
	}
	if err = s.repo.UpdateFileObjectProcessingState(runCtx, &domainconversation.FileObjectProcessing{
		FileObjectID:       fileObj.ID,
		UserID:             fileObj.UserID,
		DetectedMIME:       fileObj.DetectedMIME,
		FileCategory:       fileObj.FileCategory,
		ProcessingStatus:   "extracted",
		ExtractStatus:      "ready",
		ExtractEngine:      extractResult.Engine,
		ExtractStoragePath: extractPath,
		ExtractChars:       len([]rune(extractResult.Text)),
		ExtractPages:       extractResult.PageCount,
		PreviewText:        preview,
		OCRUsed:            extractResult.OCRUsed,
		RAGReady:           resultRAGReady,
		RAGReason:          resultRAGReason,
		ExtractorVersion:   s.version(),
		StartedAt:          &startedAt,
		CompletedAt:        &now,
	}); err != nil {
		return err
	}
	processingStatus = "extracted"
	processingReady = true
	extractStatus = "ready"
	extractedAt := &now
	extractorVersion = s.version()
	if err = s.repo.UpdateFileObjectProcessing(runCtx, fileObj.UserID, fileObj.FileID, repository.UpdateFileObjectProcessingInput{
		ProcessingStatus: &processingStatus,
		ProcessingReady:  &processingReady,
		ExtractStatus:    &extractStatus,
		PageCount:        &extractResult.PageCount,
		ExtractedAt:      &extractedAt,
		ExtractorVersion: &extractorVersion,
	}); err != nil {
		return err
	}

	if indexingAvailable && supportsRAG(fileObj.FileCategory) && s.embeddingSvc.ShouldTrigger(*fileObj) {
		processingStatus = "embedding"
		_ = s.repo.UpdateFileObjectProcessing(runCtx, fileObj.UserID, fileObj.FileID, repository.UpdateFileObjectProcessingInput{
			ProcessingStatus: &processingStatus,
		})
		embedCtx, embedCancel := context.WithTimeout(runCtx, fixedEmbeddingTimeout)
		embedErr := s.embeddingSvc.ProcessFile(embedCtx, *fileObj)
		embedCancel()
		if embedErr != nil {
			_ = s.repo.UpdateFileObjectProcessingState(runCtx, &domainconversation.FileObjectProcessing{
				FileObjectID:       fileObj.ID,
				UserID:             fileObj.UserID,
				DetectedMIME:       fileObj.DetectedMIME,
				FileCategory:       fileObj.FileCategory,
				ProcessingStatus:   "ready",
				ExtractStatus:      "ready",
				ExtractEngine:      extractResult.Engine,
				ExtractStoragePath: extractPath,
				ExtractChars:       len([]rune(extractResult.Text)),
				ExtractPages:       extractResult.PageCount,
				PreviewText:        preview,
				OCRUsed:            extractResult.OCRUsed,
				RAGReady:           false,
				RAGReason:          "embed_failed",
				ErrorCode:          "embed_failed",
				ErrorMessage:       truncateError(embedErr.Error(), 255),
				ExtractorVersion:   s.version(),
				StartedAt:          &startedAt,
				CompletedAt:        &now,
			})
			processingStatus = "ready"
			processingReady = true
			processingErrorCode = "embed_failed"
			processingErrorMessage = truncateError(embedErr.Error(), 255)
			_ = s.repo.UpdateFileObjectProcessing(runCtx, fileObj.UserID, fileObj.FileID, repository.UpdateFileObjectProcessingInput{
				ProcessingStatus:       &processingStatus,
				ProcessingReady:        &processingReady,
				ProcessingErrorCode:    &processingErrorCode,
				ProcessingErrorMessage: &processingErrorMessage,
			})
			return nil
		}
	}

	if err = s.repo.UpdateFileObjectProcessingState(runCtx, &domainconversation.FileObjectProcessing{
		FileObjectID:       fileObj.ID,
		UserID:             fileObj.UserID,
		DetectedMIME:       fileObj.DetectedMIME,
		FileCategory:       fileObj.FileCategory,
		ProcessingStatus:   "ready",
		ExtractStatus:      "ready",
		ExtractEngine:      extractResult.Engine,
		ExtractStoragePath: extractPath,
		ExtractChars:       len([]rune(extractResult.Text)),
		ExtractPages:       extractResult.PageCount,
		PreviewText:        preview,
		OCRUsed:            extractResult.OCRUsed,
		RAGReady:           ragAvailable && supportsRAG(fileObj.FileCategory),
		RAGReason: func() string {
			if !supportsRAG(fileObj.FileCategory) {
				return "not_applicable"
			}
			if ragAvailable {
				return "ready"
			}
			return ragReason
		}(),
		ExtractorVersion: s.version(),
		StartedAt:        &startedAt,
		CompletedAt:      &now,
	}); err != nil {
		return err
	}
	processingStatus = "ready"
	processingReady = true
	processingErrorCode = ""
	processingErrorMessage = ""
	return s.repo.UpdateFileObjectProcessing(runCtx, fileObj.UserID, fileObj.FileID, repository.UpdateFileObjectProcessingInput{
		ProcessingStatus:       &processingStatus,
		ProcessingReady:        &processingReady,
		ProcessingErrorCode:    &processingErrorCode,
		ProcessingErrorMessage: &processingErrorMessage,
	})
}

// GetFileProcessingStatus 查询文件处理状态。
func (s *Service) GetFileProcessingStatus(ctx context.Context, userID uint, fileID string) (*FileProcessingStatusDTO, error) {
	fileObj, err := s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
	if err != nil || fileObj == nil {
		return nil, err
	}
	result, err := s.repo.GetFileObjectProcessingByObjectID(ctx, fileObj.ID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	dto := &FileProcessingStatusDTO{
		FileID:           fileObj.FileID,
		DetectedMIME:     fileObj.DetectedMIME,
		FileCategory:     fileObj.FileCategory,
		ProcessingStatus: fileObj.ProcessingStatus,
		ProcessingReady:  fileObj.ProcessingReady,
		ExtractStatus:    fileObj.ExtractStatus,
		EmbedStatus:      fileObj.EmbedStatus,
		ErrorCode:        fileObj.ProcessingErrorCode,
		ErrorMessage:     fileObj.ProcessingErrorMessage,
	}
	if result != nil {
		dto.PreviewText = result.PreviewText
		dto.OCRUsed = result.OCRUsed
		dto.RAGReady = result.RAGReady
		dto.RAGReason = result.RAGReason
		dto.ErrorCode = result.ErrorCode
		dto.ErrorMessage = result.ErrorMessage
		dto.ExtractChars = result.ExtractChars
		dto.ExtractPages = result.ExtractPages
		dto.StartedAt = result.StartedAt
		dto.CompletedAt = result.CompletedAt
	}
	dto.ErrorMessage = HumanizeFileProcessingError(dto.FileCategory, dto.ErrorCode, dto.ErrorMessage)
	return dto, nil
}

// WaitUntilReady 等待文件处理完成，并在就绪时返回提取产物。
func (s *Service) WaitUntilReady(
	ctx context.Context,
	userID uint,
	fileID string,
	onProgress func(fileObj *domainconversation.FileObject),
) (*ReadyFileResult, error) {
	for {
		fileObj, err := s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
		if err != nil || fileObj == nil {
			return nil, err
		}
		if fileObj.ProcessingReady {
			result := &ReadyFileResult{
				File:          *fileObj,
				DetectedMIME:  fileObj.DetectedMIME,
				FileCategory:  fileObj.FileCategory,
				PageCount:     fileObj.PageCount,
				ExtractStatus: fileObj.ExtractStatus,
				EmbedStatus:   fileObj.EmbedStatus,
			}
			if processingResult, resultErr := s.repo.GetFileObjectProcessingByObjectID(ctx, fileObj.ID); resultErr == nil && processingResult != nil && strings.TrimSpace(processingResult.ExtractStoragePath) != "" && s.extractSvc != nil {
				if text, readErr := s.extractSvc.ReadExtractedText(ctx, processingResult.ExtractStoragePath); readErr == nil {
					result.ExtractedText = text
				}
			}
			return result, nil
		}
		if fileObj.ProcessingStatus == "failed" {
			if onProgress != nil {
				onProgress(fileObj)
			}
			message := HumanizeFileProcessingError(fileObj.FileCategory, fileObj.ProcessingErrorCode, fileObj.ProcessingErrorMessage)
			if message == "" {
				message = fileObj.ProcessingStatus
			}
			return nil, fmt.Errorf("%w: %s", ErrFileProcessingFailed, message)
		}
		if onProgress != nil {
			onProgress(fileObj)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	}
}

func (s *Service) runFileProcessingWorker(ctx context.Context, consumerName string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		claimed, claimErr := s.cache.ClaimTimedOutFileProcessingMessages(ctx, consumerName)
		if claimErr != nil {
			if ctx.Err() != nil {
				return
			}
			if s.logger != nil {
				s.logger.Warn("file_processing_worker_claim_failed", zap.Error(claimErr))
			}
		} else if len(claimed) > 0 {
			for _, msg := range claimed {
				s.handleProcessingMessage(ctx, msg)
			}
			continue
		}

		messages, err := s.cache.ReadFileProcessingMessages(ctx, consumerName)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if s.logger != nil {
				s.logger.Warn("file_processing_worker_read_failed", zap.Error(err))
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		for _, msg := range messages {
			s.handleProcessingMessage(ctx, msg)
		}
	}
}

func (s *Service) handleProcessingMessage(ctx context.Context, msg repository.FileProcessingMessage) {
	if msg.UserID == 0 || msg.FileID == "" {
		_ = s.cache.AckFileProcessingMessage(ctx, msg.ID)
		_ = s.cache.DeleteFileProcessingMessage(ctx, msg.ID)
		return
	}

	err := s.ProcessFile(ctx, msg.UserID, msg.FileID)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		if msg.Retry < fileProcessingMaxRetries {
			_ = s.enqueueFileProcessing(ctx, msg.UserID, msg.FileID, msg.Retry+1, err.Error())
		} else {
			_ = s.cache.SendFileProcessingToDLQ(ctx, msg.UserID, msg.FileID, msg.Retry, err.Error())
			s.forceFinalizeFailed(msg.UserID, msg.FileID, err)
		}
		if s.logger != nil {
			s.logger.Warn("process_queued_file_failed",
				zap.Uint("user_id", msg.UserID),
				zap.String("file_id", msg.FileID),
				zap.Int("retry", msg.Retry),
				zap.Error(err),
			)
		}
	}

	_ = s.cache.AckFileProcessingMessage(ctx, msg.ID)
	_ = s.cache.DeleteFileProcessingMessage(ctx, msg.ID)
}

func (s *Service) forceFinalizeFailed(userID uint, fileID string, processingErr error) {
	if s == nil || s.repo == nil || userID == 0 || strings.TrimSpace(fileID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), failurePersistTimeout)
	defer cancel()

	fileObj, err := s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
	if err != nil || fileObj == nil {
		return
	}
	if fileObj.ProcessingStatus == "ready" || fileObj.ProcessingStatus == "failed" {
		return
	}

	code, message := resolveProcessingFailure(fileObj, processingErr)
	if persistErr := s.markFileProcessingFailed(ctx, fileObj, code, message); persistErr != nil && s.logger != nil {
		s.logger.Warn("force_finalize_file_processing_failed",
			zap.Uint("user_id", userID),
			zap.String("file_id", fileID),
			zap.Error(persistErr),
		)
	}
}

func (s *Service) enqueueFileProcessing(ctx context.Context, userID uint, fileID string, retry int, lastError string) error {
	if s.cache == nil {
		go func() {
			_ = s.ProcessFile(context.Background(), userID, fileID)
		}()
		return nil
	}
	return s.cache.EnqueueFileProcessing(ctx, userID, fileID, retry, lastError)
}

func (s *Service) markFileProcessingFailed(ctx context.Context, fileObj *domainconversation.FileObject, code string, message string) error {
	if fileObj == nil {
		return nil
	}
	writeCtx := ctx
	if writeCtx == nil || writeCtx.Err() != nil {
		var cancel context.CancelFunc
		writeCtx, cancel = context.WithTimeout(context.Background(), failurePersistTimeout)
		defer cancel()
	}
	now := time.Now()
	if err := s.repo.UpdateFileObjectProcessingState(writeCtx, &domainconversation.FileObjectProcessing{
		FileObjectID:     fileObj.ID,
		UserID:           fileObj.UserID,
		DetectedMIME:     fileObj.DetectedMIME,
		FileCategory:     fileObj.FileCategory,
		ProcessingStatus: "failed",
		ExtractStatus:    "failed",
		RAGReady:         false,
		RAGReason:        code,
		ErrorCode:        code,
		ErrorMessage:     truncateError(message, 255),
		ExtractorVersion: s.version(),
		CompletedAt:      &now,
	}); err != nil {
		return err
	}
	processingStatus := "failed"
	processingReady := false
	processingErrorMessage := truncateError(message, 255)
	extractStatus := "failed"
	return s.repo.UpdateFileObjectProcessing(writeCtx, fileObj.UserID, fileObj.FileID, repository.UpdateFileObjectProcessingInput{
		ProcessingStatus:       &processingStatus,
		ProcessingReady:        &processingReady,
		ProcessingErrorCode:    &code,
		ProcessingErrorMessage: &processingErrorMessage,
		ExtractStatus:          &extractStatus,
	})
}

func (s *Service) extractTextForProcessing(ctx context.Context, fileObj domainconversation.FileObject) (extraction.Result, error) {
	type extractOutcome struct {
		result extraction.Result
		err    error
	}

	done := make(chan extractOutcome, 1)
	go func() {
		result, err := s.extractSvc.ExtractStoredFile(ctx, extraction.ExtractInput{
			File:                  fileObj,
			PDFMaxPages:           0,
			OCREngine:             s.snapshot().ExtractOCREngine,
			ImageOCREnabled:       s.snapshot().ExtractImageOCREnabled,
			PDFOCRFallbackEnabled: s.snapshot().ExtractPDFOCRFallbackEnabled,
		})
		done <- extractOutcome{result: result, err: err}
	}()

	select {
	case <-ctx.Done():
		return extraction.Result{}, ctx.Err()
	case outcome := <-done:
		return outcome.result, outcome.err
	}
}

func (s *Service) snapshot() config.Config {
	if s == nil || s.cfg == nil {
		return config.Config{}
	}
	return s.cfg.Snapshot()
}

func resolveProcessingExtractTimeout(cfg config.Config, fileCategory string) time.Duration {
	primaryTimeout := resolvePrimaryExtractTimeout(cfg)
	ocrTimeout := resolveOCRExtractTimeout(cfg)

	switch strings.ToLower(strings.TrimSpace(fileCategory)) {
	case "image":
		if cfg.ExtractImageOCREnabled {
			return ocrTimeout
		}
	case "pdf":
		if cfg.ExtractPDFOCRFallbackEnabled {
			return primaryTimeout + ocrTimeout
		}
	}
	return primaryTimeout
}

func resolvePrimaryExtractTimeout(cfg config.Config) time.Duration {
	timeoutSeconds := 0
	switch strings.ToLower(strings.TrimSpace(cfg.ExtractEngine)) {
	case extraction.EngineTika:
		timeoutSeconds = cfg.ExtractTikaTimeoutSeconds
	case extraction.EngineDocling:
		timeoutSeconds = cfg.ExtractDoclingTimeoutSeconds
	case extraction.EngineMinerU:
		timeoutSeconds = cfg.ExtractMinerUTimeoutSeconds
	default:
		timeoutSeconds = int(defaultExtractTimeout / time.Second)
	}
	if timeoutSeconds <= 0 {
		return defaultExtractTimeout
	}
	return time.Duration(timeoutSeconds) * time.Second
}

func resolveOCRExtractTimeout(cfg config.Config) time.Duration {
	timeoutSeconds := 0
	switch strings.ToLower(strings.TrimSpace(cfg.ExtractOCREngine)) {
	case extraction.OCREngineTesseract:
		timeoutSeconds = cfg.ExtractTesseractOCRTimeoutSeconds
	case extraction.OCREngineRapidOCR:
		timeoutSeconds = cfg.ExtractRapidOCRTimeoutSeconds
	case extraction.OCREnginePaddle:
		timeoutSeconds = cfg.ExtractPaddleOCRTimeoutSeconds
	case extraction.OCREngineTencent:
		timeoutSeconds = cfg.ExtractTencentOCRTimeoutSeconds
	case extraction.OCREngineAliyun:
		timeoutSeconds = cfg.ExtractAliyunOCRTimeoutSeconds
	case extraction.OCREngineLLM:
		timeoutSeconds = cfg.ExtractLLMOCRTimeoutSeconds
	default:
		timeoutSeconds = int(defaultExtractTimeout / time.Second)
	}
	if timeoutSeconds <= 0 {
		return defaultExtractTimeout
	}
	return time.Duration(timeoutSeconds) * time.Second
}

func (s *Service) version() string {
	if strings.TrimSpace(s.extractorVersion) == "" {
		return DefaultExtractorVersion
	}
	return s.extractorVersion
}

func classifyProcessingErrorCode(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "tesseract_ocr_disabled"):
		return "tesseract_ocr_disabled"
	case strings.Contains(msg, "tesseract_ocr_failed"):
		return "tesseract_ocr_failed"
	case strings.Contains(msg, "tesseract_ocr_empty_content"):
		return "tesseract_ocr_empty_content"
	case strings.Contains(msg, "tesseract_ocr_unprocessable"):
		return "tesseract_ocr_unprocessable"
	case strings.Contains(msg, "tesseract_ocr_invalid_response"):
		return "tesseract_ocr_invalid_response"
	case strings.Contains(msg, "tesseract_ocr_unauthorized"):
		return "tesseract_ocr_unauthorized"
	case strings.Contains(msg, "tesseract_ocr_forbidden"):
		return "tesseract_ocr_forbidden"
	case strings.Contains(msg, "tesseract_ocr_http_"):
		return "tesseract_ocr_http_error"
	case strings.Contains(msg, "tesseract_ocr_unavailable"):
		return "tesseract_ocr_unavailable"
	case strings.Contains(msg, "rapidocr_ocr_disabled"):
		return "rapidocr_ocr_disabled"
	case strings.Contains(msg, "rapidocr_ocr_failed"):
		return "rapidocr_ocr_failed"
	case strings.Contains(msg, "rapidocr_ocr_empty_content"):
		return "rapidocr_ocr_empty_content"
	case strings.Contains(msg, "rapidocr_ocr_unprocessable"):
		return "rapidocr_ocr_unprocessable"
	case strings.Contains(msg, "rapidocr_ocr_invalid_response"):
		return "rapidocr_ocr_invalid_response"
	case strings.Contains(msg, "rapidocr_ocr_unauthorized"):
		return "rapidocr_ocr_unauthorized"
	case strings.Contains(msg, "rapidocr_ocr_forbidden"):
		return "rapidocr_ocr_forbidden"
	case strings.Contains(msg, "rapidocr_ocr_http_"):
		return "rapidocr_ocr_http_error"
	case strings.Contains(msg, "rapidocr_ocr_unavailable"):
		return "rapidocr_ocr_unavailable"
	case strings.Contains(msg, "llm_ocr_disabled"):
		return "llm_ocr_disabled"
	case strings.Contains(msg, "llm_ocr_failed"):
		return "llm_ocr_failed"
	case strings.Contains(msg, "llm_ocr_empty_content"):
		return "llm_ocr_empty_content"
	case strings.Contains(msg, "llm_ocr_unprocessable"):
		return "llm_ocr_unprocessable"
	case strings.Contains(msg, "llm_ocr_invalid_response"):
		return "llm_ocr_invalid_response"
	case strings.Contains(msg, "llm_ocr_unauthorized"):
		return "llm_ocr_unauthorized"
	case strings.Contains(msg, "llm_ocr_forbidden"):
		return "llm_ocr_forbidden"
	case strings.Contains(msg, "llm_ocr_http_"):
		return "llm_ocr_http_error"
	case strings.Contains(msg, "llm_ocr_unavailable"):
		return "llm_ocr_unavailable"
	case strings.Contains(msg, "ocr_disabled"):
		return "ocr_disabled"
	case strings.Contains(msg, "ocr_failed"):
		return "ocr_failed"
	case strings.Contains(msg, "ocr_empty_content"):
		return "ocr_empty_content"
	case strings.Contains(msg, "ocr_unprocessable"):
		return "ocr_unprocessable"
	case strings.Contains(msg, "ocr_unauthorized"):
		return "ocr_unauthorized"
	case strings.Contains(msg, "ocr_forbidden"):
		return "ocr_forbidden"
	case strings.Contains(msg, "ocr_http_"):
		return "ocr_http_error"
	case strings.Contains(msg, "tika_empty_content"):
		return "tika_empty_content"
	case strings.Contains(msg, "tika_unprocessable"):
		return "tika_unprocessable"
	case strings.Contains(msg, "tika_unauthorized"):
		return "tika_unauthorized"
	case strings.Contains(msg, "tika_forbidden"):
		return "tika_forbidden"
	case strings.Contains(msg, "tika_unsupported_media_type"):
		return "tika_unsupported_media_type"
	case strings.Contains(msg, "tika_http_"):
		return "tika_http_error"
	case strings.Contains(msg, "deadline") || strings.Contains(msg, "timeout"):
		return "extract_timeout"
	default:
		return "extract_failed"
	}
}

func resolveProcessingFailure(fileObj *domainconversation.FileObject, err error) (string, string) {
	code := classifyProcessingErrorCode(err)
	return code, resolveProcessingFailureMessage(fileObj, code, err)
}

func resolveProcessingFailureMessage(fileObj *domainconversation.FileObject, code string, err error) string {
	if err == nil {
		return ""
	}
	category := ""
	if fileObj != nil {
		category = fileObj.FileCategory
	}
	return HumanizeFileProcessingError(category, code, err.Error())
}

func HumanizeFileProcessingError(fileCategory string, code string, message string) string {
	raw := strings.TrimSpace(message)
	normalizedCode := strings.ToLower(strings.TrimSpace(code))
	if raw == "" {
		raw = normalizedCode
	}
	lower := strings.ToLower(raw)

	switch normalizedCode {
	case "extract_timeout":
		return "文件提取超时，请稍后重试，或缩小文件后重试。"
	case "tesseract_ocr_disabled":
		return "PDF 未提取到可读文本，且当前未启用 Tesseract OCR。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "tesseract_ocr_failed":
		return "PDF 未提取到可读文本，且 Tesseract OCR 识别失败。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "tesseract_ocr_empty_content":
		return "Tesseract OCR 未识别出可读文本。该 PDF 可能是空白页、图片质量过低，或内容本身不可识别。"
	case "tesseract_ocr_unprocessable":
		detail := strings.TrimSpace(strings.TrimPrefix(raw, "tesseract_ocr_unprocessable:"))
		if detail == "" || detail == raw {
			return "Tesseract OCR 服务无法处理该 PDF。文件可能已损坏、加密，或超出当前 OCR 服务能力。"
		}
		return "Tesseract OCR 服务无法处理该 PDF: " + detail
	case "tesseract_ocr_invalid_response":
		return "Tesseract OCR 服务返回格式不符合当前页级协议，无法合并识别结果。"
	case "tesseract_ocr_unauthorized":
		return "Tesseract OCR 服务鉴权失败，请检查鉴权密钥配置。"
	case "tesseract_ocr_forbidden":
		return "Tesseract OCR 服务拒绝访问，请检查服务端鉴权或访问控制配置。"
	case "tesseract_ocr_http_error":
		if strings.HasPrefix(lower, "tesseract_ocr_http_") {
			if idx := strings.Index(raw, ":"); idx >= 0 {
				codePart := strings.TrimSpace(raw[:idx])
				msgPart := strings.TrimSpace(raw[idx+1:])
				if msgPart == "" {
					return "Tesseract OCR 服务请求失败: " + codePart
				}
				return "Tesseract OCR 服务请求失败: " + codePart + " - " + msgPart
			}
			return "Tesseract OCR 服务请求失败: " + raw
		}
		return "Tesseract OCR 服务请求失败。"
	case "tesseract_ocr_unavailable":
		return "当前 Tesseract OCR 服务不可用，无法从扫描件中提取文本。"
	case "rapidocr_ocr_disabled":
		return "PDF 未提取到可读文本，且当前未启用 RapidOCR。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "rapidocr_ocr_failed":
		return "PDF 未提取到可读文本，且 RapidOCR 识别失败。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "rapidocr_ocr_empty_content":
		return "RapidOCR 未识别出可读文本。该 PDF 可能是空白页、图片质量过低，或内容本身不可识别。"
	case "rapidocr_ocr_unprocessable":
		detail := strings.TrimSpace(strings.TrimPrefix(raw, "rapidocr_ocr_unprocessable:"))
		if detail == "" || detail == raw {
			return "RapidOCR 服务无法处理该 PDF。文件可能已损坏、加密，或超出当前 OCR 服务能力。"
		}
		return "RapidOCR 服务无法处理该 PDF: " + detail
	case "rapidocr_ocr_invalid_response":
		return "RapidOCR 服务返回格式不符合当前页级协议，无法合并识别结果。"
	case "rapidocr_ocr_unauthorized":
		return "RapidOCR 服务鉴权失败，请检查鉴权密钥配置。"
	case "rapidocr_ocr_forbidden":
		return "RapidOCR 服务拒绝访问，请检查服务端鉴权或访问控制配置。"
	case "rapidocr_ocr_http_error":
		if strings.HasPrefix(lower, "rapidocr_ocr_http_") {
			if idx := strings.Index(raw, ":"); idx >= 0 {
				codePart := strings.TrimSpace(raw[:idx])
				msgPart := strings.TrimSpace(raw[idx+1:])
				if msgPart == "" {
					return "RapidOCR 服务请求失败: " + codePart
				}
				return "RapidOCR 服务请求失败: " + codePart + " - " + msgPart
			}
			return "RapidOCR 服务请求失败: " + raw
		}
		return "RapidOCR 服务请求失败。"
	case "rapidocr_ocr_unavailable":
		return "当前 RapidOCR 服务不可用，无法从扫描件中提取文本。"
	case "llm_ocr_disabled":
		return "PDF 未提取到可读文本，且当前未启用 LLM OCR。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "llm_ocr_failed":
		return "PDF 未提取到可读文本，且 LLM OCR 识别失败。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "llm_ocr_empty_content":
		return "LLM OCR 未识别出可读文本。该 PDF 可能是空白页、图片质量过低，或内容本身不可识别。"
	case "llm_ocr_unprocessable":
		detail := strings.TrimSpace(strings.TrimPrefix(raw, "llm_ocr_unprocessable:"))
		if detail == "" || detail == raw {
			return "LLM OCR 服务无法处理该 PDF。文件可能已损坏、加密，或超出当前 OCR 服务能力。"
		}
		return "LLM OCR 服务无法处理该 PDF: " + detail
	case "llm_ocr_invalid_response":
		return "LLM OCR 服务返回格式不符合当前页级协议，无法合并识别结果。"
	case "llm_ocr_unauthorized":
		return "LLM OCR 服务鉴权失败，请检查鉴权密钥配置。"
	case "llm_ocr_forbidden":
		return "LLM OCR 服务拒绝访问，请检查服务端鉴权或访问控制配置。"
	case "llm_ocr_http_error":
		if strings.HasPrefix(lower, "llm_ocr_http_") {
			if idx := strings.Index(raw, ":"); idx >= 0 {
				codePart := strings.TrimSpace(raw[:idx])
				msgPart := strings.TrimSpace(raw[idx+1:])
				if msgPart == "" {
					return "LLM OCR 服务请求失败: " + codePart
				}
				return "LLM OCR 服务请求失败: " + codePart + " - " + msgPart
			}
			return "LLM OCR 服务请求失败: " + raw
		}
		return "LLM OCR 服务请求失败。"
	case "llm_ocr_unavailable":
		return "当前 LLM OCR 服务不可用，无法从扫描件中提取文本。"
	case "ocr_disabled":
		return "PDF 未提取到可读文本，且当前未启用 OCR。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "ocr_failed":
		return "PDF 未提取到可读文本，且 OCR 识别失败。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "ocr_empty_content":
		return "OCR 未识别出可读文本。该 PDF 可能是空白页、图片质量过低，或内容本身不可识别。"
	case "ocr_unprocessable":
		detail := strings.TrimSpace(strings.TrimPrefix(raw, "ocr_unprocessable:"))
		if detail == "" || detail == raw {
			return "OCR 服务无法处理该 PDF。文件可能已损坏、加密，或超出当前 OCR 服务能力。"
		}
		return "OCR 服务无法处理该 PDF: " + detail
	case "ocr_unauthorized":
		return "OCR 服务鉴权失败，请检查 OCR 鉴权密钥配置。"
	case "ocr_forbidden":
		return "OCR 服务拒绝访问，请检查服务端鉴权或访问控制配置。"
	case "ocr_http_error":
		if strings.HasPrefix(lower, "ocr_http_") {
			if idx := strings.Index(raw, ":"); idx >= 0 {
				codePart := strings.TrimSpace(raw[:idx])
				msgPart := strings.TrimSpace(raw[idx+1:])
				if msgPart == "" {
					return "OCR 服务请求失败: " + codePart
				}
				return "OCR 服务请求失败: " + codePart + " - " + msgPart
			}
			return "OCR 服务请求失败: " + raw
		}
		return "OCR 服务请求失败。"
	case "tika_empty_content":
		if strings.ToLower(strings.TrimSpace(fileCategory)) == "pdf" {
			return "Tika 未提取到可读文本。该 PDF 可能是扫描件、图片型 PDF、加密 PDF，或文档内容本身不可复制。"
		}
		return "Tika 未提取到可读文本。文件内容可能主要为图片、空白页，或文档本身不含可复制文本。"
	case "tika_unprocessable":
		detail := strings.TrimSpace(strings.TrimPrefix(raw, "tika_unprocessable:"))
		if detail == "" || detail == raw {
			return "Tika 无法处理该文件。文件可能已损坏、加密，或格式超出当前解析能力。"
		}
		return "Tika 无法处理该文件: " + detail
	case "tika_unauthorized":
		return "Tika 服务鉴权失败，请检查 Tika Token 配置。"
	case "tika_forbidden":
		return "Tika 服务拒绝访问，请检查服务端鉴权或访问控制配置。"
	case "tika_unsupported_media_type":
		return "Tika 不支持当前文件类型或 MIME 类型。"
	case "tika_http_error":
		detail := raw
		if strings.HasPrefix(lower, "tika_http_") {
			if idx := strings.Index(raw, ":"); idx >= 0 {
				codePart := strings.TrimSpace(raw[:idx])
				msgPart := strings.TrimSpace(raw[idx+1:])
				if msgPart == "" {
					return "Tika 服务请求失败: " + codePart
				}
				return "Tika 服务请求失败: " + codePart + " - " + msgPart
			}
			return "Tika 服务请求失败: " + raw
		}
		if detail == "" {
			return "Tika 服务请求失败。"
		}
		return "Tika 服务请求失败: " + detail
	}

	if strings.ToLower(strings.TrimSpace(fileCategory)) == "pdf" {
		switch {
		case raw == "extract_failed", raw == "pdf_no_extractable_text":
			return "PDF 未提取到可读文本。该文件可能是扫描件、图片型 PDF、加密 PDF，或文档内容本身不可复制。"
		case raw == "ocr_unavailable":
			return "PDF 未提取到可读文本，且当前 OCR 服务不可用。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
		case strings.HasPrefix(lower, "pdf_parse_failed:"):
			detail := strings.TrimSpace(raw[len("pdf_parse_failed:"):])
			if detail == "" {
				return "PDF 解析失败，请检查文件是否损坏、加密，或格式是否异常。"
			}
			return "PDF 解析失败: " + detail
		}
	}

	switch raw {
	case "extract_failed":
		return "无法提取文本，请检查文件是否损坏、加密，或内容是否主要为图片。"
	case "ocr_unavailable":
		return "当前 OCR 服务不可用，无法从扫描件中提取文本。"
	}

	return raw
}

func compactSnippet(content string, maxLen int) string {
	value := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if value == "" {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 120
	}
	runes := []rune(value)
	if len(runes) <= maxLen {
		return value
	}
	return string(runes[:maxLen]) + "..."
}

func truncateError(message string, limit int) string {
	value := strings.TrimSpace(message)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func supportsExtraction(category string) bool {
	switch category {
	case "pdf", "word", "excel", "text", "image":
		return true
	default:
		return false
	}
}

func supportsRAG(category string) bool {
	switch category {
	case "pdf", "word", "excel", "text", "image":
		return true
	default:
		return false
	}
}
