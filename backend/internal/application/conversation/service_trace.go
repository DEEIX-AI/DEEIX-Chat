package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"go.uber.org/zap"
)

const (
	messageTraceTypeProcess        = "process"
	messageTraceTypeTools          = "tools"
	messageTraceTypeUpstreamThink  = "upstream_think"
	messageTraceStageProcess       = "process"
	messageTraceStageThink         = "think"
	messageTraceStageTool          = "tool"
	messageTraceStatusStreaming    = "streaming"
	messageTraceStatusCompleted    = "completed"
	messageTraceStatusError        = "error"
	messageTraceThinkKindSummary   = "summary_text"
	messageTraceThinkKindContent   = "content_text"
	messageTraceThinkKindSignature = "signature"
)

const (
	processTracePayloadStage        = "trace_stage"
	processTracePayloadStages       = "trace_stages"
	processTraceKindFileContext     = "file_context"
	processTraceKindRetrieval       = "content_retrieval"
	processTraceKindCompaction      = "context_compaction"
	processTraceStatusReady         = "ready"
	processTraceStatusCompleted     = "completed"
	processTraceStatusIncomplete    = "incomplete"
	processTraceStatusEmpty         = "empty"
	processTraceStatusLowScore      = "low_score"
	processTraceStatusSkipped       = "skipped"
	processTraceFallbackFullText    = "full_text"
	processTraceFallbackUnavailable = "unavailable"
)

const (
	toolTracePreviewMaxChars = 260
	toolTraceDetailMaxChars  = 4096
)

const (
	upstreamThinkLiveFlushInterval = 80 * time.Millisecond
	upstreamThinkLiveFlushBytes    = 1024
	upstreamThinkPersistInterval   = 2 * time.Second
	upstreamThinkLiveReplaceBytes  = 16 * 1024
)

type messageTraceDraft struct {
	traceType       string
	eventID         string
	eventType       string
	eventSeq        int
	stage           string
	roundID         string
	parentEventID   string
	status          string
	title           string
	summary         string
	contentMarkdown string
	payload         map[string]interface{}
	seq             int
	startedAt       time.Time
	endedAt         *time.Time
}

type messageTraceRecorder struct {
	service       *Service
	ctx           context.Context
	cfg           config.Config
	assistant     *model.Message
	onEvent       func(string, map[string]interface{}) error
	process       *messageTraceDraft
	tools         *messageTraceDraft
	upstreamThink *messageTraceDraft
	promptTrace   *model.MessagePromptTrace
	nextEventSeq  int
	nextRoundSeq  int
	eventCounters map[string]int
	events        []model.MessageTraceEvent

	upstreamThinkLastLiveFlush  time.Time
	upstreamThinkLastPersist    time.Time
	upstreamThinkPendingText    strings.Builder
	upstreamThinkPendingReplace string
	upstreamThinkPendingKind    string
	upstreamThinkPendingReason  map[string]interface{}
	upstreamThinkBufferedByte   int
}

func formatTraceStep(label string, detail string) string {
	label = strings.TrimSpace(label)
	detail = strings.TrimSpace(detail)
	if label == "" && detail == "" {
		return ""
	}
	if label == "" {
		return detail
	}
	if detail == "" {
		return fmt.Sprintf("**%s**", label)
	}
	return fmt.Sprintf("**%s**：%s", label, detail)
}

func joinTraceParts(parts ...string) string {
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			items = append(items, value)
		}
	}
	return strings.Join(items, "；")
}

func traceCountLabel(count int, unit string) string {
	if count <= 0 {
		return ""
	}
	return fmt.Sprintf("%d %s", count, unit)
}

func traceNameScope(names []string) string {
	cleaned := make([]string, 0, len(names))
	for _, name := range names {
		value := strings.TrimSpace(name)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	if len(cleaned) <= 3 {
		return "（" + strings.Join(cleaned, "、") + "）"
	}
	return fmt.Sprintf("（%s 等 %d 个）", strings.Join(cleaned[:2], "、"), len(cleaned))
}

func traceErrorSummary(err error) string {
	detail := traceErrorDetail(err)
	if strings.Contains(detail, "不支持图片输入") {
		return "模型不支持图片输入"
	}
	if strings.TrimSpace(detail) != "" {
		return "请求未完成"
	}
	return ""
}

func traceErrorDetail(err error) string {
	if err == nil {
		return ""
	}
	detail := messageErrorSummary(err)
	lower := strings.ToLower(detail)
	if strings.Contains(lower, "support image input") || strings.Contains(lower, "image input") {
		return "当前模型不支持图片输入，请切换到支持视觉输入的模型，或移除图片后重试。"
	}
	if strings.TrimSpace(detail) == "" {
		return ""
	}
	return detail
}

func newMessageTraceRecorder(
	service *Service,
	ctx context.Context,
	assistant *model.Message,
	onEvent func(string, map[string]interface{}) error,
) *messageTraceRecorder {
	if service == nil || assistant == nil {
		return nil
	}
	return &messageTraceRecorder{
		service:   service,
		ctx:       ctx,
		cfg:       service.cfg.Snapshot(),
		assistant: assistant,
		onEvent:   onEvent,
	}
}

func (r *messageTraceRecorder) enabled() bool {
	return r != nil && r.cfg.ProcessTraceEnabled && r.assistant != nil
}

func (r *messageTraceRecorder) visible() bool {
	return r.enabled() && r.cfg.ProcessTraceVisibleToUser
}

func (r *messageTraceRecorder) ensureDraft(traceType string) *messageTraceDraft {
	if !r.enabled() {
		return nil
	}
	switch traceType {
	case messageTraceTypeProcess:
		if r.process == nil {
			r.process = r.newTraceDraft(traceType, "process", "处理", 1, messageTraceStageProcess, "process", "")
		}
		return r.process
	case messageTraceTypeUpstreamThink:
		if !r.cfg.ProcessTraceStoreUpstreamThink {
			return nil
		}
		if r.upstreamThink == nil || r.upstreamThink.status == messageTraceStatusCompleted || r.upstreamThink.status == messageTraceStatusError {
			r.upstreamThink = r.newTraceDraft(traceType, "think", "模型思考", 3, messageTraceStageThink, r.nextTraceRoundID(), "")
		}
		return r.upstreamThink
	case messageTraceTypeTools:
		if r.tools == nil {
			r.tools = &messageTraceDraft{
				traceType: traceType,
				eventType: "tool",
				stage:     messageTraceStageTool,
				status:    messageTraceStatusStreaming,
				title:     "工具",
				seq:       2,
				startedAt: time.Now(),
				payload:   make(map[string]interface{}),
			}
		}
		return r.tools
	default:
		return nil
	}
}

func (r *messageTraceRecorder) newTraceDraft(traceType string, eventType string, title string, blockSeq int, stage string, roundID string, parentEventID string) *messageTraceDraft {
	eventID, eventSeq := r.nextTraceEventIdentity(traceType)
	return &messageTraceDraft{
		traceType:     traceType,
		eventID:       eventID,
		eventType:     eventType,
		eventSeq:      eventSeq,
		stage:         stage,
		roundID:       strings.TrimSpace(roundID),
		parentEventID: strings.TrimSpace(parentEventID),
		status:        messageTraceStatusStreaming,
		title:         title,
		seq:           blockSeq,
		startedAt:     time.Now(),
		payload:       make(map[string]interface{}),
	}
}

func (r *messageTraceRecorder) nextTraceRoundID() string {
	r.nextRoundSeq++
	return fmt.Sprintf("round_%d", r.nextRoundSeq)
}

func (r *messageTraceRecorder) nextTraceEventIdentity(traceType string) (string, int) {
	if r.eventCounters == nil {
		r.eventCounters = make(map[string]int)
	}
	r.eventCounters[traceType]++
	if r.nextEventSeq <= 0 {
		r.nextEventSeq = 1
	} else {
		r.nextEventSeq++
	}
	return fmt.Sprintf("%s_%d", traceType, r.eventCounters[traceType]), r.nextEventSeq
}

func (r *messageTraceRecorder) appendProcessSection(summary string, markdown string, payload map[string]interface{}, status string) {
	if !r.enabled() {
		return
	}
	value := strings.TrimSpace(markdown)
	if value == "" {
		return
	}
	draft := r.ensureDraft(messageTraceTypeProcess)
	if draft == nil {
		return
	}
	if draft.contentMarkdown != "" {
		draft.contentMarkdown += "\n\n"
	}
	draft.contentMarkdown += value
	if strings.TrimSpace(summary) != "" {
		draft.summary = strings.TrimSpace(summary)
	}
	if strings.TrimSpace(status) != "" {
		nextStatus := strings.TrimSpace(status)
		draft.status = nextStatus
		if nextStatus == messageTraceStatusStreaming {
			draft.endedAt = nil
		}
	}
	mergeTracePayload(draft.payload, payload)
	r.persistDraft(draft, false)
	r.emitProcessUpdate()
}

func (r *messageTraceRecorder) appendToolSection(summary string, markdown string, payload map[string]interface{}, status string) {
	if !r.enabled() {
		return
	}
	value := strings.TrimSpace(markdown)
	if value == "" {
		return
	}
	r.completeProcess()
	draft := r.ensureDraft(messageTraceTypeTools)
	if draft == nil {
		return
	}
	roundID, parentEventID := r.currentToolTraceBinding()
	draft.stage = messageTraceStageTool
	draft.roundID = roundID
	draft.parentEventID = parentEventID
	if isToolTracePayload(payload) {
		mergeToolTracePayload(draft.payload, payload)
		if rendered := renderToolTraceMarkdownFromPayload(draft.payload); rendered != "" {
			draft.contentMarkdown = rendered
		} else {
			draft.contentMarkdown = value
		}
	} else {
		if draft.contentMarkdown != "" {
			draft.contentMarkdown += "\n\n"
		}
		draft.contentMarkdown += value
		mergeTracePayload(draft.payload, payload)
	}
	if strings.TrimSpace(status) != "" {
		nextStatus := strings.TrimSpace(status)
		draft.status = nextStatus
		if nextStatus == messageTraceStatusStreaming {
			draft.endedAt = nil
		}
	}
	if aggregateSummary := summarizeToolTraceDraft(draft); aggregateSummary != "" {
		draft.summary = aggregateSummary
	} else if strings.TrimSpace(summary) != "" {
		draft.summary = strings.TrimSpace(summary)
	}
	r.persistDraft(draft, false)
	event := r.newTraceDraft(messageTraceTypeTools, "tool", "工具", draft.seq, messageTraceStageTool, roundID, parentEventID)
	event.summary = strings.TrimSpace(summary)
	if event.summary == "" {
		event.summary = draft.summary
	}
	event.contentMarkdown = value
	event.payload = cloneTracePayload(payload)
	event.status = messageTraceStatusCompleted
	now := time.Now()
	event.endedAt = &now
	r.persistTraceEvent(r.ctx, event, false)
	r.emitToolUpdate()
}

func (r *messageTraceRecorder) syncToolSection(summary string, markdown string, payload map[string]interface{}, status string) {
	if !r.enabled() {
		return
	}
	value := strings.TrimSpace(markdown)
	if value == "" {
		return
	}
	r.completeProcess()
	draft := r.ensureDraft(messageTraceTypeTools)
	if draft == nil {
		return
	}
	roundID, parentEventID := r.currentToolTraceBinding()
	draft.stage = messageTraceStageTool
	draft.roundID = roundID
	draft.parentEventID = parentEventID
	if isToolTracePayload(payload) {
		mergeToolTracePayload(draft.payload, payload)
		if rendered := renderToolTraceMarkdownFromPayload(draft.payload); rendered != "" {
			draft.contentMarkdown = rendered
		} else {
			draft.contentMarkdown = value
		}
		if aggregateSummary := summarizeToolTracePayload(draft.payload); aggregateSummary != "" {
			draft.summary = aggregateSummary
		} else if strings.TrimSpace(summary) != "" {
			draft.summary = strings.TrimSpace(summary)
		}
	} else {
		draft.contentMarkdown = value
		draft.payload = cloneTracePayload(payload)
		if strings.TrimSpace(summary) != "" {
			draft.summary = strings.TrimSpace(summary)
		} else if aggregateSummary := summarizeToolTracePayload(payload); aggregateSummary != "" {
			draft.summary = aggregateSummary
		}
	}
	if strings.TrimSpace(status) != "" {
		nextStatus := strings.TrimSpace(status)
		draft.status = nextStatus
		if nextStatus == messageTraceStatusStreaming {
			draft.endedAt = nil
		}
	}
	r.persistDraft(draft, false)
	r.emitToolUpdate()
}

func (r *messageTraceRecorder) currentToolTraceBinding() (string, string) {
	if r == nil {
		return "", ""
	}
	if r.upstreamThink != nil {
		roundID := strings.TrimSpace(r.upstreamThink.roundID)
		if roundID == "" {
			roundID = r.nextTraceRoundID()
			r.upstreamThink.roundID = roundID
		}
		return roundID, strings.TrimSpace(r.upstreamThink.eventID)
	}
	return r.nextTraceRoundID(), ""
}

func (r *messageTraceRecorder) appendUpstreamReasoning(kind string, text string, payload map[string]interface{}) {
	if !r.enabled() {
		return
	}
	draft := r.ensureDraft(messageTraceTypeUpstreamThink)
	if draft == nil {
		return
	}
	if text == "" {
		return
	}
	r.completeProcess()

	switch kind {
	case messageTraceThinkKindSummary:
		draft.contentMarkdown += text
		draft.summary = summarizeThinkText(draft.contentMarkdown)
	case messageTraceThinkKindSignature:
	default:
		draft.contentMarkdown += text
		if strings.TrimSpace(draft.summary) == "" {
			draft.summary = summarizeThinkText(draft.contentMarkdown)
		}
	}
	if draft.status != messageTraceStatusCompleted {
		draft.status = messageTraceStatusStreaming
	}
	mergeUpstreamReasoningPayload(draft, kind, payload)
	r.queueUpstreamThinkLiveUpdate(draft, kind, text, "", payload)
}

func (r *messageTraceRecorder) syncStructuredThink(content string, summary string, payload map[string]interface{}) {
	if !r.enabled() {
		return
	}
	if content == "" && summary == "" {
		return
	}
	r.completeProcess()
	draft := r.ensureDraft(messageTraceTypeUpstreamThink)
	if draft == nil {
		return
	}
	previousContent := draft.contentMarkdown
	displayContent := strings.TrimSpace(content)
	if displayContent == "" {
		displayContent = strings.TrimSpace(summary)
	}
	if displayContent != "" {
		draft.contentMarkdown = displayContent
	}
	if strings.TrimSpace(summary) != "" {
		draft.summary = summarizeThinkText(summary)
	} else if strings.TrimSpace(draft.summary) == "" {
		draft.summary = summarizeThinkText(draft.contentMarkdown)
	}
	if strings.TrimSpace(draft.status) == "" {
		draft.status = messageTraceStatusStreaming
	}
	mergeUpstreamReasoningPayload(draft, messageTraceThinkKindContent, payload)
	deltaText, replaceText := diffUpstreamThinkContent(previousContent, draft.contentMarkdown)
	r.queueUpstreamThinkLiveUpdate(draft, messageTraceThinkKindContent, deltaText, replaceText, payload)
}

// recordPromptTrace 把 PromptPlan 摘要合并进处理轨迹，供前端结构化展示。
func (r *messageTraceRecorder) recordPromptTrace(trace *model.MessagePromptTrace) {
	if !r.enabled() || trace == nil {
		return
	}
	draft := r.ensureDraft(messageTraceTypeProcess)
	if draft == nil {
		return
	}
	r.promptTrace = cloneMessagePromptTrace(trace)
	if draft.payload == nil {
		draft.payload = make(map[string]interface{})
	}
	draft.payload["prompt_trace"] = messagePromptTracePayload(trace)
	if strings.TrimSpace(draft.summary) == "" {
		draft.summary = buildPromptTraceSummary(trace)
	}
	draft.status = messageTraceStatusStreaming
	draft.endedAt = nil
	r.persistDraft(draft, false)
	r.emitProcessUpdate()
}

func (r *messageTraceRecorder) completeDraft(draft *messageTraceDraft) bool {
	if !r.enabled() || draft == nil || draft.status == messageTraceStatusCompleted || draft.status == messageTraceStatusError {
		return false
	}
	now := time.Now()
	draft.status = messageTraceStatusCompleted
	draft.endedAt = &now
	if draft.traceType != messageTraceTypeTools {
		r.upsertSnapshotEvent(draft, tracePayloadJSON(draft.payload))
	}
	if r.service != nil && r.service.repo != nil {
		go r.persistDraftBackground(cloneTraceDraft(draft))
	}
	return true
}

func (r *messageTraceRecorder) completeProcess() {
	if r.completeDraft(r.process) {
		r.emitProcessUpdate()
	}
}

func (r *messageTraceRecorder) completeTools() {
	if r.completeDraft(r.tools) {
		r.emitToolUpdate()
	}
}

func (r *messageTraceRecorder) completeUpstreamThink() {
	if r.completeDraft(r.upstreamThink) {
		r.flushUpstreamThinkLiveUpdate(r.upstreamThink, true, false)
	}
}

func (r *messageTraceRecorder) complete() {
	r.completeProcess()
	r.completeTools()
	r.completeUpstreamThink()
}

func (r *messageTraceRecorder) fail(err error) {
	if !r.enabled() {
		return
	}
	now := time.Now()
	summary := traceErrorSummary(err)
	detail := traceErrorDetail(err)
	process := r.ensureDraft(messageTraceTypeProcess)
	if process != nil {
		process.status = messageTraceStatusError
		if summary != "" {
			process.summary = summary
		}
		payload := map[string]interface{}{}
		if detail != "" {
			payload["error"] = detail
		}
		if debug := messageErrorDebug(err); debug != nil {
			payload["upstream_debug"] = debug
		}
		mergeTracePayload(process.payload, payload)
		process.endedAt = &now
		r.persistDraft(process, true)
	}
	if r.upstreamThink != nil {
		r.upstreamThink.status = messageTraceStatusError
		r.upstreamThink.endedAt = &now
		r.persistDraft(r.upstreamThink, true)
	}
	if r.tools != nil {
		r.tools.status = messageTraceStatusError
		r.tools.endedAt = &now
		r.persistDraft(r.tools, true)
	}
}

func (r *messageTraceRecorder) attachToMessage(message *model.Message) {
	if message == nil || !r.visible() {
		return
	}
	message.ProcessTrace = r.snapshot()
}

func (r *messageTraceRecorder) snapshot() *model.MessageProcessTrace {
	if !r.visible() {
		return nil
	}
	process := traceDraftToBlock(r.process)
	tools := traceDraftToBlock(r.tools)
	upstreamThink := traceDraftToBlock(r.upstreamThink)
	if process == nil && tools == nil && upstreamThink == nil && len(r.events) == 0 {
		return nil
	}
	return &model.MessageProcessTrace{
		Enabled:       true,
		Status:        aggregateTraceStatus(r.process, r.tools, r.upstreamThink),
		Process:       process,
		Tools:         tools,
		UpstreamThink: upstreamThink,
		PromptTrace:   cloneMessagePromptTrace(r.promptTrace),
		Events:        append([]model.MessageTraceEvent(nil), r.events...),
	}
}

func (r *messageTraceRecorder) persistDraft(draft *messageTraceDraft, force bool) {
	r.persistDraftCtx(r.ctx, draft, force)
}

func cloneTraceDraft(draft *messageTraceDraft) *messageTraceDraft {
	if draft == nil {
		return nil
	}
	cloned := *draft
	if draft.payload != nil {
		cloned.payload = make(map[string]interface{}, len(draft.payload))
		for key, value := range draft.payload {
			cloned.payload[key] = value
		}
	}
	return &cloned
}

func cloneTracePayload(payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		return make(map[string]interface{})
	}
	cloned := make(map[string]interface{}, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}

// persistDraftBackground 使用独立的 background context 持久化 trace，
// 专供 complete() 的异步 goroutine 调用，避免请求 context 取消后写入失败。
func (r *messageTraceRecorder) persistDraftBackground(draft *messageTraceDraft) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !r.enabled() || draft == nil {
		return
	}
	payloadJSON := tracePayloadJSON(draft.payload)
	r.persistMessageTraceRow(ctx, draft, payloadJSON)
	if draft.traceType != messageTraceTypeTools {
		r.persistTraceEventRow(ctx, draft, payloadJSON)
	}
}

func (r *messageTraceRecorder) persistDraftCtx(ctx context.Context, draft *messageTraceDraft, force bool) {
	if !r.enabled() || draft == nil {
		return
	}
	payloadJSON := tracePayloadJSON(draft.payload)
	if draft.traceType != messageTraceTypeTools {
		r.upsertSnapshotEvent(draft, payloadJSON)
	}
	if !force && !r.cfg.ProcessTracePersistInflight {
		return
	}
	r.persistMessageTraceRow(ctx, draft, payloadJSON)
	if draft.traceType != messageTraceTypeTools {
		r.persistTraceEventRow(ctx, draft, payloadJSON)
	}
}

type upstreamThinkLiveUpdate struct {
	kind            string
	delta           string
	contentMarkdown string
	reasoning       map[string]interface{}
}

func (r *messageTraceRecorder) queueUpstreamThinkLiveUpdate(draft *messageTraceDraft, kind string, deltaText string, replaceText string, payload map[string]interface{}) {
	if !r.enabled() || draft == nil {
		return
	}
	if deltaText != "" {
		r.upstreamThinkBufferedByte += len(deltaText)
		if len(deltaText) > upstreamThinkLiveReplaceBytes {
			deltaText = ""
		}
	}
	if deltaText != "" {
		_, _ = r.upstreamThinkPendingText.WriteString(deltaText)
	}
	if replaceText != "" {
		r.upstreamThinkBufferedByte += len(replaceText)
		if len(replaceText) <= upstreamThinkLiveReplaceBytes {
			r.upstreamThinkPendingReplace = replaceText
		}
	}
	if strings.TrimSpace(kind) != "" {
		r.upstreamThinkPendingKind = strings.TrimSpace(kind)
	}
	if reasoning := liveUpstreamReasoningPayload(kind, payload); len(reasoning) > 0 {
		r.upstreamThinkPendingReason = reasoning
	}
	if !r.shouldFlushUpstreamThinkLiveUpdate() {
		return
	}
	r.flushUpstreamThinkLiveUpdate(draft, false, true)
}

func (r *messageTraceRecorder) shouldFlushUpstreamThinkLiveUpdate() bool {
	if r == nil {
		return false
	}
	if r.upstreamThinkLastLiveFlush.IsZero() {
		return true
	}
	if r.upstreamThinkBufferedByte >= upstreamThinkLiveFlushBytes {
		return true
	}
	return time.Since(r.upstreamThinkLastLiveFlush) >= upstreamThinkLiveFlushInterval
}

func (r *messageTraceRecorder) shouldPersistUpstreamThinkSnapshot() bool {
	if r == nil {
		return false
	}
	if !r.cfg.ProcessTracePersistInflight {
		return false
	}
	if r.upstreamThinkLastPersist.IsZero() {
		return true
	}
	return time.Since(r.upstreamThinkLastPersist) >= upstreamThinkPersistInterval
}

func (r *messageTraceRecorder) flushUpstreamThinkLiveUpdate(draft *messageTraceDraft, force bool, persistSnapshot bool) {
	if !r.enabled() || draft == nil {
		return
	}
	if persistSnapshot && r.shouldPersistUpstreamThinkSnapshot() {
		r.persistDraft(draft, false)
		r.upstreamThinkLastPersist = time.Now()
	}
	update := upstreamThinkLiveUpdate{
		kind:            r.upstreamThinkPendingKind,
		delta:           r.upstreamThinkPendingText.String(),
		contentMarkdown: r.upstreamThinkPendingReplace,
		reasoning:       r.upstreamThinkPendingReason,
	}
	if !force && update.delta == "" && update.contentMarkdown == "" && len(update.reasoning) == 0 {
		return
	}
	r.emitUpstreamThinkDelta(update)
	r.resetUpstreamThinkLiveBuffer()
}

func (r *messageTraceRecorder) resetUpstreamThinkLiveBuffer() {
	if r == nil {
		return
	}
	r.upstreamThinkLastLiveFlush = time.Now()
	r.upstreamThinkPendingText.Reset()
	r.upstreamThinkPendingReplace = ""
	r.upstreamThinkPendingKind = ""
	r.upstreamThinkPendingReason = nil
	r.upstreamThinkBufferedByte = 0
}

func (r *messageTraceRecorder) persistMessageTraceRow(ctx context.Context, draft *messageTraceDraft, payloadJSON string) {
	item := &model.MessageTrace{
		MessageID:       r.assistant.ID,
		ConversationID:  r.assistant.ConversationID,
		UserID:          r.assistant.UserID,
		RunID:           r.assistant.RunID,
		TraceType:       draft.traceType,
		Status:          draft.status,
		Stage:           draft.stage,
		RoundID:         draft.roundID,
		ParentEventID:   draft.parentEventID,
		Title:           draft.title,
		Summary:         truncateError(strings.TrimSpace(draft.summary), 255),
		ContentMarkdown: draft.contentMarkdown,
		PayloadJSON:     payloadJSON,
		Seq:             draft.seq,
		StartedAt:       draft.startedAt,
		EndedAt:         draft.endedAt,
	}
	if err := r.service.repo.UpsertConversationMessageTrace(ctx, item); err != nil && r.service.logger != nil {
		r.service.logger.Warn("upsert_conversation_message_trace_failed",
			zap.Uint("assistant_message_id", r.assistant.ID),
			zap.String("trace_type", draft.traceType),
			zap.Error(err),
		)
	}
}

func (r *messageTraceRecorder) persistTraceEvent(ctx context.Context, draft *messageTraceDraft, force bool) {
	if !r.enabled() || draft == nil {
		return
	}
	payloadJSON := tracePayloadJSON(draft.payload)
	r.upsertSnapshotEvent(draft, payloadJSON)
	if !force && !r.cfg.ProcessTracePersistInflight {
		return
	}
	r.persistTraceEventRow(ctx, draft, payloadJSON)
}

func tracePayloadJSON(payload map[string]interface{}) string {
	if len(payload) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func (r *messageTraceRecorder) persistTraceEventRow(ctx context.Context, draft *messageTraceDraft, payloadJSON string) {
	item := &model.MessageTraceEventRow{
		MessageID:       r.assistant.ID,
		ConversationID:  r.assistant.ConversationID,
		UserID:          r.assistant.UserID,
		RunID:           r.assistant.RunID,
		EventID:         draft.eventID,
		EventType:       draft.eventType,
		Phase:           draft.traceType,
		Stage:           draft.stage,
		RoundID:         draft.roundID,
		ParentEventID:   draft.parentEventID,
		Status:          draft.status,
		Title:           draft.title,
		Summary:         truncateError(strings.TrimSpace(draft.summary), 255),
		ContentMarkdown: draft.contentMarkdown,
		PayloadJSON:     payloadJSON,
		Seq:             draft.eventSeq,
		StartedAt:       draft.startedAt,
		EndedAt:         draft.endedAt,
	}
	if err := r.service.repo.UpsertConversationMessageTraceEvent(ctx, item); err != nil && r.service.logger != nil {
		r.service.logger.Warn("upsert_conversation_message_trace_event_failed",
			zap.Uint("assistant_message_id", r.assistant.ID),
			zap.String("event_id", draft.eventID),
			zap.Error(err),
		)
	}
}

func (r *messageTraceRecorder) upsertSnapshotEvent(draft *messageTraceDraft, payloadJSON string) {
	event := model.MessageTraceEvent{
		EventID:         draft.eventID,
		EventType:       draft.eventType,
		Phase:           draft.traceType,
		Stage:           draft.stage,
		RoundID:         draft.roundID,
		ParentEventID:   draft.parentEventID,
		Title:           draft.title,
		Summary:         truncateError(strings.TrimSpace(draft.summary), 255),
		ContentMarkdown: draft.contentMarkdown,
		Status:          draft.status,
		Seq:             draft.eventSeq,
		StartedAt:       draft.startedAt,
		EndedAt:         draft.endedAt,
		UpdatedAt:       time.Now(),
		PayloadJSON:     payloadJSON,
	}
	for idx, item := range r.events {
		if item.EventID == event.EventID {
			r.events[idx] = event
			return
		}
	}
	r.events = append(r.events, event)
}

func (r *messageTraceRecorder) emitProcessUpdate() {
	if !r.visible() || r.process == nil {
		return
	}
	emitEvent(r.onEvent, "process_update", map[string]interface{}{
		"status": r.process.status,
		"block":  traceDraftToBlock(r.process),
		"trace":  r.snapshot(),
	})
}

func (r *messageTraceRecorder) emitToolUpdate() {
	if !r.visible() || r.tools == nil {
		return
	}
	emitEvent(r.onEvent, "process_update", map[string]interface{}{
		"status": r.tools.status,
		"block":  traceDraftToBlock(r.tools),
		"trace":  r.snapshot(),
	})
}

func (r *messageTraceRecorder) emitUpstreamThinkDelta(update upstreamThinkLiveUpdate) {
	if !r.visible() || r.upstreamThink == nil {
		return
	}
	payload := map[string]interface{}{
		"status":  r.upstreamThink.status,
		"title":   r.upstreamThink.title,
		"summary": r.upstreamThink.summary,
		"stage":   r.upstreamThink.stage,
		"roundID": r.upstreamThink.roundID,
		"eventID": r.upstreamThink.eventID,
	}
	if update.kind != "" {
		payload["kind"] = update.kind
	}
	if update.delta != "" {
		payload["delta"] = update.delta
	}
	if update.contentMarkdown != "" {
		payload["contentMarkdown"] = update.contentMarkdown
	}
	if len(update.reasoning) > 0 {
		payload["reasoning"] = update.reasoning
	}
	emitEvent(r.onEvent, "upstream_think_delta", payload)
}

func traceDraftToBlock(draft *messageTraceDraft) *model.MessageTraceBlock {
	if draft == nil {
		return nil
	}
	if strings.TrimSpace(draft.contentMarkdown) == "" && strings.TrimSpace(draft.summary) == "" {
		return nil
	}
	updatedAt := draft.startedAt
	if draft.endedAt != nil {
		updatedAt = *draft.endedAt
	}
	var payloadJSON string
	if len(draft.payload) > 0 {
		if raw, err := json.Marshal(draft.payload); err == nil {
			payloadJSON = string(raw)
		}
	}
	return &model.MessageTraceBlock{
		Title:           draft.title,
		Summary:         draft.summary,
		ContentMarkdown: draft.contentMarkdown,
		Status:          draft.status,
		Stage:           draft.stage,
		RoundID:         draft.roundID,
		ParentEventID:   draft.parentEventID,
		UpdatedAt:       updatedAt,
		PayloadJSON:     payloadJSON,
	}
}

func aggregateTraceStatus(drafts ...*messageTraceDraft) string {
	hasStreaming := false
	hasCompleted := false
	for _, draft := range drafts {
		if draft == nil {
			continue
		}
		switch draft.status {
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

func mergeTracePayload(dst map[string]interface{}, src map[string]interface{}) {
	if dst == nil || len(src) == 0 {
		return
	}
	for key, value := range src {
		if key == processTracePayloadStage {
			appendProcessTraceStagePayload(dst, value)
			continue
		}
		if key == processTracePayloadStages {
			appendProcessTraceStagePayloads(dst, value)
			continue
		}
		if key == "tool_calls" {
			if existing, ok := dst[key].([]map[string]interface{}); ok {
				if incoming, ok := value.([]map[string]interface{}); ok {
					dst[key] = append(existing, incoming...)
					continue
				}
			}
		}
		dst[key] = value
	}
}

func appendProcessTraceStagePayload(dst map[string]interface{}, value interface{}) {
	stage, ok := value.(map[string]interface{})
	if !ok || len(stage) == 0 {
		return
	}
	existing := normalizeProcessTraceStagePayloads(dst[processTracePayloadStages])
	dst[processTracePayloadStages] = append(existing, stage)
}

func appendProcessTraceStagePayloads(dst map[string]interface{}, value interface{}) {
	stages := normalizeProcessTraceStagePayloads(value)
	if len(stages) == 0 {
		return
	}
	existing := normalizeProcessTraceStagePayloads(dst[processTracePayloadStages])
	dst[processTracePayloadStages] = append(existing, stages...)
}

func normalizeProcessTraceStagePayloads(value interface{}) []map[string]interface{} {
	switch items := value.(type) {
	case []map[string]interface{}:
		return append([]map[string]interface{}{}, items...)
	case []interface{}:
		result := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			if stage, ok := item.(map[string]interface{}); ok && len(stage) > 0 {
				result = append(result, stage)
			}
		}
		return result
	default:
		return nil
	}
}

func isToolTracePayload(payload map[string]interface{}) bool {
	if payload == nil {
		return false
	}
	return len(normalizeTraceToolCalls(payload["tool_calls"])) > 0
}

func mergeToolTracePayload(dst map[string]interface{}, src map[string]interface{}) {
	if dst == nil || len(src) == 0 {
		return
	}
	for key, value := range src {
		if key != "tool_calls" {
			dst[key] = value
			continue
		}
		existing := normalizeTraceToolCalls(dst[key])
		incoming := normalizeTraceToolCalls(value)
		for _, call := range incoming {
			merged := false
			for idx, current := range existing {
				if !shouldMergeTraceToolCall(current, call) {
					continue
				}
				existing[idx] = mergeTraceToolCall(current, call)
				merged = true
				break
			}
			if !merged {
				existing = append(existing, cloneTraceToolCall(call))
			}
		}
		dst[key] = existing
	}
}

func shouldMergeTraceToolCall(existing map[string]interface{}, incoming map[string]interface{}) bool {
	existingID := traceToolCallID(existing)
	incomingID := traceToolCallID(incoming)
	if existingID != "" && incomingID != "" {
		return existingID == incomingID
	}
	if !sameTraceToolKind(existing, incoming) {
		return false
	}
	existingInput := traceToolInputKey(existing)
	incomingInput := traceToolInputKey(incoming)
	if existingInput == "" || incomingInput == "" {
		return true
	}
	return existingInput == incomingInput
}

func mergeTraceToolCall(existing map[string]interface{}, incoming map[string]interface{}) map[string]interface{} {
	merged := cloneTraceToolCall(existing)
	for key, value := range incoming {
		if key == "status" {
			merged[key] = mergeTraceToolStatus(getTraceString(merged[key]), getTraceString(value))
			continue
		}
		if traceValueIsEmpty(value) {
			continue
		}
		merged[key] = value
	}
	if getTraceString(merged["status"]) == "" {
		merged["status"] = getTraceString(incoming["status"])
	}
	return merged
}

func cloneTraceToolCall(item map[string]interface{}) map[string]interface{} {
	cloned := make(map[string]interface{}, len(item))
	for key, value := range item {
		cloned[key] = value
	}
	return cloned
}

func traceToolCallID(item map[string]interface{}) string {
	return firstTraceString(item, "tool_call_id", "id", "call_id")
}

func traceToolInputKey(item map[string]interface{}) string {
	return firstTraceString(item, "input_preview", "input")
}

func sameTraceToolKind(left map[string]interface{}, right map[string]interface{}) bool {
	leftName := strings.TrimSpace(getTraceString(left["name"]))
	rightName := strings.TrimSpace(getTraceString(right["name"]))
	leftType := strings.TrimSpace(getTraceString(left["type"]))
	rightType := strings.TrimSpace(getTraceString(right["type"]))
	if leftName != "" && rightName != "" {
		return leftName == rightName
	}
	if leftType != "" && rightType != "" {
		return leftType == rightType
	}
	return false
}

func traceValueIsEmpty(value interface{}) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []interface{}:
		return len(typed) == 0
	case []map[string]interface{}:
		return len(typed) == 0
	case map[string]interface{}:
		return len(typed) == 0
	default:
		return false
	}
}

func mergeTraceToolStatus(existing string, incoming string) string {
	current := strings.TrimSpace(existing)
	next := strings.TrimSpace(incoming)
	if next == "" {
		return current
	}
	if current == "" || traceToolStatusRank(next) >= traceToolStatusRank(current) {
		return next
	}
	return current
}

func traceToolStatusRank(status string) int {
	switch strings.TrimSpace(status) {
	case "error", "failed":
		return 4
	case "success", "completed", "reused":
		return 3
	case "streaming", "requested", "in_progress", "queued", "searching":
		return 2
	default:
		return 1
	}
}

func renderToolTraceMarkdownFromPayload(payload map[string]interface{}) string {
	summary, markdown, _ := buildToolTrace(toolTraceRowsFromPayload(payload))
	_ = summary
	return markdown
}

func toolTraceRowsFromPayload(payload map[string]interface{}) []model.ToolCall {
	items := normalizeTraceToolCalls(payload["tool_calls"])
	rows := make([]model.ToolCall, 0, len(items))
	for _, item := range items {
		rows = append(rows, model.ToolCall{
			ToolCallID: firstTraceString(item, "tool_call_id", "id", "call_id"),
			ToolType:   strings.TrimSpace(getTraceString(item["type"])),
			ToolName:   strings.TrimSpace(getTraceString(item["name"])),
			Status:     strings.TrimSpace(getTraceString(item["status"])),
			LatencyMS:  traceInt64(item["latency_ms"]),
			InputJSON:  firstTraceString(item, "input_preview", "input"),
			OutputJSON: firstTraceString(item, "output_preview", "output_text", "output"),
			ErrorJSON:  strings.TrimSpace(getTraceString(item["error"])),
		})
	}
	return rows
}

func firstTraceString(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(getTraceString(item[key])); value != "" {
			return value
		}
	}
	return ""
}

func traceInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case json.Number:
		result, _ := typed.Int64()
		return result
	default:
		return 0
	}
}

func summarizeToolTracePayload(payload map[string]interface{}) string {
	if payload == nil {
		return ""
	}
	items := normalizeTraceToolCalls(payload["tool_calls"])
	if len(items) == 0 {
		return ""
	}
	errorCount := 0
	for _, item := range items {
		switch strings.TrimSpace(getTraceString(item["status"])) {
		case "error", "failed":
			errorCount++
		}
	}
	return formatToolTraceSummary(len(items), errorCount)
}

func summarizeToolTraceDraft(draft *messageTraceDraft) string {
	if draft == nil {
		return ""
	}
	contentTotal, contentErrors := countToolTraceMarkdownRows(draft.contentMarkdown)
	payloadTotal := len(normalizeTraceToolCalls(draft.payload["tool_calls"]))
	if contentTotal > payloadTotal {
		return formatToolTraceSummary(contentTotal, contentErrors)
	}
	return summarizeToolTracePayload(draft.payload)
}

func formatToolTraceSummary(total int, errorCount int) string {
	if total <= 0 {
		return ""
	}
	if errorCount > 0 {
		return fmt.Sprintf("完成 %d 次工具调用，%d 次失败", total, errorCount)
	}
	return fmt.Sprintf("%d 次工具调用已完成", total)
}

func countToolTraceMarkdownRows(markdown string) (int, int) {
	total := 0
	errorCount := 0
	for _, line := range strings.Split(markdown, "\n") {
		value := strings.TrimSpace(line)
		if !strings.HasPrefix(value, "**") {
			continue
		}
		total++
		if strings.Contains(value, "执行失败") {
			errorCount++
		}
	}
	return total, errorCount
}

func normalizeTraceToolCalls(value interface{}) []map[string]interface{} {
	switch typed := value.(type) {
	case []map[string]interface{}:
		return typed
	case []interface{}:
		items := make([]map[string]interface{}, 0, len(typed))
		for _, item := range typed {
			if payload, ok := item.(map[string]interface{}); ok {
				items = append(items, payload)
			}
		}
		return items
	default:
		return nil
	}
}

func getTraceString(value interface{}) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func diffUpstreamThinkContent(previous string, next string) (string, string) {
	if next == "" || next == previous {
		return "", ""
	}
	if previous != "" && strings.HasPrefix(next, previous) {
		return next[len(previous):], ""
	}
	if previous == "" {
		return next, ""
	}
	return "", next
}

func liveUpstreamReasoningPayload(kind string, payload map[string]interface{}) map[string]interface{} {
	reasoning := map[string]interface{}{}
	if strings.TrimSpace(kind) != "" {
		reasoning["kind"] = strings.TrimSpace(kind)
	}
	for _, key := range []string{"event_type", "item_id", "status"} {
		if value, ok := payload[key]; ok {
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				reasoning[key] = strings.TrimSpace(text)
			}
		}
	}
	return reasoning
}

func mergeUpstreamReasoningPayload(draft *messageTraceDraft, kind string, payload map[string]interface{}) {
	if draft == nil {
		return
	}
	reasoning := map[string]interface{}{}
	if existing, ok := draft.payload["reasoning"].(map[string]interface{}); ok {
		for key, value := range existing {
			reasoning[key] = value
		}
	}
	reasoning["kind"] = kind
	reasoning["summary_text"] = draft.summary
	reasoning["content_text"] = draft.contentMarkdown
	for _, key := range []string{"event_type", "item_id", "status", "signature", "encrypted_content"} {
		if value, ok := payload[key]; ok {
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				reasoning[key] = strings.TrimSpace(text)
			}
		}
	}
	draft.payload["reasoning"] = reasoning
}

func summarizeThinkText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return compactSnippet(trimmed, 80)
}

type attachmentTraceFileRef struct {
	FileID      string `json:"file_id"`
	FileName    string `json:"file_name"`
	Kind        string `json:"kind"`
	MimeType    string `json:"mime_type"`
	ContextMode string `json:"context_mode"`
}

type attachmentTraceFileGroups struct {
	DirectImages []string `json:"direct_images"`
	Adaptive     []string `json:"adaptive"`
	Retrieval    []string `json:"retrieval"`
	FullContext  []string `json:"full_context"`
	Skipped      []string `json:"skipped"`
}

type attachmentTraceRefGroups struct {
	DirectImages []attachmentTraceFileRef `json:"direct_images"`
	Adaptive     []attachmentTraceFileRef `json:"adaptive"`
	Retrieval    []attachmentTraceFileRef `json:"retrieval"`
	FullContext  []attachmentTraceFileRef `json:"full_context"`
	Skipped      []attachmentTraceFileRef `json:"skipped"`
}

type attachmentTracePayload struct {
	FileMode      string                    `json:"file_mode"`
	FileNames     []string                  `json:"file_names"`
	FileRefs      []attachmentTraceFileRef  `json:"file_refs"`
	FileGroups    attachmentTraceFileGroups `json:"file_groups"`
	FileGroupRefs attachmentTraceRefGroups  `json:"file_group_refs"`
}

func buildAttachmentProcessTrace(
	fileMode string,
	attachments []AttachmentInput,
) (string, string, map[string]interface{}) {
	if len(attachments) == 0 {
		return "", "", nil
	}

	payload := attachmentTracePayload{
		FileMode:  strings.TrimSpace(fileMode),
		FileNames: make([]string, 0, len(attachments)),
		FileRefs:  make([]attachmentTraceFileRef, 0, len(attachments)),
	}
	for _, item := range attachments {
		name := strings.TrimSpace(item.FileName)
		if name == "" {
			name = strings.TrimSpace(item.FileID)
		}
		payload.FileNames = append(payload.FileNames, name)
		ref := newAttachmentTraceFileRef(item, name)
		payload.FileRefs = append(payload.FileRefs, ref)
		if item.ContextMode == fileContextModeDirectImage {
			payload.FileGroups.DirectImages = append(payload.FileGroups.DirectImages, name)
			payload.FileGroupRefs.DirectImages = append(payload.FileGroupRefs.DirectImages, ref)
			continue
		}
		switch item.ContextMode {
		case fileContextModeRAG:
			payload.FileGroups.Retrieval = append(payload.FileGroups.Retrieval, name)
			payload.FileGroupRefs.Retrieval = append(payload.FileGroupRefs.Retrieval, ref)
		case fileContextModeRAGFallback:
			payload.FileGroups.FullContext = append(payload.FileGroups.FullContext, name)
			payload.FileGroupRefs.FullContext = append(payload.FileGroupRefs.FullContext, ref)
		case fileContextModeSkipped:
			payload.FileGroups.Skipped = append(payload.FileGroups.Skipped, name)
			payload.FileGroupRefs.Skipped = append(payload.FileGroupRefs.Skipped, ref)
		case fileContextModeFull:
			if payload.FileMode == "auto" {
				payload.FileGroups.Adaptive = append(payload.FileGroups.Adaptive, name)
				payload.FileGroupRefs.Adaptive = append(payload.FileGroupRefs.Adaptive, ref)
			} else {
				payload.FileGroups.FullContext = append(payload.FileGroups.FullContext, name)
				payload.FileGroupRefs.FullContext = append(payload.FileGroupRefs.FullContext, ref)
			}
		default:
			payload.FileGroups.FullContext = append(payload.FileGroups.FullContext, name)
			payload.FileGroupRefs.FullContext = append(payload.FileGroupRefs.FullContext, ref)
		}
	}
	includedCount := len(attachments) - len(payload.FileGroups.Skipped)
	skippedCount := len(payload.FileGroups.Skipped)
	summary := formatAttachmentProcessCounts(includedCount, skippedCount, "已纳入")
	detail := fmt.Sprintf("文件已就绪，%s。", formatAttachmentProcessCounts(includedCount, skippedCount, "纳入"))
	return summary, formatTraceStep("文件上下文", detail), attachmentTracePayloadMap(payload)
}

func formatAttachmentProcessCounts(includedCount int, skippedCount int, includedVerb string) string {
	parts := make([]string, 0, 2)
	if includedCount > 0 || skippedCount == 0 {
		parts = append(parts, fmt.Sprintf("%s %d 个文件", includedVerb, includedCount))
	}
	if skippedCount > 0 {
		parts = append(parts, fmt.Sprintf("未纳入 %d 个文件", skippedCount))
	}
	return strings.Join(parts, "，")
}

func newAttachmentTraceFileRef(item AttachmentInput, fallbackName string) attachmentTraceFileRef {
	name := strings.TrimSpace(item.FileName)
	if name == "" {
		name = strings.TrimSpace(fallbackName)
	}
	if name == "" {
		name = strings.TrimSpace(item.FileID)
	}
	return attachmentTraceFileRef{
		FileID:      strings.TrimSpace(item.FileID),
		FileName:    name,
		Kind:        strings.TrimSpace(item.Kind),
		MimeType:    strings.TrimSpace(item.MimeType),
		ContextMode: strings.TrimSpace(item.ContextMode),
	}
}

func attachmentTracePayloadMap(payload attachmentTracePayload) map[string]interface{} {
	includedCount := len(payload.FileRefs) - len(payload.FileGroupRefs.Skipped)
	if includedCount < 0 {
		includedCount = 0
	}
	return map[string]interface{}{
		"file_mode":       payload.FileMode,
		"file_names":      payload.FileNames,
		"file_refs":       payload.FileRefs,
		"file_groups":     payload.FileGroups,
		"file_group_refs": payload.FileGroupRefs,
		processTracePayloadStage: map[string]interface{}{
			"kind":           processTraceKindFileContext,
			"status":         processTraceStatusReady,
			"included_count": includedCount,
			"skipped_count":  len(payload.FileGroupRefs.Skipped),
		},
	}
}

func buildRAGProcessTrace(
	query string,
	fileObjs []model.FileObject,
	chunks []model.RAGChunk,
) (string, string, map[string]interface{}) {
	if len(fileObjs) == 0 {
		return "", "", nil
	}
	names := make([]string, 0, len(fileObjs))
	for _, item := range fileObjs {
		name := strings.TrimSpace(item.FileName)
		if name == "" {
			name = strings.TrimSpace(item.FileID)
		}
		names = append(names, name)
	}
	citations := make([]map[string]interface{}, 0, len(chunks))
	for _, chunk := range chunks {
		citations = append(citations, map[string]interface{}{
			"file_name":   chunk.FileName,
			"file_id":     chunk.FileID,
			"chunk_index": chunk.ChunkIndex,
			"score":       chunk.Score,
			"preview":     compactSnippet(chunk.Content, 100),
		})
	}
	detail := fmt.Sprintf("检索已完成，共检索 %d 个文件，命中 %d 个段落。", len(names), len(chunks))
	return fmt.Sprintf("检索到 %d 段相关内容", len(chunks)), formatTraceStep("内容检索", detail), map[string]interface{}{
		"query":           compactSnippet(query, 240),
		"file_names":      names,
		"hit_chunk_count": len(chunks),
		"citations":       citations,
		processTracePayloadStage: map[string]interface{}{
			"kind":        processTraceKindRetrieval,
			"status":      processTraceStatusCompleted,
			"file_count":  len(names),
			"chunk_count": len(chunks),
		},
	}
}

func buildToolTrace(rows []model.ToolCall) (string, string, map[string]interface{}) {
	if len(rows) == 0 {
		return "", "", nil
	}
	toolCalls := make([]map[string]interface{}, 0, len(rows))
	lines := make([]string, 0, len(rows))
	successCount := 0
	errorCount := 0
	requestedCount := 0
	for _, row := range rows {
		toolName := strings.TrimSpace(row.ToolName)
		if toolName == "" {
			toolName = "unknown"
		}
		status := strings.TrimSpace(row.Status)
		statusLabel := "已完成"
		switch status {
		case "success":
			statusLabel = "已完成"
			successCount++
		case "reused":
			statusLabel = "已复用"
			successCount++
		case "requested", "streaming":
			statusLabel = "进行中"
			requestedCount++
		case "error", "failed":
			statusLabel = "失败"
			errorCount++
		case "":
			status = "completed"
		}
		parts := []string{statusLabel}
		if row.LatencyMS > 0 {
			parts = append(parts, fmt.Sprintf("%dms", row.LatencyMS))
		}
		input := strings.TrimSpace(row.InputJSON)
		output := strings.TrimSpace(row.OutputJSON)
		inputDisplay := collapseWhitespace(input)
		inputPreview := compactSnippet(inputDisplay, toolTracePreviewMaxChars)
		outputPreview := toolOutputPreview(output)
		inputDetail, inputTruncated := toolTraceDetail(input, toolTraceDetailMaxChars)
		outputDetail, outputTruncated := toolTraceDetail(output, toolTraceDetailMaxChars)
		if strings.TrimSpace(row.ErrorJSON) != "" {
			parts = append(parts, compactSnippet(collapseWhitespace(strings.TrimSpace(row.ErrorJSON)), toolTracePreviewMaxChars))
		} else if outputPreview != "" {
			parts = append(parts, "结果："+outputPreview)
		}
		lines = append(lines, formatTraceStep(toolName, joinTraceParts(parts...)))
		toolCalls = append(toolCalls, map[string]interface{}{
			"tool_call_id":     strings.TrimSpace(row.ToolCallID),
			"name":             toolName,
			"type":             strings.TrimSpace(row.ToolType),
			"status":           status,
			"latency_ms":       row.LatencyMS,
			"error":            strings.TrimSpace(row.ErrorJSON),
			"input_preview":    inputPreview,
			"input_detail":     inputDetail,
			"input_size":       len(input),
			"input_truncated":  inputTruncated,
			"output_preview":   outputPreview,
			"output_detail":    outputDetail,
			"output_size":      len(output),
			"output_truncated": outputTruncated,
		})
	}
	summary := fmt.Sprintf("%d 次工具调用已完成", len(rows))
	if requestedCount > 0 && successCount == 0 && errorCount == 0 {
		summary = fmt.Sprintf("%d 次工具调用进行中", len(rows))
	} else if errorCount > 0 {
		summary = fmt.Sprintf("%d 次工具调用，%d 次失败", len(rows), errorCount)
	} else if successCount == len(rows) {
		summary = fmt.Sprintf("%d 次工具调用已完成", len(rows))
	}
	return summary, strings.Join(lines, "\n"), map[string]interface{}{
		"tool_calls": toolCalls,
	}
}

func toolOutputPreview(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	var payload interface{}
	if err := json.Unmarshal([]byte(value), &payload); err == nil {
		if text := readableMCPToolResultPreview(payload); text != "" {
			return compactSnippet(collapseWhitespace(text), toolTracePreviewMaxChars)
		}
		if text := readableJSONPreview(payload); text != "" {
			return compactSnippet(collapseWhitespace(text), toolTracePreviewMaxChars)
		}
		if normalized, marshalErr := json.Marshal(payload); marshalErr == nil {
			value = string(normalized)
		}
	}
	return compactSnippet(collapseWhitespace(value), toolTracePreviewMaxChars)
}

func toolTraceDetail(raw string, maxChars int) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}
	runes := []rune(value)
	if maxChars <= 0 {
		maxChars = toolTraceDetailMaxChars
	}
	if len(runes) <= maxChars {
		return value, false
	}
	return compactSnippet(collapseWhitespace(value), maxChars), true
}

func readableMCPToolResultPreview(value interface{}) string {
	payload, ok := value.(map[string]interface{})
	if !ok || !looksLikeMCPToolResult(payload) {
		return ""
	}

	parts := make([]string, 0, 4)
	if text := readableMCPContentPreview(payload["content"]); text != "" {
		parts = append(parts, text)
	}
	if text := readableJSONPreview(payload["structuredContent"]); text != "" {
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		if summary := summarizeMCPContent(payload["content"]); summary != "" {
			parts = append(parts, summary)
		}
	}
	return strings.Join(parts, "；")
}

func looksLikeMCPToolResult(payload map[string]interface{}) bool {
	if _, ok := payload["content"]; ok {
		return true
	}
	if _, ok := payload["structuredContent"]; ok {
		return true
	}
	if _, ok := payload["isError"]; ok {
		return true
	}
	return false
}

func readableMCPContentPreview(value interface{}) string {
	items, ok := value.([]interface{})
	if !ok || len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, min(len(items), 3))
	for _, item := range items {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if text := readableMCPTextBlock(block); text != "" {
			parts = append(parts, text)
		}
		if len(parts) >= 3 {
			break
		}
	}
	return strings.Join(parts, "；")
}

func readableMCPTextBlock(block map[string]interface{}) string {
	text := stringFromJSONValue(block["text"])
	if text == "" {
		return ""
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		if preview := readableJSONPreview(parsed); preview != "" {
			return preview
		}
	}
	return text
}

func summarizeMCPContent(value interface{}) string {
	items, ok := value.([]interface{})
	if !ok || len(items) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, item := range items {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		blockType := stringFromJSONValue(block["type"])
		if blockType == "" {
			blockType = "content"
		}
		counts[blockType]++
	}
	summaries := make([]string, 0, 4)
	if counts["image"] > 0 {
		summaries = append(summaries, fmt.Sprintf("返回 %d 张图片", counts["image"]))
	}
	if counts["audio"] > 0 {
		summaries = append(summaries, fmt.Sprintf("返回 %d 段音频", counts["audio"]))
	}
	if counts["resource"] > 0 || counts["resource_link"] > 0 {
		summaries = append(summaries, fmt.Sprintf("返回 %d 个资源", counts["resource"]+counts["resource_link"]))
	}
	return strings.Join(summaries, "；")
}

func readableJSONPreview(value interface{}) string {
	switch typed := value.(type) {
	case []interface{}:
		parts := make([]string, 0, min(len(typed), 3))
		for _, item := range typed {
			if text := readableJSONPreview(item); text != "" {
				parts = append(parts, text)
			}
			if len(parts) >= 3 {
				break
			}
		}
		return strings.Join(parts, "；")
	case map[string]interface{}:
		for _, key := range []string{"summary", "answer", "text", "content", "message", "result"} {
			if text := stringFromJSONValue(typed[key]); text != "" {
				return text
			}
		}
		if title := stringFromJSONValue(typed["title"]); title != "" {
			if url := stringFromJSONValue(typed["url"]); url != "" {
				return title + " " + url
			}
			return title
		}
		for _, key := range []string{"url", "uri", "link"} {
			if text := stringFromJSONValue(typed[key]); text != "" {
				return text
			}
		}
		for _, key := range []string{"results", "items", "data", "sources", "citations"} {
			if text := readableJSONPreview(typed[key]); text != "" {
				return text
			}
		}
	case string:
		return typed
	}
	return ""
}

func stringFromJSONValue(value interface{}) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func collapseWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func buildCompactionProcessTrace(snapshot *model.ContextSnapshot) (string, string, map[string]interface{}) {
	if snapshot == nil {
		return "", "", nil
	}
	detail := strings.Join([]string{
		"对话已压缩并生成滚动摘要。",
		fmt.Sprintf("- 压缩区间：第 %d-%d 轮。", snapshot.FromTurn, snapshot.ToTurn),
		fmt.Sprintf("- Tokens 缩减：%d → %d。", snapshot.SourceTokens, snapshot.SummaryTokens),
	}, "\n")
	return fmt.Sprintf("已压缩第 %d-%d 轮上下文", snapshot.FromTurn, snapshot.ToTurn), formatTraceStep("上下文压缩", detail), map[string]interface{}{
		"strategy":       snapshot.Strategy,
		"from_turn":      snapshot.FromTurn,
		"to_turn":        snapshot.ToTurn,
		"source_tokens":  snapshot.SourceTokens,
		"summary_tokens": snapshot.SummaryTokens,
		processTracePayloadStage: map[string]interface{}{
			"kind":           processTraceKindCompaction,
			"status":         processTraceStatusCompleted,
			"from_turn":      snapshot.FromTurn,
			"to_turn":        snapshot.ToTurn,
			"source_tokens":  snapshot.SourceTokens,
			"summary_tokens": snapshot.SummaryTokens,
		},
	}
}

func buildPromptTraceSummary(trace *model.MessagePromptTrace) string {
	if trace == nil {
		return ""
	}
	if trace.StatefulUsed {
		return fmt.Sprintf("续接发送 %d 条消息", trace.SentMessageCount)
	}
	return fmt.Sprintf("准备 %d tokens 上下文", trace.SentTokenEstimate)
}

type thinkingDeltaRouter struct {
	buffer     string
	tagName    string
	inThinking bool
	resolved   bool
}

func (r *thinkingDeltaRouter) consume(delta string) (string, string) {
	if delta == "" {
		return "", ""
	}
	if r.resolved {
		return delta, ""
	}
	if r.inThinking {
		return r.consumeThinking(delta)
	}
	r.buffer += delta
	_, tagName, openEnd, openPending, ok := parseLeadingThinkingOpenTag(r.buffer)
	if openPending {
		return "", ""
	}
	if !ok {
		visible := r.buffer
		r.buffer = ""
		r.resolved = true
		return visible, ""
	}
	tail := r.buffer[openEnd:]
	r.buffer = ""
	r.tagName = tagName
	r.inThinking = true
	return r.consumeThinking(tail)
}

func (r *thinkingDeltaRouter) consumeThinking(delta string) (string, string) {
	if delta == "" {
		return "", ""
	}
	r.buffer += delta
	closeStart, closeEnd, found := findThinkingCloseTag(r.buffer, 0, r.tagName)
	if found {
		think := r.buffer[:closeStart]
		visible := r.buffer[closeEnd:]
		r.buffer = ""
		r.tagName = ""
		r.inThinking = false
		r.resolved = true
		return visible, think
	}
	think, carry := splitThinkingCloseSafeRemainder(r.buffer, r.tagName)
	r.buffer = carry
	return "", think
}

func (r *thinkingDeltaRouter) flush() (string, string) {
	if r.resolved {
		return "", ""
	}
	if r.buffer == "" {
		r.inThinking = false
		r.tagName = ""
		r.resolved = true
		return "", ""
	}
	value := r.buffer
	r.buffer = ""
	if r.inThinking {
		r.inThinking = false
		r.tagName = ""
		r.resolved = true
		return "", value
	}
	r.resolved = true
	return value, ""
}

func splitThinkingContent(content string) (string, string) {
	visible, think, _ := splitLeadingThinkingBlock(content, true)
	return strings.TrimSpace(visible), strings.TrimSpace(think)
}

func splitAssistantOutputThinkingContent(content string) (string, string) {
	_, tagName, openEnd, openPending, ok := parseLeadingThinkingOpenTag(content)
	if openPending {
		return strings.TrimSpace(content), ""
	}
	if !ok {
		return strings.TrimSpace(content), ""
	}
	closeStart, closeEnd, found := findThinkingCloseTag(content, openEnd, tagName)
	if !found {
		return "", strings.TrimSpace(content[openEnd:])
	}
	return strings.TrimSpace(content[closeEnd:]), strings.TrimSpace(content[openEnd:closeStart])
}

func splitLeadingThinkingBlock(content string, flush bool) (visible string, think string, pending bool) {
	if content == "" {
		return "", "", false
	}
	_, tagName, openEnd, openPending, ok := parseLeadingThinkingOpenTag(content)
	if openPending {
		if flush {
			return content, "", false
		}
		return "", "", true
	}
	if !ok {
		return content, "", false
	}
	closeStart, closeEnd, found := findThinkingCloseTag(content, openEnd, tagName)
	if !found {
		if flush {
			return content, "", false
		}
		return "", "", true
	}
	return content[closeEnd:], content[openEnd:closeStart], false
}

func parseLeadingThinkingOpenTag(content string) (prefixEnd int, tagName string, openEnd int, pending bool, ok bool) {
	prefixEnd = leadingWhitespaceEnd(content)
	if prefixEnd >= len(content) {
		return prefixEnd, "", 0, true, false
	}
	if content[prefixEnd] != '<' {
		return prefixEnd, "", 0, false, false
	}
	closeAngle := strings.IndexByte(content[prefixEnd:], '>')
	if closeAngle < 0 {
		return prefixEnd, "", 0, isPotentialThinkingOpenTagPrefix(content[prefixEnd:]), false
	}
	openEnd = prefixEnd + closeAngle + 1
	body := strings.TrimSpace(content[prefixEnd+1 : openEnd-1])
	if body == "" || strings.HasPrefix(body, "/") || strings.HasSuffix(body, "/") {
		return prefixEnd, "", 0, false, false
	}
	name := strings.ToLower(strings.Fields(body)[0])
	switch name {
	case "think", "thinking":
		return prefixEnd, name, openEnd, false, true
	default:
		return prefixEnd, "", 0, false, false
	}
}

func isPotentialThinkingOpenTagPrefix(fragment string) bool {
	lower := strings.ToLower(fragment)
	for _, tagName := range []string{"think", "thinking"} {
		candidate := "<" + tagName
		if strings.HasPrefix(candidate, lower) {
			return true
		}
		if strings.HasPrefix(lower, candidate) {
			if len(lower) == len(candidate) {
				return true
			}
			next := lower[len(candidate)]
			if next == '/' || isASCIIWhitespace(next) {
				return true
			}
		}
	}
	return false
}

func findThinkingCloseTag(content string, start int, tagName string) (int, int, bool) {
	lower := strings.ToLower(content)
	target := "</" + tagName
	searchStart := start
	for {
		relative := strings.Index(lower[searchStart:], target)
		if relative < 0 {
			return 0, 0, false
		}
		closeStart := searchStart + relative
		closeEnd := closeStart + len(target)
		for closeEnd < len(content) && isASCIIWhitespace(content[closeEnd]) {
			closeEnd++
		}
		if closeEnd < len(content) && content[closeEnd] == '>' {
			return closeStart, closeEnd + 1, true
		}
		searchStart = closeStart + len(target)
	}
}

func splitThinkingCloseSafeRemainder(value string, tagName string) (string, string) {
	lastLeft := strings.LastIndex(value, "<")
	if lastLeft < 0 {
		return value, ""
	}
	suffix := value[lastLeft:]
	if isPotentialThinkingCloseTagPrefix(suffix, tagName) {
		return value[:lastLeft], suffix
	}
	return value, ""
}

func isPotentialThinkingCloseTagPrefix(fragment string, tagName string) bool {
	lower := strings.ToLower(fragment)
	target := "</" + strings.ToLower(strings.TrimSpace(tagName))
	if target == "</" {
		return false
	}
	if strings.HasPrefix(target, lower) {
		return true
	}
	if !strings.HasPrefix(lower, target) {
		return false
	}
	for index := len(target); index < len(lower); index++ {
		if !isASCIIWhitespace(lower[index]) {
			return false
		}
	}
	return true
}

func leadingWhitespaceEnd(content string) int {
	for index, item := range content {
		if !isWhitespaceRune(item) {
			return index
		}
	}
	return len(content)
}

func isWhitespaceRune(item rune) bool {
	switch item {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	default:
		return false
	}
}

func isASCIIWhitespace(item byte) bool {
	switch item {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	default:
		return false
	}
}
