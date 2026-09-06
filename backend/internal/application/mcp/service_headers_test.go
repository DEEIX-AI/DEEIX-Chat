package mcp

import "testing"

func TestParseHeadersJSONRejectsForbiddenHeaders(t *testing.T) {
	_, err := parseHeadersJSON(`{"Host":"internal","X-Custom":"ok"}`)
	if err == nil {
		t.Fatal("expected Host header to be rejected")
	}

	headers, err := parseHeadersJSON(`{"X-Custom":"ok","X-Trace":"1"}`)
	if err != nil {
		t.Fatalf("expected safe headers to parse, got %v", err)
	}
	if headers["X-Custom"] != "ok" || headers["X-Trace"] != "1" {
		t.Fatalf("unexpected headers: %#v", headers)
	}
}
