package uicomponent

import (
	"context"
	"errors"
	"strings"
	"testing"

	domainuicomponent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/uicomponent"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type fakeRepo struct {
	items   map[uint]domainuicomponent.Component
	created []domainuicomponent.Component
	patched []repository.UIComponentPatch
	deleted []uint
}

func (r *fakeRepo) ListUIComponents(_ context.Context, filter repository.UIComponentListFilter, _ int, _ int) ([]domainuicomponent.Component, int64, error) {
	results := make([]domainuicomponent.Component, 0)
	for _, id := range filter.IDs {
		if item, ok := r.items[id]; ok && item.Enabled {
			results = append(results, item)
		}
	}
	return results, int64(len(results)), nil
}

func (r *fakeRepo) GetUIComponent(_ context.Context, id uint) (*domainuicomponent.Component, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &item, nil
}

func (r *fakeRepo) CreateUIComponent(_ context.Context, item *domainuicomponent.Component) (*domainuicomponent.Component, error) {
	r.created = append(r.created, *item)
	return item, nil
}

func (r *fakeRepo) PatchUIComponent(_ context.Context, id uint, patch repository.UIComponentPatch) (*domainuicomponent.Component, error) {
	r.patched = append(r.patched, patch)
	item := r.items[id]
	return &item, nil
}

func (r *fakeRepo) DeleteUIComponent(_ context.Context, id uint) error {
	r.deleted = append(r.deleted, id)
	return nil
}

func validWrite() WriteInput {
	return WriteInput{Name: "stock-glance", Description: "股票速览", PropsSummary: "{symbol}", RendererSource: "<div id=root></div>", Enabled: true}
}

func TestCreateUserRejectsInvalidName(t *testing.T) {
	service := NewService(&fakeRepo{items: map[uint]domainuicomponent.Component{}})
	for _, name := range []string{"", "Stock", "stock_glance", "-lead", "trail-", "has space", "中文"} {
		input := validWrite()
		input.Name = name
		if _, err := service.CreateUser(context.Background(), 1, input); !errors.Is(err, ErrInvalidComponent) {
			t.Fatalf("name %q: expected ErrInvalidComponent, got %v", name, err)
		}
	}
}

func TestCreateUserRequiresRendererSourceAndValidSchema(t *testing.T) {
	service := NewService(&fakeRepo{items: map[uint]domainuicomponent.Component{}})

	noSource := validWrite()
	noSource.RendererSource = "  "
	if _, err := service.CreateUser(context.Background(), 1, noSource); !errors.Is(err, ErrInvalidComponent) {
		t.Fatalf("expected renderer source to be required, got %v", err)
	}

	badSchema := validWrite()
	badSchema.PropsSchema = "[1,2]"
	if _, err := service.CreateUser(context.Background(), 1, badSchema); !errors.Is(err, ErrInvalidComponent) {
		t.Fatalf("expected non-object schema to be rejected, got %v", err)
	}

	tooLarge := validWrite()
	tooLarge.RendererSource = strings.Repeat("x", maxRendererSourceBytes+1)
	if _, err := service.CreateUser(context.Background(), 1, tooLarge); !errors.Is(err, ErrInvalidComponent) {
		t.Fatalf("expected oversized renderer source to be rejected, got %v", err)
	}
}

func TestCreateUserAssignsScopeSandboxAndDefaultVersion(t *testing.T) {
	repo := &fakeRepo{items: map[uint]domainuicomponent.Component{}}
	service := NewService(repo)

	item, err := service.CreateUser(context.Background(), 7, validWrite())
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if item.Scope != domainuicomponent.ScopeUser || item.OwnerUserID != 7 || item.RendererKind != domainuicomponent.RendererSandbox || item.Version != 1 {
		t.Fatalf("unexpected component: %+v", item)
	}
}

func TestUpdateAdminProtectsBuiltinContract(t *testing.T) {
	repo := &fakeRepo{items: map[uint]domainuicomponent.Component{
		1: {ID: 1, Scope: domainuicomponent.ScopeBuiltin, Name: "chart", RendererKind: domainuicomponent.RendererBuiltin, Enabled: true},
	}}
	service := NewService(repo)
	name := "chart2"
	source := "<div/>"
	description := "改描述"
	for _, input := range []PatchInput{{Name: &name}, {RendererSource: &source}, {Description: &description}} {
		if _, err := service.UpdateAdmin(context.Background(), 1, 1, input); !errors.Is(err, ErrBuiltinProtected) {
			t.Fatalf("expected builtin contract to be protected, got %v", err)
		}
	}

	disabled := false
	if _, err := service.UpdateAdmin(context.Background(), 1, 1, PatchInput{Enabled: &disabled}); err != nil {
		t.Fatalf("builtin enabled must be editable, got %v", err)
	}
	if len(repo.patched) != 1 || repo.patched[0].Enabled == nil || *repo.patched[0].Enabled {
		t.Fatalf("expected enabled=false patch, got %+v", repo.patched)
	}
}

func TestDeleteAdminRefusesBuiltinAndUserScopes(t *testing.T) {
	repo := &fakeRepo{items: map[uint]domainuicomponent.Component{
		1: {ID: 1, Scope: domainuicomponent.ScopeBuiltin},
		2: {ID: 2, Scope: domainuicomponent.ScopeUser, OwnerUserID: 9},
		3: {ID: 3, Scope: domainuicomponent.ScopePlatform},
	}}
	service := NewService(repo)
	if err := service.DeleteAdmin(context.Background(), 1, 1); !errors.Is(err, ErrBuiltinProtected) {
		t.Fatalf("builtin delete: %v", err)
	}
	if err := service.DeleteAdmin(context.Background(), 1, 2); !errors.Is(err, ErrComponentNotFound) {
		t.Fatalf("admin must not delete user components: %v", err)
	}
	if err := service.DeleteAdmin(context.Background(), 1, 3); err != nil || len(repo.deleted) != 1 {
		t.Fatalf("platform delete: err=%v deleted=%v", err, repo.deleted)
	}
}

func TestResolveVisibleKeepsRequestOrderAndDropsHidden(t *testing.T) {
	repo := &fakeRepo{items: map[uint]domainuicomponent.Component{
		1: {ID: 1, Name: "a", Enabled: true},
		2: {ID: 2, Name: "b", Enabled: false},
		3: {ID: 3, Name: "c", Enabled: true},
	}}
	service := NewService(repo)
	items, err := service.ResolveVisible(context.Background(), 1, []uint{3, 2, 1, 3, 0})
	if err != nil {
		t.Fatalf("ResolveVisible: %v", err)
	}
	if len(items) != 2 || items[0].Name != "c" || items[1].Name != "a" {
		t.Fatalf("unexpected resolution: %+v", items)
	}
}

func TestCatalogPromptListsEachComponentOnce(t *testing.T) {
	prompt := CatalogPrompt(domainuicomponent.Builtin())
	for _, want := range []string{"`deeix-ui`", "<nature>", "不是工具", `<component name="card-grid">`, `<component name="data-table">`, `<component name="chart">`, "<props>{"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in prompt:\n%s", want, prompt)
		}
	}
	if CatalogPrompt(nil) != "" {
		t.Fatal("empty catalog must produce no prompt")
	}
	if strings.Contains(prompt, `version="`) {
		t.Fatal("v1 components must not ask the model to emit a version")
	}
	v2 := CatalogPrompt([]domainuicomponent.Component{{Name: "chart", Version: 2, Description: "d", PropsSummary: "{}"}})
	if !strings.Contains(v2, `<component name="chart" version="2">`) {
		t.Fatalf("breaking versions must be announced in the catalog, got %q", v2)
	}
}

func TestListVisibleReturnsNothingWhenFeatureDisabled(t *testing.T) {
	repo := &fakeRepo{items: map[uint]domainuicomponent.Component{1: {ID: 1, Scope: domainuicomponent.ScopeBuiltin, Name: "chart", Enabled: true}}}
	service := NewService(repo)
	service.SetFeatureEnabled(func() bool { return false })
	items, total, err := service.ListVisible(context.Background(), 7, ListInput{})
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("disabled feature must hide the catalog, got %d/%d err=%v", len(items), total, err)
	}
	resolved, err := service.ResolveVisible(context.Background(), 7, []uint{1})
	if err != nil || len(resolved) != 0 {
		t.Fatalf("disabled feature must resolve to no components, got %v err=%v", resolved, err)
	}
}
