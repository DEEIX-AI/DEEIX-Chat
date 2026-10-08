package llm

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func modelsListServer(t *testing.T, hits *atomic.Int64) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data":   []map[string]any{{"id": "gpt-4o", "object": "model"}},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

// 管理员上游会被局部授信，所以同一个回环地址可达；用户自配端点必须被严格策略拦下，且请求不能发出。
func TestUntrustedEndpointNeverReachesLoopbackEvenWhenTrustedRoutesWould(t *testing.T) {
	var hits atomic.Int64
	server := modelsListServer(t, &hits)
	client := NewClient(security.NewStrictOutboundPolicy(true))

	route := portllm.RouteConfig{Protocol: portllm.AdapterOpenAIChatCompletions, BaseURL: server.URL + "/v1", APIKey: "sk-test"}
	if _, err := client.ListModels(t.Context(), route); err != nil {
		t.Fatalf("admin route to an explicitly configured endpoint should work: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("admin route hits = %d, want 1", hits.Load())
	}

	route.UntrustedEndpoint = true
	_, err := client.ListModels(t.Context(), route)
	if err == nil {
		t.Fatal("user-configured loopback endpoint must be rejected")
	}
	if !errors.Is(err, security.ErrUnsafeOutboundURL) {
		t.Fatalf("expected an unsafe outbound error, got %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("untrusted request reached the server: hits = %d", hits.Load())
	}
}

func TestUntrustedEndpointRefusesRedirects(t *testing.T) {
	var targetHits atomic.Int64
	target := modelsListServer(t, &targetHits)
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redirector.Close)

	// 测试需要连到本机 httptest，所以注入一个不强制的策略；重定向拒绝与策略无关。
	client := newClientWithUntrustedPolicy(security.NewStrictOutboundPolicy(true), security.NewStrictOutboundPolicy(false))
	_, err := client.ListModels(t.Context(), portllm.RouteConfig{
		Protocol:          portllm.AdapterOpenAIChatCompletions,
		BaseURL:           redirector.URL + "/v1",
		APIKey:            "sk-test",
		UntrustedEndpoint: true,
	})
	if err == nil || !strings.Contains(err.Error(), errUntrustedRedirect.Error()) {
		t.Fatalf("expected redirect refusal, got %v", err)
	}
	if targetHits.Load() != 0 {
		t.Fatalf("redirect target was contacted %d times", targetHits.Load())
	}
}

func TestUntrustedEndpointRejectsRequestsToAnotherOrigin(t *testing.T) {
	client := newClientWithUntrustedPolicy(security.NewStrictOutboundPolicy(true), security.NewStrictOutboundPolicy(false))
	request, err := http.NewRequest(http.MethodGet, "https://other.example.com/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.doRouteRequest(portllm.RouteConfig{
		BaseURL:           "https://api.example.com/v1",
		UntrustedEndpoint: true,
	}, request)
	if err == nil || !strings.Contains(err.Error(), "changed configured origin") {
		t.Fatalf("expected origin mismatch refusal, got %v", err)
	}
}

func TestUntrustedEndpointWorksForAllowedTargets(t *testing.T) {
	var hits atomic.Int64
	server := modelsListServer(t, &hits)
	client := newClientWithUntrustedPolicy(security.NewStrictOutboundPolicy(true), security.NewStrictOutboundPolicy(false))
	items, err := client.ListModels(t.Context(), portllm.RouteConfig{
		Protocol:          portllm.AdapterOpenAIChatCompletions,
		BaseURL:           server.URL + "/v1",
		APIKey:            "sk-test",
		UntrustedEndpoint: true,
	})
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if len(items) != 1 || items[0].ID != "gpt-4o" {
		t.Fatalf("unexpected models: %#v", items)
	}
}
