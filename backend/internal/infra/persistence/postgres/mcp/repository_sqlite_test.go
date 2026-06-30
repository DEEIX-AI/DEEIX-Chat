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
	}); err != nil {
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
	}); err != nil {
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

func TestReorderServersWithToolsSQLiteRejectsForeignTool(t *testing.T) {
	db := openMCPSQLiteTestDB(t)
	ctx := context.Background()
	repo := NewRepo(db)

	serverA := createMCPServer(t, db, "server-a")
	serverB := createMCPServer(t, db, "server-b")
	if err := repo.ReplaceServerTools(ctx, serverA.ID, []domainmcp.Tool{
		{Name: "tool_a", DisplayName: "Tool A", InputSchemaJSON: "{}", Status: "active"},
	}); err != nil {
		t.Fatalf("replace server a tools: %v", err)
	}
	if err := repo.ReplaceServerTools(ctx, serverB.ID, []domainmcp.Tool{
		{Name: "tool_b", DisplayName: "Tool B", InputSchemaJSON: "{}", Status: "active"},
	}); err != nil {
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
	}); err != nil {
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
	}); err != nil {
		t.Fatalf("replace server a tools: %v", err)
	}
	if err := repo.ReplaceServerTools(ctx, serverB.ID, []domainmcp.Tool{
		{Name: "tool_b", DisplayName: "Tool B", InputSchemaJSON: "{}", Status: "active"},
	}); err != nil {
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
	if err := db.AutoMigrate(&model.MCPServer{}, &model.MCPTool{}); err != nil {
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
