package conversation

import "time"

// ContextArtifactKind 表示对话上下文证据的来源类别。
type ContextArtifactKind string

const (
	ContextArtifactFileRAGChunk    ContextArtifactKind = "file_rag_chunk"
	ContextArtifactFileRAGFallback ContextArtifactKind = "file_rag_fallback"
	ContextArtifactSemanticRecall  ContextArtifactKind = "semantic_recall"
	ContextArtifactUserMemory      ContextArtifactKind = "user_memory"
	ContextArtifactToolResult      ContextArtifactKind = "tool_result"
	ContextArtifactNativeTool      ContextArtifactKind = "native_tool_result"
	ContextArtifactSummary         ContextArtifactKind = "conversation_summary"
)

// ContextArtifact 保存一次对话请求中被选入上下文规划的证据引用。
type ContextArtifact struct {
	ID             uint
	ConversationID uint
	MessageID      uint
	UserID         uint
	RunID          string
	Kind           ContextArtifactKind
	SourceType     string
	SourceID       string
	SourceTitle    string
	Content        string
	ContentHash    string
	TokenEstimate  int64
	Score          float64
	MetadataJSON   string
	ExpiresAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
