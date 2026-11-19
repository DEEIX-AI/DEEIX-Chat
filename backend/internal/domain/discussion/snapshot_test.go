package discussion

import (
	"errors"
	"reflect"
	"testing"
)

func filterTestPlan() RuntimePlan {
	return RuntimePlan{
		Rounds: []RuntimeRound{
			{Index: 1, Steps: []RuntimeStep{{ID: "r1s1", Participant: "p1", Model: "model-a"}, {ID: "r1s2", Participant: "p2", Model: "model-b"}}},
			{Index: 2, Steps: []RuntimeStep{{ID: "r2s1", Participant: "p2", Model: "model-b"}}},
			{Index: 3, Steps: []RuntimeStep{{ID: "r3s1", Participant: "p1", Model: "model-a"}}},
		},
		Summary: RuntimeSummary{Participant: "judge", Model: "model-judge"},
	}
}

func TestFilterDisabledModelsSkipsEmptyRoundAndKeepsIndexes(t *testing.T) {
	plan := filterTestPlan()
	filtered, err := FilterDisabledModels(plan, []string{"p2"})
	if err != nil {
		t.Fatalf("FilterDisabledModels: %v", err)
	}
	if len(filtered.Rounds) != 2 || filtered.Rounds[0].Index != 1 || filtered.Rounds[1].Index != 3 {
		t.Fatalf("unexpected rounds: %#v", filtered.Rounds)
	}
	if len(filtered.Rounds[0].Steps) != 1 || filtered.Rounds[0].Steps[0].Participant != "p1" {
		t.Fatalf("disabled participant was not filtered: %#v", filtered.Rounds[0].Steps)
	}
	if filtered.Summary.Model != "model-judge" {
		t.Fatalf("summary changed: %#v", filtered.Summary)
	}
	if len(plan.Rounds) != 3 || len(plan.Rounds[0].Steps) != 2 {
		t.Fatal("filter mutated the full snapshot plan")
	}
}

func TestFilterDisabledModelsRejectsEmptyExecutionAndSummary(t *testing.T) {
	plan := filterTestPlan()
	if _, err := FilterDisabledModels(plan, []string{"p1", "p2"}); !errors.Is(err, ErrNoEnabledModels) {
		t.Fatalf("expected no enabled models, got %v", err)
	}
	if _, err := FilterDisabledModels(plan, []string{"judge"}); !errors.Is(err, ErrSummaryModelLocked) {
		t.Fatalf("expected summary lock, got %v", err)
	}
}

func TestPlanBindingsIncludesSummaryAndMergeOverrides(t *testing.T) {
	plan := filterTestPlan()
	bindings := PlanBindings(plan)
	want := map[string]string{"p1": "model-a", "p2": "model-b", "judge": "model-judge"}
	if !reflect.DeepEqual(bindings, want) {
		t.Fatalf("bindings = %#v, want %#v", bindings, want)
	}
	merged := MergeBindings(plan, map[string]string{"p2": " model-new ", "p1": ""})
	if merged["p2"] != "model-new" || merged["p1"] != "model-a" || merged["judge"] != "model-judge" {
		t.Fatalf("unexpected merged bindings: %#v", merged)
	}
}

func TestSummaryRoundIndexUsesOriginalRoundNumbers(t *testing.T) {
	plan := filterTestPlan()
	if _, err := FilterDisabledModels(plan, []string{"p1", "p2"}); !errors.Is(err, ErrNoEnabledModels) {
		t.Fatalf("unexpected filter result: %v", err)
	}
	filtered, err := FilterDisabledModels(plan, []string{"p2"})
	if err != nil {
		t.Fatal(err)
	}
	if got := SummaryRoundIndex(filtered); got != 4 {
		t.Fatalf("summary round = %d, want 4", got)
	}
}
