package conversation

import (
	"encoding/json"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

func TestBuildPromptContextArtifactsRecordsRAGFallbackAndRecall(t *testing.T) {
	items := buildPromptContextArtifacts(promptContextArtifactInput{
		ConversationID: 7,
		UserID:         11,
		MessageID:      13,
		RunID:          "run_1",
		Query:          "解释文件",
		RAGChunks: []model.RAGChunk{{
			FileID:     "file_a",
			FileName:   "A.md",
			ChunkIndex: 2,
			Content:    "RAG 命中的证据",
			Score:      0.87,
		}},
		RAGFallbacks: []ragFallbackEvidence{{
			Reason: "rag_empty",
			Attachment: AttachmentInput{
				FileID:        "file_b",
				FileName:      "B.md",
				SHA256:        "sha_b",
				ExtractStatus: "ready",
				EmbedStatus:   "ready",
				ExtractedText: "全文回退证据",
			},
		}},
		RecallChunks: []model.MessageChunk{{
			MessageID:  3,
			Role:       "assistant",
			ChunkIndex: 1,
			Content:    "历史语义召回证据",
			Similarity: 0.82,
		}},
		Memories: []domainmemory.UserMemory{{
			MemoryKey: "language",
			Value:     "优先使用中文回答",
			Scope:     "profile",
			UpdatedBy: "manual",
		}},
	})

	if len(items) != 4 {
		t.Fatalf("expected 4 artifacts, got %#v", items)
	}
	if items[0].Kind != model.ContextArtifactFileRAGChunk || items[0].SourceID != "file_a:2" {
		t.Fatalf("expected file rag artifact, got %#v", items[0])
	}
	if items[1].Kind != model.ContextArtifactFileRAGFallback || items[1].SourceID != "file_b" {
		t.Fatalf("expected fallback artifact, got %#v", items[1])
	}
	if !hasContextArtifact(items, model.ContextArtifactSemanticRecall, "3:1") {
		t.Fatalf("expected recall artifact, got %#v", items)
	}
	if !hasContextArtifact(items, model.ContextArtifactUserMemory, "language") {
		t.Fatalf("expected memory artifact, got %#v", items)
	}
	for _, item := range items {
		if item.ConversationID != 7 || item.UserID != 11 || item.MessageID != 13 || item.RunID != "run_1" {
			t.Fatalf("artifact identity mismatch: %#v", item)
		}
		if item.ContentHash == "" || item.TokenEstimate <= 0 || item.MetadataJSON == "" {
			t.Fatalf("artifact missing trace fields: %#v", item)
		}
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(item.MetadataJSON), &metadata); err != nil {
			t.Fatalf("invalid metadata json: %v", err)
		}
	}
}

func hasContextArtifact(items []model.ContextArtifact, kind model.ContextArtifactKind, sourceID string) bool {
	for _, item := range items {
		if item.Kind == kind && item.SourceID == sourceID {
			return true
		}
	}
	return false
}

func TestBuildPromptContextArtifactsSkipsEmptyEvidence(t *testing.T) {
	items := buildPromptContextArtifacts(promptContextArtifactInput{
		RAGChunks: []model.RAGChunk{{
			FileID:     "file_a",
			ChunkIndex: 1,
			Content:    " ",
		}},
		RAGFallbacks: []ragFallbackEvidence{{
			Attachment: AttachmentInput{
				FileID:        "file_b",
				ExtractedText: "",
			},
		}},
		RecallChunks: []model.MessageChunk{{
			MessageID: 1,
			Content:   "",
		}},
		Memories: []domainmemory.UserMemory{{
			MemoryKey: "empty",
			Value:     "",
		}},
	})
	if len(items) != 0 {
		t.Fatalf("expected empty artifacts, got %#v", items)
	}
}

func TestApplyContextArtifactRetentionSetsExpiresAt(t *testing.T) {
	svc := &Service{cfg: config.NewRuntime(config.Config{ContextArtifactRetentionDays: 7})}
	items := []model.ContextArtifact{{Content: "evidence"}}

	svc.applyContextArtifactRetention(items)

	if items[0].ExpiresAt == nil {
		t.Fatal("expected expires_at to be set")
	}
	if !items[0].ExpiresAt.After(items[0].CreatedAt) {
		t.Fatalf("expected future expires_at, got %#v", items[0].ExpiresAt)
	}
}

func TestApplyContextArtifactRetentionCanBeDisabled(t *testing.T) {
	svc := &Service{cfg: config.NewRuntime(config.Config{ContextArtifactRetentionDays: 0})}
	items := []model.ContextArtifact{{Content: "evidence"}}

	svc.applyContextArtifactRetention(items)

	if items[0].ExpiresAt != nil {
		t.Fatalf("expected no expires_at, got %#v", items[0].ExpiresAt)
	}
}

func TestBuildToolContextArtifactsRecordsLocalAndNativeTools(t *testing.T) {
	items := buildToolContextArtifacts(toolContextArtifactInput{
		ConversationID: 7,
		UserID:         11,
		MessageID:      13,
		RunID:          "run_1",
		Rows: []model.ToolCall{
			{
				ToolCallID: "call_local",
				ToolType:   "function",
				ToolName:   "search_web",
				Status:     "success",
				InputJSON:  `{"query":"DEEIX Chat"}`,
				OutputJSON: `{"answer":"result"}`,
			},
			{
				ToolCallID: "call_native",
				ToolType:   "web_search_call",
				ToolName:   "web_search",
				Status:     "success",
				OutputJSON: `{"url":"https://example.com"}`,
			},
		},
	})
	if len(items) != 2 {
		t.Fatalf("expected two tool artifacts, got %#v", items)
	}
	if items[0].Kind != model.ContextArtifactToolResult || items[0].SourceID != "call_local" {
		t.Fatalf("expected local tool artifact, got %#v", items[0])
	}
	if items[1].Kind != model.ContextArtifactNativeTool || items[1].SourceID != "call_native" {
		t.Fatalf("expected native tool artifact, got %#v", items[1])
	}
}

func TestBuildSnapshotContextArtifactRecordsSummary(t *testing.T) {
	item := buildSnapshotContextArtifact(snapshotContextArtifactInput{
		ConversationID: 7,
		UserID:         11,
		MessageID:      14,
		RunID:          "run_1",
		Snapshot: &model.ContextSnapshot{
			ID:            3,
			RunID:         "run_1",
			FromTurn:      1,
			ToTurn:        6,
			SourceTokens:  1000,
			SummaryTokens: 120,
			SummaryText:   "压缩摘要内容",
			Strategy:      "token_cap",
		},
	})
	if item == nil {
		t.Fatal("expected snapshot artifact")
	}
	if item.Kind != model.ContextArtifactSummary || item.SourceID != "3" || item.MessageID != 14 {
		t.Fatalf("unexpected snapshot artifact: %#v", item)
	}
	if item.TokenEstimate != 120 || item.ContentHash == "" || item.MetadataJSON == "" {
		t.Fatalf("snapshot artifact missing fields: %#v", item)
	}
}

func TestSelectHistoricalContextArtifactsUsesFollowUpAndDeduplicatesCurrentEvidence(t *testing.T) {
	items := selectHistoricalContextArtifacts(historicalContextArtifactInput{
		CurrentMessageID: 9,
		Query:            "把刚才这个文件总结短一点",
		CurrentRAGChunks: []model.RAGChunk{{
			Content: "当前轮已经命中的重复证据",
		}},
		Candidates: []model.ContextArtifact{
			{
				MessageID:     8,
				Kind:          model.ContextArtifactFileRAGChunk,
				SourceTitle:   "A.md",
				Content:       "当前轮已经命中的重复证据",
				TokenEstimate: 10,
			},
			{
				MessageID:     7,
				Kind:          model.ContextArtifactFileRAGChunk,
				SourceTitle:   "B.md",
				Content:       "旧轮文件证据，说明系统分层和测试要求。",
				TokenEstimate: 10,
			},
			{
				MessageID:     9,
				Kind:          model.ContextArtifactFileRAGChunk,
				SourceTitle:   "current.md",
				Content:       "当前消息自己的证据不应被召回。",
				TokenEstimate: 10,
			},
		},
	})

	if len(items) != 1 {
		t.Fatalf("expected one historical artifact, got %#v", items)
	}
	if items[0].SourceTitle != "B.md" {
		t.Fatalf("expected B.md artifact, got %#v", items[0])
	}
}

func TestSelectHistoricalContextArtifactsRequiresRelevanceWithoutFollowUp(t *testing.T) {
	items := selectHistoricalContextArtifacts(historicalContextArtifactInput{
		Query: "部署 测试",
		Candidates: []model.ContextArtifact{
			{
				MessageID:     1,
				Kind:          model.ContextArtifactFileRAGChunk,
				SourceTitle:   "ops.md",
				Content:       "上线部署前必须先跑测试。",
				TokenEstimate: 10,
			},
			{
				MessageID:     2,
				Kind:          model.ContextArtifactFileRAGChunk,
				SourceTitle:   "music.md",
				Content:       "歌单统计结果。",
				TokenEstimate: 10,
			},
		},
	})

	if len(items) != 1 {
		t.Fatalf("expected one relevant artifact, got %#v", items)
	}
	if items[0].SourceTitle != "ops.md" {
		t.Fatalf("expected ops.md artifact, got %#v", items[0])
	}
}
