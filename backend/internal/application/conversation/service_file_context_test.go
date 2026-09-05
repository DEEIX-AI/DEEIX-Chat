package conversation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	apprag "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/rag"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstore"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestFileContextPlanRAGObjectsPreservesFileRevision(t *testing.T) {
	updatedAt := time.Unix(123, 456)
	files := fileContextPlanRAGObjects([]AttachmentInput{{
		FileObjID:     7,
		FileID:        "file_7",
		FileName:      "policy.md",
		EmbedStatus:   "ready",
		ChunkCount:    3,
		FileUpdatedAt: updatedAt,
	}})
	if len(files) != 1 || !files[0].UpdatedAt.Equal(updatedAt) {
		t.Fatalf("fileContextPlanRAGObjects() = %#v, want file revision preserved", files)
	}
}

type conversationTestStoreProvider struct {
	store objectstore.Store
	opens int
}

func (p *conversationTestStoreProvider) Open(context.Context) (objectstore.Store, error) {
	p.opens++
	return p.store, nil
}

var _ appstorage.Provider = (*conversationTestStoreProvider)(nil)

func TestCollectConversationFileIDsIgnoresFailedHistoricalMessages(t *testing.T) {
	messages := []model.Message{
		{
			Status:      "success",
			Attachments: `[{"file_id":"file_success"}]`,
		},
		{
			Status:      "error",
			Attachments: `[{"file_id":"file_failed"}]`,
		},
	}

	got := collectConversationFileIDs(messages, []string{"file_current"})
	want := []string{"file_success", "file_current"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestBindAttachmentMessageRolesPrefersUserOwnership(t *testing.T) {
	items := []AttachmentInput{{FileID: "shared"}, {FileID: "assistant_only"}}
	messages := []model.Message{
		{Role: "assistant", Attachments: `[{"file_id":"shared"},{"file_id":"assistant_only"}]`},
		{Role: "user", Attachments: `[{"file_id":"shared"}]`},
	}

	got := bindAttachmentMessageRoles(items, messages)
	if got[0].MessageRole != "user" || got[1].MessageRole != "assistant" {
		t.Fatalf("expected user role to win for shared attachment, got %#v", got)
	}
}

func TestResolveKnowledgeBaseRAGFilesFiltersAndDeduplicatesReadyFiles(t *testing.T) {
	service := &Service{
		cfg:    config.NewRuntime(config.Config{KnowledgeBaseEnabled: true}),
		ragSvc: &apprag.Service{},
		knowledgeBaseResolver: knowledgeBaseResolverStub{resolveFiles: func(context.Context, uint, []string) ([]domainknowledgebase.KnowledgeBase, []model.FileObject, error) {
			return []domainknowledgebase.KnowledgeBase{{PublicID: "kb-one", ReadyFileCount: 1}}, []model.FileObject{
				{ID: 1, FileID: "ready", ProcessingReady: true, EmbedStatus: "ready", ChunkCount: 2},
				{ID: 1, FileID: "ready", ProcessingReady: true, EmbedStatus: "ready", ChunkCount: 2},
				{ID: 2, FileID: "indexing", ProcessingReady: false, EmbedStatus: "processing"},
			}, nil
		}},
	}

	files, err := service.resolveKnowledgeBaseRAGFiles(context.Background(), 11, []string{"kb-one"}, true)
	if err != nil {
		t.Fatalf("resolveKnowledgeBaseRAGFiles() error = %v", err)
	}
	if len(files) != 1 || files[0].FileID != "ready" {
		t.Fatalf("resolved files = %#v, want one ready deduplicated file", files)
	}
}

func TestResolveKnowledgeBaseRAGFilesMapsUnavailableReference(t *testing.T) {
	service := &Service{
		cfg:    config.NewRuntime(config.Config{KnowledgeBaseEnabled: true}),
		ragSvc: &apprag.Service{},
		knowledgeBaseResolver: knowledgeBaseResolverStub{resolveFiles: func(context.Context, uint, []string) ([]domainknowledgebase.KnowledgeBase, []model.FileObject, error) {
			return nil, nil, domainknowledgebase.ErrReferenceUnavailable
		}},
	}

	_, err := service.resolveKnowledgeBaseRAGFiles(context.Background(), 11, []string{"missing"}, true)
	if !errors.Is(err, ErrInvalidKnowledgeBaseReference) {
		t.Fatalf("resolveKnowledgeBaseRAGFiles() error = %v, want ErrInvalidKnowledgeBaseReference", err)
	}
}

func TestResolveKnowledgeBaseRAGFilesSkipsWhenFeatureDisabled(t *testing.T) {
	service := &Service{
		cfg:    config.NewRuntime(config.Config{KnowledgeBaseEnabled: false}),
		ragSvc: &apprag.Service{},
		knowledgeBaseResolver: knowledgeBaseResolverStub{resolveFiles: func(context.Context, uint, []string) ([]domainknowledgebase.KnowledgeBase, []model.FileObject, error) {
			t.Fatal("resolver should not be called when knowledge base feature is disabled")
			return nil, nil, nil
		}},
	}

	files, err := service.resolveKnowledgeBaseRAGFiles(context.Background(), 11, []string{"kb-one"}, true)
	if err != nil {
		t.Fatalf("resolveKnowledgeBaseRAGFiles() error = %v", err)
	}
	if files != nil {
		t.Fatalf("resolved files = %#v, want nil when feature disabled", files)
	}
}

func TestConversationImageRefsPreferRecentImagesWithinBudget(t *testing.T) {
	messages := make([]model.Message, 0, maxConversationImageContextCount+1)
	attachments := make([]AttachmentInput, 0, maxConversationImageContextCount+1)
	for index := 0; index <= maxConversationImageContextCount; index++ {
		fileID := fmt.Sprintf("image_%d", index)
		messages = append(messages, model.Message{Role: "user", Attachments: fmt.Sprintf(`[{"file_id":%q,"kind":"image","mime_type":"image/png"}]`, fileID)})
		attachments = append(attachments, AttachmentInput{FileID: fileID, Kind: "image", MimeType: "image/png", ContextMode: fileContextModeDirectImage})
	}

	refs := conversationImageRefs(messages, attachments, maxConversationImageContextCount)
	if len(refs) != maxConversationImageContextCount {
		t.Fatalf("expected %d recent images, got %#v", maxConversationImageContextCount, refs)
	}
	if refs[0].fileID != "image_1" || refs[len(refs)-1].fileID != "image_10" {
		t.Fatalf("expected oldest image to be trimmed, got %#v", refs)
	}
}

func TestInjectConversationImageContextKeepsOwnershipAndUsesCache(t *testing.T) {
	store := objectstore.NewLocal(t.TempDir())
	for key, data := range map[string][]byte{
		"images/one": []byte("image-one"),
		"images/two": []byte("image-two"),
	} {
		if _, err := store.Put(t.Context(), key, bytes.NewReader(data), objectstore.PutOptions{ContentType: "image/png"}); err != nil {
			t.Fatalf("put test image %s: %v", key, err)
		}
	}

	domainMessages := []model.Message{
		{Role: "user", Content: "描述第一张图片", Attachments: `[{"file_id":"image_1","kind":"image","mime_type":"image/png"}]`},
		{Role: "assistant", Content: "第一张是雪山"},
		{Role: "user", Content: "第二张和第一张有什么不同", Attachments: `[{"file_id":"image_2","kind":"image","mime_type":"image/png"}]`},
		{Role: "assistant", Content: "第二张是河谷"},
		{Role: "user", Content: "第一张图片里有没有云朵"},
	}
	attachments := []AttachmentInput{
		{FileID: "image_1", Kind: "image", MimeType: "image/png", StoragePath: "images/one", ContextMode: fileContextModeDirectImage},
		{FileID: "image_2", Kind: "image", MimeType: "image/png", StoragePath: "images/two", ContextMode: fileContextModeDirectImage},
	}
	provider := &conversationTestStoreProvider{store: store}
	service := &Service{storeProvider: provider, imageContextCache: defaultPreparedConversationImageCache()}
	history := historyMessagesFromDomain(domainMessages, historyMessageOptions{})

	got, err := service.injectConversationImageContext(t.Context(), history, domainMessages, attachments, config.Config{ImageMaxDimension: 1024})
	if err != nil {
		t.Fatalf("inject historical images: %v", err)
	}
	if len(got[0].Parts) != 2 || string(got[0].Parts[1].Data) != "image-one" {
		t.Fatalf("expected first image on first user message, got %#v", got[0])
	}
	if len(got[2].Parts) != 2 || string(got[2].Parts[1].Data) != "image-two" {
		t.Fatalf("expected second image on second user message, got %#v", got[2])
	}
	if _, err = service.injectConversationImageContext(t.Context(), history, domainMessages, attachments, config.Config{ImageMaxDimension: 1024}); err != nil {
		t.Fatalf("inject cached historical images: %v", err)
	}
	if provider.opens != 1 {
		t.Fatalf("expected prepared image cache to avoid repeated storage opens, got %d", provider.opens)
	}
}

func TestInjectConversationImageContextRejectsMissingAndOversizedContext(t *testing.T) {
	domainMessages := []model.Message{{Role: "user", Attachments: `[{"file_id":"missing","kind":"image","mime_type":"image/png"}]`}}
	service := &Service{imageContextCache: defaultPreparedConversationImageCache()}
	_, err := service.injectConversationImageContext(t.Context(), historyMessagesFromDomain(domainMessages, historyMessageOptions{}), domainMessages, nil, config.Config{})
	if !errors.Is(err, ErrInvalidFileReference) {
		t.Fatalf("expected missing historical image to fail explicitly, got %v", err)
	}

	domainMessages = []model.Message{{Role: "user", Attachments: `[{"file_id":"stored","kind":"image","mime_type":"image/png"}]`}}
	attachments := []AttachmentInput{{FileID: "stored", Kind: "image", MimeType: "image/png", StoragePath: "images/stored.png", ContextMode: fileContextModeDirectImage}}
	_, err = service.injectConversationImageContext(t.Context(), historyMessagesFromDomain(domainMessages, historyMessageOptions{}), domainMessages, attachments, config.Config{})
	if !errors.Is(err, appstorage.ErrProviderNotConfigured) {
		t.Fatalf("expected missing object store provider to fail explicitly, got %v", err)
	}

	largeData := make([]byte, 11*1024*1024)
	cache := defaultPreparedConversationImageCache()
	attachments = []AttachmentInput{
		{FileID: "one", Kind: "image", MimeType: "image/png", StoragePath: "one", ContextMode: fileContextModeDirectImage},
		{FileID: "two", Kind: "image", MimeType: "image/png", StoragePath: "two", ContextMode: fileContextModeDirectImage},
	}
	for _, att := range attachments {
		cache.put(preparedConversationImageCacheKey(att, 1024, "image/png"), preparedConversationImage{data: largeData, mimeType: "image/png"})
	}
	service = &Service{imageContextCache: cache}
	domainMessages = []model.Message{
		{Role: "user", Attachments: `[{"file_id":"one","kind":"image","mime_type":"image/png"}]`},
		{Role: "assistant"},
		{Role: "user", Attachments: `[{"file_id":"two","kind":"image","mime_type":"image/png"}]`},
	}
	_, err = service.injectConversationImageContext(t.Context(), historyMessagesFromDomain(domainMessages, historyMessageOptions{}), domainMessages, attachments, config.Config{ImageMaxDimension: 1024})
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("expected aggregate image context budget failure, got %v", err)
	}
}

func TestResizeImageIfNeededReturnsActualMIME(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 120, G: 80, B: 40, A: 255})
		}
	}
	var source bytes.Buffer
	if err := png.Encode(&source, img); err != nil {
		t.Fatalf("encode source image: %v", err)
	}
	resized, mimeType := resizeImageIfNeeded(source.Bytes(), "image/webp", 2)
	if mimeType != "image/jpeg" {
		t.Fatalf("expected resized non-PNG image to report JPEG, got %q", mimeType)
	}
	if _, format, err := image.Decode(bytes.NewReader(resized)); err != nil || format != "jpeg" {
		t.Fatalf("expected JPEG bytes, format=%q err=%v", format, err)
	}
}

func TestInjectUserContextUsesCompactXMLForRAG(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "怎么发布？"}}
	chunks := []model.RAGChunk{{
		FileName:   "AGENTS.md",
		ChunkIndex: 3,
		Content:    "Run pnpm build.",
	}}

	got := injectUserContext(t.Context(), messages, userContextInput{RAGChunks: chunks}, config.Config{}, nil)
	for _, want := range []string{"<ctx>", "<rag>", `<doc name="AGENTS.md" i="3">Run pnpm build.</doc>`, "</ctx>", "<q>怎么发布？</q>"} {
		if !strings.Contains(got[0].Content, want) {
			t.Fatalf("expected RAG XML to contain %q, got %q", want, got[0].Content)
		}
	}
	if strings.Contains(got[0].Content, "<files>") {
		t.Fatalf("did not expect files section for RAG-only context, got %q", got[0].Content)
	}
}

func TestInjectUserContextIncludesKnowledgeBaseMissNotice(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "知识库里怎么规定？"}}
	notice := "The selected knowledge base returned no relevant evidence."

	got := injectUserContext(t.Context(), messages, userContextInput{RAGNotice: notice}, config.Config{}, nil)
	if len(got) != 1 || !strings.Contains(got[0].Content, "<rag_status>"+notice+"</rag_status>") {
		t.Fatalf("expected knowledge-base miss notice, got %#v", got)
	}
	if !strings.Contains(got[0].Content, "<q>知识库里怎么规定？</q>") {
		t.Fatalf("expected original request to remain present, got %q", got[0].Content)
	}
}

func TestInjectUserContextPreservesExistingImageParts(t *testing.T) {
	messages := []llm.Message{{
		Role: "user",
		Parts: []llm.ContentPart{
			{Kind: llm.ContentPartText, Text: "继续分析"},
			{Kind: llm.ContentPartImage, MimeType: "image/png", Data: []byte("image")},
		},
	}}
	got := injectUserContext(t.Context(), messages, userContextInput{RAGChunks: []model.RAGChunk{{FileName: "note.md", Content: "偏好简洁回答"}}}, config.Config{}, nil)
	if len(got) != 1 || len(got[0].Parts) != 2 {
		t.Fatalf("expected text and existing image parts, got %#v", got)
	}
	if got[0].Parts[1].Kind != llm.ContentPartImage || string(got[0].Parts[1].Data) != "image" {
		t.Fatalf("expected existing image part to be preserved, got %#v", got[0].Parts)
	}
	if !strings.Contains(got[0].Parts[0].Text, "继续分析") || !strings.Contains(got[0].Parts[0].Text, "偏好简洁回答") {
		t.Fatalf("expected dynamic context and original text, got %q", got[0].Parts[0].Text)
	}
}

func TestInjectUserContextSilentlySkipsImagesWithoutObjectStoreProvider(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "describe this image"}}
	got := injectUserContext(t.Context(), messages, userContextInput{Attachments: []AttachmentInput{{
		FileID: "image-1", Kind: "image", MimeType: "image/png", StoragePath: "images/one.png", Current: true,
	}}}, config.Config{}, nil)
	if len(got) != 1 || len(got[0].Parts) != 0 || got[0].Content != messages[0].Content {
		t.Fatalf("expected missing provider to leave the message unchanged, got %#v", got)
	}
}

func TestPrependStableFileContextKeepsFilesAtPromptTop(t *testing.T) {
	messages := []llm.Message{
		{Role: "user", Content: "第一轮问题"},
		{Role: "assistant", Content: "第一轮回答"},
		{Role: "user", Content: "继续修改上一轮回答"},
	}
	attachments := []AttachmentInput{
		{
			FileID:        "b",
			FileName:      "B.md",
			FileCategory:  "document",
			ExtractedText: "second file",
		},
		{
			FileID:        "a",
			FileName:      "A.md",
			FileCategory:  "document",
			ExtractedText: "first file",
		},
	}

	got := prependStableFileContext(messages, attachments)
	if len(got) != len(messages)+1 {
		t.Fatalf("expected stable context to be prepended, got %d messages", len(got))
	}
	if got[0].Role != "system" {
		t.Fatalf("expected top context role system, got %q", got[0].Role)
	}
	for _, want := range []string{"<ctx>", "<files>", `<file name="A.md">first file</file>`, `<file name="B.md">second file</file>`, "</ctx>"} {
		if !strings.Contains(got[0].Content, want) {
			t.Fatalf("expected top context to contain %q, got %q", want, got[0].Content)
		}
	}
	if strings.Contains(got[len(got)-1].Content, "<files>") {
		t.Fatalf("expected latest user message to stay focused on current turn, got %q", got[len(got)-1].Content)
	}
	if strings.Index(got[0].Content, `name="A.md"`) > strings.Index(got[0].Content, `name="B.md"`) {
		t.Fatalf("expected stable file order by file id, got %q", got[0].Content)
	}
}

func TestPrependStableFileContextIncludesHistoricalImageOCRText(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "继续看图"}}
	attachments := []AttachmentInput{{
		Kind:          "image",
		MimeType:      "image/png",
		FileName:      "photo.png",
		ExtractedText: "图片 OCR 文字",
		ContextMode:   fileContextModeFull,
	}}

	got := prependStableFileContext(messages, attachments)
	if len(got) != len(messages)+1 {
		t.Fatalf("expected historical image OCR text to be prepended, got %#v", got)
	}
	if !strings.Contains(got[0].Content, `<file name="photo.png">图片 OCR 文字</file>`) {
		t.Fatalf("expected stable context to contain OCR text, got %q", got[0].Content)
	}
}

func TestPrependStableFileContextSkipsCurrentDirectImageOCRText(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "看图"}}
	attachments := []AttachmentInput{{
		Kind:          "image",
		MimeType:      "image/png",
		FileName:      "photo.png",
		ExtractedText: "本轮图片 OCR 不应重复注入",
		ContextMode:   fileContextModeDirectImage,
		Current:       true,
	}}

	got := prependStableFileContext(messages, attachments)
	if len(got) != len(messages) {
		t.Fatalf("expected current direct image OCR text to stay out of stable context, got %#v", got)
	}
}

func TestPrependStableFileContextEscapesXMLFileContext(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "总结文件"}}
	attachments := []AttachmentInput{{
		FileName:      `A&B "notes".md`,
		FileCategory:  "document",
		ExtractedText: "Use <tag> & keep > value.\n\nNext line.",
	}}

	got := prependStableFileContext(messages, attachments)
	if len(got) != len(messages)+1 {
		t.Fatalf("expected stable file context to be prepended, got %#v", got)
	}
	for _, want := range []string{
		`<file name="A&amp;B &#34;notes&#34;.md">`,
		"Use &lt;tag&gt; &amp; keep &gt; value.\n\nNext line.",
	} {
		if !strings.Contains(got[0].Content, want) {
			t.Fatalf("expected escaped XML content to contain %q, got %q", want, got[0].Content)
		}
	}
	if strings.Contains(got[0].Content, "&#xA;") {
		t.Fatalf("expected XML text content to keep real newlines, got %q", got[0].Content)
	}
}

func TestBuildConversationFileContextPlanSkipsOversizedFileWhenRAGUnavailable(t *testing.T) {
	cfg := config.Config{FileFullContextMaxTokens: 10}
	plan := buildConversationFileContextPlan([]AttachmentInput{{
		FileID:        "file_large",
		FileName:      "large.md",
		FileCategory:  "document",
		ExtractedText: strings.Repeat("token ", 100),
		EmbedStatus:   "pending",
	}}, "auto", cfg, "gpt-5.5", "", false)

	if len(plan.FullAttachments) != 0 || len(plan.RAGAttachments) != 0 || len(plan.Skipped) != 1 {
		t.Fatalf("expected oversized unavailable file to be skipped, got %#v", plan)
	}
	if plan.Skipped[0].ContextMode != fileContextModeSkipped {
		t.Fatalf("expected skipped context mode, got %#v", plan.Skipped[0])
	}
}

func TestBuildConversationFileContextPlanOnlyDirectUploadsCurrentImages(t *testing.T) {
	cfg := config.Config{RAGEnabled: true, EmbeddingEnabled: true}
	plan := buildConversationFileContextPlan([]AttachmentInput{
		{
			FileID:       "file_current_image",
			Kind:         "image",
			MimeType:     "image/png",
			DetectedMIME: "image/png",
			Current:      true,
		},
		{
			FileID:       "file_history_image",
			Kind:         "image",
			MimeType:     "image/png",
			DetectedMIME: "image/png",
			EmbedStatus:  "ready",
		},
	}, "auto", cfg, "gpt-5.5", "", true)

	if len(plan.FullAttachments) != 1 || plan.FullAttachments[0].FileID != "file_current_image" {
		t.Fatalf("expected only current image to be direct/full context, got %#v", plan.FullAttachments)
	}
	if plan.FullAttachments[0].ContextMode != fileContextModeDirectImage {
		t.Fatalf("expected current image to use direct image mode, got %#v", plan.FullAttachments[0])
	}
	if len(plan.RAGAttachments) != 1 || plan.RAGAttachments[0].FileID != "file_history_image" {
		t.Fatalf("expected historical OCR image to use RAG instead of direct upload, got %#v", plan.RAGAttachments)
	}
	if plan.RAGAttachments[0].ContextMode != fileContextModeRAG {
		t.Fatalf("expected historical image context mode RAG, got %#v", plan.RAGAttachments[0])
	}
}

func TestBuildConversationFileContextPlanDirectUploadsHistoricalUserImages(t *testing.T) {
	plan := buildConversationFileContextPlan([]AttachmentInput{{
		FileID:       "file_history_user_image",
		Kind:         "image",
		MimeType:     "image/png",
		DetectedMIME: "image/png",
		MessageRole:  "user",
	}}, "rag", config.Config{RAGEnabled: true, EmbeddingEnabled: true}, "gpt-5.5", "", true)

	if len(plan.FullAttachments) != 1 || plan.FullAttachments[0].ContextMode != fileContextModeDirectImage {
		t.Fatalf("expected historical user image to remain direct image context, got %#v", plan)
	}
	if len(plan.RAGAttachments) != 0 {
		t.Fatalf("expected historical user image not to fall back to OCR/RAG, got %#v", plan.RAGAttachments)
	}
}

func TestBuildConversationFileContextPlanUsesRAGForHistoricalImageOCRWhenRequested(t *testing.T) {
	cfg := config.Config{RAGEnabled: true, EmbeddingEnabled: true}
	plan := buildConversationFileContextPlan([]AttachmentInput{{
		FileID:        "file_history_image",
		Kind:          "image",
		MimeType:      "image/png",
		DetectedMIME:  "image/png",
		EmbedStatus:   "ready",
		ExtractedText: "historical image OCR",
	}}, "rag", cfg, "gpt-5.5", "", true)

	if len(plan.RAGAttachments) != 1 || plan.RAGAttachments[0].FileID != "file_history_image" {
		t.Fatalf("expected historical image OCR to use RAG in rag mode, got %#v", plan.RAGAttachments)
	}
	if len(plan.FullAttachments) != 0 {
		t.Fatalf("expected no full attachments while RAG is available in rag mode, got %#v", plan.FullAttachments)
	}
}

func TestBuildConversationFileContextPlanUsesHistoricalImageOCRTextAsFullContext(t *testing.T) {
	plan := buildConversationFileContextPlan([]AttachmentInput{{
		FileID:        "file_history_image",
		Kind:          "image",
		MimeType:      "image/png",
		DetectedMIME:  "image/png",
		ExtractedText: "historical image OCR",
		EmbedStatus:   "pending",
	}}, "auto", config.Config{}, "gpt-5.5", "", false)

	if len(plan.FullAttachments) != 1 || plan.FullAttachments[0].FileID != "file_history_image" {
		t.Fatalf("expected historical image OCR text to use full context, got %#v", plan.FullAttachments)
	}
	if plan.FullAttachments[0].ContextMode != fileContextModeFull {
		t.Fatalf("expected historical image OCR text context mode full, got %#v", plan.FullAttachments[0])
	}
	if len(plan.RAGAttachments) != 0 {
		t.Fatalf("expected no RAG attachments when retrieval is unavailable, got %#v", plan.RAGAttachments)
	}
}

func TestBuildConversationFileContextPlanKeepsVideoOutOfTextContext(t *testing.T) {
	plan := buildConversationFileContextPlan([]AttachmentInput{{
		FileID:        "file_video",
		Kind:          "video",
		MimeType:      "video/mp4",
		DetectedMIME:  "video/mp4",
		FileCategory:  fileCategoryVideo,
		ExtractedText: "unexpected video text",
	}}, "auto", config.Config{}, "gpt-5.5", "", false)

	if len(plan.FullAttachments) != 0 || len(plan.RAGAttachments) != 0 {
		t.Fatalf("expected video to stay out of text context, got %#v", plan)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].ContextMode != fileContextModeSkipped {
		t.Fatalf("expected video to be skipped, got %#v", plan.Skipped)
	}
}

func TestImageAttachmentsForCurrentUserSkipsHistoricalImages(t *testing.T) {
	got := imageAttachmentsForCurrentUser([]AttachmentInput{
		{
			FileID:   "file_history_image",
			Kind:     "image",
			MimeType: "image/png",
		},
		{
			FileID:   "file_current_image",
			Kind:     "image",
			MimeType: "image/png",
			Current:  true,
		},
	})

	if len(got) != 1 || got[0].FileID != "file_current_image" {
		t.Fatalf("expected only current image to be injected as image part, got %#v", got)
	}
}

func TestShouldShowAttachmentProcessTraceSkipsHistoricalSkippedOnly(t *testing.T) {
	if shouldShowAttachmentProcessTrace([]AttachmentInput{{
		FileID:      "file_history_image",
		ContextMode: fileContextModeSkipped,
	}}) {
		t.Fatal("expected historical skipped-only attachments to stay out of process trace")
	}
}

func TestShouldShowAttachmentProcessTraceSkipsHistoricalDirectImages(t *testing.T) {
	items := []AttachmentInput{{
		FileID:      "file_history_image",
		Kind:        "image",
		ContextMode: fileContextModeDirectImage,
	}}
	if shouldShowAttachmentProcessTrace(items) {
		t.Fatal("expected historical direct images to stay out of repeated process traces")
	}
	if got := attachmentProcessTraceItems(items); len(got) != 0 {
		t.Fatalf("expected historical direct images to be filtered from trace payload, got %#v", got)
	}
}

func TestShouldShowAttachmentProcessTraceKeepsCurrentOrIncludedFiles(t *testing.T) {
	if !shouldShowAttachmentProcessTrace([]AttachmentInput{{
		FileID:      "file_current_image",
		ContextMode: fileContextModeSkipped,
		Current:     true,
	}}) {
		t.Fatal("expected current skipped attachments to be visible in process trace")
	}
	if !shouldShowAttachmentProcessTrace([]AttachmentInput{{
		FileID:      "file_history_image",
		ContextMode: fileContextModeRAG,
	}}) {
		t.Fatal("expected included historical attachments to be visible in process trace")
	}
}

func TestSplitRetrievalFallbackAttachmentsRespectsFullContextBudget(t *testing.T) {
	cfg := config.Config{FileFullContextMaxTokens: 10}
	fallbacks, skipped := splitRetrievalFallbackAttachments([]AttachmentInput{
		{
			FileID:        "small",
			FileName:      "small.md",
			FileCategory:  "document",
			ExtractedText: "short text",
		},
		{
			FileID:        "large",
			FileName:      "large.md",
			FileCategory:  "document",
			ExtractedText: strings.Repeat("token ", 100),
		},
	}, cfg)

	if len(fallbacks) != 1 || fallbacks[0].FileID != "small" || fallbacks[0].ContextMode != fileContextModeRAGFallback {
		t.Fatalf("expected only small file to fallback, got %#v", fallbacks)
	}
	if len(skipped) != 1 || skipped[0].FileID != "large" || skipped[0].ContextMode != fileContextModeSkipped {
		t.Fatalf("expected large file to be skipped, got %#v", skipped)
	}
}

func TestInjectUserContextCombinesDataContexts(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "继续"}}
	input := userContextInput{
		Snapshot: &snapshotContext{
			Summary:  "之前讨论了部署流程。",
			FromTurn: 1,
			ToTurn:   4,
			Strategy: "auto",
		},
		Memory: []domainmemory.UserMemory{{
			MemoryKey: "team",
			Value:     "prefers short answers",
		}},
		HistoricalArtifacts: []model.ContextArtifact{{
			Kind:        model.ContextArtifactFileRAGChunk,
			SourceTitle: "部署文档",
			Content:     "旧轮 RAG 证据提到先执行迁移。",
		}},
		RecallChunks: []model.MessageChunk{{
			Role:       "assistant",
			ChunkIndex: 2,
			Content:    "历史里提到需要先跑测试。",
		}},
	}

	got := injectUserContext(t.Context(), messages, input, config.Config{}, nil)
	for _, want := range []string{
		`<sum from="1" to="4" strategy="auto">之前讨论了部署流程。</sum>`,
		`<mem k="team">prefers short answers</mem>`,
		`<ev k="file_rag_chunk" src="部署文档">旧轮 RAG 证据提到先执行迁移。</ev>`,
		`<msg role="assistant" i="2">历史里提到需要先跑测试。</msg>`,
		"<q>继续</q>",
	} {
		if !strings.Contains(got[0].Content, want) {
			t.Fatalf("expected unified context to contain %q, got %q", want, got[0].Content)
		}
	}
}
