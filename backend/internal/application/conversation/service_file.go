package conversation

import (
	"context"
	"errors"
	"strings"

	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
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
	s.embeddingSvc.MaybeTrigger(*target)
}

// ListFiles 分页查询用户文件。
func (s *Service) ListFiles(
	ctx context.Context,
	userID uint,
	page int,
	pageSize int,
	searchQuery string,
	filterKind string,
	sortBy string,
) (*appupload.ListFilesResult, error) {
	return s.uploadSvc.ListFiles(ctx, userID, page, pageSize, searchQuery, filterKind, sortBy)
}

// UploadFile 上传文件并扣减用户配额。
func (s *Service) UploadFile(ctx context.Context, input appupload.UploadFileInput) (*appupload.UploadFileResult, error) {
	return s.uploadSvc.UploadFile(ctx, input)
}

// DeleteFile 删除文件并回收配额。
func (s *Service) DeleteFile(ctx context.Context, userID uint, fileID string) (*appupload.DeleteFileResult, error) {
	return s.uploadSvc.DeleteFile(ctx, userID, fileID)
}

// RenameFile 重命名当前用户文件。
func (s *Service) RenameFile(ctx context.Context, userID uint, fileID string, fileName string) (*model.FileObject, error) {
	return s.uploadSvc.RenameFile(ctx, userID, fileID, fileName)
}

// UpdateFileRagOptOut 更新文件的 RAG 检索开关。
func (s *Service) UpdateFileRagOptOut(ctx context.Context, userID uint, fileID string, ragOptOut bool) (*model.FileObject, error) {
	return s.uploadSvc.UpdateFileRagOptOut(ctx, userID, fileID, ragOptOut)
}

// OpenFileContent 打开当前用户的文件内容。
func (s *Service) OpenFileContent(ctx context.Context, userID uint, fileID string) (*appupload.FileContentResult, error) {
	return s.uploadSvc.OpenFileContent(ctx, userID, fileID)
}

// GetFileExtract 读取当前用户文件的提取文本产物。
func (s *Service) GetFileExtract(ctx context.Context, userID uint, fileID string) (*FileExtractResult, error) {
	normalizedFileID := strings.TrimSpace(fileID)
	if normalizedFileID == "" {
		return nil, ErrInvalidFileReference
	}
	fileObj, err := s.repo.GetActiveFileObjectByID(ctx, userID, normalizedFileID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
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
