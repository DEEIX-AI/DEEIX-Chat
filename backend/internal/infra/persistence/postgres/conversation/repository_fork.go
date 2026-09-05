package conversation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/toolresult"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const forkEventCreateBatchSize = 200

type forkToolOutputKey struct {
	runID      string
	toolCallID string
}

// CreateForkedConversation 在一个事务内创建 fork 会话、重建消息父链，并复制仍有效的附件引用与历史展示轨迹。
func (r *Repo) CreateForkedConversation(ctx context.Context, input repository.CreateForkedConversationInput) error {
	if input.Conversation == nil ||
		input.SourceConversationID == 0 ||
		input.Conversation.UserID == 0 ||
		strings.TrimSpace(input.Conversation.PublicID) == "" ||
		strings.TrimSpace(input.Conversation.SessionKey) == "" ||
		len(input.Messages) == 0 ||
		len(input.Messages) > maxAncestorQueryDepth {
		return repository.ErrInvalidInput
	}

	sourceMessageIDs := make([]uint, 0, len(input.Messages))
	seenSourceMessageIDs := make(map[uint]struct{}, len(input.Messages))
	for _, item := range input.Messages {
		if item.SourceMessageID == 0 || strings.TrimSpace(item.Message.PublicID) == "" {
			return repository.ErrInvalidInput
		}
		if _, exists := seenSourceMessageIDs[item.SourceMessageID]; exists {
			return repository.ErrInvalidInput
		}
		seenSourceMessageIDs[item.SourceMessageID] = struct{}{}
		sourceMessageIDs = append(sourceMessageIDs, item.SourceMessageID)
	}

	target := *input.Conversation
	target.MessageCount = len(input.Messages)
	var createdConversation models.Conversation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sourceConversation models.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("id = ? AND user_id = ?", input.SourceConversationID, target.UserID).
			First(&sourceConversation).Error; err != nil {
			return err
		}

		lockedSourceMessages := make([]models.Message, 0, len(sourceMessageIDs))
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("conversation_id = ? AND user_id = ? AND id IN ?", input.SourceConversationID, target.UserID, sourceMessageIDs).
			Find(&lockedSourceMessages).Error; err != nil {
			return err
		}
		if len(lockedSourceMessages) != len(sourceMessageIDs) {
			return repository.ErrNotFound
		}

		createdConversation = toConversationModel(&target)
		if err := tx.Create(&createdConversation).Error; err != nil {
			return err
		}

		targetMessageIDs := make(map[uint]uint, len(input.Messages))
		for _, item := range input.Messages {
			message := item.Message
			message.ID = 0
			message.ConversationID = createdConversation.ID
			message.UserID = target.UserID
			message.ParentMessageID = nil
			if item.SourceParentMessageID != nil {
				parentID, exists := targetMessageIDs[*item.SourceParentMessageID]
				if !exists {
					return repository.ErrInvalidInput
				}
				message.ParentMessageID = &parentID
			}

			entity := toMessageModel(&message)
			if err := tx.Create(&entity).Error; err != nil {
				return err
			}
			targetMessageIDs[item.SourceMessageID] = entity.ID
		}

		if err := cloneForkedAttachments(tx, target.UserID, input.SourceConversationID, createdConversation.ID, targetMessageIDs); err != nil {
			return err
		}
		return cloneForkedDisplayTraces(tx, target.UserID, input.SourceConversationID, createdConversation.ID, targetMessageIDs)
	})
	if err != nil {
		return dberror.Translate(err)
	}

	*input.Conversation = toConversationDomain(createdConversation)
	return nil
}

func cloneForkedDisplayTraces(
	tx *gorm.DB,
	userID uint,
	sourceConversationID uint,
	targetConversationID uint,
	targetMessageIDs map[uint]uint,
) error {
	sourceMessageIDs := make([]uint, 0, len(targetMessageIDs))
	for sourceMessageID := range targetMessageIDs {
		sourceMessageIDs = append(sourceMessageIDs, sourceMessageID)
	}

	sourceEvents := make([]models.ChatRunEvent, 0)
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(
			"conversation_id = ? AND user_id = ? AND message_id IN ? AND event_scope IN ?",
			sourceConversationID,
			userID,
			sourceMessageIDs,
			[]string{chatRunEventScopeTraceBlock, chatRunEventScopeTraceEvent, chatRunEventScopeToolCall},
		).
		Order("message_id ASC, seq ASC, id ASC").
		Find(&sourceEvents).Error; err != nil {
		return err
	}
	if len(sourceEvents) == 0 {
		return nil
	}

	toolOutputs := make(map[forkToolOutputKey]string)
	for _, source := range sourceEvents {
		if source.EventScope == chatRunEventScopeToolCall && strings.TrimSpace(source.ToolCallID) != "" {
			toolOutputs[forkToolOutputKey{runID: source.RunID, toolCallID: source.ToolCallID}] = source.OutputJSON
		}
	}

	targetEvents := make([]models.ChatRunEvent, 0, len(sourceEvents))
	for _, source := range sourceEvents {
		targetMessageID, exists := targetMessageIDs[source.MessageID]
		if !exists {
			return repository.ErrInvalidInput
		}
		targetRunID := forkedEventRunID(targetMessageID, source.RunID)
		payloadJSON := source.PayloadJSON
		if source.EventScope != chatRunEventScopeToolCall {
			payloadJSON = enrichForkedToolTracePayload(source.PayloadJSON, source.RunID, targetRunID, toolOutputs)
		}
		targetEvents = append(targetEvents, models.ChatRunEvent{
			BaseModel: models.BaseModel{
				CreatedAt: source.CreatedAt,
				UpdatedAt: source.UpdatedAt,
			},
			MessageID:       targetMessageID,
			ConversationID:  targetConversationID,
			UserID:          userID,
			RunID:           targetRunID,
			EventScope:      source.EventScope,
			EventID:         source.EventID,
			EventType:       source.EventType,
			Phase:           source.Phase,
			Stage:           source.Stage,
			RoundID:         source.RoundID,
			ParentEventID:   source.ParentEventID,
			Status:          source.Status,
			Title:           source.Title,
			Summary:         source.Summary,
			ContentMarkdown: source.ContentMarkdown,
			PayloadJSON:     payloadJSON,
			Seq:             source.Seq,
			ToolCallID:      source.ToolCallID,
			ToolName:        source.ToolName,
			MCPServerID:     source.MCPServerID,
			MCPServerName:   source.MCPServerName,
			LatencyMS:       source.LatencyMS,
			InputJSON:       source.InputJSON,
			OutputJSON:      source.OutputJSON,
			ErrorJSON:       source.ErrorJSON,
			StartedAt:       source.StartedAt,
			EndedAt:         source.EndedAt,
		})
	}
	return tx.CreateInBatches(&targetEvents, forkEventCreateBatchSize).Error
}

func enrichForkedToolTracePayload(
	payloadJSON string,
	sourceRunID string,
	targetRunID string,
	toolOutputs map[forkToolOutputKey]string,
) string {
	var payload map[string]any
	if json.Unmarshal([]byte(payloadJSON), &payload) != nil {
		return payloadJSON
	}
	calls, ok := payload["tool_calls"].([]any)
	if !ok {
		return payloadJSON
	}
	changed := false
	for _, item := range calls {
		call, ok := item.(map[string]any)
		if !ok {
			continue
		}
		toolCallID := forkToolCallID(call)
		output, hasDetail := toolOutputs[forkToolOutputKey{runID: sourceRunID, toolCallID: toolCallID}]
		if hasDetail {
			if detailRunID := strings.TrimSpace(targetRunID); detailRunID != "" && call["detail_run_id"] != detailRunID {
				call["detail_run_id"] = detailRunID
				changed = true
			}
		} else if _, exists := call["detail_run_id"]; exists {
			delete(call, "detail_run_id")
			changed = true
		}
		if call["output_presentation"] != nil {
			continue
		}
		presentation := toolresult.BuildPresentation(output)
		if presentation == nil {
			continue
		}
		call["output_presentation"] = presentation
		changed = true
	}
	if !changed {
		return payloadJSON
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return payloadJSON
	}
	return string(encoded)
}

func forkToolCallID(call map[string]any) string {
	for _, key := range []string{"tool_call_id", "id", "call_id"} {
		if value, ok := call[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func forkedEventRunID(targetMessageID uint, sourceRunID string) string {
	sourceRunHash := sha256.Sum256([]byte(sourceRunID))
	return fmt.Sprintf("fork_trace_%d_%x", targetMessageID, sourceRunHash[:8])
}

func cloneForkedAttachments(
	tx *gorm.DB,
	userID uint,
	sourceConversationID uint,
	targetConversationID uint,
	targetMessageIDs map[uint]uint,
) error {
	sourceMessageIDs := make([]uint, 0, len(targetMessageIDs))
	for sourceMessageID := range targetMessageIDs {
		sourceMessageIDs = append(sourceMessageIDs, sourceMessageID)
	}

	sourceAttachments := make([]models.Attachment, 0)
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("conversation_id = ? AND user_id = ? AND message_id IN ? AND status <> ?", sourceConversationID, userID, sourceMessageIDs, "deleted").
		Order("id ASC").
		Find(&sourceAttachments).Error; err != nil {
		return err
	}
	if len(sourceAttachments) == 0 {
		return nil
	}

	fileIDs := make([]string, 0, len(sourceAttachments))
	seenFileIDs := make(map[string]struct{}, len(sourceAttachments))
	for _, item := range sourceAttachments {
		fileID := strings.TrimSpace(item.FileID)
		if fileID == "" {
			continue
		}
		if _, exists := seenFileIDs[fileID]; exists {
			continue
		}
		seenFileIDs[fileID] = struct{}{}
		fileIDs = append(fileIDs, fileID)
	}
	if len(fileIDs) == 0 {
		return nil
	}

	activeFiles := make([]models.FileObject, 0, len(fileIDs))
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND status = ? AND file_id IN ?", userID, "active", fileIDs).
		Find(&activeFiles).Error; err != nil {
		return err
	}
	filesByID := make(map[string]models.FileObject, len(activeFiles))
	for _, item := range activeFiles {
		filesByID[item.FileID] = item
	}

	now := time.Now().UTC()
	attachments := make([]models.Attachment, 0, len(sourceAttachments))
	for _, source := range sourceAttachments {
		file, exists := filesByID[strings.TrimSpace(source.FileID)]
		if !exists {
			continue
		}
		targetMessageID, exists := targetMessageIDs[source.MessageID]
		if !exists {
			return repository.ErrInvalidInput
		}
		kind := strings.TrimSpace(source.Kind)
		if kind == "" {
			kind = "file"
		}
		fileName := strings.TrimSpace(source.FileName)
		if fileName == "" {
			fileName = file.FileName
		}
		mimeType := strings.TrimSpace(source.MimeType)
		if mimeType == "" {
			mimeType = file.MimeType
		}
		fileSize := source.FileSize
		if fileSize <= 0 {
			fileSize = file.SizeBytes
		}
		attachments = append(attachments, models.Attachment{
			ConversationID: targetConversationID,
			MessageID:      targetMessageID,
			UserID:         userID,
			FileID:         file.FileID,
			Kind:           kind,
			FileName:       fileName,
			MimeType:       mimeType,
			FileSize:       fileSize,
			SHA256:         file.SHA256,
			StoragePath:    file.StoragePath,
			Status:         "active",
			MetaJSON:       source.MetaJSON,
			UploadedAt:     now,
		})
	}
	if len(attachments) == 0 {
		return nil
	}
	return tx.Create(&attachments).Error
}

var _ repository.ConversationForkRepository = (*Repo)(nil)
