package uicomponent

import (
	"context"
	"errors"
	"slices"
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
	// 原语名（panel、button…）与能力名（widget）都已被占用。
	for _, name := range []string{"", "Stock", "stock_glance", "-lead", "trail-", "has space", "中文", "panel", "button", "widget"} {
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
	prompt := CatalogPrompt(domainuicomponent.Builtin(), domainuicomponent.BlockFormatStatic)
	for _, want := range []string{"`deeix-ui`", "<nature>", "不是工具", `<component name="card-grid">`, `<component name="data-table">`, `<component name="chart">`, "<props>{"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in prompt:\n%s", want, prompt)
		}
	}
	if CatalogPrompt(nil, domainuicomponent.BlockFormatStatic) != "" || CatalogPrompt(nil, domainuicomponent.BlockFormatLive) != "" {
		t.Fatal("empty catalog must produce no prompt")
	}
	if strings.Contains(prompt, `version="`) {
		t.Fatal("v1 components must not ask the model to emit a version")
	}
	v2 := CatalogPrompt([]domainuicomponent.Component{{Name: "chart", Version: 2, Description: "d", PropsSummary: "{}"}}, domainuicomponent.BlockFormatStatic)
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

func TestCatalogPromptStaticFormatIsUnchangedWhenUndeclared(t *testing.T) {
	builtin := domainuicomponent.Builtin()
	if CatalogPrompt(builtin, 0) != CatalogPrompt(builtin, domainuicomponent.BlockFormatStatic) {
		t.Fatal("a client that declares no block format must get the static catalog")
	}
	prompt := CatalogPrompt(builtin, domainuicomponent.BlockFormatStatic)
	if !strings.HasPrefix(prompt, catalogPromptHeader) || strings.Contains(prompt, "<live>") {
		t.Fatal("the static catalog must keep its header byte for byte")
	}
	if !strings.Contains(prompt, `<component name="calculator">`) {
		t.Fatal("static-format clients still get calculator")
	}
}

func TestCatalogPromptLiveFormat(t *testing.T) {
	builtin := domainuicomponent.Builtin()
	prompt := CatalogPrompt(builtin, domainuicomponent.BlockFormatLive)
	for _, want := range []string{"<nature>", "<principle>", "<use>", "<avoid>", "<choose>", "<followup>", "<live>", "<primitives>", "<example>", "ui_state", "ask:", "where:", "</catalog>", `<component name="chart">`} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in the interactive catalog", want)
		}
	}
	if strings.Contains(prompt, `<component name="calculator">`) {
		t.Fatal("panel supersedes calculator in the interactive catalog")
	}
	for _, primitive := range domainuicomponent.Primitives {
		if !strings.Contains(prompt, primitive) {
			t.Fatalf("primitive %q missing from the interactive catalog", primitive)
		}
	}

	custom := []domainuicomponent.Component{{Scope: domainuicomponent.ScopeUser, RendererKind: domainuicomponent.RendererSandbox, Name: "calculator", Version: 1, Description: "d", PropsSummary: "{}"}}
	if !strings.Contains(CatalogPrompt(custom, domainuicomponent.BlockFormatLive), `<component name="calculator">`) {
		t.Fatal("only the builtin calculator is superseded, not a custom component of the same name")
	}
	calculator := domainuicomponent.Component{Scope: domainuicomponent.ScopeBuiltin, RendererKind: domainuicomponent.RendererBuiltin, Name: "calculator", Version: 1, Description: "d", PropsSummary: "{}"}
	if got := CatalogPrompt([]domainuicomponent.Component{calculator}, domainuicomponent.BlockFormatLive); strings.Contains(got, "<live>") || !strings.Contains(got, `<component name="calculator">`) {
		t.Fatal("without panel there is nothing to replace calculator, so it stays in the interactive catalog")
	}
}

// panel 与 widget 只决定可交互格式的目录写不写对应的条款与整节，自己不作为组件列出；静态格式两者都不出现。
func TestCatalogPromptCapabilitiesGateLiveSections(t *testing.T) {
	builtin := domainuicomponent.Builtin()
	without := func(names ...string) []domainuicomponent.Component {
		kept := make([]domainuicomponent.Component, 0, len(builtin))
		for _, component := range builtin {
			if !slices.Contains(names, component.Name) {
				kept = append(kept, component)
			}
		}
		return kept
	}
	for _, format := range []int{domainuicomponent.BlockFormatStatic, domainuicomponent.BlockFormatLive} {
		prompt := CatalogPrompt(builtin, format)
		if strings.Contains(prompt, `<component name="panel">`) || strings.Contains(prompt, `<component name="widget">`) {
			t.Fatalf("capabilities must not be listed as components (format %d)", format)
		}
	}
	if static := CatalogPrompt(builtin, domainuicomponent.BlockFormatStatic); strings.Contains(static, "<widget>") || strings.Contains(static, domainuicomponent.WidgetFenceLanguage) {
		t.Fatal("static-format clients cannot render widgets")
	}

	cases := []struct {
		name          string
		components    []domainuicomponent.Component
		panel, widget bool
	}{
		{name: "both", components: builtin, panel: true, widget: true},
		{name: "no widget", components: without("widget"), panel: true},
		{name: "no panel", components: without("panel"), widget: true},
		{name: "neither", components: without("panel", "widget")},
	}
	for _, tc := range cases {
		prompt := CatalogPrompt(tc.components, domainuicomponent.BlockFormatLive)
		_, choose, _ := strings.Cut(prompt, "<choose>")
		choose, _, _ = strings.Cut(choose, "</choose>")
		_, use, _ := strings.Cut(prompt, "  <use>满足")
		use, _, _ = strings.Cut(use, "</use>")
		if got := strings.Contains(prompt, "<live>"); got != tc.panel {
			t.Fatalf("%s: <live> present = %v", tc.name, got)
		}
		if got := strings.Contains(choose, "panel") || strings.Contains(use, "用 panel"); got != tc.panel {
			t.Fatalf("%s: panel named in <use>/<choose> = %v", tc.name, got)
		}
		if got := strings.Contains(prompt, "<widget>\n") || strings.Contains(choose, "widget") || strings.Contains(use, "<widget>"); got != tc.widget {
			t.Fatalf("%s: widget described = %v", tc.name, got)
		}
		if got := strings.Contains(prompt, `<component name="calculator">`); got == tc.panel {
			t.Fatalf("%s: calculator listed = %v, want it only without panel", tc.name, got)
		}
		if !strings.Contains(prompt, `<component name="chart">`) || !strings.HasSuffix(prompt, "</catalog>") {
			t.Fatalf("%s: the catalog must still be listed", tc.name)
		}
	}

	onlyCapabilities := CatalogPrompt(onlyCapabilitiesFor(builtin), domainuicomponent.BlockFormatLive)
	if !strings.Contains(onlyCapabilities, "<live>") || !strings.Contains(onlyCapabilities, "<widget>") || strings.Contains(onlyCapabilities, "<catalog>") {
		t.Fatal("panel and widget alone still describe their blocks, with no empty catalog")
	}
	if CatalogPrompt(onlyCapabilitiesFor(builtin), domainuicomponent.BlockFormatStatic) != "" {
		t.Fatal("a static-format client with only capabilities selected gets no catalog layer")
	}
}

func onlyCapabilitiesFor(components []domainuicomponent.Component) []domainuicomponent.Component {
	var kept []domainuicomponent.Component
	for _, component := range components {
		if domainuicomponent.IsCapability(component.Name) {
			kept = append(kept, component)
		}
	}
	return kept
}

// 使用条件必须正反两面都写：只写「什么时候用」会让模型到处用，只写限制又会让它不敢用。
func TestCatalogPromptLiveStatesWhenNotToUseComponents(t *testing.T) {
	_, when, _ := strings.Cut(liveCatalogHeader(true, true), "<when>")
	when, _, _ = strings.Cut(when, "</when>")
	for _, want := range []string{"不是装饰", "默认就该出组件", "该用而没用", "超过五行", "拿不准时", "工具结果", "优先于 <use>", "不要用 panel 包一个静态组件", "最多三个", "新 id", "代替回答", "<check>", "讲解公式", "定性概念", "<widget>", "由轻到重"} {
		if !strings.Contains(when, want) {
			t.Fatalf("expected %q in the interactive <when> section", want)
		}
	}
}

// 内联图解在沙箱里运行：提示词必须写明格式、无网络、主题变量与状态接口，且静态格式的目录里不出现。
func TestCatalogPromptLiveDescribesWidgets(t *testing.T) {
	_, widget, found := strings.Cut(liveCatalogHeader(true, true), "<widget>\n")
	if !found {
		t.Fatal("the interactive catalog must describe widgets")
	}
	widget, _, _ = strings.Cut(widget, "</widget>")
	for _, want := range []string{domainuicomponent.WidgetFenceLanguage, "没有网络", "CDN", "var(--foreground)", "prefers-reduced-motion", "setState", "deeix.suggest", "Artifact", "16px", "controls", `class="stable"`, `class="on"`, `class="num"`} {
		if !strings.Contains(widget, want) {
			t.Fatalf("expected %q in the widget section", want)
		}
	}
	// 追问框由宿主统一渲染：图解自己画的输入框和按钮与界面不一致，按钮也只能填入输入框。
	for _, unwanted := range []string{"textarea", "deeix.ask", `class="ask"`} {
		if strings.Contains(widget, unwanted) {
			t.Fatalf("the widget section must not ask for a hand-written follow-up (%q)", unwanted)
		}
	}
	if strings.Contains(CatalogPrompt(domainuicomponent.Builtin(), domainuicomponent.BlockFormatStatic), domainuicomponent.WidgetFenceLanguage) {
		t.Fatal("static-format clients cannot render widgets")
	}
}
