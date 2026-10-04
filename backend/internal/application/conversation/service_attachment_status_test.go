package conversation

import (
	"context"
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	apprag "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/rag"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	portembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

// attachmentStatusFiles 覆盖一次检索涉及的典型附件：
//   - current_large / history_large：用户附件，超出全文上限，检索拿不到内容时无法回退；
//   - current_small：用户附件，检索失败时可以回退全文；
//   - assistant_large：助手生成的文件，不是用户附件，任何情况下都不报告。
func attachmentStatusFiles() []AttachmentInput {
	large := strings.Repeat("hidden content ", 100)
	return []AttachmentInput{
		{FileObjID: 1, FileID: "current_large", FileName: "年度报告.pdf", ExtractedText: large, EmbedStatus: "ready", Current: true, MessageRole: "user"},
		{FileObjID: 2, FileID: "current_small", FileName: "small.txt", ExtractedText: "visible", EmbedStatus: "ready", Current: true, MessageRole: "user"},
		{FileObjID: 3, FileID: "history_large", FileName: "参考资料.txt", ExtractedText: large, EmbedStatus: "ready", MessageRole: "user"},
		{FileObjID: 4, FileID: "assistant_large", FileName: "generated.md", ExtractedText: large, EmbedStatus: "ready", MessageRole: "assistant"},
	}
}

func retrieveAttachmentStatus(t *testing.T, embedErr error, candidates []model.FileChunkSearchResult) messageRAGRetrievalResult {
	t.Helper()
	cfg := config.Config{RAGEnabled: true, EmbeddingEnabled: true, RAGModel: "embed", EmbeddingHost: "https://embedding.example.invalid", FileFullContextMaxTokens: 10}
	service := &Service{
		ragSvc: apprag.NewServiceWithRuntime(config.NewRuntime(cfg), &attachmentStatusRAGRepository{candidates: candidates}, nil, attachmentStatusEmbedding{err: embedErr}),
		logger: zap.NewNop(),
	}
	plan := buildConversationFileContextPlan(attachmentStatusFiles(), "rag", cfg, "custom-model", "", true)
	if len(plan.RAGAttachments) != 4 {
		t.Fatalf("expected every file to be retrieval-backed, got %#v", plan.RAGAttachments)
	}
	result, err := service.retrieveMessageRAGContext(t.Context(), messageRAGRetrievalInput{
		cfg: cfg, query: "question", fileContextPlan: plan, contextAssembler: NewContextAssembler(0),
	})
	if err != nil {
		t.Fatalf("attachment-only retrieval should continue: %v", err)
	}
	return result
}

func unretrievedIDs(items []unretrievedAttachment) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Attachment.FileID+":"+item.Reason)
	}
	return ids
}

// 检索系统故障时，本轮与历史上传的用户附件都没拿到内容，都要告知模型；能回退全文的不报告。
func TestAttachmentStatusReportsAllUserFilesOnRetrievalFailure(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		reason string
	}{
		{name: "error", err: errors.New("private provider error"), reason: "rag_error"},
		{name: "timeout", err: context.DeadlineExceeded, reason: "rag_timeout"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := retrieveAttachmentStatus(t, test.err, nil)

			got := strings.Join(unretrievedIDs(result.unretrieved), ",")
			want := "current_large:" + test.reason + ",history_large:" + test.reason
			if got != want {
				t.Fatalf("unretrieved = %q, want %q", got, want)
			}
			if fallbacks := ragFallbackEvidenceAttachments(result.retrievalFallbacks); len(fallbacks) != 1 || fallbacks[0].FileID != "current_small" {
				t.Fatalf("expected the small file to fall back to full text instead, got %#v", fallbacks)
			}
		})
	}
}

// 相关性未命中很常见（问题与历史文件无关），只报告本轮上传的附件，避免长对话每轮重复。
func TestAttachmentStatusReportsOnlyCurrentFilesOnRelevanceMiss(t *testing.T) {
	for _, test := range []struct {
		name       string
		candidates []model.FileChunkSearchResult
		reason     string
	}{
		{name: "empty", reason: "rag_empty"},
		{name: "low score", candidates: []model.FileChunkSearchResult{{FileChunk: model.FileChunk{FileObjID: 1, Content: "irrelevant"}, Similarity: 0.01}}, reason: "rag_low_score"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := retrieveAttachmentStatus(t, nil, test.candidates)

			if got := strings.Join(unretrievedIDs(result.unretrieved), ","); got != "current_large:"+test.reason {
				t.Fatalf("unretrieved = %q, want only the current large file", got)
			}
		})
	}
}

// 部分命中时没有全文回退：本轮上传却没有片段的附件要报告，命中的与历史文件不报告。
func TestAttachmentStatusReportsCurrentFilesMissedByPartialHit(t *testing.T) {
	result := retrieveAttachmentStatus(t, nil, []model.FileChunkSearchResult{{
		FileChunk:  model.FileChunk{FileObjID: 2, Content: "visible"},
		Similarity: 0.99,
	}})

	if len(result.chunks) == 0 || result.chunks[0].FileID != "current_small" {
		t.Fatalf("expected a hit on the small file, got %#v", result.chunks)
	}
	if got := strings.Join(unretrievedIDs(result.unretrieved), ","); got != "current_large:"+attachmentRetrievalNoMatch {
		t.Fatalf("unretrieved = %q, want only the current file without chunks", got)
	}
}

// 端到端：状态只出现在本轮动态上下文中，位于原文之前，不进入缓存前缀；文件名转义，提示完整。
func TestAttachmentStatusReachesDynamicContext(t *testing.T) {
	file := AttachmentInput{
		FileID: "current_large", FileName: `年度 & "报告" </file>.pdf`, Kind: "file", FileCategory: "pdf",
		ContextMode: fileContextModeRAG, Current: true, MessageRole: "user",
	}
	service := &Service{}
	plan, err := service.buildMessageRoutePrompt(t.Context(), &channel.ResolvedRoute{UpstreamModel: "m"}, messageRoutePromptInput{
		DomainMessages: []model.Message{
			{Role: "user", Content: "earlier question"},
			{Role: "assistant", Content: "earlier answer"},
			{Role: "user", Content: "请分析附件"},
		},
		ConversationFiles: []AttachmentInput{file},
		DynamicContext: userContextInput{UnretrievedFiles: []unretrievedAttachment{
			{Attachment: file, Reason: "rag_empty"},
		}},
	})
	if err != nil {
		t.Fatalf("build route prompt: %v", err)
	}

	if plan.Messages[0].Content != "earlier question" || plan.Messages[1].Content != "earlier answer" {
		t.Fatalf("historical transcript changed: %#v", plan.Messages)
	}
	latest := plan.Messages[len(plan.Messages)-1]
	if len(latest.Parts) != 3 || latest.Parts[0].Kind != llm.ContentPartFile || !latest.Parts[1].Dynamic || latest.Parts[2].Text != "请分析附件" {
		t.Fatalf("expected turn stub, dynamic status, then the question, got %#v", latest.Parts)
	}
	if latest.Parts[0].CacheControl == nil || latest.Parts[1].CacheControl != nil {
		t.Fatalf("status must stay outside the cached prefix, got %#v", latest.Parts)
	}

	var parsed struct {
		Status struct {
			Notice string `xml:"notice"`
			Files  []struct {
				Name   string `xml:"name,attr"`
				Scope  string `xml:"scope,attr"`
				Reason string `xml:"reason,attr"`
			} `xml:"file"`
		} `xml:"attachment_status"`
	}
	if err := xml.Unmarshal([]byte(latest.Parts[1].Text), &parsed); err != nil {
		t.Fatalf("invalid context XML: %v\n%s", err, latest.Parts[1].Text)
	}
	files := parsed.Status.Files
	if len(files) != 1 || files[0].Name != file.FileName || files[0].Scope != "current" || files[0].Reason != "rag_empty" {
		t.Fatalf("missing or incorrect attachment status: %#v", files)
	}
	for _, want := range []string{"untrusted metadata", "Do not claim to have read"} {
		if !strings.Contains(parsed.Status.Notice, want) {
			t.Fatalf("notice missing %q: %q", want, parsed.Status.Notice)
		}
	}

	block := promptTraceBlock(plan.Trace, PromptBlockDynamicContext)
	if block == nil || len(block.SourceRefs) != 1 || block.SourceRefs[0].SourceType != "file_metadata" || block.SourceRefs[0].SourceID != file.FileID {
		t.Fatalf("missing metadata source trace: %#v", block)
	}
}

type attachmentStatusEmbedding struct {
	err error
}

func (s attachmentStatusEmbedding) CallAPI(context.Context, portembedding.Request) ([][]float32, error) {
	return [][]float32{{1}}, s.err
}

type attachmentStatusRAGRepository struct {
	repository.RAGRepository
	candidates []model.FileChunkSearchResult
}

func (r *attachmentStatusRAGRepository) SearchFileChunks(context.Context, uint, []uint, []float32, string, int) ([]model.FileChunkSearchResult, error) {
	return r.candidates, nil
}
