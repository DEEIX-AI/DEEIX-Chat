package security

import "testing"

func TestRedactHeadersJSONMasksSensitiveHeaders(t *testing.T) {
	got := RedactHeadersJSON(`{"Authorization":"Bearer secret","X-API-Key":"key","X-Title":"DEEIX"}`)
	want := `{"Authorization":"********","X-API-Key":"********","X-Title":"DEEIX"}`
	if got != want {
		t.Fatalf("unexpected redacted headers: got %s want %s", got, want)
	}
}
