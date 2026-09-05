package conversation

import (
	"context"
	"errors"
	"strings"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	conversationPreviewMessageLimit     = 10
	conversationPreviewAncestorMaxDepth = 100
	conversationExportVersion           = 1
	conversationExportScopeFull         = "full"
)

// DeleteConversationOptions 定义会话删除选项。
type DeleteConversationOptions struct {
	DeleteFiles bool
}

// DeleteConversationResult 返回会话删除结果。
type DeleteConversationResult struct {
	Deleted          bool
	DeletedFileCount int
	Quota            *model.StorageQuota
}

// ConversationSearchResult 表示会话搜索列表中的单个结果。
type ConversationSearchResult struct {
	Conversation model.Conversation
}

// ListConversationsInput 描述用户会话列表的分页与筛选条件。
type ListConversationsInput struct {
	UserID        uint
	Page          int
	PageSize      int
	StatusFilter  string
	StarredFilter string
	ShareFilter   string
	ProjectFilter string
	SearchQuery   string
}

// CreateConversation 创建用户新会话。
func (s *Service) CreateConversation(ctx context.Context, userID uint, title string, modelName string, projectPublicID string) (*model.Conversation, error) {
	normalizedTitle := strings.TrimSpace(title)
	if normalizedTitle == "" {
		normalizedTitle = "新对话"
	}

	normalizedModel := strings.TrimSpace(modelName)
	var projectID *uint
	var project *model.ConversationProject
	if normalizedProjectID := strings.TrimSpace(projectPublicID); normalizedProjectID != "" {
		resolvedProject, err := s.repo.GetConversationProjectByPublicID(ctx, userID, normalizedProjectID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, ErrConversationProjectNotFound
			}
			return nil, err
		}
		project = resolvedProject
		projectID = &project.ID
		if normalizedModel == "" {
			projectDefaultModel := strings.TrimSpace(project.DefaultModel)
			if projectDefaultModel != "" {
				available, availabilityErr := s.isAvailableConversationProjectDefaultModel(ctx, userID, projectDefaultModel)
				if availabilityErr != nil {
					return nil, availabilityErr
				}
				if available {
					normalizedModel = projectDefaultModel
				}
			}
		}
	}

	item := &model.Conversation{
		UserID:          userID,
		ProjectID:       projectID,
		PublicID:        normalizePublicID(uuid.NewString()),
		Title:           normalizedTitle,
		LabelsJSON:      "[]",
		Model:           normalizedModel,
		Provider:        inferProvider(normalizedModel),
		SessionKey:      uuid.NewString(),
		MessageCount:    0,
		Status:          "active",
		ContextPolicy:   buildContextPolicyJSON(s.cfg.Snapshot()),
		LastCompactedAt: nil,
		LastResponseID:  "",
	}
	if err := s.repo.CreateConversation(ctx, item); err != nil {
		return nil, err
	}
	if project != nil {
		item.ProjectPublicID = project.PublicID
		item.ProjectName = project.Name
		item.ProjectSystemPrompt = project.SystemPrompt
	}
	return item, nil
}

// ListConversations 分页查询会话。
func (s *Service) ListConversations(ctx context.Context, input ListConversationsInput) ([]model.Conversation, int64, error) {
	offset, limit := pagination.Offset(input.Page, input.PageSize)
	return s.repo.ListConversationsByUser(ctx, repository.ConversationListInput{
		UserID:        input.UserID,
		Offset:        offset,
		Limit:         limit,
		StatusFilter:  input.StatusFilter,
		StarredFilter: input.StarredFilter,
		ShareFilter:   input.ShareFilter,
		ProjectFilter: normalizeConversationProjectFilter(input.ProjectFilter),
		SearchQuery:   input.SearchQuery,
	})
}

// SearchConversations 分页搜索当前用户的会话，并通过前瞻记录判断是否还有下一页。
func (s *Service) SearchConversations(
	ctx context.Context,
	userID uint,
	page int,
	pageSize int,
	searchQuery string,
) ([]ConversationSearchResult, bool, error) {
	offset, limit := pagination.Offset(page, pageSize)
	items, err := s.repo.ListConversationsForSearch(
		ctx,
		userID,
		offset,
		limit+1,
		searchQuery,
	)
	if err != nil || len(items) == 0 {
		return []ConversationSearchResult{}, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	results := make([]ConversationSearchResult, 0, len(items))
	for _, item := range items {
		results = append(results, ConversationSearchResult{
			Conversation: item,
		})
	}
	return results, hasMore, nil
}

// ListMessages 查询会话消息（分页）。
func (s *Service) ListMessages(ctx context.Context, userID uint, conversationID uint, page int, pageSize int) ([]model.Message, int64, error) {
	if _, err := s.repo.GetConversationByUser(ctx, conversationID, userID); err != nil {
		return nil, 0, ErrConversationNotFound
	}

	offset, limit := pagination.Offset(page, pageSize)
	items, total, err := s.repo.ListMessages(ctx, conversationID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	if err = s.hydrateMessageFeedback(ctx, userID, items); err != nil {
		return nil, 0, err
	}
	if err = s.hydrateMessageProcessTraces(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListMessagesBeforeID 查询指定消息 ID 之前的一页会话消息。
func (s *Service) ListMessagesBeforeID(ctx context.Context, userID uint, conversationID uint, beforeID uint, pageSize int) ([]model.Message, int64, error) {
	if beforeID == 0 {
		return []model.Message{}, 0, nil
	}
	if _, err := s.repo.GetConversationByUser(ctx, conversationID, userID); err != nil {
		return nil, 0, ErrConversationNotFound
	}

	_, limit := pagination.Normalize(1, pageSize)
	items, total, err := s.repo.ListMessagesBeforeID(ctx, conversationID, beforeID, limit)
	if err != nil {
		return nil, 0, err
	}
	if err = s.hydrateMessageFeedback(ctx, userID, items); err != nil {
		return nil, 0, err
	}
	if err = s.hydrateMessageProcessTraces(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ExportConversation 查询单会话完整导出数据。
func (s *Service) ExportConversation(ctx context.Context, userID uint, publicID string) (*ConversationExportResult, error) {
	conversation, err := s.repo.GetConversationByPublicID(ctx, publicID, userID)
	if err != nil {
		return nil, ErrConversationNotFound
	}

	items, err := s.repo.ListAllMessages(ctx, conversation.ID)
	if err != nil {
		return nil, err
	}
	if err = s.hydrateMessageFeedback(ctx, userID, items); err != nil {
		return nil, err
	}
	if err = s.hydrateMessageProcessTraces(ctx, items); err != nil {
		return nil, err
	}

	runs, err := s.repo.ListConversationRunsByRunIDs(ctx, userID, conversation.ID, model.CollectMessageRunIDs(items))
	if err != nil {
		return nil, err
	}

	return &ConversationExportResult{
		Version:                 conversationExportVersion,
		ExportScope:             conversationExportScopeFull,
		ExportedAt:              time.Now().UTC(),
		Conversation:            conversation,
		Messages:                items,
		Runs:                    runs,
		TotalMessages:           int64(len(items)),
		TotalRuns:               int64(len(runs)),
		DefaultMessagePublicIDs: exportDefaultMessagePublicIDs(items),
	}, nil
}

// ListAllConversationsAfterID 按主键游标分页列出会话（管理员导出用）。
func (s *Service) ListAllConversationsAfterID(ctx context.Context, afterID uint, limit int) ([]model.Conversation, error) {
	return s.repo.ListAllConversationsAfterID(ctx, afterID, limit)
}

// ListUserConversationsAfterID 按主键游标分页列出指定用户的会话。
func (s *Service) ListUserConversationsAfterID(ctx context.Context, userID uint, afterID uint, limit int) ([]model.Conversation, error) {
	return s.repo.ListUserConversationsAfterID(ctx, userID, afterID, limit)
}

// ExportUserConversationData 导出单会话完整数据，校验用户归属。
func (s *Service) ExportUserConversationData(ctx context.Context, userID uint, conversation *model.Conversation) (*ConversationExportResult, error) {
	if conversation.UserID != userID {
		return nil, ErrConversationNotFound
	}
	return s.ExportConversationData(ctx, conversation)
}

// ExportConversationData 导出单会话完整数据，不做用户归属校验（管理员用）。
func (s *Service) ExportConversationData(ctx context.Context, conversation *model.Conversation) (*ConversationExportResult, error) {
	items, err := s.repo.ListAllMessages(ctx, conversation.ID)
	if err != nil {
		return nil, err
	}

	var runs []model.Run
	runIDs := model.CollectMessageRunIDs(items)
	if len(runIDs) > 0 && conversation.UserID > 0 {
		var runsErr error
		runs, runsErr = s.repo.ListConversationRunsByRunIDs(ctx, conversation.UserID, conversation.ID, runIDs)
		if runsErr != nil && s.logger != nil {
			s.logger.Warn("export_conversation_runs_failed", zap.Uint("conversation_id", conversation.ID), zap.Error(runsErr))
		}
	}
	if runs == nil {
		runs = []model.Run{}
	}

	return &ConversationExportResult{
		Version:                 conversationExportVersion,
		ExportScope:             conversationExportScopeFull,
		ExportedAt:              time.Now().UTC(),
		Conversation:            conversation,
		Messages:                items,
		Runs:                    runs,
		TotalMessages:           int64(len(items)),
		TotalRuns:               int64(len(runs)),
		DefaultMessagePublicIDs: exportDefaultMessagePublicIDs(items),
	}, nil
}

func exportDefaultMessagePublicIDs(items []model.Message) []string {
	return publicIDsFromMessages(buildLatestVisibleMessages(items))
}

// ListRecentMessages 查询会话最近消息窗口，供对话页恢复最新上下文。
func (s *Service) ListRecentMessages(ctx context.Context, userID uint, conversationID uint, limit int) ([]model.Message, int64, error) {
	if _, err := s.repo.GetConversationByUser(ctx, conversationID, userID); err != nil {
		return nil, 0, ErrConversationNotFound
	}

	normalizedLimit := normalizeRecentMessageLimit(limit)
	items, total, err := s.repo.ListRecentMessages(ctx, conversationID, normalizedLimit)
	if err != nil {
		return nil, 0, err
	}
	if err = s.hydrateMessageFeedback(ctx, userID, items); err != nil {
		return nil, 0, err
	}
	if err = s.hydrateMessageProcessTraces(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListConversationPreviewMessages 返回最新分支末尾的可见消息，供搜索结果按需预览。
func (s *Service) ListConversationPreviewMessages(ctx context.Context, userID uint, conversationPublicID string) ([]model.Message, error) {
	conversation, err := s.repo.GetConversationByPublicID(ctx, strings.TrimSpace(conversationPublicID), userID)
	if err != nil {
		return nil, ErrConversationNotFound
	}
	return s.repo.ListLatestBranchPreviewMessages(
		ctx,
		conversation.ID,
		conversationPreviewAncestorMaxDepth,
		conversationPreviewMessageLimit,
	)
}

// GetConversationByPublicID 查询用户会话元信息（公开 ID）。
func (s *Service) GetConversationByPublicID(ctx context.Context, userID uint, publicID string) (*model.Conversation, error) {
	item, err := s.repo.GetConversationByPublicID(ctx, publicID, userID)
	if err != nil {
		return nil, ErrConversationNotFound
	}
	return item, nil
}

// SetMessageFeedback 设置当前用户对消息的点赞/点踩反馈。
func (s *Service) SetMessageFeedback(
	ctx context.Context,
	userID uint,
	messagePublicID string,
	feedback string,
) (*MessageFeedbackResult, error) {
	normalizedPublicID := strings.TrimSpace(messagePublicID)
	if normalizedPublicID == "" {
		return nil, ErrMessageNotFound
	}

	normalizedFeedback := normalizeMessageFeedback(feedback)
	if feedback != "" && normalizedFeedback == "" {
		return nil, ErrInvalidMessageFeedback
	}

	message, err := s.repo.GetMessageByPublicIDForUser(ctx, userID, normalizedPublicID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}
	if message.Role != "assistant" {
		return nil, ErrMessageFeedbackTargetInvalid
	}

	if normalizedFeedback == "" {
		if err = s.repo.DeleteMessageFeedback(ctx, userID, message.ID); err != nil {
			return nil, err
		}
	} else {
		if err = s.repo.UpsertMessageFeedback(ctx, &model.MessageFeedback{
			UserID:         userID,
			ConversationID: message.ConversationID,
			MessageID:      message.ID,
			Feedback:       normalizedFeedback,
		}); err != nil {
			return nil, err
		}
	}

	items := []model.Message{*message}
	if err = s.hydrateMessageFeedback(ctx, userID, items); err != nil {
		return nil, err
	}
	enriched := items[0]

	return &MessageFeedbackResult{
		MessageID:       enriched.ID,
		MessagePublicID: enriched.PublicID,
		MyFeedback:      enriched.MyFeedback,
		ThumbsUpCount:   enriched.ThumbsUpCount,
		ThumbsDownCount: enriched.ThumbsDownCount,
	}, nil
}

// UpdateAssistantMessageContent 更新当前用户的一条 assistant 消息正文。
func (s *Service) UpdateAssistantMessageContent(
	ctx context.Context,
	userID uint,
	messagePublicID string,
	content string,
) (*model.Message, error) {
	normalizedPublicID := strings.TrimSpace(messagePublicID)
	if normalizedPublicID == "" {
		return nil, ErrMessageNotFound
	}
	normalizedContent := strings.TrimSpace(content)
	if normalizedContent == "" {
		return nil, ErrInvalidMessageContent
	}

	message, err := s.repo.GetMessageByPublicIDForUser(ctx, userID, normalizedPublicID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}
	if message.Role != "assistant" {
		return nil, ErrMessageEditTargetInvalid
	}
	if message.Status == "pending" {
		return nil, ErrMessageEditStateInvalid
	}

	updated, err := s.repo.UpdateAssistantMessageContent(ctx, userID, normalizedPublicID, normalizedContent, time.Now())
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}
	items := []model.Message{*updated}
	if err = s.hydrateMessageFeedback(ctx, userID, items); err != nil {
		return nil, err
	}
	if err = s.hydrateMessageProcessTraces(ctx, items); err != nil {
		return nil, err
	}
	updated = &items[0]
	return updated, nil
}

// RenameConversation 重命名会话。
func (s *Service) RenameConversation(ctx context.Context, userID uint, publicID string, title string) (*model.Conversation, error) {
	normalizedTitle := strings.TrimSpace(title)
	if normalizedTitle == "" {
		return nil, ErrInvalidConversationTitle
	}
	item, err := s.repo.UpdateConversationTitleByPublicID(ctx, userID, publicID, normalizedTitle)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	return item, nil
}

// SetConversationStar 设置会话星标状态。
func (s *Service) SetConversationStar(ctx context.Context, userID uint, publicID string, starred bool) (*model.Conversation, error) {
	item, err := s.repo.UpdateConversationStarByPublicID(ctx, userID, publicID, starred)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	return item, nil
}

// SetConversationArchived 设置会话归档状态。
func (s *Service) SetConversationArchived(ctx context.Context, userID uint, publicID string, archived bool) (*model.Conversation, error) {
	item, err := s.repo.UpdateConversationArchiveByPublicID(ctx, userID, publicID, archived)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	return item, nil
}

// DeleteConversation 删除会话（软删除），并按需清理不再被其他会话引用的文件。
func (s *Service) DeleteConversation(ctx context.Context, userID uint, publicID string, options DeleteConversationOptions) (*DeleteConversationResult, error) {
	cleanupFileIDs, err := s.repo.DeleteConversationByPublicID(ctx, userID, publicID, options.DeleteFiles)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	result := &DeleteConversationResult{Deleted: true}
	if options.DeleteFiles {
		result.DeletedFileCount, result.Quota = s.deleteConversationFiles(ctx, userID, cleanupFileIDs)
	}
	return result, nil
}

// GetConversation 查询用户会话元信息。
func (s *Service) GetConversation(ctx context.Context, userID uint, conversationID uint) (*model.Conversation, error) {
	item, err := s.repo.GetConversationByUser(ctx, conversationID, userID)
	if err != nil {
		return nil, ErrConversationNotFound
	}
	return item, nil
}

// ListConversationRuns 分页查询会话运行日志。
func (s *Service) ListConversationRuns(
	ctx context.Context,
	userID uint,
	conversationID uint,
	page int,
	pageSize int,
) ([]model.Run, int64, error) {
	if _, err := s.repo.GetConversationByUser(ctx, conversationID, userID); err != nil {
		return nil, 0, ErrConversationNotFound
	}

	offset, limit := pagination.Offset(page, pageSize)
	return s.repo.ListConversationRuns(ctx, userID, conversationID, offset, limit)
}

// GetConversationSystemDefaultModel 返回后台配置的新会话系统推荐模型。
func (s *Service) GetConversationSystemDefaultModel() string {
	if s == nil || s.cfg == nil {
		return ""
	}
	cfg := s.cfg.Snapshot()
	return strings.TrimSpace(cfg.ConversationDefaultModel)
}

// EventLogListFilter 描述管理员对话事件筛选和排序条件。
type EventLogListFilter struct {
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

// ListConversationEventLogs 分页查询管理员对话事件。
func (s *Service) ListConversationEventLogs(ctx context.Context, page int, pageSize int, filter EventLogListFilter) ([]model.EventLog, int64, error) {
	offset, limit := pagination.Offset(page, pageSize)
	return s.repo.ListConversationEventLogs(ctx, repository.ConversationEventLogListFilter{
		Query:          filter.Query,
		EventScope:     filter.EventScope,
		EventType:      filter.EventType,
		Status:         filter.Status,
		UserID:         filter.UserID,
		ConversationID: filter.ConversationID,
		CreatedFrom:    filter.CreatedFrom,
		CreatedTo:      filter.CreatedTo,
		Sort:           filter.Sort,
	}, offset, limit)
}

// GetConversationEventLog 查询单条管理员对话事件详情。
func (s *Service) GetConversationEventLog(ctx context.Context, eventID uint) (*model.EventLog, error) {
	if eventID == 0 {
		return nil, ErrConversationEventNotFound
	}
	item, err := s.repo.GetConversationEventLog(ctx, eventID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationEventNotFound
		}
		return nil, err
	}
	return item, nil
}

// GetConversationToolCallDetail 查询当前用户指定运行内的工具调用结果详情。
func (s *Service) GetConversationToolCallDetail(
	ctx context.Context,
	userID uint,
	runID string,
	toolCallID string,
) (*model.ToolCallDetail, error) {
	runID = strings.TrimSpace(runID)
	toolCallID = strings.TrimSpace(toolCallID)
	if userID == 0 || runID == "" || toolCallID == "" {
		return nil, ErrToolCallNotFound
	}
	item, err := s.repo.GetConversationToolCallDetail(ctx, userID, runID, toolCallID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrToolCallNotFound
		}
		return nil, err
	}
	return item, nil
}

// ListConversationRunsByRunIDs 批量查询消息对应的运行快照。
func (s *Service) ListConversationRunsByRunIDs(
	ctx context.Context,
	userID uint,
	conversationID uint,
	runIDs []string,
) ([]model.Run, error) {
	if len(runIDs) == 0 {
		return nil, nil
	}
	if _, err := s.repo.GetConversationByUser(ctx, conversationID, userID); err != nil {
		return nil, ErrConversationNotFound
	}
	return s.repo.ListConversationRunsByRunIDs(ctx, userID, conversationID, runIDs)
}

// ListConversationRunStatusesByRunIDs 批量查询当前用户的运行状态。
func (s *Service) ListConversationRunStatusesByRunIDs(
	ctx context.Context,
	userID uint,
	runIDs []string,
) ([]model.RunStatus, error) {
	normalized := make([]string, 0, len(runIDs))
	seen := make(map[string]struct{}, len(runIDs))
	for _, rawRunID := range runIDs {
		runID := strings.TrimSpace(rawRunID)
		if runID == "" {
			continue
		}
		if _, exists := seen[runID]; exists {
			continue
		}
		seen[runID] = struct{}{}
		normalized = append(normalized, runID)
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	return s.repo.ListConversationRunStatusesByRunIDs(ctx, userID, normalized)
}

func normalizeRecentMessageLimit(limit int) int {
	_, normalizedLimit := pagination.Normalize(1, limit)
	return normalizedLimit
}
