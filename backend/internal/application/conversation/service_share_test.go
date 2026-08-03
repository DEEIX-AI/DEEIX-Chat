package conversation

import (
	"context"
	"reflect"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
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

// 克隆共享会话时逐字段手写赋值，漏字段的后果与祖先链 CTE 漏列相同：
// 克隆出的会话可继续对话，历史 assistant 消息却没有推理内容，回传形同虚设。
type cloneSharedMessageRepositoryStub struct {
	repository.ConversationRepository
	created []model.Message
}

func (s *cloneSharedMessageRepositoryStub) CreateMessage(_ context.Context, message *model.Message) error {
	message.ID = uint(len(s.created) + 1)
	s.created = append(s.created, *message)
	return nil
}

func TestCloneSharedMessagePreservesReasoningContent(t *testing.T) {
	repo := &cloneSharedMessageRepositoryStub{}
	service := &Service{repo: repo}

	source := model.Message{
		PublicID:         "a1",
		Role:             "assistant",
		ContentType:      "text",
		Content:          "答复",
		ReasoningContent: "历史推理内容",
		ReasoningTokens:  125,
		Status:           "success",
	}

	cloned, err := service.cloneSharedMessage(context.Background(), 1, 2, source, "run_clone", map[string]uint{})
	if err != nil {
		t.Fatalf("cloneSharedMessage() error = %v", err)
	}
	if cloned.ReasoningContent != "历史推理内容" {
		t.Fatalf("cloned reasoning content = %q, want preserved", cloned.ReasoningContent)
	}
	// reasoning_tokens 一直被复制，若 reasoning_content 丢失会造成行内自相矛盾。
	if cloned.ReasoningTokens != 125 {
		t.Fatalf("cloned reasoning tokens = %d, want 125", cloned.ReasoningTokens)
	}
	if len(repo.created) != 1 || repo.created[0].ReasoningContent != "历史推理内容" {
		t.Fatalf("persisted row lost reasoning content: %#v", repo.created)
	}
}
