package conversation

import (
	"context"
	"errors"
	"fmt"
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestUpsertMessageUIStatesReplacesAndClearsBlocks(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	conversation := models.Conversation{
		UserID: 1, PublicID: "conv_ui_state", Title: "UI", LabelsJSON: "[]",
		SessionKey: "session_ui_state", MessageCount: 1, Status: "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message := models.Message{
		ConversationID: conversation.ID, UserID: 1, PublicID: "msg_ui_state",
		Role: "assistant", ContentType: "text", Content: "reply", BranchReason: "default", Status: "success",
	}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}

	if err := repo.UpsertMessageUIStates(ctx, 1, conversation.ID, message.ID, map[string]string{
		"loan": `{"amount":300,"years":20}`,
		"quiz": `{"answers":[1,2]}`,
	}); err != nil {
		t.Fatalf("upsert ui states: %v", err)
	}

	// 同块再次写入应覆盖旧值而不是新增一行；空值表示清除该块。
	if err := repo.UpsertMessageUIStates(ctx, 1, conversation.ID, message.ID, map[string]string{
		"loan": `{"amount":500,"years":15}`,
		"quiz": "",
	}); err != nil {
		t.Fatalf("replace ui states: %v", err)
	}

	stateMap, err := repo.GetMessageUIStateMap(ctx, 1, []uint{message.ID})
	if err != nil {
		t.Fatalf("get ui state map: %v", err)
	}
	blocks := stateMap[message.ID]
	if len(blocks) != 1 {
		t.Fatalf("expected 1 remaining block, got %d (%v)", len(blocks), blocks)
	}
	if blocks["loan"] != `{"amount":500,"years":15}` {
		t.Fatalf("loan state = %q", blocks["loan"])
	}

	var count int64
	if err := db.Model(&models.ConversationMessageUIState{}).
		Where("message_id = ?", message.ID).
		Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 persisted row after replace, got %d", count)
	}

	// 其它用户读不到该状态。
	otherUserMap, err := repo.GetMessageUIStateMap(ctx, 2, []uint{message.ID})
	if err != nil {
		t.Fatalf("get ui state map for other user: %v", err)
	}
	if len(otherUserMap) != 0 {
		t.Fatalf("other user should not read ui states, got %v", otherUserMap)
	}
}

func TestUpsertMessageUIStatesRejectsTooManyBlocks(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	conversation := models.Conversation{
		UserID: 1, PublicID: "conv_ui_limit", Title: "UI", LabelsJSON: "[]",
		SessionKey: "session_ui_limit", MessageCount: 1, Status: "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message := models.Message{
		ConversationID: conversation.ID, UserID: 1, PublicID: "msg_ui_limit",
		Role: "assistant", ContentType: "text", Content: "reply", BranchReason: "default", Status: "success",
	}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}

	full := make(map[string]string, maxUIStateBlocksPerMessage)
	for index := 0; index < maxUIStateBlocksPerMessage; index++ {
		full[fmt.Sprintf("block-%d", index)] = `{"v":1}`
	}
	if err := repo.UpsertMessageUIStates(ctx, 1, conversation.ID, message.ID, full); err != nil {
		t.Fatalf("upsert full set: %v", err)
	}

	// 已达到上限后再新增块应被拒绝，且不写入任何行。
	if err := repo.UpsertMessageUIStates(ctx, 1, conversation.ID, message.ID, map[string]string{"extra": `{"v":1}`}); !errors.Is(err, repository.ErrMessageUIStateLimitExceeded) {
		t.Fatalf("expected limit error, got %v", err)
	}

	var count int64
	if err := db.Model(&models.ConversationMessageUIState{}).
		Where("message_id = ?", message.ID).
		Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != int64(maxUIStateBlocksPerMessage) {
		t.Fatalf("expected %d rows, got %d", maxUIStateBlocksPerMessage, count)
	}

	// 达到上限后仍可更新已有的块：覆盖不新增行，不应被上限拦下。
	if err := repo.UpsertMessageUIStates(ctx, 1, conversation.ID, message.ID, map[string]string{"block-1": `{"v":2}`}); err != nil {
		t.Fatalf("update existing block at limit: %v", err)
	}
	stateMap, err := repo.GetMessageUIStateMap(ctx, 1, []uint{message.ID})
	if err != nil {
		t.Fatalf("get ui state map: %v", err)
	}
	if stateMap[message.ID]["block-1"] != `{"v":2}` {
		t.Fatalf("block-1 state = %q", stateMap[message.ID]["block-1"])
	}

	// 删除一个块后应能再写入新块。
	if err := repo.UpsertMessageUIStates(ctx, 1, conversation.ID, message.ID, map[string]string{
		"block-0": "",
		"extra":   `{"v":1}`,
	}); err != nil {
		t.Fatalf("replace with removal: %v", err)
	}
}

func TestUpsertMessageUIStatesRejectsInvalidBlockID(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)

	err := repo.UpsertMessageUIStates(context.Background(), 1, 1, 1, map[string]string{"bad id": `{"v":1}`})
	if !errors.Is(err, repository.ErrMessageUIStateInvalid) {
		t.Fatalf("expected invalid block id error, got %v", err)
	}
}

// 消息在请求途中被删除（或并不属于该用户）时不应留下孤儿状态行。
func TestUpsertMessageUIStatesRequiresOwnedMessage(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	conversation := models.Conversation{
		UserID: 1, PublicID: "conv_ui_owner", Title: "UI", LabelsJSON: "[]",
		SessionKey: "session_ui_owner", MessageCount: 1, Status: "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message := models.Message{
		ConversationID: conversation.ID, UserID: 1, PublicID: "msg_ui_owner",
		Role: "assistant", ContentType: "text", Content: "reply", BranchReason: "default", Status: "success",
	}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}

	if err := repo.UpsertMessageUIStates(ctx, 2, conversation.ID, message.ID, map[string]string{"loan": `{"v":1}`}); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("other user write: expected not found, got %v", err)
	}
	if err := db.Delete(&message).Error; err != nil {
		t.Fatalf("delete message: %v", err)
	}
	if err := repo.UpsertMessageUIStates(ctx, 1, conversation.ID, message.ID, map[string]string{"loan": `{"v":1}`}); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("deleted message write: expected not found, got %v", err)
	}

	var count int64
	if err := db.Model(&models.ConversationMessageUIState{}).Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no orphan rows, got %d", count)
	}
}

func TestCreateForkedConversationClonesMessageUIStates(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	source := models.Conversation{
		UserID: 1, PublicID: "conv_source", Title: "Source", LabelsJSON: "[]",
		SessionKey: "session_source", MessageCount: 2, Status: "active",
	}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source conversation: %v", err)
	}
	root := models.Message{
		ConversationID: source.ID, UserID: 1, PublicID: "msg_source_root",
		Role: "user", ContentType: "text", Content: "root", BranchReason: "default", Status: "success",
	}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create source root: %v", err)
	}
	leaf := models.Message{
		ConversationID: source.ID, UserID: 1, PublicID: "msg_source_leaf", ParentMessageID: &root.ID,
		Role: "assistant", ContentType: "text", Content: "leaf", BranchReason: "default", Status: "success",
	}
	if err := db.Create(&leaf).Error; err != nil {
		t.Fatalf("create source leaf: %v", err)
	}
	if err := db.Create(&models.ConversationMessageUIState{
		UserID: 1, ConversationID: source.ID, MessageID: leaf.ID,
		BlockID: "loan", StateJSON: `{"amount":300}`,
	}).Error; err != nil {
		t.Fatalf("create source ui state: %v", err)
	}

	target := &domainconversation.Conversation{
		UserID: 1, PublicID: "conv_fork", Title: "Fork", LabelsJSON: "[]",
		SessionKey: "session_fork", MessageCount: 2, Status: "active",
	}
	if err := repo.CreateForkedConversation(ctx, repository.CreateForkedConversationInput{
		SourceConversationID: source.ID,
		Conversation:         target,
		Messages: []repository.ForkConversationMessage{
			{SourceMessageID: root.ID, Message: domainconversation.Message{
				PublicID: "msg_fork_root", Role: "user", ContentType: "text",
				Content: "root", BranchReason: "default", Status: "success",
			}},
			{SourceMessageID: leaf.ID, SourceParentMessageID: &root.ID, Message: domainconversation.Message{
				PublicID: "msg_fork_leaf", Role: "assistant", ContentType: "text",
				Content: "leaf", BranchReason: "default", Status: "success",
			}},
		},
	}); err != nil {
		t.Fatalf("create forked conversation: %v", err)
	}

	var forkedLeaf models.Message
	if err := db.Where("public_id = ?", "msg_fork_leaf").First(&forkedLeaf).Error; err != nil {
		t.Fatalf("load forked leaf: %v", err)
	}
	stateMap, err := repo.GetMessageUIStateMap(ctx, 1, []uint{forkedLeaf.ID})
	if err != nil {
		t.Fatalf("get forked ui state map: %v", err)
	}
	blocks := stateMap[forkedLeaf.ID]
	if blocks["loan"] != `{"amount":300}` {
		t.Fatalf("forked ui state = %v", blocks)
	}

	// 源消息仍保留自己的状态，两边互不影响。
	var sourceRows, forkedRows int64
	if err := db.Model(&models.ConversationMessageUIState{}).
		Where("message_id = ?", leaf.ID).Count(&sourceRows).Error; err != nil {
		t.Fatalf("count source rows: %v", err)
	}
	if err := db.Model(&models.ConversationMessageUIState{}).
		Where("message_id = ?", forkedLeaf.ID).Count(&forkedRows).Error; err != nil {
		t.Fatalf("count forked rows: %v", err)
	}
	if sourceRows != 1 || forkedRows != 1 {
		t.Fatalf("expected one row on each side, got source=%d forked=%d", sourceRows, forkedRows)
	}
}

func TestDeleteMessageRemovesItsUIStates(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	if err := db.AutoMigrate(&models.ChatContextRecord{}); err != nil {
		t.Fatalf("migrate context records: %v", err)
	}
	repo := NewRepo(db)
	ctx := context.Background()

	conversation := models.Conversation{
		UserID: 1, PublicID: "conv_ui_delete", Title: "UI", LabelsJSON: "[]",
		SessionKey: "session_ui_delete", MessageCount: 3, Status: "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	root := models.Message{
		ConversationID: conversation.ID, UserID: 1, PublicID: "msg_ui_delete_root",
		Role: "user", ContentType: "text", Content: "root", BranchReason: "default", Status: "success",
	}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create root: %v", err)
	}
	deleted := models.Message{
		ConversationID: conversation.ID, UserID: 1, PublicID: "msg_ui_delete_target", ParentMessageID: &root.ID,
		Role: "assistant", ContentType: "text", Content: "reply", BranchReason: "default", Status: "success",
	}
	if err := db.Create(&deleted).Error; err != nil {
		t.Fatalf("create target: %v", err)
	}
	kept := models.Message{
		ConversationID: conversation.ID, UserID: 1, PublicID: "msg_ui_delete_kept", ParentMessageID: &root.ID,
		Role: "assistant", ContentType: "text", Content: "sibling", BranchReason: "retry", Status: "success",
	}
	if err := db.Create(&kept).Error; err != nil {
		t.Fatalf("create sibling: %v", err)
	}
	for _, messageID := range []uint{deleted.ID, kept.ID} {
		if err := repo.UpsertMessageUIStates(ctx, 1, conversation.ID, messageID, map[string]string{"loan": `{"v":1}`}); err != nil {
			t.Fatalf("seed ui state: %v", err)
		}
	}

	if _, err := repo.DeleteMessageAndReparentChildren(ctx, 1, conversation.ID, deleted.ID); err != nil {
		t.Fatalf("delete message: %v", err)
	}

	stateMap, err := repo.GetMessageUIStateMap(ctx, 1, []uint{deleted.ID, kept.ID})
	if err != nil {
		t.Fatalf("get ui state map: %v", err)
	}
	if _, exists := stateMap[deleted.ID]; exists {
		t.Fatalf("deleted message should not keep ui state, got %v", stateMap[deleted.ID])
	}
	if stateMap[kept.ID]["loan"] != `{"v":1}` {
		t.Fatalf("sibling ui state must be untouched, got %v", stateMap[kept.ID])
	}
}
