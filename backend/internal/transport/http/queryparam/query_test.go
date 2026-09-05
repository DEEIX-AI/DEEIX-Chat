package queryparam

import "testing"

func TestOptionalBool(t *testing.T) {
	if value := OptionalBool("true"); value == nil || !*value {
		t.Fatalf("OptionalBool(true) = %v, want true", value)
	}
	if value := OptionalBool("false"); value == nil || *value {
		t.Fatalf("OptionalBool(false) = %v, want false", value)
	}
	for _, raw := range []string{"", "invalid", " true "} {
		if value := OptionalBool(raw); value != nil {
			t.Fatalf("OptionalBool(%q) = %v, want nil", raw, *value)
		}
	}
}
