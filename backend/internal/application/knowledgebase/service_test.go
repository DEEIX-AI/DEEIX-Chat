package knowledgebase

import (
	"context"
	"errors"
	"testing"

	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestAddUserFilesRejectsDuplicateIDs(t *testing.T) {
	repo := &knowledgeBaseRepositoryStub{
		item: &domainknowledgebase.KnowledgeBase{ID: 7, Scope: domainknowledgebase.ScopeUser, OwnerUserID: 11},
	}
	service := NewService(repo)

	err := service.AddUserFiles(context.Background(), 11, "kb-one", []string{"file-one", " file-one "})
	if !errors.Is(err, ErrInvalidKnowledgeBase) {
		t.Fatalf("AddUserFiles() error = %v, want ErrInvalidKnowledgeBase", err)
	}
	if repo.addCalls != 0 {
		t.Fatalf("AddKnowledgeBaseFiles() calls = %d, want 0", repo.addCalls)
	}
}

func TestResolveFilesRejectsDuplicateAndOversizedSelections(t *testing.T) {
	repo := &knowledgeBaseRepositoryStub{}
	service := NewService(repo)

	for name, ids := range map[string][]string{
		"duplicate": {"kb-one", "kb-one"},
		"too many":  {"1", "2", "3", "4", "5", "6", "7", "8", "9"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := service.ResolveFiles(context.Background(), 11, ids)
			if !errors.Is(err, domainknowledgebase.ErrReferenceUnavailable) {
				t.Fatalf("ResolveFiles() error = %v, want ErrReferenceUnavailable", err)
			}
		})
	}
	if repo.resolveCalls != 0 {
		t.Fatalf("ResolveVisibleKnowledgeBaseFiles() calls = %d, want 0", repo.resolveCalls)
	}
}

func TestCreateUserAlwaysCreatesEnabledKnowledgeBase(t *testing.T) {
	repo := &knowledgeBaseRepositoryStub{}
	service := NewService(repo)

	item, err := service.CreateUser(context.Background(), 11, WriteInput{Name: "Personal", Enabled: false})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if item == nil || !item.Enabled {
		t.Fatalf("CreateUser() item = %#v, want enabled personal knowledge base", item)
	}
}

func TestUpdateUserRejectsAvailabilityPatch(t *testing.T) {
	repo := &knowledgeBaseRepositoryStub{}
	service := NewService(repo)
	enabled := false

	_, err := service.UpdateUser(context.Background(), 11, "kb-one", PatchInput{Enabled: &enabled})
	if !errors.Is(err, ErrInvalidKnowledgeBase) {
		t.Fatalf("UpdateUser() error = %v, want ErrInvalidKnowledgeBase", err)
	}
}

func TestListAdminFilesPreservesRepositoryFailure(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repo := &knowledgeBaseRepositoryStub{getErr: wantErr}
	service := NewService(repo)

	_, _, err := service.ListAdminFiles(context.Background(), "kb-one", 1, 20)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ListAdminFiles() error = %v, want %v", err, wantErr)
	}
}

func TestListAvailableUserFilesUsesOwnerAndQuery(t *testing.T) {
	repo := &knowledgeBaseRepositoryStub{
		item: &domainknowledgebase.KnowledgeBase{ID: 7, Scope: domainknowledgebase.ScopeUser, OwnerUserID: 11},
	}
	service := NewService(repo)

	_, _, err := service.ListAvailableUserFiles(context.Background(), 11, "kb-one", ListInput{
		Query: " policy ", Page: 2, PageSize: 25,
	})
	if err != nil {
		t.Fatalf("ListAvailableUserFiles() error = %v", err)
	}
	if repo.availableKnowledgeBaseID != 7 || repo.availableOwnerUserID != 11 || repo.availableQuery != "policy" || repo.availableOffset != 25 || repo.availableLimit != 25 {
		t.Fatalf("ListAvailableKnowledgeBaseFiles() args = kb %d owner %d query %q offset %d limit %d",
			repo.availableKnowledgeBaseID, repo.availableOwnerUserID, repo.availableQuery, repo.availableOffset, repo.availableLimit)
	}
}

func TestListAvailableAdminFilesUsesPlatformOwner(t *testing.T) {
	repo := &knowledgeBaseRepositoryStub{
		item: &domainknowledgebase.KnowledgeBase{ID: 7, Scope: domainknowledgebase.ScopeBuiltin},
	}
	service := NewService(repo)

	_, _, err := service.ListAvailableAdminFiles(context.Background(), 11, "kb-one", ListInput{})
	if err != nil {
		t.Fatalf("ListAvailableAdminFiles() error = %v", err)
	}
	if repo.availableOwnerUserID != 0 {
		t.Fatalf("ListAvailableKnowledgeBaseFiles() owner = %d, want platform owner 0", repo.availableOwnerUserID)
	}
}

func TestListPlatformFilesUsesPlatformOwnerAndPaging(t *testing.T) {
	repo := &knowledgeBaseRepositoryStub{}
	service := NewService(repo)

	_, _, err := service.ListPlatformFiles(context.Background(), 11, ListInput{
		Query: " policy ", Page: 2, PageSize: 25,
	})
	if err != nil {
		t.Fatalf("ListPlatformFiles() error = %v", err)
	}
	if repo.sourceOwnerUserID != 0 || repo.sourceQuery != "policy" || repo.sourceOffset != 25 || repo.sourceLimit != 25 {
		t.Fatalf("ListKnowledgeBaseSourceFiles() args = owner %d query %q offset %d limit %d",
			repo.sourceOwnerUserID, repo.sourceQuery, repo.sourceOffset, repo.sourceLimit)
	}
}

func TestUploadBuiltinFileUsesSystemOwnership(t *testing.T) {
	uploader := &knowledgeBaseFileUploaderStub{result: &appupload.UploadFileResult{}}
	service := NewService(&knowledgeBaseRepositoryStub{})
	service.SetFileUploader(uploader)

	_, err := service.UploadBuiltinFile(context.Background(), 11, appupload.UploadFileInput{FileName: "policy.md"})
	if err != nil {
		t.Fatalf("UploadBuiltinFile() error = %v", err)
	}
	if uploader.input.UserID != 11 || uploader.input.Ownership != appupload.FileOwnershipSystem || uploader.input.Purpose != "knowledge_base" {
		t.Fatalf("UploadFile() input = %#v, want actor 11 with system ownership", uploader.input)
	}
}

func TestDeletePlatformFileUsesPlatformOwner(t *testing.T) {
	cleaner := &knowledgeBaseFileCleanerStub{deleted: map[string]bool{"file-one": true}}
	service := NewService(&knowledgeBaseRepositoryStub{})
	service.SetFileCleaner(cleaner)

	if err := service.DeletePlatformFile(context.Background(), 11, " file-one "); err != nil {
		t.Fatalf("DeletePlatformFile() error = %v", err)
	}
	if cleaner.calls != 1 || cleaner.userID != 0 || cleaner.fileID != "file-one" {
		t.Fatalf("DeleteFileIfUnreferenced() = calls %d owner %d file %q, want 1/0/file-one", cleaner.calls, cleaner.userID, cleaner.fileID)
	}
}

func TestDeletePlatformFileRejectsReferencedFile(t *testing.T) {
	cleaner := &knowledgeBaseFileCleanerStub{deleted: map[string]bool{}}
	service := NewService(&knowledgeBaseRepositoryStub{})
	service.SetFileCleaner(cleaner)

	err := service.DeletePlatformFile(context.Background(), 11, "file-one")
	if !errors.Is(err, ErrPlatformFileInUse) {
		t.Fatalf("DeletePlatformFile() error = %v, want ErrPlatformFileInUse", err)
	}
}

func TestOpenPlatformFileContentUsesPlatformOwner(t *testing.T) {
	opener := &knowledgeBaseFileOpenerStub{result: &appupload.FileContentResult{}}
	service := NewService(&knowledgeBaseRepositoryStub{})
	service.SetFileContentOpener(opener)

	result, err := service.OpenPlatformFileContent(context.Background(), 11, " file-one ")
	if err != nil {
		t.Fatalf("OpenPlatformFileContent() error = %v", err)
	}
	if result != opener.result || opener.userID != 0 || opener.fileID != "file-one" {
		t.Fatalf("OpenPlatformFileContent() opener = user %d file %q, want user 0 file-one", opener.userID, opener.fileID)
	}
}

func TestOpenPlatformFileContentMapsMissingFile(t *testing.T) {
	opener := &knowledgeBaseFileOpenerStub{err: repository.ErrNotFound}
	service := NewService(&knowledgeBaseRepositoryStub{})
	service.SetFileContentOpener(opener)

	_, err := service.OpenPlatformFileContent(context.Background(), 11, "file-one")
	if !errors.Is(err, ErrKnowledgeBaseFileNotFound) {
		t.Fatalf("OpenPlatformFileContent() error = %v, want ErrKnowledgeBaseFileNotFound", err)
	}
}

func TestOpenVisibleFileContentUsesLinkedFileOwner(t *testing.T) {
	repo := &knowledgeBaseRepositoryStub{
		item: &domainknowledgebase.KnowledgeBase{
			ID: 7, PublicID: "kb-one", Scope: domainknowledgebase.ScopeBuiltin, Enabled: true,
		},
		file: &domainconversation.FileObject{FileID: "file-one", UserID: 19},
	}
	opener := &knowledgeBaseFileOpenerStub{result: &appupload.FileContentResult{}}
	service := NewService(repo)
	service.SetFileContentOpener(opener)

	result, err := service.OpenVisibleFileContent(context.Background(), 11, "kb-one", "file-one")
	if err != nil {
		t.Fatalf("OpenVisibleFileContent() error = %v", err)
	}
	if result != opener.result || opener.userID != 19 || opener.fileID != "file-one" {
		t.Fatalf("OpenVisibleFileContent() opener = user %d file %q, want user 19 file-one", opener.userID, opener.fileID)
	}
}

func TestOpenVisibleFileContentRejectsUnlinkedFile(t *testing.T) {
	repo := &knowledgeBaseRepositoryStub{
		item:    &domainknowledgebase.KnowledgeBase{ID: 7, Scope: domainknowledgebase.ScopeBuiltin, Enabled: true},
		fileErr: repository.ErrNotFound,
	}
	opener := &knowledgeBaseFileOpenerStub{}
	service := NewService(repo)
	service.SetFileContentOpener(opener)

	_, err := service.OpenVisibleFileContent(context.Background(), 11, "kb-one", "file-missing")
	if !errors.Is(err, ErrKnowledgeBaseFileNotFound) {
		t.Fatalf("OpenVisibleFileContent() error = %v, want ErrKnowledgeBaseFileNotFound", err)
	}
	if opener.calls != 0 {
		t.Fatalf("OpenFileContent() calls = %d, want 0", opener.calls)
	}
}

func TestDeleteUserOptionallyCleansOnlyUnreferencedFiles(t *testing.T) {
	repo := &knowledgeBaseRepositoryStub{
		item: &domainknowledgebase.KnowledgeBase{ID: 7, Scope: domainknowledgebase.ScopeUser, OwnerUserID: 11},
		deleteFiles: []repository.KnowledgeBaseFileCleanupCandidate{
			{UserID: 11, FileID: "file-delete"},
			{UserID: 11, FileID: "file-retain"},
		},
	}
	cleaner := &knowledgeBaseFileCleanerStub{deleted: map[string]bool{"file-delete": true}}
	service := NewService(repo)
	service.SetFileCleaner(cleaner)

	result, err := service.DeleteUser(context.Background(), 11, "kb-one", true)
	if err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if result.DeletedFileCount != 1 {
		t.Fatalf("DeleteUser() deleted file count = %d, want 1", result.DeletedFileCount)
	}
	if cleaner.calls != 2 {
		t.Fatalf("DeleteFileIfUnreferenced() calls = %d, want 2", cleaner.calls)
	}
}

func TestDeleteUserKeepsFilesByDefault(t *testing.T) {
	repo := &knowledgeBaseRepositoryStub{
		item:        &domainknowledgebase.KnowledgeBase{ID: 7, Scope: domainknowledgebase.ScopeUser, OwnerUserID: 11},
		deleteFiles: []repository.KnowledgeBaseFileCleanupCandidate{{UserID: 11, FileID: "file-one"}},
	}
	cleaner := &knowledgeBaseFileCleanerStub{deleted: map[string]bool{"file-one": true}}
	service := NewService(repo)
	service.SetFileCleaner(cleaner)

	result, err := service.DeleteUser(context.Background(), 11, "kb-one", false)
	if err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if result.DeletedFileCount != 0 || cleaner.calls != 0 {
		t.Fatalf("DeleteUser() result = %#v, cleaner calls = %d, want no file cleanup", result, cleaner.calls)
	}
}

func TestDeleteUserRequiresCleanerBeforeDeletingFiles(t *testing.T) {
	repo := &knowledgeBaseRepositoryStub{
		item: &domainknowledgebase.KnowledgeBase{
			ID: 7, Scope: domainknowledgebase.ScopeUser, OwnerUserID: 11,
		},
	}
	service := NewService(repo)

	_, err := service.DeleteUser(context.Background(), 11, "kb-one", true)
	if !errors.Is(err, ErrKnowledgeBaseFileCleanupUnavailable) {
		t.Fatalf("DeleteUser() error = %v, want ErrKnowledgeBaseFileCleanupUnavailable", err)
	}
	if repo.deleteCalls != 0 {
		t.Fatalf("DeleteKnowledgeBase() calls = %d, want 0", repo.deleteCalls)
	}
}

type knowledgeBaseFileCleanerStub struct {
	deleted map[string]bool
	calls   int
	userID  uint
	fileID  string
	err     error
}

type knowledgeBaseFileOpenerStub struct {
	result *appupload.FileContentResult
	err    error
	userID uint
	fileID string
	calls  int
}

type knowledgeBaseFileUploaderStub struct {
	input  appupload.UploadFileInput
	result *appupload.UploadFileResult
	err    error
}

func (s *knowledgeBaseFileUploaderStub) UploadFile(_ context.Context, input appupload.UploadFileInput) (*appupload.UploadFileResult, error) {
	s.input = input
	return s.result, s.err
}

func (s *knowledgeBaseFileOpenerStub) OpenFileContent(_ context.Context, userID uint, fileID string) (*appupload.FileContentResult, error) {
	s.calls++
	s.userID = userID
	s.fileID = fileID
	return s.result, s.err
}

func (s *knowledgeBaseFileCleanerStub) DeleteFileIfUnreferenced(_ context.Context, userID uint, fileID string) (bool, error) {
	s.calls++
	s.userID = userID
	s.fileID = fileID
	return s.deleted[fileID], s.err
}

type knowledgeBaseRepositoryStub struct {
	item                     *domainknowledgebase.KnowledgeBase
	getErr                   error
	file                     *domainconversation.FileObject
	fileErr                  error
	deleteFiles              []repository.KnowledgeBaseFileCleanupCandidate
	deleteErr                error
	deleteCalls              int
	addCalls                 int
	resolveCalls             int
	availableKnowledgeBaseID uint
	availableOwnerUserID     uint
	availableQuery           string
	availableOffset          int
	availableLimit           int
	sourceOwnerUserID        uint
	sourceQuery              string
	sourceOffset             int
	sourceLimit              int
}

func (s *knowledgeBaseRepositoryStub) ListKnowledgeBases(context.Context, repository.KnowledgeBaseListFilter, int, int) ([]domainknowledgebase.KnowledgeBase, int64, error) {
	return nil, 0, nil
}

func (s *knowledgeBaseRepositoryStub) GetKnowledgeBaseByPublicID(context.Context, string) (*domainknowledgebase.KnowledgeBase, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.item == nil {
		return nil, repository.ErrNotFound
	}
	item := *s.item
	return &item, nil
}

func (s *knowledgeBaseRepositoryStub) GetKnowledgeBaseAccessByPublicID(ctx context.Context, publicID string) (*domainknowledgebase.KnowledgeBase, error) {
	return s.GetKnowledgeBaseByPublicID(ctx, publicID)
}

func (s *knowledgeBaseRepositoryStub) CreateKnowledgeBase(_ context.Context, item *domainknowledgebase.KnowledgeBase) (*domainknowledgebase.KnowledgeBase, error) {
	result := *item
	return &result, nil
}

func (s *knowledgeBaseRepositoryStub) PatchKnowledgeBase(context.Context, uint, repository.KnowledgeBasePatch) (*domainknowledgebase.KnowledgeBase, error) {
	return nil, nil
}

func (s *knowledgeBaseRepositoryStub) DeleteKnowledgeBase(context.Context, uint) ([]repository.KnowledgeBaseFileCleanupCandidate, error) {
	s.deleteCalls++
	return s.deleteFiles, s.deleteErr
}

func (s *knowledgeBaseRepositoryStub) ListKnowledgeBaseFiles(context.Context, uint, int, int) ([]domainconversation.FileObject, int64, error) {
	return nil, 0, nil
}

func (s *knowledgeBaseRepositoryStub) GetKnowledgeBaseFileProcessingStatuses(context.Context, uint, []string) ([]domainconversation.FileObject, error) {
	return nil, nil
}

func (s *knowledgeBaseRepositoryStub) GetKnowledgeBaseFileProcessingSnapshot(context.Context, uint, []string) (*repository.KnowledgeBaseFileProcessingSnapshot, error) {
	return &repository.KnowledgeBaseFileProcessingSnapshot{}, nil
}

func (s *knowledgeBaseRepositoryStub) ListKnowledgeBaseSourceFiles(_ context.Context, ownerUserID uint, query string, offset int, limit int) ([]domainconversation.FileObject, int64, error) {
	s.sourceOwnerUserID = ownerUserID
	s.sourceQuery = query
	s.sourceOffset = offset
	s.sourceLimit = limit
	return nil, 0, nil
}

func (s *knowledgeBaseRepositoryStub) ListAvailableKnowledgeBaseFiles(_ context.Context, knowledgeBaseID uint, ownerUserID uint, query string, offset int, limit int) ([]domainconversation.FileObject, int64, error) {
	s.availableKnowledgeBaseID = knowledgeBaseID
	s.availableOwnerUserID = ownerUserID
	s.availableQuery = query
	s.availableOffset = offset
	s.availableLimit = limit
	return nil, 0, nil
}

func (s *knowledgeBaseRepositoryStub) GetKnowledgeBaseFile(context.Context, uint, string) (*domainconversation.FileObject, error) {
	if s.fileErr != nil {
		return nil, s.fileErr
	}
	if s.file == nil {
		return nil, repository.ErrNotFound
	}
	item := *s.file
	return &item, nil
}

func (s *knowledgeBaseRepositoryStub) AddKnowledgeBaseFiles(context.Context, uint, string, uint, uint, []string) error {
	s.addCalls++
	return nil
}

func (s *knowledgeBaseRepositoryStub) RemoveKnowledgeBaseFile(context.Context, uint, string) error {
	return nil
}

func (s *knowledgeBaseRepositoryStub) ResolveVisibleKnowledgeBaseFiles(context.Context, uint, []string) ([]domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error) {
	s.resolveCalls++
	return nil, nil, nil
}
