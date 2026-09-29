package conversation

import (
	"encoding/json"
	"strings"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

// 旧版终结重放会在首次完成后立即持久化
// 重复的完成事件。本地类生产数据的间隔最高不到两秒；
// 保留少量余量，同时拒绝合并更晚的、可能真实的轮次。
const legacyReasoningReplayMaxGap = 3 * time.Second

type persistedReasoningEventMetadata struct {
	EventType string
	ItemID    string
}

type legacyThinkReplayCandidate struct {
	rowIndex int
	metadata persistedReasoningEventMetadata
}

// normalizeLegacyThinkReplayEvents 隐藏旧版流式终结路径
// 重放的推理快照。这些路径可能将相同的实时与终态
// 内容持久化为不同轮次，有时还缺少推理元数据。保留
// 权威身份并合并终态快照，且不修改存储。
func normalizeLegacyThinkReplayEvents(rows []model.MessageTraceEventRow) []model.MessageTraceEventRow {
	if len(rows) < 2 {
		return rows
	}
	workingRows := append([]model.MessageTraceEventRow(nil), rows...)

	groups := make(map[string][]legacyThinkReplayCandidate)
	for index, row := range workingRows {
		if !isPersistedThinkTraceEvent(row) {
			continue
		}
		content := strings.TrimSpace(row.ContentMarkdown)
		if content == "" {
			continue
		}
		metadata, _ := persistedReasoningMetadata(row.PayloadJSON)
		key := strings.TrimSpace(row.RunID) + "\x00" + content
		groups[key] = append(groups[key], legacyThinkReplayCandidate{rowIndex: index, metadata: metadata})
	}

	removed := make(map[int]struct{})
	for _, candidates := range groups {
		if len(candidates) < 2 {
			continue
		}
		for position := 0; position < len(candidates); {
			replayEnd := position
			for replayEnd+1 < len(candidates) {
				first := candidates[replayEnd]
				second := candidates[replayEnd+1]
				if !isLegacyReasoningReplayPair(workingRows, first, second) {
					break
				}
				replayEnd++
			}
			if replayEnd == position {
				position++
				continue
			}

			canonicalPosition := position
			if position > 0 && isPersistedLiveReasoningEvent(candidates[position-1].metadata.EventType) {
				canonicalPosition = position - 1
			}
			canonicalIndex := candidates[canonicalPosition].rowIndex
			finalIndex := candidates[replayEnd].rowIndex
			workingRows[canonicalIndex] = mergePersistedReasoningSnapshot(workingRows[canonicalIndex], workingRows[finalIndex])
			for duplicatePosition := canonicalPosition + 1; duplicatePosition <= replayEnd; duplicatePosition++ {
				removed[candidates[duplicatePosition].rowIndex] = struct{}{}
			}
			position = replayEnd + 1
		}
	}
	if len(removed) == 0 {
		return rows
	}

	normalized := make([]model.MessageTraceEventRow, 0, len(workingRows)-len(removed))
	for index, row := range workingRows {
		if _, duplicate := removed[index]; duplicate {
			continue
		}
		normalized = append(normalized, row)
	}
	return normalized
}

func isPersistedThinkTraceEvent(row model.MessageTraceEventRow) bool {
	return strings.EqualFold(strings.TrimSpace(row.EventType), "think") ||
		strings.EqualFold(strings.TrimSpace(row.Phase), messageTraceTypeUpstreamThink) ||
		strings.EqualFold(strings.TrimSpace(row.Stage), messageTraceStageThink)
}

func persistedReasoningMetadata(payloadJSON string) (persistedReasoningEventMetadata, bool) {
	payload := struct {
		Reasoning struct {
			EventType string `json:"event_type"`
			ItemID    string `json:"item_id"`
		} `json:"reasoning"`
	}{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(payloadJSON)), &payload); err != nil {
		return persistedReasoningEventMetadata{}, false
	}
	metadata := persistedReasoningEventMetadata{
		EventType: strings.ToLower(strings.TrimSpace(payload.Reasoning.EventType)),
		ItemID:    strings.TrimSpace(payload.Reasoning.ItemID),
	}
	return metadata, metadata.EventType != ""
}

func isLegacyReasoningReplayPair(
	rows []model.MessageTraceEventRow,
	firstCandidate legacyThinkReplayCandidate,
	secondCandidate legacyThinkReplayCandidate,
) bool {
	first := rows[firstCandidate.rowIndex]
	second := rows[secondCandidate.rowIndex]
	if strings.TrimSpace(first.ContentMarkdown) != strings.TrimSpace(second.ContentMarkdown) ||
		reasoningItemIDsConflict(firstCandidate.metadata, secondCandidate.metadata) ||
		first.CreatedAt.IsZero() || second.CreatedAt.Before(first.CreatedAt) ||
		second.CreatedAt.Sub(first.CreatedAt) > legacyReasoningReplayMaxGap {
		return false
	}

	firstEventType := firstCandidate.metadata.EventType
	secondEventType := secondCandidate.metadata.EventType
	if isPersistedLiveReasoningEvent(firstEventType) && secondEventType == "response.completed" {
		return true
	}
	if firstEventType == "response.completed" && secondEventType == "response.completed" {
		return strings.TrimSpace(first.PayloadJSON) == strings.TrimSpace(second.PayloadJSON)
	}
	if firstEventType != "" && secondEventType != "" {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(first.Status), messageTraceStatusCompleted) ||
		!strings.EqualFold(strings.TrimSpace(second.Status), messageTraceStatusCompleted) {
		return false
	}
	return !hasInterveningToolTraceEvent(rows, firstCandidate.rowIndex, secondCandidate.rowIndex)
}

func reasoningItemIDsConflict(first persistedReasoningEventMetadata, second persistedReasoningEventMetadata) bool {
	return first.ItemID != "" && second.ItemID != "" && first.ItemID != second.ItemID
}

func hasInterveningToolTraceEvent(rows []model.MessageTraceEventRow, firstIndex int, secondIndex int) bool {
	for index := firstIndex + 1; index < secondIndex; index++ {
		row := rows[index]
		if strings.EqualFold(strings.TrimSpace(row.EventType), "tool") ||
			strings.EqualFold(strings.TrimSpace(row.Phase), messageTraceTypeTools) ||
			strings.EqualFold(strings.TrimSpace(row.Stage), messageTraceStageTool) {
			return true
		}
	}
	return false
}

func isPersistedLiveReasoningEvent(eventType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(eventType))
	return normalized == "chat.completion.chunk" ||
		strings.HasSuffix(normalized, ".delta") ||
		(strings.Contains(normalized, "reasoning") && strings.HasSuffix(normalized, ".done"))
}

func mergePersistedReasoningSnapshot(
	canonical model.MessageTraceEventRow,
	final model.MessageTraceEventRow,
) model.MessageTraceEventRow {
	if strings.TrimSpace(final.Status) != "" {
		canonical.Status = final.Status
	}
	if strings.TrimSpace(final.Title) != "" {
		canonical.Title = final.Title
	}
	if strings.TrimSpace(final.Summary) != "" {
		canonical.Summary = final.Summary
	}
	canonical.ContentMarkdown = final.ContentMarkdown
	canonical.PayloadJSON = final.PayloadJSON
	if final.EndedAt != nil {
		canonical.EndedAt = final.EndedAt
	}
	if final.UpdatedAt.After(canonical.UpdatedAt) {
		canonical.UpdatedAt = final.UpdatedAt
	}
	return canonical
}
