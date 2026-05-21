package conversation

import (
	"strings"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestCollectConversationFileIDsIgnoresFailedHistoricalMessages(t *testing.T) {
	messages := []model.Message{
		{
			Status:      "success",
			Attachments: `[{"file_id":"file_success"}]`,
		},
		{
			Status:      "error",
			Attachments: `[{"file_id":"file_failed"}]`,
		},
	}

	got := collectConversationFileIDs(messages, []string{"file_current"})
	want := []string{"file_success", "file_current"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestInjectUserContextUsesCompactXMLForRAG(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "怎么发布？"}}
	chunks := []model.RAGChunk{{
		FileName:   "AGENTS.md",
		ChunkIndex: 3,
		Content:    "Run pnpm build.",
	}}

	got := injectUserContext(t.Context(), messages, userContextInput{RAGChunks: chunks}, config.Config{}, nil)
	for _, want := range []string{"<ctx>", "<rag>", `<doc name="AGENTS.md" i="3">Run pnpm build.</doc>`, "</ctx>", "<q>怎么发布？</q>"} {
		if !strings.Contains(got[0].Content, want) {
			t.Fatalf("expected RAG XML to contain %q, got %q", want, got[0].Content)
		}
	}
	if strings.Contains(got[0].Content, "<files>") {
		t.Fatalf("did not expect files section for RAG-only context, got %q", got[0].Content)
	}
}

func TestPrependStableFileContextKeepsFilesAtPromptTop(t *testing.T) {
	messages := []llm.Message{
		{Role: "user", Content: "第一轮问题"},
		{Role: "assistant", Content: "第一轮回答"},
		{Role: "user", Content: "继续修改上一轮回答"},
	}
	attachments := []AttachmentInput{
		{
			FileID:        "b",
			FileName:      "B.md",
			FileCategory:  "document",
			ExtractedText: "second file",
		},
		{
			FileID:        "a",
			FileName:      "A.md",
			FileCategory:  "document",
			ExtractedText: "first file",
		},
	}

	got := prependStableFileContext(messages, attachments)
	if len(got) != len(messages)+1 {
		t.Fatalf("expected stable context to be prepended, got %d messages", len(got))
	}
	if got[0].Role != "system" {
		t.Fatalf("expected top context role system, got %q", got[0].Role)
	}
	for _, want := range []string{"<ctx>", "<files>", `<file name="A.md">first file</file>`, `<file name="B.md">second file</file>`, "</ctx>"} {
		if !strings.Contains(got[0].Content, want) {
			t.Fatalf("expected top context to contain %q, got %q", want, got[0].Content)
		}
	}
	if strings.Contains(got[len(got)-1].Content, "<files>") {
		t.Fatalf("expected latest user message to stay focused on current turn, got %q", got[len(got)-1].Content)
	}
	if strings.Index(got[0].Content, `name="A.md"`) > strings.Index(got[0].Content, `name="B.md"`) {
		t.Fatalf("expected stable file order by file id, got %q", got[0].Content)
	}
}

func TestPrependStableFileContextSkipsImages(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "看图"}}
	attachments := []AttachmentInput{{
		Kind:          "image",
		MimeType:      "image/png",
		FileName:      "photo.png",
		ExtractedText: "image text should not be injected as stable file",
	}}

	got := prependStableFileContext(messages, attachments)
	if len(got) != len(messages) {
		t.Fatalf("expected image attachment to stay out of stable text context, got %#v", got)
	}
}

func TestPrependStableFileContextEscapesXMLFileContext(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "总结文件"}}
	attachments := []AttachmentInput{{
		FileName:      `A&B "notes".md`,
		FileCategory:  "document",
		ExtractedText: "Use <tag> & keep > value.\n\nNext line.",
	}}

	got := prependStableFileContext(messages, attachments)
	if len(got) != len(messages)+1 {
		t.Fatalf("expected stable file context to be prepended, got %#v", got)
	}
	for _, want := range []string{
		`<file name="A&amp;B &#34;notes&#34;.md">`,
		"Use &lt;tag&gt; &amp; keep &gt; value.\n\nNext line.",
	} {
		if !strings.Contains(got[0].Content, want) {
			t.Fatalf("expected escaped XML content to contain %q, got %q", want, got[0].Content)
		}
	}
	if strings.Contains(got[0].Content, "&#xA;") {
		t.Fatalf("expected XML text content to keep real newlines, got %q", got[0].Content)
	}
}

func TestBuildConversationFileContextPlanSkipsOversizedFileWhenRAGUnavailable(t *testing.T) {
	cfg := config.Config{FileFullContextMaxTokens: 10}
	plan := buildConversationFileContextPlan([]AttachmentInput{{
		FileID:        "file_large",
		FileName:      "large.md",
		FileCategory:  "document",
		ExtractedText: strings.Repeat("token ", 100),
		EmbedStatus:   "pending",
	}}, "auto", cfg, "gpt-5.5", "", false)

	if len(plan.FullAttachments) != 0 || len(plan.RAGAttachments) != 0 || len(plan.Skipped) != 1 {
		t.Fatalf("expected oversized unavailable file to be skipped, got %#v", plan)
	}
	if plan.Skipped[0].ContextMode != fileContextModeSkipped {
		t.Fatalf("expected skipped context mode, got %#v", plan.Skipped[0])
	}
}

func TestSplitRetrievalFallbackAttachmentsRespectsFullContextBudget(t *testing.T) {
	cfg := config.Config{FileFullContextMaxTokens: 10}
	fallbacks, skipped := splitRetrievalFallbackAttachments([]AttachmentInput{
		{
			FileID:        "small",
			FileName:      "small.md",
			FileCategory:  "document",
			ExtractedText: "short text",
		},
		{
			FileID:        "large",
			FileName:      "large.md",
			FileCategory:  "document",
			ExtractedText: strings.Repeat("token ", 100),
		},
	}, cfg)

	if len(fallbacks) != 1 || fallbacks[0].FileID != "small" || fallbacks[0].ContextMode != fileContextModeRAGFallback {
		t.Fatalf("expected only small file to fallback, got %#v", fallbacks)
	}
	if len(skipped) != 1 || skipped[0].FileID != "large" || skipped[0].ContextMode != fileContextModeSkipped {
		t.Fatalf("expected large file to be skipped, got %#v", skipped)
	}
}

func TestInjectUserContextCombinesDataContexts(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "继续"}}
	input := userContextInput{
		Snapshot: &snapshotContext{
			Summary:  "之前讨论了部署流程。",
			FromTurn: 1,
			ToTurn:   4,
			Strategy: "auto",
		},
		Memory: []domainmemory.UserMemory{{
			MemoryKey: "team",
			Value:     "prefers short answers",
		}},
		HistoricalArtifacts: []model.ContextArtifact{{
			Kind:        model.ContextArtifactFileRAGChunk,
			SourceTitle: "部署文档",
			Content:     "旧轮 RAG 证据提到先执行迁移。",
		}},
		RecallChunks: []model.MessageChunk{{
			Role:       "assistant",
			ChunkIndex: 2,
			Content:    "历史里提到需要先跑测试。",
		}},
	}

	got := injectUserContext(t.Context(), messages, input, config.Config{}, nil)
	for _, want := range []string{
		`<sum from="1" to="4" strategy="auto">之前讨论了部署流程。</sum>`,
		`<mem k="team">prefers short answers</mem>`,
		`<ev k="file_rag_chunk" src="部署文档">旧轮 RAG 证据提到先执行迁移。</ev>`,
		`<msg role="assistant" i="2">历史里提到需要先跑测试。</msg>`,
		"<q>继续</q>",
	} {
		if !strings.Contains(got[0].Content, want) {
			t.Fatalf("expected unified context to contain %q, got %q", want, got[0].Content)
		}
	}
}
