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
	if len(cfg.Rounds) != 2 || cfg.Rounds[0].Index != 1 || !cfg.Rounds[0].Parallel {
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
