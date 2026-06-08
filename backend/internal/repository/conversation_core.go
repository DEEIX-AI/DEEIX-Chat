package repository

import (
	"context"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
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
	ContentType     string
	Content         string
	OutputTokens    int64
	ReasoningTokens int64
	LatencyMS       int64
	Status          string
	ErrorCode       string
	ErrorMessage    string
}

// ConversationMetadataRepository 封装会话元信息与用户访问能力。
type ConversationMetadataRepository interface {
	CreateConversation(ctx context.Context, item *domainconversation.Conversation) error
	ListConversationsByUser(ctx context.Context, userID uint, offset int, limit int, statusFilter string, starredFilter string, shareFilter string, projectFilter string) ([]domainconversation.Conversation, int64, error)
	GetConversationByUser(ctx context.Context, conversationID uint, userID uint) (*domainconversation.Conversation, error)
	GetConversationByPublicID(ctx context.Context, publicID string, userID uint) (*domainconversation.Conversation, error)
	CreateConversationProject(ctx context.Context, item *domainconversation.ConversationProject) error
	ListConversationProjects(ctx context.Context, userID uint, statusFilter string) ([]domainconversation.ConversationProject, error)
	GetConversationProjectByPublicID(ctx context.Context, userID uint, publicID string) (*domainconversation.ConversationProject, error)
	UpdateConversationProjectMetadataByPublicID(ctx context.Context, userID uint, publicID string, patch domainconversation.ConversationProjectPatch) (*domainconversation.ConversationProject, error)
	DeleteConversationProjectByPublicID(ctx context.Context, userID uint, publicID string, deleteConversations bool, deleteFiles bool) ([]string, error)
	ReorderConversationProjects(ctx context.Context, userID uint, publicIDs []string) error
	UpdateConversationProjectAssignmentByPublicID(ctx context.Context, userID uint, conversationPublicID string, projectID *uint) (*domainconversation.Conversation, error)
	BatchUpdateConversationProjectByPublicIDs(ctx context.Context, userID uint, conversationPublicIDs []string, projectID *uint) (int64, error)
	GetActiveConversationShareByConversation(ctx context.Context, userID uint, conversationID uint) (*domainconversation.ConversationShare, error)
	GetLatestConversationShareByConversation(ctx context.Context, userID uint, conversationID uint) (*domainconversation.ConversationShare, error)
	GetActiveConversationShareByShareID(ctx context.Context, shareID string) (*domainconversation.ConversationShare, *domainconversation.Conversation, error)
	CreateConversationShare(ctx context.Context, item *domainconversation.ConversationShare) error
	ReplaceActiveConversationShare(ctx context.Context, item *domainconversation.ConversationShare) error
	RevokeActiveConversationShares(ctx context.Context, userID uint, conversationIDs []uint) error
	TouchConversationShareAccess(ctx context.Context, shareID string, accessedAt time.Time) error
	UpdateConversationTitleByPublicID(ctx context.Context, userID uint, publicID string, title string) (*domainconversation.Conversation, error)
	UpdateConversationMetadata(ctx context.Context, conversationID uint, title string, labelsJSON string) (*domainconversation.Conversation, error)
	UpdateConversationStarByPublicID(ctx context.Context, userID uint, publicID string, starred bool) (*domainconversation.Conversation, error)
	UpdateConversationArchiveByPublicID(ctx context.Context, userID uint, publicID string, archived bool) (*domainconversation.Conversation, error)
	DeleteConversationByPublicID(ctx context.Context, userID uint, publicID string, deleteFiles bool) ([]string, error)
	GetUserByID(ctx context.Context, userID uint) (*domainuser.User, error)
	IncrementMessageCount(ctx context.Context, conversationID uint, delta int) error
	UpdateConversationLastResponseID(ctx context.Context, conversationID uint, responseID string) error
	UpdateConversationStatefulResponse(ctx context.Context, conversationID uint, responseID string, promptFingerprint string) error
	UpdateConversationModel(ctx context.Context, conversationID uint, platformModelName string, provider string) error
}

// MessageRepository 封装消息读写能力。
type MessageRepository interface {
	CreateMessage(ctx context.Context, item *domainconversation.Message) error
	CreateMessagePairWithUserAttachments(ctx context.Context, userMessage *domainconversation.Message, assistantMessage *domainconversation.Message, userAttachments []domainconversation.Attachment) error
	CompleteAssistantMessageWithAttachments(ctx context.Context, userMessageID uint, userUsage MessageUsageUpdate, assistantMessageID uint, assistantCompletion AssistantMessageCompletionUpdate, assistantAttachments []domainconversation.Attachment) error
	GetMessageByPublicID(ctx context.Context, conversationID uint, userID uint, publicID string) (*domainconversation.Message, error)
	GetMessageByPublicIDForUser(ctx context.Context, userID uint, publicID string) (*domainconversation.Message, error)
	UpdateMessageUsage(ctx context.Context, messageID uint, inputTokens int64, outputTokens int64, cacheReadTokens int64, cacheWriteTokens int64, reasoningTokens int64) error
	UpdateMessageState(ctx context.Context, messageID uint, status string, errorCode string, errorMessage string) error
	UpdateAssistantMessageContent(ctx context.Context, userID uint, publicID string, content string, editedAt time.Time) (*domainconversation.Message, error)
	CancelPendingGenerationMessagesByRunID(ctx context.Context, userID uint, runID string, errorCode string, errorMessage string) (bool, error)
	InterruptPendingAssistantMessageByRunID(ctx context.Context, userID uint, runID string, errorCode string, errorMessage string) (bool, error)
	UpdateAssistantMessageCompletion(ctx context.Context, messageID uint, content string, outputTokens int64, reasoningTokens int64, latencyMS int64, status string, errorCode string, errorMessage string) error
	UpdateMessageBilling(ctx context.Context, messageID uint, billedCurrency string, billedNanousd int64, pricingSnapshot string) error
	SumMessageTokens(ctx context.Context, conversationID uint) (int64, error)
	ListMessages(ctx context.Context, conversationID uint, offset int, limit int) ([]domainconversation.Message, int64, error)
	ListMessagesBeforeID(ctx context.Context, conversationID uint, beforeID uint, limit int) ([]domainconversation.Message, int64, error)
	ListAllMessages(ctx context.Context, conversationID uint) ([]domainconversation.Message, error)
	ListMessagesForShare(ctx context.Context, conversationID uint, publicIDs []string) ([]domainconversation.Message, error)
	ListRecentMessages(ctx context.Context, conversationID uint, limit int) ([]domainconversation.Message, int64, error)
	GetMessageByID(ctx context.Context, conversationID uint, messageID uint) (*domainconversation.Message, error)
	GetLatestMessage(ctx context.Context, conversationID uint) (*domainconversation.Message, error)
	ListMessageAncestors(ctx context.Context, conversationID uint, leafMessageID uint, maxDepth int) ([]domainconversation.Message, error)
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
	CreateConversationRun(ctx context.Context, item *domainconversation.Run) error
	UpsertConversationMessageTrace(ctx context.Context, item *domainconversation.MessageTrace) error
	ListConversationMessageTracesByMessageIDs(ctx context.Context, messageIDs []uint) ([]domainconversation.MessageTrace, error)
	UpsertConversationMessageTraceEvent(ctx context.Context, item *domainconversation.MessageTraceEventRow) error
	ListConversationMessageTraceEventsByMessageIDs(ctx context.Context, messageIDs []uint) ([]domainconversation.MessageTraceEventRow, error)
	CreateConversationToolCalls(ctx context.Context, items []domainconversation.ToolCall) error
	ListConversationRuns(ctx context.Context, userID uint, conversationID uint, offset int, limit int) ([]domainconversation.Run, int64, error)
	ListConversationRunsByRunIDs(ctx context.Context, userID uint, conversationID uint, runIDs []string) ([]domainconversation.Run, error)
}

// MessageEmbeddingRepository 封装消息历史向量存储与检索能力。
type MessageEmbeddingRepository interface {
	UpsertMessageChunks(ctx context.Context, chunks []domainconversation.MessageChunk, embeddings [][]float32) error
	SearchMessageChunks(ctx context.Context, conversationID uint, userID uint, queryEmbedding []float32, topK int, minSimilarity float64) ([]domainconversation.MessageChunk, error)
}

// CompactRepository 封装上下文压缩快照能力。
type CompactRepository interface {
	SumMessageTokens(ctx context.Context, conversationID uint) (int64, error)
	ListMessages(ctx context.Context, conversationID uint, offset int, limit int) ([]domainconversation.Message, int64, error)
	CreateContextSnapshot(ctx context.Context, item *domainconversation.ContextSnapshot) error
	GetContextSnapshotByRunID(ctx context.Context, runID string) (*domainconversation.ContextSnapshot, error)
	GetLatestContextSnapshot(ctx context.Context, conversationID uint) (*domainconversation.ContextSnapshot, error)
	UpdateConversationCompactedAt(ctx context.Context, conversationID uint, compactedAt time.Time) error
}
