package conversation

import (
	"encoding/json"
	"strings"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/toolresult"
)

// tracePayload is the application-owned trace model. Provider-specific JSON is
// kept at the protocol boundary; trace state itself is always represented by
// explicit fields before it is serialized for persistence or events.
type tracePayload struct {
	FileMode       string                     `json:"file_mode,omitempty"`
	FileNames      []string                   `json:"file_names,omitempty"`
	FileRefs       []attachmentTraceFileRef   `json:"file_refs,omitempty"`
	FileGroups     *attachmentTraceFileGroups `json:"file_groups,omitempty"`
	FileGroupRefs  *attachmentTraceRefGroups  `json:"file_group_refs,omitempty"`
	Query          string                     `json:"query,omitempty"`
	HitChunkCount  int                        `json:"hit_chunk_count,omitempty"`
	CandidateCount int                        `json:"candidate_count,omitempty"`
	FilteredCount  int                        `json:"filtered_count,omitempty"`
	MaxScore       float32                    `json:"max_score,omitempty"`
	Fallback       string                     `json:"fallback,omitempty"`
	Citations      []traceCitation            `json:"citations,omitempty"`
	ToolCalls      []traceToolCall            `json:"tool_calls,omitempty"`
	TraceStage     *traceStage                `json:"trace_stage,omitempty"`
	Stages         []traceStage               `json:"trace_stages,omitempty"`
	Strategy       string                     `json:"strategy,omitempty"`
	FromTurn       int                        `json:"from_turn,omitempty"`
	ToTurn         int                        `json:"to_turn,omitempty"`
	SourceTokens   int64                      `json:"source_tokens,omitempty"`
	SummaryTokens  int64                      `json:"summary_tokens,omitempty"`
	Reasoning      *traceReasoning            `json:"reasoning,omitempty"`
	PromptTrace    *promptTracePayload        `json:"prompt_trace,omitempty"`
	Error          string                     `json:"error,omitempty"`
	Reason         string                     `json:"reason,omitempty"`
	Status         string                     `json:"status,omitempty"`
	ToolID         uint                       `json:"tool_id,omitempty"`
	ToolName       string                     `json:"tool_name,omitempty"`
	SkillCount     int                        `json:"skill_count,omitempty"`
	SkillIDs       []uint                     `json:"skill_ids,omitempty"`
	SkillTitles    []string                   `json:"skill_titles,omitempty"`
	SkillTriggers  []string                   `json:"skill_triggers,omitempty"`
	UpstreamDebug  json.RawMessage            `json:"upstream_debug,omitempty"`
}

type traceStage struct {
	Kind           string  `json:"kind,omitempty"`
	Status         string  `json:"status,omitempty"`
	IncludedCount  int     `json:"included_count,omitempty"`
	SkippedCount   int     `json:"skipped_count,omitempty"`
	FromTurn       int     `json:"from_turn,omitempty"`
	ToTurn         int     `json:"to_turn,omitempty"`
	SourceTokens   int64   `json:"source_tokens,omitempty"`
	SummaryTokens  int64   `json:"summary_tokens,omitempty"`
	FileCount      int     `json:"file_count,omitempty"`
	ChunkCount     int     `json:"chunk_count,omitempty"`
	CandidateCount int     `json:"candidate_count,omitempty"`
	FilteredCount  int     `json:"filtered_count,omitempty"`
	MaxScore       float32 `json:"max_score,omitempty"`
	Fallback       string  `json:"fallback,omitempty"`
	Reason         string  `json:"reason,omitempty"`
}

type traceCitation struct {
	FileName   string  `json:"file_name,omitempty"`
	FileID     string  `json:"file_id,omitempty"`
	ChunkIndex int     `json:"chunk_index,omitempty"`
	Score      float32 `json:"score,omitempty"`
	Preview    string  `json:"preview,omitempty"`
}

type traceToolCall struct {
	ToolCallID         string                   `json:"tool_call_id,omitempty"`
	Name               string                   `json:"name,omitempty"`
	Type               string                   `json:"type,omitempty"`
	Status             string                   `json:"status,omitempty"`
	LatencyMS          int64                    `json:"latency_ms,omitempty"`
	Error              string                   `json:"error,omitempty"`
	InputPreview       string                   `json:"input_preview,omitempty"`
	InputDetail        string                   `json:"input_detail,omitempty"`
	InputSize          int                      `json:"input_size,omitempty"`
	OutputPreview      string                   `json:"output_preview,omitempty"`
	OutputDetail       string                   `json:"output_detail,omitempty"`
	DetailRunID        string                   `json:"detail_run_id,omitempty"`
	OutputPresentation *toolresult.Presentation `json:"output_presentation,omitempty"`
}

type traceReasoning struct {
	Kind             string `json:"kind,omitempty"`
	EventType        string `json:"event_type,omitempty"`
	ItemID           string `json:"item_id,omitempty"`
	Status           string `json:"status,omitempty"`
	Signature        string `json:"signature,omitempty"`
	EncryptedContent string `json:"encrypted_content,omitempty"`
}

// promptTracePayload is kept separate from the domain display model so
// persisted legacy payloads can be decoded without an untyped intermediate map.
type promptTracePayload struct {
	Mode                   string                    `json:"mode,omitempty"`
	PromptFingerprint      string                    `json:"promptFingerprint,omitempty"`
	StatefulUsed           bool                      `json:"statefulUsed,omitempty"`
	StatefulDisabledReason string                    `json:"statefulDisabledReason,omitempty"`
	TotalTokenEstimate     int64                     `json:"totalTokenEstimate,omitempty"`
	SentTokenEstimate      int64                     `json:"sentTokenEstimate,omitempty"`
	FullMessageCount       int                       `json:"fullMessageCount,omitempty"`
	SentMessageCount       int                       `json:"sentMessageCount,omitempty"`
	StatefulSavedMessages  int                       `json:"statefulSavedMessages,omitempty"`
	StatefulSavedTokens    int64                     `json:"statefulSavedTokens,omitempty"`
	Blocks                 []promptTraceBlockPayload `json:"blocks,omitempty"`
}

type promptTraceBlockPayload struct {
	Kind          string                     `json:"kind,omitempty"`
	Title         string                     `json:"title,omitempty"`
	TokenEstimate int64                      `json:"tokenEstimate,omitempty"`
	Cacheable     bool                       `json:"cacheable,omitempty"`
	SourceCount   int                        `json:"sourceCount,omitempty"`
	SourceRefs    []promptTraceSourcePayload `json:"sourceRefs,omitempty"`
}

type promptTraceSourcePayload struct {
	SourceType string `json:"sourceType,omitempty"`
	SourceID   string `json:"sourceID,omitempty"`
	Title      string `json:"title,omitempty"`
	ArtifactID uint   `json:"artifactID,omitempty"`
}

func (p *tracePayload) clone() *tracePayload {
	if p == nil {
		return nil
	}
	cloned := *p
	cloned.FileNames = append([]string(nil), p.FileNames...)
	cloned.FileRefs = append([]attachmentTraceFileRef(nil), p.FileRefs...)
	if p.FileGroups != nil {
		groups := *p.FileGroups
		groups.DirectImages = append([]string(nil), p.FileGroups.DirectImages...)
		groups.Adaptive = append([]string(nil), p.FileGroups.Adaptive...)
		groups.Retrieval = append([]string(nil), p.FileGroups.Retrieval...)
		groups.FullContext = append([]string(nil), p.FileGroups.FullContext...)
		groups.Skipped = append([]string(nil), p.FileGroups.Skipped...)
		cloned.FileGroups = &groups
	}
	if p.FileGroupRefs != nil {
		groups := *p.FileGroupRefs
		groups.DirectImages = append([]attachmentTraceFileRef(nil), p.FileGroupRefs.DirectImages...)
		groups.Adaptive = append([]attachmentTraceFileRef(nil), p.FileGroupRefs.Adaptive...)
		groups.Retrieval = append([]attachmentTraceFileRef(nil), p.FileGroupRefs.Retrieval...)
		groups.FullContext = append([]attachmentTraceFileRef(nil), p.FileGroupRefs.FullContext...)
		groups.Skipped = append([]attachmentTraceFileRef(nil), p.FileGroupRefs.Skipped...)
		cloned.FileGroupRefs = &groups
	}
	cloned.Citations = append([]traceCitation(nil), p.Citations...)
	cloned.ToolCalls = make([]traceToolCall, len(p.ToolCalls))
	for index, call := range p.ToolCalls {
		cloned.ToolCalls[index] = call
		if call.OutputPresentation != nil {
			presentation := *call.OutputPresentation
			cloned.ToolCalls[index].OutputPresentation = &presentation
		}
	}
	cloned.Stages = append([]traceStage(nil), p.Stages...)
	if p.TraceStage != nil {
		stage := *p.TraceStage
		cloned.TraceStage = &stage
	}
	if p.Reasoning != nil {
		reasoning := *p.Reasoning
		cloned.Reasoning = &reasoning
	}
	if p.PromptTrace != nil {
		prompt := *p.PromptTrace
		prompt.Blocks = make([]promptTraceBlockPayload, len(p.PromptTrace.Blocks))
		for index, block := range p.PromptTrace.Blocks {
			prompt.Blocks[index] = block
			prompt.Blocks[index].SourceRefs = append([]promptTraceSourcePayload(nil), block.SourceRefs...)
		}
		cloned.PromptTrace = &prompt
	}
	if p.UpstreamDebug != nil {
		cloned.UpstreamDebug = append(json.RawMessage(nil), p.UpstreamDebug...)
	}
	return &cloned
}

func tracePayloadFromPromptTrace(trace *model.MessagePromptTrace) *promptTracePayload {
	if trace == nil {
		return nil
	}
	payload := &promptTracePayload{
		Mode: trace.Mode, PromptFingerprint: trace.PromptFingerprint,
		StatefulUsed: trace.StatefulUsed, StatefulDisabledReason: trace.StatefulDisabledReason,
		TotalTokenEstimate: trace.TotalTokenEstimate, SentTokenEstimate: trace.SentTokenEstimate,
		FullMessageCount: trace.FullMessageCount, SentMessageCount: trace.SentMessageCount,
		StatefulSavedMessages: trace.StatefulSavedMessages, StatefulSavedTokens: trace.StatefulSavedTokens,
		Blocks: make([]promptTraceBlockPayload, 0, len(trace.Blocks)),
	}
	for _, block := range trace.Blocks {
		item := promptTraceBlockPayload{Kind: block.Kind, Title: block.Title, TokenEstimate: block.TokenEstimate, Cacheable: block.Cacheable, SourceCount: block.SourceCount}
		for _, ref := range block.SourceRefs {
			item.SourceRefs = append(item.SourceRefs, promptTraceSourcePayload{SourceType: ref.SourceType, SourceID: ref.SourceID, Title: ref.Title, ArtifactID: ref.ArtifactID})
		}
		payload.Blocks = append(payload.Blocks, item)
	}
	return payload
}

func (p *promptTracePayload) toDomain() *model.MessagePromptTrace {
	if p == nil || (p.Mode == "" && len(p.Blocks) == 0) {
		return nil
	}
	trace := &model.MessagePromptTrace{Mode: p.Mode, PromptFingerprint: p.PromptFingerprint, StatefulUsed: p.StatefulUsed, StatefulDisabledReason: p.StatefulDisabledReason, TotalTokenEstimate: p.TotalTokenEstimate, SentTokenEstimate: p.SentTokenEstimate, FullMessageCount: p.FullMessageCount, SentMessageCount: p.SentMessageCount, StatefulSavedMessages: p.StatefulSavedMessages, StatefulSavedTokens: p.StatefulSavedTokens}
	for _, block := range p.Blocks {
		item := model.MessagePromptTraceBlock{Kind: block.Kind, Title: block.Title, TokenEstimate: block.TokenEstimate, Cacheable: block.Cacheable, SourceCount: block.SourceCount}
		for _, ref := range block.SourceRefs {
			item.SourceRefs = append(item.SourceRefs, model.MessagePromptTraceSourceRef{SourceType: ref.SourceType, SourceID: ref.SourceID, Title: ref.Title, ArtifactID: ref.ArtifactID})
		}
		trace.Blocks = append(trace.Blocks, item)
	}
	return trace
}

func trimTraceReasoning(value *traceReasoning) *traceReasoning {
	if value == nil {
		return nil
	}
	copy := *value
	copy.Kind = strings.TrimSpace(copy.Kind)
	copy.EventType = strings.TrimSpace(copy.EventType)
	copy.ItemID = strings.TrimSpace(copy.ItemID)
	copy.Status = strings.TrimSpace(copy.Status)
	copy.Signature = strings.TrimSpace(copy.Signature)
	copy.EncryptedContent = strings.TrimSpace(copy.EncryptedContent)
	if copy.Kind == "" && copy.EventType == "" && copy.ItemID == "" && copy.Status == "" && copy.Signature == "" && copy.EncryptedContent == "" {
		return nil
	}
	return &copy
}

func (p *tracePayload) ReasoningItemID() string {
	if p == nil || p.Reasoning == nil {
		return ""
	}
	return p.Reasoning.ItemID
}

func firstTraceStage(payload *tracePayload) *traceStage {
	if payload == nil {
		return nil
	}
	if payload.TraceStage != nil {
		stage := *payload.TraceStage
		return &stage
	}
	if len(payload.Stages) == 0 {
		return nil
	}
	stage := payload.Stages[len(payload.Stages)-1]
	return &stage
}
