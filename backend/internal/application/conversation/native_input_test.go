package conversation

import (
	"bytes"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstorage"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestResolveNativeInputPolicy(t *testing.T) {
	pdfCatalog := []string{"text", "image", "pdf"}
	for _, test := range []struct {
		name      string
		route     channel.ResolvedRoute
		wantImage bool
		wantPDF   bool
	}{
		{name: "unknown model keeps sending images", route: channel.ResolvedRoute{Protocol: llm.AdapterAnthropicMessages}, wantImage: true},
		{name: "anthropic catalog pdf on any base url", route: channel.ResolvedRoute{Protocol: llm.AdapterAnthropicMessages, BaseURL: "https://relay.example.com", CatalogInputModalities: pdfCatalog}, wantImage: true, wantPDF: true},
		{name: "gemini catalog pdf", route: channel.ResolvedRoute{Protocol: llm.AdapterGoogleGenerateContent, CatalogInputModalities: pdfCatalog}, wantImage: true, wantPDF: true},
		{name: "official openai", route: channel.ResolvedRoute{Protocol: llm.AdapterOpenAIResponses, BaseURL: "https://api.openai.com/v1", CatalogInputModalities: pdfCatalog}, wantImage: true, wantPDF: true},
		{name: "openai-compatible relay needs explicit declaration", route: channel.ResolvedRoute{Protocol: llm.AdapterOpenAIChatCompletions, BaseURL: "https://relay.example.com/v1", CatalogInputModalities: pdfCatalog}, wantImage: true},
		{name: "openai-compatible relay with explicit declaration", route: channel.ResolvedRoute{Protocol: llm.AdapterOpenAIChatCompletions, BaseURL: "https://relay.example.com/v1", ModelCapabilitiesJSON: `{"inputModalities":["image","pdf"]}`}, wantImage: true, wantPDF: true},
		{name: "official openrouter", route: channel.ResolvedRoute{Protocol: llm.AdapterOpenRouterChat, BaseURL: "https://openrouter.ai/api/v1", CatalogInputModalities: pdfCatalog}, wantImage: true, wantPDF: true},
		{name: "protocol without document blocks", route: channel.ResolvedRoute{Protocol: llm.AdapterXAIResponses, BaseURL: "https://api.x.ai/v1", CatalogInputModalities: pdfCatalog}, wantImage: true},
		{name: "text-only catalog model", route: channel.ResolvedRoute{Protocol: llm.AdapterOpenAIChatCompletions, CatalogInputModalities: []string{"text"}}},
		{name: "explicit declaration overrides catalog", route: channel.ResolvedRoute{Protocol: llm.AdapterAnthropicMessages, CatalogInputModalities: pdfCatalog, ModelCapabilitiesJSON: `{"inputModalities":["image"]}`}, wantImage: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			route := test.route
			got := resolveNativeInputPolicy(&route)
			if got.Image != test.wantImage || got.PDF != test.wantPDF {
				t.Fatalf("policy = %+v, want image=%v pdf=%v", got, test.wantImage, test.wantPDF)
			}
		})
	}
}

func nativePDF(fileID string, size int64, pages int, current bool) AttachmentInput {
	return AttachmentInput{
		FileID: fileID, FileName: fileID + ".pdf", FileCategory: fileCategoryPDF, MimeType: "application/pdf",
		StoragePath: "files/" + fileID, FileSize: size, PageCount: pages, ExtractedText: "提取文本",
		ContextMode: fileContextModeFull, Current: current, MessageRole: "user",
	}
}

// 预算内越新的文件越优先；走检索、助手生成、过大的文件不原生发送；扫描件（无提取文本）可以原生发送。
func TestSelectNativeDocumentsPrefersNewestWithinBudget(t *testing.T) {
	retrieval := nativePDF("retrieval", 1024, 2, false)
	retrieval.ContextMode = fileContextModeRAG
	assistant := nativePDF("assistant", 1024, 2, false)
	assistant.MessageRole = "assistant"
	scanned := nativePDF("scanned", 1024, 3, false)
	scanned.ContextMode, scanned.ExtractedText = fileContextModeSkipped, ""
	tooLong := nativePDF("too_long_text", 1024, 3, false)
	tooLong.ContextMode = fileContextModeSkipped
	attachments := []AttachmentInput{
		nativePDF("oldest", 4*1024*1024, 10, false),
		retrieval, assistant, scanned, tooLong,
		nativePDF("huge", maxNativeDocumentBytes+1, 1, false),
		nativePDF("recent", 3*1024*1024, 40, false),
		nativePDF("current", 2*1024*1024, 50, true),
	}

	selected := selectNativeDocuments(attachments, nativeInputPolicy{PDF: true})

	ids := make([]string, 0, len(selected))
	for _, att := range selected {
		ids = append(ids, att.FileID)
	}
	// current(50 页)+recent(40 页)+scanned(3 页) 已占 93 页，oldest 的 10 页超出页数上限。
	if got := strings.Join(ids, ","); got != "current,recent,scanned" {
		t.Fatalf("selected = %q", got)
	}
	if selectNativeDocuments(attachments, nativeInputPolicy{}) != nil {
		t.Fatal("no native documents without PDF support")
	}
}

// 端到端：原生 PDF 随所属轮次发送并留文件名说明；不支持原生文档的路由（故障转移）退回文本。
func TestBuildMessageRoutePromptSendsNativePDFPerRoute(t *testing.T) {
	store := objectstorage.NewLocal(t.TempDir())
	if _, err := store.Put(t.Context(), "files/report", bytes.NewReader([]byte("%PDF-1.7 report")), objectstorage.PutOptions{ContentType: "application/pdf"}); err != nil {
		t.Fatalf("put pdf: %v", err)
	}
	service := &Service{storeProvider: &conversationTestStoreProvider{store: store}}
	report := nativePDF("report", int64(len("%PDF-1.7 report")), 2, true)
	input := messageRoutePromptInput{
		DomainMessages:    []model.Message{{Role: "user", Content: "总结这份报告"}},
		ConversationFiles: []AttachmentInput{report},
	}

	native, err := service.buildMessageRoutePrompt(t.Context(), &channel.ResolvedRoute{
		Protocol: llm.AdapterAnthropicMessages, CatalogInputModalities: []string{"text", "image", "pdf"},
	}, input)
	if err != nil {
		t.Fatalf("build native prompt: %v", err)
	}
	parts := native.Messages[len(native.Messages)-1].Parts
	if len(parts) != 3 || parts[0].Kind != llm.ContentPartFile || parts[1].Kind != llm.ContentPartDocument || parts[2].Text != "总结这份报告" {
		t.Fatalf("expected file note, native document, then question, got %#v", parts)
	}
	if !strings.Contains(parts[0].Text, `<document source="report.pdf" access="native">`) || strings.Contains(parts[0].Text, "提取文本") {
		t.Fatalf("native files must be named without duplicating the extracted text, got %q", parts[0].Text)
	}
	if string(parts[1].Data) != "%PDF-1.7 report" || parts[1].MimeType != "application/pdf" || parts[1].PageCount != 2 {
		t.Fatalf("unexpected native document part: %#v", parts[1])
	}
	// 没有动态上下文时整条消息都会在下一轮原样重现，缓存断点落在最后一个内容块上，覆盖原生文档。
	if parts[1].CacheControl != nil || parts[2].CacheControl == nil {
		t.Fatalf("expected the cache boundary after the native document, got %#v", parts)
	}

	fallback, err := service.buildMessageRoutePrompt(t.Context(), &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIChatCompletions, BaseURL: "https://relay.example.com/v1", CatalogInputModalities: []string{"text", "image", "pdf"},
	}, input)
	if err != nil {
		t.Fatalf("build fallback prompt: %v", err)
	}
	for _, part := range fallback.Messages[len(fallback.Messages)-1].Parts {
		if part.Kind == llm.ContentPartDocument {
			t.Fatalf("relay routes must not receive native documents, got %#v", part)
		}
	}
	if documents, _ := turnParts(t, fallback.Messages[len(fallback.Messages)-1]); !strings.Contains(documents, "提取文本") {
		t.Fatalf("relay routes fall back to extracted text, got %q", documents)
	}
}

// 纯文本模型：图片不再以图片内容块发送，所属轮次留「不支持」说明，模型能如实告知用户。
func TestBuildMessageRoutePromptExplainsImagesForTextOnlyModels(t *testing.T) {
	service := &Service{}
	image := AttachmentInput{FileID: "photo", FileName: "photo.png", Kind: "image", MimeType: "image/png", StoragePath: "images/photo",
		ContextMode: fileContextModeDirectImage, Current: true, MessageRole: "user"}
	plan, err := service.buildMessageRoutePrompt(t.Context(), &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIChatCompletions, CatalogInputModalities: []string{"text"},
	}, messageRoutePromptInput{
		DomainMessages:    []model.Message{{Role: "user", Content: "图里是什么"}},
		ConversationFiles: []AttachmentInput{image},
		DynamicContext:    userContextInput{Attachments: []AttachmentInput{image}},
	})
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}
	latest := plan.Messages[len(plan.Messages)-1]
	for _, part := range latest.Parts {
		if part.Kind == llm.ContentPartImage {
			t.Fatalf("text-only models must not receive image parts, got %#v", latest.Parts)
		}
	}
	documents, question := turnParts(t, latest)
	if !strings.Contains(documents, `<document source="photo.png" access="unsupported">`) || question != "图里是什么" {
		t.Fatalf("expected an unsupported note before the question, got %#v", latest)
	}
}

// 文件上下文展示按路由的最终决定分组：原生 PDF 归入 native，模型不支持的图片归入 unsupported 并计入未纳入。
func TestAttachmentRouteTraceShowsNativeAndUnsupportedFiles(t *testing.T) {
	store := objectstorage.NewLocal(t.TempDir())
	if _, err := store.Put(t.Context(), "files/report", bytes.NewReader([]byte("%PDF-1.7")), objectstorage.PutOptions{ContentType: "application/pdf"}); err != nil {
		t.Fatalf("put pdf: %v", err)
	}
	provider := &conversationTestStoreProvider{store: store}
	service := &Service{storeProvider: provider}
	report := nativePDF("report", int64(len("%PDF-1.7")), 1, true)
	missing := nativePDF("missing", 1024, 1, true)
	photo := AttachmentInput{FileID: "photo", FileName: "photo.png", Kind: "image", MimeType: "image/png", ContextMode: fileContextModeDirectImage, Current: true, MessageRole: "user"}
	files := []AttachmentInput{report, missing, photo}
	cache := newNativeDocumentCache()

	pdfRoute := &channel.ResolvedRoute{Protocol: llm.AdapterAnthropicMessages, CatalogInputModalities: []string{"text", "pdf"}}
	trace := attachmentRouteTrace{fileMode: "auto", items: files}
	trace.resolve(t.Context(), service, pdfRoute, files, cache)

	groups := trace.payload.FileGroupRefs
	if len(groups.Native) != 1 || groups.Native[0].FileID != "report" {
		t.Fatalf("expected the readable PDF as native, got %#v", groups)
	}
	if len(groups.Adaptive) != 1 || groups.Adaptive[0].FileID != "missing" {
		t.Fatalf("an unreadable PDF falls back to extracted text, got %#v", groups)
	}
	if len(groups.Unsupported) != 1 || groups.Unsupported[0].FileID != "photo" || len(groups.DirectImages) != 0 {
		t.Fatalf("expected the image as unsupported on a model without image input, got %#v", groups)
	}
	if stage := firstTraceStage(trace.payload); stage == nil || stage.IncludedCount != 2 || stage.SkippedCount != 1 {
		t.Fatalf("unsupported files count as not included, got %#v", stage)
	}

	// 故障转移到另一条路由：重新判断，但已读取（或读取失败）的文件不再访问存储。
	opens := provider.opens
	relay := &channel.ResolvedRoute{Protocol: llm.AdapterOpenAIChatCompletions, BaseURL: "https://relay.example.com/v1", CatalogInputModalities: []string{"text", "image", "pdf"}}
	trace.resolve(t.Context(), service, relay, files, cache)
	groups = trace.payload.FileGroupRefs
	if len(groups.Native) != 0 || len(groups.DirectImages) != 1 || len(groups.Adaptive) != 2 {
		t.Fatalf("relay route must show extracted text and direct images, got %#v", groups)
	}
	trace.resolve(t.Context(), service, pdfRoute, files, cache)
	if provider.opens != opens {
		t.Fatalf("cached native documents must not be read again, opens %d -> %d", opens, provider.opens)
	}
}

func TestReplaceProcessSectionUpdatesFileContextAfterFailover(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg:       config.Config{ProcessTraceEnabled: true},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_1"},
	}
	files := []AttachmentInput{nativePDF("report", 1024, 1, true)}
	first := routeAttachmentTraceItems(files, nativeInputPolicy{Image: true, PDF: true}, map[string]llm.ContentPart{"report": {}}, false)
	summary, markdown, payload := buildAttachmentProcessTrace("auto", first)
	recorder.appendProcessSection(summary, markdown, payload, messageTraceStatusStreaming)
	recorder.appendProcessSection("检索完成", "**内容检索**：完成。", nil, messageTraceStatusStreaming)

	nextSummary, nextMarkdown, nextPayload := buildAttachmentProcessTrace("auto", routeAttachmentTraceItems(files, nativeInputPolicy{Image: true}, nil, false))
	recorder.replaceProcessSection(summary, markdown, nextSummary, nextMarkdown, nextPayload)

	if refs := recorder.process.payload.FileGroupRefs; len(refs.Native) != 0 || len(refs.Adaptive) != 1 {
		t.Fatalf("expected file groups of the failover route, got %#v", refs)
	}
	if strings.Count(recorder.process.contentMarkdown, "**文件上下文**") != 1 || !strings.Contains(recorder.process.contentMarkdown, "**内容检索**") {
		t.Fatalf("expected the file context section replaced in place, got %q", recorder.process.contentMarkdown)
	}
	if recorder.process.summary != "检索完成" {
		t.Fatalf("a later stage title must be kept, got %q", recorder.process.summary)
	}
	fileStages := 0
	for _, stage := range recorder.process.payload.Stages {
		if stage.Kind == processTraceKindFileContext {
			fileStages++
		}
	}
	if fileStages != 1 {
		t.Fatalf("expected a single file context stage, got %#v", recorder.process.payload.Stages)
	}
}
