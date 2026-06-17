package skill

import (
	"encoding/json"
	"strings"
	"testing"

	domainskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/skill"
)

func TestSkillSummaryResponseDoesNotExposeMarkdown(t *testing.T) {
	payload, err := json.Marshal(toSkillSummaryResponse(domainskill.Skill{
		ID:          1,
		Scope:       domainskill.ScopeBuiltin,
		Title:       "Review",
		Trigger:     "review",
		Description: "Review code",
		Markdown:    "private SKILL.md",
		Enabled:     true,
	}))
	if err != nil {
		t.Fatalf("marshal summary response: %v", err)
	}
	text := string(payload)
	if strings.Contains(text, "markdown") || strings.Contains(text, "private SKILL.md") {
		t.Fatalf("summary response exposed markdown: %s", text)
	}
}

func TestSkillResponseExposesMarkdownForDetail(t *testing.T) {
	payload, err := json.Marshal(toSkillResponse(domainskill.Skill{
		ID:       1,
		Scope:    domainskill.ScopeBuiltin,
		Title:    "Review",
		Trigger:  "review",
		Markdown: "private SKILL.md",
		Enabled:  true,
	}))
	if err != nil {
		t.Fatalf("marshal detail response: %v", err)
	}
	text := string(payload)
	if !strings.Contains(text, `"markdown":"private SKILL.md"`) {
		t.Fatalf("detail response did not expose markdown: %s", text)
	}
}
