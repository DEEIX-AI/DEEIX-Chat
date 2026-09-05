package conversation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apprag "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/rag"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	ragFallbackErrorCanceled    = "内容检索已取消"
	ragFallbackErrorTimeout     = "内容检索超时"
	ragFallbackErrorUnavailable = "内容检索不可用"
	ragFallbackErrorFailed      = "内容检索失败"
)

func ragFallbackErrorMessage(status apprag.RetrieveStatus, err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return ragFallbackErrorCanceled
	case status == apprag.RetrieveStatusTimeout || errors.Is(err, context.DeadlineExceeded):
		return ragFallbackErrorTimeout
	case status == apprag.RetrieveStatusUnavailable:
		return ragFallbackErrorUnavailable
	default:
		return ragFallbackErrorFailed
	}
}

// messageRAGRetrievalInput 描述一次消息发送的检索范围：本轮可检索附件与显式选择的知识库文件。
type messageRAGRetrievalInput struct {
	input              SendMessageInput
	cfg                config.Config
	query              string
	fileContextPlan    conversationFileContextPlan
	knowledgeBaseFiles []model.FileObject
	contextAssembler   *ContextAssembler
	traceRecorder      *messageTraceRecorder
}

// messageRAGRetrievalResult 是检索阶段对提示词组装的全部贡献。
type messageRAGRetrievalResult struct {
	chunks []model.RAGChunk
	// fallbacks 是全部全文回退证据，含检索前就因 RAG 不可用而回退的附件；
	// retrievalFallbacks 只含检索失败或未命中后新增的回退，它们要作为附件注入稳定上下文。
	fallbacks          []ragFallbackEvidence
	retrievalFallbacks []ragFallbackEvidence
	// notice 在显式选择的知识库没有可用证据时提示模型不得声称答案有知识库依据。
	notice string
}

// retrieveMessageRAGContext 对本轮可检索附件与知识库文件执行语义检索，并把失败与未命中折叠为
// 全文回退证据。显式选择的知识库是硬性来源要求：检索失败或知识库不可用时直接终止发送，
// 而不是静默忽略用户配置的语料给出看似成功的回答；仅附件的请求仍可走有界的全文回退。
func (s *Service) retrieveMessageRAGContext(ctx context.Context, in messageRAGRetrievalInput) (messageRAGRetrievalResult, error) {
	cfg := in.cfg
	input := in.input
	fileContextPlan := in.fileContextPlan
	traceRecorder := in.traceRecorder
	result := messageRAGRetrievalResult{
		fallbacks: ragFallbackEvidencesFromAttachments(
			filterAttachmentsByContextMode(fileContextPlan.FullAttachments, fileContextModeRAGFallback),
			"rag_unavailable",
			"",
		),
		retrievalFallbacks: make([]ragFallbackEvidence, 0),
		chunks:             make([]model.RAGChunk, 0),
	}
	if !cfg.RAGEnabled || (len(fileContextPlan.RAGAttachments) == 0 && len(in.knowledgeBaseFiles) == 0) {
		return result, nil
	}

	readyObjs := mergeRAGFileObjects(fileContextPlanRAGObjects(fileContextPlan.RAGAttachments), in.knowledgeBaseFiles)
	knowledgeBaseFileIDs := make(map[string]struct{}, len(in.knowledgeBaseFiles))
	for _, file := range in.knowledgeBaseFiles {
		if fileID := strings.TrimSpace(file.FileID); fileID != "" {
			knowledgeBaseFileIDs[fileID] = struct{}{}
		}
	}
	emitEvent(input.OnEvent, "rag_search", map[string]any{
		"message": "正在检索相关内容…",
	})
	ragCtx, ragSpan := platformtracing.Start(ctx, "conversation.rag.retrieve",
		trace.WithAttributes(
			attribute.Int64("conversation.id", int64(input.ConversationID)),
			attribute.Int64("user.id", int64(input.UserID)),
			attribute.Int("conversation.rag.file_count", len(readyObjs)),
		),
	)
	ragCallCtx := ragCtx
	ragCancel := func() {}
	if cfg.RAGWaitReadyMS > 0 {
		ragCallCtx, ragCancel = context.WithTimeout(ragCtx, time.Duration(cfg.RAGWaitReadyMS)*time.Millisecond)
	}
	ragResult, ragErr := s.ragSvc.RetrieveWithStatus(ragCallCtx, apprag.RetrieveInput{
		UserID:   input.UserID,
		Query:    in.query,
		FileObjs: readyObjs,
	})
	ragCancel()
	platformtracing.RecordError(ragSpan, ragErr)
	ragSpan.SetAttributes(
		attribute.String("conversation.rag.status", string(ragResult.Status)),
		attribute.String("conversation.rag.reason", strings.TrimSpace(ragResult.Reason)),
		attribute.Int("conversation.rag.candidate_count", ragResult.CandidateCount),
		attribute.Int("conversation.rag.filtered_count", ragResult.FilteredCount),
		attribute.Float64("conversation.rag.max_score", float64(ragResult.MaxScore)),
		attribute.Bool("conversation.rag.cached", ragResult.Cached),
	)
	ragSpan.End()
	ragChunks := in.contextAssembler.DeduplicateRAGChunks(ragResult.Chunks)
	knowledgeBaseHit := false
	for _, chunk := range ragChunks {
		if _, ok := knowledgeBaseFileIDs[strings.TrimSpace(chunk.FileID)]; ok {
			knowledgeBaseHit = true
			break
		}
	}
	knowledgeBaseSelected := len(input.KnowledgeBaseIDs) > 0

	switch {
	case ragErr != nil:
		s.logger.Warn("rag_retrieval_failed",
			zap.String("trace_id", traceid.FromContext(ctx)),
			zap.Uint("user_id", input.UserID),
			zap.Error(ragErr),
		)
		fallbacks, skipped := splitRetrievalFallbackAttachments(fileContextPlan.RAGAttachments, cfg)
		fallbackLabel := "已改用全文"
		if len(fallbacks) == 0 {
			fallbackLabel = "没有可用全文"
		}
		fallbackReason := normalizeRAGFallbackReason(ragResult.Status, "rag_error")
		if traceRecorder != nil {
			traceRecorder.appendProcessSection(
				"内容检索未完成，"+fallbackLabel,
				formatTraceStep(
					"内容检索",
					fmt.Sprintf("文件已检索，检索未完成，%s。", fallbackLabel),
				),
				buildRAGFallbackProcessTracePayload(in.query, readyObjs, ragResult, fallbackReason, len(fallbacks) > 0, ragErr),
				messageTraceStatusStreaming,
			)
		}
		evidences := ragFallbackEvidencesFromAttachments(fallbacks, fallbackReason, ragFallbackErrorMessage(ragResult.Status, ragErr))
		result.fallbacks = append(result.fallbacks, evidences...)
		result.retrievalFallbacks = append(result.retrievalFallbacks, evidences...)
		appendRAGFallbackSkippedTrace(traceRecorder, skipped, fallbackReason)
		if knowledgeBaseSelected {
			return result, ErrKnowledgeBaseUnavailable
		}
	case knowledgeBaseSelected && ragResult.Status == apprag.RetrieveStatusUnavailable:
		return result, ErrKnowledgeBaseUnavailable
	case len(ragChunks) == 0:
		fallbacks, skipped := splitRetrievalFallbackAttachments(fileContextPlan.RAGAttachments, cfg)
		fallbackLabel := "已改用全文"
		if len(fallbacks) == 0 {
			fallbackLabel = "没有可用全文"
		}
		ragStatus := normalizeRAGFallbackReason(ragResult.Status, "rag_empty")
		missLabel := "未检索到相关片段"
		if ragResult.Status == apprag.RetrieveStatusLowScore {
			missLabel = "检索结果低于相似度阈值"
		}
		if traceRecorder != nil {
			traceRecorder.appendProcessSection(
				"未检索到相关片段，"+fallbackLabel,
				formatTraceStep("内容检索", fmt.Sprintf("文件已检索，%s，%s。", missLabel, fallbackLabel)),
				buildRAGFallbackProcessTracePayload(in.query, readyObjs, ragResult, ragStatus, len(fallbacks) > 0, nil),
				messageTraceStatusStreaming,
			)
		}
		evidences := ragFallbackEvidencesFromAttachments(fallbacks, ragStatus, "")
		result.fallbacks = append(result.fallbacks, evidences...)
		result.retrievalFallbacks = append(result.retrievalFallbacks, evidences...)
		appendRAGFallbackSkippedTrace(traceRecorder, skipped, ragStatus)
		if knowledgeBaseSelected {
			result.notice = knowledgeBaseNoEvidenceNotice
		}
	default:
		if traceRecorder != nil {
			summary, markdown, payload := buildRAGProcessTrace(in.query, readyObjs, ragChunks)
			traceRecorder.appendProcessSection(summary, markdown, payload, messageTraceStatusStreaming)
		}
		result.chunks = append(result.chunks, ragChunks...)
		if knowledgeBaseSelected && !knowledgeBaseHit {
			result.notice = knowledgeBaseNoEvidenceNotice
		}
	}
	return result, nil
}
