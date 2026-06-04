package upload

import (
	"bufio"
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
	"time"

	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstore"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var errLocalFileTooLarge = errors.New("local file too large")

// FileCapability 描述上传服务可见的文档能力边界。
type FileCapability struct {
	RAGAvailable         bool
	EffectiveDocMaxBytes int64
}

// Hooks 封装上传后续编排动作。
type Hooks struct {
	ResolveCapability      func(ctx context.Context) FileCapability
	InitializeUploadedFile func(ctx context.Context, file *domainconversation.FileObject) error
}

// ErrorSet 允许上层注入统一错误语义。
type ErrorSet struct {
	InvalidFileReference error
	InvalidFileName      error
	FileNotFound         error
	StorageQuotaExceeded error
	FileTooLarge         error
	MIMEBlocked          error
	EmbeddingUnavailable error
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
}

// UploadFileInput 定义文件上传请求。
type UploadFileInput struct {
	UserID       uint
	Purpose      string
	FileName     string
	MimeType     string
	DeclaredSize int64
	Reader       io.Reader
}

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

// FileContentResult 定义文件内容读取结果。
type FileContentResult struct {
	File        domainconversation.FileObject
	Reader      io.ReadCloser
	ContentType string
	SizeBytes   int64
	ModTime     time.Time
}

// NewService 创建上传服务。
func NewService(cfg config.Config, repo repository.UploadRepository, logger *zap.Logger, hooks Hooks, errors ErrorSet, extractorVersion string) *Service {
	return NewServiceWithRuntime(config.NewRuntime(cfg), repo, logger, hooks, errors, extractorVersion)
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
		storeProvider:    appstorage.NewRuntimeProvider(cfg, nil),
	}
}

// SetObjectStoreProvider 注入对象存储 provider，供应用装配层替换具体实现。
func (s *Service) SetObjectStoreProvider(provider appstorage.Provider) {
	if provider != nil {
		s.storeProvider = provider
	}
}

func (s *Service) openObjectStore(ctx context.Context) (objectstore.Store, error) {
	if s.storeProvider == nil {
		s.storeProvider = appstorage.NewRuntimeProvider(s.cfg, nil)
	}
	return s.storeProvider.Open(ctx)
}

const (
	defaultPageSize            = 20
	maxPageSize                = 100
	embeddingTimeoutStaleAfter = 6 * time.Minute
)

// ListFiles 分页查询用户文件。
func (s *Service) ListFiles(
	ctx context.Context,
	userID uint,
	page int,
	pageSize int,
	searchQuery string,
	filterKind string,
	sortBy string,
) (*ListFilesResult, error) {
	offset, limit := normalizePage(page, pageSize)
	_, _ = s.repo.MarkTimedOutFileEmbeddingsFailed(
		ctx,
		userID,
		time.Now().Add(-embeddingTimeoutStaleAfter),
		"向量化超时，请检查向量化服务配置后重试",
	)
	items, total, err := s.repo.ListFileObjectsByUserWithFilter(ctx, userID, offset, limit, searchQuery, filterKind, sortBy)
	if err != nil {
		return nil, err
	}
	quota, err := s.repo.GetOrInitUserStorageQuota(ctx, userID, s.cfg.Snapshot().UserStorageQuotaBytes)
	if err != nil {
		return nil, err
	}
	return &ListFilesResult{
		Items: items,
		Total: total,
		Quota: *quota,
	}, nil
}

func normalizePage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	return offset, pageSize
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
	storageUserID := strings.TrimSpace(userItem.PublicID)
	if storageUserID == "" {
		storageUserID = fmt.Sprintf("uid_%d", userItem.ID)
	}
	store, err := s.openObjectStore(ctx)
	if err != nil {
		return nil, err
	}
	relativePath, detectedMIME, shaValue, sizeBytes, err := saveUploadedFile(
		ctx,
		store,
		input.Reader,
		storageUserID,
		fileID,
		normalizedName,
		maxUploadBytes,
		normalizedMIME,
	)
	if err != nil {
		if errors.Is(err, errLocalFileTooLarge) {
			return nil, s.errFileTooLarge()
		}
		return nil, err
	}
	category := inferFileCategory(detectedMIME, normalizedName)
	logRemoveErr := func(path string, err error) {
		if err != nil && s.logger != nil {
			s.logger.Warn("remove_uploaded_file_failed", zap.String("path", path), zap.Error(err))
		}
	}

	if isDangerousMIME(detectedMIME) {
		logRemoveErr(relativePath, store.Delete(ctx, relativePath))
		return nil, s.errDangerousMIMEType()
	}
	if !isAllowedMIME(detectedMIME, cfg) {
		logRemoveErr(relativePath, store.Delete(ctx, relativePath))
		return nil, s.errMIMEBlocked()
	}
	if typeLimit := maxBytesForCategory(category, cfg); typeLimit > 0 && sizeBytes > typeLimit {
		logRemoveErr(relativePath, store.Delete(ctx, relativePath))
		return nil, s.errFileTooLarge()
	}

	if result, reused, reuseErr := s.tryReuseExistingFile(ctx, store, input.UserID, shaValue, sizeBytes, cfg.UserStorageQuotaBytes); reuseErr != nil {
		logRemoveErr(relativePath, store.Delete(ctx, relativePath))
		return nil, reuseErr
	} else if reused {
		logRemoveErr(relativePath, store.Delete(ctx, relativePath))
		return result, nil
	}

	fileItem := &domainconversation.FileObject{
		FileID:           fileID,
		UserID:           input.UserID,
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
		ProcessingReady:  category == fileCategoryImage && !cfg.ExtractImageOCREnabled,
		ExtractStatus:    "none",
		EmbedStatus:      "none",
		ExtractorVersion: s.resolveExtractorVersion(),
		ExpiresAt:        nil,
	}

	quota, err := s.repo.CreateFileObjectAndConsumeQuota(ctx, fileItem, cfg.UserStorageQuotaBytes)
	if err != nil && errors.Is(err, repository.ErrDuplicate) {
		if result, reused, reuseErr := s.tryReuseExistingFile(ctx, store, input.UserID, shaValue, sizeBytes, cfg.UserStorageQuotaBytes); reuseErr != nil {
			logRemoveErr(relativePath, store.Delete(ctx, relativePath))
			return nil, reuseErr
		} else if reused {
			logRemoveErr(relativePath, store.Delete(ctx, relativePath))
			return result, nil
		}
		quota, err = s.repo.CreateFileObjectAndConsumeQuota(ctx, fileItem, cfg.UserStorageQuotaBytes)
	}
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			logRemoveErr(relativePath, store.Delete(ctx, relativePath))
			return nil, err
		}
		logRemoveErr(relativePath, store.Delete(ctx, relativePath))
		if errors.Is(err, s.errors.StorageQuotaExceeded) {
			return nil, s.errStorageQuotaExceeded()
		}
		return nil, err
	}
	if initErr := s.initializeUploadedFile(ctx, fileItem); initErr != nil {
		if s.logger != nil {
			s.logger.Warn("initialize_uploaded_file_failed",
				zap.String("file_id", fileItem.FileID),
				zap.Error(initErr),
			)
		}
	} else if category != fileCategoryImage {
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
	deletedFile, quota, shouldRemovePhysical, err := s.repo.DeleteFileObjectAndReleaseQuota(ctx, userID, normalizedFileID, cfg.UserStorageQuotaBytes, options)
	if err != nil {
		if options.RequireUnreferenced && errors.Is(err, repository.ErrConflict) {
			return nil, false, nil
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
	return s.repo.RenameFileObjectByID(ctx, userID, normalizedFileID, normalizedName)
}

// UpdateFileRagOptOut 更新用户文件的 RAG 检索开关。
func (s *Service) UpdateFileRagOptOut(ctx context.Context, userID uint, fileID string, ragOptOut bool) (*domainconversation.FileObject, error) {
	normalizedFileID := strings.TrimSpace(fileID)
	if normalizedFileID == "" {
		return nil, s.errInvalidFileReference()
	}
	return s.repo.UpdateFileObjectRagOptOut(ctx, userID, normalizedFileID, ragOptOut)
}

// OpenFileContent 打开当前用户的文件内容。
func (s *Service) OpenFileContent(ctx context.Context, userID uint, fileID string) (*FileContentResult, error) {
	normalizedFileID := strings.TrimSpace(fileID)
	if normalizedFileID == "" {
		return nil, s.errInvalidFileReference()
	}

	item, err := s.repo.GetActiveFileObjectByID(ctx, userID, normalizedFileID)
	if err != nil {
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
	fileCategoryImage   = "image"
	fileCategoryPDF     = "pdf"
	fileCategoryWord    = "word"
	fileCategoryExcel   = "excel"
	fileCategoryText    = "text"
	fileCategoryUnknown = "unknown"
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

func (s *Service) resolveCapability(ctx context.Context) FileCapability {
	if s == nil || s.hooks.ResolveCapability == nil {
		return FileCapability{}
	}
	return s.hooks.ResolveCapability(ctx)
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

func (s *Service) errStorageQuotaExceeded() error {
	return pickError(s.errors.StorageQuotaExceeded, "storage quota exceeded")
}

func (s *Service) errFileTooLarge() error {
	return pickError(s.errors.FileTooLarge, "file too large")
}

func (s *Service) errMIMEBlocked() error {
	return pickError(s.errors.MIMEBlocked, "mime blocked")
}

func (s *Service) errEmbeddingUnavailable() error {
	return pickError(s.errors.EmbeddingUnavailable, "embedding unavailable")
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
	}
	if ext != "" && isTextMIMEForEmbed("", "sample."+ext) {
		return "text/plain"
	}
	if value == "application/zip" {
		switch ext {
		case "docx":
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
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
	case mimeType == "application/pdf" || ext == "pdf":
		return fileCategoryPDF
	case strings.Contains(mimeType, "wordprocessingml") || strings.Contains(mimeType, "msword") || ext == "docx" || ext == "doc":
		return fileCategoryWord
	case strings.Contains(mimeType, "spreadsheetml") || strings.Contains(mimeType, "ms-excel") || mimeType == "text/csv" || ext == "xlsx" || ext == "xls" || ext == "csv":
		return fileCategoryExcel
	case isTextMIMEForEmbed(mimeType, fileName):
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
	return cfg.FileDocMaxBytes
}

func supportsRAG(category string) bool {
	switch category {
	case fileCategoryPDF, fileCategoryWord, fileCategoryExcel, fileCategoryText:
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

func saveUploadedFile(
	ctx context.Context,
	store objectstore.Store,
	reader io.Reader,
	userPublicID string,
	fileID string,
	fileName string,
	maxUploadBytes int64,
	declaredMIME string,
) (string, string, string, int64, error) {
	normalizedUserID := strings.TrimSpace(userPublicID)
	if normalizedUserID == "" {
		normalizedUserID = "unknown_user"
	}
	if maxUploadBytes <= 0 {
		maxUploadBytes = 20 * 1024 * 1024
	}

	tmpFile, err := os.CreateTemp("", fileID+"_*.upload")
	if err != nil {
		return "", "", "", 0, err
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
	}()

	bufferedReader := bufio.NewReader(reader)
	header, _ := bufferedReader.Peek(512)
	detectedMIME := detectContentMIME(header, declaredMIME, fileName)

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmpFile, hasher), io.LimitReader(bufferedReader, maxUploadBytes+1))
	if err != nil {
		return "", "", "", 0, err
	}
	if written > maxUploadBytes {
		return "", "", "", 0, errLocalFileTooLarge
	}

	now := time.Now()
	relativePath := filepath.Join(
		normalizedUserID,
		now.Format("2006"),
		now.Format("01"),
		fileID+"_"+sanitizeFileName(fileName),
	)
	relativePath = filepath.ToSlash(relativePath)
	if _, err = tmpFile.Seek(0, io.SeekStart); err != nil {
		return "", "", "", 0, err
	}
	if _, err = store.Put(ctx, relativePath, tmpFile, objectstore.PutOptions{
		SizeBytes:   written,
		ContentType: detectedMIME,
	}); err != nil {
		return "", "", "", 0, err
	}

	return relativePath, detectedMIME, hex.EncodeToString(hasher.Sum(nil)), written, nil
}
