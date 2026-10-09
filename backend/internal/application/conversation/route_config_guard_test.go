package conversation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
)

// 用户自带 Key 的路由必须走强制 SSRF 防护的客户端。任何在本包手写 llm.RouteConfig 的地方都可能漏传
// UntrustedEndpoint，因此只允许 routeConfigFromResolved 构造它。
func TestRouteConfigIsOnlyBuiltByTheSharedHelper(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		count := strings.Count(string(raw), "llm.RouteConfig{")
		allowed := 0
		if name == "service_message_send.go" {
			allowed = 1
		}
		if count != allowed {
			t.Errorf("%s builds llm.RouteConfig{} %d time(s); use routeConfigFromResolved so UntrustedEndpoint is never dropped", name, count)
		}
	}
}

func TestRouteConfigCarriesUntrustedEndpoint(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol:          "openai_chat_completions",
		BaseURL:           "https://api.example.com/v1",
		APIKey:            "sk-test",
		UpstreamModel:     "gpt-4o",
		UntrustedEndpoint: true,
	}
	if config := messageRouteConfig(route, "", ""); !config.UntrustedEndpoint {
		t.Fatal("messageRouteConfig dropped UntrustedEndpoint")
	}
	if config := routeConfigFromResolved(route, "images", "", ""); !config.UntrustedEndpoint || config.Endpoint != "images" {
		t.Fatalf("routeConfigFromResolved = %#v", config)
	}
	route.UntrustedEndpoint = false
	if config := messageRouteConfig(route, "", ""); config.UntrustedEndpoint {
		t.Fatal("platform routes must stay trusted")
	}
}
