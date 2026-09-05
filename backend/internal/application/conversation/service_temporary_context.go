package conversation

import (
	"context"
	"strings"
	"time"

	apprag "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/rag"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func (s *Service) prepareTemporaryKnowledgeContext(
	ctx context.Context,
	input TemporaryChatInput,
	traceRecorder *messageTraceRecorder,
) (userContextInput, []model.MessageKnowledgeSource, error) {
	if len(input.KnowledgeBaseIDs) == 0 {
		return userContextInput{}, nil, nil
	}
	cfg := s.cfg.Snapshot()
	if !cfg.KnowledgeBaseEnabled {
		// 知识库功能已被后台关闭：静默忽略引用，临时聊天继续按无知识库处理。
		return userContextInput{}, nil, nil
	}
	if s.embeddingSvc == nil || s.ragSvc == nil || s.knowledgeBaseResolver == nil {
		return userContextInput{}, nil, ErrKnowledgeBaseUnavailable
	}

	capability := s.resolveChatFileCapability(ctx)
	files, err := s.resolveKnowledgeBaseRAGFiles(
		ctx,
		input.UserID,
		input.KnowledgeBaseIDs,
		cfg.RAGEnabled && cfg.EmbeddingEnabled && capability.RAGAvailable,
	)
	if err != nil {
		return userContextInput{}, nil, err
	}

	domainMessages := make([]model.Message, 0, len(input.Messages))
	for _, item := range input.Messages {
		domainMessages = append(domainMessages, model.Message{Role: item.Role, Content: item.Content})
	}
	query := buildRAGQuery(domainMessages, input.Messages[len(input.Messages)-1].Content, cfg.RAGQueryHistoryTurns)
	emitEvent(input.OnEvent, "rag_search", map[string]any{
		"message": "正在检索相关内容…",
	})

	ragCtx := ctx
	cancel := func() {}
	if cfg.RAGWaitReadyMS > 0 {
		ragCtx, cancel = context.WithTimeout(ctx, time.Duration(cfg.RAGWaitReadyMS)*time.Millisecond)
	}
	result, retrieveErr := s.ragSvc.RetrieveWithStatus(ragCtx, apprag.RetrieveInput{
		UserID:    input.UserID,
		Query:     query,
		FileObjs:  files,
		Ephemeral: true,
	})
	cancel()
	if retrieveErr != nil || result.Status == apprag.RetrieveStatusUnavailable {
		return userContextInput{}, nil, ErrKnowledgeBaseUnavailable
	}

	chunks := NewContextAssembler(0).DeduplicateRAGChunks(result.Chunks)
	userContext := userContextInput{RAGChunks: chunks}
	if len(chunks) == 0 {
		userContext.RAGNotice = knowledgeBaseNoEvidenceNotice
		if traceRecorder != nil {
			traceRecorder.appendProcessSection(
				"知识库未检索到相关内容",
				formatTraceStep("内容检索", "已检索所选知识库，但没有足够相关的片段。"),
				&tracePayload{Query: strings.TrimSpace(query), Status: strings.TrimSpace(string(result.Status))},
				messageTraceStatusCompleted,
			)
		}
	} else if traceRecorder != nil {
		summary, markdown, payload := buildRAGProcessTrace(query, files, chunks)
		traceRecorder.appendProcessSection(summary, markdown, payload, messageTraceStatusCompleted)
	}

	return userContext, messageKnowledgeSourcesFromRAGChunks(chunks), nil
}
