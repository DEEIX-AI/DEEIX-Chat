package toolresult

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactSecretDecodesToolJSONAndTextBlocks(t *testing.T) {
	const secret = "dxf1_sensitive"
	raw := `{"dxf\u0031_sensitive":"dxf\u0031_sensitive","content":[{"type":"text","text":"{\"credential\":\"dxf\\u0031_sensitive\"}"}],"number":9007199254740993}`
	output := RedactSecret(raw, secret)
	if strings.Contains(output, "sensitive") || !strings.Contains(output, "9007199254740993") || !json.Valid([]byte(output)) {
		t.Fatalf("unsafe output: %s", output)
	}
	if RedactSecret("failed: "+secret, secret) != "failed: [redacted]" {
		t.Fatal("plain error leaked bearer")
	}
	for _, raw := range []string{`mcp tool error: {"credential":"dxf\u0031_sensitive"}`, "Sources:\n" + `{"credential":"dxf\u0031_sensitive"}`} {
		if output := RedactSecret(raw, secret); strings.Contains(output, "sensitive") {
			t.Fatalf("embedded JSON leaked: %s", output)
		}
	}
}
