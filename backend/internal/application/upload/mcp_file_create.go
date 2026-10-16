package upload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/objectstorage"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// FileCreateStoragePath derives a stable candidate key before object I/O.
func (s *Service) FileCreateStoragePath(ctx context.Context, grant *domainmcp.FileCreateGrant, prepared *PreparedTemporaryFile) (string, error) {
	user, err := s.repo.GetUserByID(ctx, grant.UserID)
	if err != nil {
		return "", err
	}
	owner := user.PublicID
	if owner == "" {
		owner = fmt.Sprintf("uid_%d", grant.UserID)
	}
	return filepath.ToSlash(filepath.Join(owner, grant.CreatedAt.Format("2006"), grant.CreatedAt.Format("01"), grant.CandidateID+"_"+prepared.FileName)), nil
}

// CommitFileCreate uses the caller's receipt transaction for file/quota settlement.
// The candidate key is already durably bound to the exact bytes. Never delete it
// on an uncertain storage/transaction error: replay reconciles that same key.
func (s *Service) CommitFileCreate(ctx context.Context, repo repository.UploadRepository, grant *domainmcp.FileCreateGrant, prepared *PreparedTemporaryFile) (*UploadFileResult, error) {
	cfg := s.snapshot()
	store, err := s.openObjectStore(ctx)
	if err != nil {
		return nil, err
	}
	existing, err := repo.GetLatestActiveFileObjectBySHA(ctx, grant.UserID, prepared.SHA256, prepared.SizeBytes)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		matches, matchErr := objectMatchesContent(ctx, store, existing.StoragePath, prepared.SHA256, prepared.SizeBytes)
		if matchErr != nil {
			return nil, matchErr
		}
		// A create-only capability must not repair/delete another file.
		if !matches {
			return nil, repository.ErrFileCreateConflict
		}
		quota, quotaErr := repo.GetOrInitUserStorageQuota(ctx, grant.UserID, cfg.UserStorageQuotaBytes)
		if quotaErr != nil {
			return nil, quotaErr
		}
		return &UploadFileResult{File: *existing, Quota: *quota, Reused: true}, nil
	}
	quota, err := repo.GetOrInitUserStorageQuota(ctx, grant.UserID, cfg.UserStorageQuotaBytes)
	if err != nil {
		return nil, err
	}
	if quota.QuotaBytes > 0 && quota.UsedBytes+quota.ReservedBytes+prepared.SizeBytes > quota.QuotaBytes {
		return nil, s.errStorageQuotaExceeded()
	}
	matches, err := objectMatchesContent(ctx, store, grant.StoragePath, prepared.SHA256, prepared.SizeBytes)
	if err != nil {
		return nil, err
	}
	if !matches {
		if err = putPreparedFile(ctx, store, grant.StoragePath, prepared); err != nil {
			return nil, err
		}
		matches, err = objectMatchesContent(ctx, store, grant.StoragePath, prepared.SHA256, prepared.SizeBytes)
		if err != nil {
			return nil, err
		}
		if !matches {
			return nil, errors.New("file create candidate content verification failed")
		}
	}
	file := s.fileObjectFromPrepared(grant.CandidateID, grant.UserID, "mcp_output", grant.StoragePath, prepared)
	quota, err = repo.CreateFileObjectAndConsumeQuota(ctx, file, cfg.UserStorageQuotaBytes)
	if err != nil {
		if errors.Is(err, repository.ErrStorageQuotaExceeded) {
			return nil, s.errStorageQuotaExceeded()
		}
		return nil, err
	}
	return &UploadFileResult{File: *file, Quota: *quota}, nil
}

func putPreparedFile(ctx context.Context, store objectstorage.Store, key string, prepared *PreparedTemporaryFile) error {
	file, err := os.Open(prepared.AbsolutePath)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck
	_, err = store.Put(ctx, key, file, objectstorage.PutOptions{SizeBytes: prepared.SizeBytes, ContentType: prepared.DetectedMIME})
	return err
}

func (s *Service) fileObjectFromPrepared(fileID string, userID uint, purpose string, key string, prepared *PreparedTemporaryFile) *domainconversation.FileObject {
	cfg := s.snapshot()
	category := prepared.FileCategory
	return &domainconversation.FileObject{
		FileID: fileID, UserID: userID, Purpose: normalizePurpose(purpose), FileName: prepared.FileName,
		MimeType: prepared.MimeType, DetectedMIME: prepared.DetectedMIME, FileCategory: category,
		SizeBytes: prepared.SizeBytes, SHA256: prepared.SHA256, StoragePath: key, Status: "active",
		ProcessingStatus: "uploaded", ProcessingReady: category == fileCategoryVideo || category == fileCategoryAudio || (category == fileCategoryImage && !cfg.ExtractImageOCREnabled),
		ExtractStatus: "none", EmbedStatus: "none", ExtractorVersion: s.resolveExtractorVersion(),
	}
}

// RemoveFileCreateCandidate is only called under an expired receipt's lock.
func (s *Service) RemoveFileCreateCandidate(ctx context.Context, key string) error {
	store, err := s.openObjectStore(ctx)
	if err != nil {
		return err
	}
	err = store.Delete(ctx, key)
	if errors.Is(err, objectstorage.ErrNotFound) {
		return nil
	}
	return err
}

// FinishFileCreateProcessing reuses the existing processing hook then reloads
// persisted state; creation does not imply extraction/OCR/embedding completed.
func (s *Service) FinishFileCreateProcessing(ctx context.Context, userID uint, fileID string) (*domainconversation.FileObject, error) {
	file, err := s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
	if err != nil {
		return nil, err
	}
	if (file.ProcessingStatus == "uploaded" || file.ProcessingStatus == "queued") && s.hooks.EnsureFileCreateProcessing != nil {
		initCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		_ = s.hooks.EnsureFileCreateProcessing(initCtx, file)
		cancel()
		return s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
	}
	return file, nil
}
