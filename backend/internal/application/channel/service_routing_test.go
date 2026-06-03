package channel

import (
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestBindingCircuitKeyUsesBindingCode(t *testing.T) {
	if got := bindingCircuitKey("upm_42"); got != "upstream-model-upm_42" {
		t.Fatalf("expected binding-level circuit key, got %q", got)
	}
	if got := bindingCircuitKey("upm:42"); got != "upstream-model-upm-42" {
		t.Fatalf("expected colon-free binding-level circuit key, got %q", got)
	}
	if got := bindingCircuitKey(""); got != "" {
		t.Fatalf("expected empty key for empty binding code, got %q", got)
	}
}

func TestRemoveCandidateUsesUpstreamModelIDInsteadOfPlatformModelName(t *testing.T) {
	items := []routeCandidate{
		{row: repository.ChannelUpstreamRouteRow{UpstreamID: 1, UpstreamModelID: 10, PlatformModelName: "gpt-5.5"}},
		{row: repository.ChannelUpstreamRouteRow{UpstreamID: 1, UpstreamModelID: 11, PlatformModelName: "gpt-5.5"}},
		{row: repository.ChannelUpstreamRouteRow{UpstreamID: 2, UpstreamModelID: 10, PlatformModelName: "gpt-5.5"}},
	}

	got := removeCandidate(items, 1, 10)
	if len(got) != 2 {
		t.Fatalf("expected only one route candidate removed, got %d", len(got))
	}
	for _, item := range got {
		if item.row.UpstreamID == 1 && item.row.UpstreamModelID == 10 {
			t.Fatalf("route candidate was not removed: %#v", got)
		}
	}
}

func TestBuildResolvedRouteSnapshotsModelIdentity(t *testing.T) {
	route := buildResolvedRoute(repository.ChannelUpstreamRouteRow{
		RouteID:           5,
		PlatformModelID:   9,
		PlatformModelName: "gpt-5.5",
		UpstreamModelID:   7,
		UpstreamID:        3,
		UpstreamName:      "OpenAI Official",
		BindingCode:       "upm_abc",
		ModelVendor:       "openai",
		ModelIcon:         "openai",
		UpstreamModelName: "gpt-5.5-20260501",
		Protocol:          "openai_responses",
	}, "sk-test")

	if route.RouteID != 5 || route.PlatformModelID != 9 || route.UpstreamModelID != 7 {
		t.Fatalf("expected route identity snapshot, got %#v", route)
	}
	if route.UpstreamModel != "gpt-5.5-20260501" {
		t.Fatalf("expected upstream model name, got %q", route.UpstreamModel)
	}
	if route.PlatformModelName != "gpt-5.5" || route.BindingCode != "upm_abc" || route.ModelVendor != "openai" || route.ModelIcon != "openai" {
		t.Fatalf("expected platform model snapshot, got %#v", route)
	}
}

func TestRouteScopeAllowsModelAccessDefaultsToUserScope(t *testing.T) {
	for _, scope := range []string{"", "unknown", RouteScopeUser} {
		if routeScopeAllowsModelAccess(scope, ModelAccessScopeInternal) {
			t.Fatalf("scope %q should not access internal model", scope)
		}
		if !routeScopeAllowsModelAccess(scope, ModelAccessScopePublic) {
			t.Fatalf("scope %q should access public model", scope)
		}
	}
}

func TestRouteScopeAllowsInternalModelForInternalScope(t *testing.T) {
	if !routeScopeAllowsModelAccess(RouteScopeInternal, ModelAccessScopeInternal) {
		t.Fatalf("internal scope should access internal model")
	}
}
