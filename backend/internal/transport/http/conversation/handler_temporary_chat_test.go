package conversation

import "testing"

func TestTemporaryChatSessionHashDoesNotExposeSessionID(t *testing.T) {
	const sessionID = "private-session-id"
	first := temporaryChatSessionHash(sessionID)
	second := temporaryChatSessionHash(sessionID)
	if first == "" || first == sessionID {
		t.Fatalf("hash must be non-empty and opaque: %q", first)
	}
	if first != second {
		t.Fatalf("hash must be stable: %q != %q", first, second)
	}
	if len(first) != 32 {
		t.Fatalf("hash length = %d, want 32", len(first))
	}
}
