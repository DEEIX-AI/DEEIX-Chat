package conversation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
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

func TestConversationProjectDefaultsRoundTripAndDelete(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	project := domainconversation.ConversationProject{
		UserID:            1,
		PublicID:          "project_defaults",
		Name:              "Project defaults",
		MCPDefaultMode:    domainconversation.ConversationProjectMCPDefaultModeCustom,
		DefaultMCPToolIDs: []uint{7, 3},
		DefaultSkillIDs:   []uint{11, 5},
		Status:            "active",
	}
	if err := repo.CreateConversationProject(ctx, &project); err != nil {
		t.Fatalf("CreateConversationProject() error = %v", err)
	}
	if !reflect.DeepEqual(project.DefaultMCPToolIDs, []uint{7, 3}) || !reflect.DeepEqual(project.DefaultSkillIDs, []uint{11, 5}) {
		t.Fatalf("created defaults = MCP %v Skills %v", project.DefaultMCPToolIDs, project.DefaultSkillIDs)
	}

	loaded, err := repo.GetConversationProjectByPublicID(ctx, 1, project.PublicID)
	if err != nil {
		t.Fatalf("GetConversationProjectByPublicID() error = %v", err)
	}
	if loaded.MCPDefaultMode != domainconversation.ConversationProjectMCPDefaultModeCustom ||
		!reflect.DeepEqual(loaded.DefaultMCPToolIDs, []uint{7, 3}) ||
		!reflect.DeepEqual(loaded.DefaultSkillIDs, []uint{11, 5}) {
		t.Fatalf("loaded project defaults = %#v", loaded)
	}

	nextMCPToolIDs := []uint{}
	nextSkillIDs := []uint{5}
	inheritMode := domainconversation.ConversationProjectMCPDefaultModeInherit
	updated, err := repo.UpdateConversationProjectMetadataByPublicID(ctx, 1, project.PublicID, domainconversation.ConversationProjectPatch{
		MCPDefaultMode:    &inheritMode,
		DefaultMCPToolIDs: &nextMCPToolIDs,
		DefaultSkillIDs:   &nextSkillIDs,
	})
	if err != nil {
		t.Fatalf("UpdateConversationProjectMetadataByPublicID() error = %v", err)
	}
	if updated.MCPDefaultMode != inheritMode || len(updated.DefaultMCPToolIDs) != 0 || !reflect.DeepEqual(updated.DefaultSkillIDs, nextSkillIDs) {
		t.Fatalf("updated project defaults = %#v", updated)
	}

	if _, err = repo.DeleteConversationProjectByPublicID(ctx, 1, project.PublicID, false, false); err != nil {
		t.Fatalf("DeleteConversationProjectByPublicID() error = %v", err)
	}
	var associationCount int64
	if err = db.Model(&model.ConversationProjectMCPTool{}).Where("project_id = ?", project.ID).Count(&associationCount).Error; err != nil {
		t.Fatalf("count project MCP associations: %v", err)
	}
	if associationCount != 0 {
		t.Fatalf("project MCP association count = %d, want 0", associationCount)
	}
	if err = db.Model(&model.ConversationProjectSkill{}).Where("project_id = ?", project.ID).Count(&associationCount).Error; err != nil {
		t.Fatalf("count project Skill associations: %v", err)
	}
	if associationCount != 0 {
		t.Fatalf("project Skill association count = %d, want 0", associationCount)
	}
}

func TestListConversationEventLogsHydratesRunRouteSnapshot(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Now()

	run := model.ConversationRun{
		RunID:             "run_with_route",
		UserID:            1,
		ConversationID:    2,
		ProviderProtocol:  "openai_responses",
		UpstreamName:      "OpenAI Official",
		PlatformModelName: "gpt-5.5",
		RoutedBindingCode: "binding_openai",
		UpstreamModelName: "gpt-5.5-pro",
		Status:            "error",
		StartedAt:         now,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("create conversation run: %v", err)
	}

	events := []model.ChatRunEvent{
		{
			ConversationID: 2,
			UserID:         1,
			RunID:          run.RunID,
			EventScope:     "trace_event",
			EventID:        "event_with_route",
			EventType:      "error",
			Status:         "error",
			StartedAt:      now,
		},
		{
			ConversationID: 2,
			UserID:         1,
			RunID:          "run_before_route",
			EventScope:     "trace_event",
			EventID:        "event_without_route",
			EventType:      "error",
			Status:         "error",
			StartedAt:      now,
		},
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("create conversation events: %v", err)
	}

	items, total, err := repo.ListConversationEventLogs(ctx, repository.ConversationEventLogListFilter{}, 0, 10)
	if err != nil {
		t.Fatalf("ListConversationEventLogs() error = %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("got total=%d len=%d, want 2", total, len(items))
	}
	itemsByRunID := make(map[string]domainconversation.EventLog, len(items))
	for _, item := range items {
		itemsByRunID[item.RunID] = item
	}
	withRoute := itemsByRunID[run.RunID]
	if withRoute.UpstreamName != run.UpstreamName ||
		withRoute.ProviderProtocol != run.ProviderProtocol ||
		withRoute.PlatformModelName != run.PlatformModelName ||
		withRoute.RoutedBindingCode != run.RoutedBindingCode ||
		withRoute.UpstreamModelName != run.UpstreamModelName {
		t.Fatalf("route snapshot = %#v, want run snapshot %#v", withRoute, run)
	}
	withoutRoute := itemsByRunID["run_before_route"]
	if withoutRoute.UpstreamName != "" || withoutRoute.ProviderProtocol != "" || withoutRoute.UpstreamModelName != "" {
		t.Fatalf("unexpected route snapshot for unmatched run: %#v", withoutRoute)
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

func TestUpdateAssistantMessageCompletionPersistsReasoningContent(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	conversation := model.Conversation{
		UserID:     1,
		PublicID:   "conv_reasoning_completion",
		Title:      "reasoning completion",
		LabelsJSON: "[]",
		SessionKey: "session_reasoning_completion",
		Status:     "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message := model.Message{
		ConversationID: conversation.ID,
		UserID:         1,
		PublicID:       "msg_reasoning_completion",
		Role:           "assistant",
		ContentType:    "text",
		Content:        "",
		BranchReason:   "default",
		Status:         "pending",
	}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}

	err := repo.UpdateAssistantMessageCompletion(ctx, message.ID, repository.AssistantMessageCompletionUpdate{
		ContentType:      "text",
		Content:          "final answer",
		ReasoningContent: "stored reasoning",
		Status:           "success",
	})
	if err != nil {
		t.Fatalf("UpdateAssistantMessageCompletion() error = %v", err)
	}

	got, err := repo.GetMessageByID(ctx, conversation.ID, message.ID)
	if err != nil {
		t.Fatalf("GetMessageByID() error = %v", err)
	}
	if got.Content != "final answer" || got.ReasoningContent != "stored reasoning" {
		t.Fatalf("unexpected completed message: %#v", got)
	}
}

func TestUpdateConversationMetadataSQLiteUsesPortableTrim(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	conversation := model.Conversation{
		UserID:     1,
		PublicID:   "conv_metadata_sqlite",
		Title:      " 新对话 ",
		LabelsJSON: "[]",
		SessionKey: "session_metadata_sqlite",
		Status:     "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	updated, err := repo.UpdateConversationMetadata(ctx, conversation.ID, repository.ConversationMetadataPatch{
		Title: "SQLite 标题",
	})
	if err != nil {
		t.Fatalf("UpdateConversationMetadata() error = %v", err)
	}
	if updated.Title != "SQLite 标题" {
		t.Fatalf("updated title = %q, want %q", updated.Title, "SQLite 标题")
	}
}

func TestUpdateConversationLabelsAppliesGeneratedLabelsWhenEligible(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	conversation := model.Conversation{
		PublicID:   "generated-label-eligible",
		UserID:     1,
		Title:      "已有标题",
		LabelsJSON: `[]`,
		SessionKey: "generated-label-eligible-session",
		Status:     "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	updated, applied, err := repo.SetGeneratedConversationLabelsIfEligible(context.Background(), conversation.ID, `["自动标签"]`)
	if err != nil {
		t.Fatalf("SetGeneratedConversationLabelsIfEligible() error = %v", err)
	}
	if !applied || updated.LabelsJSON != `["自动标签"]` {
		t.Fatalf("generated labels were not applied: applied=%v labels=%q", applied, updated.LabelsJSON)
	}
}

func TestUpdateConversationLabelsByPublicIDIsUserScopedAndMarksManualManagement(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	conversation := model.Conversation{
		PublicID:   "manual-label-user-scope",
		UserID:     1,
		Title:      "已有标题",
		LabelsJSON: `[]`,
		SessionKey: "manual-label-user-scope-session",
		Status:     "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	if _, err := repo.UpdateConversationLabelsByPublicID(context.Background(), 2, conversation.PublicID, `["越权标签"]`); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected other user update to return not found, got %v", err)
	}
	updated, err := repo.UpdateConversationLabelsByPublicID(context.Background(), 1, conversation.PublicID, `["手动标签"]`)
	if err != nil {
		t.Fatalf("UpdateConversationLabelsByPublicID() error = %v", err)
	}
	if updated.LabelsJSON != `["手动标签"]` || !updated.LabelsManuallyManaged {
		t.Fatalf("manual labels were not persisted correctly: %#v", updated)
	}
}

func TestUpdateConversationLabelsGeneratedLabelsDoNotOverwriteManualLabels(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	conversation := model.Conversation{
		PublicID:              "generated-label-race",
		UserID:                1,
		Title:                 "已有标题",
		LabelsJSON:            `["手动标签"]`,
		LabelsManuallyManaged: true,
		SessionKey:            "generated-label-race-session",
		Status:                "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	updated, applied, err := repo.SetGeneratedConversationLabelsIfEligible(context.Background(), conversation.ID, `["自动标签"]`)
	if err != nil {
		t.Fatalf("SetGeneratedConversationLabelsIfEligible() error = %v", err)
	}
	if applied {
		t.Fatal("generated labels update unexpectedly applied")
	}
	if updated.LabelsJSON != `["手动标签"]` {
		t.Fatalf("generated labels overwrote manual labels: %q", updated.LabelsJSON)
	}
	if !updated.UpdatedAt.Equal(conversation.UpdatedAt) {
		t.Fatalf("skipped generated labels changed updated_at: got %v, want %v", updated.UpdatedAt, conversation.UpdatedAt)
	}
}

func TestUpdateConversationLabelsGeneratedLabelsDoNotRestoreManuallyClearedLabels(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	conversation := model.Conversation{
		PublicID:              "generated-label-manual-clear-race",
		UserID:                1,
		Title:                 "已有标题",
		LabelsJSON:            `[]`,
		LabelsManuallyManaged: true,
		SessionKey:            "generated-label-manual-clear-race-session",
		Status:                "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	updated, applied, err := repo.SetGeneratedConversationLabelsIfEligible(context.Background(), conversation.ID, `["自动标签"]`)
	if err != nil {
		t.Fatalf("SetGeneratedConversationLabelsIfEligible() error = %v", err)
	}
	if applied {
		t.Fatal("generated labels update unexpectedly applied")
	}
	if updated.LabelsJSON != `[]` {
		t.Fatalf("generated labels restored manually cleared labels: %q", updated.LabelsJSON)
	}
	if !updated.UpdatedAt.Equal(conversation.UpdatedAt) {
		t.Fatalf("skipped generated labels changed updated_at: got %v, want %v", updated.UpdatedAt, conversation.UpdatedAt)
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
	toolOnlyConversation := model.Conversation{
		UserID:     1,
		PublicID:   "conv_tool_only_search",
		Title:      "Tool output",
		LabelsJSON: "[]",
		Model:      "gpt-test",
		Provider:   "openai",
		SessionKey: "session_tool_only_search",
		Status:     "active",
	}
	wildcardConversation := model.Conversation{
		UserID:     1,
		PublicID:   "conv_literal_wildcard_search",
		Title:      "Progress 100%",
		LabelsJSON: "[]",
		Model:      "gpt-test",
		Provider:   "openai",
		SessionKey: "session_literal_wildcard_search",
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
		toolOnlyConversation,
		wildcardConversation,
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
	var toolOnlyTarget model.Conversation
	if err := db.Where("public_id = ?", "conv_tool_only_search").First(&toolOnlyTarget).Error; err != nil {
		t.Fatalf("load tool-only target: %v", err)
	}
	if err := db.Create(&model.Message{
		ConversationID: toolOnlyTarget.ID,
		UserID:         1,
		PublicID:       "msg_tool_only_search",
		Role:           "tool",
		ContentType:    "text",
		Content:        "InternalToolOnlyKeyword",
		BranchReason:   "default",
		Status:         "success",
	}).Error; err != nil {
		t.Fatalf("create tool-only message: %v", err)
	}

	tests := []struct {
		name   string
		query  string
		wantID string
	}{
		{name: "title", query: "budget", wantID: "conv_title_search"},
		{name: "project", query: "research", wantID: "conv_project_search"},
		{name: "message", query: "aurorakeyword", wantID: "conv_message_search"},
		{name: "literal wildcard", query: "%", wantID: "conv_literal_wildcard_search"},
		{name: "tool messages are excluded", query: "internaltoolonlykeyword", wantID: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, total, err := repo.ListConversationsByUser(ctx, 1, 0, 10, "active", "all", "all", "all", tt.query)
			if err != nil {
				t.Fatalf("ListConversationsByUser() error = %v", err)
			}
			if tt.wantID == "" {
				if total != 0 || len(items) != 0 {
					t.Fatalf("items = %#v, total = %d, want no results", items, total)
				}
				return
			}
			if total != 1 || len(items) != 1 || items[0].PublicID != tt.wantID {
				t.Fatalf("items = %#v, want %q", items, tt.wantID)
			}
		})
	}
}

func TestListConversationsForSearchReturnsOrderedWindowWithoutStatusFiltering(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Now()

	items := []model.Conversation{
		{
			BaseModel:  model.BaseModel{UpdatedAt: now.Add(-2 * time.Hour)},
			UserID:     1,
			PublicID:   "conv_search_oldest",
			Title:      "Needle oldest",
			LabelsJSON: "[]",
			Model:      "gpt-test",
			Provider:   "openai",
			SessionKey: "session_search_oldest",
			Status:     "active",
		},
		{
			BaseModel:  model.BaseModel{UpdatedAt: now.Add(-time.Hour)},
			UserID:     1,
			PublicID:   "conv_search_middle",
			Title:      "Needle middle",
			LabelsJSON: "[]",
			Model:      "gpt-test",
			Provider:   "openai",
			SessionKey: "session_search_middle",
			Status:     "archived",
		},
		{
			BaseModel:  model.BaseModel{UpdatedAt: now},
			UserID:     1,
			PublicID:   "conv_search_latest",
			Title:      "Needle latest",
			LabelsJSON: "[]",
			Model:      "gpt-test",
			Provider:   "openai",
			SessionKey: "session_search_latest",
			Status:     "active",
		},
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatalf("create conversations: %v", err)
	}

	results, err := repo.ListConversationsForSearch(ctx, 1, 1, 2, "needle")
	if err != nil {
		t.Fatalf("ListConversationsForSearch() error = %v", err)
	}
	if len(results) != 2 || results[0].PublicID != "conv_search_middle" || results[1].PublicID != "conv_search_oldest" {
		t.Fatalf("results = %#v, want middle and oldest conversations", results)
	}
}

func TestListLatestBranchPreviewMessagesReturnsLatestVisibleWindow(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	conversation := model.Conversation{
		UserID:     1,
		PublicID:   "conv_latest_branch_preview",
		Title:      "Latest branch preview",
		LabelsJSON: "[]",
		Model:      "gpt-test",
		Provider:   "openai",
		SessionKey: "session_latest_branch_preview",
		Status:     "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	createMessage := func(publicID string, role string, parentID *uint) model.Message {
		t.Helper()
		item := model.Message{
			ConversationID:  conversation.ID,
			UserID:          1,
			PublicID:        publicID,
			ParentMessageID: parentID,
			Role:            role,
			ContentType:     "text",
			Content:         publicID + " content",
			BranchReason:    "default",
			Status:          "success",
		}
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create message %q: %v", publicID, err)
		}
		return item
	}

	root := createMessage("msg_root", "user", nil)
	rootID := root.ID
	createMessage("msg_old_branch", "assistant", &rootID)

	latestBranch := createMessage("msg_latest_branch", "assistant", &rootID)
	latestVisibleIDs := []string{root.PublicID, latestBranch.PublicID}
	parentID := latestBranch.ID
	for i := 1; i <= 12; i++ {
		role := "user"
		if i%2 == 0 {
			role = "assistant"
		}
		item := createMessage(fmt.Sprintf("msg_latest_%02d", i), role, &parentID)
		latestVisibleIDs = append(latestVisibleIDs, item.PublicID)
		parentID = item.ID
	}
	createMessage("msg_latest_tool", "tool", &parentID)

	items, err := repo.ListLatestBranchPreviewMessages(ctx, conversation.ID, 100, 10)
	if err != nil {
		t.Fatalf("ListLatestBranchPreviewMessages() error = %v", err)
	}
	if len(items) != 10 {
		t.Fatalf("len(items) = %d, want 10", len(items))
	}

	wantPublicIDs := latestVisibleIDs[len(latestVisibleIDs)-10:]
	for i, item := range items {
		if item.PublicID != wantPublicIDs[i] {
			t.Fatalf("items[%d].PublicID = %q, want %q", i, item.PublicID, wantPublicIDs[i])
		}
		if item.Role != "user" && item.Role != "assistant" {
			t.Fatalf("items[%d].Role = %q, want visible role", i, item.Role)
		}
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
	if err := db.AutoMigrate(&model.Conversation{}, &model.ConversationProject{}, &model.ConversationProjectMCPTool{}, &model.ConversationProjectSkill{}, &model.ConversationShare{}, &model.Message{}, &model.Attachment{}, &model.FileObject{}, &model.ConversationRun{}, &model.ChatRunEvent{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	return db
}
