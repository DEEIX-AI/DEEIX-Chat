package conversation

import (
	"context"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

func createMessageDeleteFixture(t *testing.T) (*gorm.DB, *Repo, []model.Message) {
	t.Helper()
	db := openConversationRepositoryTestDB(t)
	if err := db.AutoMigrate(&model.ChatContextRecord{}); err != nil {
		t.Fatalf("migrate chat context records: %v", err)
	}
	repo := NewRepo(db)

	conversation := model.Conversation{
		UserID: 1, PublicID: "conv_delete", Title: "Delete", LabelsJSON: "[]",
		SessionKey: "session_delete", MessageCount: 6, Status: "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	// U1 → A1 → U2 → A2 → U3 → A3
	user1 := model.Message{ConversationID: conversation.ID, UserID: 1, PublicID: "msg_u1", Role: "user", ContentType: "text", Content: "u1", BranchReason: "default", Status: "success"}
	assistant1 := model.Message{ConversationID: conversation.ID, UserID: 1, PublicID: "msg_a1", Role: "assistant", ContentType: "text", Content: "a1", BranchReason: "default", Status: "success"}
	user2 := model.Message{ConversationID: conversation.ID, UserID: 1, PublicID: "msg_u2", Role: "user", ContentType: "text", Content: "u2", BranchReason: "default", Status: "success"}
	assistant2 := model.Message{ConversationID: conversation.ID, UserID: 1, PublicID: "msg_a2", Role: "assistant", ContentType: "text", Content: "a2", BranchReason: "default", Status: "success"}
	user3 := model.Message{ConversationID: conversation.ID, UserID: 1, PublicID: "msg_u3", Role: "user", ContentType: "text", Content: "u3", BranchReason: "default", Status: "success"}
	assistant3 := model.Message{ConversationID: conversation.ID, UserID: 1, PublicID: "msg_a3", Role: "assistant", ContentType: "text", Content: "a3", BranchReason: "default", Status: "success"}
	messages := []model.Message{user1, assistant1, user2, assistant2, user3, assistant3}
	if err := db.Create(&messages).Error; err != nil {
		t.Fatalf("create messages: %v", err)
	}
	linkParent(t, db, &messages[1], &messages[0])
	linkParent(t, db, &messages[2], &messages[1])
	linkParent(t, db, &messages[3], &messages[2])
	linkParent(t, db, &messages[4], &messages[3])
	linkParent(t, db, &messages[5], &messages[4])
	return db, repo, messages
}

func linkParent(t *testing.T, db *gorm.DB, child *model.Message, parent *model.Message) {
	t.Helper()
	if err := db.Model(&model.Message{}).Where("id = ?", child.ID).Update("parent_message_id", parent.ID).Error; err != nil {
		t.Fatalf("link parent: %v", err)
	}
	child.ParentMessageID = &parent.ID
}

func TestDeleteMessageAndReparentChildrenSplicesMiddleMessage(t *testing.T) {
	db, repo, messages := createMessageDeleteFixture(t)
	ctx := context.Background()

	reparented, err := repo.DeleteMessageAndReparentChildren(ctx, 1, messages[0].ConversationID, messages[2].ID)
	if err != nil {
		t.Fatalf("delete message: %v", err)
	}
	if reparented != 1 {
		t.Fatalf("reparented = %d, want 1", reparented)
	}

	var user2 model.Message
	if err := db.Unscoped().Where("id = ?", messages[2].ID).First(&user2).Error; err != nil {
		t.Fatalf("reload deleted message: %v", err)
	}
	if !user2.DeletedAt.Valid {
		t.Fatalf("expected soft delete for message %d", messages[2].ID)
	}

	var assistant2 model.Message
	if err := db.Where("id = ?", messages[3].ID).First(&assistant2).Error; err != nil {
		t.Fatalf("reload reparented message: %v", err)
	}
	if assistant2.ParentMessageID == nil || *assistant2.ParentMessageID != messages[1].ID {
		t.Fatalf("assistant2 parent = %v, want %d", assistant2.ParentMessageID, messages[1].ID)
	}

	var assistant3 model.Message
	if err := db.Where("id = ?", messages[5].ID).First(&assistant3).Error; err != nil {
		t.Fatalf("reload descendant message: %v", err)
	}
	if assistant3.ParentMessageID == nil || *assistant3.ParentMessageID != messages[4].ID {
		t.Fatalf("assistant3 parent = %v, want %d", assistant3.ParentMessageID, messages[4].ID)
	}

	var conversation model.Conversation
	if err := db.Where("id = ?", messages[0].ConversationID).First(&conversation).Error; err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if conversation.MessageCount != 5 {
		t.Fatalf("message count = %d, want 5", conversation.MessageCount)
	}

	active, err := repo.ListAllMessages(ctx, messages[0].ConversationID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(active) != 5 {
		t.Fatalf("active messages = %d, want 5", len(active))
	}
}

func TestDeleteMessageAndReparentChildrenRejectsPendingAndRoot(t *testing.T) {
	db, repo, messages := createMessageDeleteFixture(t)
	ctx := context.Background()

	if err := db.Model(&model.Message{}).Where("id = ?", messages[3].ID).Update("status", "pending").Error; err != nil {
		t.Fatalf("mark pending: %v", err)
	}
	if _, err := repo.DeleteMessageAndReparentChildren(ctx, 1, messages[0].ConversationID, messages[3].ID); err != repository.ErrMessageDeleteStateInvalid {
		t.Fatalf("pending delete err = %v, want ErrMessageDeleteStateInvalid", err)
	}

	if _, err := repo.DeleteMessageAndReparentChildren(ctx, 1, messages[0].ConversationID, messages[0].ID); err != repository.ErrMessageDeleteRootInvalid {
		t.Fatalf("root delete err = %v, want ErrMessageDeleteRootInvalid", err)
	}

	if _, err := repo.DeleteMessageAndReparentChildren(ctx, 2, messages[0].ConversationID, messages[2].ID); err == nil {
		t.Fatalf("expected error for other user's message")
	}
}

func TestDeleteMessageAndReparentChildrenInvalidatesSubtreeSnapshots(t *testing.T) {
	db, repo, messages := createMessageDeleteFixture(t)
	ctx := context.Background()

	records := []model.ChatContextRecord{
		{RecordType: chatContextRecordSnapshot, ConversationID: messages[0].ConversationID, UserID: 1, RunID: "run_subtree", CoveredUntilMessageID: messages[4].ID, CoveredUntilPublicID: "msg_u3", CoveragePathHash: "hash_subtree", CoveredMessageCount: 5, SummaryText: "covers subtree"},
		{RecordType: chatContextRecordSnapshot, ConversationID: messages[0].ConversationID, UserID: 1, RunID: "run_outside", CoveredUntilMessageID: messages[0].ID, CoveredUntilPublicID: "msg_u1", CoveragePathHash: "hash_outside", CoveredMessageCount: 1, SummaryText: "covers root"},
		{RecordType: "artifact", ConversationID: messages[0].ConversationID, UserID: 1, MessageID: messages[4].ID, Kind: "rag", SourceType: "file"},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatalf("create context records: %v", err)
	}

	// 删除 A2：其子树含 A2/U3/A3，两条快照边界（U3 子树内、U1 子树外）分别命中/保留。
	if _, err := repo.DeleteMessageAndReparentChildren(ctx, 1, messages[0].ConversationID, messages[3].ID); err != nil {
		t.Fatalf("delete message: %v", err)
	}

	var snapshotCount int64
	if err := db.Model(&model.ChatContextRecord{}).
		Where("record_type = ? AND conversation_id = ?", chatContextRecordSnapshot, messages[0].ConversationID).
		Count(&snapshotCount).Error; err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if snapshotCount != 1 {
		t.Fatalf("snapshot count = %d, want 1", snapshotCount)
	}

	var kept model.ChatContextRecord
	if err := db.Where("record_type = ? AND conversation_id = ?", chatContextRecordSnapshot, messages[0].ConversationID).First(&kept).Error; err != nil {
		t.Fatalf("reload kept snapshot: %v", err)
	}
	if kept.CoveredUntilMessageID != messages[0].ID {
		t.Fatalf("kept snapshot boundary = %d, want %d", kept.CoveredUntilMessageID, messages[0].ID)
	}

	var artifactCount int64
	if err := db.Model(&model.ChatContextRecord{}).
		Where("record_type = ? AND conversation_id = ?", "artifact", messages[0].ConversationID).
		Count(&artifactCount).Error; err != nil {
		t.Fatalf("count artifacts: %v", err)
	}
	if artifactCount != 1 {
		t.Fatalf("artifact count = %d, want 1", artifactCount)
	}
}
