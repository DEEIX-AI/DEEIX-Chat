package processing

import (
	"context"
	"errors"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// EnsureFileCreateProcessing resumes the existing pipeline after receipt commit.
// A conditional transition cannot reset a progressed file. Repeated queue
// delivery is safe because processing workers already claim with attempt IDs.
func (s *Service) EnsureFileCreateProcessing(ctx context.Context, file *domainconversation.FileObject) error {
	return s.ensureUploadedFileProcessing(ctx, file, false)
}

func (s *Service) ensureUploadedFileProcessing(ctx context.Context, file *domainconversation.FileObject, terminalQueueError bool) error {
	if file == nil {
		return nil
	}
	if file.ProcessingStatus != "uploaded" && file.ProcessingStatus != "queued" {
		return nil
	}
	if file.ProcessingStatus == "uploaded" {
		now := time.Now()
		state := &domainconversation.FileObjectProcessing{
			FileObjectID: file.ID, UserID: file.UserID, DetectedMIME: file.DetectedMIME,
			FileCategory: file.FileCategory, ProcessingStatus: "queued", ExtractStatus: "none",
			ExtractorVersion: s.version(), StartedAt: &now,
		}
		if file.FileCategory == "video" || file.FileCategory == "audio" || (file.FileCategory == "image" && !s.snapshot().ExtractImageOCREnabled) {
			state = s.readyWithoutExtractionState(file, file.FileCategory+"_not_applicable")
		} else if !supportsExtraction(file.FileCategory) {
			state.ProcessingStatus = "failed"
			state.ErrorCode = "mime_blocked"
			state.ErrorMessage = "unsupported file category"
		}
		state.ExpectedStatus = "uploaded"
		if err := s.repo.UpdateFileObjectProcessingState(ctx, state); err != nil {
			if errors.Is(err, repository.ErrConflict) {
				return nil
			}
			return err
		}
		file.ProcessingStatus = state.ProcessingStatus
		file.ProcessingReady = state.ProcessingReady
		file.ExtractStatus = state.ExtractStatus
		if state.ProcessingStatus != "queued" {
			return nil
		}
	}
	err := s.enqueueFileProcessing(ctx, file.UserID, file.FileID, 0, "")
	if err != nil && terminalQueueError {
		code := "queue_unavailable"
		if errors.Is(err, repository.ErrFileProcessingQueueFull) {
			code = "queue_full"
		}
		state := s.failedFileProcessingState(file, code, HumanizeFileProcessingError(file.FileCategory, code, ""))
		state.ExpectedStatus = "queued"
		_ = s.repo.UpdateFileObjectProcessingState(ctx, state)
	}
	return err
}
