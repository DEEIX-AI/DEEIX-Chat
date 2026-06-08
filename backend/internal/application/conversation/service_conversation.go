package conversation

import (
	"context"
	"errors"
	"strings"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/google/uuid"
)

const (
	defaultPageSize             = 20
	maxPageSize                 = 100
	maxMessagePageSize          = 1000
	conversationExportVersion   = 1
	conversationExportScopeFull = "full"
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

// CreateConversation 创建用户新会话。
func (s *Service) CreateConversation(ctx context.Context, userID uint, title string, modelName string, projectPublicID string) (*model.Conversation, error) {
	normalizedTitle := strings.TrimSpace(title)
	if normalizedTitle == "" {
		normalizedTitle = "新会话"
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
func (s *Service) ListConversations(
	ctx context.Context,
	userID uint,
	page int,
	pageSize int,
	statusFilter string,
	starredFilter string,
	shareFilter string,
	projectFilter string,
) ([]model.Conversation, int64, error) {
	offset, limit := normalizePage(page, pageSize)
	return s.repo.ListConversationsByUser(ctx, userID, offset, limit, statusFilter, starredFilter, shareFilter, normalizeConversationProjectFilter(projectFilter))
}

// ListMessages 查询会话消息（分页）。
func (s *Service) ListMessages(ctx context.Context, userID uint, conversationID uint, page int, pageSize int) ([]model.Message, int64, error) {
	if _, err := s.repo.GetConversationByUser(ctx, conversationID, userID); err != nil {
		return nil, 0, ErrConversationNotFound
	}

	offset, limit := normalizeMessagePage(page, pageSize)
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

	_, limit := normalizeMessagePage(1, pageSize)
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

	runs, err := s.repo.ListConversationRunsByRunIDs(ctx, userID, conversation.ID, collectExportMessageRunIDs(items))
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

func exportDefaultMessagePublicIDs(items []model.Message) []string {
	return publicIDsFromMessages(buildLatestVisibleMessages(items))
}

func collectExportMessageRunIDs(items []model.Message) []string {
	seen := make(map[string]struct{}, len(items))
	runIDs := make([]string, 0, len(items))
	for _, item := range items {
		runID := strings.TrimSpace(item.RunID)
		if runID == "" {
			continue
		}
		if _, ok := seen[runID]; ok {
			continue
		}
		seen[runID] = struct{}{}
		runIDs = append(runIDs, runID)
	}
	return runIDs
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

	offset, limit := normalizePage(page, pageSize)
	return s.repo.ListConversationRuns(ctx, userID, conversationID, offset, limit)
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

func normalizePage(page int, pageSize int) (int, int) {
	return normalizePageWithMax(page, pageSize, maxPageSize)
}

func normalizeMessagePage(page int, pageSize int) (int, int) {
	return normalizePageWithMax(page, pageSize, maxMessagePageSize)
}

func normalizeRecentMessageLimit(limit int) int {
	_, normalizedLimit := normalizeMessagePage(1, limit)
	return normalizedLimit
}

func normalizePageWithMax(page int, pageSize int, maxAllowedPageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if maxAllowedPageSize > 0 && pageSize > maxAllowedPageSize {
		pageSize = maxAllowedPageSize
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	return offset, pageSize
}
