package upload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/filetype"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/objectstore"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var errLocalFileTooLarge = errors.New("local file too large")

// Hooks 封装上传后续编排动作。
type Hooks struct {
	InitializeUploadedFile func(ctx context.Context, file *domainconversation.FileObject) error
}

// ErrorSet 允许上层注入统一错误语义。
type ErrorSet struct {
	InvalidFileReference error
	InvalidFileName      error
	FileNotFound         error
	FileInUse            error
	StorageQuotaExceeded error
	FileTooLarge         error
	MIMEBlocked          error
	DangerousMIMEType    error
}

// Service 封装文件上传、配额和物理存储能力。
type Service struct {
	cfg              *config.Runtime
	repo             repository.UploadRepository
	logger           *zap.Logger
	hooks            Hooks
	errors           ErrorSet
	extractorVersion string
	storeProvider    appstorage.Provider
	uploadGatesMu    sync.Mutex
	uploadGates      map[string]*uploadContentGate
}

// uploadContentGate 同一内容（用户+SHA+大小）上传的互斥闸门：token 为容量 1 的许可通道，users 记录等待者数量用于回收。
type uploadContentGate struct {
	token chan struct{}
	users int
}

// UploadFileInput 定义文件上传请求。
type UploadFileInput struct {
	UserID       uint
	Ownership    FileOwnership
	Purpose      string
	FileName     string
	MimeType     string
	DeclaredSize int64
	Reader       io.Reader
}

// FileOwnership 定义文件所属的存储账户。
// 用户文件计入上传者额度；平台文件使用保留的 owner ID 0 和独立的无限额平台账户。
type FileOwnership string

const (
	FileOwnershipUser   FileOwnership = "user"
	FileOwnershipSystem FileOwnership = "system"
)

// UploadFileResult 定义文件上传结果。
type UploadFileResult struct {
	File   domainconversation.FileObject
	Quota  domainconversation.StorageQuota
	Reused bool
}

// DeleteFileResult 定义文件删除结果。
type DeleteFileResult struct {
	Deleted bool
	FileID  string
	Quota   domainconversation.StorageQuota
}

// ListFilesResult 定义文件列表和当前用户存储配额。
type ListFilesResult struct {
	Items []domainconversation.FileObject
	Total int64
	Quota domainconversation.StorageQuota
}

// ListFilesInput 定义用户文件列表的分页、筛选与排序条件。
type ListFilesInput struct {
	UserID      uint
	Page        int
	PageSize    int
	SearchQuery string
	FilterKind  string
	SortBy      string
}

type saveUploadedFileInput struct {
	Store          objectstore.Store
	Reader         io.Reader
	UserPublicID   string
	FileID         string
	FileName       string
	MaxUploadBytes int64
	DeclaredMIME   string
}

// FileContentResult 定义文件内容读取结果。
type FileContentResult struct {
	File        domainconversation.FileObject
	Reader      io.ReadCloser
	ContentType string
	SizeBytes   int64
	ModTime     time.Time
}

// NewServiceWithRuntime 创建使用运行时配置容器的上传服务。
func NewServiceWithRuntime(cfg *config.Runtime, repo repository.UploadRepository, logger *zap.Logger, hooks Hooks, errors ErrorSet, extractorVersion string) *Service {
	return &Service{
		cfg:              cfg,
		repo:             repo,
		logger:           logger,
		hooks:            hooks,
		errors:           errors,
		extractorVersion: strings.TrimSpace(extractorVersion),
		uploadGates:      make(map[string]*uploadContentGate),
	}
}

// SetObjectStoreProvider 注入对象存储 provider，供应用装配层替换具体实现。
func (s *Service) SetObjectStoreProvider(provider appstorage.Provider) {
	if provider != nil {
		s.storeProvider = provider
	}
}

func (s *Service) openObjectStore(ctx context.Context) (objectstore.Store, error) {
	if s == nil || s.storeProvider == nil {
		return nil, appstorage.ErrProviderNotConfigured
	}
	return s.storeProvider.Open(ctx)
}

const (
	embeddingTimeoutStaleAfter = 6 * time.Minute
)

// ListFiles 分页查询用户文件。
func (s *Service) ListFiles(ctx context.Context, input ListFilesInput) (*ListFilesResult, error) {
	offset, limit := pagination.Offset(input.Page, input.PageSize)
	_, _ = s.repo.MarkTimedOutFileEmbeddingsFailed(
		ctx,
		input.UserID,
		time.Now().Add(-embeddingTimeoutStaleAfter),
		"向量化超时，请检查向量化服务配置后重试",
	)
	items, total, err := s.repo.ListFileObjectsByUserWithFilter(ctx, repository.ListFileObjectsInput{
		UserID:      input.UserID,
		Offset:      offset,
		Limit:       limit,
		SearchQuery: input.SearchQuery,
		FilterKind:  input.FilterKind,
		SortBy:      input.SortBy,
	})
	if err != nil {
		return nil, err
	}
	quota, err := s.repo.GetOrInitUserStorageQuota(ctx, input.UserID, s.cfg.Snapshot().UserStorageQuotaBytes)
	if err != nil {
		return nil, err
	}
	return &ListFilesResult{
		Items: items,
		Total: total,
		Quota: *quota,
	}, nil
}

// UploadFile 上传文件并扣减用户配额。
func (s *Service) UploadFile(ctx context.Context, input UploadFileInput) (*UploadFileResult, error) {
	if input.Reader == nil {
		return nil, s.errInvalidFileReference()
	}
	normalizedName := sanitizeFileName(input.FileName)
	if normalizedName == "" {
		return nil, s.errInvalidFileReference()
	}

	normalizedMIME := strings.TrimSpace(input.MimeType)
	if normalizedMIME == "" {
		normalizedMIME = "application/octet-stream"
	}

	userItem, err := s.repo.GetUserByID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	if userItem.Status != domainuser.StatusActive {
		return nil, s.errInvalidFileReference()
	}

	cfg := s.snapshot()
	maxUploadBytes := cfg.MaxUploadFileBytes
	if maxUploadBytes <= 0 {
		maxUploadBytes = 20 * 1024 * 1024
	}
	if input.DeclaredSize > 0 && input.DeclaredSize > maxUploadBytes {
		return nil, s.errFileTooLarge()
	}

	fileID := "file_" + conv.NormalizePublicID(uuid.NewString())
	ownerUserID := input.UserID
	quotaBytes := cfg.UserStorageQuotaBytes
	storageOwner := strings.TrimSpace(userItem.PublicID)
	if input.Ownership == FileOwnershipSystem {
		ownerUserID = 0
		quotaBytes = 0
		storageOwner = "system"
	} else if input.Ownership != "" && input.Ownership != FileOwnershipUser {
		return nil, s.errInvalidFileReference()
	}
	if storageOwner == "" {
		storageOwner = fmt.Sprintf("uid_%d", userItem.ID)
	}
	store, err := s.openObjectStore(ctx)
	if err != nil {
		return nil, err
	}
	relativePath, detectedMIME, shaValue, sizeBytes, err := saveUploadedFile(ctx, saveUploadedFileInput{
		Store:          store,
		Reader:         input.Reader,
		UserPublicID:   storageOwner,
		FileID:         fileID,
		FileName:       normalizedName,
		MaxUploadBytes: maxUploadBytes,
		DeclaredMIME:   normalizedMIME,
	})
	if err != nil {
		if errors.Is(err, errLocalFileTooLarge) {
			return nil, s.errFileTooLarge()
		}
		return nil, err
	}
	category := inferFileCategory(detectedMIME, normalizedName)
	removeUploadedObject := func(path string) {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		err := store.Delete(cleanupCtx, path)
		if err != nil && s.logger != nil {
			s.logger.Warn("remove_uploaded_file_failed", zap.String("path", path), zap.Error(err))
		}
	}

	if isDangerousMIME(detectedMIME) {
		removeUploadedObject(relativePath)
		return nil, s.errDangerousMIMEType()
	}
	if !isAllowedMIME(detectedMIME, cfg) {
		removeUploadedObject(relativePath)
		return nil, s.errMIMEBlocked()
	}
	if typeLimit := maxBytesForCategory(category, cfg); typeLimit > 0 && sizeBytes > typeLimit {
		removeUploadedObject(relativePath)
		return nil, s.errFileTooLarge()
	}
	releaseUploadGate, err := s.acquireUploadGate(ctx, ownerUserID, shaValue, sizeBytes)
	if err != nil {
		removeUploadedObject(relativePath)
		return nil, err
	}
	defer releaseUploadGate()

	if result, reused, reuseErr := s.tryReuseExistingFile(ctx, store, ownerUserID, shaValue, sizeBytes, quotaBytes); reuseErr != nil {
		removeUploadedObject(relativePath)
		return nil, reuseErr
	} else if reused {
		removeUploadedObject(relativePath)
		return result, nil
	}

	fileItem := &domainconversation.FileObject{
		FileID:           fileID,
		UserID:           ownerUserID,
		Purpose:          normalizePurpose(input.Purpose),
		FileName:         normalizedName,
		MimeType:         normalizedMIME,
		DetectedMIME:     detectedMIME,
		FileCategory:     category,
		SizeBytes:        sizeBytes,
		SHA256:           shaValue,
		StoragePath:      relativePath,
		Status:           "active",
		ProcessingStatus: "uploaded",
		ProcessingReady:  category == fileCategoryVideo || (category == fileCategoryImage && !cfg.ExtractImageOCREnabled),
		ExtractStatus:    "none",
		EmbedStatus:      "none",
		ExtractorVersion: s.resolveExtractorVersion(),
		ExpiresAt:        nil,
	}

	quota, err := s.repo.CreateFileObjectAndConsumeQuota(ctx, fileItem, quotaBytes)
	if err != nil && errors.Is(err, repository.ErrDuplicate) {
		if result, reused, reuseErr := s.tryReuseExistingFile(ctx, store, ownerUserID, shaValue, sizeBytes, quotaBytes); reuseErr != nil {
			removeUploadedObject(relativePath)
			return nil, reuseErr
		} else if reused {
			removeUploadedObject(relativePath)
			return result, nil
		}
		quota, err = s.repo.CreateFileObjectAndConsumeQuota(ctx, fileItem, quotaBytes)
	}
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			removeUploadedObject(relativePath)
			return nil, err
		}
		removeUploadedObject(relativePath)
		if errors.Is(err, repository.ErrStorageQuotaExceeded) || errors.Is(err, s.errors.StorageQuotaExceeded) {
			return nil, s.errStorageQuotaExceeded()
		}
		return nil, err
	}
	initializeCtx, cancelInitialize := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	initErr := s.initializeUploadedFile(initializeCtx, fileItem)
	cancelInitialize()
	if initErr != nil {
		if s.logger != nil {
			s.logger.Warn("initialize_uploaded_file_failed",
				zap.String("file_id", fileItem.FileID),
				zap.Error(initErr),
			)
		}
	} else if fileCategoryRequiresProcessing(category) {
		fileItem.ProcessingStatus = "queued"
		fileItem.ProcessingReady = false
		fileItem.ProcessingErrorCode = ""
		fileItem.ProcessingErrorMessage = ""
		fileItem.ExtractStatus = "none"
	}

	return &UploadFileResult{
		File:   *fileItem,
		Quota:  *quota,
		Reused: false,
	}, nil
}

// acquireUploadGate 获取同一内容的上传互斥许可，保证相同内容的并发上传串行执行；
// 返回释放函数，ctx 取消时返回错误并回收等待计数。
func (s *Service) acquireUploadGate(ctx context.Context, userID uint, shaValue string, sizeBytes int64) (func(), error) {
	key := fmt.Sprintf("%d:%s:%d", userID, shaValue, sizeBytes)
	s.uploadGatesMu.Lock()
	gate := s.uploadGates[key]
	if gate == nil {
		gate = &uploadContentGate{token: make(chan struct{}, 1)}
		gate.token <- struct{}{}
		s.uploadGates[key] = gate
	}
	gate.users++
	s.uploadGatesMu.Unlock()

	select {
	case <-gate.token:
		return func() {
			gate.token <- struct{}{}
			s.releaseUploadGate(key, gate)
		}, nil
	case <-ctx.Done():
		s.releaseUploadGate(key, gate)
		return nil, ctx.Err()
	}
}

// releaseUploadGate 递减闸门使用计数，最后一个使用者负责从映射中删除闸门，避免条目泄漏。
func (s *Service) releaseUploadGate(key string, gate *uploadContentGate) {
	s.uploadGatesMu.Lock()
	defer s.uploadGatesMu.Unlock()
	gate.users--
	if gate.users == 0 && s.uploadGates[key] == gate {
		delete(s.uploadGates, key)
	}
}

func (s *Service) tryReuseExistingFile(
	ctx context.Context,
	store objectstore.Store,
	userID uint,
	shaValue string,
	sizeBytes int64,
	quotaBytes int64,
) (*UploadFileResult, bool, error) {
	for {
		existingFile, err := s.repo.GetLatestActiveFileObjectBySHA(ctx, userID, shaValue, sizeBytes)
		if err != nil {
			return nil, false, err
		}
		if existingFile == nil {
			return nil, false, nil
		}

		matches, matchErr := objectMatchesContent(ctx, store, existingFile.StoragePath, shaValue, sizeBytes)
		if matchErr != nil {
			return nil, false, matchErr
		}
		if !matches {
			if s.logger != nil {
				s.logger.Warn("stale_file_object_content_mismatch",
					zap.String("file_id", existingFile.FileID),
					zap.String("path", existingFile.StoragePath),
				)
			}
			deletedFile, _, shouldRemovePhysical, deleteErr := s.repo.DeleteFileObjectAndReleaseQuota(ctx, userID, existingFile.FileID, quotaBytes, repository.DeleteFileObjectOptions{})
			if deleteErr != nil {
				return nil, false, deleteErr
			}
			if shouldRemovePhysical && deletedFile != nil {
				if rmErr := store.Delete(ctx, deletedFile.StoragePath); rmErr != nil && s.logger != nil {
					s.logger.Warn("remove_stale_file_object_failed",
						zap.String("file_id", deletedFile.FileID),
						zap.String("path", deletedFile.StoragePath),
						zap.Error(rmErr),
					)
				}
			}
			continue
		}

		accessedAt := time.Now()
		if touchErr := s.repo.TouchFileObjectLastAccessedAt(ctx, userID, existingFile.FileID, accessedAt); touchErr != nil && s.logger != nil {
			s.logger.Warn("touch_duplicate_file_access_failed",
				zap.String("file_id", existingFile.FileID),
				zap.Error(touchErr),
			)
		} else if touchErr == nil {
			existingFile.LastAccessedAt = &accessedAt
		}
		quota, quotaErr := s.repo.GetOrInitUserStorageQuota(ctx, userID, quotaBytes)
		if quotaErr != nil {
			return nil, false, quotaErr
		}
		return &UploadFileResult{
			File:   *existingFile,
			Quota:  *quota,
			Reused: true,
		}, true, nil
	}
}

func objectMatchesContent(ctx context.Context, store objectstore.Store, path string, expectedSHA256 string, expectedSize int64) (bool, error) {
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		return false, nil
	}
	reader, info, err := store.Open(ctx, normalizedPath)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotFound) || errors.Is(err, objectstore.ErrInvalidKey) {
			return false, nil
		}
		return false, err
	}
	if reader != nil {
		defer reader.Close() //nolint:errcheck
	}
	if info.SizeBytes > 0 && info.SizeBytes != expectedSize {
		return false, nil
	}
	hasher := sha256.New()
	readBytes, err := io.Copy(hasher, reader)
	if err != nil {
		return false, err
	}
	return readBytes == expectedSize && hex.EncodeToString(hasher.Sum(nil)) == expectedSHA256, nil
}

// DeleteFile 删除文件并回收配额。
func (s *Service) DeleteFile(ctx context.Context, userID uint, fileID string) (*DeleteFileResult, error) {
	result, _, err := s.deleteFile(ctx, userID, fileID, repository.DeleteFileObjectOptions{})
	return result, err
}

// DeleteFileIfUnreferenced 仅在文件未被活跃会话引用时删除文件并回收配额。
func (s *Service) DeleteFileIfUnreferenced(ctx context.Context, userID uint, fileID string) (*DeleteFileResult, bool, error) {
	return s.deleteFile(ctx, userID, fileID, repository.DeleteFileObjectOptions{RequireUnreferenced: true})
}

func (s *Service) deleteFile(ctx context.Context, userID uint, fileID string, options repository.DeleteFileObjectOptions) (*DeleteFileResult, bool, error) {
	normalizedFileID := strings.TrimSpace(fileID)
	if normalizedFileID == "" {
		return nil, false, s.errInvalidFileReference()
	}

	cfg := s.snapshot()
	quotaBytes := cfg.UserStorageQuotaBytes
	if userID == 0 {
		quotaBytes = 0
	}
	deletedFile, quota, shouldRemovePhysical, err := s.repo.DeleteFileObjectAndReleaseQuota(ctx, userID, normalizedFileID, quotaBytes, options)
	if err != nil {
		if options.RequireUnreferenced && errors.Is(err, repository.ErrConflict) {
			return nil, false, nil
		}
		if errors.Is(err, repository.ErrConflict) {
			return nil, false, s.errFileInUse()
		}
		if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrFileNotFound) {
			return nil, false, s.errFileNotFound()
		}
		return nil, false, err
	}
	if shouldRemovePhysical {
		store, storeErr := s.openObjectStore(ctx)
		if storeErr != nil {
			if s.logger != nil {
				s.logger.Warn("object_store_init_failed", zap.Error(storeErr))
			}
		} else if rmErr := store.Delete(ctx, deletedFile.StoragePath); rmErr != nil && s.logger != nil {
			s.logger.Warn("remove_deleted_file_failed",
				zap.String("file_id", normalizedFileID),
				zap.String("path", deletedFile.StoragePath),
				zap.Error(rmErr),
			)
		}
	}

	return &DeleteFileResult{
		Deleted: true,
		FileID:  normalizedFileID,
		Quota:   *quota,
	}, true, nil
}

// RenameFile 重命名当前用户文件。
func (s *Service) RenameFile(ctx context.Context, userID uint, fileID string, fileName string) (*domainconversation.FileObject, error) {
	normalizedFileID := strings.TrimSpace(fileID)
	if normalizedFileID == "" {
		return nil, s.errInvalidFileReference()
	}
	normalizedName := sanitizeFileName(fileName)
	if normalizedName == "" {
		return nil, s.errInvalidFileName()
	}
	item, err := s.repo.RenameFileObjectByID(ctx, userID, normalizedFileID, normalizedName)
	if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrFileNotFound) {
		return nil, s.errFileNotFound()
	}
	return item, err
}

// UpdateFileRAGOptOut 更新用户文件的 RAG 检索开关。
func (s *Service) UpdateFileRAGOptOut(ctx context.Context, userID uint, fileID string, ragOptOut bool) (*domainconversation.FileObject, error) {
	normalizedFileID := strings.TrimSpace(fileID)
	if normalizedFileID == "" {
		return nil, s.errInvalidFileReference()
	}
	item, err := s.repo.UpdateFileObjectRAGOptOut(ctx, userID, normalizedFileID, ragOptOut)
	if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrFileNotFound) {
		return nil, s.errFileNotFound()
	}
	return item, err
}

// ValidateImageFile 确认文件属于当前用户且可作为图片头像使用。
func (s *Service) ValidateImageFile(ctx context.Context, userID uint, fileID string) error {
	normalizedFileID := strings.TrimSpace(fileID)
	if normalizedFileID == "" {
		return s.errInvalidFileReference()
	}

	item, err := s.repo.GetActiveFileObjectByID(ctx, userID, normalizedFileID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrFileNotFound) {
			return s.errFileNotFound()
		}
		return err
	}
	if item.FileCategory == fileCategoryImage {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(item.DetectedMIME)), "image/") ||
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(item.MimeType)), "image/") {
		return nil
	}
	return s.errMIMEBlocked()
}

// OpenFileContent 打开当前用户的文件内容。
func (s *Service) OpenFileContent(ctx context.Context, userID uint, fileID string) (*FileContentResult, error) {
	normalizedFileID := strings.TrimSpace(fileID)
	if normalizedFileID == "" {
		return nil, s.errInvalidFileReference()
	}

	item, err := s.repo.GetActiveFileObjectByID(ctx, userID, normalizedFileID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrFileNotFound) {
			return nil, s.errFileNotFound()
		}
		return nil, err
	}

	store, err := s.openObjectStore(ctx)
	if err != nil {
		return nil, err
	}
	reader, info, err := store.Open(ctx, item.StoragePath)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotFound) {
			return nil, s.errFileNotFound()
		}
		return nil, err
	}

	contentType := strings.TrimSpace(item.MimeType)
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(item.FileName)))
	}
	if contentType == "" {
		contentType = strings.TrimSpace(info.ContentType)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	accessedAt := time.Now()
	item.LastAccessedAt = &accessedAt
	if touchErr := s.repo.TouchFileObjectLastAccessedAt(ctx, userID, normalizedFileID, accessedAt); touchErr != nil && s.logger != nil {
		s.logger.Warn("touch_file_last_accessed_failed",
			zap.String("file_id", normalizedFileID),
			zap.Error(touchErr),
		)
	}

	return &FileContentResult{
		File:        *item,
		Reader:      reader,
		ContentType: contentType,
		SizeBytes:   info.SizeBytes,
		ModTime:     info.ModTime,
	}, nil
}

const (
	fileCategoryImage        = "image"
	fileCategoryVideo        = "video"
	fileCategoryPDF          = "pdf"
	fileCategoryWord         = "word"
	fileCategoryPresentation = "presentation"
	fileCategoryExcel        = "excel"
	fileCategoryText         = "text"
	fileCategoryUnknown      = "unknown"
)

var dangerousMIMETypes = map[string]struct{}{
	"application/x-executable":                      {},
	"application/x-mach-binary":                     {},
	"application/x-elf":                             {},
	"application/x-sh":                              {},
	"application/x-shellscript":                     {},
	"application/x-bat":                             {},
	"application/x-msdos-program":                   {},
	"application/x-dosexec":                         {},
	"application/x-msdownload":                      {},
	"application/vnd.microsoft.portable-executable": {},
}

func (s *Service) snapshot() config.Config {
	if s == nil || s.cfg == nil {
		return config.Config{}
	}
	return s.cfg.Snapshot()
}

func (s *Service) initializeUploadedFile(ctx context.Context, file *domainconversation.FileObject) error {
	if s == nil || s.hooks.InitializeUploadedFile == nil {
		return nil
	}
	return s.hooks.InitializeUploadedFile(ctx, file)
}

func (s *Service) resolveExtractorVersion() string {
	if strings.TrimSpace(s.extractorVersion) == "" {
		return "file-pipeline-v1"
	}
	return s.extractorVersion
}

func (s *Service) errInvalidFileReference() error {
	return pickError(s.errors.InvalidFileReference, "invalid file reference")
}

func (s *Service) errInvalidFileName() error {
	return pickError(s.errors.InvalidFileName, "invalid file name")
}

func (s *Service) errFileNotFound() error {
	return pickError(s.errors.FileNotFound, "file not found")
}

func (s *Service) errFileInUse() error {
	return pickError(s.errors.FileInUse, "file in use")
}

func (s *Service) errStorageQuotaExceeded() error {
	return pickError(s.errors.StorageQuotaExceeded, "storage quota exceeded")
}

func (s *Service) errFileTooLarge() error {
	return pickError(s.errors.FileTooLarge, "file too large")
}

func (s *Service) errMIMEBlocked() error {
	return pickError(s.errors.MIMEBlocked, "mime blocked")
}

func (s *Service) errDangerousMIMEType() error {
	return pickError(s.errors.DangerousMIMEType, "dangerous file type not allowed")
}

func pickError(err error, fallback string) error {
	if err != nil {
		return err
	}
	return errors.New(fallback)
}

func sanitizeFileName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	base = strings.ReplaceAll(base, " ", "_")
	base = strings.ReplaceAll(base, "/", "_")
	base = strings.ReplaceAll(base, "\\", "_")
	base = strings.TrimSpace(base)
	if base == "." || base == "" {
		return ""
	}
	return base
}

func normalizePurpose(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "conversation_input"
	}
	return value
}

func normalizeDetectedMIME(detected string, fileName string) string {
	value := normalizeMIMEValue(detected)
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(strings.TrimSpace(fileName)), "."))
	if isActiveFileExtension(ext) || isActiveUploadMIME(value) {
		return "text/plain"
	}
	switch ext {
	case "pdf":
		return "application/pdf"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "doc":
		return "application/msword"
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "ppt":
		return "application/vnd.ms-powerpoint"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "xls":
		return "application/vnd.ms-excel"
	case "csv":
		return "text/csv"
	case "md", "markdown":
		return "text/markdown"
	case "json":
		return "application/json"
	case "yaml", "yml":
		return "text/yaml"
	case "toml":
		return "application/toml"
	case "mp4":
		return "video/mp4"
	case "webm":
		return "video/webm"
	}
	if ext != "" && filetype.IsText("", "sample."+ext) {
		return "text/plain"
	}
	if value == "application/zip" {
		switch ext {
		case "docx":
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		case "pptx":
			return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
		case "xlsx":
			return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}
	}
	return value
}

func normalizeMIMEValue(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if idx := strings.Index(value, ";"); idx > 0 {
		value = strings.TrimSpace(value[:idx])
	}
	return value
}

func isActiveFileExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "html", "htm", "css", "js", "jsx", "mjs", "ts", "tsx", "xml", "xhtml", "svg":
		return true
	default:
		return false
	}
}

func isActiveUploadMIME(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "text/html",
		"text/css",
		"text/javascript",
		"text/xml",
		"application/javascript",
		"application/ecmascript",
		"application/x-javascript",
		"application/typescript",
		"application/xml",
		"application/xhtml+xml",
		"image/svg+xml":
		return true
	default:
		return false
	}
}

func detectContentMIME(header []byte, declared string, fileName string) string {
	if len(header) == 0 {
		return normalizeDetectedMIME(declared, fileName)
	}
	return normalizeDetectedMIME(http.DetectContentType(header), fileName)
}

func inferFileCategory(mimeType string, fileName string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(strings.TrimSpace(fileName)), "."))
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return fileCategoryImage
	case strings.HasPrefix(mimeType, "video/"):
		return fileCategoryVideo
	case mimeType == "application/pdf" || ext == "pdf":
		return fileCategoryPDF
	case strings.Contains(mimeType, "wordprocessingml") || strings.Contains(mimeType, "msword") || ext == "docx" || ext == "doc":
		return fileCategoryWord
	case strings.Contains(mimeType, "presentationml") || strings.Contains(mimeType, "ms-powerpoint") || ext == "pptx" || ext == "ppt":
		return fileCategoryPresentation
	case strings.Contains(mimeType, "spreadsheetml") || strings.Contains(mimeType, "ms-excel") || mimeType == "text/csv" || ext == "xlsx" || ext == "xls" || ext == "csv":
		return fileCategoryExcel
	case filetype.IsText(mimeType, fileName):
		return fileCategoryText
	default:
		return fileCategoryUnknown
	}
}

func isAllowedMIME(mimeType string, cfg config.Config) bool {
	items := strings.Split(cfg.FileAllowedMIMETypes, ",")
	allowed := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" {
			continue
		}
		allowed[value] = struct{}{}
	}
	if len(allowed) == 0 {
		return true
	}
	_, ok := allowed[strings.ToLower(strings.TrimSpace(mimeType))]
	return ok
}

func maxBytesForCategory(category string, cfg config.Config) int64 {
	if category == fileCategoryImage {
		return cfg.FileImageMaxBytes
	}
	if category == fileCategoryVideo {
		return 0
	}
	return cfg.FileDocMaxBytes
}

func fileCategoryRequiresProcessing(category string) bool {
	switch category {
	case fileCategoryPDF, fileCategoryWord, fileCategoryPresentation, fileCategoryExcel, fileCategoryText:
		return true
	default:
		return false
	}
}

func isDangerousMIME(mimeType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(mimeType))
	if normalized == "" {
		return false
	}
	if idx := strings.Index(normalized, ";"); idx > 0 {
		normalized = strings.TrimSpace(normalized[:idx])
	}
	_, blocked := dangerousMIMETypes[normalized]
	return blocked
}

func saveUploadedFile(ctx context.Context, input saveUploadedFileInput) (string, string, string, int64, error) {
	normalizedUserID := strings.TrimSpace(input.UserPublicID)
	if normalizedUserID == "" {
		normalizedUserID = "unknown_user"
	}
	if input.MaxUploadBytes <= 0 {
		input.MaxUploadBytes = 20 * 1024 * 1024
	}

	staged, err := stageUploadedFile(input.Reader, input.FileID, input.FileName, input.MaxUploadBytes, input.DeclaredMIME)
	if err != nil {
		return "", "", "", 0, err
	}
	defer os.Remove(staged.absolutePath) //nolint:errcheck

	now := time.Now()
	relativePath := filepath.Join(
		normalizedUserID,
		now.Format("2006"),
		now.Format("01"),
		input.FileID+"_"+sanitizeFileName(input.FileName),
	)
	relativePath = filepath.ToSlash(relativePath)
	tmpFile, err := os.Open(staged.absolutePath)
	if err != nil {
		return "", "", "", 0, err
	}
	defer tmpFile.Close() //nolint:errcheck
	if _, err = input.Store.Put(ctx, relativePath, tmpFile, objectstore.PutOptions{
		SizeBytes:   staged.sizeBytes,
		ContentType: staged.detectedMIME,
	}); err != nil {
		return "", "", "", 0, err
	}

	return relativePath, staged.detectedMIME, staged.sha256, staged.sizeBytes, nil
}
