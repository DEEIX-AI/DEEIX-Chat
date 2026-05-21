package conversation

import (
	"strings"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestBuildPromptPlanLayersStableDynamicAndToolGuidance(t *testing.T) {
	base := []llm.Message{
		{Role: "system", Content: "用户偏好：回答简洁"},
		{Role: "user", Content: "第一轮问题"},
		{Role: "assistant", Content: "第一轮回答"},
		{Role: "user", Content: "第二轮问题"},
	}
	plan := buildPromptPlan(t.Context(), promptPlanInput{
		BaseMessages: base,
		StableAttachments: []AttachmentInput{{
			FileID:        "file_a",
			FileName:      "A.md",
			FileCategory:  "document",
			ExtractedText: "稳定文件全文",
		}},
		DynamicContext: userContextInput{
			HistoricalArtifacts: []model.ContextArtifact{{
				Kind:          model.ContextArtifactFileRAGChunk,
				ID:            42,
				SourceID:      "old_file:1",
				SourceTitle:   "old.md",
				Content:       "旧轮命中的证据",
				TokenEstimate: 6,
			}},
			RAGChunks: []model.RAGChunk{{
				FileID:     "file_a",
				FileName:   "A.md",
				ChunkIndex: 2,
				Content:    "本轮 RAG 片段",
			}},
			RecallChunks: []model.MessageChunk{{
				MessageID:  5,
				Role:       "assistant",
				ChunkIndex: 1,
				Content:    "本轮语义召回",
			}},
			CurrentArtifacts: []model.ContextArtifact{
				{
					ID:          84,
					Kind:        model.ContextArtifactFileRAGChunk,
					SourceID:    "file_a:2",
					SourceTitle: "A.md",
				},
				{
					ID:          85,
					Kind:        model.ContextArtifactSemanticRecall,
					SourceID:    "5:1",
					SourceTitle: "assistant",
				},
			},
		},
		ToolRuntime: selectedToolRuntime{
			definitions: []llm.ToolDefinition{{
				Name:        "search_web",
				Description: "搜索网页",
				InputSchema: []byte(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
			}},
		},
		Config: config.Config{},
	})

	if len(plan.Messages) != 6 {
		t.Fatalf("expected 6 messages, got %#v", plan.Messages)
	}
	if plan.Messages[0].Role != "system" || !strings.Contains(plan.Messages[0].Content, "<files>") {
		t.Fatalf("expected stable file context first, got %#v", plan.Messages[0])
	}
	if plan.Messages[1].Content != "用户偏好：回答简洁" {
		t.Fatalf("expected existing system policy second, got %#v", plan.Messages[1])
	}
	if !strings.HasPrefix(strings.TrimSpace(plan.Messages[2].Content), "# tool_use") {
		t.Fatalf("expected tool guidance after leading system messages, got %#v", plan.Messages[2])
	}
	for index := 0; index <= 2; index++ {
		if plan.Messages[index].CacheControl == nil || plan.Messages[index].CacheControl.Type != "ephemeral" {
			t.Fatalf("expected leading system message %d to be cacheable, got %#v", index, plan.Messages[index].CacheControl)
		}
	}
	if plan.Messages[3].Content != "第一轮问题" {
		t.Fatalf("expected historical user content to stay raw, got %q", plan.Messages[3].Content)
	}
	last := plan.Messages[len(plan.Messages)-1]
	for _, want := range []string{"<evs>", "旧轮命中的证据", "<rag>", "本轮 RAG 片段", "<q>第二轮问题</q>"} {
		if !strings.Contains(last.Content, want) {
			t.Fatalf("expected latest user to contain %q, got %q", want, last.Content)
		}
	}
	for _, blockKind := range []PromptBlockKind{PromptBlockTranscript, PromptBlockStableContext, PromptBlockHistoricalEvidence, PromptBlockDynamicContext, PromptBlockToolGuidance} {
		if !promptTraceHasBlock(plan.Trace, blockKind) {
			t.Fatalf("expected trace to contain %s, got %#v", blockKind, plan.Trace.Blocks)
		}
	}
	dynamicBlock := promptTraceBlock(plan.Trace, PromptBlockDynamicContext)
	if dynamicBlock == nil || len(dynamicBlock.SourceRefs) == 0 {
		t.Fatalf("expected dynamic trace source refs, got %#v", dynamicBlock)
	}
	ragRef := dynamicBlock.SourceRefs[0]
	if ragRef.SourceType != "file_rag_chunk" || ragRef.SourceID != "file_a" || !strings.Contains(ragRef.Title, "#3") {
		t.Fatalf("expected rag trace source to link by file id and title chunk, got %#v", ragRef)
	}
	if ragRef.ArtifactID != 84 {
		t.Fatalf("expected current rag trace source to carry artifact id, got %#v", ragRef)
	}
	recallRef := dynamicBlock.SourceRefs[1]
	if recallRef.SourceType != "semantic_recall" || recallRef.SourceID != "5:1" || recallRef.ArtifactID != 85 {
		t.Fatalf("expected current recall trace source to carry artifact id, got %#v", recallRef)
	}
	historicalBlock := promptTraceBlock(plan.Trace, PromptBlockHistoricalEvidence)
	if historicalBlock == nil || len(historicalBlock.SourceRefs) == 0 {
		t.Fatalf("expected historical trace source refs, got %#v", historicalBlock)
	}
	if historicalBlock.SourceRefs[0].ArtifactID != 42 {
		t.Fatalf("expected historical source ref to carry artifact id, got %#v", historicalBlock.SourceRefs[0])
	}
}

func TestStableAttachmentSourceRefsUsesFallbackArtifactID(t *testing.T) {
	refs := stableAttachmentSourceRefs(
		[]AttachmentInput{{
			FileID:        "file_b",
			FileName:      "B.md",
			ExtractedText: "全文回退证据",
		}},
		[]model.ContextArtifact{{
			ID:          88,
			Kind:        model.ContextArtifactFileRAGFallback,
			SourceID:    "file_b",
			SourceTitle: "B.md",
		}},
	)
	if len(refs) != 1 {
		t.Fatalf("expected one source ref, got %#v", refs)
	}
	if refs[0].SourceType != "file_rag_fallback" || refs[0].SourceID != "file_b" || refs[0].ArtifactID != 88 {
		t.Fatalf("expected fallback source ref to carry artifact id, got %#v", refs[0])
	}
}

func TestBuildPromptPlanKeepsRawMessagesWhenNoContext(t *testing.T) {
	base := []llm.Message{
		{Role: "user", Content: "你好"},
		{Role: "assistant", Content: "你好，有什么可以帮你？"},
		{Role: "user", Content: "继续"},
	}

	plan := buildPromptPlan(t.Context(), promptPlanInput{
		BaseMessages: base,
		Config:       config.Config{},
	})

	if len(plan.Messages) != len(base) {
		t.Fatalf("expected message count unchanged, got %#v", plan.Messages)
	}
	for index := range base {
		if plan.Messages[index].Role != base[index].Role || plan.Messages[index].Content != base[index].Content {
			t.Fatalf("expected raw message at %d, got %#v want %#v", index, plan.Messages[index], base[index])
		}
	}
	if len(plan.Trace.Blocks) != 1 || plan.Trace.Blocks[0].Kind != PromptBlockTranscript {
		t.Fatalf("expected transcript-only trace, got %#v", plan.Trace.Blocks)
	}
}

func promptTraceHasBlock(trace PromptTrace, kind PromptBlockKind) bool {
	return promptTraceBlock(trace, kind) != nil
}

func promptTraceBlock(trace PromptTrace, kind PromptBlockKind) *PromptBlockTrace {
	for _, block := range trace.Blocks {
		if block.Kind == kind {
			item := block
			return &item
		}
	}
	return nil
}
