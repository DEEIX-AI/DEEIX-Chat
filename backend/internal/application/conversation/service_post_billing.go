package conversation

import (
	"context"
	"time"

	appcompact "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/compact"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"go.uber.org/zap"
)

// postBillingCompactionTask 保存主调用结算后执行上下文压缩所需的运行信息。
type postBillingCompactionTask struct {
	Async          bool
	Input          appcompact.MaybeCompactConversationInput
	ConversationID uint
	UserID         uint
	MessageID      uint
	RunID          string
	PreserveTurns  int
	OnEvent        func(eventType string, payload map[string]interface{}) error
	TraceRecorder  *messageTraceRecorder
}

// runPostBillingCompaction 在独立超时内执行后置压缩任务。
func (s *Service) runPostBillingCompaction(task *postBillingCompactionTask, message *model.Message) {
	if task == nil {
		return
	}
	if s.compactSvc == nil {
		s.completePostBillingCompactionTrace(task, message)
		return
	}
	run := func(ctx context.Context) {
		ctx = withBasicServiceBillingContext(ctx, task.UserID, task.ConversationID)
		snapshot, err := s.compactSvc.MaybeCompactConversation(ctx, task.Input)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("post_billing_compaction_failed",
					zap.Uint("user_id", task.UserID),
					zap.Uint("conversation_id", task.ConversationID),
					zap.Error(err),
				)
			}
			s.completePostBillingCompactionTrace(task, message)
			return
		}
		if snapshot != nil {
			s.invalidateSnapshotCache(task.ConversationID)
			_ = s.repo.UpdateConversationLastResponseID(ctx, task.ConversationID, "")
			s.persistSnapshotContextArtifact(ctx, snapshotContextArtifactInput{
				ConversationID: task.ConversationID,
				UserID:         task.UserID,
				MessageID:      task.MessageID,
				RunID:          task.RunID,
				Snapshot:       snapshot,
			})
			if task.TraceRecorder != nil {
				summary, markdown, payload := buildCompactionProcessTrace(snapshot)
				task.TraceRecorder.appendProcessSection(summary, markdown, payload, messageTraceStatusStreaming)
			}
			if task.OnEvent != nil {
				preview := []rune(snapshot.SummaryText)
				if len(preview) > 80 {
					preview = preview[:80]
				}
				emitEvent(task.OnEvent, "compact_done", map[string]interface{}{
					"method":          snapshot.Strategy,
					"freed_tokens":    snapshot.SourceTokens - snapshot.SummaryTokens,
					"kept_turns":      task.PreserveTurns,
					"summary_preview": string(preview),
				})
			}
		}
		s.completePostBillingCompactionTrace(task, message)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	run(ctx)
}

// runPostBillingTasks 按配置串行或异步执行压缩，并在其后安排会话元数据。
func (s *Service) runPostBillingTasks(input SendMessageBillingInput) {
	if input.Result == nil {
		return
	}
	task := input.Result.postBillingCompaction
	input.Result.postBillingCompaction = nil
	if task != nil && task.Async {
		// 异步任务只读取请求完成时的快照，不能继续持有 handler 正在返回的结果对象。
		result := *input.Result
		input.Result = &result
		go func() {
			s.runPostBillingCompaction(task, &result.AssistantMessage)
			s.scheduleConversationMetadataAfterBilling(input)
		}()
		return
	}
	s.runPostBillingCompaction(task, &input.Result.AssistantMessage)
	s.scheduleConversationMetadataAfterBilling(input)
}

// discardPostBillingCompaction 在主账单失败时终止尚未开始的压缩 trace。
func (s *Service) discardPostBillingCompaction(result *SendMessageResult) {
	if result == nil || result.postBillingCompaction == nil {
		return
	}
	task := result.postBillingCompaction
	result.postBillingCompaction = nil
	s.completePostBillingCompactionTrace(task, &result.AssistantMessage)
}

// completePostBillingCompactionTrace 将同步压缩 trace 回填到响应消息。
func (s *Service) completePostBillingCompactionTrace(task *postBillingCompactionTask, message *model.Message) {
	if task == nil || task.TraceRecorder == nil {
		return
	}
	task.TraceRecorder.complete()
	task.TraceRecorder.attachToMessage(message)
}
