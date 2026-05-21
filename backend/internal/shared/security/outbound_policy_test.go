package security

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type outboundCallSiteRule struct {
	reason string
	files  map[string]struct{}
}

func TestOutboundHTTPCallSitesAreExplicit(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}
	backendDir := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	rules := map[string]outboundCallSiteRule{
		"http.DefaultClient": {
			reason: "default HTTP client bypasses the SSRF policy; use security.NewOutboundHTTPClient for external URLs or an approved trusted-internal client",
			files:  map[string]struct{}{},
		},
		"http.Get(": {
			reason: "package-level HTTP helpers bypass the SSRF policy; use an explicit client",
			files:  map[string]struct{}{},
		},
		"http.Post(": {
			reason: "package-level HTTP helpers bypass the SSRF policy; use an explicit client",
			files:  map[string]struct{}{},
		},
		"http.Head(": {
			reason: "package-level HTTP helpers bypass the SSRF policy; use an explicit client",
			files:  map[string]struct{}{},
		},
		"&http.Client{": {
			reason: "direct HTTP clients must be reviewed for external-vs-internal trust boundaries",
			files: allowFiles(
				"internal/application/auth/service.go",
				"internal/infra/embedding/client.go",
				"internal/infra/extract/mineru/client.go",
				"internal/infra/geoip/client.go",
				"internal/infra/llm/client.go",
				"internal/infra/mcp/client.go",
				"internal/infra/observability/tracing/http.go",
				"internal/shared/security/security.go",
				"internal/transport/http/billing/payment.go",
			),
		},
		"platformtracing.NewHTTPClient(": {
			reason: "raw tracing HTTP clients are reserved for trusted internal services or fixed endpoints",
			files: allowFiles(
				"internal/infra/extract/docling/client.go",
				"internal/infra/extract/mineru/client.go",
				"internal/infra/extract/ocr/client.go",
				"internal/infra/extract/tika/client.go",
				"internal/infra/geoip/mmdb.go",
			),
		},
	}

	for _, root := range []string{
		filepath.Join(backendDir, "cmd"),
		filepath.Join(backendDir, "internal"),
	} {
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(backendDir, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			source := string(contents)
			for pattern, rule := range rules {
				if !strings.Contains(source, pattern) {
					continue
				}
				if _, allowed := rule.files[rel]; !allowed {
					t.Fatalf("unreviewed outbound HTTP call site %q in %s: %s", pattern, rel, rule.reason)
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func allowFiles(files ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(files))
	for _, file := range files {
		result[file] = struct{}{}
	}
	return result
}
