package uicomponent

import (
	"regexp"
	"testing"
)

// 内置目录与前端 shared/components/markdown/ui-blocks/registry.tsx 按 name@version 配对。
// 新增或改版内置组件时，两侧必须同时更新；本测试固定后端一侧，避免单边漂移。
func TestBuiltinCatalogMatchesFrontendRegistry(t *testing.T) {
	want := map[string]int{
		"card-grid":     1,
		"stat-grid":     1,
		"chart":         1,
		"function-plot": 1,
		"data-table":    1,
		"gantt":         1,
		"diff":          1,
		"calculator":    1,
		"decision-tree": 1,
		"quiz":          1,
	}
	namePattern := regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	got := Builtin()
	if len(got) != len(want) {
		t.Fatalf("builtin catalog has %d components, want %d", len(got), len(want))
	}
	seen := map[string]bool{}
	for _, component := range got {
		version, ok := want[component.Name]
		if !ok || seen[component.Name] {
			t.Fatalf("unexpected or duplicate builtin component %q", component.Name)
		}
		seen[component.Name] = true
		if component.Version != version {
			t.Fatalf("%s version = %d, want %d", component.Name, component.Version, version)
		}
		if !namePattern.MatchString(component.Name) {
			t.Fatalf("%q is not a valid component name", component.Name)
		}
		if component.Description == "" || component.PropsSummary == "" || component.RendererKind != RendererBuiltin || !component.Enabled {
			t.Fatalf("builtin component %q is incomplete: %+v", component.Name, component)
		}
	}
}

func TestBuiltinCatalogFitsStorage(t *testing.T) {
	for _, item := range Builtin() {
		if n := len([]rune(item.Description)); n > 256 {
			t.Fatalf("%s description is %d runes, column allows 256", item.Name, n)
		}
		if n := len([]rune(item.PropsSummary)); n > 1024 {
			t.Fatalf("%s props summary is %d runes, column allows 1024", item.Name, n)
		}
	}
}
