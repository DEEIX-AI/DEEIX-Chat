package repository

import (
	"context"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

// MessageUsageUpdate 定义消息 token 用量更新字段。
type MessageUsageUpdate struct {
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	ReasoningTokens  int64
}

// AssistantMessageCompletionUpdate 定义助手消息完成态更新字段。
type AssistantMessageCompletionUpdate struct {
	ContentType      string
	Content          string
	ReasoningContent string
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	ReasoningTokens  int64
	LatencyMS        int64
	Status           string
	ErrorCode        string
	ErrorMessage     string
	KnowledgeSources []domainconversation.MessageKnowledgeSource
}

// ForkConversationMessage 描述 fork 时待创建的消息及其源消息关系。
// Message 仅承载新消息字段；父子关系通过源消息 ID 在事务内重建。
type ForkConversationMessage struct {
	SourceMessageID       uint
	SourceParentMessageID *uint
	Message               domainconversation.Message
}

// CreateForkedConversationInput 描述一次原子 fork 写入。
type CreateForkedConversationInput struct {
	SourceConversationID uint
	Conversation         *domainconversation.Conversation
	Messages             []ForkConversationMessage
}

// ConversationForkRepository 封装新会话、消息链、附件引用和历史展示轨迹的原子复制能力。
type ConversationForkRepository interface {
	CreateForkedConversation(ctx context.Context, input CreateForkedConversationInput) error
}

// ConversationMetadataPatch 定义自动生成会话元数据的更新字段。
type ConversationMetadataPatch struct {
	Title             string
	ReplaceableTitles []string
}

// ConversationListInput 描述用户会话列表的分页与筛选条件。
type ConversationListInput struct {
	UserID        uint
	Offset        int
	Limit         int
	StatusFilter  string
	StarredFilter string
	ShareFilter   string
	ProjectFilter string
	SearchQuery   string
}

// DeleteConversationProjectOptions controls which project-owned records are removed.
type DeleteConversationProjectOptions struct {
	DeleteConversations bool
	DeleteFiles         bool
}

// ConversationMetadataRepository 封装会话元信息与用户访问能力。
type ConversationMetadataRepository interface {
	CreateConversation(ctx context.Context, item *domainconversation.Conversation) error
	ListConversationsByUser(ctx context.Context, input ConversationListInput) ([]domainconversation.Conversation, int64, error)
	ListConversationsForSearch(ctx context.Context, userID uint, offset int, limit int, searchQuery string) ([]domainconversation.Conversation, error)
	GetConversationByUser(ctx context.Context, conversationID uint, userID uint) (*domainconversation.Conversation, error)
	GetConversationByPublicID(ctx context.Context, publicID string, userID uint) (*domainconversation.Conversation, error)
	CreateConversationProject(ctx context.Context, item *domainconversation.ConversationProject) error
	ListConversationProjects(ctx context.Context, userID uint, statusFilter string) ([]domainconversation.ConversationProject, error)
	GetConversationProjectByPublicID(ctx context.Context, userID uint, publicID string) (*domainconversation.ConversationProject, error)
	UpdateConversationProjectMetadataByPublicID(ctx context.Context, userID uint, publicID string, patch domainconversation.ConversationProjectPatch) (*domainconversation.ConversationProject, error)
	DeleteConversationProjectByPublicID(ctx context.Context, userID uint, publicID string, options DeleteConversationProjectOptions) ([]string, error)
	ReorderConversationProjects(ctx context.Context, userID uint, publicIDs []string) error
	UpdateConversationProjectAssignmentByPublicID(ctx context.Context, userID uint, conversationPublicID string, projectID *uint) (*domainconversation.Conversation, error)
	BatchUpdateConversationProjectByPublicIDs(ctx context.Context, userID uint, conversationPublicIDs []string, projectID *uint) (int64, error)
	GetLatestConversationShareByConversation(ctx context.Context, userID uint, conversationID uint) (*domainconversation.ConversationShare, error)
	GetActiveConversationShareByShareID(ctx context.Context, shareID string) (*domainconversation.ConversationShare, *domainconversation.Conversation, error)
	ReplaceActiveConversationShare(ctx context.Context, item *domainconversation.ConversationShare) error
	RevokeActiveConversationShares(ctx context.Context, userID uint, conversationIDs []uint) error
	TouchConversationShareAccess(ctx context.Context, shareID string, accessedAt time.Time) error
	UpdateConversationTitleByPublicID(ctx context.Context, userID uint, publicID string, title string) (*domainconversation.Conversation, error)
	UpdateConversationMetadata(ctx context.Context, conversationID uint, patch ConversationMetadataPatch) (*domainconversation.Conversation, error)
	UpdateConversationLabelsByPublicID(ctx context.Context, userID uint, publicID string, labelsJSON string) (*domainconversation.Conversation, error)
	SetGeneratedConversationLabelsIfEligible(ctx context.Context, conversationID uint, labelsJSON string) (*domainconversation.Conversation, bool, error)
	UpdateConversationStarByPublicID(ctx context.Context, userID uint, publicID string, starred bool) (*domainconversation.Conversation, error)
	UpdateConversationArchiveByPublicID(ctx context.Context, userID uint, publicID string, archived bool) (*domainconversation.Conversation, error)
	DeleteConversationByPublicID(ctx context.Context, userID uint, publicID string, deleteFiles bool) ([]string, error)
	IncrementMessageCount(ctx context.Context, conversationID uint, delta int) error
	UpdateConversationLastResponseID(ctx context.Context, conversationID uint, responseID string) error
	UpdateConversationStatefulResponse(ctx context.Context, conversationID uint, responseID string, promptFingerprint string) error
	UpdateConversationModel(ctx context.Context, conversationID uint, platformModelName string, provider string) error
	ListAllConversationsAfterID(ctx context.Context, afterID uint, limit int) ([]domainconversation.Conversation, error)
	ListUserConversationsAfterID(ctx context.Context, userID uint, afterID uint, limit int) ([]domainconversation.Conversation, error)
}

// MessageRepository 封装消息读写能力。
type MessageRepository interface {
	CreateMessage(ctx context.Context, item *domainconversation.Message) error
	CreateAssistantBranchMessage(ctx context.Context, assistantMessage *domainconversation.Message) error
	CreateMessagePairWithUserAttachments(ctx context.Context, userMessage *domainconversation.Message, assistantMessage *domainconversation.Message, userAttachments []domainconversation.Attachment) error
	CompleteAssistantMessageWithAttachments(ctx context.Context, userMessageID uint, userUsage MessageUsageUpdate, assistantMessageID uint, assistantCompletion AssistantMessageCompletionUpdate, assistantAttachments []domainconversation.Attachment) error
	CompleteAssistantMessageWithGeneratedAttachments(ctx context.Context, assistantMessageID uint, assistantCompletion AssistantMessageCompletionUpdate, assistantAttachments []domainconversation.Attachment) error
	GetMessageByPublicID(ctx context.Context, conversationID uint, userID uint, publicID string) (*domainconversation.Message, error)
	GetMessageByPublicIDForUser(ctx context.Context, userID uint, publicID string) (*domainconversation.Message, error)
	UpdateMessageUsage(ctx context.Context, messageID uint, usage MessageUsageUpdate) error
	UpdateMessageState(ctx context.Context, messageID uint, status string, errorCode string, errorMessage string) error
	UpdateAssistantMessageContent(ctx context.Context, userID uint, publicID string, content string, editedAt time.Time) (*domainconversation.Message, error)
	CancelPendingGenerationMessagesByRunID(ctx context.Context, userID uint, runID string, errorCode string, errorMessage string) (bool, error)
	InterruptPendingAssistantMessageByRunID(ctx context.Context, userID uint, runID string, errorCode string, errorMessage string) (bool, error)
	UpdateAssistantMessageCompletion(ctx context.Context, messageID uint, update AssistantMessageCompletionUpdate) error
	UpdateMessageBilling(ctx context.Context, messageID uint, billedCurrency string, billedNanousd int64, pricingSnapshot string) error
	ListMessages(ctx context.Context, conversationID uint, offset int, limit int) ([]domainconversation.Message, int64, error)
	ListMessagesBeforeID(ctx context.Context, conversationID uint, beforeID uint, limit int) ([]domainconversation.Message, int64, error)
	ListAllMessages(ctx context.Context, conversationID uint) ([]domainconversation.Message, error)
	ListMessagesForShare(ctx context.Context, conversationID uint, publicIDs []string) ([]domainconversation.Message, error)
	ListRecentMessages(ctx context.Context, conversationID uint, limit int) ([]domainconversation.Message, int64, error)
	GetMessageByID(ctx context.Context, conversationID uint, messageID uint) (*domainconversation.Message, error)
	ListMessageAncestors(ctx context.Context, conversationID uint, leafMessageID uint, maxDepth int) ([]domainconversation.Message, error)
	ListLatestBranchPreviewMessages(ctx context.Context, conversationID uint, maxDepth int, limit int) ([]domainconversation.Message, error)
}

// MessageFeedbackRepository 封装消息反馈能力。
type MessageFeedbackRepository interface {
	UpsertMessageFeedback(ctx context.Context, item *domainconversation.MessageFeedback) error
	DeleteMessageFeedback(ctx context.Context, userID uint, messageID uint) error
	GetUserMessageFeedbackMap(ctx context.Context, userID uint, messageIDs []uint) (map[uint]string, error)
	GetMessageFeedbackCounts(ctx context.Context, messageIDs []uint) (map[uint]map[string]int64, error)
}

// ConversationTraceRepository 封装附件、运行轨迹与工具调用能力。
type ConversationTraceRepository interface {
	CreateAttachments(ctx context.Context, items []domainconversation.Attachment) error
	// CreateConversationRun 原子占用全局唯一的 run ID；重复 ID 返回 ErrDuplicate。
	CreateConversationRun(ctx context.Context, item *domainconversation.Run) error
	// UpdateConversationRun 更新已占用的运行快照，且不允许转移运行归属。
	UpdateConversationRun(ctx context.Context, item *domainconversation.Run) error
	UpsertConversationMessageTrace(ctx context.Context, item *domainconversation.MessageTrace) error
	ListConversationMessageTracesByMessageIDs(ctx context.Context, messageIDs []uint) ([]domainconversation.MessageTrace, error)
	UpsertConversationMessageTraceEvent(ctx context.Context, item *domainconversation.MessageTraceEventRow) error
	ListConversationMessageTraceEventsByMessageIDs(ctx context.Context, messageIDs []uint) ([]domainconversation.MessageTraceEventRow, error)
	CreateConversationToolCall(ctx context.Context, item *domainconversation.ToolCall) error
	CreateConversationToolCalls(ctx context.Context, items []domainconversation.ToolCall) error
	ListConversationRuns(ctx context.Context, userID uint, conversationID uint, offset int, limit int) ([]domainconversation.Run, int64, error)
	ListConversationRunsByRunIDs(ctx context.Context, userID uint, conversationID uint, runIDs []string) ([]domainconversation.Run, error)
	ListConversationRunStatusesByRunIDs(ctx context.Context, userID uint, runIDs []string) ([]domainconversation.RunStatus, error)
	ListConversationEventLogs(ctx context.Context, filter ConversationEventLogListFilter, offset int, limit int) ([]domainconversation.EventLog, int64, error)
	GetConversationEventLog(ctx context.Context, eventID uint) (*domainconversation.EventLog, error)
	GetConversationToolCallDetail(ctx context.Context, userID uint, runID string, toolCallID string) (*domainconversation.ToolCallDetail, error)
}

// ConversationEventLogListFilter 描述管理员对话事件列表筛选和排序条件。
type ConversationEventLogListFilter struct {
	Query          string
	EventScope     string
	EventType      string
	Status         string
	UserID         uint
	ConversationID uint
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	Sort           string
}

// MessageEmbeddingRepository 封装消息历史向量存储与检索能力。
type MessageEmbeddingRepository interface {
	UpsertMessageChunks(ctx context.Context, chunks []domainconversation.MessageChunk, embeddings [][]float32) error
	SearchMessageChunks(ctx context.Context, input MessageChunkSearchInput) ([]domainconversation.MessageChunk, error)
}

// HistoricalMessageScope 描述当前消息所在分支中可参与历史召回的祖先范围。
// ExcludeThroughMessageID 用于快照场景：排除该边界消息及其全部祖先。
type HistoricalMessageScope struct {
	ConversationID          uint
	UserID                  uint
	LeafMessageID           uint
	ExcludeThroughMessageID uint
}

// Valid 表示分支范围包含可验证的会话和叶消息锚点。
func (scope HistoricalMessageScope) Valid() bool {
	return scope.ConversationID > 0 && scope.UserID > 0 && scope.LeafMessageID > 0
}

// MessageChunkSearchInput 描述当前分支内的历史消息语义检索。
type MessageChunkSearchInput struct {
	Scope              HistoricalMessageScope
	QueryEmbedding     []float32
	EmbeddingSignature string
	TopK               int
	MinSimilarity      float64
}

// CompactRepository 封装上下文压缩快照能力。
type CompactRepository interface {
	CreateContextSnapshot(ctx context.Context, item *domainconversation.ContextSnapshot) error
	GetContextSnapshotByRunID(ctx context.Context, runID string) (*domainconversation.ContextSnapshot, error)
	GetLatestContextSnapshot(ctx context.Context, conversationID uint) (*domainconversation.ContextSnapshot, error)
	UpdateConversationCompactedAt(ctx context.Context, conversationID uint, compactedAt time.Time) error
}
