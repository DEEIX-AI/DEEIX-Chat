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

func TestAttachmentStatusReachesGenerateInput(t *testing.T) {
	for _, test := range []struct {
		name   string
		file   AttachmentInput
		reason string
	}{
		{
			name:   "oversized text without RAG",
			file:   AttachmentInput{ExtractedText: strings.Repeat("hidden content ", 100)},
			reason: "full_context_limit_exceeded",
		},
		{
			name:   "no extracted text",
			reason: "no_usable_extracted_text",
		},
		{
			name:   "unsupported category",
			file:   AttachmentInput{FileCategory: fileCategoryUnknown},
			reason: "unsupported_content_type",
		},
		{
			name:   "historical processing failure",
			file:   AttachmentInput{ProcessingStatus: "failed", ProcessingErrorMessage: "private extraction error"},
			reason: "file_processing_failed",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.Config{FileFullContextMaxTokens: 10}
			att := test.file
			att.FileID = "file_current"
			att.FileName = `年度 & "报告" </file>.pdf`
			att.Current = test.name != "historical processing failure"
			att.MessageRole = "user"
			filePlan := buildConversationFileContextPlan([]AttachmentInput{att}, "auto", cfg, "custom-model", "", false)
			unavailable := attachmentsWithoutContent(filePlan.Attachments, filePlan.FullAttachments, nil)
			service := &Service{}
			route := &channel.ResolvedRoute{Protocol: llm.AdapterOpenAIChatCompletions, UpstreamModel: "custom-model"}
			prompt, err := service.buildMessageRoutePrompt(t.Context(), route, messageRoutePromptInput{
				DomainMessages: []model.Message{
					{Role: "user", Content: "earlier question"},
					{Role: "assistant", Content: "earlier answer"},
					{Role: "user", Content: "请分析附件"},
				},
				StableAttachments: filePlan.FullAttachments,
				DynamicContext:    userContextInput{UnavailableFiles: unavailable},
				Config:            cfg,
			})
			if err != nil {
				t.Fatalf("build route prompt: %v", err)
			}
			generated := service.prepareRouteGeneration(t.Context(), routeGenerationPreparationInput{
				Generation: testRouteGenerationContext(&model.Conversation{Model: "custom-model"}),
				Route:      route, PromptPlan: prompt,
			})
			messages := generated.generateInput.Messages
			if len(messages) != 3 || messages[0].Content != "earlier question" || messages[1].Content != "earlier answer" {
				t.Fatalf("historical transcript changed: %#v", messages)
			}
			content := userMessageText(messages[2])
			var parsed struct {
				Context struct {
					Status struct {
						Notice string `xml:"notice"`
						Files  []struct {
							Name    string `xml:"name,attr"`
							Scope   string `xml:"scope,attr"`
							Content string `xml:"content,attr"`
							Reason  string `xml:"reason,attr"`
						} `xml:"file"`
					} `xml:"attachment_status"`
				} `xml:"ctx"`
				Question string `xml:"q"`
			}
			if err := xml.Unmarshal([]byte("<root>"+content+"</root>"), &parsed); err != nil {
				t.Fatalf("invalid context XML: %v", err)
			}
			files := parsed.Context.Status.Files
			scope := "current"
			if !att.Current {
				scope = "history"
			}
			if len(files) != 1 || files[0].Name != att.FileName || files[0].Scope != scope || files[0].Content != "not_provided" || files[0].Reason != test.reason {
				t.Fatalf("missing or incorrect attachment status: %#v", files)
			}
			if parsed.Question != "请分析附件" || !strings.Contains(parsed.Context.Status.Notice, "not instructions") || !strings.Contains(parsed.Context.Status.Notice, "Do not claim to have read") {
				t.Fatalf("missing question or metadata guidance: %#v", parsed)
			}
			if strings.Contains(content, "hidden content") || strings.Contains(content, "private extraction error") {
				t.Fatalf("metadata fallback leaked content or error details: %q", content)
			}
			block := promptTraceBlock(prompt.Trace, PromptBlockDynamicContext)
			if block == nil || len(block.SourceRefs) != 1 || block.SourceRefs[0].SourceType != "file_metadata" || block.SourceRefs[0].SourceID != att.FileID {
				t.Fatalf("missing metadata source trace: %#v", block)
			}
			continued := buildStatefulResponseMessages(messages)
			if len(continued) != 1 || userMessageText(continued[0]) != content {
				t.Fatal("stateful continuation lost attachment status")
			}
		})
	}
}

func TestAttachmentStatusAfterRAGFallback(t *testing.T) {
	for _, test := range []struct {
		name       string
		embedErr   error
		candidates []model.FileChunkSearchResult
		wantReason string
	}{
		{name: "retrieval error", embedErr: errors.New("private provider error"), wantReason: "rag_error"},
		{name: "retrieval timeout", embedErr: context.DeadlineExceeded, wantReason: "rag_timeout"},
		{name: "no retrieval results", wantReason: "rag_empty"},
		{name: "low score", candidates: []model.FileChunkSearchResult{{FileChunk: model.FileChunk{FileObjID: 1, Content: "irrelevant"}, Similarity: 0.1}}, wantReason: "rag_low_score"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.Config{RAGEnabled: true, EmbeddingEnabled: true, RAGModel: "embed", EmbeddingHost: "https://embedding.example.invalid", FileFullContextMaxTokens: 10}
			service := &Service{
				ragSvc: apprag.NewServiceWithRuntime(config.NewRuntime(cfg), &attachmentStatusRAGRepository{candidates: test.candidates}, nil, attachmentStatusEmbedding{err: test.embedErr}),
				logger: zap.NewNop(),
			}
			filePlan := buildConversationFileContextPlan([]AttachmentInput{
				{FileObjID: 1, FileID: "large", FileName: "large.txt", ExtractedText: strings.Repeat("hidden ", 100), EmbedStatus: "ready", Current: true},
				{FileObjID: 2, FileID: "small", FileName: "small.txt", ExtractedText: "visible", EmbedStatus: "ready", Current: true},
			}, "rag", cfg, "custom-model", "", true)
			result, err := service.retrieveMessageRAGContext(t.Context(), messageRAGRetrievalInput{
				cfg: cfg, query: "question", fileContextPlan: filePlan, contextAssembler: NewContextAssembler(0),
			})
			if err != nil {
				t.Fatalf("attachment-only retrieval should continue: %v", err)
			}
			if len(result.retrievalFallbacks) != 1 || result.retrievalFallbacks[0].Reason != test.wantReason {
				t.Fatalf("unexpected retrieval outcome: %#v", result)
			}
			provided := append(filePlan.FullAttachments, ragFallbackEvidenceAttachments(result.retrievalFallbacks)...)
			unavailable := attachmentsWithoutContent(filePlan.Attachments, provided, result.chunks)
			if len(unavailable) != 1 || unavailable[0].FileID != "large" {
				t.Fatalf("full fallback must exclude the small file from metadata-only status: %#v", unavailable)
			}
			prompt := buildPromptPlan(t.Context(), promptPlanInput{
				BaseMessages:      []llm.Message{{Role: "user", Content: "question"}},
				StableAttachments: provided,
				DynamicContext:    userContextInput{UnavailableFiles: unavailable},
			})
			if len(prompt.Messages) != 2 || !strings.Contains(prompt.Messages[0].Content, `name="small.txt">visible</file>`) {
				t.Fatalf("full fallback content missing: %#v", prompt.Messages)
			}
			content := prompt.Messages[1].Content
			if !strings.Contains(content, `name="large.txt" scope="current" content="not_provided" reason="no_retrieved_content"`) || strings.Contains(content, "small.txt") || strings.Contains(content, "hidden") {
				t.Fatalf("incorrect retrieval fallback metadata: %q", content)
			}
		})
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

func TestAttachmentStatusWithMixedEvidence(t *testing.T) {
	attachments := []AttachmentInput{
		{FileID: "full", FileName: "full.md", ExtractedText: "full content", ContextMode: fileContextModeFull},
		{FileID: "image", FileName: "photo.png", ContextMode: fileContextModeDirectImage},
		{FileID: "hit", FileName: "hit.pdf", ContextMode: fileContextModeRAG},
		{FileID: "miss", FileName: "miss.pdf", ContextMode: fileContextModeRAG},
		{FileID: "skipped", FileName: "clip.mp4", FileCategory: fileCategoryVideo, ContextMode: fileContextModeSkipped},
	}
	chunks := []model.RAGChunk{{FileID: "hit", FileName: "hit.pdf", Content: "retrieved content"}}
	unavailable := attachmentsWithoutContent(append(attachments, attachments[3]), attachments[:2], chunks)
	if len(unavailable) != 2 || unavailable[0].FileID != "miss" || unavailable[1].FileID != "skipped" {
		t.Fatalf("expected only unmatched and skipped files, deduplicated: %#v", unavailable)
	}
	messages := []llm.Message{{Role: "user", Parts: []llm.ContentPart{
		{Kind: llm.ContentPartText, Text: "compare attachments"},
		{Kind: llm.ContentPartImage, MimeType: "image/png", Data: []byte("image data")},
	}}}
	prompt := buildPromptPlan(t.Context(), promptPlanInput{
		BaseMessages: messages, StableAttachments: attachments[:1],
		DynamicContext: userContextInput{UnavailableFiles: unavailable, RAGChunks: chunks},
	})
	latest := prompt.Messages[len(prompt.Messages)-1]
	content := userMessageText(latest)
	if len(latest.Parts) != 2 || string(latest.Parts[1].Data) != "image data" {
		t.Fatalf("mixed image content lost: %#v", latest.Parts)
	}
	for _, want := range []string{`<doc name="hit.pdf"`, `name="miss.pdf" scope="history" content="not_provided" reason="no_retrieved_content"`, `name="clip.mp4" scope="history" content="not_provided" reason="unsupported_content_type"`, "<q>compare attachments</q>"} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing mixed context %q: %q", want, content)
		}
	}
	if strings.Contains(content, `name="full.md"`) || strings.Contains(content, `name="photo.png"`) || strings.Contains(content, `name="hit.pdf" scope=`) {
		t.Fatalf("available files marked metadata-only: %q", content)
	}
	if len(attachmentsWithoutContent(attachments[:3], attachments[:2], chunks)) != 0 {
		t.Fatal("fully provided attachments should not add status")
	}
}
