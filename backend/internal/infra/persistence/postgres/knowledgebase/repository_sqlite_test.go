package knowledgebase

import (
	"context"
	"errors"
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRepositoryVisibilityResolutionAndDeletion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:knowledge_base_repository?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err = db.AutoMigrate(
		&model.KnowledgeBase{},
		&model.KnowledgeBaseFile{},
		&model.ConversationProjectKnowledgeBase{},
		&model.FileObject{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	items := []model.KnowledgeBase{
		{PublicID: "builtin-enabled", Scope: domainknowledgebase.ScopeBuiltin, Name: "Built in", Enabled: true, SortOrder: 1},
		{PublicID: "builtin-disabled", Scope: domainknowledgebase.ScopeBuiltin, Name: "Disabled", Enabled: false, SortOrder: 2},
		{PublicID: "mine", Scope: domainknowledgebase.ScopeUser, OwnerUserID: 11, Name: "Mine", Enabled: true, SortOrder: 1},
		{PublicID: "other", Scope: domainknowledgebase.ScopeUser, OwnerUserID: 22, Name: "Other", Enabled: true, SortOrder: 1},
	}
	if err = db.Create(&items).Error; err != nil {
		t.Fatalf("seed knowledge bases: %v", err)
	}
	if err = db.Model(&items[1]).Update("enabled", false).Error; err != nil {
		t.Fatalf("disable knowledge base fixture: %v", err)
	}
	files := []model.FileObject{
		{FileID: "builtin-file", UserID: 0, FileName: "policy.md", Status: "active", ProcessingReady: true, EmbedStatus: "ready", ChunkCount: 2},
		{FileID: "builtin-available", UserID: 0, FileName: "platform-guide.md", Status: "active"},
		{FileID: "mine-file", UserID: 11, FileName: "notes.md", Status: "active", ProcessingReady: true, EmbedStatus: "ready", ChunkCount: 2},
		{FileID: "other-file", UserID: 22, FileName: "private.md", Status: "active", ProcessingReady: true, EmbedStatus: "ready", ChunkCount: 2},
		{
			FileID: "mine-pending", UserID: 11, FileName: "pending.md", MimeType: "text/markdown",
			FileCategory: "text", StoragePath: "uploads/pending.md", Status: "active",
			ProcessingReady: true, ExtractStatus: "ready", EmbedStatus: "processing", EmbedSignature: "model@1536",
		},
		{FileID: "mine-available", UserID: 11, FileName: "available.md", Status: "active"},
	}
	if err = db.Create(&files).Error; err != nil {
		t.Fatalf("seed files: %v", err)
	}
	links := []model.KnowledgeBaseFile{
		{KnowledgeBaseID: items[0].ID, FileObjectID: files[0].ID, AddedByUserID: 99},
		{KnowledgeBaseID: items[2].ID, FileObjectID: files[2].ID, AddedByUserID: 11},
		{KnowledgeBaseID: items[3].ID, FileObjectID: files[3].ID, AddedByUserID: 22},
		{KnowledgeBaseID: items[2].ID, FileObjectID: files[4].ID, AddedByUserID: 11},
	}
	if err = db.Create(&links).Error; err != nil {
		t.Fatalf("seed links: %v", err)
	}
	if err = db.Create(&model.ConversationProjectKnowledgeBase{ProjectID: 101, KnowledgeBaseID: items[0].ID}).Error; err != nil {
		t.Fatalf("seed project knowledge base link: %v", err)
	}

	repo := NewRepo(db)
	processingStatuses, err := repo.GetKnowledgeBaseFileProcessingStatuses(
		context.Background(), items[2].ID, []string{"mine-pending"},
	)
	if err != nil || len(processingStatuses) != 1 {
		t.Fatalf("GetKnowledgeBaseFileProcessingStatuses() = %#v, %v", processingStatuses, err)
	}
	processingStatus := processingStatuses[0]
	if processingStatus.FileName != "pending.md" || processingStatus.MimeType != "text/markdown" ||
		processingStatus.FileCategory != "text" || processingStatus.StoragePath != "uploads/pending.md" ||
		processingStatus.Status != "active" || !processingStatus.ProcessingReady || processingStatus.ExtractStatus != "ready" ||
		processingStatus.EmbedStatus != "processing" || processingStatus.EmbedSignature != "model@1536" {
		t.Fatalf("incomplete knowledge base processing projection: %#v", processingStatus)
	}
	listedFiles, listedTotal, err := repo.ListKnowledgeBaseFiles(context.Background(), items[2].ID, 0, 100)
	if err != nil || listedTotal != 2 {
		t.Fatalf("ListKnowledgeBaseFiles() = %#v, total=%d, err=%v", listedFiles, listedTotal, err)
	}
	var listedPending *domainconversation.FileObject
	for i := range listedFiles {
		if listedFiles[i].FileID == "mine-pending" {
			listedPending = &listedFiles[i]
			break
		}
	}
	if listedPending == nil || listedPending.StoragePath != "uploads/pending.md" || listedPending.EmbedSignature != "model@1536" {
		t.Fatalf("incomplete knowledge base list projection: %#v", listedPending)
	}
	createdDisabled, err := repo.CreateKnowledgeBase(context.Background(), &domainknowledgebase.KnowledgeBase{
		PublicID: "created-disabled", Scope: domainknowledgebase.ScopeBuiltin, Name: "Created disabled", Enabled: false,
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase(disabled) error = %v", err)
	}
	if createdDisabled.Enabled {
		t.Fatal("CreateKnowledgeBase(disabled) persisted enabled=true")
	}
	if _, err = repo.CreateKnowledgeBase(context.Background(), &domainknowledgebase.KnowledgeBase{
		PublicID: "duplicate-mine", Scope: domainknowledgebase.ScopeUser, OwnerUserID: 11, Name: "Mine", Enabled: true,
	}); !errors.Is(err, repository.ErrDuplicate) {
		t.Fatalf("CreateKnowledgeBase(duplicate name) error = %v, want ErrDuplicate", err)
	}
	userID := uint(11)
	visible, total, err := repo.ListKnowledgeBases(context.Background(), repository.KnowledgeBaseListFilter{VisibleUserID: &userID}, 0, 100)
	if err != nil {
		t.Fatalf("ListKnowledgeBases() error = %v", err)
	}
	if total != 2 || len(visible) != 2 || visible[0].PublicID != "builtin-enabled" || visible[1].PublicID != "mine" {
		t.Fatalf("visible = %+v total=%d, want enabled built-in and own personal", visible, total)
	}
	if visible[0].ReadyFileCount != 1 || visible[1].ReadyFileCount != 1 {
		t.Fatalf("ready counts = %d/%d, want 1/1", visible[0].ReadyFileCount, visible[1].ReadyFileCount)
	}
	linkedFile, err := repo.GetKnowledgeBaseFile(context.Background(), items[0].ID, "builtin-file")
	if err != nil || linkedFile.FileID != "builtin-file" || linkedFile.UserID != 0 {
		t.Fatalf("GetKnowledgeBaseFile() = %#v, %v, want linked built-in file", linkedFile, err)
	}
	if _, err = repo.GetKnowledgeBaseFile(context.Background(), items[0].ID, "other-file"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("GetKnowledgeBaseFile(unlinked) error = %v, want ErrNotFound", err)
	}
	available, availableTotal, err := repo.ListAvailableKnowledgeBaseFiles(context.Background(), items[2].ID, userID, "available", 0, 50)
	if err != nil || availableTotal != 1 || len(available) != 1 || available[0].FileID != "mine-available" {
		t.Fatalf("ListAvailableKnowledgeBaseFiles() = %#v total=%d error=%v, want mine-available", available, availableTotal, err)
	}
	platformAvailable, platformAvailableTotal, err := repo.ListAvailableKnowledgeBaseFiles(context.Background(), items[0].ID, 0, "platform", 0, 50)
	if err != nil || platformAvailableTotal != 1 || len(platformAvailable) != 1 || platformAvailable[0].FileID != "builtin-available" || platformAvailable[0].UserID != 0 {
		t.Fatalf("ListAvailableKnowledgeBaseFiles(platform) = %#v total=%d error=%v, want builtin-available", platformAvailable, platformAvailableTotal, err)
	}
	platformFiles, platformFilesTotal, err := repo.ListKnowledgeBaseSourceFiles(context.Background(), 0, "", 0, 50)
	if err != nil || platformFilesTotal != 2 || len(platformFiles) != 2 {
		t.Fatalf("ListKnowledgeBaseSourceFiles(platform) = %#v total=%d error=%v, want both platform files", platformFiles, platformFilesTotal, err)
	}

	resolvedBases, resolvedFiles, err := repo.ResolveVisibleKnowledgeBaseFiles(
		context.Background(), userID, []string{"builtin-enabled", "mine"},
	)
	if err != nil {
		t.Fatalf("ResolveVisibleKnowledgeBaseFiles() error = %v", err)
	}
	if len(resolvedBases) != 2 || len(resolvedFiles) != 2 {
		t.Fatalf("resolved bases/files = %d/%d, want 2/2", len(resolvedBases), len(resolvedFiles))
	}
	for _, file := range resolvedFiles {
		if file.FileID == "mine-pending" {
			t.Fatal("ResolveVisibleKnowledgeBaseFiles() returned a file that is not ready for retrieval")
		}
	}
	if _, _, err = repo.ResolveVisibleKnowledgeBaseFiles(context.Background(), userID, []string{"other"}); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("resolve other user's base error = %v, want ErrNotFound", err)
	}
	if err = repo.AddKnowledgeBaseFiles(context.Background(), items[2].ID, domainknowledgebase.ScopeUser, userID, userID, []string{"other-file"}); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("add other user's file error = %v, want ErrNotFound", err)
	}
	var revisionBefore uint64
	if err = db.Model(&model.KnowledgeBase{}).Where("id = ?", items[2].ID).Pluck("revision", &revisionBefore).Error; err != nil {
		t.Fatalf("load revision before duplicate add: %v", err)
	}
	if err = repo.AddKnowledgeBaseFiles(context.Background(), items[2].ID, domainknowledgebase.ScopeUser, userID, userID, []string{"mine-file"}); err != nil {
		t.Fatalf("add already linked file: %v", err)
	}
	var revisionAfter uint64
	if err = db.Model(&model.KnowledgeBase{}).Where("id = ?", items[2].ID).Pluck("revision", &revisionAfter).Error; err != nil {
		t.Fatalf("load revision after duplicate add: %v", err)
	}
	if revisionAfter != revisionBefore {
		t.Fatalf("duplicate add changed revision from %d to %d", revisionBefore, revisionAfter)
	}
	orderedFiles := []model.FileObject{
		{FileID: "ordered-second", UserID: userID, FileName: "second.md", Status: "active"},
		{FileID: "ordered-first", UserID: userID, FileName: "first.md", Status: "active"},
	}
	if err = db.Create(&orderedFiles).Error; err != nil {
		t.Fatalf("seed ordered files: %v", err)
	}
	if err = repo.AddKnowledgeBaseFiles(
		context.Background(),
		items[2].ID,
		domainknowledgebase.ScopeUser,
		userID,
		userID,
		[]string{"ordered-first", "ordered-second"},
	); err != nil {
		t.Fatalf("add ordered files: %v", err)
	}
	var orderedLinks []model.KnowledgeBaseFile
	if err = db.Where("knowledge_base_id = ? AND file_object_id IN ?", items[2].ID, []uint{orderedFiles[0].ID, orderedFiles[1].ID}).
		Order("sort_order ASC").Find(&orderedLinks).Error; err != nil {
		t.Fatalf("load ordered links: %v", err)
	}
	if len(orderedLinks) != 2 || orderedLinks[0].FileObjectID != orderedFiles[1].ID || orderedLinks[1].FileObjectID != orderedFiles[0].ID {
		t.Fatalf("ordered links = %#v, want request order", orderedLinks)
	}

	disabled := false
	if _, err = repo.PatchKnowledgeBase(context.Background(), items[0].ID, repository.KnowledgeBasePatch{Enabled: &disabled}); err != nil {
		t.Fatalf("disable built-in knowledge base: %v", err)
	}
	var projectLinkCount int64
	if err = db.Model(&model.ConversationProjectKnowledgeBase{}).Where("knowledge_base_id = ?", items[0].ID).Count(&projectLinkCount).Error; err != nil {
		t.Fatalf("count project links after disable: %v", err)
	}
	if projectLinkCount != 1 {
		t.Fatalf("project links after disable = %d, want 1", projectLinkCount)
	}

	cleanupCandidates, err := repo.DeleteKnowledgeBase(context.Background(), items[2].ID)
	if err != nil {
		t.Fatalf("DeleteKnowledgeBase() error = %v", err)
	}
	wantCleanupCandidates := map[string]uint{
		files[2].FileID:        files[2].UserID,
		files[4].FileID:        files[4].UserID,
		orderedFiles[0].FileID: orderedFiles[0].UserID,
		orderedFiles[1].FileID: orderedFiles[1].UserID,
	}
	if len(cleanupCandidates) != len(wantCleanupCandidates) {
		t.Fatalf("DeleteKnowledgeBase() cleanup candidate count = %d, want %d", len(cleanupCandidates), len(wantCleanupCandidates))
	}
	for _, candidate := range cleanupCandidates {
		if wantUserID, ok := wantCleanupCandidates[candidate.FileID]; !ok || candidate.UserID != wantUserID {
			t.Fatalf("DeleteKnowledgeBase() unexpected cleanup candidate = %#v", candidate)
		}
	}
	var fileCount int64
	if err = db.Model(&model.FileObject{}).Where("id = ?", files[1].ID).Count(&fileCount).Error; err != nil {
		t.Fatalf("count retained file: %v", err)
	}
	if fileCount != 1 {
		t.Fatalf("retained file count = %d, want 1", fileCount)
	}
}
