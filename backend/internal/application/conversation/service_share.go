package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	conversationShareStatusNone    = "none"
	conversationShareStatusActive  = "active"
	conversationShareStatusRevoked = "revoked"
)

// ConversationShareResult 是当前用户管理分享时返回的分享状态。
type ConversationShareResult struct {
	ShareID        string
	Status         string
	TitleSnapshot  string
	ModelSnapshot  string
	MessageCount   int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	RevokedAt      *time.Time
	LastAccessedAt *time.Time
}

// PublicSharedConversationResult 是公开分享页可读取的快照内容。
type PublicSharedConversationResult struct {
	ShareID           string
	Title             string
	Model             string
	CreatedAt         time.Time
	LastAccessedAt    *time.Time
	Messages          []model.Message
	RunModels         map[string]PublicSharedRunModel
	DefaultMessageIDs []string
}

// PublicSharedRunModel 是公开分享页可展示的模型快照。
type PublicSharedRunModel struct {
	PlatformModelName string
	UpstreamModelName string
	ModelVendor       string
	ModelIcon         string
}

type sharedAttachmentSnapshot struct {
	FileID                 string `json:"file_id"`
	Kind                   string `json:"kind"`
	FileName               string `json:"file_name"`
	MimeType               string `json:"mime_type"`
	DetectedMIME           string `json:"detected_mime"`
	FileCategory           string `json:"file_category"`
	FileSize               int64  `json:"file_size"`
	ProcessingStatus       string `json:"processing_status"`
	ProcessingReady        bool   `json:"processing_ready"`
	ProcessingErrorCode    string `json:"processing_error_code"`
	ProcessingErrorMessage string `json:"processing_error_message"`
}

// GetConversationShare 查询当前会话最近一次分享状态。
func (s *Service) GetConversationShare(ctx context.Context, userID uint, conversationPublicID string) (*ConversationShareResult, error) {
	conversation, err := s.repo.GetConversationByPublicID(ctx, strings.TrimSpace(conversationPublicID), userID)
	if err != nil {
		return nil, ErrConversationNotFound
	}
	share, err := s.repo.GetLatestConversationShareByConversation(ctx, userID, conversation.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &ConversationShareResult{
				Status: conversationShareStatusNone,
			}, nil
		}
		return nil, err
	}
	return toConversationShareResult(share), nil
}

// CreateConversationShare 创建公开快照，默认包含会话全部分支消息。
func (s *Service) CreateConversationShare(ctx context.Context, userID uint, conversationPublicID string, defaultMessagePublicIDs []string) (*ConversationShareResult, error) {
	return s.createConversationShare(ctx, userID, conversationPublicID, defaultMessagePublicIDs, false)
}

// RegenerateConversationShare 关闭旧链接并生成新的公开快照。
func (s *Service) RegenerateConversationShare(ctx context.Context, userID uint, conversationPublicID string, defaultMessagePublicIDs []string) (*ConversationShareResult, error) {
	return s.createConversationShare(ctx, userID, conversationPublicID, defaultMessagePublicIDs, true)
}

func (s *Service) createConversationShare(ctx context.Context, userID uint, conversationPublicID string, defaultMessagePublicIDs []string, regenerated bool) (*ConversationShareResult, error) {
	conversation, err := s.repo.GetConversationByPublicID(ctx, strings.TrimSpace(conversationPublicID), userID)
	if err != nil {
		return nil, ErrConversationNotFound
	}

	messageIDs, defaultMessageIDs, err := s.resolveShareMessageIDs(ctx, conversation.ID, defaultMessagePublicIDs)
	if err != nil {
		return nil, err
	}
	if len(messageIDs) == 0 {
		return nil, ErrInvalidConversationShare
	}

	encoded, err := json.Marshal(messageIDs)
	if err != nil {
		return nil, err
	}
	encodedDefault, err := json.Marshal(defaultMessageIDs)
	if err != nil {
		return nil, err
	}
	var regeneratedAt *time.Time
	if regenerated {
		now := time.Now().UTC()
		regeneratedAt = &now
	}
	share := &model.ConversationShare{
		ShareID:               normalizePublicID(uuid.NewString()),
		ConversationID:        conversation.ID,
		UserID:                userID,
		Status:                conversationShareStatusActive,
		TitleSnapshot:         conversation.Title,
		ModelSnapshot:         conversation.Model,
		MessageIDsJSON:        string(encoded),
		DefaultMessageIDsJSON: string(encodedDefault),
		RegeneratedAt:         regeneratedAt,
	}
	if err = s.repo.ReplaceActiveConversationShare(ctx, share); err != nil {
		if s.logger != nil {
			s.logger.Error("replace_conversation_share_failed",
				zap.Uint("user_id", userID),
				zap.Uint("conversation_id", conversation.ID),
				zap.Bool("regenerated", regenerated),
				zap.Int("message_count", len(messageIDs)),
				zap.Error(err),
			)
		}
		return nil, normalizeConversationSharePersistenceError(err)
	}
	return toConversationShareResult(share), nil
}

func normalizeConversationSharePersistenceError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "default_message_ids_json") && strings.Contains(message, "does not exist") {
		return ErrConversationShareSchemaOutdated
	}
	return err
}

// RevokeConversationShare 关闭单个会话的公开分享。
func (s *Service) RevokeConversationShare(ctx context.Context, userID uint, conversationPublicID string) (*ConversationShareResult, error) {
	conversation, err := s.repo.GetConversationByPublicID(ctx, strings.TrimSpace(conversationPublicID), userID)
	if err != nil {
		return nil, ErrConversationNotFound
	}
	if err = s.repo.RevokeActiveConversationShares(ctx, userID, []uint{conversation.ID}); err != nil {
		return nil, err
	}
	share, err := s.repo.GetLatestConversationShareByConversation(ctx, userID, conversation.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &ConversationShareResult{Status: conversationShareStatusNone}, nil
		}
		return nil, err
	}
	return toConversationShareResult(share), nil
}

// RevokeConversationShares 批量关闭会话公开分享。
func (s *Service) RevokeConversationShares(ctx context.Context, userID uint, conversationPublicIDs []string) error {
	conversationIDs := make([]uint, 0, len(conversationPublicIDs))
	seen := make(map[string]struct{}, len(conversationPublicIDs))
	for _, publicID := range conversationPublicIDs {
		normalized := strings.TrimSpace(publicID)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		conversation, err := s.repo.GetConversationByPublicID(ctx, normalized, userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrConversationNotFound
			}
			return err
		}
		conversationIDs = append(conversationIDs, conversation.ID)
	}
	if len(conversationIDs) == 0 {
		return nil
	}
	return s.repo.RevokeActiveConversationShares(ctx, userID, conversationIDs)
}

// GetPublicSharedConversation 读取公开分享快照。原会话软删除后，仓储查询会自然返回不存在。
func (s *Service) GetPublicSharedConversation(ctx context.Context, shareID string) (*PublicSharedConversationResult, error) {
	normalizedShareID := strings.TrimSpace(shareID)
	if normalizedShareID == "" {
		return nil, ErrConversationShareNotFound
	}
	share, conversation, err := s.repo.GetActiveConversationShareByShareID(ctx, normalizedShareID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationShareNotFound
		}
		return nil, err
	}
	messageIDs := decodeShareMessageIDs(share.MessageIDsJSON)
	if len(messageIDs) == 0 {
		return nil, ErrConversationShareNotFound
	}
	messages, err := s.repo.ListMessagesForShare(ctx, conversation.ID, messageIDs)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, ErrConversationShareNotFound
	}
	if err = s.hydrateMessageProcessTraces(ctx, messages); err != nil {
		return nil, err
	}
	runModels, err := s.loadPublicSharedRunModels(ctx, share.UserID, conversation.ID, messages)
	if err != nil {
		return nil, err
	}
	_ = s.repo.TouchConversationShareAccess(ctx, share.ShareID, time.Now().UTC())
	title := strings.TrimSpace(share.TitleSnapshot)
	if title == "" {
		title = conversation.Title
	}
	platformModel := strings.TrimSpace(share.ModelSnapshot)
	if platformModel == "" {
		platformModel = conversation.Model
	}
	return &PublicSharedConversationResult{
		ShareID:           share.ShareID,
		Title:             title,
		Model:             platformModel,
		CreatedAt:         share.CreatedAt,
		LastAccessedAt:    share.LastAccessedAt,
		Messages:          messages,
		RunModels:         runModels,
		DefaultMessageIDs: resolvePublicDefaultMessageIDs(share.DefaultMessageIDsJSON, messages),
	}, nil
}

// CloneSharedConversation 将公开分享快照克隆到当前登录用户账户。
func (s *Service) CloneSharedConversation(ctx context.Context, userID uint, shareID string) (*model.Conversation, error) {
	normalizedShareID := strings.TrimSpace(shareID)
	if normalizedShareID == "" {
		return nil, ErrConversationShareNotFound
	}
	share, sourceConversation, err := s.repo.GetActiveConversationShareByShareID(ctx, normalizedShareID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationShareNotFound
		}
		return nil, err
	}
	messageIDs := decodeShareMessageIDs(share.MessageIDsJSON)
	if len(messageIDs) == 0 {
		return nil, ErrConversationShareNotFound
	}
	messages, err := s.repo.ListMessagesForShare(ctx, sourceConversation.ID, messageIDs)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, ErrConversationShareNotFound
	}
	if err = s.hydrateMessageProcessTraces(ctx, messages); err != nil {
		return nil, err
	}

	clonedFiles, err := s.cloneSharedFiles(ctx, share.UserID, userID, messages)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(share.TitleSnapshot)
	if title == "" {
		title = sourceConversation.Title
	}
	platformModel := strings.TrimSpace(share.ModelSnapshot)
	if platformModel == "" {
		platformModel = sourceConversation.Model
	}
	targetConversation := &model.Conversation{
		UserID:          userID,
		PublicID:        normalizePublicID(uuid.NewString()),
		Title:           title,
		LabelsJSON:      "[]",
		Model:           platformModel,
		Provider:        inferProvider(platformModel),
		SessionKey:      uuid.NewString(),
		MessageCount:    0,
		Status:          "active",
		ContextPolicy:   buildContextPolicyJSON(s.cfg.Snapshot()),
		LastCompactedAt: nil,
		LastResponseID:  "",
	}
	if err = s.repo.CreateConversation(ctx, targetConversation); err != nil {
		return nil, err
	}

	runIDMap, err := s.cloneSharedRuns(ctx, share.UserID, sourceConversation.ID, userID, targetConversation.ID, messages)
	if err != nil {
		return nil, err
	}

	defaultMessageIDs := resolvePublicDefaultMessageIDs(share.DefaultMessageIDsJSON, messages)
	orderedMessages := orderSharedMessagesForClone(messages, defaultMessageIDs)
	clonedMessageIDs := make(map[string]uint, len(orderedMessages))
	for _, sourceMessage := range orderedMessages {
		clonedRunID := runIDMap[strings.TrimSpace(sourceMessage.RunID)]
		if clonedRunID == "" && sourceMessage.ProcessTrace != nil {
			clonedRunID = "run_" + normalizePublicID(uuid.NewString())
		}
		clonedMessage, err := s.cloneSharedMessage(ctx, userID, targetConversation.ID, sourceMessage, clonedRunID, clonedMessageIDs)
		if err != nil {
			return nil, err
		}
		clonedMessageIDs[sourceMessage.PublicID] = clonedMessage.ID
		if err = s.cloneSharedMessageAttachments(ctx, userID, targetConversation.ID, clonedMessage.ID, sourceMessage.Attachments, clonedFiles); err != nil {
			return nil, err
		}
		if err = s.cloneSharedMessageTrace(ctx, userID, targetConversation.ID, clonedMessage.ID, clonedRunID, sourceMessage.ProcessTrace); err != nil {
			return nil, err
		}
	}
	if err = s.repo.IncrementMessageCount(ctx, targetConversation.ID, len(orderedMessages)); err != nil {
		return nil, err
	}
	targetConversation.MessageCount = len(orderedMessages)
	return targetConversation, nil
}

func (s *Service) cloneSharedRuns(
	ctx context.Context,
	sourceUserID uint,
	sourceConversationID uint,
	targetUserID uint,
	targetConversationID uint,
	messages []model.Message,
) (map[string]string, error) {
	sourceRunIDs := make([]string, 0, len(messages))
	seen := make(map[string]struct{}, len(messages))
	for _, message := range messages {
		runID := strings.TrimSpace(message.RunID)
		if runID == "" {
			continue
		}
		if _, exists := seen[runID]; exists {
			continue
		}
		seen[runID] = struct{}{}
		sourceRunIDs = append(sourceRunIDs, runID)
	}
	if len(sourceRunIDs) == 0 {
		return map[string]string{}, nil
	}
	runs, err := s.repo.ListConversationRunsByRunIDs(ctx, sourceUserID, sourceConversationID, sourceRunIDs)
	if err != nil {
		return nil, err
	}
	byRunID := make(map[string]model.Run, len(runs))
	for _, run := range runs {
		runID := strings.TrimSpace(run.RunID)
		if runID != "" {
			byRunID[runID] = run
		}
	}
	result := make(map[string]string, len(byRunID))
	for _, sourceRunID := range sourceRunIDs {
		sourceRun, ok := byRunID[sourceRunID]
		if !ok {
			continue
		}
		clonedRunID := "run_" + normalizePublicID(uuid.NewString())
		cloned := sourceRun
		cloned.ID = 0
		cloned.RunID = clonedRunID
		cloned.RequestID = normalizePublicID(uuid.NewString())
		cloned.UserID = targetUserID
		cloned.ConversationID = targetConversationID
		if cloned.StartedAt.IsZero() {
			cloned.StartedAt = time.Now().UTC()
		}
		if err := s.repo.CreateConversationRun(ctx, &cloned); err != nil {
			return nil, err
		}
		result[sourceRunID] = clonedRunID
	}
	return result, nil
}

func (s *Service) cloneSharedFiles(
	ctx context.Context,
	sourceUserID uint,
	targetUserID uint,
	messages []model.Message,
) (map[string]*model.FileObject, error) {
	fileIDs := collectSharedAttachmentFileIDs(messages)
	if len(fileIDs) == 0 {
		return map[string]*model.FileObject{}, nil
	}
	cloned := make(map[string]*model.FileObject, len(fileIDs))
	quotaLimit := s.cfg.Snapshot().UserStorageQuotaBytes
	for _, sourceFileID := range fileIDs {
		source, err := s.repo.GetActiveFileObjectByID(ctx, sourceUserID, sourceFileID)
		if err != nil {
			if isFileNotFoundError(err) {
				return nil, ErrFileNotFound
			}
			return nil, err
		}
		target := cloneFileObjectForUser(source, targetUserID)
		if _, err = s.repo.CreateFileObjectAndConsumeQuota(ctx, target, quotaLimit); err != nil {
			if isStorageQuotaExceededError(err) {
				return nil, ErrStorageQuotaExceeded
			}
			return nil, err
		}
		if err = s.repo.CloneFileObjectProcessingState(ctx, source.ID, target.ID, targetUserID); err != nil {
			return nil, err
		}
		s.cloneOrTriggerEmbedding(ctx, source, target)
		cloned[sourceFileID] = target
	}
	return cloned, nil
}

func collectSharedAttachmentFileIDs(messages []model.Message) []string {
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, message := range messages {
		for _, attachment := range parseSharedAttachmentSnapshots(message.Attachments) {
			fileID := strings.TrimSpace(attachment.FileID)
			if fileID == "" {
				continue
			}
			if _, exists := seen[fileID]; exists {
				continue
			}
			seen[fileID] = struct{}{}
			result = append(result, fileID)
		}
	}
	return result
}

func cloneFileObjectForUser(source *model.FileObject, targetUserID uint) *model.FileObject {
	if source == nil {
		return &model.FileObject{}
	}
	target := *source
	target.ID = 0
	target.FileID = "file_" + normalizePublicID(uuid.NewString())
	target.UserID = targetUserID
	target.Status = "active"
	target.LastAccessedAt = nil
	target.CreatedAt = time.Time{}
	target.UpdatedAt = time.Time{}
	return &target
}

func (s *Service) cloneSharedMessage(
	ctx context.Context,
	userID uint,
	conversationID uint,
	source model.Message,
	clonedRunID string,
	clonedMessageIDs map[string]uint,
) (*model.Message, error) {
	var parentMessageID *uint
	if parentID, ok := clonedMessageIDs[strings.TrimSpace(source.ParentPublicID)]; ok {
		value := parentID
		parentMessageID = &value
	}
	var sourceMessageID *uint
	if sourceID, ok := clonedMessageIDs[strings.TrimSpace(source.SourcePublicID)]; ok {
		value := sourceID
		sourceMessageID = &value
	}
	branchReason := strings.TrimSpace(source.BranchReason)
	if branchReason == "" {
		branchReason = "default"
	}
	status := strings.TrimSpace(source.Status)
	if status == "" {
		status = "success"
	}
	contentType := strings.TrimSpace(source.ContentType)
	if contentType == "" {
		contentType = "text"
	}
	message := &model.Message{
		ConversationID:   conversationID,
		UserID:           userID,
		PublicID:         normalizePublicID(uuid.NewString()),
		ParentMessageID:  parentMessageID,
		RunID:            clonedRunID,
		Role:             strings.TrimSpace(source.Role),
		ContentType:      contentType,
		Content:          source.Content,
		BranchReason:     branchReason,
		SourceMessageID:  sourceMessageID,
		TokenUsage:       source.TokenUsage,
		InputTokens:      source.InputTokens,
		OutputTokens:     source.OutputTokens,
		CacheReadTokens:  source.CacheReadTokens,
		CacheWriteTokens: source.CacheWriteTokens,
		ReasoningTokens:  source.ReasoningTokens,
		LatencyMS:        source.LatencyMS,
		BilledCurrency:   "USD",
		BilledNanousd:    0,
		PricingSnapshot:  "",
		Status:           status,
		ErrorCode:        source.ErrorCode,
		ErrorMessage:     source.ErrorMessage,
	}
	if message.Role == "" {
		message.Role = "assistant"
	}
	if err := s.repo.CreateMessage(ctx, message); err != nil {
		return nil, err
	}
	return message, nil
}

func (s *Service) cloneSharedMessageAttachments(
	ctx context.Context,
	userID uint,
	conversationID uint,
	messageID uint,
	rawAttachments string,
	clonedFiles map[string]*model.FileObject,
) error {
	snapshots := parseSharedAttachmentSnapshots(rawAttachments)
	if len(snapshots) == 0 {
		return nil
	}
	now := time.Now().UTC()
	items := make([]model.Attachment, 0, len(snapshots))
	for _, snapshot := range snapshots {
		sourceFileID := strings.TrimSpace(snapshot.FileID)
		if sourceFileID == "" {
			continue
		}
		targetFile := clonedFiles[sourceFileID]
		if targetFile == nil {
			return ErrFileNotFound
		}
		kind := strings.TrimSpace(snapshot.Kind)
		if kind == "" {
			kind = "file"
		}
		fileName := strings.TrimSpace(snapshot.FileName)
		if fileName == "" {
			fileName = targetFile.FileName
		}
		mimeType := strings.TrimSpace(snapshot.MimeType)
		if mimeType == "" {
			mimeType = targetFile.MimeType
		}
		fileSize := snapshot.FileSize
		if fileSize <= 0 {
			fileSize = targetFile.SizeBytes
		}
		items = append(items, model.Attachment{
			ConversationID: conversationID,
			MessageID:      messageID,
			UserID:         userID,
			FileID:         targetFile.FileID,
			Kind:           kind,
			FileName:       fileName,
			MimeType:       mimeType,
			FileSize:       fileSize,
			SHA256:         targetFile.SHA256,
			StoragePath:    targetFile.StoragePath,
			Status:         "active",
			MetaJSON:       "",
			UploadedAt:     now,
		})
	}
	return s.repo.CreateAttachments(ctx, items)
}

func (s *Service) cloneSharedMessageTrace(
	ctx context.Context,
	userID uint,
	conversationID uint,
	messageID uint,
	runID string,
	trace *model.MessageProcessTrace,
) error {
	if trace == nil {
		return nil
	}
	startedAt := time.Now().UTC()
	if err := s.cloneSharedTraceBlock(ctx, userID, conversationID, messageID, runID, messageTraceTypeProcess, 1, startedAt, trace.Process); err != nil {
		return err
	}
	if err := s.cloneSharedTraceBlock(ctx, userID, conversationID, messageID, runID, messageTraceTypeTools, 2, startedAt, trace.Tools); err != nil {
		return err
	}
	if err := s.cloneSharedTraceBlock(ctx, userID, conversationID, messageID, runID, messageTraceTypeUpstreamThink, 3, startedAt, trace.UpstreamThink); err != nil {
		return err
	}
	for _, event := range trace.Events {
		eventID := strings.TrimSpace(event.EventID)
		if eventID == "" {
			eventID = "event_" + normalizePublicID(uuid.NewString())
		}
		eventStartedAt := event.StartedAt
		if eventStartedAt.IsZero() {
			eventStartedAt = startedAt
		}
		row := &model.MessageTraceEventRow{
			MessageID:       messageID,
			ConversationID:  conversationID,
			UserID:          userID,
			RunID:           runID,
			EventID:         eventID,
			EventType:       event.EventType,
			Phase:           event.Phase,
			Stage:           event.Stage,
			RoundID:         event.RoundID,
			ParentEventID:   event.ParentEventID,
			Status:          event.Status,
			Title:           event.Title,
			Summary:         event.Summary,
			ContentMarkdown: event.ContentMarkdown,
			PayloadJSON:     sanitizeSharedTracePayloadJSON(event.PayloadJSON),
			Seq:             event.Seq,
			StartedAt:       eventStartedAt,
			EndedAt:         event.EndedAt,
		}
		if err := s.repo.UpsertConversationMessageTraceEvent(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) cloneSharedTraceBlock(
	ctx context.Context,
	userID uint,
	conversationID uint,
	messageID uint,
	runID string,
	traceType string,
	seq int,
	startedAt time.Time,
	block *model.MessageTraceBlock,
) error {
	if block == nil {
		return nil
	}
	rowStartedAt := block.UpdatedAt
	if rowStartedAt.IsZero() {
		rowStartedAt = startedAt
	}
	return s.repo.UpsertConversationMessageTrace(ctx, &model.MessageTrace{
		MessageID:       messageID,
		ConversationID:  conversationID,
		UserID:          userID,
		RunID:           runID,
		TraceType:       traceType,
		Status:          block.Status,
		Stage:           block.Stage,
		RoundID:         block.RoundID,
		ParentEventID:   block.ParentEventID,
		Title:           block.Title,
		Summary:         block.Summary,
		ContentMarkdown: block.ContentMarkdown,
		PayloadJSON:     sanitizeSharedTracePayloadJSON(block.PayloadJSON),
		Seq:             seq,
		StartedAt:       rowStartedAt,
	})
}

func orderSharedMessagesForClone(messages []model.Message, defaultMessagePublicIDs []string) []model.Message {
	if len(messages) <= 1 {
		return append([]model.Message(nil), messages...)
	}
	byParent := make(map[string][]model.Message, len(messages))
	byPublicID := make(map[string]model.Message, len(messages))
	for _, message := range messages {
		publicID := strings.TrimSpace(message.PublicID)
		if publicID == "" {
			continue
		}
		parentKey := strings.TrimSpace(message.ParentPublicID)
		byParent[parentKey] = append(byParent[parentKey], message)
		byPublicID[publicID] = message
	}
	for parentKey := range byParent {
		sort.SliceStable(byParent[parentKey], func(i, j int) bool {
			return byParent[parentKey][i].ID < byParent[parentKey][j].ID
		})
	}
	defaultSelection := make(map[string]string, len(defaultMessagePublicIDs))
	for _, publicID := range defaultMessagePublicIDs {
		message, ok := byPublicID[strings.TrimSpace(publicID)]
		if !ok {
			continue
		}
		defaultSelection[strings.TrimSpace(message.ParentPublicID)] = message.PublicID
	}

	result := make([]model.Message, 0, len(messages))
	visited := make(map[string]struct{}, len(messages))
	var appendSubtree func(parentPublicID string)
	appendSubtree = func(parentPublicID string) {
		children := append([]model.Message(nil), byParent[parentPublicID]...)
		if len(children) == 0 {
			return
		}
		selectedPublicID := strings.TrimSpace(defaultSelection[parentPublicID])
		if selectedPublicID != "" {
			sort.SliceStable(children, func(i, j int) bool {
				leftSelected := children[i].PublicID == selectedPublicID
				rightSelected := children[j].PublicID == selectedPublicID
				if leftSelected != rightSelected {
					return !leftSelected && rightSelected
				}
				return children[i].ID < children[j].ID
			})
		}
		for _, child := range children {
			publicID := strings.TrimSpace(child.PublicID)
			if publicID == "" {
				continue
			}
			if _, exists := visited[publicID]; exists {
				continue
			}
			visited[publicID] = struct{}{}
			result = append(result, child)
			appendSubtree(publicID)
		}
	}
	appendSubtree("")
	if len(result) == len(byPublicID) {
		return result
	}
	for _, message := range messages {
		publicID := strings.TrimSpace(message.PublicID)
		if publicID == "" {
			continue
		}
		if _, exists := visited[publicID]; exists {
			continue
		}
		visited[publicID] = struct{}{}
		result = append(result, message)
	}
	return result
}

func parseSharedAttachmentSnapshots(raw string) []sharedAttachmentSnapshot {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	items := []sharedAttachmentSnapshot{}
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return nil
	}
	return items
}

func sanitizeSharedTracePayloadJSON(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	payload := map[string]interface{}{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return ""
	}
	deleteSharedTraceInternalFields(payload, "")
	if len(payload) == 0 {
		return ""
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func deleteSharedTraceInternalFields(payload map[string]interface{}, parentKey string) {
	for key, value := range payload {
		if isSharedTraceInternalField(key, parentKey) {
			delete(payload, key)
			continue
		}
		switch child := value.(type) {
		case map[string]interface{}:
			deleteSharedTraceInternalFields(child, key)
		case []interface{}:
			for _, item := range child {
				if itemMap, ok := item.(map[string]interface{}); ok {
					deleteSharedTraceInternalFields(itemMap, key)
				}
			}
		}
	}
}

func isSharedTraceInternalField(key string, parentKey string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(strings.TrimSpace(key)))
	parent := strings.ToLower(strings.TrimSpace(parentKey))
	if normalized == "upstreamname" || normalized == "upstreamdebug" ||
		normalized == "authorization" || normalized == "proxyauthorization" ||
		normalized == "cookie" || normalized == "setcookie" {
		return true
	}
	if parent == "upstream" && (normalized == "name" || normalized == "displayname") {
		return true
	}
	return strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "secretkey") ||
		strings.Contains(normalized, "accesskey")
}

func isFileNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, repository.ErrNotFound) || strings.Contains(strings.ToLower(err.Error()), "file not found")
}

func isStorageQuotaExceededError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrStorageQuotaExceeded) || strings.Contains(strings.ToLower(err.Error()), "storage quota exceeded")
}

func (s *Service) resolveShareMessageIDs(ctx context.Context, conversationID uint, defaultPublicIDs []string) ([]string, []string, error) {
	messages, err := s.repo.ListMessagesForShare(ctx, conversationID, nil)
	if err != nil {
		return nil, nil, err
	}
	allIDs := make([]string, 0, len(messages))
	available := make(map[string]struct{}, len(messages))
	for _, item := range messages {
		publicID := strings.TrimSpace(item.PublicID)
		if publicID != "" {
			allIDs = append(allIDs, publicID)
			available[publicID] = struct{}{}
		}
	}
	defaultIDs := normalizeMessagePublicIDs(defaultPublicIDs)
	if len(defaultIDs) > 0 {
		for _, publicID := range defaultIDs {
			if _, exists := available[publicID]; !exists {
				return nil, nil, ErrInvalidConversationShare
			}
		}
		return allIDs, defaultIDs, nil
	}
	return allIDs, publicIDsFromMessages(buildLatestVisibleShareMessages(messages)), nil
}

func normalizeMessagePublicIDs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func decodeShareMessageIDs(raw string) []string {
	ids := []string{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &ids); err != nil {
		return nil
	}
	return normalizeMessagePublicIDs(ids)
}

func resolvePublicDefaultMessageIDs(raw string, messages []model.Message) []string {
	decoded := decodeShareMessageIDs(raw)
	if len(decoded) > 0 {
		available := make(map[string]struct{}, len(messages))
		for _, message := range messages {
			publicID := strings.TrimSpace(message.PublicID)
			if publicID != "" {
				available[publicID] = struct{}{}
			}
		}
		result := make([]string, 0, len(decoded))
		for _, publicID := range decoded {
			if _, exists := available[publicID]; exists {
				result = append(result, publicID)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return publicIDsFromMessages(buildLatestVisibleShareMessages(messages))
}

func publicIDsFromMessages(messages []model.Message) []string {
	result := make([]string, 0, len(messages))
	for _, item := range messages {
		publicID := strings.TrimSpace(item.PublicID)
		if publicID != "" {
			result = append(result, publicID)
		}
	}
	return result
}

func buildLatestVisibleShareMessages(messages []model.Message) []model.Message {
	children := make(map[string][]model.Message)
	for _, item := range messages {
		parentKey := strings.TrimSpace(item.ParentPublicID)
		children[parentKey] = append(children[parentKey], item)
	}
	for parentKey := range children {
		sort.SliceStable(children[parentKey], func(i, j int) bool {
			return children[parentKey][i].ID < children[parentKey][j].ID
		})
	}

	visible := make([]model.Message, 0, len(messages))
	visited := make(map[string]struct{}, len(messages))
	parentKey := ""
	for {
		siblings := children[parentKey]
		if len(siblings) == 0 {
			break
		}
		selected := siblings[len(siblings)-1]
		publicID := strings.TrimSpace(selected.PublicID)
		if publicID == "" {
			break
		}
		if _, exists := visited[publicID]; exists {
			break
		}
		visited[publicID] = struct{}{}
		visible = append(visible, selected)
		parentKey = publicID
	}
	return visible
}

func (s *Service) loadPublicSharedRunModels(
	ctx context.Context,
	userID uint,
	conversationID uint,
	messages []model.Message,
) (map[string]PublicSharedRunModel, error) {
	runIDs := make([]string, 0, len(messages))
	seen := make(map[string]struct{}, len(messages))
	for _, item := range messages {
		runID := strings.TrimSpace(item.RunID)
		if runID == "" {
			continue
		}
		if _, exists := seen[runID]; exists {
			continue
		}
		seen[runID] = struct{}{}
		runIDs = append(runIDs, runID)
	}
	if len(runIDs) == 0 {
		return map[string]PublicSharedRunModel{}, nil
	}
	runs, err := s.repo.ListConversationRunsByRunIDs(ctx, userID, conversationID, runIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[string]PublicSharedRunModel, len(runs))
	for _, run := range runs {
		runID := strings.TrimSpace(run.RunID)
		if runID == "" {
			continue
		}
		result[runID] = PublicSharedRunModel{
			PlatformModelName: run.PlatformModelName,
			UpstreamModelName: run.UpstreamModelName,
			ModelVendor:       run.ModelVendor,
			ModelIcon:         run.ModelIcon,
		}
	}
	return result, nil
}

// OpenSharedConversationFileContent 按公开分享快照范围读取附件内容。
func (s *Service) OpenSharedConversationFileContent(ctx context.Context, shareID string, fileID string) (*appupload.FileContentResult, error) {
	normalizedShareID := strings.TrimSpace(shareID)
	normalizedFileID := strings.TrimSpace(fileID)
	if normalizedShareID == "" {
		return nil, ErrConversationShareNotFound
	}
	if normalizedFileID == "" {
		return nil, ErrInvalidFileReference
	}
	share, conversation, err := s.repo.GetActiveConversationShareByShareID(ctx, normalizedShareID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationShareNotFound
		}
		return nil, err
	}
	messageIDs := decodeShareMessageIDs(share.MessageIDsJSON)
	if len(messageIDs) == 0 {
		return nil, ErrConversationShareNotFound
	}
	messages, err := s.repo.ListMessagesForShare(ctx, conversation.ID, messageIDs)
	if err != nil {
		return nil, err
	}
	if !sharedMessagesIncludeFile(messages, normalizedFileID) {
		return nil, ErrFileNotFound
	}
	return s.uploadSvc.OpenFileContent(ctx, share.UserID, normalizedFileID)
}

func sharedMessagesIncludeFile(messages []model.Message, fileID string) bool {
	for _, message := range messages {
		attachments := []struct {
			FileID string `json:"file_id"`
		}{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(message.Attachments)), &attachments); err != nil {
			continue
		}
		for _, attachment := range attachments {
			if strings.TrimSpace(attachment.FileID) == fileID {
				return true
			}
		}
	}
	return false
}

func toConversationShareResult(share *model.ConversationShare) *ConversationShareResult {
	if share == nil {
		return &ConversationShareResult{Status: conversationShareStatusNone}
	}
	status := strings.TrimSpace(share.Status)
	if status == "" {
		status = conversationShareStatusNone
	}
	return &ConversationShareResult{
		ShareID:        share.ShareID,
		Status:         status,
		TitleSnapshot:  share.TitleSnapshot,
		ModelSnapshot:  share.ModelSnapshot,
		MessageCount:   len(decodeShareMessageIDs(share.MessageIDsJSON)),
		CreatedAt:      share.CreatedAt,
		UpdatedAt:      share.UpdatedAt,
		RevokedAt:      share.RevokedAt,
		LastAccessedAt: share.LastAccessedAt,
	}
}
