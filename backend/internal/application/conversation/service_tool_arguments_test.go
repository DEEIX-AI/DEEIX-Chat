package conversation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeToolArgumentsCoercesSchemaDeclaredScalars(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"count":{"type":"number"},"safe":{"type":"boolean"}},"required":["query"]}`)

	got, err := normalizeToolArguments(`{"query":"DEEIX Chat","count":"3","safe":"true"}`, schema)
	if err != nil {
		t.Fatalf("normalize arguments: %v", err)
	}
	if got != `{"count":3,"query":"DEEIX Chat","safe":true}` {
		t.Fatalf("unexpected normalized arguments: %s", got)
	}
}

func TestNormalizeToolArgumentsAcceptsNumericEnumValues(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"safesearch":{"type":"number","enum":[0,1,2],"default":0}},"required":["query"]}`)

	got, err := normalizeToolArguments(`{"query":"weather","safesearch":0}`, schema)
	if err != nil {
		t.Fatalf("normalize arguments: %v", err)
	}
	if got != `{"query":"weather","safesearch":0}` {
		t.Fatalf("unexpected normalized arguments: %s", got)
	}
}

func TestNormalizeToolArgumentsRejectsMissingRequiredField(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"count":{"type":"number"}},"required":["query"]}`)

	_, err := normalizeToolArguments(`{"count":3}`, schema)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "required parameter `query` is missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeToolArgumentsAllowsMissingOptionalField(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"count":{"type":"number"}}}`)

	got, err := normalizeToolArguments(`{"count":"3"}`, schema)
	if err != nil {
		t.Fatalf("normalize arguments: %v", err)
	}
	if got != `{"count":3}` {
		t.Fatalf("unexpected normalized arguments: %s", got)
	}
}

func TestNormalizeToolArgumentsRejectsInvalidJSON(t *testing.T) {
	_, err := normalizeToolArguments(`{bad`, json.RawMessage(`{"type":"object","properties":{}}`))
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "valid JSON object") {
		t.Fatalf("unexpected error: %v", err)
	}
}
