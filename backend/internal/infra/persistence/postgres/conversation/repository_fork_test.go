package conversation

import (
	"context"
	"errors"
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestCreateForkedConversationCommitsCompleteFork(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	source := model.Conversation{
		UserID: 1, PublicID: "conv_source", Title: "Source", LabelsJSON: "[]",
		SessionKey: "session_source", MessageCount: 2, Status: "active",
	}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source conversation: %v", err)
	}
	root := model.Message{
		ConversationID: source.ID, UserID: 1, PublicID: "msg_source_root",
		Role: "user", ContentType: "text", Content: "root", BranchReason: "default", Status: "success",
	}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create source root: %v", err)
	}
	leaf := model.Message{
		ConversationID: source.ID, UserID: 1, PublicID: "msg_source_leaf", ParentMessageID: &root.ID,
		Role: "assistant", ContentType: "text", Content: "leaf", BranchReason: "default", Status: "success",
	}
	if err := db.Create(&leaf).Error; err != nil {
		t.Fatalf("create source leaf: %v", err)
	}
	files := []model.FileObject{
		{FileID: "file_active", UserID: 1, FileName: "current.png", MimeType: "image/png", SizeBytes: 20, SHA256: "active-sha", StoragePath: "objects/active", Status: "active"},
		{FileID: "file_deleted", UserID: 1, FileName: "deleted.png", MimeType: "image/png", SizeBytes: 30, SHA256: "deleted-sha", StoragePath: "objects/deleted", Status: "deleted"},
	}
	if err := db.Create(&files).Error; err != nil {
		t.Fatalf("create file objects: %v", err)
	}
	sourceAttachments := []model.Attachment{
		{
			ConversationID: source.ID, MessageID: root.ID, UserID: 1, FileID: files[0].FileID,
			Kind: "image", FileName: "original.png", MimeType: "image/png", FileSize: 10,
			SHA256: "source-sha", StoragePath: "old/path", Status: "active", MetaJSON: `{"width":100}`,
		},
		{
			ConversationID: source.ID, MessageID: leaf.ID, UserID: 1, FileID: files[1].FileID,
			Kind: "image", FileName: "deleted.png", MimeType: "image/png", FileSize: 30,
			SHA256: "deleted-sha", StoragePath: "objects/deleted", Status: "active",
		},
	}
	if err := db.Create(&sourceAttachments).Error; err != nil {
		t.Fatalf("create source attachments: %v", err)
	}

	target := &domainconversation.Conversation{
		UserID: 1, PublicID: "conv_fork", Title: "Source", LabelsJSON: "[]",
		SessionKey: "session_fork", MessageCount: 2, Status: "active",
	}
	rootMessage := domainconversation.Message{
		UserID: 1, PublicID: "msg_fork_root", Role: "user", ContentType: "text",
		Content: "root", BranchReason: "default", Status: "success",
	}
	leafMessage := domainconversation.Message{
		UserID: 1, PublicID: "msg_fork_leaf", Role: "assistant", ContentType: "text",
		Content: "leaf", BranchReason: "default", Status: "success",
	}
	if err := repo.CreateForkedConversation(ctx, repository.CreateForkedConversationInput{
		SourceConversationID: source.ID,
		Conversation:         target,
		Messages: []repository.ForkConversationMessage{
			{SourceMessageID: root.ID, Message: rootMessage},
			{SourceMessageID: leaf.ID, SourceParentMessageID: &root.ID, Message: leafMessage},
		},
	}); err != nil {
		t.Fatalf("CreateForkedConversation() error = %v", err)
	}
	if target.ID == 0 || target.MessageCount != 2 {
		t.Fatalf("target conversation = %+v, want persisted two-message fork", target)
	}

	var targetMessages []model.Message
	if err := db.Where("conversation_id = ?", target.ID).Order("id ASC").Find(&targetMessages).Error; err != nil {
		t.Fatalf("load target messages: %v", err)
	}
	if len(targetMessages) != 2 {
		t.Fatalf("len(targetMessages) = %d, want 2", len(targetMessages))
	}
	if targetMessages[0].ParentMessageID != nil {
		t.Fatal("fork root unexpectedly has a parent")
	}
	if parent := targetMessages[1].ParentMessageID; parent == nil || *parent != targetMessages[0].ID {
		t.Fatalf("fork leaf parent = %v, want %d", parent, targetMessages[0].ID)
	}

	var targetAttachments []model.Attachment
	if err := db.Where("conversation_id = ?", target.ID).Find(&targetAttachments).Error; err != nil {
		t.Fatalf("load target attachments: %v", err)
	}
	if len(targetAttachments) != 1 {
		t.Fatalf("len(targetAttachments) = %d, want 1 active file reference", len(targetAttachments))
	}
	attachment := targetAttachments[0]
	if attachment.MessageID != targetMessages[0].ID || attachment.FileID != "file_active" {
		t.Fatalf("target attachment owner = (%d, %q), want (%d, %q)", attachment.MessageID, attachment.FileID, targetMessages[0].ID, "file_active")
	}
	if attachment.SHA256 != "active-sha" || attachment.StoragePath != "objects/active" {
		t.Fatalf("target attachment did not use current file metadata: %+v", attachment)
	}
	if attachment.MetaJSON != sourceAttachments[0].MetaJSON || attachment.FileName != sourceAttachments[0].FileName {
		t.Fatalf("target attachment did not preserve message metadata: %+v", attachment)
	}

	var sourceMessageCount int64
	if err := db.Model(&model.Message{}).Where("conversation_id = ?", source.ID).Count(&sourceMessageCount).Error; err != nil {
		t.Fatalf("count source messages: %v", err)
	}
	if sourceMessageCount != 2 {
		t.Fatalf("source message count = %d, want 2", sourceMessageCount)
	}
}

func TestCreateForkedConversationRollsBackPartialWrites(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	source := model.Conversation{
		UserID: 1, PublicID: "conv_source_rollback", LabelsJSON: "[]",
		SessionKey: "session_source_rollback", MessageCount: 2, Status: "active",
	}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source conversation: %v", err)
	}
	root := model.Message{
		ConversationID: source.ID, UserID: 1, PublicID: "msg_rollback_root",
		Role: "user", ContentType: "text", BranchReason: "default", Status: "success",
	}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create source root: %v", err)
	}
	leaf := model.Message{
		ConversationID: source.ID, UserID: 1, PublicID: "msg_rollback_leaf", ParentMessageID: &root.ID,
		Role: "assistant", ContentType: "text", BranchReason: "default", Status: "success",
	}
	if err := db.Create(&leaf).Error; err != nil {
		t.Fatalf("create source leaf: %v", err)
	}

	target := &domainconversation.Conversation{
		UserID: 1, PublicID: "conv_fork_rollback", LabelsJSON: "[]",
		SessionKey: "session_fork_rollback", MessageCount: 2, Status: "active",
	}
	duplicatePublicID := "msg_fork_duplicate"
	err := repo.CreateForkedConversation(ctx, repository.CreateForkedConversationInput{
		SourceConversationID: source.ID,
		Conversation:         target,
		Messages: []repository.ForkConversationMessage{
			{SourceMessageID: root.ID, Message: domainconversation.Message{PublicID: duplicatePublicID, Role: "user", ContentType: "text", BranchReason: "default", Status: "success"}},
			{SourceMessageID: leaf.ID, SourceParentMessageID: &root.ID, Message: domainconversation.Message{PublicID: duplicatePublicID, Role: "assistant", ContentType: "text", BranchReason: "default", Status: "success"}},
		},
	})
	if err == nil {
		t.Fatal("CreateForkedConversation() error = nil, want unique constraint failure")
	}
	if target.ID != 0 {
		t.Fatalf("target ID = %d after rollback, want 0", target.ID)
	}

	var conversationCount int64
	if err := db.Model(&model.Conversation{}).Where("public_id = ?", target.PublicID).Count(&conversationCount).Error; err != nil {
		t.Fatalf("count rolled back conversation: %v", err)
	}
	if conversationCount != 0 {
		t.Fatalf("rolled back conversation count = %d, want 0", conversationCount)
	}
	var messageCount int64
	if err := db.Model(&model.Message{}).Where("public_id = ?", duplicatePublicID).Count(&messageCount).Error; err != nil {
		t.Fatalf("count rolled back messages: %v", err)
	}
	if messageCount != 0 {
		t.Fatalf("rolled back message count = %d, want 0", messageCount)
	}
}

func TestCreateForkedConversationRejectsMessageOutsideSourceConversation(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	conversations := []model.Conversation{
		{UserID: 1, PublicID: "conv_source_scope", SessionKey: "session_source_scope", Status: "active"},
		{UserID: 1, PublicID: "conv_other_scope", SessionKey: "session_other_scope", Status: "active"},
	}
	if err := db.Create(&conversations).Error; err != nil {
		t.Fatalf("create source conversations: %v", err)
	}
	foreignMessage := model.Message{
		ConversationID: conversations[1].ID, UserID: 1, PublicID: "msg_other_scope",
		Role: "user", ContentType: "text", BranchReason: "default", Status: "success",
	}
	if err := db.Create(&foreignMessage).Error; err != nil {
		t.Fatalf("create foreign message: %v", err)
	}

	target := &domainconversation.Conversation{
		UserID: 1, PublicID: "conv_fork_scope", SessionKey: "session_fork_scope", Status: "active",
	}
	err := repo.CreateForkedConversation(ctx, repository.CreateForkedConversationInput{
		SourceConversationID: conversations[0].ID,
		Conversation:         target,
		Messages: []repository.ForkConversationMessage{{
			SourceMessageID: foreignMessage.ID,
			Message: domainconversation.Message{
				PublicID: "msg_fork_scope", Role: "user", ContentType: "text", BranchReason: "default", Status: "success",
			},
		}},
	})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("CreateForkedConversation() error = %v, want repository.ErrNotFound", err)
	}

	var targetCount int64
	if err := db.Model(&model.Conversation{}).Where("public_id = ?", target.PublicID).Count(&targetCount).Error; err != nil {
		t.Fatalf("count rejected target: %v", err)
	}
	if targetCount != 0 {
		t.Fatalf("rejected target count = %d, want 0", targetCount)
	}
}
