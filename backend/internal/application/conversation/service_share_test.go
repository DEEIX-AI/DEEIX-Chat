package conversation

import (
	"reflect"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestSharedMessagesIncludeFileUsesSnapshotAttachments(t *testing.T) {
	messages := []model.Message{
		{ID: 1, PublicID: "u1", Role: "user", Attachments: `[{"file_id":"file_a"}]`},
		{ID: 2, PublicID: "a1", ParentPublicID: "u1", Role: "assistant", Attachments: `[{"file_id":"file_b"}]`},
	}

	if !sharedMessagesIncludeFile(messages, "file_a") {
		t.Fatal("expected file_a to be included")
	}
	if !sharedMessagesIncludeFile(messages, "file_b") {
		t.Fatal("expected file_b to be included")
	}
	if sharedMessagesIncludeFile(messages, "file_c") {
		t.Fatal("did not expect file_c to be included")
	}
}

func TestResolvePublicDefaultMessageIDsUsesStoredPath(t *testing.T) {
	messages := []model.Message{
		{ID: 1, PublicID: "u1", Role: "user"},
		{ID: 2, PublicID: "a1", ParentPublicID: "u1", Role: "assistant"},
		{ID: 3, PublicID: "u2-old", ParentPublicID: "a1", Role: "user"},
		{ID: 4, PublicID: "a2-old", ParentPublicID: "u2-old", Role: "assistant"},
		{ID: 5, PublicID: "u2-new", ParentPublicID: "a1", Role: "user"},
		{ID: 6, PublicID: "a2-new", ParentPublicID: "u2-new", Role: "assistant"},
	}

	got := resolvePublicDefaultMessageIDs(`["u1","a1","u2-old","a2-old"]`, messages)
	want := []string{"u1", "a1", "u2-old", "a2-old"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default branch mismatch: got %v, want %v", got, want)
	}
}

func TestResolvePublicDefaultMessageIDsFallsBackToLatestBranch(t *testing.T) {
	messages := []model.Message{
		{ID: 1, PublicID: "u1", Role: "user"},
		{ID: 2, PublicID: "a1", ParentPublicID: "u1", Role: "assistant"},
		{ID: 3, PublicID: "u2-old", ParentPublicID: "a1", Role: "user"},
		{ID: 4, PublicID: "a2-old", ParentPublicID: "u2-old", Role: "assistant"},
		{ID: 5, PublicID: "u2-new", ParentPublicID: "a1", Role: "user"},
		{ID: 6, PublicID: "a2-new", ParentPublicID: "u2-new", Role: "assistant"},
	}

	got := resolvePublicDefaultMessageIDs("", messages)
	want := []string{"u1", "a1", "u2-new", "a2-new"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fallback branch mismatch: got %v, want %v", got, want)
	}
}

func TestOrderSharedMessagesForCloneMakesDefaultBranchLatest(t *testing.T) {
	messages := []model.Message{
		{ID: 1, PublicID: "u1", Role: "user"},
		{ID: 2, PublicID: "a1", ParentPublicID: "u1", Role: "assistant"},
		{ID: 3, PublicID: "u2-old", ParentPublicID: "a1", Role: "user"},
		{ID: 4, PublicID: "a2-old", ParentPublicID: "u2-old", Role: "assistant"},
		{ID: 5, PublicID: "u2-new", ParentPublicID: "a1", Role: "user"},
		{ID: 6, PublicID: "a2-new", ParentPublicID: "u2-new", Role: "assistant"},
	}

	ordered := orderSharedMessagesForClone(messages, []string{"u1", "a1", "u2-old", "a2-old"})
	got := make([]string, 0, len(ordered))
	for _, message := range ordered {
		got = append(got, message.PublicID)
	}
	want := []string{"u1", "a1", "u2-new", "a2-new", "u2-old", "a2-old"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clone order mismatch: got %v, want %v", got, want)
	}
}

func TestSanitizeSharedTracePayloadJSONRemovesInternalFields(t *testing.T) {
	got := sanitizeSharedTracePayloadJSON(`{
		"tool_calls": [{"tool_call_id":"call_1","output":"ok"}],
		"upstream_debug": {"authorization":"Bearer token"},
		"upstream": {"name":"hidden","model":"visible"},
		"api_key": "secret"
	}`)
	want := `{"tool_calls":[{"output":"ok","tool_call_id":"call_1"}],"upstream":{"model":"visible"}}`
	if got != want {
		t.Fatalf("sanitized payload mismatch: got %s, want %s", got, want)
	}
}

func TestNormalizeMessagePublicIDsDeduplicatesAndKeepsOrder(t *testing.T) {
	got := normalizeMessagePublicIDs([]string{"", " msg_a ", "msg_b", "msg_a", "\n"})
	want := []string{"msg_a", "msg_b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized ids mismatch: got %v, want %v", got, want)
	}
}
