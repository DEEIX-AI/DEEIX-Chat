package ocr

import (
	"encoding/json"
	"testing"
)

func TestTraditionalOCRPayloadPagesCount(t *testing.T) {
	var payload traditionalOCRPayload
	if err := json.Unmarshal([]byte(`{"pages":3,"text":"hello"}`), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if got := payload.PageCount(); got != 3 {
		t.Fatalf("PageCount() = %d, want 3", got)
	}
	if got := payload.ExtractedText(); got != "hello" {
		t.Fatalf("ExtractedText() = %q, want hello", got)
	}
}

func TestTraditionalOCRPayloadPagesItems(t *testing.T) {
	raw := `{
		"pages": [
			{"page": 1, "text": "first"},
			{"page_number": 2, "markdown": "second"}
		]
	}`
	var payload traditionalOCRPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if got := payload.PageCount(); got != 2 {
		t.Fatalf("PageCount() = %d, want 2", got)
	}

	pageTexts := payload.ExtractedPageTexts()
	if len(pageTexts) != 2 {
		t.Fatalf("len(ExtractedPageTexts()) = %d, want 2", len(pageTexts))
	}
	if pageTexts[0].PageNumber != 1 || pageTexts[0].Text != "first" {
		t.Fatalf("pageTexts[0] = %+v, want page 1 first", pageTexts[0])
	}
	if pageTexts[1].PageNumber != 2 || pageTexts[1].Text != "second" {
		t.Fatalf("pageTexts[1] = %+v, want page 2 second", pageTexts[1])
	}
}
