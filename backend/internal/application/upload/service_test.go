package upload

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstore"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestUploadFileReturnsExistingActiveDuplicate(t *testing.T) {
	ctx := context.Background()
	repo := newUploadTestRepo()
	store := newUploadTestStore()
	service := newUploadTestService(repo, store)

	first, err := service.UploadFile(ctx, uploadTestInput("notes.md", "same content"))
	if err != nil {
		t.Fatalf("first upload failed: %v", err)
	}
	second, err := service.UploadFile(ctx, uploadTestInput("copy.md", "same content"))
	if err != nil {
		t.Fatalf("second upload failed: %v", err)
	}

	if second.File.FileID != first.File.FileID {
		t.Fatalf("duplicate upload should return existing file id, got %s want %s", second.File.FileID, first.File.FileID)
	}
	if !second.Reused {
		t.Fatal("duplicate upload should be marked reused")
	}
	if got := repo.activeFileCount(); got != 1 {
		t.Fatalf("duplicate upload should not create a second active row, got %d", got)
	}
	if got := store.objectCount(); got != 1 {
		t.Fatalf("duplicate upload should remove the transient object, got %d stored objects", got)
	}
	if second.Quota.UsedBytes != int64(len("same content")) {
		t.Fatalf("duplicate upload should not consume quota twice, got %d", second.Quota.UsedBytes)
	}
	if second.File.LastAccessedAt == nil {
		t.Fatal("duplicate upload should touch the existing file access time")
	}
}

func TestUploadFileAllowsReuploadAfterDelete(t *testing.T) {
	ctx := context.Background()
	repo := newUploadTestRepo()
	store := newUploadTestStore()
	service := newUploadTestService(repo, store)

	first, err := service.UploadFile(ctx, uploadTestInput("notes.md", "same content"))
	if err != nil {
		t.Fatalf("first upload failed: %v", err)
	}
	if _, err = service.DeleteFile(ctx, 1, first.File.FileID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	second, err := service.UploadFile(ctx, uploadTestInput("notes.md", "same content"))
	if err != nil {
		t.Fatalf("reupload failed: %v", err)
	}

	if second.File.FileID == first.File.FileID {
		t.Fatal("reupload after delete should create a fresh logical file")
	}
	if second.Reused {
		t.Fatal("reupload after delete should not be marked reused")
	}
	if got := repo.activeFileCount(); got != 1 {
		t.Fatalf("reupload after delete should leave one active row, got %d", got)
	}
	if got := store.objectCount(); got != 1 {
		t.Fatalf("reupload after delete should leave one physical object, got %d", got)
	}
	if second.Quota.UsedBytes != int64(len("same content")) {
		t.Fatalf("reupload quota mismatch, got %d", second.Quota.UsedBytes)
	}
}

func TestDeleteFileIfUnreferencedSkipsReferencedFile(t *testing.T) {
	ctx := context.Background()
	repo := newUploadTestRepo()
	store := newUploadTestStore()
	service := newUploadTestService(repo, store)

	uploaded, err := service.UploadFile(ctx, uploadTestInput("notes.md", "same content"))
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	repo.referencedFileIDs[uploaded.File.FileID] = true

	result, deleted, err := service.DeleteFileIfUnreferenced(ctx, 1, uploaded.File.FileID)
	if err != nil {
		t.Fatalf("delete if unreferenced failed: %v", err)
	}
	if deleted || result != nil {
		t.Fatal("referenced file should be skipped without returning a delete result")
	}
	if status := repo.fileStatus(uploaded.File.FileID); status != "active" {
		t.Fatalf("referenced file should remain active, got %q", status)
	}
	if got := store.objectCount(); got != 1 {
		t.Fatalf("referenced file should keep physical object, got %d objects", got)
	}
}

func TestUploadFileReplacesStaleDuplicatePointer(t *testing.T) {
	ctx := context.Background()
	repo := newUploadTestRepo()
	store := newUploadTestStore()
	service := newUploadTestService(repo, store)

	stale, err := service.UploadFile(ctx, uploadTestInput("notes.md", "same content"))
	if err != nil {
		t.Fatalf("seed upload failed: %v", err)
	}
	if err = store.Delete(ctx, stale.File.StoragePath); err != nil {
		t.Fatalf("delete physical object failed: %v", err)
	}

	fresh, err := service.UploadFile(ctx, uploadTestInput("notes.md", "same content"))
	if err != nil {
		t.Fatalf("reupload with stale pointer failed: %v", err)
	}

	if fresh.File.FileID == stale.File.FileID {
		t.Fatal("stale duplicate pointer should not be reused")
	}
	if fresh.Reused {
		t.Fatal("stale duplicate pointer replacement should not be marked reused")
	}
	if status := repo.fileStatus(stale.File.FileID); status != "deleted" {
		t.Fatalf("stale pointer should be marked deleted, got %q", status)
	}
	if got := repo.activeFileCount(); got != 1 {
		t.Fatalf("stale cleanup should leave one active row, got %d", got)
	}
	if got := store.objectCount(); got != 1 {
		t.Fatalf("stale cleanup should leave one physical object, got %d", got)
	}
	if fresh.Quota.UsedBytes != int64(len("same content")) {
		t.Fatalf("stale cleanup quota mismatch, got %d", fresh.Quota.UsedBytes)
	}
}

func TestUploadFileReusesAfterConcurrentDuplicateConflict(t *testing.T) {
	ctx := context.Background()
	repo := newUploadTestRepo()
	store := newUploadTestStore()
	service := newUploadTestService(repo, store)

	existing, err := service.UploadFile(ctx, uploadTestInput("notes.md", "same content"))
	if err != nil {
		t.Fatalf("seed upload failed: %v", err)
	}
	repo.missNextDuplicateLookup = true
	repo.failNextCreateDuplicate = true

	result, err := service.UploadFile(ctx, uploadTestInput("copy.md", "same content"))
	if err != nil {
		t.Fatalf("duplicate conflict upload failed: %v", err)
	}

	if result.File.FileID != existing.File.FileID {
		t.Fatalf("duplicate conflict should return existing file id, got %s want %s", result.File.FileID, existing.File.FileID)
	}
	if !result.Reused {
		t.Fatal("duplicate conflict should be marked reused")
	}
	if got := repo.activeFileCount(); got != 1 {
		t.Fatalf("duplicate conflict should not create a second active row, got %d", got)
	}
	if got := store.objectCount(); got != 1 {
		t.Fatalf("duplicate conflict should remove the transient object, got %d stored objects", got)
	}
}

func TestUploadFileRetriesCreateAfterStaleDuplicateConflict(t *testing.T) {
	ctx := context.Background()
	repo := newUploadTestRepo()
	store := newUploadTestStore()
	service := newUploadTestService(repo, store)

	stale, err := service.UploadFile(ctx, uploadTestInput("notes.md", "same content"))
	if err != nil {
		t.Fatalf("seed upload failed: %v", err)
	}
	if err = store.Delete(ctx, stale.File.StoragePath); err != nil {
		t.Fatalf("delete physical object failed: %v", err)
	}
	repo.missNextDuplicateLookup = true
	repo.failNextCreateDuplicate = true

	result, err := service.UploadFile(ctx, uploadTestInput("copy.md", "same content"))
	if err != nil {
		t.Fatalf("stale duplicate conflict upload failed: %v", err)
	}

	if result.File.FileID == stale.File.FileID {
		t.Fatal("stale duplicate conflict should create a fresh file")
	}
	if result.Reused {
		t.Fatal("stale duplicate conflict replacement should not be marked reused")
	}
	if status := repo.fileStatus(stale.File.FileID); status != "deleted" {
		t.Fatalf("stale pointer should be marked deleted, got %q", status)
	}
	if got := repo.activeFileCount(); got != 1 {
		t.Fatalf("stale duplicate conflict should leave one active row, got %d", got)
	}
	if got := store.objectCount(); got != 1 {
		t.Fatalf("stale duplicate conflict should leave one physical object, got %d", got)
	}
}

func TestUploadFileAllowsUnlimitedUserStorageQuota(t *testing.T) {
	ctx := context.Background()
	repo := newUploadTestRepo()
	store := newUploadTestStore()
	service := newUploadTestService(repo, store)
	cfg := service.cfg.Snapshot()
	cfg.UserStorageQuotaBytes = 0
	cfg.MaxUploadFileBytes = 1024 * 1024
	service.cfg.Store(cfg)
	repo.quota.QuotaBytes = 1

	result, err := service.UploadFile(ctx, uploadTestInput("notes.md", "content"))
	if err != nil {
		t.Fatalf("unlimited quota upload failed: %v", err)
	}
	if result.Quota.QuotaBytes != 0 {
		t.Fatalf("expected quota limit to sync to unlimited, got %d", result.Quota.QuotaBytes)
	}
	if result.Quota.UsedBytes != result.File.SizeBytes {
		t.Fatalf("expected used bytes to increase, got quota=%#v file=%#v", result.Quota, result.File)
	}
}

func TestNormalizeDetectedMIMEDowngradesActiveContent(t *testing.T) {
	tests := []struct {
		detected string
		fileName string
	}{
		{detected: "text/html; charset=utf-8", fileName: "safe.txt"},
		{detected: "text/plain", fileName: "index.html"},
		{detected: "application/javascript", fileName: "script.js"},
		{detected: "image/svg+xml", fileName: "icon.svg"},
	}
	for _, tt := range tests {
		if got := normalizeDetectedMIME(tt.detected, tt.fileName); got != "text/plain" {
			t.Fatalf("normalizeDetectedMIME(%q, %q) = %q, want text/plain", tt.detected, tt.fileName, got)
		}
	}
}

func newUploadTestService(repo *uploadTestRepo, store *uploadTestStore) *Service {
	cfg := config.Config{
		MaxUploadFileBytes:    1024 * 1024,
		UserStorageQuotaBytes: 10 * 1024 * 1024,
		FileAllowedMIMETypes:  "",
	}
	service := NewServiceWithRuntime(config.NewRuntime(cfg), repo, nil, Hooks{}, ErrorSet{
		InvalidFileReference: repository.ErrInvalidInput,
		InvalidFileName:      repository.ErrInvalidInput,
		StorageQuotaExceeded: repository.ErrConflict,
		FileTooLarge:         repository.ErrInvalidInput,
		MIMEBlocked:          repository.ErrInvalidInput,
		DangerousMIMEType:    repository.ErrInvalidInput,
	}, "test")
	service.SetObjectStoreProvider(uploadTestStoreProvider{store: store})
	return service
}

func uploadTestInput(fileName string, content string) UploadFileInput {
	return UploadFileInput{
		UserID:       1,
		Purpose:      "chat",
		FileName:     fileName,
		MimeType:     "text/plain",
		DeclaredSize: int64(len(content)),
		Reader:       strings.NewReader(content),
	}
}

type uploadTestStoreProvider struct {
	store *uploadTestStore
}

func (p uploadTestStoreProvider) Open(ctx context.Context) (objectstore.Store, error) {
	_ = ctx
	return p.store, nil
}

type uploadTestStore struct {
	objects map[string][]byte
}

func newUploadTestStore() *uploadTestStore {
	return &uploadTestStore{objects: map[string][]byte{}}
}

func (s *uploadTestStore) Put(ctx context.Context, key string, body io.Reader, opts objectstore.PutOptions) (objectstore.ObjectInfo, error) {
	_ = ctx
	data, err := io.ReadAll(body)
	if err != nil {
		return objectstore.ObjectInfo{}, err
	}
	s.objects[key] = append([]byte(nil), data...)
	return objectstore.ObjectInfo{Key: key, SizeBytes: int64(len(data)), ContentType: opts.ContentType, ModTime: time.Now()}, nil
}

func (s *uploadTestStore) Open(ctx context.Context, key string) (io.ReadCloser, objectstore.ObjectInfo, error) {
	_ = ctx
	data, ok := s.objects[key]
	if !ok {
		return nil, objectstore.ObjectInfo{}, objectstore.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), objectstore.ObjectInfo{Key: key, SizeBytes: int64(len(data)), ModTime: time.Now()}, nil
}

func (s *uploadTestStore) Delete(ctx context.Context, key string) error {
	_ = ctx
	delete(s.objects, key)
	return nil
}

func (s *uploadTestStore) Materialize(ctx context.Context, key string) (string, func(), error) {
	_ = ctx
	if _, ok := s.objects[key]; !ok {
		return "", nil, objectstore.ErrNotFound
	}
	return key, func() {}, nil
}

func (s *uploadTestStore) objectCount() int {
	return len(s.objects)
}

type uploadTestRepo struct {
	user                    domainuser.User
	nextID                  uint
	files                   []domainconversation.FileObject
	quota                   domainconversation.StorageQuota
	missNextDuplicateLookup bool
	failNextCreateDuplicate bool
	referencedFileIDs       map[string]bool
}

func newUploadTestRepo() *uploadTestRepo {
	return &uploadTestRepo{
		user: domainuser.User{ID: 1, PublicID: "user_1", Status: domainuser.StatusActive},
		quota: domainconversation.StorageQuota{
			UserID:     1,
			QuotaBytes: 10 * 1024 * 1024,
		},
		referencedFileIDs: make(map[string]bool),
	}
}

func (r *uploadTestRepo) ListFileObjectsByUserWithFilter(context.Context, uint, int, int, string, string, string) ([]domainconversation.FileObject, int64, error) {
	return nil, 0, nil
}

func (r *uploadTestRepo) MarkTimedOutFileEmbeddingsFailed(context.Context, uint, time.Time, string) (int64, error) {
	return 0, nil
}

func (r *uploadTestRepo) GetActiveFileObjectByID(_ context.Context, userID uint, fileID string) (*domainconversation.FileObject, error) {
	for i := range r.files {
		if r.files[i].UserID == userID && r.files[i].FileID == fileID && r.files[i].Status == "active" {
			result := r.files[i]
			return &result, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *uploadTestRepo) RenameFileObjectByID(context.Context, uint, string, string) (*domainconversation.FileObject, error) {
	return nil, nil
}

func (r *uploadTestRepo) UpdateFileObjectRagOptOut(context.Context, uint, string, bool) (*domainconversation.FileObject, error) {
	return nil, nil
}

func (r *uploadTestRepo) TouchFileObjectLastAccessedAt(_ context.Context, userID uint, fileID string, accessedAt time.Time) error {
	for i := range r.files {
		if r.files[i].UserID == userID && r.files[i].FileID == fileID && r.files[i].Status == "active" {
			r.files[i].LastAccessedAt = &accessedAt
			return nil
		}
	}
	return repository.ErrNotFound
}

func (r *uploadTestRepo) GetUserByID(context.Context, uint) (*domainuser.User, error) {
	result := r.user
	return &result, nil
}

func (r *uploadTestRepo) GetLatestActiveFileObjectBySHA(_ context.Context, userID uint, sha256 string, sizeBytes int64) (*domainconversation.FileObject, error) {
	if r.missNextDuplicateLookup {
		r.missNextDuplicateLookup = false
		return nil, nil
	}
	for i := len(r.files) - 1; i >= 0; i-- {
		item := r.files[i]
		if item.UserID == userID && item.Status == "active" && item.SHA256 == sha256 && item.SizeBytes == sizeBytes {
			result := item
			return &result, nil
		}
	}
	return nil, nil
}

func (r *uploadTestRepo) CreateFileObjectAndConsumeQuota(_ context.Context, item *domainconversation.FileObject, quotaLimit int64) (*domainconversation.StorageQuota, error) {
	if r.failNextCreateDuplicate {
		r.failNextCreateDuplicate = false
		return nil, repository.ErrDuplicate
	}
	if quotaLimit < 0 {
		quotaLimit = 0
	}
	r.quota.QuotaBytes = quotaLimit
	nextUsed := r.quota.UsedBytes + item.SizeBytes
	if r.quota.QuotaBytes > 0 && nextUsed > r.quota.QuotaBytes {
		return nil, repository.ErrConflict
	}
	r.nextID++
	now := time.Now()
	item.ID = r.nextID
	item.CreatedAt = now
	item.UpdatedAt = now
	r.files = append(r.files, *item)
	r.quota.UsedBytes = nextUsed
	r.quota.UpdatedAt = now
	return cloneQuota(r.quota), nil
}

func (r *uploadTestRepo) DeleteFileObjectAndReleaseQuota(_ context.Context, userID uint, fileID string, quotaLimit int64, options repository.DeleteFileObjectOptions) (*domainconversation.FileObject, *domainconversation.StorageQuota, bool, error) {
	if quotaLimit < 0 {
		quotaLimit = 0
	}
	r.quota.QuotaBytes = quotaLimit
	for i := range r.files {
		if r.files[i].UserID != userID || r.files[i].FileID != fileID || r.files[i].Status != "active" {
			continue
		}
		if options.RequireUnreferenced && r.referencedFileIDs[fileID] {
			return nil, nil, false, repository.ErrConflict
		}
		deleted := r.files[i]
		r.files[i].Status = "deleted"
		remainingRefs := 0
		for j := range r.files {
			if i != j && r.files[j].Status == "active" && r.files[j].StoragePath == deleted.StoragePath {
				remainingRefs++
			}
		}
		if remainingRefs == 0 {
			r.quota.UsedBytes -= deleted.SizeBytes
			if r.quota.UsedBytes < 0 {
				r.quota.UsedBytes = 0
			}
		}
		return &deleted, cloneQuota(r.quota), remainingRefs == 0, nil
	}
	return nil, nil, false, repository.ErrNotFound
}

func (r *uploadTestRepo) GetOrInitUserStorageQuota(_ context.Context, _ uint, quotaLimit int64) (*domainconversation.StorageQuota, error) {
	if quotaLimit < 0 {
		quotaLimit = 0
	}
	r.quota.QuotaBytes = quotaLimit
	return cloneQuota(r.quota), nil
}

func (r *uploadTestRepo) activeFileCount() int {
	count := 0
	for _, item := range r.files {
		if item.Status == "active" {
			count++
		}
	}
	return count
}

func (r *uploadTestRepo) fileStatus(fileID string) string {
	for _, item := range r.files {
		if item.FileID == fileID {
			return item.Status
		}
	}
	return ""
}

func cloneQuota(q domainconversation.StorageQuota) *domainconversation.StorageQuota {
	result := q
	return &result
}

var _ repository.UploadRepository = (*uploadTestRepo)(nil)
var _ objectstore.Store = (*uploadTestStore)(nil)
