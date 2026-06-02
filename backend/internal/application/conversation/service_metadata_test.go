package conversation

import (
	"strings"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestBuildConversationMetadataMessagesTruncatesToBudget(t *testing.T) {
	userMsg := model.Message{Content: strings.Repeat("用户输入内容", 6000)}
	assistantMsg := model.Message{Content: strings.Repeat("助手回复内容", 6000)}

	got := buildConversationMetadataMessages(userMsg, assistantMsg)

	if tokens := estimateTokens(got); tokens > conversationMetadataMessageMaxTokens {
		t.Fatalf("metadata messages exceeded budget: got %d, want <= %d", tokens, conversationMetadataMessageMaxTokens)
	}
	if !strings.HasPrefix(got, "user:\n") {
		previewEnd := 32
		if len(got) < previewEnd {
			previewEnd = len(got)
		}
		t.Fatalf("expected metadata messages to keep leading user content, got %q", got[:previewEnd])
	}
	if !strings.Contains(got, "[truncated]") {
		t.Fatal("expected metadata messages to mark truncated content")
	}
}

func TestParseGeneratedConversationTitleHandlesLooseJSON(t *testing.T) {
	cases := map[string]string{
		`{"title":"项目协作规范说明文档"}`:                       "项目协作规范说明文档",
		"```markdown\n{\"title\":\"项目协作规范说明文档\"}\n```": "项目协作规范说明文档",
		"```json\n{\"title\":\"项目协作规范说明文档\"}\n```":     "项目协作规范说明文档",
		`{"title": 项目协作规范说明文档}`:                        "项目协作规范说明文档",
		`{title: 项目协作规范说明文档}`:                          "项目协作规范说明文档",
	}
	for raw, want := range cases {
		got := sanitizeGeneratedConversationTitle(parseGeneratedConversationTitle(raw))
		if got != want {
			t.Fatalf("unexpected title for %q: got %q, want %q", raw, got, want)
		}
	}
}

func TestParseGeneratedConversationTitleRejectsDirtyOutput(t *testing.T) {
	cases := []string{
		`title: 项目协作规范说明文档`,
		`这是标题：项目协作规范说明文档`,
		`标题如下：{"title": 项目协作规范说明文档}`,
		`{"subtitle": 项目协作规范说明文档}`,
	}
	for _, raw := range cases {
		if got := sanitizeGeneratedConversationTitle(parseGeneratedConversationTitle(raw)); got != "" {
			t.Fatalf("expected dirty title output to be rejected for %q, got %q", raw, got)
		}
	}
}

func TestConversationTitleFromFirstUserMessage(t *testing.T) {
	cases := map[string]string{
		"  这是一条很长的第一条用户消息，用来测试标题截断  ":        "这是一条很长的第一条用户消息，用来测试标",
		"\n\nhello   world   from   DEEIX\n": "hello world from DEE",
		"\"简短标题\"":                           "简短标题",
		"   ":                                "",
	}
	for input, want := range cases {
		if got := conversationTitleFromFirstUserMessage(input); got != want {
			t.Fatalf("unexpected first-message title for %q: got %q, want %q", input, got, want)
		}
	}
}

func TestShouldGenerateConversationMetadataAfterFailedFirstTurn(t *testing.T) {
	conversation := model.Conversation{
		Title:        "新会话",
		LabelsJSON:   "[]",
		MessageCount: 2,
	}

	if !shouldGenerateConversationMetadata(conversation) {
		t.Fatal("expected placeholder metadata to be generated even when failed messages already exist")
	}
}

func TestConversationLabelsEmpty(t *testing.T) {
	emptyCases := []string{"", "null", "[]", "  []  "}
	for _, value := range emptyCases {
		if !conversationLabelsEmpty(value) {
			t.Fatalf("expected labels %q to be empty", value)
		}
	}
	if conversationLabelsEmpty(`["技术"]`) {
		t.Fatal("expected non-empty labels to be preserved")
	}
}
