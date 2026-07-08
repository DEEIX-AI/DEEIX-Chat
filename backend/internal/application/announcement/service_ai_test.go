package announcement

import "testing"

func TestParseAnnouncementDraftExtractsFencedJSON(t *testing.T) {
	raw := "```json\n{\"title\":\"系统维护通知\",\"contentMarkdown\":\"请注意以下安排：\\n\\n- 维护期间部分功能可能短暂不可用\\n- 完成后无需重新登录\",\"type\":\"warning\",\"pinned\":true,\"priority\":60}\n```"
	got, err := parseAnnouncementDraft(raw)
	if err != nil {
		t.Fatalf("parse draft: %v", err)
	}
	if got.Title != "系统维护通知" || got.Type != "warning" || !got.Pinned || got.Priority != 60 {
		t.Fatalf("unexpected draft: %#v", got)
	}
}

func TestParseAnnouncementDraftNormalizesOptionalFields(t *testing.T) {
	raw := `{"title":"Update","contentMarkdown":"- One\n- Two","type":"unknown","pinned":false,"priority":999}`
	got, err := parseAnnouncementDraft(raw)
	if err != nil {
		t.Fatalf("parse draft: %v", err)
	}
	if got.Type != "general" {
		t.Fatalf("expected invalid type to fall back to general, got %q", got.Type)
	}
	if got.Priority != 100 {
		t.Fatalf("expected priority to be clamped, got %d", got.Priority)
	}
}
