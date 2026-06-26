package conversation

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTranslateErrorAllowsNil(t *testing.T) {
	if err := translateError(nil); err != nil {
		t.Fatalf("translateError(nil) = %v, want nil", err)
	}
}

func TestListMessagesBeforeIDReturnsPreviousWindowAscending(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	conversation := model.Conversation{
		UserID:     1,
		PublicID:   "conv_before",
		Title:      "before window",
		LabelsJSON: "[]",
		SessionKey: "session_before",
		Status:     "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	messages := make([]model.Message, 0, 5)
	var parentID *uint
	for index := 1; index <= 5; index++ {
		message := model.Message{
			ConversationID:  conversation.ID,
			UserID:          1,
			PublicID:        fmt.Sprintf("msg_%d", index),
			ParentMessageID: parentID,
			Role:            "user",
			ContentType:     "text",
			Content:         fmt.Sprintf("message %d", index),
			BranchReason:    "default",
			Status:          "success",
		}
		if err := db.Create(&message).Error; err != nil {
			t.Fatalf("create message %d: %v", index, err)
		}
		messages = append(messages, message)
		nextParentID := message.ID
		parentID = &nextParentID
	}

	got, total, err := repo.ListMessagesBeforeID(ctx, conversation.ID, messages[4].ID, 2)
	if err != nil {
		t.Fatalf("ListMessagesBeforeID() error = %v", err)
	}
	if total != int64(len(messages)) {
		t.Fatalf("total = %d, want %d", total, len(messages))
	}
	if len(got) != 2 || got[0].PublicID != "msg_3" || got[1].PublicID != "msg_4" {
		t.Fatalf("unexpected previous window: %#v", got)
	}
	if got[1].ParentPublicID != "msg_3" {
		t.Fatalf("expected parent public id hydrated, got %q", got[1].ParentPublicID)
	}
}

func TestListMessageAncestorsUntilStopsAtBoundary(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	conversation := model.Conversation{
		UserID:     1,
		PublicID:   "conv_ancestors_until",
		Title:      "ancestors until",
		LabelsJSON: "[]",
		SessionKey: "session_ancestors_until",
		Status:     "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	messages := make([]model.Message, 0, 6)
	var parentID *uint
	for index := 1; index <= 6; index++ {
		message := model.Message{
			ConversationID:  conversation.ID,
			UserID:          1,
			PublicID:        fmt.Sprintf("msg_%d", index),
			ParentMessageID: parentID,
			Role:            "user",
			ContentType:     "text",
			Content:         fmt.Sprintf("message %d", index),
			BranchReason:    "default",
			Status:          "success",
		}
		if err := db.Create(&message).Error; err != nil {
			t.Fatalf("create message %d: %v", index, err)
		}
		messages = append(messages, message)
		nextParentID := message.ID
		parentID = &nextParentID
	}

	got, found, err := repo.ListMessageAncestorsUntil(ctx, conversation.ID, messages[5].ID, messages[2].ID, 10)
	if err != nil {
		t.Fatalf("ListMessageAncestorsUntil() error = %v", err)
	}
	if !found {
		t.Fatal("expected boundary to be found")
	}
	if len(got) != 4 {
		t.Fatalf("expected boundary through leaf, got %#v", got)
	}
	if got[0].PublicID != "msg_3" || got[len(got)-1].PublicID != "msg_6" {
		t.Fatalf("expected msg_3..msg_6, got %#v", got)
	}
	if got[0].ParentPublicID != "msg_2" {
		t.Fatalf("expected boundary parent public id hydrated, got %q", got[0].ParentPublicID)
	}
}

func TestListMessageAncestorsUntilReportsMissingBoundary(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	conversation := model.Conversation{
		UserID:     1,
		PublicID:   "conv_missing_boundary",
		Title:      "missing boundary",
		LabelsJSON: "[]",
		SessionKey: "session_missing_boundary",
		Status:     "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message := model.Message{
		ConversationID: conversation.ID,
		UserID:         1,
		PublicID:       "msg_1",
		Role:           "user",
		ContentType:    "text",
		Content:        "message 1",
		BranchReason:   "default",
		Status:         "success",
	}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}

	got, found, err := repo.ListMessageAncestorsUntil(ctx, conversation.ID, message.ID, message.ID+100, 10)
	if err != nil {
		t.Fatalf("ListMessageAncestorsUntil() error = %v", err)
	}
	if found {
		t.Fatal("expected boundary to be missing")
	}
	if len(got) != 1 || got[0].PublicID != "msg_1" {
		t.Fatalf("expected available ancestor path, got %#v", got)
	}
}

func TestUpdateConversationMetadataSQLiteUsesPortableTrim(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	conversation := model.Conversation{
		UserID:     1,
		PublicID:   "conv_metadata_sqlite",
		Title:      " 新会话 ",
		LabelsJSON: "[]",
		SessionKey: "session_metadata_sqlite",
		Status:     "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	updated, err := repo.UpdateConversationMetadata(ctx, conversation.ID, repository.ConversationMetadataPatch{
		Title:      "SQLite 标题",
		LabelsJSON: `["技术"]`,
	})
	if err != nil {
		t.Fatalf("UpdateConversationMetadata() error = %v", err)
	}
	if updated.Title != "SQLite 标题" {
		t.Fatalf("updated title = %q, want %q", updated.Title, "SQLite 标题")
	}
	if updated.LabelsJSON != `["技术"]` {
		t.Fatalf("updated labels = %q, want %q", updated.LabelsJSON, `["技术"]`)
	}
}

func TestUpdateConversationMetadataCanReplaceAutomaticFallbackTitle(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	conversation := model.Conversation{
		UserID:     1,
		PublicID:   "conv_metadata_fallback",
		Title:      "画一张城市夜景",
		LabelsJSON: "[]",
		SessionKey: "session_metadata_fallback",
		Status:     "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	updated, err := repo.UpdateConversationMetadata(ctx, conversation.ID, repository.ConversationMetadataPatch{
		Title:             "城市夜景图像生成",
		ReplaceableTitles: []string{"画一张城市夜景"},
	})
	if err != nil {
		t.Fatalf("UpdateConversationMetadata() error = %v", err)
	}
	if updated.Title != "城市夜景图像生成" {
		t.Fatalf("updated title = %q, want %q", updated.Title, "城市夜景图像生成")
	}

	if err := db.Model(&model.Conversation{}).Where("id = ?", conversation.ID).Update("title", "手动标题").Error; err != nil {
		t.Fatalf("set manual title: %v", err)
	}
	updated, err = repo.UpdateConversationMetadata(ctx, conversation.ID, repository.ConversationMetadataPatch{
		Title:             "不应覆盖",
		ReplaceableTitles: []string{"画一张城市夜景"},
	})
	if err != nil {
		t.Fatalf("UpdateConversationMetadata() error = %v", err)
	}
	if updated.Title != "手动标题" {
		t.Fatalf("manual title was overwritten: got %q", updated.Title)
	}
}

func TestListConversationsByUserSearchesMetadataProjectsAndMessages(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	project := model.ConversationProject{
		UserID:      1,
		PublicID:    "proj_research",
		Name:        "Research Notes",
		Description: "knowledge base",
		Status:      "active",
	}
	if err := db.Create(&project).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}

	projectConversation := model.Conversation{
		UserID:     1,
		ProjectID:  &project.ID,
		PublicID:   "conv_project_search",
		Title:      "Project conversation",
		LabelsJSON: "[]",
		Model:      "gpt-test",
		Provider:   "openai",
		SessionKey: "session_project_search",
		Status:     "active",
	}
	titleConversation := model.Conversation{
		UserID:     1,
		PublicID:   "conv_title_search",
		Title:      "Quarterly Budget",
		LabelsJSON: `["finance"]`,
		Model:      "claude-test",
		Provider:   "anthropic",
		SessionKey: "session_title_search",
		Status:     "active",
	}
	messageConversation := model.Conversation{
		UserID:     1,
		PublicID:   "conv_message_search",
		Title:      "Ordinary chat",
		LabelsJSON: "[]",
		Model:      "gemini-test",
		Provider:   "gemini",
		SessionKey: "session_message_search",
		Status:     "active",
	}
	otherUserConversation := model.Conversation{
		UserID:     2,
		PublicID:   "conv_other_user",
		Title:      "Private Budget",
		LabelsJSON: "[]",
		Model:      "gpt-test",
		Provider:   "openai",
		SessionKey: "session_other_user",
		Status:     "active",
	}
	for _, conversation := range []model.Conversation{
		projectConversation,
		titleConversation,
		messageConversation,
		otherUserConversation,
	} {
		if err := db.Create(&conversation).Error; err != nil {
			t.Fatalf("create conversation %q: %v", conversation.PublicID, err)
		}
	}

	var messageTarget model.Conversation
	if err := db.Where("public_id = ?", "conv_message_search").First(&messageTarget).Error; err != nil {
		t.Fatalf("load message target: %v", err)
	}
	if err := db.Create(&model.Message{
		ConversationID: messageTarget.ID,
		UserID:         1,
		PublicID:       "msg_search",
		Role:           "user",
		ContentType:    "text",
		Content:        "The launch checklist mentions AuroraKeyword",
		BranchReason:   "default",
		Status:         "success",
	}).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}

	tests := []struct {
		name   string
		query  string
		wantID string
	}{
		{name: "title", query: "budget", wantID: "conv_title_search"},
		{name: "project", query: "research", wantID: "conv_project_search"},
		{name: "message", query: "aurorakeyword", wantID: "conv_message_search"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, total, err := repo.ListConversationsByUser(ctx, 1, 0, 10, "active", "all", "all", "all", tt.query)
			if err != nil {
				t.Fatalf("ListConversationsByUser() error = %v", err)
			}
			if total != 1 {
				t.Fatalf("total = %d, want 1; items=%#v", total, items)
			}
			if len(items) != 1 || items[0].PublicID != tt.wantID {
				t.Fatalf("items = %#v, want %q", items, tt.wantID)
			}
		})
	}
}

func TestGetLatestConversationRunModelUsesLatestSuccessfulUserRun(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Now()

	runs := []model.ConversationRun{
		{
			UserID:            1,
			ConversationID:    11,
			RunID:             "run_old_success",
			PlatformModelName: "gpt-old",
			Status:            "success",
			StartedAt:         now.Add(-5 * time.Minute),
		},
		{
			UserID:            2,
			ConversationID:    21,
			RunID:             "run_other_user",
			PlatformModelName: "gpt-other",
			Status:            "success",
			StartedAt:         now.Add(-4 * time.Minute),
		},
		{
			UserID:            1,
			ConversationID:    12,
			RunID:             "run_latest_success",
			PlatformModelName: "gpt-latest",
			Status:            "success",
			StartedAt:         now.Add(-3 * time.Minute),
		},
		{
			UserID:            1,
			ConversationID:    13,
			RunID:             "run_error",
			PlatformModelName: "gpt-error",
			Status:            "error",
			StartedAt:         now.Add(-2 * time.Minute),
		},
		{
			UserID:         1,
			ConversationID: 14,
			RunID:          "run_empty_model",
			Status:         "success",
			StartedAt:      now.Add(-1 * time.Minute),
		},
	}
	for _, run := range runs {
		if err := db.Create(&run).Error; err != nil {
			t.Fatalf("create run %q: %v", run.RunID, err)
		}
	}

	got, err := repo.GetLatestConversationRunModel(ctx, 1)
	if err != nil {
		t.Fatalf("GetLatestConversationRunModel() error = %v", err)
	}
	if got == nil || got.PlatformModelName != "gpt-latest" {
		t.Fatalf("latest model = %#v, want gpt-latest", got)
	}
}

func openConversationRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&model.Conversation{}, &model.ConversationProject{}, &model.ConversationShare{}, &model.Message{}, &model.Attachment{}, &model.FileObject{}, &model.ConversationRun{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	return db
}
