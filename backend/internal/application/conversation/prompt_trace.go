package conversation

import (
	"encoding/json"
	"strings"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

type messagePromptTraceInput struct {
	Plan               PromptTrace
	Mode               string
	PromptFingerprint  string
	StatefulDecision   statefulResponseDecision
	SentMessages       []llm.Message
	FullMessages       []llm.Message
	PreviousResponseID string
}

// buildMessagePromptTrace 将应用层 PromptPlan 转成消息 trace 可展示的稳定结构。
func buildMessagePromptTrace(input messagePromptTraceInput) *model.MessagePromptTrace {
	shape := summarizePromptShape(input.Mode, input.SentMessages, input.FullMessages, input.PreviousResponseID)
	blocks := make([]model.MessagePromptTraceBlock, 0, len(input.Plan.Blocks))
	for _, block := range input.Plan.Blocks {
		blocks = append(blocks, model.MessagePromptTraceBlock{
			Kind:          string(block.Kind),
			Title:         strings.TrimSpace(block.Title),
			TokenEstimate: block.TokenEstimate,
			Cacheable:     block.Cacheable,
			SourceCount:   block.SourceCount,
			SourceRefs:    promptTraceSourceRefs(block.SourceRefs),
		})
	}
	disabledReason := strings.TrimSpace(input.StatefulDecision.DisabledReason)
	if strings.TrimSpace(input.PreviousResponseID) != "" {
		disabledReason = ""
	}
	if !shouldExposePromptTraceDisabledReason(disabledReason) {
		disabledReason = ""
	}
	return &model.MessagePromptTrace{
		Mode:                   shape.Mode,
		PromptFingerprint:      strings.TrimSpace(input.PromptFingerprint),
		StatefulUsed:           strings.TrimSpace(input.PreviousResponseID) != "",
		StatefulDisabledReason: disabledReason,
		TotalTokenEstimate:     input.Plan.TotalTokenEstimate,
		SentTokenEstimate:      shape.TotalTokens,
		FullMessageCount:       shape.FullMessageCount,
		SentMessageCount:       shape.MessageCount,
		StatefulSavedMessages:  shape.StatefulSavedMsgs,
		StatefulSavedTokens:    shape.StatefulSavedToken,
		Blocks:                 blocks,
	}
}

func shouldExposePromptTraceDisabledReason(reason string) bool {
	switch strings.TrimSpace(reason) {
	case "", "route_or_branch_not_eligible":
		return false
	default:
		return true
	}
}

// cloneMessagePromptTrace 复制 PromptTrace，避免后续 payload 合并修改内存快照。
func cloneMessagePromptTrace(trace *model.MessagePromptTrace) *model.MessagePromptTrace {
	if trace == nil {
		return nil
	}
	cloned := *trace
	if len(trace.Blocks) > 0 {
		cloned.Blocks = append([]model.MessagePromptTraceBlock(nil), trace.Blocks...)
		for index := range cloned.Blocks {
			if len(trace.Blocks[index].SourceRefs) > 0 {
				cloned.Blocks[index].SourceRefs = append([]model.MessagePromptTraceSourceRef(nil), trace.Blocks[index].SourceRefs...)
			}
		}
	}
	return &cloned
}

// messagePromptTracePayload 将 PromptTrace 写入 trace payload，供持久化后复原。
func messagePromptTracePayload(trace *model.MessagePromptTrace) map[string]interface{} {
	if trace == nil {
		return nil
	}
	blocks := make([]map[string]interface{}, 0, len(trace.Blocks))
	for _, block := range trace.Blocks {
		sourceRefs := make([]map[string]interface{}, 0, len(block.SourceRefs))
		for _, ref := range block.SourceRefs {
			sourceRef := map[string]interface{}{
				"sourceType": strings.TrimSpace(ref.SourceType),
				"sourceID":   strings.TrimSpace(ref.SourceID),
				"title":      strings.TrimSpace(ref.Title),
			}
			if ref.ArtifactID > 0 {
				sourceRef["artifactID"] = ref.ArtifactID
			}
			sourceRefs = append(sourceRefs, sourceRef)
		}
		blocks = append(blocks, map[string]interface{}{
			"kind":          strings.TrimSpace(block.Kind),
			"title":         strings.TrimSpace(block.Title),
			"tokenEstimate": block.TokenEstimate,
			"cacheable":     block.Cacheable,
			"sourceCount":   block.SourceCount,
			"sourceRefs":    sourceRefs,
		})
	}
	return map[string]interface{}{
		"mode":                   strings.TrimSpace(trace.Mode),
		"promptFingerprint":      strings.TrimSpace(trace.PromptFingerprint),
		"statefulUsed":           trace.StatefulUsed,
		"statefulDisabledReason": strings.TrimSpace(trace.StatefulDisabledReason),
		"totalTokenEstimate":     trace.TotalTokenEstimate,
		"sentTokenEstimate":      trace.SentTokenEstimate,
		"fullMessageCount":       trace.FullMessageCount,
		"sentMessageCount":       trace.SentMessageCount,
		"statefulSavedMessages":  trace.StatefulSavedMessages,
		"statefulSavedTokens":    trace.StatefulSavedTokens,
		"blocks":                 blocks,
	}
}

// messagePromptTraceFromPayload 从 process trace payload 中恢复结构化 PromptTrace。
func messagePromptTraceFromPayload(raw string) *model.MessagePromptTrace {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	payload := map[string]interface{}{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return nil
	}
	rawTrace, ok := payload["prompt_trace"]
	if !ok {
		rawTrace, ok = payload["promptTrace"]
	}
	traceMap, ok := rawTrace.(map[string]interface{})
	if !ok {
		return nil
	}
	trace := &model.MessagePromptTrace{
		Mode:                   promptTraceString(traceMap, "mode"),
		PromptFingerprint:      promptTraceString(traceMap, "promptFingerprint"),
		StatefulUsed:           promptTraceBool(traceMap, "statefulUsed"),
		StatefulDisabledReason: promptTraceString(traceMap, "statefulDisabledReason"),
		TotalTokenEstimate:     promptTraceInt64(traceMap, "totalTokenEstimate"),
		SentTokenEstimate:      promptTraceInt64(traceMap, "sentTokenEstimate"),
		FullMessageCount:       int(promptTraceInt64(traceMap, "fullMessageCount")),
		SentMessageCount:       int(promptTraceInt64(traceMap, "sentMessageCount")),
		StatefulSavedMessages:  int(promptTraceInt64(traceMap, "statefulSavedMessages")),
		StatefulSavedTokens:    promptTraceInt64(traceMap, "statefulSavedTokens"),
	}
	if blocks, ok := traceMap["blocks"].([]interface{}); ok {
		for _, item := range blocks {
			blockMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			trace.Blocks = append(trace.Blocks, model.MessagePromptTraceBlock{
				Kind:          promptTraceString(blockMap, "kind"),
				Title:         promptTraceString(blockMap, "title"),
				TokenEstimate: promptTraceInt64(blockMap, "tokenEstimate"),
				Cacheable:     promptTraceBool(blockMap, "cacheable"),
				SourceCount:   int(promptTraceInt64(blockMap, "sourceCount")),
				SourceRefs:    promptTraceSourceRefsFromPayload(blockMap["sourceRefs"]),
			})
		}
	}
	if trace.Mode == "" && len(trace.Blocks) == 0 {
		return nil
	}
	return trace
}

// promptTraceSourceRefs 将规划器来源引用转换为领域 trace 来源引用。
func promptTraceSourceRefs(refs []PromptSourceRef) []model.MessagePromptTraceSourceRef {
	if len(refs) == 0 {
		return nil
	}
	result := make([]model.MessagePromptTraceSourceRef, 0, len(refs))
	for _, ref := range refs {
		result = append(result, model.MessagePromptTraceSourceRef{
			SourceType: strings.TrimSpace(ref.SourceType),
			SourceID:   strings.TrimSpace(ref.SourceID),
			Title:      strings.TrimSpace(ref.Title),
			ArtifactID: ref.ArtifactID,
		})
	}
	return result
}

// promptTraceSourceRefsFromPayload 从持久化 payload 中恢复来源引用。
func promptTraceSourceRefsFromPayload(raw interface{}) []model.MessagePromptTraceSourceRef {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	refs := make([]model.MessagePromptTraceSourceRef, 0, len(items))
	for _, item := range items {
		refMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		refs = append(refs, model.MessagePromptTraceSourceRef{
			SourceType: promptTraceString(refMap, "sourceType"),
			SourceID:   promptTraceString(refMap, "sourceID"),
			Title:      promptTraceString(refMap, "title"),
			ArtifactID: uint(promptTraceInt64(refMap, "artifactID")),
		})
	}
	return refs
}

func promptTraceString(payload map[string]interface{}, key string) string {
	if value, ok := payload[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func promptTraceBool(payload map[string]interface{}, key string) bool {
	if value, ok := payload[key].(bool); ok {
		return value
	}
	return false
}

func promptTraceInt64(payload map[string]interface{}, key string) int64 {
	switch value := payload[key].(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	case json.Number:
		result, _ := value.Int64()
		return result
	default:
		return 0
	}
}
