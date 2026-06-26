package conversation

import (
	"errors"
	"strings"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestBuildConversationMetadataMessagesTruncatesToBudget(t *testing.T) {
	userMsg := model.Message{Content: strings.Repeat("用户输入内容", 6000)}

	got := buildConversationMetadataMessages(userMsg)

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

func TestBuildConversationMetadataMessagesUsesOnlyUserMessage(t *testing.T) {
	userMsg := model.Message{Content: "帮我设计一套灰度发布方案"}

	got := buildConversationMetadataMessages(userMsg)

	if !strings.Contains(got, "user:\n帮我设计一套灰度发布方案") {
		t.Fatalf("expected metadata messages to include user bubble, got %q", got)
	}
	if strings.Contains(got, "assistant:") {
		t.Fatalf("expected metadata messages to exclude assistant content, got %q", got)
	}
}

func TestParseGeneratedConversationTitleHandlesLooseJSON(t *testing.T) {
	cases := map[string]string{
		`{"title":"项目协作规范说明文档"}`:                       "项目协作规范说明文档",
		"```markdown\n{\"title\":\"项目协作规范说明文档\"}\n```": "项目协作规范说明文档",
		"```json\n{\"title\":\"项目协作规范说明文档\"}\n```":     "项目协作规范说明文档",
		`{"title": 项目协作规范说明文档}`:                        "项目协作规范说明文档",
		`{title: 项目协作规范说明文档}`:                          "项目协作规范说明文档",
		`标题如下：{ "title": "项目协作规范说明文档" }`:               "项目协作规范说明文档",
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
		`{"subtitle": 项目协作规范说明文档}`,
	}
	for _, raw := range cases {
		if got := sanitizeGeneratedConversationTitle(parseGeneratedConversationTitle(raw)); got != "" {
			t.Fatalf("expected dirty title output to be rejected for %q, got %q", raw, got)
		}
	}
}

func TestParseGeneratedConversationLabelsHandlesLooseJSON(t *testing.T) {
	cases := map[string][]string{
		`{"labels":["技术","运维"]}`:                     {"技术", "运维"},
		"```json\n{\"labels\":[\"技术\",\"运维\"]}\n```": {"技术", "运维"},
		`标签如下：{ "labels": ["技术", "运维"] }`:            {"技术", "运维"},
		`{labels: [技术, 运维]}`:                         {"技术", "运维"},
		`{tags: ["技术", "运维"]}`:                       {"技术", "运维"},
	}
	for raw, want := range cases {
		got := sanitizeGeneratedConversationLabels(parseGeneratedConversationLabels(raw))
		if len(got) != len(want) {
			t.Fatalf("unexpected labels length for %q: got %#v, want %#v", raw, got, want)
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("unexpected labels for %q: got %#v, want %#v", raw, got, want)
			}
		}
	}
}

func TestConversationTitleFromFirstUserMessage(t *testing.T) {
	cases := map[string]string{
		"  这是一条很长的第一条用户消息，用来测试标题截断  ":        "这是一条很长的第一条用户消息，用",
		"\n\nhello   world   from   DEEIX\n": "hello world from",
		"\"简短标题\"":                           "简短标题",
		"   ":                                "",
	}
	for input, want := range cases {
		if got := conversationTitleFromFirstUserMessage(input); got != want {
			t.Fatalf("unexpected first-message title for %q: got %q, want %q", input, got, want)
		}
	}
}

func TestConversationFallbackTitleUsesUnifiedLimit(t *testing.T) {
	if conversationFallbackTitleMaxRunes != 16 {
		t.Fatalf("expected fallback title limit to stay unified at 16, got %d", conversationFallbackTitleMaxRunes)
	}

	got := conversationTitleFromFirstUserMessage("0123456789abcdefXYZ")
	if got != "0123456789abcdef" {
		t.Fatalf("expected fallback title to truncate to 16 runes, got %q", got)
	}
}

func TestBuildConversationMetadataMessagesEmptyWhenNoText(t *testing.T) {
	got := buildConversationMetadataMessages(model.Message{})
	if got != "" {
		t.Fatalf("expected no metadata prompt body for empty messages, got %q", got)
	}
}

func TestConversationMetadataRefreshHint(t *testing.T) {
	cases := []struct {
		name         string
		conversation model.Conversation
		userMsg      model.Message
		want         string
	}{
		{
			name:         "not needed when title and labels already exist",
			conversation: model.Conversation{Title: "已有标题", LabelsJSON: `["技术"]`},
			userMsg:      model.Message{Content: "新的问题"},
			want:         conversationMetadataRefreshNotNeeded,
		},
		{
			name:         "skip when no titleable text",
			conversation: model.Conversation{Title: "新会话", LabelsJSON: "[]"},
			want:         conversationMetadataRefreshNoContent,
		},
		{
			name:         "pending when metadata needed and text exists",
			conversation: model.Conversation{Title: "新会话", LabelsJSON: "[]"},
			userMsg:      model.Message{Content: "帮我整理本周项目计划"},
			want:         conversationMetadataRefreshPending,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := conversationMetadataRefreshHint(tc.conversation, tc.userMsg)
			if got != tc.want {
				t.Fatalf("unexpected metadata refresh hint: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildConversationTitleMessagesUsesCompletedTranscript(t *testing.T) {
	messages := []model.Message{
		{Role: "system", Content: "系统提示词"},
		{Role: "user", Content: "第一轮问题", Status: "completed"},
		{Role: "assistant", Content: "第一轮回答", Status: "completed"},
		{Role: "assistant", Content: "还在生成的回答", Status: "pending"},
		{Role: "tool", Content: "工具结果", Status: "completed"},
		{Role: "user", Content: "后续目标变化", Status: "completed"},
	}

	got := buildConversationTitleMessages(messages)

	if strings.Contains(got, "系统提示词") || strings.Contains(got, "工具结果") || strings.Contains(got, "还在生成的回答") {
		t.Fatalf("expected title messages to include only completed user/assistant transcript, got %q", got)
	}
	if !strings.Contains(got, "user:\n第一轮问题") || !strings.Contains(got, "assistant:\n第一轮回答") || !strings.Contains(got, "user:\n后续目标变化") {
		t.Fatalf("expected title messages to keep completed conversation content, got %q", got)
	}
}

func TestBuildConversationTitleMessagesPrioritizesLatestTranscript(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: strings.Repeat("很早以前的问题", 6000), Status: "completed"},
		{Role: "assistant", Content: "很早以前的回答", Status: "completed"},
		{Role: "user", Content: "最新目标是重新整理订阅方案", Status: "completed"},
		{Role: "assistant", Content: "围绕最新目标继续分析", Status: "completed"},
	}

	got := buildConversationTitleMessages(messages)

	if strings.Contains(got, "很早以前的问题") {
		t.Fatalf("expected title messages to drop oldest content when over budget, got %q", got)
	}
	if !strings.Contains(got, "最新目标是重新整理订阅方案") || !strings.Contains(got, "围绕最新目标继续分析") {
		t.Fatalf("expected title messages to keep latest transcript, got %q", got)
	}
}

func TestConversationTitleFromMessagesPrefersLatestUserMessage(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "早期主题是部署配置", Status: "completed"},
		{Role: "assistant", Content: "助手先说了一段话", Status: "completed"},
		{Role: "user", Content: "最新主题是订阅方案", Status: "completed"},
	}

	if got := conversationTitleFromMessages(messages); got != "最新主题是订阅方案" {
		t.Fatalf("expected fallback title from latest user message, got %q", got)
	}
}

func TestConversationMetadataFallsBackToFirstUserMessageTitle(t *testing.T) {
	resolvedTitle := resolveConversationMetadataTitle(
		shouldAutoReplaceConversationTitle("新会话"),
		"",
		"设置为跟随后，Grok 4.3 对话标题没有自动生成",
	)
	if resolvedTitle == "" || resolvedTitle == "新会话" {
		t.Fatalf("expected first user message fallback title, got %q", resolvedTitle)
	}
}

func TestShouldAutoReplaceConversationTitleIncludesEnglishNewChat(t *testing.T) {
	if !shouldAutoReplaceConversationTitle("New chat") {
		t.Fatal("expected English localized new chat title to be replaceable")
	}
	if shouldAutoReplaceConversationTitle("新对话") {
		t.Fatal("expected non-persisted Chinese navigation label not to be replaceable")
	}
}

func TestConversationMetadataErrorDoesNotLeakWhenEitherTaskSucceeds(t *testing.T) {
	titleErr := errors.New("title failed")
	labelsErr := errors.New("labels failed")

	if err := resolveConversationMetadataError("有效标题", "", nil, labelsErr); err != nil {
		t.Fatalf("expected labels error not to fail metadata when title exists, got %v", err)
	}
	if err := resolveConversationMetadataError("", `["技术"]`, titleErr, nil); err != nil {
		t.Fatalf("expected title error not to fail metadata when labels exist, got %v", err)
	}
	if err := resolveConversationMetadataError("", "", titleErr, labelsErr); !errors.Is(err, titleErr) {
		t.Fatalf("expected first task error when nothing is generated, got %v", err)
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
