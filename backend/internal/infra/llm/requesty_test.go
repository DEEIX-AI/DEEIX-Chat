package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func TestListModelsRequestyMergesManagedPoliciesBeforeCatalog(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"origin": "router", "message": "Invalid authorization token"},
			})
			return
		}
		switch r.URL.Path {
		case "/v1/models/managed":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object": "list",
				"data": []map[string]any{
					{"id": "claude-sonnet-4-5", "object": "model", "owned_by": "system"},
					{"id": "gpt-5-mini@eu", "object": "model", "owned_by": "system"},
				},
			})
		case "/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object": "list",
				"data": []map[string]any{
					{"id": "openai/gpt-4o-mini", "object": "model", "owned_by": "system"},
					{"id": "claude-sonnet-4-5", "object": "model", "owned_by": "system"},
					{"id": "anthropic/claude-sonnet-4-5", "object": "model", "owned_by": "system"},
				},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	items, err := NewClient(security.NewStrictOutboundPolicy(true)).listModelsRequesty(context.Background(), portllm.RouteConfig{
		Protocol: portllm.AdapterOpenAIChatCompletions,
		BaseURL:  server.URL + "/v1",
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("expected requesty listing to succeed, got %v", err)
	}
	if len(paths) != 2 || paths[0] != "/v1/models" || paths[1] != "/v1/models/managed" {
		t.Fatalf("expected catalog then managed requests, got %#v", paths)
	}
	expected := []string{"claude-sonnet-4-5", "gpt-5-mini@eu", "openai/gpt-4o-mini", "anthropic/claude-sonnet-4-5"}
	if len(items) != len(expected) {
		t.Fatalf("expected %d merged models, got %#v", len(expected), items)
	}
	for index, id := range expected {
		if items[index].ID != id {
			t.Fatalf("expected model %d to be %q, got %#v", index, id, items)
		}
	}
}

func TestListModelsRequestyFallsBackToCatalogWhenManagedUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models/managed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data":   []map[string]any{{"id": "openai/gpt-4o-mini", "object": "model", "owned_by": "system"}},
		})
	}))
	defer server.Close()

	items, err := NewClient(security.NewStrictOutboundPolicy(true)).listModelsRequesty(context.Background(), portllm.RouteConfig{
		Protocol: portllm.AdapterOpenAIChatCompletions,
		BaseURL:  server.URL + "/v1",
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("expected catalog fallback to succeed, got %v", err)
	}
	if len(items) != 1 || items[0].ID != "openai/gpt-4o-mini" {
		t.Fatalf("unexpected fallback models: %#v", items)
	}
}

func TestListModelsRequestySurfacesInvalidKeyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"origin": "router", "message": "Invalid authorization token"},
		})
	}))
	defer server.Close()

	_, err := NewClient(security.NewStrictOutboundPolicy(true)).listModelsRequesty(context.Background(), portllm.RouteConfig{
		Protocol: portllm.AdapterOpenAIChatCompletions,
		BaseURL:  server.URL + "/v1",
		APIKey:   "bad-key",
	})
	if err == nil {
		t.Fatal("expected invalid key to fail the catalog request")
	}
}

func TestIsRequestyBaseURL(t *testing.T) {
	for _, raw := range []string{
		"https://router.requesty.ai/v1",
		"https://router.eu.requesty.ai/v1/",
		"https://requesty.ai",
	} {
		if !isRequestyBaseURL(raw) {
			t.Fatalf("expected %q to be recognised as a Requesty base URL", raw)
		}
	}
	for _, raw := range []string{
		"https://openrouter.ai/api/v1",
		"https://api.openai.com/v1",
		"https://notrequesty.ai/v1",
		"",
	} {
		if isRequestyBaseURL(raw) {
			t.Fatalf("expected %q not to be recognised as a Requesty base URL", raw)
		}
	}
}

func TestBuildRequestyManagedModelsURL(t *testing.T) {
	if got := buildRequestyManagedModelsURL("https://router.requesty.ai/v1"); got != "https://router.requesty.ai/v1/models/managed" {
		t.Fatalf("unexpected managed models url %q", got)
	}
	if got := buildRequestyManagedModelsURL("https://router.requesty.ai"); got != "https://router.requesty.ai/v1/models/managed" {
		t.Fatalf("unexpected managed models url without version %q", got)
	}
}
