package conversation

import (
	"context"
	"errors"
	"strings"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

// FileExtractResult 表示当前用户可读取的文件提取文本。
type FileExtractResult struct {
	FileID       string
	ExtractText  string
	PreviewText  string
	ExtractChars int
	ExtractPages int
	OCRUsed      bool
}

func (s *Service) cloneOrTriggerEmbedding(ctx context.Context, source *model.FileObject, target *model.FileObject) {
	if target == nil {
		return
	}
	if source != nil && source.EmbedStatus == "ready" && source.ChunkCount > 0 {
		if err := s.repo.CloneFileEmbeddingArtifacts(ctx, source, target); err == nil {
			return
		} else if s.logger != nil {
			s.logger.Warn("clone_embedding_artifacts_failed",
				zap.String("source_file_id", source.FileID),
				zap.String("target_file_id", target.FileID),
				zap.Error(err),
			)
		}
	}
	s.embeddingSvc.MaybeTrigger(ctx, *target)
}

// GetFileExtract 读取当前用户文件的提取文本产物。
func (s *Service) GetFileExtract(ctx context.Context, userID uint, fileID string) (*FileExtractResult, error) {
	normalizedFileID := strings.TrimSpace(fileID)
	if normalizedFileID == "" {
		return nil, ErrInvalidFileReference
	}
	fileObj, err := s.repo.GetActiveFileObjectByID(ctx, userID, normalizedFileID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrFileNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	if fileObj == nil {
		return nil, ErrFileNotFound
	}

	result, err := s.repo.GetFileObjectProcessingByObjectID(ctx, fileObj.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrFileProcessingNotReady
		}
		return nil, err
	}
	if result == nil || strings.TrimSpace(result.ExtractStoragePath) == "" || strings.TrimSpace(fileObj.ExtractStatus) != "ready" {
		return nil, ErrFileProcessingNotReady
	}
	if s.extractSvc == nil {
		return nil, ErrFileProcessingNotReady
	}

	text, err := s.extractSvc.ReadExtractedText(ctx, result.ExtractStoragePath)
	if err != nil {
		return nil, err
	}
	return &FileExtractResult{
		FileID:       fileObj.FileID,
		ExtractText:  text,
		PreviewText:  result.PreviewText,
		ExtractChars: result.ExtractChars,
		ExtractPages: result.ExtractPages,
		OCRUsed:      result.OCRUsed,
	}, nil
}
