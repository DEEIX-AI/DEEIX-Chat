package schema

import (
	"strings"
	"testing"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type legacyMCPTool struct {
	ID              uint `gorm:"primaryKey"`
	ServerID        uint
	Name            string
	DisplayName     string
	Description     string
	InputSchemaJSON string
	Status          string
	SortOrder       int
	UpdatedAt       time.Time
}

func (legacyMCPTool) TableName() string {
	return "mcp_tools"
}

func TestMigrateLeavesLegacyMCPToolMetadataPendingConfirmation(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err = db.AutoMigrate(&legacyMCPTool{}); err != nil {
		t.Fatalf("migrate legacy MCP tool: %v", err)
	}
	updatedAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	legacy := legacyMCPTool{
		ServerID:        1,
		Name:            "tool_a",
		DisplayName:     "Existing title",
		Description:     "Existing description",
		InputSchemaJSON: "{}",
		Status:          "active",
		UpdatedAt:       updatedAt,
	}
	if err = db.Create(&legacy).Error; err != nil {
		t.Fatalf("create legacy MCP tool: %v", err)
	}

	if err = Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	var migrated model.MCPTool
	if err = db.First(&migrated, legacy.ID).Error; err != nil {
		t.Fatalf("load migrated MCP tool: %v", err)
	}
	if migrated.MetadataCustomized != nil {
		t.Fatalf("legacy metadata state = %v, want pending confirmation", *migrated.MetadataCustomized)
	}
	if !migrated.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("legacy updated_at = %s, want %s", migrated.UpdatedAt, updatedAt)
	}
}

func TestBackfillContextArtifactMessageIDsUsesAssistantRunOwner(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err = db.AutoMigrate(&model.Message{}, &model.ChatContextRecord{}); err != nil {
		t.Fatalf("migrate context artifacts: %v", err)
	}
	userMessage := model.Message{
		ConversationID: 7,
		UserID:         11,
		PublicID:       "msg_user",
		RunID:          "run_tool",
		Role:           "user",
		Status:         "success",
	}
	if err = db.Create(&userMessage).Error; err != nil {
		t.Fatalf("create user message: %v", err)
	}
	assistantMessage := model.Message{
		ConversationID:  7,
		UserID:          11,
		PublicID:        "msg_assistant",
		ParentMessageID: &userMessage.ID,
		RunID:           "run_tool",
		Role:            "assistant",
		Status:          "success",
	}
	if err = db.Create(&assistantMessage).Error; err != nil {
		t.Fatalf("create assistant message: %v", err)
	}
	ambiguousUser := model.Message{
		ConversationID: 8,
		UserID:         13,
		PublicID:       "msg_ambiguous_user",
		RunID:          "run_ambiguous",
		Role:           "user",
		Status:         "success",
	}
	if err = db.Create(&ambiguousUser).Error; err != nil {
		t.Fatalf("create ambiguous user message: %v", err)
	}
	ambiguousAssistants := []model.Message{
		{
			ConversationID: 8, UserID: 13, PublicID: "msg_ambiguous_assistant_1",
			ParentMessageID: &ambiguousUser.ID, RunID: "run_ambiguous", Role: "assistant", Status: "success",
		},
		{
			ConversationID: 8, UserID: 13, PublicID: "msg_ambiguous_assistant_2",
			ParentMessageID: &ambiguousUser.ID, RunID: "run_ambiguous", Role: "assistant", Status: "success",
		},
	}
	if err = db.Create(&ambiguousAssistants).Error; err != nil {
		t.Fatalf("create ambiguous assistant messages: %v", err)
	}
	artifacts := []model.ChatContextRecord{
		{
			RecordType:     "artifact",
			ConversationID: 7,
			MessageID:      userMessage.ID,
			UserID:         11,
			RunID:          "run_tool",
			Kind:           "tool_result",
			SourceType:     "tool_call",
			SourceID:       "call_1",
			Content:        "tool output",
		},
		{
			RecordType:     "artifact",
			ConversationID: 7,
			MessageID:      userMessage.ID,
			UserID:         11,
			RunID:          "run_tool",
			Kind:           "file_rag_chunk",
			SourceType:     "file_chunk",
			SourceID:       "file_1:0",
			Content:        "file output",
		},
		{
			RecordType:     "artifact",
			ConversationID: 7,
			MessageID:      99,
			UserID:         12,
			RunID:          "run_tool",
			Kind:           "tool_result",
			SourceType:     "tool_call",
			SourceID:       "foreign_user_call",
			Content:        "foreign user output",
		},
		{
			RecordType:     "artifact",
			ConversationID: 8,
			MessageID:      ambiguousUser.ID,
			UserID:         13,
			RunID:          "run_ambiguous",
			Kind:           "tool_result",
			SourceType:     "tool_call",
			SourceID:       "ambiguous_call",
			Content:        "ambiguous output",
		},
		{
			RecordType:     "snapshot",
			ConversationID: 7,
			MessageID:      userMessage.ID,
			UserID:         11,
			RunID:          "run_tool",
			SummaryText:    "snapshot remains anchored by its own schema",
		},
	}
	if err = db.Create(&artifacts).Error; err != nil {
		t.Fatalf("create artifacts: %v", err)
	}

	if err = backfillContextArtifactMessageIDs(db); err != nil {
		t.Fatalf("backfillContextArtifactMessageIDs() error = %v", err)
	}
	if err = backfillContextArtifactMessageIDs(db); err != nil {
		t.Fatalf("backfillContextArtifactMessageIDs() second error = %v", err)
	}

	var toolArtifact model.ChatContextRecord
	if err = db.Where("source_id = ?", "call_1").First(&toolArtifact).Error; err != nil {
		t.Fatalf("load tool artifact: %v", err)
	}
	if toolArtifact.MessageID != assistantMessage.ID {
		t.Fatalf("tool artifact message id = %d, want %d", toolArtifact.MessageID, assistantMessage.ID)
	}
	var fileArtifact model.ChatContextRecord
	if err = db.Where("source_type = ?", "file_chunk").First(&fileArtifact).Error; err != nil {
		t.Fatalf("load file artifact: %v", err)
	}
	if fileArtifact.MessageID != assistantMessage.ID {
		t.Fatalf("file artifact message id = %d, want %d", fileArtifact.MessageID, assistantMessage.ID)
	}
	var foreignUserArtifact model.ChatContextRecord
	if err = db.Where("source_id = ?", "foreign_user_call").First(&foreignUserArtifact).Error; err != nil {
		t.Fatalf("load foreign user artifact: %v", err)
	}
	if foreignUserArtifact.MessageID != 99 {
		t.Fatalf("foreign user artifact message id = %d, want unchanged 99", foreignUserArtifact.MessageID)
	}
	var ambiguousArtifact model.ChatContextRecord
	if err = db.Where("source_id = ?", "ambiguous_call").First(&ambiguousArtifact).Error; err != nil {
		t.Fatalf("load ambiguous artifact: %v", err)
	}
	if ambiguousArtifact.MessageID != ambiguousUser.ID {
		t.Fatalf("ambiguous artifact message id = %d, want unchanged %d", ambiguousArtifact.MessageID, ambiguousUser.ID)
	}
	var snapshot model.ChatContextRecord
	if err = db.Where("record_type = ?", "snapshot").First(&snapshot).Error; err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snapshot.MessageID != userMessage.ID {
		t.Fatalf("snapshot message id = %d, want unchanged %d", snapshot.MessageID, userMessage.ID)
	}
}

func TestSeedBillingCatalogBindsDefaultPermissionGroup(t *testing.T) {
	db := openSchemaTestDB(t)
	if err := SeedPermissionGroups(db); err != nil {
		t.Fatalf("SeedPermissionGroups() error = %v", err)
	}
	if err := SeedBillingCatalog(db); err != nil {
		t.Fatalf("SeedBillingCatalog() error = %v", err)
	}

	var plans []model.BillingPlan
	if err := db.Order("code ASC").Find(&plans).Error; err != nil {
		t.Fatalf("list plans: %v", err)
	}
	if len(plans) == 0 {
		t.Fatal("expected seeded billing plans")
	}
	for _, plan := range plans {
		if plan.PermissionGroupID == nil {
			t.Fatalf("plan %q PermissionGroupID is nil", plan.Code)
		}
	}
}

func TestSeedBillingCatalogBackfillsExistingPlans(t *testing.T) {
	db := openSchemaTestDB(t)
	if err := SeedPermissionGroups(db); err != nil {
		t.Fatalf("SeedPermissionGroups() error = %v", err)
	}
	if err := db.Create(&model.BillingPlan{
		Code:     "pro",
		Name:     "Pro",
		IsActive: true,
	}).Error; err != nil {
		t.Fatalf("seed plan: %v", err)
	}

	if err := SeedBillingCatalog(db); err != nil {
		t.Fatalf("SeedBillingCatalog() error = %v", err)
	}

	var plan model.BillingPlan
	if err := db.Where("code = ?", "pro").First(&plan).Error; err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if plan.PermissionGroupID == nil {
		t.Fatal("expected existing plan to be bound to default permission group")
	}
}

func TestSeedPermissionGroupsClearsDefaultGroupUserAccess(t *testing.T) {
	db := openSchemaTestDB(t)
	defaultGroup := model.PermissionGroup{Name: "Default", IsDefault: true}
	manualGroup := model.PermissionGroup{Name: "Manual"}
	if err := db.Create(&[]model.PermissionGroup{defaultGroup, manualGroup}).Error; err != nil {
		t.Fatalf("create groups: %v", err)
	}
	var groups []model.PermissionGroup
	if err := db.Order("id ASC").Find(&groups).Error; err != nil {
		t.Fatalf("list groups: %v", err)
	}
	if err := db.Create(&[]model.PermissionGroupUserAccess{
		{GroupID: groups[0].ID, UserID: 1},
		{GroupID: groups[1].ID, UserID: 1},
	}).Error; err != nil {
		t.Fatalf("create group users: %v", err)
	}

	if err := SeedPermissionGroups(db); err != nil {
		t.Fatalf("SeedPermissionGroups() error = %v", err)
	}

	var defaultRows int64
	if err := db.Model(&model.PermissionGroupUserAccess{}).
		Where("group_id = ?", groups[0].ID).
		Count(&defaultRows).Error; err != nil {
		t.Fatalf("count default rows: %v", err)
	}
	if defaultRows != 0 {
		t.Fatalf("expected default group user access to be cleared, got %d", defaultRows)
	}
	var manualRows int64
	if err := db.Model(&model.PermissionGroupUserAccess{}).
		Where("group_id = ?", groups[1].ID).
		Count(&manualRows).Error; err != nil {
		t.Fatalf("count manual rows: %v", err)
	}
	if manualRows != 1 {
		t.Fatalf("expected manual group user access to remain, got %d", manualRows)
	}
}

func TestSeedPermissionGroupsInitializesDefaultAllModelsRule(t *testing.T) {
	db := openSchemaTestDB(t)
	if err := SeedPermissionGroups(db); err != nil {
		t.Fatalf("SeedPermissionGroups() error = %v", err)
	}

	var defaultGroup model.PermissionGroup
	if err := db.Where("is_default = ?", true).First(&defaultGroup).Error; err != nil {
		t.Fatalf("get default group: %v", err)
	}
	var rule model.PermissionGroupModelRule
	if err := db.Where("group_id = ? AND rule_type = ?", defaultGroup.ID, domainchannel.PermissionGroupModelRuleAll).
		First(&rule).Error; err != nil {
		t.Fatalf("expected default all-model rule: %v", err)
	}
}

func TestSeedPermissionGroupsDoesNotRecreateDefaultAllRuleAfterAccessConfigured(t *testing.T) {
	db := openSchemaTestDB(t)
	defaultGroup := model.PermissionGroup{Name: "Default", IsDefault: true}
	if err := db.Create(&defaultGroup).Error; err != nil {
		t.Fatalf("create default group: %v", err)
	}
	manualGroup := model.PermissionGroup{Name: "Manual"}
	if err := db.Create(&manualGroup).Error; err != nil {
		t.Fatalf("create manual group: %v", err)
	}
	if err := db.Create(&model.PermissionGroupModelRule{
		GroupID:  manualGroup.ID,
		RuleType: domainchannel.PermissionGroupModelRuleVendor,
		Value:    "openai",
	}).Error; err != nil {
		t.Fatalf("create existing rule: %v", err)
	}

	if err := SeedPermissionGroups(db); err != nil {
		t.Fatalf("SeedPermissionGroups() error = %v", err)
	}

	var count int64
	if err := db.Model(&model.PermissionGroupModelRule{}).
		Where("group_id = ? AND rule_type = ?", defaultGroup.ID, domainchannel.PermissionGroupModelRuleAll).
		Count(&count).Error; err != nil {
		t.Fatalf("count default all rule: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no default all rule to be recreated after access was configured, got %d", count)
	}
}

func TestSeedModelVendorsPromotesBuiltInsPreservesEditsAndBackfillsExistingKeys(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err = db.AutoMigrate(&model.LLMModelVendor{}, &model.LLMPlatformModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	customized := model.LLMModelVendor{
		Key: "openai", Name: "OpenAI Custom", Icon: "custom-icon", BuiltIn: false, SortOrder: 99,
	}
	if err = db.Create(&customized).Error; err != nil {
		t.Fatalf("create customized vendor: %v", err)
	}
	if err = db.Create(&model.LLMPlatformModel{Name: "acme-chat", Vendor: "acme-ai"}).Error; err != nil {
		t.Fatalf("create existing model: %v", err)
	}
	if err = db.Create(&model.LLMPlatformModel{Name: "legacy-openai", Vendor: "OpenAI"}).Error; err != nil {
		t.Fatalf("create legacy model: %v", err)
	}

	if err = SeedModelVendors(db); err != nil {
		t.Fatalf("SeedModelVendors() error = %v", err)
	}

	var openAI model.LLMModelVendor
	if err = db.Where("key = ?", "openai").First(&openAI).Error; err != nil {
		t.Fatalf("load OpenAI vendor: %v", err)
	}
	if openAI.Name != customized.Name || openAI.Icon != customized.Icon || openAI.SortOrder != customized.SortOrder {
		t.Fatalf("expected customized vendor to remain unchanged, got %#v", openAI)
	}
	if !openAI.BuiltIn {
		t.Fatalf("expected matching custom vendor to be promoted to built-in, got %#v", openAI)
	}
	var custom model.LLMModelVendor
	if err = db.Where("key = ?", "acme-ai").First(&custom).Error; err != nil {
		t.Fatalf("load backfilled vendor: %v", err)
	}
	if custom.Name != "acme-ai" || custom.BuiltIn {
		t.Fatalf("unexpected backfilled vendor: %#v", custom)
	}
	var legacy model.LLMModelVendor
	if err = db.Where("key = ?", "OpenAI").First(&legacy).Error; err != nil {
		t.Fatalf("load legacy vendor with duplicate display name: %v", err)
	}
	if legacy.Name != "OpenAI" || legacy.BuiltIn {
		t.Fatalf("unexpected legacy vendor: %#v", legacy)
	}
}

func openSchemaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err = db.AutoMigrate(
		&model.PermissionGroup{},
		&model.PermissionGroupUserAccess{},
		&model.PermissionGroupModelAccess{},
		&model.PermissionGroupModelRule{},
		&model.BillingPlan{},
		&model.BillingPrice{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}
