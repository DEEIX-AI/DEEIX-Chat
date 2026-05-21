package conversation

import "testing"

func TestTranslateErrorAllowsNil(t *testing.T) {
	if err := translateError(nil); err != nil {
		t.Fatalf("translateError(nil) = %v, want nil", err)
	}
}
