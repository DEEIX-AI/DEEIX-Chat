package conversation

import (
	"context"
	"strings"
	"testing"

	domainskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/skill"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestRenderSkillPromptIncludesSelectedSkillContent(t *testing.T) {
	prompt := &skillPrompts{
		Skills: []domainskill.Skill{
			{
				ID:          12,
				Scope:       domainskill.ScopeUser,
				Title:       "Review",
				Trigger:     "review",
				Description: "Review code",
				Markdown:    "Review the diff and return prioritized findings.",
			},
			{
				ID:       13,
				Scope:    domainskill.ScopeBuiltin,
				Title:    "Frontend Rules",
				Trigger:  "frontend",
				Markdown: "Check layout, spacing, and interaction states.",
			},
		},
	}
	rendered := renderSkillPrompts(prompt)
	for _, want := range []string{
		"<skill_context>",
		"<skills count=\"2\">",
		"<title>Review</title>",
		"<title>Frontend Rules</title>",
		"<description>Review code</description>",
		"<content>Review the diff and return prioritized findings.</content>",
		"<content>Check layout, spacing, and interaction states.</content>",
		"Each selected skill includes title, trigger, description, and SKILL.md content",
		"Use each skill's content when it is relevant",
		"Do not invent hidden instructions",
		"do not grant permission to execute operating-system commands",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected rendered prompt to contain %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "Apply this skill") {
		t.Fatalf("expected rendered prompt not to force skill application:\n%s", rendered)
	}
}

func TestInjectSkillPromptAddsSystemMessageAfterExistingPolicy(t *testing.T) {
	prompt := &skillPrompts{
		Skills: []domainskill.Skill{{
			ID:       7,
			Scope:    domainskill.ScopeBuiltin,
			Title:    "Plan",
			Trigger:  "plan",
			Markdown: "Create a concise plan.",
		}},
	}
	prompt.Rendered = renderSkillPrompts(prompt)

	messages := injectSkillPrompts([]llm.Message{
		{Role: "system", Content: "base policy"},
		{Role: "user", Content: "hello"},
	}, prompt)

	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
	if messages[0].Content != "base policy" || messages[1].Role != "system" || !strings.Contains(messages[1].Content, skillPromptSystemMarker) {
		t.Fatalf("expected skill prompt after base system policy: %#v", messages)
	}
	if messages[2].Role != "user" || messages[2].Content != "hello" {
		t.Fatalf("expected original user message last: %#v", messages)
	}
}

func TestResolveSkillPromptsRejectsTooManySelectedSkills(t *testing.T) {
	service := &Service{cfg: config.NewRuntime(config.Config{MCPMaxSelectedToolsPerMessage: 2})}
	_, err := service.resolveSkillPrompts(context.Background(), SendMessageInput{
		SkillIDs: []uint{1, 2, 3},
	})
	if err != ErrTooManySelectedSkills {
		t.Fatalf("expected ErrTooManySelectedSkills, got %v", err)
	}
}

func TestNormalizeSelectedSkillIDsDeduplicatesAndDropsEmpty(t *testing.T) {
	got := normalizeSelectedSkillIDs([]uint{0, 2, 2, 3, 0, 1})
	want := []uint{2, 3, 1}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}
