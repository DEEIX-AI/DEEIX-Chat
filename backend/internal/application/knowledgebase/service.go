package knowledgebase

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	maxKnowledgeBaseNameLength        = 80
	maxKnowledgeBaseDescriptionLength = 255
	maxKnowledgeBasesPerRequest       = 8
	maxFilesPerAddRequest             = 100
)

// Service 封装知识库业务逻辑。
type Service struct {
	repo         repository.KnowledgeBaseRepository
	auditWriter  auditWriter
	fileCleaner  fileCleaner
	fileOpener   fileContentOpener
	fileUploader fileUploader
	logger       *zap.Logger
}

type auditWriter interface {
	Write(ctx context.Context, requestID string, actorUserID uint, action string, resource string, resourceID string, ip string, userAgent string, detail interface{})
}

type fileCleaner interface {
	DeleteFileIfUnreferenced(ctx context.Context, userID uint, fileID string) (bool, error)
}

type fileContentOpener interface {
	OpenFileContent(ctx context.Context, userID uint, fileID string) (*appupload.FileContentResult, error)
}

type fileUploader interface {
	UploadFile(ctx context.Context, input appupload.UploadFileInput) (*appupload.UploadFileResult, error)
}

// DeleteResult 描述知识库删除及其可选文件清理结果。
type DeleteResult struct {
	DeletedFileCount int
}

// NewService 创建知识库服务。
func NewService(repo repository.KnowledgeBaseRepository) *Service {
	return &Service{repo: repo}
}

// SetAuditWriter 注入审计写入器。
func (s *Service) SetAuditWriter(writer auditWriter) {
	s.auditWriter = writer
}

// SetFileCleaner 注入文件安全清理能力。
func (s *Service) SetFileCleaner(cleaner fileCleaner) {
	s.fileCleaner = cleaner
}

// SetFileContentOpener 注入文件内容读取能力。
func (s *Service) SetFileContentOpener(opener fileContentOpener) {
	s.fileOpener = opener
}

// SetFileUploader injects the shared upload pipeline used for platform-owned assets.
func (s *Service) SetFileUploader(uploader fileUploader) {
	s.fileUploader = uploader
}

// SetLogger 注入结构化日志记录器。
func (s *Service) SetLogger(logger *zap.Logger) {
	s.logger = logger
}

// AuditInput 描述知识库审计写入。
type AuditInput struct {
	UserID     uint
	RequestID  string
	Action     string
	ResourceID string
	ClientIP   string
	UserAgent  string
	Detail     interface{}
}

// RecordAudit 记录知识库审计日志。
func (s *Service) RecordAudit(ctx context.Context, input AuditInput) {
	if s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(ctx, strings.TrimSpace(input.RequestID), input.UserID, strings.TrimSpace(input.Action),
		"knowledge_bases", strings.TrimSpace(input.ResourceID), strings.TrimSpace(input.ClientIP), strings.TrimSpace(input.UserAgent), input.Detail)
}

// ListInput 定义知识库列表入参。
type ListInput struct {
	Query    string
	Sort     string
	IDs      []string
	Enabled  *bool
	Page     int
	PageSize int
}

// WriteInput 定义知识库创建入参。
type WriteInput struct {
	Name        string
	Description string
	Enabled     bool
	SortOrder   int
}

// PatchInput 定义知识库更新入参。
type PatchInput struct {
	Name        *string
	Description *string
	Enabled     *bool
	SortOrder   *int
}

// ListVisible 查询当前用户可使用的内置和个人知识库。
func (s *Service) ListVisible(ctx context.Context, userID uint, input ListInput) ([]domainknowledgebase.KnowledgeBase, int64, error) {
	if userID == 0 {
		return nil, 0, ErrInvalidKnowledgeBase
	}
	offset, limit := normalizePage(input.Page, input.PageSize)
	publicIDs := normalizePublicIDs(input.IDs, maxKnowledgeBasesPerRequest)
	if len(input.IDs) > 0 && len(publicIDs) == 0 {
		return []domainknowledgebase.KnowledgeBase{}, 0, nil
	}
	return s.repo.ListKnowledgeBases(ctx, repository.KnowledgeBaseListFilter{
		Query: strings.TrimSpace(input.Query), Sort: strings.TrimSpace(input.Sort), PublicIDs: publicIDs, VisibleUserID: &userID,
	}, offset, limit)
}

// ListMine 查询当前用户个人知识库。
func (s *Service) ListMine(ctx context.Context, userID uint, input ListInput) ([]domainknowledgebase.KnowledgeBase, int64, error) {
	if userID == 0 {
		return nil, 0, ErrInvalidKnowledgeBase
	}
	offset, limit := normalizePage(input.Page, input.PageSize)
	publicIDs := normalizePublicIDs(input.IDs, maxKnowledgeBasesPerRequest)
	if len(input.IDs) > 0 && len(publicIDs) == 0 {
		return []domainknowledgebase.KnowledgeBase{}, 0, nil
	}
	return s.repo.ListKnowledgeBases(ctx, repository.KnowledgeBaseListFilter{
		Query: strings.TrimSpace(input.Query), Sort: strings.TrimSpace(input.Sort), PublicIDs: publicIDs, Scope: domainknowledgebase.ScopeUser, OwnerUserID: &userID, Enabled: input.Enabled,
	}, offset, limit)
}

// ListAdminBuiltin 查询管理员内置知识库。
func (s *Service) ListAdminBuiltin(ctx context.Context, input ListInput) ([]domainknowledgebase.KnowledgeBase, int64, error) {
	offset, limit := normalizePage(input.Page, input.PageSize)
	publicIDs := normalizePublicIDs(input.IDs, maxKnowledgeBasesPerRequest)
	if len(input.IDs) > 0 && len(publicIDs) == 0 {
		return []domainknowledgebase.KnowledgeBase{}, 0, nil
	}
	return s.repo.ListKnowledgeBases(ctx, repository.KnowledgeBaseListFilter{
		Query: strings.TrimSpace(input.Query), Sort: strings.TrimSpace(input.Sort), PublicIDs: publicIDs, Scope: domainknowledgebase.ScopeBuiltin, Enabled: input.Enabled,
	}, offset, limit)
}

// GetVisible 查询当前用户可见知识库。
func (s *Service) GetVisible(ctx context.Context, userID uint, publicID string) (*domainknowledgebase.KnowledgeBase, error) {
	if userID == 0 {
		return nil, ErrInvalidKnowledgeBase
	}
	item, err := s.get(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if !isVisibleToUser(item, userID) {
		return nil, ErrKnowledgeBaseNotFound
	}
	return item, nil
}

// GetAdmin 查询管理员可管理的内置知识库。
func (s *Service) GetAdmin(ctx context.Context, publicID string) (*domainknowledgebase.KnowledgeBase, error) {
	item, err := s.get(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if item.Scope != domainknowledgebase.ScopeBuiltin {
		return nil, ErrKnowledgeBaseNotFound
	}
	return item, nil
}

// CreateUser 创建个人知识库。
func (s *Service) CreateUser(ctx context.Context, userID uint, input WriteInput) (*domainknowledgebase.KnowledgeBase, error) {
	if userID == 0 {
		return nil, ErrInvalidKnowledgeBase
	}
	// 个人知识库始终可用。停用是管理员控制内置知识库发布状态的能力，
	// 不暴露给个人知识库，避免产生 UI 无法恢复的隐藏数据。
	input.Enabled = true
	item, err := normalizeWriteInput(input, domainknowledgebase.ScopeUser, userID, userID)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, item)
}

// CreateBuiltin 创建管理员内置知识库。
func (s *Service) CreateBuiltin(ctx context.Context, actorUserID uint, input WriteInput) (*domainknowledgebase.KnowledgeBase, error) {
	if actorUserID == 0 {
		return nil, ErrInvalidKnowledgeBase
	}
	item, err := normalizeWriteInput(input, domainknowledgebase.ScopeBuiltin, 0, actorUserID)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, item)
}

// UpdateUser 更新个人知识库。
func (s *Service) UpdateUser(ctx context.Context, userID uint, publicID string, input PatchInput) (*domainknowledgebase.KnowledgeBase, error) {
	if userID == 0 || input.Enabled != nil {
		return nil, ErrInvalidKnowledgeBase
	}
	item, err := s.get(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if item.Scope != domainknowledgebase.ScopeUser || item.OwnerUserID != userID {
		return nil, ErrKnowledgeBaseNotFound
	}
	return s.update(ctx, item.ID, userID, input)
}

// UpdateBuiltin 更新管理员内置知识库。
func (s *Service) UpdateBuiltin(ctx context.Context, actorUserID uint, publicID string, input PatchInput) (*domainknowledgebase.KnowledgeBase, error) {
	if actorUserID == 0 {
		return nil, ErrInvalidKnowledgeBase
	}
	item, err := s.get(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if item.Scope != domainknowledgebase.ScopeBuiltin {
		return nil, ErrKnowledgeBaseNotFound
	}
	return s.update(ctx, item.ID, actorUserID, input)
}

// DeleteUser 删除个人知识库，并可选清理不再被引用的文件。
func (s *Service) DeleteUser(ctx context.Context, userID uint, publicID string, deleteFiles bool) (DeleteResult, error) {
	if userID == 0 {
		return DeleteResult{}, ErrInvalidKnowledgeBase
	}
	item, err := s.get(ctx, publicID)
	if err != nil {
		return DeleteResult{}, err
	}
	if item.Scope != domainknowledgebase.ScopeUser || item.OwnerUserID != userID {
		return DeleteResult{}, ErrKnowledgeBaseNotFound
	}
	return s.delete(ctx, item.ID, deleteFiles)
}

// DeleteBuiltin 删除内置知识库，并可选清理不再被引用的文件。
func (s *Service) DeleteBuiltin(ctx context.Context, publicID string, deleteFiles bool) (DeleteResult, error) {
	item, err := s.get(ctx, publicID)
	if err != nil {
		return DeleteResult{}, err
	}
	if item.Scope != domainknowledgebase.ScopeBuiltin {
		return DeleteResult{}, ErrKnowledgeBaseNotFound
	}
	return s.delete(ctx, item.ID, deleteFiles)
}

func (s *Service) delete(ctx context.Context, knowledgeBaseID uint, deleteFiles bool) (DeleteResult, error) {
	if deleteFiles && s.fileCleaner == nil {
		return DeleteResult{}, ErrKnowledgeBaseFileCleanupUnavailable
	}
	candidates, err := s.repo.DeleteKnowledgeBase(ctx, knowledgeBaseID)
	if err != nil {
		return DeleteResult{}, mapRepositoryError(err)
	}
	result := DeleteResult{}
	if !deleteFiles {
		return result, nil
	}
	for _, candidate := range candidates {
		deleted, cleanupErr := s.fileCleaner.DeleteFileIfUnreferenced(ctx, candidate.UserID, candidate.FileID)
		if cleanupErr != nil {
			if s.logger != nil {
				s.logger.Warn("delete_knowledge_base_file_failed",
					zap.Uint("user_id", candidate.UserID),
					zap.String("file_id", candidate.FileID),
					zap.Error(cleanupErr),
				)
			}
			continue
		}
		if deleted {
			result.DeletedFileCount++
		}
	}
	return result, nil
}

// ListVisibleFiles 查询当前用户可见知识库内的文件。
func (s *Service) ListVisibleFiles(ctx context.Context, userID uint, publicID string, page int, pageSize int) ([]domainconversation.FileObject, int64, error) {
	item, err := s.GetVisible(ctx, userID, publicID)
	if err != nil {
		return nil, 0, err
	}
	offset, limit := normalizePage(page, pageSize)
	return s.repo.ListKnowledgeBaseFiles(ctx, item.ID, offset, limit)
}

// ListAdminFiles 查询管理员内置知识库文件。
func (s *Service) ListAdminFiles(ctx context.Context, publicID string, page int, pageSize int) ([]domainconversation.FileObject, int64, error) {
	item, err := s.get(ctx, publicID)
	if err != nil {
		return nil, 0, err
	}
	if item.Scope != domainknowledgebase.ScopeBuiltin {
		return nil, 0, ErrKnowledgeBaseNotFound
	}
	offset, limit := normalizePage(page, pageSize)
	return s.repo.ListKnowledgeBaseFiles(ctx, item.ID, offset, limit)
}

// GetVisibleFileProcessingStatuses 批量查询当前用户可见知识库内的文件处理状态。
func (s *Service) GetVisibleFileProcessingStatuses(ctx context.Context, userID uint, publicID string, fileIDs []string) ([]domainconversation.FileObject, error) {
	if userID == 0 {
		return nil, ErrInvalidKnowledgeBase
	}
	item, err := s.getForAccess(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if !isVisibleToUser(item, userID) {
		return nil, ErrKnowledgeBaseNotFound
	}
	return s.getFileProcessingStatuses(ctx, item.ID, fileIDs)
}

// GetVisibleFileProcessingSnapshot 查询当前用户可见知识库的处理状态快照。
func (s *Service) GetVisibleFileProcessingSnapshot(ctx context.Context, userID uint, publicID string, fileIDs []string) (*domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error) {
	if userID == 0 {
		return nil, nil, ErrInvalidKnowledgeBase
	}
	item, err := s.getForAccess(ctx, publicID)
	if err != nil {
		return nil, nil, err
	}
	if !isVisibleToUser(item, userID) {
		return nil, nil, ErrKnowledgeBaseNotFound
	}
	return s.getFileProcessingSnapshot(ctx, item, fileIDs)
}

// GetAdminFileProcessingStatuses 批量查询内置知识库内的文件处理状态。
func (s *Service) GetAdminFileProcessingStatuses(ctx context.Context, publicID string, fileIDs []string) ([]domainconversation.FileObject, error) {
	item, err := s.getForAccess(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if item.Scope != domainknowledgebase.ScopeBuiltin {
		return nil, ErrKnowledgeBaseNotFound
	}
	return s.getFileProcessingStatuses(ctx, item.ID, fileIDs)
}

// GetAdminFileProcessingSnapshot 查询内置知识库的处理状态快照。
func (s *Service) GetAdminFileProcessingSnapshot(ctx context.Context, publicID string, fileIDs []string) (*domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error) {
	item, err := s.getForAccess(ctx, publicID)
	if err != nil {
		return nil, nil, err
	}
	if item.Scope != domainknowledgebase.ScopeBuiltin {
		return nil, nil, ErrKnowledgeBaseNotFound
	}
	return s.getFileProcessingSnapshot(ctx, item, fileIDs)
}

func (s *Service) getFileProcessingSnapshot(ctx context.Context, item *domainknowledgebase.KnowledgeBase, fileIDs []string) (*domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error) {
	ids := normalizePublicIDs(fileIDs, maxFilesPerAddRequest)
	if len(fileIDs) > 0 && len(ids) == 0 {
		return nil, nil, ErrInvalidKnowledgeBase
	}
	snapshot, err := s.repo.GetKnowledgeBaseFileProcessingSnapshot(ctx, item.ID, ids)
	if err != nil {
		return nil, nil, mapRepositoryError(err)
	}
	item.FileCount = snapshot.FileCount
	item.ReadyFileCount = snapshot.ReadyFileCount
	item.ProcessingFileCount = snapshot.ProcessingFileCount
	return item, snapshot.Files, nil
}

func (s *Service) getFileProcessingStatuses(ctx context.Context, knowledgeBaseID uint, fileIDs []string) ([]domainconversation.FileObject, error) {
	ids := normalizePublicIDs(fileIDs, maxFilesPerAddRequest)
	if len(ids) == 0 {
		return nil, ErrInvalidKnowledgeBase
	}
	items, err := s.repo.GetKnowledgeBaseFileProcessingStatuses(ctx, knowledgeBaseID, ids)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return items, nil
}

// ListPlatformFiles 查询平台资料池中的全部有效文件。
func (s *Service) ListPlatformFiles(ctx context.Context, actorUserID uint, input ListInput) ([]domainconversation.FileObject, int64, error) {
	if actorUserID == 0 {
		return nil, 0, ErrInvalidKnowledgeBase
	}
	offset, limit := normalizePage(input.Page, input.PageSize)
	return s.repo.ListKnowledgeBaseSourceFiles(ctx, 0, strings.TrimSpace(input.Query), offset, limit)
}

// ListAvailableUserFiles 查询当前用户尚未加入个人知识库的文件。
func (s *Service) ListAvailableUserFiles(ctx context.Context, userID uint, publicID string, input ListInput) ([]domainconversation.FileObject, int64, error) {
	if userID == 0 {
		return nil, 0, ErrInvalidKnowledgeBase
	}
	item, err := s.get(ctx, publicID)
	if err != nil {
		return nil, 0, err
	}
	if item.Scope != domainknowledgebase.ScopeUser || item.OwnerUserID != userID {
		return nil, 0, ErrKnowledgeBaseNotFound
	}
	offset, limit := normalizePage(input.Page, input.PageSize)
	return s.repo.ListAvailableKnowledgeBaseFiles(ctx, item.ID, userID, strings.TrimSpace(input.Query), offset, limit)
}

// ListAvailableAdminFiles 查询尚未加入指定内置知识库的平台资料。
func (s *Service) ListAvailableAdminFiles(ctx context.Context, actorUserID uint, publicID string, input ListInput) ([]domainconversation.FileObject, int64, error) {
	if actorUserID == 0 {
		return nil, 0, ErrInvalidKnowledgeBase
	}
	item, err := s.get(ctx, publicID)
	if err != nil {
		return nil, 0, err
	}
	if item.Scope != domainknowledgebase.ScopeBuiltin {
		return nil, 0, ErrKnowledgeBaseNotFound
	}
	offset, limit := normalizePage(input.Page, input.PageSize)
	return s.repo.ListAvailableKnowledgeBaseFiles(ctx, item.ID, 0, strings.TrimSpace(input.Query), offset, limit)
}

// UploadBuiltinFile 通过统一文件处理链路上传平台资料。
// 管理员仅作为操作人；文件 owner ID 固定为 0，不计入任何个人额度。
func (s *Service) UploadBuiltinFile(ctx context.Context, actorUserID uint, input appupload.UploadFileInput) (*appupload.UploadFileResult, error) {
	if actorUserID == 0 || s.fileUploader == nil {
		return nil, ErrInvalidKnowledgeBase
	}
	input.UserID = actorUserID
	input.Ownership = appupload.FileOwnershipSystem
	input.Purpose = "knowledge_base"
	return s.fileUploader.UploadFile(ctx, input)
}

// DeletePlatformFile 删除未被任何资源引用的平台资料。
// owner ID 固定为 0，确保该入口无法删除任何用户个人文件。
func (s *Service) DeletePlatformFile(ctx context.Context, actorUserID uint, fileID string) error {
	fileID = strings.TrimSpace(fileID)
	if actorUserID == 0 || fileID == "" {
		return ErrInvalidKnowledgeBase
	}
	if s.fileCleaner == nil {
		return ErrKnowledgeBaseFileCleanupUnavailable
	}
	deleted, err := s.fileCleaner.DeleteFileIfUnreferenced(ctx, 0, fileID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrPlatformFileInUse
	}
	return nil
}

// OpenPlatformFileContent 打开平台资料池中的文件内容。
func (s *Service) OpenPlatformFileContent(ctx context.Context, actorUserID uint, fileID string) (*appupload.FileContentResult, error) {
	fileID = strings.TrimSpace(fileID)
	if actorUserID == 0 || fileID == "" {
		return nil, ErrInvalidKnowledgeBase
	}
	if s.fileOpener == nil {
		return nil, ErrKnowledgeBaseFileContentUnavailable
	}
	result, err := s.fileOpener.OpenFileContent(ctx, 0, fileID)
	if err != nil {
		return nil, mapFileRepositoryError(err)
	}
	return result, nil
}

// OpenVisibleFileContent 打开当前用户可见且仍属于知识库的文件内容。
func (s *Service) OpenVisibleFileContent(ctx context.Context, userID uint, publicID string, fileID string) (*appupload.FileContentResult, error) {
	item, err := s.GetVisible(ctx, userID, publicID)
	if err != nil {
		return nil, err
	}
	return s.openFileContent(ctx, item.ID, fileID)
}

// OpenAdminFileContent 打开内置知识库中仍有关联的文件内容。
func (s *Service) OpenAdminFileContent(ctx context.Context, publicID string, fileID string) (*appupload.FileContentResult, error) {
	item, err := s.get(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if item.Scope != domainknowledgebase.ScopeBuiltin {
		return nil, ErrKnowledgeBaseNotFound
	}
	return s.openFileContent(ctx, item.ID, fileID)
}

func (s *Service) openFileContent(ctx context.Context, knowledgeBaseID uint, fileID string) (*appupload.FileContentResult, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, ErrInvalidKnowledgeBase
	}
	file, err := s.repo.GetKnowledgeBaseFile(ctx, knowledgeBaseID, fileID)
	if err != nil {
		return nil, mapFileRepositoryError(err)
	}
	if s.fileOpener == nil {
		return nil, ErrKnowledgeBaseFileContentUnavailable
	}
	return s.fileOpener.OpenFileContent(ctx, file.UserID, file.FileID)
}

// AddUserFiles 将当前用户文件加入个人知识库。
func (s *Service) AddUserFiles(ctx context.Context, userID uint, publicID string, fileIDs []string) error {
	if userID == 0 {
		return ErrInvalidKnowledgeBase
	}
	item, err := s.get(ctx, publicID)
	if err != nil {
		return err
	}
	if item.Scope != domainknowledgebase.ScopeUser || item.OwnerUserID != userID {
		return ErrKnowledgeBaseNotFound
	}
	return s.addFiles(ctx, item, userID, fileIDs)
}

// AddBuiltinFiles 将平台资料加入内置知识库。
func (s *Service) AddBuiltinFiles(ctx context.Context, actorUserID uint, publicID string, fileIDs []string) error {
	if actorUserID == 0 {
		return ErrInvalidKnowledgeBase
	}
	item, err := s.get(ctx, publicID)
	if err != nil {
		return err
	}
	if item.Scope != domainknowledgebase.ScopeBuiltin {
		return ErrKnowledgeBaseNotFound
	}
	return s.addFiles(ctx, item, actorUserID, fileIDs)
}

// RemoveUserFile 将文件移出个人知识库。
func (s *Service) RemoveUserFile(ctx context.Context, userID uint, publicID string, fileID string) error {
	if userID == 0 {
		return ErrInvalidKnowledgeBase
	}
	item, err := s.get(ctx, publicID)
	if err != nil {
		return err
	}
	if item.Scope != domainknowledgebase.ScopeUser || item.OwnerUserID != userID {
		return ErrKnowledgeBaseNotFound
	}
	return mapFileRepositoryError(s.repo.RemoveKnowledgeBaseFile(ctx, item.ID, strings.TrimSpace(fileID)))
}

// RemoveBuiltinFile 将文件移出内置知识库。
func (s *Service) RemoveBuiltinFile(ctx context.Context, publicID string, fileID string) error {
	item, err := s.get(ctx, publicID)
	if err != nil {
		return err
	}
	if item.Scope != domainknowledgebase.ScopeBuiltin {
		return ErrKnowledgeBaseNotFound
	}
	return mapFileRepositoryError(s.repo.RemoveKnowledgeBaseFile(ctx, item.ID, strings.TrimSpace(fileID)))
}

// ResolveFiles 解析一组当前用户可使用的知识库及其文件，供会话检索使用。
func (s *Service) ResolveFiles(ctx context.Context, userID uint, publicIDs []string) ([]domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error) {
	ids := normalizePublicIDs(publicIDs, maxKnowledgeBasesPerRequest)
	if userID == 0 || len(ids) == 0 || len(ids) != len(publicIDs) {
		return nil, nil, domainknowledgebase.ErrReferenceUnavailable
	}
	bases, files, err := s.repo.ResolveVisibleKnowledgeBaseFiles(ctx, userID, ids)
	if err != nil {
		mapped := mapRepositoryError(err)
		if errors.Is(mapped, ErrKnowledgeBaseNotFound) || errors.Is(mapped, ErrInvalidKnowledgeBase) {
			return nil, nil, domainknowledgebase.ErrReferenceUnavailable
		}
		return nil, nil, mapped
	}
	return bases, files, nil
}

func (s *Service) get(ctx context.Context, publicID string) (*domainknowledgebase.KnowledgeBase, error) {
	if strings.TrimSpace(publicID) == "" {
		return nil, ErrInvalidKnowledgeBase
	}
	item, err := s.repo.GetKnowledgeBaseByPublicID(ctx, strings.TrimSpace(publicID))
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return item, nil
}

func (s *Service) getForAccess(ctx context.Context, publicID string) (*domainknowledgebase.KnowledgeBase, error) {
	if strings.TrimSpace(publicID) == "" {
		return nil, ErrInvalidKnowledgeBase
	}
	item, err := s.repo.GetKnowledgeBaseAccessByPublicID(ctx, strings.TrimSpace(publicID))
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return item, nil
}

func isVisibleToUser(item *domainknowledgebase.KnowledgeBase, userID uint) bool {
	return item != nil && item.Enabled && (item.Scope == domainknowledgebase.ScopeBuiltin || (item.Scope == domainknowledgebase.ScopeUser && item.OwnerUserID == userID))
}

func (s *Service) create(ctx context.Context, item *domainknowledgebase.KnowledgeBase) (*domainknowledgebase.KnowledgeBase, error) {
	result, err := s.repo.CreateKnowledgeBase(ctx, item)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return result, nil
}

func (s *Service) update(ctx context.Context, id uint, actorUserID uint, input PatchInput) (*domainknowledgebase.KnowledgeBase, error) {
	patch, err := normalizePatchInput(input, actorUserID)
	if err != nil {
		return nil, err
	}
	result, err := s.repo.PatchKnowledgeBase(ctx, id, patch)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return result, nil
}

func (s *Service) addFiles(ctx context.Context, item *domainknowledgebase.KnowledgeBase, actorUserID uint, fileIDs []string) error {
	ids := normalizePublicIDs(fileIDs, maxFilesPerAddRequest)
	if len(ids) == 0 || len(ids) != len(fileIDs) {
		return ErrInvalidKnowledgeBase
	}
	if err := s.repo.AddKnowledgeBaseFiles(ctx, item.ID, item.Scope, item.OwnerUserID, actorUserID, ids); err != nil {
		return mapFileRepositoryError(err)
	}
	return nil
}

func normalizeWriteInput(input WriteInput, scope string, ownerUserID uint, actorUserID uint) (*domainknowledgebase.KnowledgeBase, error) {
	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)
	if name == "" || utf8.RuneCountInString(name) > maxKnowledgeBaseNameLength || utf8.RuneCountInString(description) > maxKnowledgeBaseDescriptionLength {
		return nil, ErrInvalidKnowledgeBase
	}
	if scope != domainknowledgebase.ScopeBuiltin && scope != domainknowledgebase.ScopeUser {
		return nil, ErrInvalidKnowledgeBase
	}
	return &domainknowledgebase.KnowledgeBase{
		PublicID: conv.NormalizePublicID(uuid.NewString()), Scope: scope, OwnerUserID: ownerUserID,
		Name: name, Description: description, Enabled: input.Enabled, SortOrder: input.SortOrder, Revision: 1,
		CreatedByUserID: actorUserID, UpdatedByUserID: actorUserID,
	}, nil
}

func normalizePatchInput(input PatchInput, actorUserID uint) (repository.KnowledgeBasePatch, error) {
	patch := repository.KnowledgeBasePatch{UpdatedByUserIDSet: true, UpdatedByUserID: actorUserID}
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		if value == "" || utf8.RuneCountInString(value) > maxKnowledgeBaseNameLength {
			return repository.KnowledgeBasePatch{}, ErrInvalidKnowledgeBase
		}
		patch.Name = &value
	}
	if input.Description != nil {
		value := strings.TrimSpace(*input.Description)
		if utf8.RuneCountInString(value) > maxKnowledgeBaseDescriptionLength {
			return repository.KnowledgeBasePatch{}, ErrInvalidKnowledgeBase
		}
		patch.Description = &value
	}
	patch.Enabled = input.Enabled
	patch.SortOrder = input.SortOrder
	if patch.Name == nil && patch.Description == nil && patch.Enabled == nil && patch.SortOrder == nil {
		return repository.KnowledgeBasePatch{}, ErrInvalidKnowledgeBase
	}
	return patch, nil
}

func normalizePage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	return (page - 1) * pageSize, pageSize
}

func normalizePublicIDs(values []string, limit int) []string {
	if len(values) > limit {
		return nil
	}
	return uniqueNonEmpty(values)
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func mapRepositoryError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return ErrKnowledgeBaseNotFound
	case errors.Is(err, repository.ErrDuplicate):
		return ErrKnowledgeBaseConflict
	case errors.Is(err, repository.ErrInvalidInput):
		return ErrInvalidKnowledgeBase
	default:
		return err
	}
}

func mapFileRepositoryError(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ErrKnowledgeBaseFileNotFound
	}
	return mapRepositoryError(err)
}
