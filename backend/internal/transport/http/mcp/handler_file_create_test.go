package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	appmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/mcp"
	appprocessing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/processing"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	memorycache "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	storeinfra "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstorage"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	conversationrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/conversation"
	mcprepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/mcp"
	settingsrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/settings"
	userrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/mcpauth"
	portstore "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/objectstorage"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fileCreateFixture struct {
	db      *gorm.DB
	runtime *config.Runtime
	repo    *conversationrepo.Repo
	servers *mcprepo.Repo
	service *appmcp.FileCreateService
	uploads *appupload.Service
	store   portstore.Store
	root    string
	dsn     string
	server  models.MCPServer
	tool    models.MCPTool
}

type fileCreateStoreProvider struct{ store portstore.Store }

func (p fileCreateStoreProvider) Open(context.Context) (portstore.Store, error) { return p.store, nil }

func openFileCreateDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func newFileCreateFixture(t *testing.T) *fileCreateFixture {
	t.Helper()
	f := &fileCreateFixture{root: t.TempDir()}
	f.dsn = filepath.ToSlash(filepath.Join(t.TempDir(), "files.sqlite")) + "?_busy_timeout=10000&_journal_mode=WAL"
	f.db = openFileCreateDB(t, f.dsn)
	return initializeFileCreateFixture(t, f)
}

func initializeFileCreateFixture(t *testing.T, f *fileCreateFixture) *fileCreateFixture {
	t.Helper()
	if err := f.db.AutoMigrate(&models.MCPServer{}, &models.MCPTool{}, &models.MCPFileCreateGrant{}, &models.SystemSetting{}, &models.User{}, &models.FileObject{}, &models.UserStorageQuota{}); err != nil {
		t.Fatal(err)
	}
	for id := uint(1); id <= 2; id++ {
		user := models.User{BaseModel: models.BaseModel{ID: id}, Username: fmt.Sprintf("user%d", id), PublicID: fmt.Sprintf("usr%d", id), Status: "active"}
		if err := f.db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	f.server = models.MCPServer{Name: "generator", BaseURL: "https://generator.example/mcp", Status: "active", FileCreateEnabled: true}
	if err := f.db.Create(&f.server).Error; err != nil {
		t.Fatal(err)
	}
	f.tool = models.MCPTool{ServerID: f.server.ID, Name: "generate", Status: "active"}
	if err := f.db.Create(&f.tool).Error; err != nil {
		t.Fatal(err)
	}
	f.runtime = config.NewRuntime(config.Config{MCPEnable: true, MaxUploadFileBytes: 1024, UserStorageQuotaBytes: 1024, FileAllowedMIMETypes: "text/plain", MCPUserContextSecret: "shared-secret"})
	f.repo = conversationrepo.NewRepo(f.db)
	f.servers = mcprepo.NewRepo(f.db)
	f.store = storeinfra.NewLocal(f.root)
	f.uploads = appupload.NewServiceWithRuntime(f.runtime, f.repo, zap.NewNop(), appupload.Hooks{}, appconversation.UploadErrorSet(), "test")
	f.uploads.SetObjectStoreProvider(fileCreateStoreProvider{f.store})
	f.service = appmcp.NewFileCreateService(f.runtime, f.servers, f.repo, f.uploads, nil, zap.NewNop())
	return f
}

func (f *fileCreateFixture) token(t *testing.T, userID uint) string {
	t.Helper()
	server, err := f.servers.GetServer(context.Background(), f.server.ID)
	if err != nil {
		t.Fatal(err)
	}
	token, err := f.service.IssueFileCreateGrant(context.Background(), appmcp.FileCreateCall{ServerID: f.server.ID, ToolID: f.tool.ID, UserID: userID, BaseURL: f.server.BaseURL, Epoch: server.FileCreateEpoch, CallID: "00000000-0000-0000-0000-000000000001", RequestID: "request1"})
	if err != nil || token == "" {
		t.Fatalf("issue: %v", err)
	}
	return token
}

func fileCreateInput(name, text string) appupload.TemporaryFileInput {
	return appupload.TemporaryFileInput{FileName: name, MimeType: "text/plain", Reader: strings.NewReader(text), DeclaredSize: int64(len(text))}
}

func TestFileCreateOwnershipQuotaAndReplay(t *testing.T) {
	f := newFileCreateFixture(t)
	ctx := context.Background()
	token := f.token(t, 2)
	first, err := f.service.Create(ctx, token, f.server.ID, fileCreateInput("report.txt", "hello"))
	if err != nil {
		t.Fatal(err)
	}
	if first.File.UserID != 2 || first.File.Purpose != "mcp_output" || first.Replayed {
		t.Fatalf("wrong creation: %#v", first)
	}
	// Simulate response loss/restart: a new service instance replays the DB receipt.
	restarted := appmcp.NewFileCreateService(f.runtime, f.servers, conversationrepo.NewRepo(f.db), f.uploads, nil, zap.NewNop())
	second, err := restarted.Create(ctx, token, f.server.ID, fileCreateInput("report.txt", "hello"))
	if err != nil || !second.Replayed || second.File.FileID != first.File.FileID {
		t.Fatalf("replay: %#v / %v", second, err)
	}
	files, err := f.uploads.ListFiles(ctx, appupload.ListFilesInput{UserID: 2, Page: 1, PageSize: 20})
	if err != nil || files.Total != 1 || files.Quota.UsedBytes != 5 {
		t.Fatalf("list/quota: %#v / %v", files, err)
	}
	other, err := f.uploads.ListFiles(ctx, appupload.ListFilesInput{UserID: 1, Page: 1, PageSize: 20})
	if err != nil || other.Total != 0 || other.Quota.UsedBytes != 0 {
		t.Fatalf("other user: %#v / %v", other, err)
	}
	for _, input := range []appupload.TemporaryFileInput{fileCreateInput("changed.txt", "hello"), fileCreateInput("report.txt", "world")} {
		if _, err = f.service.Create(ctx, token, f.server.ID, input); !errors.Is(err, appmcp.ErrFileCreateConflict) {
			t.Fatalf("changed file accepted: %v", err)
		}
	}
	// A different request with equal content reuses the file but is not a replay.
	third, err := f.service.Create(ctx, f.token(t, 2), f.server.ID, fileCreateInput("copy.txt", "hello"))
	if err != nil || !third.Reused || third.Replayed || third.File.FileID != first.File.FileID {
		t.Fatalf("dedup: %#v / %v", third, err)
	}
	if err = f.db.Model(&models.FileObject{}).Where("file_id = ?", first.File.FileID).Update("status", "deleted").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Create(ctx, token, f.server.ID, fileCreateInput("report.txt", "hello")); !errors.Is(err, appmcp.ErrFileCreateResultGone) {
		t.Fatalf("deleted result resurrected: %v", err)
	}
}

func TestFileCreateTrustBoundary(t *testing.T) {
	for _, test := range []string{"wrong service", "forged token", "shared HMAC", "expired", "server disabled", "permission revoked", "reenabled", "user disabled", "user reenabled", "tool disabled", "tool reenabled", "global disabled", "global reenabled"} {
		t.Run(test, func(t *testing.T) {
			f := newFileCreateFixture(t)
			token := f.token(t, 1)
			id := f.server.ID
			ctx := context.Background()
			grant, err := f.service.Authorize(ctx, token, id)
			if err != nil {
				t.Fatal(err)
			}
			switch test {
			case "wrong service":
				id++
			case "forged token":
				token = "dxf1_" + strings.Repeat("x", 43)
			case "shared HMAC":
				token, err = mcpauth.Sign("shared-secret", mcpauth.Payload{UserID: 2, Audience: f.server.BaseURL, JTI: "fake", ExpiresAt: time.Now().Add(time.Hour).Unix()})
			case "expired":
				err = f.db.Model(&models.MCPFileCreateGrant{}).Where("token_hash = ?", grant.TokenHash).Update("expires_at", time.Now().Add(-time.Second)).Error
			case "server disabled":
				status := "inactive"
				_, err = f.servers.UpdateServer(ctx, id, repository.UpdateMCPServerInput{Status: &status})
			case "permission revoked":
				enabled := false
				_, err = f.servers.UpdateServer(ctx, id, repository.UpdateMCPServerInput{FileCreateEnabled: &enabled})
			case "reenabled":
				enabled := false
				_, err = f.servers.UpdateServer(ctx, id, repository.UpdateMCPServerInput{FileCreateEnabled: &enabled})
				enabled = true
				if err == nil {
					_, err = f.servers.UpdateServer(ctx, id, repository.UpdateMCPServerInput{FileCreateEnabled: &enabled})
				}
			case "user disabled":
				err = f.db.Model(&models.User{}).Where("id = ?", 1).Update("status", "suspended").Error
			case "user reenabled":
				err = userrepo.NewRepo(f.db).UpdateUserStatus(ctx, 1, "suspended")
				if err == nil {
					err = userrepo.NewRepo(f.db).UpdateUserStatus(ctx, 1, "active")
				}
			case "tool disabled":
				err = f.db.Model(&models.MCPTool{}).Where("id = ?", f.tool.ID).Update("status", "inactive").Error
			case "tool reenabled":
				_, err = f.servers.UpdateServerToolsStatus(ctx, id, []uint{f.tool.ID}, "inactive")
				if err == nil {
					status := "active"
					_, err = f.servers.UpdateTool(ctx, f.tool.ID, repository.UpdateMCPToolInput{Status: &status})
				}
			case "global disabled":
				err = f.db.Create(&models.SystemSetting{Namespace: "mcp", Key: "mcp_enable", Value: "false", ValueType: "bool"}).Error
			case "global reenabled":
				set := settingsrepo.NewRepo(f.db)
				err = set.Upsert(ctx, []domainsettings.SystemSetting{{Namespace: "mcp", Key: "mcp_enable", Value: "false", ValueType: "bool"}})
				if err == nil {
					err = set.Upsert(ctx, []domainsettings.SystemSetting{{Namespace: "mcp", Key: "mcp_enable", Value: "true", ValueType: "bool"}})
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.service.Create(ctx, token, id, fileCreateInput("report.txt", "hello")); !errors.Is(err, appmcp.ErrFileCreateUnauthorized) {
				t.Fatalf("expected authorization failure, got %v", err)
			}
			var count int64
			if err = f.db.Model(&models.FileObject{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("unauthorized file count %d: %v", count, err)
			}
		})
	}
}

func TestFileCreateIssuanceChecksConnectionSnapshotAndBoolAliases(t *testing.T) {
	f := newFileCreateFixture(t)
	ctx := context.Background()
	for _, enabled := range []string{"TRUE", "1", "true"} {
		if err := settingsrepo.NewRepo(f.db).Upsert(ctx, []domainsettings.SystemSetting{{Namespace: "mcp", Key: "mcp_enable", Value: enabled, ValueType: "bool"}}); err != nil {
			t.Fatal(err)
		}
		_ = f.token(t, 1)
	}
	headers := `{"X-Tenant":"changed"}`
	if _, err := f.servers.UpdateServer(ctx, f.server.ID, repository.UpdateMCPServerInput{HeadersJSON: &headers}); err != nil {
		t.Fatal(err)
	}
	_, err := f.service.IssueFileCreateGrant(ctx, appmcp.FileCreateCall{ServerID: f.server.ID, ToolID: f.tool.ID, UserID: 1, BaseURL: f.server.BaseURL, Epoch: 0, CallID: "new-call"})
	if !errors.Is(err, repository.ErrFileCreateUnauthorized) {
		t.Fatalf("stale route snapshot received fresh authorization: %v", err)
	}
}

type fileCreateAuditCapture struct{ records []appaudit.WriteInput }

func (a *fileCreateAuditCapture) Write(_ context.Context, input appaudit.WriteInput) {
	a.records = append(a.records, input)
}

func TestFileCreateAuditIdentifiesServiceAndUserWithoutCredentials(t *testing.T) {
	f := newFileCreateFixture(t)
	audit := &fileCreateAuditCapture{}
	svc := appmcp.NewFileCreateService(f.runtime, f.servers, f.repo, f.uploads, audit, zap.NewNop())
	token := f.token(t, 2)
	if _, err := svc.Create(context.Background(), token, f.server.ID, fileCreateInput("report.txt", "private-file-content")); err != nil {
		t.Fatal(err)
	}
	if len(audit.records) != 1 || audit.records[0].ActorUserID != 2 || audit.records[0].Action != "mcp_create_file" {
		t.Fatal("missing provenance")
	}
	raw, _ := json.Marshal(audit.records)
	if strings.Contains(string(raw), token) || strings.Contains(string(raw), "private-file-content") || !strings.Contains(string(raw), "mcp_server_id") {
		t.Fatalf("unsafe audit: %s", raw)
	}
}

type flakyFileCreateUploads struct {
	repository.UploadRepository
	fail bool
}

func (r *flakyFileCreateUploads) GetActiveFileObjectByID(ctx context.Context, userID uint, fileID string) (*domainconversation.FileObject, error) {
	if r.fail {
		return nil, errors.New("database temporarily unavailable")
	}
	return r.UploadRepository.GetActiveFileObjectByID(ctx, userID, fileID)
}

func TestFileCreateMaintenanceKeepsTransientFailuresAndResumesRealQueue(t *testing.T) {
	f := newFileCreateFixture(t)
	ctx := context.Background()
	token := f.token(t, 1)
	created, err := f.service.Create(ctx, token, f.server.ID, fileCreateInput("report.txt", "hello"))
	if err != nil {
		t.Fatal(err)
	}
	grant, err := f.service.Authorize(ctx, token, f.server.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.db.Model(&models.MCPFileCreateGrant{}).Where("token_hash = ?", grant.TokenHash).Update("expires_at", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	flaky := &flakyFileCreateUploads{UploadRepository: f.repo, fail: true}
	cache := memorycache.New()
	processing := appprocessing.NewServiceWithRuntime(appprocessing.Dependencies{Config: f.runtime, Repository: f.repo, Cache: cache, Logger: zap.NewNop()})
	uploads := appupload.NewServiceWithRuntime(f.runtime, flaky, zap.NewNop(), appupload.Hooks{EnsureFileCreateProcessing: processing.EnsureFileCreateProcessing}, appconversation.UploadErrorSet(), "test")
	uploads.SetObjectStoreProvider(fileCreateStoreProvider{f.store})
	service := appmcp.NewFileCreateService(f.runtime, f.servers, f.repo, uploads, nil, zap.NewNop())
	service.Maintain(ctx)
	var count int64
	if err = f.db.Model(&models.MCPFileCreateGrant{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("transient failure lost receipt: %d / %v", count, err)
	}
	flaky.fail = false
	service.Maintain(ctx)
	service.Maintain(ctx)
	messages, err := cache.ReadFileProcessingMessages(ctx, "test-worker")
	if err != nil || len(messages) != 1 || messages[0].FileID != created.File.FileID {
		t.Fatalf("handoff did not resume: %#v / %v", messages, err)
	}
	// A paused browser initializer carries uploaded state after an MCP worker started.
	if err = f.db.Model(&models.FileObject{}).Where("file_id = ?", created.File.FileID).Updates(map[string]any{"processing_status": "ready", "extract_status": "ready", "preview_text": "preserved"}).Error; err != nil {
		t.Fatal(err)
	}
	stale := created.File
	if err = processing.InitializeUploadedFile(ctx, &stale); err != nil {
		t.Fatal(err)
	}
	current, err := f.repo.GetActiveFileObjectByID(ctx, 1, created.File.FileID)
	if err != nil || current.ProcessingStatus != "ready" || current.PreviewText != "preserved" {
		t.Fatalf("stale browser initializer reset worker: %#v / %v", current, err)
	}
	service.Maintain(ctx)
	if err = f.db.Model(&models.MCPFileCreateGrant{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("finished receipt not expired: %d / %v", count, err)
	}
}

func TestFileCreateExpiredCleanupPreservesSharedReferencesAndRetriesRemoval(t *testing.T) {
	for _, mode := range []string{"missing metadata", "shared reference", "removal failure"} {
		t.Run(mode, func(t *testing.T) {
			f := newFileCreateFixture(t)
			ctx := context.Background()
			token := f.token(t, 1)
			result, err := f.service.Create(ctx, token, f.server.ID, fileCreateInput("report.txt", "hello"))
			if err != nil {
				t.Fatal(err)
			}
			grant, err := f.service.Authorize(ctx, token, f.server.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = f.db.Unscoped().Where("file_id = ?", result.File.FileID).Delete(&models.FileObject{}).Error; err != nil {
				t.Fatal(err)
			}
			if mode == "shared reference" {
				if err = f.db.Create(&models.FileObject{FileID: "shared", UserID: 2, StoragePath: grant.StoragePath, Status: "active"}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err = f.db.Model(&models.MCPFileCreateGrant{}).Where("token_hash = ?", grant.TokenHash).Update("expires_at", time.Unix(0, 0)).Error; err != nil {
				t.Fatal(err)
			}
			if mode == "removal failure" {
				err = f.repo.ExpireFileCreateGrant(ctx, grant.TokenHash, time.Now(), func(string) error { return errors.New("storage unavailable") })
				if err == nil {
					t.Fatal("failed removal discarded receipt")
				}
				var count int64
				if err = f.db.Model(&models.MCPFileCreateGrant{}).Count(&count).Error; err != nil || count != 1 {
					t.Fatal("lost cleanup evidence")
				}
			}
			f.service.Maintain(ctx)
			reader, _, err := f.store.Open(ctx, grant.StoragePath)
			if mode == "shared reference" {
				if err != nil {
					t.Fatal("cleanup deleted shared file")
				}
				_ = reader.Close()
			} else if !errors.Is(err, portstore.ErrNotFound) {
				t.Fatalf("candidate remains: %v", err)
			}
		})
	}
}

func TestFileCreatePolicyRejectionsLeaveNoFiles(t *testing.T) {
	f := newFileCreateFixture(t)
	ctx := context.Background()
	token := f.token(t, 1)
	if _, err := f.service.Create(ctx, token, f.server.ID, fileCreateInput("large.txt", strings.Repeat("x", 1025))); !errors.Is(err, appconversation.ErrFileTooLarge) {
		t.Fatalf("size: %v", err)
	}
	if _, err := f.service.Create(ctx, token, f.server.ID, fileCreateInput("data.pdf", "%PDF-1.7")); !errors.Is(err, appconversation.ErrMIMEBlocked) {
		t.Fatalf("type: %v", err)
	}
	cfg := f.runtime.Snapshot()
	cfg.UserStorageQuotaBytes = 2
	f.runtime.Store(cfg)
	if _, err := f.service.Create(ctx, token, f.server.ID, fileCreateInput("quota.txt", "hello")); !errors.Is(err, appconversation.ErrStorageQuotaExceeded) {
		t.Fatalf("quota: %v", err)
	}
	var count int64
	if err := f.db.Model(&models.FileObject{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("files %d: %v", count, err)
	}
	if err := filepath.WalkDir(f.root, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			t.Error("rejected upload wrote an object")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFileCreateConcurrentIndependentServices(t *testing.T) {
	f := newFileCreateFixture(t)
	otherDB := openFileCreateDB(t, f.dsn)
	checkFileCreateConcurrency(t, f, otherDB)
}

func checkFileCreateConcurrency(t *testing.T, f *fileCreateFixture, otherDB *gorm.DB) {
	t.Helper()
	token := f.token(t, 1)
	otherRepo := conversationrepo.NewRepo(otherDB)
	otherUploads := appupload.NewServiceWithRuntime(f.runtime, otherRepo, zap.NewNop(), appupload.Hooks{}, appconversation.UploadErrorSet(), "test")
	otherUploads.SetObjectStoreProvider(fileCreateStoreProvider{storeinfra.NewLocal(f.root)})
	other := appmcp.NewFileCreateService(f.runtime, mcprepo.NewRepo(otherDB), otherRepo, otherUploads, nil, zap.NewNop())
	var wg sync.WaitGroup
	results := make(chan *appmcp.FileCreateResult, 2)
	errorsCh := make(chan error, 2)
	for _, svc := range []*appmcp.FileCreateService{f.service, other} {
		wg.Add(1)
		go func(svc *appmcp.FileCreateService) {
			defer wg.Done()
			result, err := svc.Create(context.Background(), token, f.server.ID, fileCreateInput("report.txt", "hello"))
			results <- result
			errorsCh <- err
		}(svc)
	}
	wg.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	id := ""
	for result := range results {
		if id != "" && result.File.FileID != id {
			t.Fatal("multiple files created")
		}
		id = result.File.FileID
	}
	quota, err := f.repo.GetOrInitUserStorageQuota(context.Background(), 1, 1024)
	if err != nil || quota.UsedBytes != 5 {
		t.Fatalf("quota: %#v / %v", quota, err)
	}
}

func TestFileCreatePostgresConcurrentInstances(t *testing.T) {
	dsn := os.Getenv("DEEIX_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set DEEIX_TEST_DATABASE_DSN to test PostgreSQL multi-instance receipt/quota settlement")
	}
	adminDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	schemaName := fmt.Sprintf("deeix_file_create_%d", time.Now().UnixNano())
	if err = adminDB.Exec("CREATE SCHEMA " + schemaName).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = adminDB.Exec("DROP SCHEMA " + schemaName + " CASCADE").Error
		db, _ := adminDB.DB()
		_ = db.Close()
	})
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, parseErr := url.Parse(dsn)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		query := parsed.Query()
		query.Set("search_path", schemaName)
		parsed.RawQuery = query.Encode()
		dsn = parsed.String()
	} else {
		dsn += " search_path=" + schemaName
	}
	open := func() *gorm.DB {
		db, openErr := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if openErr != nil {
			t.Fatal(openErr)
		}
		sqlDB, dbErr := db.DB()
		if dbErr != nil {
			t.Fatal(dbErr)
		}
		sqlDB.SetMaxOpenConns(8)
		t.Cleanup(func() { _ = sqlDB.Close() })
		return db
	}
	f := initializeFileCreateFixture(t, &fileCreateFixture{db: open(), root: t.TempDir()})
	if err = f.db.Exec("CREATE UNIQUE INDEX file_create_test_content ON file_objects(user_id, sha256, size_bytes) WHERE status = 'active' AND deleted_at IS NULL AND sha256 <> ''").Error; err != nil {
		t.Fatal(err)
	}
	checkFileCreateConcurrency(t, f, open())
}

type failingFileCreateRepository struct {
	repository.MCPFileCreateRepository
	rollback     bool
	loseResponse bool
}

func (r failingFileCreateRepository) WithFileCreateGrant(ctx context.Context, hash string, serverID uint, fn func(*domainmcp.FileCreateGrant, repository.UploadRepository) error) error {
	err := r.MCPFileCreateRepository.WithFileCreateGrant(ctx, hash, serverID, func(g *domainmcp.FileCreateGrant, uploads repository.UploadRepository) error {
		if err := fn(g, uploads); err != nil {
			return err
		}
		if r.rollback {
			return errors.New("injected rollback")
		}
		return nil
	})
	if err == nil && r.loseResponse {
		return errors.New("commit acknowledgment lost")
	}
	return err
}

type acceptedButFailedStore struct{ portstore.Store }

func (s acceptedButFailedStore) Put(ctx context.Context, key string, body io.Reader, opts portstore.PutOptions) (portstore.ObjectInfo, error) {
	info, err := s.Store.Put(ctx, key, body, opts)
	if err == nil {
		err = errors.New("put acknowledgment lost")
	}
	return info, err
}

func TestFileCreateFailureRecoveryAndCleanup(t *testing.T) {
	for _, mode := range []string{"rollback", "lost commit response", "lost put response", "expired abandoned"} {
		t.Run(mode, func(t *testing.T) {
			f := newFileCreateFixture(t)
			token := f.token(t, 1)
			ctx := context.Background()
			failedRepo := failingFileCreateRepository{MCPFileCreateRepository: f.repo, rollback: mode == "rollback" || mode == "expired abandoned", loseResponse: mode == "lost commit response"}
			if mode == "lost put response" {
				f.uploads.SetObjectStoreProvider(fileCreateStoreProvider{acceptedButFailedStore{f.store}})
			}
			failed := appmcp.NewFileCreateService(f.runtime, f.servers, failedRepo, f.uploads, nil, zap.NewNop())
			if _, err := failed.Create(ctx, token, f.server.ID, fileCreateInput("report.txt", "hello")); err == nil {
				t.Fatal("expected injected failure")
			}
			grant, err := f.service.Authorize(ctx, token, f.server.ID)
			if err != nil {
				t.Fatal(err)
			}
			reader, _, err := f.store.Open(ctx, grant.StoragePath)
			if err != nil {
				t.Fatalf("uncertain completion destroyed candidate: %v", err)
			}
			_ = reader.Close()
			if mode == "expired abandoned" {
				if err = f.db.Model(&models.MCPFileCreateGrant{}).Where("token_hash = ?", grant.TokenHash).Update("expires_at", time.Now().Add(-time.Second)).Error; err != nil {
					t.Fatal(err)
				}
				f.service.Maintain(ctx)
				if _, _, err = f.store.Open(ctx, grant.StoragePath); !errors.Is(err, portstore.ErrNotFound) {
					t.Fatalf("abandoned object remains: %v", err)
				}
				return
			}
			f.uploads.SetObjectStoreProvider(fileCreateStoreProvider{f.store})
			result, err := f.service.Create(ctx, token, f.server.ID, fileCreateInput("report.txt", "hello"))
			if err != nil || result.File.FileID != grant.CandidateID {
				t.Fatalf("recovery: %#v / %v", result, err)
			}
			quota, err := f.repo.GetOrInitUserStorageQuota(ctx, 1, 1024)
			if err != nil || quota.UsedBytes != 5 {
				t.Fatalf("recovery quota: %#v / %v", quota, err)
			}
		})
	}
}

func TestFileCreateHTTPRejectsForeignOwnershipAndExtraParts(t *testing.T) {
	f := newFileCreateFixture(t)
	token := f.token(t, 1)
	h := NewHandler(nil)
	h.SetFileCreateService(f.service)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewModule(h).RegisterFileCreateRoute(router.Group("/api/v1"), middleware.RateLimit(nil, f.runtime))
	for _, test := range []string{"missing token", "no bearer", "wrong user field", "extra file", "oversized", "normal"} {
		t.Run(test, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("file", "report.txt")
			if err != nil {
				t.Fatal(err)
			}
			text := "hello"
			if test == "oversized" {
				text = strings.Repeat("x", 1025)
			}
			_, _ = io.WriteString(part, text)
			if test == "wrong user field" {
				_ = writer.WriteField("user_id", "2")
			}
			if test == "extra file" {
				extra, _ := writer.CreateFormFile("file", "second.txt")
				_, _ = io.WriteString(extra, "extra")
			}
			_ = writer.Close()
			request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/mcp/servers/%d/files", f.server.ID), &body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			if test != "missing token" {
				request.Header.Set("Authorization", "Bearer "+token)
			}
			if test == "no bearer" {
				request.Header.Set("Authorization", token)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			want := 400
			switch test {
			case "missing token", "no bearer":
				want = 401
			case "oversized":
				want = 413
			case "normal":
				want = 200
			}
			if recorder.Code != want {
				t.Fatalf("status %d, expected %d: %s", recorder.Code, want, recorder.Body.String())
			}
			if want == 200 {
				var response FileCreateResponseDoc
				if err = json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Data.FileID == "" || response.Data.ProcessingStatus != "uploaded" || response.Data.ExtractStatus != "none" {
					t.Fatalf("creation/state: %#v / %v", response, err)
				}
			}
		})
	}
}
