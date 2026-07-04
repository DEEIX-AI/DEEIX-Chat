package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	appprocessing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/processing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	contextArtifactCleanupInterval = 24 * time.Hour
	contextArtifactCleanupBatch    = 1000
)

// StartBackgroundWorkers 启动文件处理后台 worker，ctx 取消时停止。
func (s *Service) StartBackgroundWorkers(ctx context.Context) {
	if s == nil {
		return
	}
	if s.processingSvc != nil {
		s.processingSvc.StartBackgroundWorkers(ctx)
	}
	s.startInMemoryCacheCleanupWorker(ctx)
	s.startContextArtifactCleanupWorker(ctx)
}

func (s *Service) startContextArtifactCleanupWorker(ctx context.Context) {
	if s == nil || s.repo == nil {
		return
	}
	go func() {
		s.deleteExpiredContextArtifacts(ctx)
		ticker := time.NewTicker(contextArtifactCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.deleteExpiredContextArtifacts(ctx)
			}
		}
	}()
}

func (s *Service) deleteExpiredContextArtifacts(ctx context.Context) {
	if s == nil || s.cfg == nil || s.cfg.Snapshot().ContextArtifactRetentionDays <= 0 {
		return
	}
	for {
		cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		deleted, err := s.repo.DeleteExpiredContextArtifacts(cleanupCtx, time.Now(), contextArtifactCleanupBatch)
		cancel()
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("context_artifact_cleanup_failed", zap.Error(err))
			}
			return
		}
		if deleted == 0 || deleted < contextArtifactCleanupBatch {
			return
		}
	}
}

func (s *Service) GetChatFilePolicy(ctx context.Context, userID uint) (*ChatFilePolicyDTO, error) {
	cfg := s.cfg.Snapshot()
	capability := s.resolveChatFileCapability(ctx)
	fileMode := "auto"
	if userID != 0 {
		if value, err := s.repo.GetUserSettingValue(ctx, userID, "chat.file_mode"); err == nil && strings.TrimSpace(value) != "" {
			fileMode = strings.TrimSpace(value)
		}
	}
	return &ChatFilePolicyDTO{
		MaxMessageFiles:        cfg.MaxMessageFiles,
		MaxUploadFileBytes:     cfg.MaxUploadFileBytes,
		AllowedMIMETypes:       sortedAllowedMIMETypes(cfg.FileAllowedMIMETypes),
		ImageMaxBytes:          cfg.FileImageMaxBytes,
		DocMaxBytes:            cfg.FileDocMaxBytes,
		EffectiveImageMaxBytes: capability.EffectiveImageMaxBytes,
		EffectiveDocMaxBytes:   capability.EffectiveDocMaxBytes,
		FullContextMaxBytes:    cfg.FileFullContextMaxBytes,
		FullContextMaxTokens:   cfg.FileFullContextMaxTokens,
		FullContextPDFMaxPages: cfg.FileFullContextPDFMaxPages,
		RAGAvailable:           capability.RAGAvailable,
		RAGAvailabilityReason:  capability.RAGAvailabilityReason,
		CapabilityMode:         capability.CapabilityMode,
		FileMode:               fileMode,
	}, nil
}

func (s *Service) GetFileProcessingStatus(ctx context.Context, userID uint, fileID string) (*appprocessing.FileProcessingStatusDTO, error) {
	result, err := s.processingSvc.GetFileProcessingStatus(ctx, userID, fileID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrFileNotFound
	}
	return result, err
}

func (s *Service) resolveAttachments(
	ctx context.Context,
	userID uint,
	fileIDs []string,
) ([]AttachmentInput, error) {
	normalizedIDs := make([]string, 0, len(fileIDs))
	seen := make(map[string]struct{}, len(fileIDs))
	for _, rawID := range fileIDs {
		fileID := strings.TrimSpace(rawID)
		if fileID == "" {
			continue
		}
		if _, exists := seen[fileID]; exists {
			continue
		}
		seen[fileID] = struct{}{}
		normalizedIDs = append(normalizedIDs, fileID)
	}

	resolved := make([]AttachmentInput, 0, len(normalizedIDs))
	if len(normalizedIDs) > 0 {
		fileObjects, err := s.repo.GetActiveFileObjectsByIDs(ctx, userID, normalizedIDs)
		if err != nil {
			return nil, err
		}
		if len(fileObjects) != len(normalizedIDs) {
			return nil, ErrInvalidFileReference
		}

		fileMap := make(map[string]model.FileObject, len(fileObjects))
		for _, item := range fileObjects {
			fileMap[item.FileID] = item
		}
		for _, fileID := range normalizedIDs {
			fileItem, ok := fileMap[fileID]
			if !ok {
				return nil, ErrInvalidFileReference
			}
			resolved = append(resolved, AttachmentInput{
				FileObjID:              fileItem.ID,
				FileID:                 fileItem.FileID,
				Kind:                   inferAttachmentKind(fileItem.DetectedMIME),
				FileName:               fileItem.FileName,
				MimeType:               fileItem.MimeType,
				DetectedMIME:           fileItem.DetectedMIME,
				FileCategory:           fileItem.FileCategory,
				FileSize:               fileItem.SizeBytes,
				SHA256:                 fileItem.SHA256,
				StoragePath:            fileItem.StoragePath,
				MetaJSON:               "",
				PageCount:              fileItem.PageCount,
				ProcessingStatus:       fileItem.ProcessingStatus,
				ProcessingReady:        fileItem.ProcessingReady,
				ProcessingErrorCode:    fileItem.ProcessingErrorCode,
				ProcessingErrorMessage: fileItem.ProcessingErrorMessage,
				ExtractStatus:          fileItem.ExtractStatus,
				EmbedStatus:            fileItem.EmbedStatus,
				RagOptOut:              fileItem.RagOptOut,
				ChunkCount:             fileItem.ChunkCount,
			})
		}
	}

	return resolved, nil
}

func (s *Service) resolveConversationFileContext(
	ctx context.Context,
	userID uint,
	fileIDs []string,
	currentFileIDs []string,
) ([]AttachmentInput, error) {
	normalizedIDs := make([]string, 0, len(fileIDs))
	seen := make(map[string]struct{}, len(fileIDs))
	for _, rawID := range fileIDs {
		fileID := strings.TrimSpace(rawID)
		if fileID == "" {
			continue
		}
		if _, exists := seen[fileID]; exists {
			continue
		}
		seen[fileID] = struct{}{}
		normalizedIDs = append(normalizedIDs, fileID)
	}
	if len(normalizedIDs) == 0 {
		return nil, nil
	}

	current := make(map[string]struct{}, len(currentFileIDs))
	for _, rawID := range currentFileIDs {
		fileID := strings.TrimSpace(rawID)
		if fileID != "" {
			current[fileID] = struct{}{}
		}
	}

	fileObjects, err := s.repo.GetActiveFileObjectsByIDs(ctx, userID, normalizedIDs)
	if err != nil {
		return nil, err
	}
	fileMap := make(map[string]model.FileObject, len(fileObjects))
	for _, item := range fileObjects {
		fileMap[item.FileID] = item
	}

	resolved := make([]AttachmentInput, 0, len(fileObjects))
	for _, fileID := range normalizedIDs {
		fileItem, ok := fileMap[fileID]
		if !ok {
			if _, required := current[fileID]; required {
				return nil, ErrInvalidFileReference
			}
			continue
		}
		_, isCurrent := current[fileID]
		resolved = append(resolved, AttachmentInput{
			FileObjID:              fileItem.ID,
			FileID:                 fileItem.FileID,
			Kind:                   inferAttachmentKind(fileItem.DetectedMIME),
			FileName:               fileItem.FileName,
			MimeType:               fileItem.MimeType,
			DetectedMIME:           fileItem.DetectedMIME,
			FileCategory:           fileItem.FileCategory,
			FileSize:               fileItem.SizeBytes,
			SHA256:                 fileItem.SHA256,
			StoragePath:            fileItem.StoragePath,
			MetaJSON:               "",
			PageCount:              fileItem.PageCount,
			ProcessingStatus:       fileItem.ProcessingStatus,
			ProcessingReady:        fileItem.ProcessingReady,
			ProcessingErrorCode:    fileItem.ProcessingErrorCode,
			ProcessingErrorMessage: fileItem.ProcessingErrorMessage,
			ExtractStatus:          fileItem.ExtractStatus,
			EmbedStatus:            fileItem.EmbedStatus,
			RagOptOut:              fileItem.RagOptOut,
			ChunkCount:             fileItem.ChunkCount,
			Current:                isCurrent,
		})
	}
	return resolved, nil
}

func (s *Service) hydrateAttachmentsForSend(
	ctx context.Context,
	userID uint,
	attachments []AttachmentInput,
	onEvent func(string, map[string]interface{}) error,
) ([]AttachmentInput, error) {
	if len(attachments) == 0 {
		return attachments, nil
	}

	// 多文件并行等待：每个文件独立 WaitUntilReady，总耗时 = max(单个文件) 而非 sum。
	// 本轮图片会作为 image part 直传；历史图片仅在 OCR 开启时等待提取文本。
	items := make([]AttachmentInput, len(attachments))
	for i, att := range attachments {
		items[i] = att // 预置，图片/空 FileID 直接保留
	}

	// mu 保护 onEvent 的并发调用（onEvent 非 goroutine-safe）。
	var mu sync.Mutex
	g, gCtx := errgroup.WithContext(ctx)

	cfg := config.Config{}
	if s != nil && s.cfg != nil {
		cfg = s.cfg.Snapshot()
	}
	for i, att := range attachments {
		if strings.TrimSpace(att.FileID) == "" ||
			(att.FileCategory == fileCategoryImage && (att.Current || !cfg.ExtractImageOCREnabled)) {
			continue
		}
		i, att := i, att // 闭包捕获
		g.Go(func() error {
			var latestFile *model.FileObject
			readyFile, err := s.processingSvc.WaitUntilReady(gCtx, userID, att.FileID, func(fileObj *model.FileObject) {
				if fileObj == nil {
					return
				}
				latestFile = fileObj
				mu.Lock()
				defer mu.Unlock()
				emitEvent(onEvent, "process_update", map[string]interface{}{
					"status": "streaming",
				})
			})
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return err
				}
				if !att.Current || att.FileCategory == fileCategoryImage {
					if latestFile != nil {
						items[i].ProcessingStatus = latestFile.ProcessingStatus
						items[i].ProcessingReady = latestFile.ProcessingReady
						items[i].ProcessingErrorCode = latestFile.ProcessingErrorCode
						items[i].ProcessingErrorMessage = latestFile.ProcessingErrorMessage
						items[i].ExtractStatus = latestFile.ExtractStatus
						items[i].EmbedStatus = latestFile.EmbedStatus
					}
					return nil
				}
				return err
			}
			items[i].FileObjID = readyFile.File.ID
			items[i].DetectedMIME = readyFile.DetectedMIME
			items[i].FileCategory = readyFile.FileCategory
			items[i].PageCount = readyFile.PageCount
			items[i].ProcessingStatus = readyFile.File.ProcessingStatus
			items[i].ProcessingReady = readyFile.File.ProcessingReady
			items[i].ProcessingErrorCode = readyFile.File.ProcessingErrorCode
			items[i].ProcessingErrorMessage = readyFile.File.ProcessingErrorMessage
			items[i].ExtractStatus = readyFile.ExtractStatus
			items[i].EmbedStatus = readyFile.EmbedStatus
			items[i].ExtractedText = readyFile.ExtractedText
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		switch {
		case errors.Is(err, appprocessing.ErrFileProcessingFailed):
			return nil, fmt.Errorf("%w: %s", ErrFileProcessingNotReady, err.Error())
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return nil, err
		default:
			return nil, ErrInvalidFileReference
		}
	}
	return items, nil
}

func canUseAttachmentFullContext(att AttachmentInput, cfg config.Config) bool {
	if att.FileCategory == fileCategoryVideo {
		return false
	}
	text := strings.TrimSpace(att.ExtractedText)
	if text == "" {
		return false
	}
	if cfg.FileFullContextMaxBytes > 0 && int64(len([]byte(text))) > cfg.FileFullContextMaxBytes {
		return false
	}
	if cfg.FileFullContextMaxTokens > 0 && estimateTokens(text) > int64(cfg.FileFullContextMaxTokens) {
		return false
	}
	if att.FileCategory == fileCategoryPDF && cfg.FileFullContextPDFMaxPages > 0 && att.PageCount > cfg.FileFullContextPDFMaxPages {
		return false
	}
	return true
}

func buildFileAttachmentSnapshot(att AttachmentInput) map[string]interface{} {
	return map[string]interface{}{
		"file_id":                  att.FileID,
		"kind":                     att.Kind,
		"file_name":                att.FileName,
		"mime_type":                att.MimeType,
		"detected_mime":            att.DetectedMIME,
		"file_category":            att.FileCategory,
		"file_size":                att.FileSize,
		"processing_status":        att.ProcessingStatus,
		"processing_ready":         att.ProcessingReady,
		"processing_error_code":    att.ProcessingErrorCode,
		"processing_error_message": att.ProcessingErrorMessage,
	}
}

func marshalAttachmentSnapshots(items []AttachmentInput) string {
	payload := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		payload = append(payload, buildFileAttachmentSnapshot(item))
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func classifyProcessingErrorCode(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "ocr_disabled"):
		return "ocr_disabled"
	case strings.Contains(msg, "ocr_failed"):
		return "ocr_failed"
	case strings.Contains(msg, "deadline") || strings.Contains(msg, "timeout"):
		return "extract_timeout"
	default:
		return "extract_failed"
	}
}

func isProcessingNotFound(err error) bool {
	return errors.Is(err, repository.ErrNotFound)
}
