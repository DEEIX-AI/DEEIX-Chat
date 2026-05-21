package conversation

import (
	"context"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func (s *Service) hydrateMessageFeedback(ctx context.Context, userID uint, items []model.Message) error {
	if len(items) == 0 {
		return nil
	}

	messageIDs := make([]uint, 0, len(items))
	for _, item := range items {
		if item.ID == 0 {
			continue
		}
		messageIDs = append(messageIDs, item.ID)
	}
	if len(messageIDs) == 0 {
		return nil
	}

	userFeedbackMap, err := s.repo.GetUserMessageFeedbackMap(ctx, userID, messageIDs)
	if err != nil {
		return err
	}
	countsMap, err := s.repo.GetMessageFeedbackCounts(ctx, messageIDs)
	if err != nil {
		return err
	}

	for i := range items {
		items[i].MyFeedback = userFeedbackMap[items[i].ID]
		if counts := countsMap[items[i].ID]; counts != nil {
			items[i].ThumbsUpCount = counts["up"]
			items[i].ThumbsDownCount = counts["down"]
		} else {
			items[i].ThumbsUpCount = 0
			items[i].ThumbsDownCount = 0
		}
	}
	return nil
}

func (s *Service) hydrateMessageProcessTraces(ctx context.Context, items []model.Message) error {
	cfg := s.cfg.Snapshot()
	if !cfg.ProcessTraceEnabled || !cfg.ProcessTraceVisibleToUser || len(items) == 0 {
		return nil
	}

	messageIDs := make([]uint, 0, len(items))
	for _, item := range items {
		if item.Role != "assistant" || item.ID == 0 {
			continue
		}
		messageIDs = append(messageIDs, item.ID)
	}
	if len(messageIDs) == 0 {
		return nil
	}

	rows, err := s.repo.ListConversationMessageTracesByMessageIDs(ctx, messageIDs)
	if err != nil {
		return err
	}
	eventRows, err := s.repo.ListConversationMessageTraceEventsByMessageIDs(ctx, messageIDs)
	if err != nil {
		return err
	}
	byMessageID := make(map[uint][]model.MessageTrace, len(messageIDs))
	for _, row := range rows {
		byMessageID[row.MessageID] = append(byMessageID[row.MessageID], row)
	}
	eventsByMessageID := make(map[uint][]model.MessageTraceEventRow, len(messageIDs))
	for _, row := range eventRows {
		eventsByMessageID[row.MessageID] = append(eventsByMessageID[row.MessageID], row)
	}

	for i := range items {
		if items[i].Role != "assistant" {
			continue
		}
		items[i].ProcessTrace = buildMessageProcessTraceDTO(byMessageID[items[i].ID], eventsByMessageID[items[i].ID])
	}
	return nil
}

func buildMessageProcessTraceDTO(rows []model.MessageTrace, eventRows []model.MessageTraceEventRow) *model.MessageProcessTrace {
	if len(rows) == 0 && len(eventRows) == 0 {
		return nil
	}
	result := &model.MessageProcessTrace{Enabled: true}
	for _, row := range rows {
		block := &model.MessageTraceBlock{
			Title:           row.Title,
			Summary:         row.Summary,
			ContentMarkdown: row.ContentMarkdown,
			Status:          row.Status,
			Stage:           row.Stage,
			RoundID:         row.RoundID,
			ParentEventID:   row.ParentEventID,
			UpdatedAt:       row.UpdatedAt,
			PayloadJSON:     row.PayloadJSON,
		}
		switch row.TraceType {
		case messageTraceTypeProcess:
			result.Process = block
			result.PromptTrace = messagePromptTraceFromPayload(row.PayloadJSON)
		case messageTraceTypeTools:
			result.Tools = block
		case messageTraceTypeUpstreamThink:
			result.UpstreamThink = block
		}
	}
	for _, row := range eventRows {
		result.Events = append(result.Events, model.MessageTraceEvent{
			EventID:         row.EventID,
			EventType:       row.EventType,
			Phase:           row.Phase,
			Stage:           row.Stage,
			RoundID:         row.RoundID,
			ParentEventID:   row.ParentEventID,
			Title:           row.Title,
			Summary:         row.Summary,
			ContentMarkdown: row.ContentMarkdown,
			Status:          row.Status,
			Seq:             row.Seq,
			StartedAt:       row.StartedAt,
			EndedAt:         row.EndedAt,
			UpdatedAt:       row.UpdatedAt,
			PayloadJSON:     row.PayloadJSON,
		})
	}
	result.Status = aggregateTraceStatusFromBlocks(result.Process, result.Tools, result.UpstreamThink)
	if result.Status == "" && len(result.Events) > 0 {
		result.Status = aggregateTraceStatusFromEvents(result.Events)
	}
	if result.Process == nil && result.Tools == nil && result.UpstreamThink == nil && len(result.Events) == 0 {
		return nil
	}
	return result
}

func aggregateTraceStatusFromEvents(events []model.MessageTraceEvent) string {
	hasStreaming := false
	hasCompleted := false
	for _, event := range events {
		switch event.Status {
		case messageTraceStatusError:
			return messageTraceStatusError
		case messageTraceStatusStreaming:
			hasStreaming = true
		case messageTraceStatusCompleted:
			hasCompleted = true
		}
	}
	if hasStreaming {
		return messageTraceStatusStreaming
	}
	if hasCompleted {
		return messageTraceStatusCompleted
	}
	return ""
}

func aggregateTraceStatusFromBlocks(blocks ...*model.MessageTraceBlock) string {
	hasStreaming := false
	hasCompleted := false
	for _, block := range blocks {
		if block == nil {
			continue
		}
		switch block.Status {
		case messageTraceStatusError:
			return messageTraceStatusError
		case messageTraceStatusStreaming:
			hasStreaming = true
		case messageTraceStatusCompleted:
			hasCompleted = true
		}
	}
	if hasStreaming {
		return messageTraceStatusStreaming
	}
	if hasCompleted {
		return messageTraceStatusCompleted
	}
	return ""
}
