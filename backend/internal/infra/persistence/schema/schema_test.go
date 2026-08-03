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
