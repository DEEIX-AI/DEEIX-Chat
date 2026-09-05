package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/toolresult"
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
	processTraceStatusPending       = "pending"
	processTraceStatusFailed        = "failed"
	processTraceFallbackFullText    = "full_text"
	processTraceFallbackUnavailable = "unavailable"
)

const (
	toolTraceCompactSummaryMaxChars      = 260
	toolTraceLegacyOutputPreviewMaxChars = 512
	toolTraceDetailMaxChars              = 4096
	maxTracePayloadBytes                 = 1024 * 1024
)

const (
	upstreamThinkLiveFlushInterval = 80 * time.Millisecond
	upstreamThinkLiveFlushBytes    = 1024
	upstreamThinkPersistInterval   = 2 * time.Second
	upstreamThinkLiveReplaceBytes  = 16 * 1024
	toolTraceLiveFlushInterval     = 80 * time.Millisecond
	toolTracePersistInterval       = 2 * time.Second
	tracePersistenceDrainTimeout   = 5 * time.Second
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
	payload         *tracePayload
	seq             int
	startedAt       time.Time
	endedAt         *time.Time
}

type tracePersistenceJob struct {
	ctx         context.Context
	draft       messageTraceDraft
	payloadJSON string
}

type messageTraceRecorder struct {
	service         *Service
	ctx             context.Context
	cfg             config.Config
	assistant       *model.Message
	onEvent         func(string, map[string]any) error
	ephemeral       bool
	process         *messageTraceDraft
	tools           *messageTraceDraft
	upstreamThink   *messageTraceDraft
	promptTrace     *model.MessagePromptTrace
	nextEventSeq    int
	nextRoundSeq    int
	eventCounters   map[string]int
	events          []model.MessageTraceEvent
	toolRoundClosed bool

	upstreamThinkLastLiveFlush  time.Time
	upstreamThinkLastPersist    time.Time
	upstreamThinkPendingText    strings.Builder
	upstreamThinkPendingReplace string
	upstreamThinkPendingKind    string
	upstreamThinkPendingReason  *traceReasoning
	upstreamThinkBufferedByte   int
	failed                      bool
	compactionPreviousSummary   string
	toolLastLiveFlush           time.Time
	toolLastPersist             time.Time

	persistQueueMu    sync.Mutex
	persistQueue      []tracePersistenceJob
	persistWorkerDone chan struct{}
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
	onEvent func(string, map[string]any) error,
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

func newEphemeralMessageTraceRecorder(
	service *Service,
	ctx context.Context,
	assistant *model.Message,
	onEvent func(string, map[string]any) error,
) *messageTraceRecorder {
	recorder := newMessageTraceRecorder(service, ctx, assistant, onEvent)
	if recorder != nil {
		recorder.ephemeral = true
	}
	return recorder
}

func (r *messageTraceRecorder) enabled() bool {
	return r != nil && r.cfg.ProcessTraceEnabled && r.assistant != nil
}

func (r *messageTraceRecorder) visible() bool {
	return r.enabled() && r.cfg.ProcessTraceVisibleToUser
}

// completeForBackgroundContinuation 同步落盘当前 trace 后切换到后台上下文。
// 进程 trace 在存在后台压缩阶段时保持 streaming，工具与模型思考仍正常收尾；
// 这样响应消息能明确展示“压缩排队中”，后台完成或失败后再原位更新该阶段。
func (r *messageTraceRecorder) completeForBackgroundContinuation() {
	if !r.enabled() {
		return
	}
	ctx, cancel := background.WithTimeout(r.ctx, 5*time.Second)
	defer cancel()
	now := time.Now()
	for _, draft := range []*messageTraceDraft{r.process, r.tools, r.upstreamThink} {
		if draft == nil || draft.status == messageTraceStatusCompleted || draft.status == messageTraceStatusError {
			continue
		}
		if draft == r.process && processTraceStageHasStatus(draft.payload, processTraceKindCompaction, processTraceStatusPending) {
			r.persistDraftCtx(ctx, draft, true)
			continue
		}
		draft.status = messageTraceStatusCompleted
		draft.endedAt = &now
		if draft.traceType != messageTraceTypeTools {
			r.upsertSnapshotEvent(draft, tracePayloadJSON(draft.payload))
		}
		r.persistDraftCtx(ctx, draft, true)
	}
	r.ctx = background.Detach(r.ctx)
	r.onEvent = nil
}

func (r *messageTraceRecorder) setCompactionProcessStage(summary string, markdown string, payload *tracePayload) {
	if !r.enabled() {
		return
	}
	draft := r.ensureDraft(messageTraceTypeProcess)
	if draft == nil {
		return
	}
	if value := strings.TrimSpace(markdown); value != "" && !strings.Contains(draft.contentMarkdown, value) {
		if draft.contentMarkdown != "" {
			draft.contentMarkdown += "\n\n"
		}
		draft.contentMarkdown += value
	}
	stage := firstTraceStage(payload)
	stageStatus := ""
	if stage != nil {
		stageStatus = stage.Status
	}
	if strings.TrimSpace(stageStatus) == processTraceStatusPending &&
		!processTraceStageHasStatus(draft.payload, processTraceKindCompaction, processTraceStatusPending) {
		r.compactionPreviousSummary = draft.summary
	}
	if value := strings.TrimSpace(summary); value != "" {
		draft.summary = value
	}
	if stage != nil {
		upsertProcessTraceStagePayload(draft.payload, stage)
	}
	if payload != nil {
		metadata := payload.clone()
		metadata.TraceStage = nil
		metadata.Stages = nil
		mergeTracePayload(draft.payload, metadata)
	}
	draft.status = messageTraceStatusStreaming
	draft.endedAt = nil
	r.persistDraft(draft, false)
	r.emitProcessUpdate()
}

func (r *messageTraceRecorder) removeProcessStage(kind string) {
	if !r.enabled() || r.process == nil {
		return
	}
	stages := append([]traceStage(nil), r.process.payload.Stages...)
	filtered := stages[:0]
	for _, stage := range stages {
		if strings.TrimSpace(stage.Kind) != strings.TrimSpace(kind) {
			filtered = append(filtered, stage)
		}
	}
	r.process.payload.Stages = filtered
	if value := strings.TrimSpace(r.compactionPreviousSummary); value != "" {
		r.process.summary = value
	}
	r.compactionPreviousSummary = ""
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
	default:
		return nil
	}
}

func (r *messageTraceRecorder) ensureToolDraft(roundID string, parentEventID string) *messageTraceDraft {
	if !r.enabled() {
		return nil
	}
	roundID = strings.TrimSpace(roundID)
	if r.tools == nil || r.tools.roundID != roundID {
		r.tools = r.newTraceDraft(messageTraceTypeTools, "tool", "工具", 2, messageTraceStageTool, roundID, parentEventID)
		r.toolRoundClosed = false
	} else if parentEventID = strings.TrimSpace(parentEventID); parentEventID != "" {
		r.tools.parentEventID = parentEventID
	}
	return r.tools
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
		payload:       &tracePayload{},
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

func (r *messageTraceRecorder) appendProcessSection(summary string, markdown string, payload *tracePayload, status string) {
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

func (r *messageTraceRecorder) appendToolSection(summary string, markdown string, payload *tracePayload, status string) {
	if !r.enabled() {
		return
	}
	value := strings.TrimSpace(markdown)
	if value == "" {
		return
	}
	r.completeProcess()
	roundID, parentEventID := r.currentToolTraceBinding()
	draft := r.ensureToolDraft(roundID, parentEventID)
	if draft == nil {
		return
	}
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
	if aggregateStatus := toolTracePayloadStatus(draft.payload); aggregateStatus != "" {
		draft.status = aggregateStatus
	} else if strings.TrimSpace(status) != "" {
		nextStatus := strings.TrimSpace(status)
		draft.status = nextStatus
	}
	r.updateToolDraftEndTime(draft)
	if aggregateSummary := summarizeToolTraceDraft(draft); aggregateSummary != "" {
		draft.summary = aggregateSummary
	} else if strings.TrimSpace(summary) != "" {
		draft.summary = strings.TrimSpace(summary)
	}
	r.flushToolDraft(draft)
}

func (r *messageTraceRecorder) syncToolSection(summary string, markdown string, payload *tracePayload, status string) {
	if !r.enabled() {
		return
	}
	value := strings.TrimSpace(markdown)
	if value == "" {
		return
	}
	r.completeProcess()
	roundID, parentEventID := r.currentToolTraceBinding()
	draft := r.ensureToolDraft(roundID, parentEventID)
	if draft == nil {
		return
	}
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
		draft.payload = payload.clone()
		if strings.TrimSpace(summary) != "" {
			draft.summary = strings.TrimSpace(summary)
		} else if aggregateSummary := summarizeToolTracePayload(payload); aggregateSummary != "" {
			draft.summary = aggregateSummary
		}
	}
	if aggregateStatus := toolTracePayloadStatus(draft.payload); aggregateStatus != "" {
		draft.status = aggregateStatus
	} else if strings.TrimSpace(status) != "" {
		nextStatus := strings.TrimSpace(status)
		draft.status = nextStatus
	}
	r.updateToolDraftEndTime(draft)
	r.flushToolDraft(draft)
}

func (r *messageTraceRecorder) updateToolDraftEndTime(draft *messageTraceDraft) {
	if draft == nil {
		return
	}
	switch draft.status {
	case messageTraceStatusStreaming:
		draft.endedAt = nil
	case messageTraceStatusCompleted, messageTraceStatusError:
		now := time.Now()
		draft.endedAt = &now
	}
}

func (r *messageTraceRecorder) flushToolDraft(draft *messageTraceDraft) {
	if !r.enabled() || draft == nil {
		return
	}
	now := time.Now()
	payloadJSON := tracePayloadJSON(draft.payload)
	r.upsertSnapshotEvent(draft, payloadJSON)
	terminal := draft.status == messageTraceStatusCompleted || draft.status == messageTraceStatusError
	if terminal {
		if r.service != nil && r.service.repo != nil {
			r.enqueueDraftPersistence(draft, payloadJSON)
		}
	} else if !r.ephemeral && r.cfg.ProcessTracePersistInflight && (r.toolLastPersist.IsZero() || now.Sub(r.toolLastPersist) >= toolTracePersistInterval) {
		if r.service != nil && r.service.repo != nil {
			r.persistMessageTraceRow(r.ctx, draft, payloadJSON)
			r.persistTraceEventRow(r.ctx, draft, payloadJSON)
		}
		r.toolLastPersist = now
	}
	if terminal || r.toolLastLiveFlush.IsZero() || now.Sub(r.toolLastLiveFlush) >= toolTraceLiveFlushInterval {
		r.emitToolUpdate()
		r.toolLastLiveFlush = now
	}
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
		if r.tools == nil || r.tools.roundID != roundID || !r.toolRoundClosed {
			return roundID, strings.TrimSpace(r.upstreamThink.eventID)
		}
	}
	if r.tools != nil && !r.toolRoundClosed && strings.TrimSpace(r.tools.roundID) != "" {
		return strings.TrimSpace(r.tools.roundID), strings.TrimSpace(r.tools.parentEventID)
	}
	return r.nextTraceRoundID(), ""
}

func (r *messageTraceRecorder) appendUpstreamReasoning(kind string, text string, payload *tracePayload) {
	if !r.enabled() {
		return
	}
	if reasoningItemChanged(r.upstreamThink, payload) {
		r.completeTools()
		r.completeUpstreamThink()
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

func (r *messageTraceRecorder) syncStructuredThink(content string, summary string, payload *tracePayload) {
	if !r.enabled() {
		return
	}
	if content == "" && summary == "" {
		return
	}
	r.completeProcess()
	if reasoningItemChanged(r.upstreamThink, payload) {
		r.completeTools()
		r.completeUpstreamThink()
	}
	draft := r.ensureDraft(messageTraceTypeUpstreamThink)
	if draft == nil {
		return
	}
	r.updateStructuredThinkDraft(draft, content, summary, payload)
}

// reconcileStructuredThink applies the final upstream snapshot to the reasoning
// event already emitted for the same item. A completed stream event is still the
// canonical event for that item; final response reconciliation must not create a
// second round merely because the live event has already completed.
func (r *messageTraceRecorder) reconcileStructuredThink(content string, summary string, payload *tracePayload) {
	if !r.enabled() || (content == "" && summary == "") {
		return
	}
	r.completeProcess()
	draft := r.upstreamThink
	if reasoningItemChanged(draft, payload) {
		r.completeTools()
		r.completeUpstreamThink()
		draft = nil
	}
	if draft == nil {
		draft = r.ensureDraft(messageTraceTypeUpstreamThink)
	}
	if draft == nil {
		return
	}
	terminal := draft.status == messageTraceStatusCompleted || draft.status == messageTraceStatusError
	r.updateStructuredThinkDraft(draft, content, summary, payload)
	if terminal {
		r.commitTerminalDraft(draft)
		r.flushUpstreamThinkLiveUpdate(draft, upstreamThinkLiveUpdateOptions{Force: true})
	}
}

func (r *messageTraceRecorder) updateStructuredThinkDraft(draft *messageTraceDraft, content string, summary string, payload *tracePayload) {
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

func reasoningItemChanged(draft *messageTraceDraft, payload *tracePayload) bool {
	if draft == nil {
		return false
	}
	if payload == nil {
		return false
	}
	nextItemID := strings.TrimSpace(payload.ReasoningItemID())
	if nextItemID == "" {
		return false
	}
	currentItemID := ""
	if draft.payload != nil && draft.payload.Reasoning != nil {
		currentItemID = strings.TrimSpace(draft.payload.Reasoning.ItemID)
	}
	return currentItemID != "" && currentItemID != nextItemID
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
		draft.payload = &tracePayload{}
	}
	draft.payload.PromptTrace = tracePayloadFromPromptTrace(trace)
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
	r.commitTerminalDraft(draft)
	return true
}

func (r *messageTraceRecorder) commitTerminalDraft(draft *messageTraceDraft) {
	if !r.enabled() || draft == nil {
		return
	}
	payloadJSON := tracePayloadJSON(draft.payload)
	r.upsertSnapshotEvent(draft, payloadJSON)
	if r.service != nil && r.service.repo != nil {
		r.enqueueDraftPersistence(draft, payloadJSON)
	}
}

func (r *messageTraceRecorder) completeProcess() {
	if r.completeDraft(r.process) {
		r.emitProcessUpdate()
	}
}

func (r *messageTraceRecorder) completeTools() {
	changed := r.completeDraft(r.tools)
	r.toolRoundClosed = true
	if changed {
		r.emitToolUpdate()
	}
}

func (r *messageTraceRecorder) completeUpstreamThink() {
	if r.completeDraft(r.upstreamThink) {
		r.flushUpstreamThinkLiveUpdate(r.upstreamThink, upstreamThinkLiveUpdateOptions{Force: true})
	}
}

func (r *messageTraceRecorder) complete() {
	r.completeProcess()
	r.completeTools()
	r.completeUpstreamThink()
	ctx, cancel := background.WithTimeout(r.ctx, tracePersistenceDrainTimeout)
	defer cancel()
	r.waitForPendingPersistence(ctx)
}

func (r *messageTraceRecorder) fail(err error) {
	ctx, cancel := background.WithTimeout(r.ctx, tracePersistenceDrainTimeout)
	defer cancel()
	r.failWithContext(ctx, err)
}

func (r *messageTraceRecorder) failWithContext(ctx context.Context, err error) {
	if !r.enabled() {
		return
	}
	if r.failed {
		return
	}
	r.failed = true
	r.waitForPendingPersistence(ctx)
	now := time.Now()
	summary := traceErrorSummary(err)
	detail := traceErrorDetail(err)
	process := r.ensureDraft(messageTraceTypeProcess)
	if process != nil {
		process.status = messageTraceStatusError
		if summary != "" {
			process.summary = summary
		}
		payload := &tracePayload{}
		if detail != "" {
			payload.Error = detail
		}
		if debug := messageErrorDebug(err); debug != nil {
			if raw, marshalErr := json.Marshal(debug); marshalErr == nil {
				payload.UpstreamDebug = raw
			}
		}
		mergeTracePayload(process.payload, payload)
		process.endedAt = &now
		r.persistDraftCtx(ctx, process, true)
	}
	if r.upstreamThink != nil {
		r.upstreamThink.status = messageTraceStatusError
		r.upstreamThink.endedAt = &now
		r.flushUpstreamThinkLiveUpdate(r.upstreamThink, upstreamThinkLiveUpdateOptions{Force: true})
		r.persistDraftCtx(ctx, r.upstreamThink, true)
	}
	if r.tools != nil {
		r.tools.status = messageTraceStatusError
		r.tools.endedAt = &now
		r.persistDraftCtx(ctx, r.tools, true)
	}
}

func (r *messageTraceRecorder) attachToMessage(message *model.Message) {
	if message == nil || !r.visible() {
		return
	}
	message.ProcessTrace = r.snapshot()
}

func (r *messageTraceRecorder) upstreamThinkContent() string {
	if r == nil || r.upstreamThink == nil {
		return ""
	}
	return r.upstreamThink.contentMarkdown
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

// enqueueDraftPersistence serializes terminal trace writes in event order. The
// JSON payload is materialized before the goroutine starts so later live-event
// reconciliation cannot mutate data being persisted in the background.
func (r *messageTraceRecorder) enqueueDraftPersistence(draft *messageTraceDraft, payloadJSON string) {
	if !r.enabled() || r.ephemeral || draft == nil || r.service == nil || r.service.repo == nil {
		return
	}
	r.persistQueueMu.Lock()
	snapshot := *draft
	snapshot.payload = nil
	r.persistQueue = append(r.persistQueue, tracePersistenceJob{
		ctx:         background.Detach(r.ctx),
		draft:       snapshot,
		payloadJSON: payloadJSON,
	})
	if r.persistWorkerDone != nil {
		r.persistQueueMu.Unlock()
		return
	}
	done := make(chan struct{})
	r.persistWorkerDone = done
	r.persistQueueMu.Unlock()

	go r.runPersistenceWorker(done)
}

func (r *messageTraceRecorder) runPersistenceWorker(done chan struct{}) {
	for {
		r.persistQueueMu.Lock()
		if len(r.persistQueue) == 0 {
			r.persistWorkerDone = nil
			close(done)
			r.persistQueueMu.Unlock()
			return
		}
		job := r.persistQueue[0]
		r.persistQueue[0] = tracePersistenceJob{}
		r.persistQueue = r.persistQueue[1:]
		r.persistQueueMu.Unlock()

		r.persistDraftBackground(job.ctx, &job.draft, job.payloadJSON)
	}
}

func (r *messageTraceRecorder) waitForPendingPersistence(ctx context.Context) {
	if r == nil {
		return
	}
	r.persistQueueMu.Lock()
	pending := r.persistWorkerDone
	r.persistQueueMu.Unlock()
	if pending == nil {
		return
	}
	select {
	case <-pending:
	case <-ctx.Done():
	}
}

// persistDraftBackground uses a detached timeout because terminal trace
// durability must not depend on the client request remaining connected.
func (r *messageTraceRecorder) persistDraftBackground(parent context.Context, draft *messageTraceDraft, payloadJSON string) {
	ctx, cancel := background.WithTimeout(parent, 5*time.Second)
	defer cancel()
	if !r.enabled() || r.ephemeral || draft == nil {
		return
	}
	r.persistMessageTraceRow(ctx, draft, payloadJSON)
	r.persistTraceEventRow(ctx, draft, payloadJSON)
}

func (r *messageTraceRecorder) persistDraftCtx(ctx context.Context, draft *messageTraceDraft, force bool) {
	if !r.enabled() || draft == nil {
		return
	}
	payloadJSON := tracePayloadJSON(draft.payload)
	r.upsertSnapshotEvent(draft, payloadJSON)
	if r.ephemeral {
		return
	}
	if !force && !r.cfg.ProcessTracePersistInflight {
		return
	}
	if r.service == nil || r.service.repo == nil {
		return
	}
	r.persistMessageTraceRow(ctx, draft, payloadJSON)
	r.persistTraceEventRow(ctx, draft, payloadJSON)
}

type upstreamThinkLiveUpdate struct {
	kind            string
	delta           string
	contentMarkdown string
	reasoning       *traceReasoning
}

func (r *messageTraceRecorder) queueUpstreamThinkLiveUpdate(draft *messageTraceDraft, kind string, deltaText string, replaceText string, payload *tracePayload) {
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
	if reasoning := liveUpstreamReasoningPayload(kind, payload); reasoning != nil {
		r.upstreamThinkPendingReason = reasoning
	}
	if !r.shouldFlushUpstreamThinkLiveUpdate() {
		return
	}
	r.flushUpstreamThinkLiveUpdate(draft, upstreamThinkLiveUpdateOptions{PersistSnapshot: true})
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

type upstreamThinkLiveUpdateOptions struct {
	Force           bool
	PersistSnapshot bool
}

func (r *messageTraceRecorder) flushUpstreamThinkLiveUpdate(draft *messageTraceDraft, options upstreamThinkLiveUpdateOptions) {
	if !r.enabled() || draft == nil {
		return
	}
	update := upstreamThinkLiveUpdate{
		kind:            r.upstreamThinkPendingKind,
		delta:           r.upstreamThinkPendingText.String(),
		contentMarkdown: r.upstreamThinkPendingReplace,
		reasoning:       r.upstreamThinkPendingReason,
	}
	if !options.Force && update.delta == "" && update.contentMarkdown == "" && update.reasoning == nil {
		return
	}
	if options.PersistSnapshot {
		r.refreshSnapshotEvent(draft)
		if r.shouldPersistUpstreamThinkSnapshot() {
			r.persistDraft(draft, false)
			r.upstreamThinkLastPersist = time.Now()
		}
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
	if r == nil || r.ephemeral || r.service == nil || r.service.repo == nil || r.assistant == nil || draft == nil {
		return
	}
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
		Summary:         textutil.TruncateTrimmed(strings.TrimSpace(draft.summary), 255),
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

func tracePayloadJSON(payload *tracePayload) string {
	if payload == nil {
		return "{}"
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	if len(raw) > maxTracePayloadBytes {
		return tracePayloadOmittedJSON(len(raw))
	}
	return string(raw)
}

func tracePayloadOmittedJSON(originalBytes int) string {
	type tracePayloadOmitted struct {
		PayloadOmitted bool   `json:"payloadOmitted"`
		OriginalBytes  int    `json:"originalBytes"`
		Reason         string `json:"reason"`
	}
	type tracePayloadOmittedEnvelope struct {
		Trace tracePayloadOmitted `json:"_trace"`
	}
	raw, err := json.Marshal(tracePayloadOmittedEnvelope{Trace: tracePayloadOmitted{
		PayloadOmitted: true,
		OriginalBytes:  originalBytes,
		Reason:         "payload_too_large",
	}})
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func (r *messageTraceRecorder) persistTraceEventRow(ctx context.Context, draft *messageTraceDraft, payloadJSON string) {
	if r == nil || r.ephemeral || r.service == nil || r.service.repo == nil || r.assistant == nil || draft == nil {
		return
	}
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
		Summary:         textutil.TruncateTrimmed(strings.TrimSpace(draft.summary), 255),
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
	r.storeSnapshotEvent(draft, payloadJSON, true)
}

func (r *messageTraceRecorder) refreshSnapshotEvent(draft *messageTraceDraft) {
	r.storeSnapshotEvent(draft, "", false)
}

func (r *messageTraceRecorder) storeSnapshotEvent(draft *messageTraceDraft, payloadJSON string, replacePayload bool) {
	if draft == nil {
		return
	}
	if payloadJSON == "" {
		payloadJSON = "{}"
	}
	event := model.MessageTraceEvent{
		EventID:         draft.eventID,
		EventType:       draft.eventType,
		Phase:           draft.traceType,
		Stage:           draft.stage,
		RoundID:         draft.roundID,
		ParentEventID:   draft.parentEventID,
		Title:           draft.title,
		Summary:         textutil.TruncateTrimmed(strings.TrimSpace(draft.summary), 255),
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
			if !replacePayload {
				event.PayloadJSON = item.PayloadJSON
			}
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
	emitEvent(r.onEvent, "process_update", map[string]any{
		"status": r.process.status,
		"block":  traceDraftToBlock(r.process),
		"trace":  r.snapshot(),
	})
}

func (r *messageTraceRecorder) emitToolUpdate() {
	if !r.visible() || r.tools == nil {
		return
	}
	emitEvent(r.onEvent, "process_update", map[string]any{
		"status": r.tools.status,
		"block":  traceDraftToBlock(r.tools),
		"trace":  r.snapshot(),
	})
}

func (r *messageTraceRecorder) emitUpstreamThinkDelta(update upstreamThinkLiveUpdate) {
	if !r.visible() || r.upstreamThink == nil {
		return
	}
	payload := map[string]any{
		"status":    r.upstreamThink.status,
		"title":     r.upstreamThink.title,
		"summary":   r.upstreamThink.summary,
		"stage":     r.upstreamThink.stage,
		"roundID":   r.upstreamThink.roundID,
		"eventID":   r.upstreamThink.eventID,
		"startedAt": r.upstreamThink.startedAt,
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
	if update.reasoning != nil {
		payload["reasoning"] = update.reasoning
	}
	if r.upstreamThink.endedAt != nil {
		payload["endedAt"] = *r.upstreamThink.endedAt
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
	payloadJSON := ""
	if draft.payload != nil {
		payloadJSON = tracePayloadJSON(draft.payload)
	}
	return &model.MessageTraceBlock{
		Title:           draft.title,
		Summary:         draft.summary,
		ContentMarkdown: draft.contentMarkdown,
		Status:          draft.status,
		Stage:           draft.stage,
		RoundID:         draft.roundID,
		ParentEventID:   draft.parentEventID,
		StartedAt:       draft.startedAt,
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

func mergeTracePayload(dst *tracePayload, src *tracePayload) {
	if dst == nil || src == nil {
		return
	}
	if dst.TraceStage != nil {
		dst.Stages = append(dst.Stages, *dst.TraceStage)
		dst.TraceStage = nil
	}
	if src.FileMode != "" {
		dst.FileMode = src.FileMode
	}
	if src.FileNames != nil {
		dst.FileNames = append([]string(nil), src.FileNames...)
	}
	if src.FileRefs != nil {
		dst.FileRefs = append([]attachmentTraceFileRef(nil), src.FileRefs...)
	}
	if src.FileGroups != nil {
		dst.FileGroups = cloneTraceFileGroups(src.FileGroups)
	}
	if src.FileGroupRefs != nil {
		dst.FileGroupRefs = cloneTraceFileRefGroups(src.FileGroupRefs)
	}
	if src.Query != "" {
		dst.Query = src.Query
	}
	if src.HitChunkCount != 0 {
		dst.HitChunkCount = src.HitChunkCount
	}
	if src.CandidateCount != 0 {
		dst.CandidateCount = src.CandidateCount
	}
	if src.FilteredCount != 0 {
		dst.FilteredCount = src.FilteredCount
	}
	if src.MaxScore != 0 {
		dst.MaxScore = src.MaxScore
	}
	if src.Fallback != "" {
		dst.Fallback = src.Fallback
	}
	if src.Citations != nil {
		dst.Citations = append([]traceCitation(nil), src.Citations...)
	}
	if src.TraceStage != nil {
		stage := *src.TraceStage
		dst.Stages = append(dst.Stages, stage)
	}
	if src.Strategy != "" {
		dst.Strategy = src.Strategy
	}
	if src.FromTurn != 0 {
		dst.FromTurn = src.FromTurn
	}
	if src.ToTurn != 0 {
		dst.ToTurn = src.ToTurn
	}
	if src.SourceTokens != 0 {
		dst.SourceTokens = src.SourceTokens
	}
	if src.SummaryTokens != 0 {
		dst.SummaryTokens = src.SummaryTokens
	}
	if src.Error != "" {
		dst.Error = src.Error
	}
	if src.ToolID != 0 {
		dst.ToolID = src.ToolID
	}
	if src.ToolName != "" {
		dst.ToolName = src.ToolName
	}
	if src.SkillCount != 0 {
		dst.SkillCount = src.SkillCount
	}
	if src.SkillIDs != nil {
		dst.SkillIDs = append([]uint(nil), src.SkillIDs...)
	}
	if src.SkillTitles != nil {
		dst.SkillTitles = append([]string(nil), src.SkillTitles...)
	}
	if src.SkillTriggers != nil {
		dst.SkillTriggers = append([]string(nil), src.SkillTriggers...)
	}
	if src.Reason != "" {
		dst.Reason = src.Reason
	}
	if src.Status != "" {
		dst.Status = src.Status
	}
	if src.UpstreamDebug != nil {
		dst.UpstreamDebug = append(json.RawMessage(nil), src.UpstreamDebug...)
	}
	if src.PromptTrace != nil {
		prompt := *src.PromptTrace
		prompt.Blocks = make([]promptTraceBlockPayload, len(src.PromptTrace.Blocks))
		for index, block := range src.PromptTrace.Blocks {
			prompt.Blocks[index] = block
			prompt.Blocks[index].SourceRefs = append([]promptTraceSourcePayload(nil), block.SourceRefs...)
		}
		dst.PromptTrace = &prompt
	}
	if src.Reasoning != nil {
		if dst.Reasoning == nil {
			dst.Reasoning = &traceReasoning{}
		}
		mergeTraceReasoning(dst.Reasoning, src.Reasoning)
	}
	for _, stage := range src.Stages {
		if stage.Kind != "" {
			dst.Stages = append(dst.Stages, stage)
		}
	}
	for _, call := range src.ToolCalls {
		mergeTraceToolCall(dst, call)
	}
}

func mergeTraceReasoning(dst *traceReasoning, src *traceReasoning) {
	if dst == nil || src == nil {
		return
	}
	if src.Kind != "" {
		dst.Kind = src.Kind
	}
	if src.EventType != "" {
		dst.EventType = src.EventType
	}
	if src.ItemID != "" {
		dst.ItemID = src.ItemID
	}
	if src.Status != "" {
		dst.Status = src.Status
	}
	if src.Signature != "" {
		dst.Signature = src.Signature
	}
	if src.EncryptedContent != "" {
		dst.EncryptedContent = src.EncryptedContent
	}
}

func cloneTraceFileGroups(groups *attachmentTraceFileGroups) *attachmentTraceFileGroups {
	if groups == nil {
		return nil
	}
	clone := *groups
	clone.DirectImages = append([]string(nil), groups.DirectImages...)
	clone.Adaptive = append([]string(nil), groups.Adaptive...)
	clone.Retrieval = append([]string(nil), groups.Retrieval...)
	clone.FullContext = append([]string(nil), groups.FullContext...)
	clone.Skipped = append([]string(nil), groups.Skipped...)
	return &clone
}

func cloneTraceFileRefGroups(groups *attachmentTraceRefGroups) *attachmentTraceRefGroups {
	if groups == nil {
		return nil
	}
	clone := *groups
	clone.DirectImages = append([]attachmentTraceFileRef(nil), groups.DirectImages...)
	clone.Adaptive = append([]attachmentTraceFileRef(nil), groups.Adaptive...)
	clone.Retrieval = append([]attachmentTraceFileRef(nil), groups.Retrieval...)
	clone.FullContext = append([]attachmentTraceFileRef(nil), groups.FullContext...)
	clone.Skipped = append([]attachmentTraceFileRef(nil), groups.Skipped...)
	return &clone
}

func upsertProcessTraceStagePayload(dst *tracePayload, stage *traceStage) {
	if dst == nil || stage == nil || stage.Kind == "" {
		return
	}
	stageCopy := *stage
	dst.TraceStage = nil
	for index := range dst.Stages {
		if dst.Stages[index].Kind == stage.Kind {
			dst.Stages[index] = stageCopy
			if index+1 < len(dst.Stages) {
				stages := make([]traceStage, 0, len(dst.Stages))
				for currentIndex, current := range dst.Stages {
					if current.Kind != stage.Kind || currentIndex == index {
						stages = append(stages, current)
					}
				}
				dst.Stages = stages
			}
			return
		}
	}
	dst.Stages = append(dst.Stages, stageCopy)
}

func processTraceStageHasStatus(payload *tracePayload, kind string, status string) bool {
	if payload == nil {
		return false
	}
	for _, stage := range payload.Stages {
		if strings.TrimSpace(stage.Kind) == strings.TrimSpace(kind) && strings.TrimSpace(stage.Status) == strings.TrimSpace(status) {
			return true
		}
	}
	return false
}

func isToolTracePayload(payload *tracePayload) bool {
	return payload != nil && len(payload.ToolCalls) > 0
}

func mergeToolTracePayload(dst *tracePayload, src *tracePayload) {
	if dst == nil || src == nil {
		return
	}
	mergeTracePayload(dst, src)
}

func shouldMergeTraceToolCall(existing traceToolCall, incoming traceToolCall) bool {
	if existing.ToolCallID != "" && incoming.ToolCallID != "" {
		return existing.ToolCallID == incoming.ToolCallID
	}
	if existing.Name != "" && incoming.Name != "" && existing.Name != incoming.Name {
		return false
	}
	if existing.Type != "" && incoming.Type != "" && existing.Type != incoming.Type {
		return false
	}
	existingActive := isActiveTraceToolStatus(existing.Status)
	incomingActive := isActiveTraceToolStatus(incoming.Status)
	if existing.ToolCallID == "" && incoming.ToolCallID != "" && existingActive && existing.InputPreview == "" {
		return true
	}
	if incoming.ToolCallID == "" && existing.ToolCallID != "" && incomingActive && incoming.InputPreview == "" {
		return true
	}
	if existing.InputPreview == "" || incoming.InputPreview == "" {
		return existingActive || incomingActive
	}
	if existing.InputPreview != incoming.InputPreview {
		return false
	}
	return existingActive || incomingActive
}

func isActiveTraceToolStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "requested", "streaming", "in_progress", "queued", "searching":
		return true
	default:
		return false
	}
}

func mergeTraceToolCall(dst *tracePayload, incoming traceToolCall) {
	for index := range dst.ToolCalls {
		if !shouldMergeTraceToolCall(dst.ToolCalls[index], incoming) {
			continue
		}
		current := &dst.ToolCalls[index]
		if traceToolStatusRank(incoming.Status) >= traceToolStatusRank(current.Status) {
			current.Status = incoming.Status
		}
		if incoming.ToolCallID != "" {
			current.ToolCallID = incoming.ToolCallID
		}
		if incoming.Name != "" {
			current.Name = incoming.Name
		}
		if incoming.Type != "" {
			current.Type = incoming.Type
		}
		if incoming.LatencyMS != 0 {
			current.LatencyMS = incoming.LatencyMS
		}
		if incoming.Error != "" {
			current.Error = incoming.Error
		}
		if incoming.InputPreview != "" {
			current.InputPreview = incoming.InputPreview
		}
		if incoming.InputDetail != "" {
			current.InputDetail = incoming.InputDetail
		}
		if incoming.InputSize != 0 {
			current.InputSize = incoming.InputSize
		}
		if incoming.OutputPreview != "" {
			current.OutputPreview = incoming.OutputPreview
		}
		if incoming.OutputDetail != "" {
			current.OutputDetail = incoming.OutputDetail
		}
		if incoming.DetailRunID != "" {
			current.DetailRunID = incoming.DetailRunID
		}
		if incoming.OutputPresentation != nil {
			current.OutputPresentation = incoming.OutputPresentation
		}
		return
	}
	dst.ToolCalls = append(dst.ToolCalls, incoming)
}

func toolTracePayloadStatus(payload *tracePayload) string {
	if payload == nil {
		return ""
	}
	calls := payload.ToolCalls
	if len(calls) == 0 {
		return ""
	}
	hasError := false
	for _, call := range calls {
		switch strings.TrimSpace(call.Status) {
		case "error", "failed":
			hasError = true
		case "success", "completed", "reused", "":
		default:
			return messageTraceStatusStreaming
		}
	}
	if hasError {
		return messageTraceStatusError
	}
	return messageTraceStatusCompleted
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

func renderToolTraceMarkdownFromPayload(payload *tracePayload) string {
	summary, markdown, _ := buildToolTrace(toolTraceRowsFromPayload(payload))
	_ = summary
	return markdown
}

func toolTraceRowsFromPayload(payload *tracePayload) []model.ToolCall {
	if payload == nil {
		return nil
	}
	items := payload.ToolCalls
	rows := make([]model.ToolCall, 0, len(items))
	for _, item := range items {
		rows = append(rows, model.ToolCall{
			ToolCallID: item.ToolCallID, ToolType: item.Type, ToolName: item.Name,
			Status: item.Status, LatencyMS: item.LatencyMS, InputJSON: item.InputPreview,
			OutputJSON: item.OutputPreview, ErrorJSON: item.Error,
		})
	}
	return rows
}

func summarizeToolTracePayload(payload *tracePayload) string {
	if payload == nil {
		return ""
	}
	items := payload.ToolCalls
	if len(items) == 0 {
		return ""
	}
	errorCount := 0
	for _, item := range items {
		switch strings.TrimSpace(item.Status) {
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
	payloadTotal := 0
	if draft.payload != nil {
		payloadTotal = len(draft.payload.ToolCalls)
	}
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

func liveUpstreamReasoningPayload(kind string, payload *tracePayload) *traceReasoning {
	reasoning := &traceReasoning{Kind: strings.TrimSpace(kind)}
	if payload != nil && payload.Reasoning != nil {
		reasoning.EventType = payload.Reasoning.EventType
		reasoning.ItemID = payload.Reasoning.ItemID
		reasoning.Status = payload.Reasoning.Status
	}
	return trimTraceReasoning(reasoning)
}

func mergeUpstreamReasoningPayload(draft *messageTraceDraft, kind string, payload *tracePayload) {
	if draft == nil {
		return
	}
	if draft.payload == nil {
		draft.payload = &tracePayload{}
	}
	reasoning := liveUpstreamReasoningPayload(kind, payload)
	if reasoning == nil {
		return
	}
	if draft.payload.Reasoning != nil {
		if reasoning.EventType == "" {
			reasoning.EventType = draft.payload.Reasoning.EventType
		}
		if reasoning.ItemID == "" {
			reasoning.ItemID = draft.payload.Reasoning.ItemID
		}
		if reasoning.Status == "" {
			reasoning.Status = draft.payload.Reasoning.Status
		}
	}
	draft.payload.Reasoning = reasoning
}

func summarizeThinkText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return textutil.CompactSnippet(trimmed, 80)
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
	FileMode      string                     `json:"file_mode"`
	FileNames     []string                   `json:"file_names"`
	FileRefs      []attachmentTraceFileRef   `json:"file_refs"`
	FileGroups    *attachmentTraceFileGroups `json:"file_groups"`
	FileGroupRefs *attachmentTraceRefGroups  `json:"file_group_refs"`
}

func buildAttachmentProcessTrace(
	fileMode string,
	attachments []AttachmentInput,
) (string, string, *tracePayload) {
	if len(attachments) == 0 {
		return "", "", nil
	}

	payload := attachmentTracePayload{
		FileMode:      strings.TrimSpace(fileMode),
		FileNames:     make([]string, 0, len(attachments)),
		FileRefs:      make([]attachmentTraceFileRef, 0, len(attachments)),
		FileGroups:    &attachmentTraceFileGroups{},
		FileGroupRefs: &attachmentTraceRefGroups{},
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

func attachmentTracePayloadMap(payload attachmentTracePayload) *tracePayload {
	includedCount := len(payload.FileRefs) - len(payload.FileGroupRefs.Skipped)
	if includedCount < 0 {
		includedCount = 0
	}
	stage := traceStage{Kind: processTraceKindFileContext, Status: processTraceStatusReady, IncludedCount: includedCount, SkippedCount: len(payload.FileGroupRefs.Skipped)}
	return &tracePayload{
		FileMode: payload.FileMode, FileNames: payload.FileNames, FileRefs: payload.FileRefs,
		FileGroups: payload.FileGroups, FileGroupRefs: payload.FileGroupRefs,
		TraceStage: &stage,
	}
}

func buildRAGProcessTrace(
	query string,
	fileObjs []model.FileObject,
	chunks []model.RAGChunk,
) (string, string, *tracePayload) {
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
	citations := make([]traceCitation, 0, len(chunks))
	for _, chunk := range chunks {
		citations = append(citations, traceCitation{FileName: chunk.FileName, FileID: chunk.FileID, ChunkIndex: chunk.ChunkIndex, Score: chunk.Score, Preview: textutil.CompactSnippet(chunk.Content, 100)})
	}
	detail := fmt.Sprintf("检索已完成，共检索 %d 个文件，命中 %d 个段落。", len(names), len(chunks))
	stage := traceStage{Kind: processTraceKindRetrieval, Status: processTraceStatusCompleted, FileCount: len(names), ChunkCount: len(chunks)}
	return fmt.Sprintf("检索到 %d 段相关内容", len(chunks)), formatTraceStep("内容检索", detail), &tracePayload{
		Query: textutil.CompactSnippet(query, 240), FileNames: names, HitChunkCount: len(chunks), Citations: citations,
		TraceStage: &stage,
	}
}

func buildToolTrace(rows []model.ToolCall) (string, string, *tracePayload) {
	if len(rows) == 0 {
		return "", "", nil
	}
	toolCalls := make([]traceToolCall, 0, len(rows))
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
		errorText := strings.TrimSpace(row.ErrorJSON)
		inputDisplay := collapseWhitespace(input)
		inputPreview := textutil.CompactSnippet(inputDisplay, toolTraceCompactSummaryMaxChars)
		outputPresentation := toolresult.BuildPresentation(output)
		outputPreview := toolOutputPreview(output, outputPresentation)
		inputDetail := toolTraceDetail(input, toolTraceDetailMaxChars)
		outputDetail := toolTraceDetail(output, toolTraceDetailMaxChars)
		errorDetail := toolTraceDetail(errorText, toolTraceDetailMaxChars)
		if errorText != "" {
			parts = append(parts, textutil.CompactSnippet(collapseWhitespace(errorText), toolTraceCompactSummaryMaxChars))
		} else if outputPreview != "" {
			parts = append(parts, "结果："+textutil.CompactSnippet(outputPreview, toolTraceCompactSummaryMaxChars))
		}
		lines = append(lines, formatTraceStep(toolName, joinTraceParts(parts...)))
		toolCallID := strings.TrimSpace(row.ToolCallID)
		toolCall := traceToolCall{ToolCallID: toolCallID, Name: toolName, Type: strings.TrimSpace(row.ToolType), Status: status, LatencyMS: row.LatencyMS, Error: errorDetail, InputPreview: inputPreview, InputDetail: inputDetail, InputSize: len(input), OutputPreview: outputPreview, OutputDetail: outputDetail}
		if detailRunID := strings.TrimSpace(row.RunID); detailRunID != "" {
			toolCall.DetailRunID = detailRunID
		}
		if outputPresentation != nil {
			toolCall.OutputPresentation = outputPresentation
		}
		toolCalls = append(toolCalls, toolCall)
	}
	summary := fmt.Sprintf("%d 次工具调用已完成", len(rows))
	if requestedCount > 0 && successCount == 0 && errorCount == 0 {
		summary = fmt.Sprintf("%d 次工具调用进行中", len(rows))
	} else if errorCount > 0 {
		summary = fmt.Sprintf("%d 次工具调用，%d 次失败", len(rows), errorCount)
	} else if successCount == len(rows) {
		summary = fmt.Sprintf("%d 次工具调用已完成", len(rows))
	}
	return summary, strings.Join(lines, "\n"), &tracePayload{ToolCalls: toolCalls}
}

func toolOutputPreview(raw string, presentation *toolresult.Presentation) string {
	if presentation != nil && strings.TrimSpace(presentation.Text) != "" {
		return toolresult.Snippet(presentation.Text, toolTraceLegacyOutputPreviewMaxChars)
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	var payload any
	if err := json.Unmarshal([]byte(value), &payload); err == nil {
		if text := readableMCPToolResultPreview(payload); text != "" {
			return toolresult.Snippet(text, toolTraceLegacyOutputPreviewMaxChars)
		}
		if text := toolresult.ReadablePreview(payload); text != "" {
			return toolresult.Snippet(text, toolTraceLegacyOutputPreviewMaxChars)
		}
		if normalized, marshalErr := json.Marshal(payload); marshalErr == nil {
			value = string(normalized)
		}
	}
	return toolresult.Snippet(value, toolTraceLegacyOutputPreviewMaxChars)
}

func toolTraceDetail(raw string, maxChars int) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if maxChars <= 0 {
		maxChars = toolTraceDetailMaxChars
	}
	if len(runes) <= maxChars {
		return value
	}
	return toolresult.Snippet(value, maxChars)
}

func readableMCPToolResultPreview(value any) string {
	payload, ok := value.(map[string]any)
	if !ok || !looksLikeMCPToolResult(payload) {
		return ""
	}

	parts := make([]string, 0, 4)
	if text := readableMCPContentPreview(payload["content"]); text != "" {
		parts = append(parts, text)
	}
	if text := toolresult.ReadablePreview(payload["structuredContent"]); text != "" {
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		if summary := summarizeMCPContent(payload["content"]); summary != "" {
			parts = append(parts, summary)
		}
	}
	return strings.Join(parts, "；")
}

func looksLikeMCPToolResult(payload map[string]any) bool {
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

func readableMCPContentPreview(value any) string {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, min(len(items), 3))
	for _, item := range items {
		block, ok := item.(map[string]any)
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

func readableMCPTextBlock(block map[string]any) string {
	text := stringFromJSONValue(block["text"])
	if text == "" {
		return ""
	}
	var parsed any
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		if preview := toolresult.ReadablePreview(parsed); preview != "" {
			return preview
		}
	}
	return text
}

func summarizeMCPContent(value any) string {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, item := range items {
		block, ok := item.(map[string]any)
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

func stringFromJSONValue(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func collapseWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func buildCompactionProcessTrace(snapshot *model.ContextSnapshot) (string, string, *tracePayload) {
	if snapshot == nil {
		return "", "", nil
	}
	detail := strings.Join([]string{
		"对话已压缩并生成滚动摘要。",
		fmt.Sprintf("- 压缩区间：第 %d-%d 轮。", snapshot.FromTurn, snapshot.ToTurn),
		fmt.Sprintf("- Tokens 缩减：%d → %d。", snapshot.SourceTokens, snapshot.SummaryTokens),
	}, "\n")
	stage := traceStage{Kind: processTraceKindCompaction, Status: processTraceStatusCompleted, FromTurn: snapshot.FromTurn, ToTurn: snapshot.ToTurn, SourceTokens: snapshot.SourceTokens, SummaryTokens: snapshot.SummaryTokens}
	return fmt.Sprintf("已压缩第 %d-%d 轮上下文", snapshot.FromTurn, snapshot.ToTurn), formatTraceStep("上下文压缩", detail), &tracePayload{
		Strategy: snapshot.Strategy, FromTurn: snapshot.FromTurn, ToTurn: snapshot.ToTurn,
		SourceTokens: snapshot.SourceTokens, SummaryTokens: snapshot.SummaryTokens,
		TraceStage: &stage,
	}
}

func buildPendingCompactionProcessTrace() (string, *tracePayload) {
	stage := traceStage{Kind: processTraceKindCompaction, Status: processTraceStatusPending}
	return "正在压缩上下文", &tracePayload{TraceStage: &stage}
}

func buildFailedCompactionProcessTrace() (string, *tracePayload) {
	stage := traceStage{Kind: processTraceKindCompaction, Status: processTraceStatusFailed}
	return "上下文压缩未完成", &tracePayload{TraceStage: &stage}
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
