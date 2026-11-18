package discussion

import (
	"strings"
	"testing"
)

func TestParseConfigAndResolveRuntimePlan(t *testing.T) {
	raw := `{
		"placeholders": [
			{"key": "p1", "label": "A", "defaultModel": "model-a"},
			{"key": "p2", "label": "B", "defaultModel": ""},
			{"key": "judge", "label": "Judge", "defaultModel": "model-a"}
		],
		"rounds": [
			{"steps": [
				{"participant": "p1", "prompt": "first"},
				{"participant": "p2", "prompt": "second"}
			]},
			{"steps": [{"participant": "p1", "prompt": "follow up"}]}
		],
		"summary": {"participant": "judge", "prompt": "summarize"}
	}`
	cfg, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if len(cfg.Rounds) != 2 || cfg.Rounds[0].Index != 1 || !cfg.Rounds[0].IsParallel() {
		t.Fatalf("unexpected rounds: %+v", cfg.Rounds)
	}
	if cfg.Rounds[0].Steps[0].ID != "r1s1" {
		t.Fatalf("expected generated step id, got %q", cfg.Rounds[0].Steps[0].ID)
	}

	plan, err := ResolveRuntimePlan(cfg, map[string]string{"p2": "model-b"})
	if err != nil {
		t.Fatalf("ResolveRuntimePlan: %v", err)
	}
	if plan.Rounds[0].Steps[0].Model != "model-a" {
		t.Fatalf("expected placeholder default model, got %q", plan.Rounds[0].Steps[0].Model)
	}
	if plan.Rounds[0].Steps[1].Model != "model-b" {
		t.Fatalf("expected runtime override, got %q", plan.Rounds[0].Steps[1].Model)
	}
	if plan.Summary.Model != "model-a" {
		t.Fatalf("expected summary default model, got %q", plan.Summary.Model)
	}
}

func TestResolveRuntimePlanSerialRoundUsesPlaceholderOrder(t *testing.T) {
	cfg, err := ParseConfig(`{
		"placeholders": [
			{"key": "p1", "defaultModel": "model-a"},
			{"key": "p2", "defaultModel": "model-b"},
			{"key": "p3", "defaultModel": "model-c"},
			{"key": "judge", "defaultModel": "model-a"}
		],
		"rounds": [{"parallel": false, "steps": [
			{"participant": "p3"},
			{"participant": "p1"},
			{"participant": "p2"}
		]}],
		"summary": {"participant": "judge"}
	}`)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if cfg.Rounds[0].IsParallel() {
		t.Fatal("explicit serial round was overwritten")
	}
	plan, err := ResolveRuntimePlan(cfg, nil)
	if err != nil {
		t.Fatalf("ResolveRuntimePlan: %v", err)
	}
	if plan.Rounds[0].IsParallel() {
		t.Fatal("runtime plan lost serial mode")
	}
	for index, participant := range []string{"p1", "p2", "p3"} {
		if plan.Rounds[0].Steps[index].Participant != participant {
			t.Fatalf("runtime steps are not in model order: %#v", plan.Rounds[0].Steps)
		}
	}
}

func TestResolveRuntimePlanUsesModeSpecificPrompt(t *testing.T) {
	cfg, err := ParseConfig(`{
		"placeholders": [
			{"key": "p1", "defaultModel": "model-a"},
			{"key": "judge", "defaultModel": "model-a"}
		],
		"rounds": [
			{"parallel": true, "parallelPrompt": "parallel first", "serialPrompt": "serial first", "steps": [{"participant": "p1", "prompt": "legacy first"}]},
			{"parallel": false, "parallelPrompt": "parallel second", "serialPrompt": "serial second", "steps": [{"participant": "p1", "prompt": "legacy second"}]},
			{"parallel": true, "steps": [{"participant": "p1", "prompt": "legacy third"}]}
		],
		"summary": {"participant": "judge"}
	}`)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	plan, err := ResolveRuntimePlan(cfg, nil)
	if err != nil {
		t.Fatalf("ResolveRuntimePlan: %v", err)
	}
	cases := []struct {
		round  int
		prompt string
	}{
		{0, "parallel first"},
		{1, "serial second"},
		{2, "legacy third"},
	}
	for _, item := range cases {
		if got := plan.Rounds[item.round].Steps[0].Prompt; got != item.prompt {
			t.Fatalf("round %d prompt = %q, want %q", item.round+1, got, item.prompt)
		}
	}
}

func TestResolveRuntimePlanRequiresEveryModel(t *testing.T) {
	cfg, err := ParseConfig(`{
		"placeholders": [{"key": "p1", "label": "A"}],
		"rounds": [{"steps": [{"participant": "p1"}]}],
		"summary": {"participant": "p1"}
	}`)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	_, err = ResolveRuntimePlan(cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "model is not selected") {
		t.Fatalf("expected missing model error, got %v", err)
	}
}

func TestValidateConfigRejectsUnknownParticipant(t *testing.T) {
	_, err := ParseConfig(`{
		"placeholders": [{"key": "p1", "label": "A"}],
		"rounds": [{"steps": [{"participant": "missing"}]}],
		"summary": {"participant": "p1"}
	}`)
	if err == nil {
		t.Fatal("expected invalid participant error")
	}
}
