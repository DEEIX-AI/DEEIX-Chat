package contentmoderation

import "testing"

func TestEvaluateHitIgnoresTopLevelFlagged(t *testing.T) {
	resp := &Response{Results: []CategoryResult{{
		Flagged: true,
		Categories: map[string]bool{
			"hate":     false,
			"violence": false,
		},
		CategoryScores: map[string]float64{"hate": 0.9},
	}}}
	if eval := EvaluateHit(resp, []string{"hate", "violence"}, ModalityText); eval.Hit {
		t.Fatal("expected no hit when selected categories are false")
	}
}

func TestEvaluateHitSelectedCategory(t *testing.T) {
	resp := &Response{Results: []CategoryResult{{
		Categories: map[string]bool{"violence": true, "hate": true},
		CategoryScores: map[string]float64{
			"violence": 0.8,
			"hate":     0.7,
		},
		CategoryAppliedInputTypes: map[string][]string{
			"violence": {"text"},
			"hate":     {"text"},
		},
	}}}
	eval := EvaluateHit(resp, []string{"violence"}, ModalityText)
	if !eval.Hit || len(eval.Categories) != 1 || eval.Categories[0] != "violence" {
		t.Fatalf("unexpected evaluation: %#v", eval)
	}
}

func TestNormalizeImageCategoriesRejectsTextOnly(t *testing.T) {
	if _, err := NormalizePolicy(Policy{InputImageCategories: []string{"hate"}}); err == nil {
		t.Fatal("expected text-only category rejection")
	}
}
