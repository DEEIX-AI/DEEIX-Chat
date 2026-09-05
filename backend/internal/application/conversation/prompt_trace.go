package conversation

import (
	"encoding/json"
	"strings"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
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

// messagePromptTraceFromPayload 从 process trace payload 中恢复结构化 PromptTrace。
func messagePromptTraceFromPayload(raw string) *model.MessagePromptTrace {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	var envelope struct {
		PromptTrace       *promptTracePayload `json:"prompt_trace"`
		LegacyPromptTrace *promptTracePayload `json:"promptTrace"`
	}
	if err := json.Unmarshal([]byte(value), &envelope); err != nil {
		return nil
	}
	if envelope.PromptTrace != nil {
		return envelope.PromptTrace.toDomain()
	}
	return envelope.LegacyPromptTrace.toDomain()
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
