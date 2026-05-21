package user

import "testing"

func TestTranslateErrorNil(t *testing.T) {
	if err := translateError(nil); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
