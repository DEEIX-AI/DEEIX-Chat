package mcp

import (
	"context"
	"errors"
	"reflect"
	"testing"

	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestReorderServersWithToolsSQLitePersistsToolOrder(t *testing.T) {
	db := openMCPSQLiteTestDB(t)
	ctx := context.Background()
	repo := NewRepo(db)

	server := createMCPServer(t, db, "server-a")
	if err := repo.ReplaceServerTools(ctx, server.ID, []domainmcp.Tool{
		{Name: "tool_a", DisplayName: "Tool A", InputSchemaJSON: "{}", Status: "active"},
		{Name: "tool_b", DisplayName: "Tool B", InputSchemaJSON: "{}", Status: "active"},
	}, false); err != nil {
		t.Fatalf("replace tools: %v", err)
	}
	initial, err := repo.ListTools(ctx, server.ID, false)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	assertToolNames(t, initial, []string{"tool_a", "tool_b"})

	reorderedGroups, err := repo.ReorderServersWithTools(ctx, []repository.ReorderMCPServerInput{
		{ServerID: server.ID, ToolIDs: []uint{initial[1].ID, initial[0].ID}},
	})
	if err != nil {
		t.Fatalf("reorder tools: %v", err)
	}
	reordered := reorderedGroups[0].Tools
	assertToolNames(t, reordered, []string{"tool_b", "tool_a"})
	if reordered[0].SortOrder != 100 || reordered[1].SortOrder != 200 {
		t.Fatalf("expected normalized sort order, got %#v", reordered)
	}

	if err := repo.ReplaceServerTools(ctx, server.ID, []domainmcp.Tool{
		{Name: "tool_a", DisplayName: "Tool A", InputSchemaJSON: `{"type":"object"}`, Status: "active"},
		{Name: "tool_b", DisplayName: "Tool B", InputSchemaJSON: "{}", Status: "active"},
		{Name: "tool_c", DisplayName: "Tool C", InputSchemaJSON: "{}", Status: "active"},
	}, false); err != nil {
		t.Fatalf("replace tools after reorder: %v", err)
	}
	afterSync, err := repo.ListTools(ctx, server.ID, false)
	if err != nil {
		t.Fatalf("list tools after sync: %v", err)
	}
	assertToolNames(t, afterSync, []string{"tool_b", "tool_a", "tool_c"})
	if afterSync[2].SortOrder <= afterSync[1].SortOrder {
		t.Fatalf("expected newly discovered tool to be appended, got %#v", afterSync)
	}
}

func TestReplaceServerToolsRefreshesRemoteMetadataAndPreservesCustomizedMetadata(t *testing.T) {
	db := openMCPSQLiteTestDB(t)
	ctx := context.Background()
	repo := NewRepo(db)
	server := createMCPServer(t, db, "server-metadata")

	if err := repo.ReplaceServerTools(ctx, server.ID, []domainmcp.Tool{
		{
			Name:            "tool_a",
			DisplayName:     "Old title",
			Description:     "Old description",
			InputSchemaJSON: `{"type":"object","required":["old"]}`,
			Status:          "active",
		},
	}, false); err != nil {
		t.Fatalf("replace initial tools: %v", err)
	}
	initial, err := repo.ListTools(ctx, server.ID, false)
	if err != nil {
		t.Fatalf("list initial tools: %v", err)
	}
	if len(initial) != 1 {
		t.Fatalf("initial tools = %#v, want one tool", initial)
	}
	initialTool := initial[0]
	if initialTool.AttachmentInputMode != domainmcp.AttachmentInputModeNone ||
		initialTool.AttachmentArgument != "" ||
		initialTool.AttachmentEncoding != "" ||
		initialTool.AttachmentPromptArgument != "" {
		t.Fatalf("new tool has unexpected attachment processor defaults: %#v", initialTool)
	}
	inactive := "inactive"
	attachmentMode := domainmcp.AttachmentInputModeImage
	attachmentArgument := "image"
	attachmentEncoding := domainmcp.AttachmentEncodingDataURL
	promptArgument := "prompt"
	if _, err = repo.UpdateTool(ctx, initialTool.ID, repository.UpdateMCPToolInput{
		Status:                   &inactive,
		AttachmentInputMode:      &attachmentMode,
		AttachmentArgument:       &attachmentArgument,
		AttachmentEncoding:       &attachmentEncoding,
		AttachmentPromptArgument: &promptArgument,
	}); err != nil {
		t.Fatalf("disable tool: %v", err)
	}

	if err = repo.ReplaceServerTools(ctx, server.ID, []domainmcp.Tool{
		{
			Name:                     "tool_a",
			DisplayName:              "New title",
			Description:              "New description",
			InputSchemaJSON:          `{"type":"object","properties":{"image":{"type":"string"},"prompt":{"type":"string"}},"required":["image"]}`,
			AttachmentInputMode:      attachmentMode,
			AttachmentArgument:       attachmentArgument,
			AttachmentEncoding:       attachmentEncoding,
			AttachmentPromptArgument: promptArgument,
			Status:                   "active",
		},
	}, false); err != nil {
		t.Fatalf("replace updated tools: %v", err)
	}

	updated, err := repo.ListTools(ctx, server.ID, false)
	if err != nil {
		t.Fatalf("list updated tools: %v", err)
	}
	if len(updated) != 1 {
		t.Fatalf("updated tools = %#v, want one tool", updated)
	}
	updatedTool := updated[0]
	if updatedTool.ID != initialTool.ID {
		t.Fatalf("tool id = %d, want preserved id %d", updatedTool.ID, initialTool.ID)
	}
	if updatedTool.DisplayName != "New title" || updatedTool.Description != "New description" {
		t.Fatalf("tool metadata = %q/%q, want refreshed values", updatedTool.DisplayName, updatedTool.Description)
	}
	if updatedTool.InputSchemaJSON != `{"type":"object","properties":{"image":{"type":"string"},"prompt":{"type":"string"}},"required":["image"]}` {
		t.Fatalf("tool schema = %s, want refreshed schema", updatedTool.InputSchemaJSON)
	}
	if updatedTool.Status != "inactive" || updatedTool.SortOrder != initialTool.SortOrder {
		t.Fatalf("local controls = %s/%d, want inactive/%d", updatedTool.Status, updatedTool.SortOrder, initialTool.SortOrder)
	}
	if updatedTool.AttachmentInputMode != attachmentMode ||
		updatedTool.AttachmentArgument != attachmentArgument ||
		updatedTool.AttachmentEncoding != attachmentEncoding ||
		updatedTool.AttachmentPromptArgument != promptArgument {
		t.Fatalf("attachment processor configuration was not preserved: %#v", updatedTool)
	}
	storedTool := loadStoredMCPTool(t, db, updatedTool.ID)
	if storedTool.MetadataCustomized == nil || *storedTool.MetadataCustomized {
		t.Fatal("remote metadata refresh unexpectedly marked tool as customized")
	}
	unchangedTitle := updatedTool.DisplayName
	unchangedDescription := updatedTool.Description
	if _, err = repo.UpdateTool(ctx, updatedTool.ID, repository.UpdateMCPToolInput{
		DisplayName: &unchangedTitle,
		Description: &unchangedDescription,
	}); err != nil {
		t.Fatalf("save unchanged metadata: %v", err)
	}
	storedTool = loadStoredMCPTool(t, db, updatedTool.ID)
	if storedTool.MetadataCustomized == nil || *storedTool.MetadataCustomized {
		t.Fatal("saving unchanged metadata marked tool as customized")
	}

	customTitle := "Custom title"
	customDescription := "Custom description"
	if _, err = repo.UpdateTool(ctx, initialTool.ID, repository.UpdateMCPToolInput{
		DisplayName: &customTitle,
		Description: &customDescription,
	}); err != nil {
		t.Fatalf("customize tool metadata: %v", err)
	}
	if err = repo.ReplaceServerTools(ctx, server.ID, []domainmcp.Tool{
		{
			Name:            "tool_a",
			DisplayName:     "Latest remote title",
			Description:     "Latest remote description",
			InputSchemaJSON: `{"type":"object","required":["latest"]}`,
			Status:          "active",
		},
	}, false); err != nil {
		t.Fatalf("replace tools after customization: %v", err)
	}

	afterCustomization, err := repo.ListTools(ctx, server.ID, false)
	if err != nil {
		t.Fatalf("list tools after customization: %v", err)
	}
	if len(afterCustomization) != 1 {
		t.Fatalf("tools after customization = %#v, want one tool", afterCustomization)
	}
	customizedTool := afterCustomization[0]
	if customizedTool.DisplayName != customTitle || customizedTool.Description != customDescription {
		t.Fatalf("effective metadata = %q/%q, want custom values", customizedTool.DisplayName, customizedTool.Description)
	}
	if customizedTool.InputSchemaJSON != `{"type":"object","required":["latest"]}` {
		t.Fatalf("tool schema = %s, want latest remote schema", customizedTool.InputSchemaJSON)
	}
	if customizedTool.Status != "inactive" || customizedTool.SortOrder != initialTool.SortOrder {
		t.Fatalf("local controls after customization = %s/%d, want inactive/%d", customizedTool.Status, customizedTool.SortOrder, initialTool.SortOrder)
	}
	if customizedTool.AttachmentInputMode != domainmcp.AttachmentInputModeNone ||
		customizedTool.AttachmentArgument != "" ||
		customizedTool.AttachmentEncoding != "" ||
		customizedTool.AttachmentPromptArgument != "" {
		t.Fatalf("disabled attachment processor configuration was not cleared: %#v", customizedTool)
	}
	storedTool = loadStoredMCPTool(t, db, customizedTool.ID)
	if storedTool.MetadataCustomized == nil || !*storedTool.MetadataCustomized {
		t.Fatal("administrator metadata update was not marked as customized")
	}
	servers, err := repo.ListServers(ctx)
	if err != nil {
		t.Fatalf("list servers after customization: %v", err)
	}
	if len(servers) != 1 || !servers[0].RequiresToolMetadataSyncConfirmation {
		t.Fatalf("server customization flag = %#v, want true", servers)
	}
	serverAfterCustomization, err := repo.GetServer(ctx, server.ID)
	if err != nil {
		t.Fatalf("get server after customization: %v", err)
	}
	if !serverAfterCustomization.RequiresToolMetadataSyncConfirmation {
		t.Fatal("single server response did not require metadata sync confirmation")
	}

	if err = repo.ReplaceServerTools(ctx, server.ID, []domainmcp.Tool{
		{
			Name:            "tool_a",
			DisplayName:     "Latest remote title",
			Description:     "Latest remote description",
			InputSchemaJSON: `{"type":"object","required":["overwritten"]}`,
			Status:          "active",
		},
	}, true); err != nil {
		t.Fatalf("replace tools with overwrite: %v", err)
	}
	afterOverwrite, err := repo.ListTools(ctx, server.ID, false)
	if err != nil {
		t.Fatalf("list tools after overwrite: %v", err)
	}
	if len(afterOverwrite) != 1 {
		t.Fatalf("tools after overwrite = %#v, want one tool", afterOverwrite)
	}
	overwrittenTool := afterOverwrite[0]
	if overwrittenTool.DisplayName != "Latest remote title" || overwrittenTool.Description != "Latest remote description" {
		t.Fatalf("overwritten metadata = %q/%q, want latest remote values", overwrittenTool.DisplayName, overwrittenTool.Description)
	}
	if overwrittenTool.InputSchemaJSON != `{"type":"object","required":["overwritten"]}` {
		t.Fatalf("tool schema after overwrite = %s", overwrittenTool.InputSchemaJSON)
	}
	storedTool = loadStoredMCPTool(t, db, overwrittenTool.ID)
	if storedTool.MetadataCustomized == nil || *storedTool.MetadataCustomized {
		t.Fatal("overwritten remote metadata remained marked as customized")
	}
	if overwrittenTool.Status != "inactive" || overwrittenTool.SortOrder != initialTool.SortOrder {
		t.Fatalf("local controls after overwrite = %s/%d, want inactive/%d", overwrittenTool.Status, overwrittenTool.SortOrder, initialTool.SortOrder)
	}
	servers, err = repo.ListServers(ctx)
	if err != nil {
		t.Fatalf("list servers after overwrite: %v", err)
	}
	if len(servers) != 1 || servers[0].RequiresToolMetadataSyncConfirmation {
		t.Fatalf("server customization flag = %#v, want false", servers)
	}
	serverAfterOverwrite, err := repo.GetServer(ctx, server.ID)
	if err != nil {
		t.Fatalf("get server after overwrite: %v", err)
	}
	if serverAfterOverwrite.RequiresToolMetadataSyncConfirmation {
		t.Fatal("single server response still required metadata sync confirmation after overwrite")
	}
}

func TestReplaceServerToolsPreservesLegacyMetadataAfterConfirmation(t *testing.T) {
	db := openMCPSQLiteTestDB(t)
	ctx := context.Background()
	repo := NewRepo(db)
	server := createMCPServer(t, db, "server-legacy-metadata")
	if err := repo.ReplaceServerTools(ctx, server.ID, []domainmcp.Tool{
		{
			Name:            "tool_a",
			DisplayName:     "Remote title",
			Description:     "Remote description",
			InputSchemaJSON: "{}",
			Status:          "active",
		},
		{
			Name:            "tool_b",
			DisplayName:     "Stable remote title",
			Description:     "Stable remote description",
			InputSchemaJSON: "{}",
			Status:          "active",
		},
	}, false); err != nil {
		t.Fatalf("replace initial tools: %v", err)
	}
	tools, err := repo.ListTools(ctx, server.ID, false)
	if err != nil || len(tools) != 2 {
		t.Fatalf("list initial tools = %#v, error = %v", tools, err)
	}
	if err = db.Model(&model.MCPTool{}).
		Where("server_id = ?", server.ID).
		UpdateColumn("metadata_customized", nil).Error; err != nil {
		t.Fatalf("simulate legacy metadata state: %v", err)
	}
	if err = db.Model(&model.MCPTool{}).
		Where("server_id = ? AND name = ?", server.ID, "tool_a").
		UpdateColumns(map[string]any{
			"display_name": "Existing title",
			"description":  "Existing description",
		}).Error; err != nil {
		t.Fatalf("simulate legacy metadata: %v", err)
	}
	servers, err := repo.ListServers(ctx)
	if err != nil || len(servers) != 1 || !servers[0].RequiresToolMetadataSyncConfirmation {
		t.Fatalf("legacy confirmation state = %#v, error = %v", servers, err)
	}

	if err = repo.ReplaceServerTools(ctx, server.ID, []domainmcp.Tool{
		{
			Name:            "tool_a",
			DisplayName:     "Latest remote title",
			Description:     "Latest remote description",
			InputSchemaJSON: `{"type":"object"}`,
			Status:          "active",
		},
		{
			Name:            "tool_b",
			DisplayName:     "Stable remote title",
			Description:     "Stable remote description",
			InputSchemaJSON: `{"type":"object"}`,
			Status:          "active",
		},
	}, false); err != nil {
		t.Fatalf("preserve legacy metadata: %v", err)
	}
	preserved, err := repo.ListTools(ctx, server.ID, false)
	if err != nil || len(preserved) != 2 {
		t.Fatalf("list preserved tools = %#v, error = %v", preserved, err)
	}
	toolsByName := map[string]domainmcp.Tool{}
	for _, tool := range preserved {
		toolsByName[tool.Name] = tool
	}
	if toolsByName["tool_a"].DisplayName != "Existing title" || toolsByName["tool_a"].Description != "Existing description" {
		t.Fatalf("preserved metadata = %q/%q", toolsByName["tool_a"].DisplayName, toolsByName["tool_a"].Description)
	}
	customized := loadStoredMCPTool(t, db, toolsByName["tool_a"].ID)
	if customized.MetadataCustomized == nil || !*customized.MetadataCustomized {
		t.Fatalf("changed legacy metadata state = %v, want true", customized.MetadataCustomized)
	}
	remoteManaged := loadStoredMCPTool(t, db, toolsByName["tool_b"].ID)
	if remoteManaged.MetadataCustomized == nil || *remoteManaged.MetadataCustomized {
		t.Fatalf("unchanged legacy metadata state = %v, want false", remoteManaged.MetadataCustomized)
	}
}

func TestRemovingMCPToolsCleansConversationProjectAssociations(t *testing.T) {
	db := openMCPSQLiteTestDB(t)
	ctx := context.Background()
	repo := NewRepo(db)
	server := createMCPServer(t, db, "server-cascade")
	if err := repo.ReplaceServerTools(ctx, server.ID, []domainmcp.Tool{
		{Name: "tool_a", DisplayName: "Tool A", InputSchemaJSON: "{}", Status: "active"},
		{Name: "tool_b", DisplayName: "Tool B", InputSchemaJSON: "{}", Status: "active"},
	}, false); err != nil {
		t.Fatalf("replace tools: %v", err)
	}
	tools, err := repo.ListTools(ctx, server.ID, false)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	for _, tool := range tools {
		if err = db.Create(&model.ConversationProjectMCPTool{ProjectID: 9, ToolID: tool.ID}).Error; err != nil {
			t.Fatalf("create project MCP association: %v", err)
		}
	}

	if err = repo.ReplaceServerTools(ctx, server.ID, []domainmcp.Tool{
		{Name: "tool_b", DisplayName: "Tool B", InputSchemaJSON: "{}", Status: "active"},
	}, false); err != nil {
		t.Fatalf("replace tools with removal: %v", err)
	}
	var associations []model.ConversationProjectMCPTool
	if err = db.Order("tool_id ASC").Find(&associations).Error; err != nil {
		t.Fatalf("list project MCP associations: %v", err)
	}
	if len(associations) != 1 || associations[0].ToolID != tools[1].ID {
		t.Fatalf("associations after tool removal = %#v", associations)
	}

	if err = repo.DeleteServer(ctx, server.ID); err != nil {
		t.Fatalf("DeleteServer() error = %v", err)
	}
	var associationCount int64
	if err = db.Model(&model.ConversationProjectMCPTool{}).Count(&associationCount).Error; err != nil {
		t.Fatalf("count project MCP associations: %v", err)
	}
	if associationCount != 0 {
		t.Fatalf("project MCP association count = %d, want 0", associationCount)
	}
}

func TestReorderServersWithToolsSQLiteRejectsForeignTool(t *testing.T) {
	db := openMCPSQLiteTestDB(t)
	ctx := context.Background()
	repo := NewRepo(db)

	serverA := createMCPServer(t, db, "server-a")
	serverB := createMCPServer(t, db, "server-b")
	if err := repo.ReplaceServerTools(ctx, serverA.ID, []domainmcp.Tool{
		{Name: "tool_a", DisplayName: "Tool A", InputSchemaJSON: "{}", Status: "active"},
	}, false); err != nil {
		t.Fatalf("replace server a tools: %v", err)
	}
	if err := repo.ReplaceServerTools(ctx, serverB.ID, []domainmcp.Tool{
		{Name: "tool_b", DisplayName: "Tool B", InputSchemaJSON: "{}", Status: "active"},
	}, false); err != nil {
		t.Fatalf("replace server b tools: %v", err)
	}
	serverBTools, err := repo.ListTools(ctx, serverB.ID, false)
	if err != nil {
		t.Fatalf("list server b tools: %v", err)
	}
	if _, err = repo.ReorderServersWithTools(ctx, []repository.ReorderMCPServerInput{
		{ServerID: serverA.ID, ToolIDs: []uint{serverBTools[0].ID}},
		{ServerID: serverB.ID, ToolIDs: []uint{}},
	}); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected foreign tool reorder to fail with record not found, got %v", err)
	}
}

func TestReorderServersWithToolsSQLiteRejectsPartialToolOrder(t *testing.T) {
	db := openMCPSQLiteTestDB(t)
	ctx := context.Background()
	repo := NewRepo(db)

	server := createMCPServer(t, db, "server-a")
	if err := repo.ReplaceServerTools(ctx, server.ID, []domainmcp.Tool{
		{Name: "tool_a", DisplayName: "Tool A", InputSchemaJSON: "{}", Status: "active"},
		{Name: "tool_b", DisplayName: "Tool B", InputSchemaJSON: "{}", Status: "active"},
	}, false); err != nil {
		t.Fatalf("replace tools: %v", err)
	}
	tools, err := repo.ListTools(ctx, server.ID, false)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	if _, err = repo.ReorderServersWithTools(ctx, []repository.ReorderMCPServerInput{
		{ServerID: server.ID, ToolIDs: []uint{tools[0].ID}},
	}); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected partial tool order to fail with record not found, got %v", err)
	}
}

func TestReorderServersWithToolsSQLitePersistsServerOrder(t *testing.T) {
	db := openMCPSQLiteTestDB(t)
	ctx := context.Background()
	repo := NewRepo(db)

	serverA := createMCPServer(t, db, "server-a")
	serverB := createMCPServer(t, db, "server-b")
	if err := repo.ReplaceServerTools(ctx, serverA.ID, []domainmcp.Tool{
		{Name: "tool_a", DisplayName: "Tool A", InputSchemaJSON: "{}", Status: "active"},
	}, false); err != nil {
		t.Fatalf("replace server a tools: %v", err)
	}
	if err := repo.ReplaceServerTools(ctx, serverB.ID, []domainmcp.Tool{
		{Name: "tool_b", DisplayName: "Tool B", InputSchemaJSON: "{}", Status: "active"},
	}, false); err != nil {
		t.Fatalf("replace server b tools: %v", err)
	}
	serverATools, err := repo.ListTools(ctx, serverA.ID, false)
	if err != nil {
		t.Fatalf("list server a tools: %v", err)
	}
	serverBTools, err := repo.ListTools(ctx, serverB.ID, false)
	if err != nil {
		t.Fatalf("list server b tools: %v", err)
	}

	reordered, err := repo.ReorderServersWithTools(ctx, []repository.ReorderMCPServerInput{
		{ServerID: serverB.ID, ToolIDs: []uint{serverBTools[0].ID}},
		{ServerID: serverA.ID, ToolIDs: []uint{serverATools[0].ID}},
	})
	if err != nil {
		t.Fatalf("reorder servers with tools: %v", err)
	}
	if reordered[0].Server.ID != serverB.ID || reordered[1].Server.ID != serverA.ID {
		t.Fatalf("expected server b before server a, got %#v", reordered)
	}
	if reordered[0].Server.SortOrder != 100 || reordered[1].Server.SortOrder != 200 {
		t.Fatalf("expected normalized server sort order, got %#v", reordered)
	}
}

func openMCPSQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.MCPServer{}, &model.MCPTool{}, &model.ConversationProjectMCPTool{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

func createMCPServer(t *testing.T, db *gorm.DB, name string) model.MCPServer {
	t.Helper()
	server := model.MCPServer{Name: name, BaseURL: "https://example.com/mcp", HeadersJSON: "{}", Status: "active"}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("create mcp server: %v", err)
	}
	return server
}

func loadStoredMCPTool(t *testing.T, db *gorm.DB, toolID uint) model.MCPTool {
	t.Helper()
	var tool model.MCPTool
	if err := db.First(&tool, "id = ?", toolID).Error; err != nil {
		t.Fatalf("load stored MCP tool: %v", err)
	}
	return tool
}

func assertToolNames(t *testing.T, tools []domainmcp.Tool, want []string) {
	t.Helper()
	got := make([]string, 0, len(tools))
	for _, tool := range tools {
		got = append(got, tool.Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected tool order %v, got %v", want, got)
	}
}
