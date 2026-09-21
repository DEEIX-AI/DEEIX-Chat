package uicomponent

import (
	"context"
	"testing"

	domainuicomponent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/uicomponent"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/schema"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sqlite connection: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = db.AutoMigrate(&model.UIComponent{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

func TestSeedUIComponentsIsIdempotentAndKeepsAdminEdits(t *testing.T) {
	db := openTestDB(t, "ui_components_seed")
	if err := schema.SeedUIComponents(db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	repo := NewRepo(db)
	items, total, err := repo.ListUIComponents(context.Background(), repository.UIComponentListFilter{Scope: domainuicomponent.ScopeBuiltin}, 0, 50)
	if err != nil || total != int64(len(domainuicomponent.Builtin())) {
		t.Fatalf("expected %d builtin rows, got %d (err=%v)", len(domainuicomponent.Builtin()), total, err)
	}

	// 启停状态保留；目录字段（描述等）每次启动以代码为准。
	disabled := false
	stale := "旧描述"
	if _, err := repo.PatchUIComponent(context.Background(), items[0].ID, repository.UIComponentPatch{Enabled: &disabled, Description: &stale}); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if err := schema.SeedUIComponents(db); err != nil {
		t.Fatalf("reseed: %v", err)
	}
	got, err := repo.GetUIComponent(context.Background(), items[0].ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Enabled {
		t.Fatalf("reseed must keep enabled=false, got %+v", got)
	}
	if got.Description != domainuicomponent.Builtin()[0].Description {
		t.Fatalf("reseed must resync builtin description from code, got %q", got.Description)
	}
	if _, total, _ = repo.ListUIComponents(context.Background(), repository.UIComponentListFilter{}, 0, 50); total != int64(len(domainuicomponent.Builtin())) {
		t.Fatalf("reseed must not duplicate rows, got %d", total)
	}

	// 目录中不存在的内置行（旧版本遗留）在下次播种时移除；平台/用户行不受影响。
	if _, err := repo.CreateUIComponent(context.Background(), &domainuicomponent.Component{Scope: domainuicomponent.ScopeBuiltin, Name: "legacy-widget", Version: 1, RendererKind: domainuicomponent.RendererBuiltin, Enabled: true}); err != nil {
		t.Fatalf("create legacy: %v", err)
	}
	if _, err := repo.CreateUIComponent(context.Background(), &domainuicomponent.Component{Scope: domainuicomponent.ScopePlatform, Name: "custom-widget", Version: 1, RendererKind: domainuicomponent.RendererSandbox, RendererSource: "<div/>", Enabled: true}); err != nil {
		t.Fatalf("create platform: %v", err)
	}
	if err := schema.SeedUIComponents(db); err != nil {
		t.Fatalf("reseed after legacy: %v", err)
	}
	items, _, _ = repo.ListUIComponents(context.Background(), repository.UIComponentListFilter{}, 0, 50)
	for _, item := range items {
		if item.Name == "legacy-widget" {
			t.Fatal("legacy builtin row must be removed by reseed")
		}
	}
	found := false
	for _, item := range items {
		found = found || item.Name == "custom-widget"
	}
	if !found {
		t.Fatal("platform rows must survive reseed")
	}
}

func TestListUIComponentsVisibleUserIDScopesPrivateComponents(t *testing.T) {
	db := openTestDB(t, "ui_components_visible")
	repo := NewRepo(db)
	ctx := context.Background()
	mustCreate := func(scope string, owner uint, name string, enabled bool) {
		t.Helper()
		if _, err := repo.CreateUIComponent(ctx, &domainuicomponent.Component{Scope: scope, OwnerUserID: owner, Name: name, Version: 1, RendererKind: domainuicomponent.RendererSandbox, Enabled: enabled}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	mustCreate(domainuicomponent.ScopeBuiltin, 0, "card-grid", true)
	mustCreate(domainuicomponent.ScopePlatform, 0, "platform-on", true)
	mustCreate(domainuicomponent.ScopePlatform, 0, "platform-off", false)
	mustCreate(domainuicomponent.ScopeUser, 7, "mine", true)
	mustCreate(domainuicomponent.ScopeUser, 8, "theirs", true)

	viewer := uint(7)
	items, _, err := repo.ListUIComponents(ctx, repository.UIComponentListFilter{VisibleUserID: &viewer}, 0, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	want := []string{"card-grid", "platform-on", "mine"}
	if len(names) != len(want) {
		t.Fatalf("visible set = %v, want %v", names, want)
	}
	for index := range want {
		if names[index] != want[index] {
			t.Fatalf("visible order = %v, want %v", names, want)
		}
	}
}
