package conversation

import (
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func turnDocumentTestFile(fileID string, name string, text string, mode string) AttachmentInput {
	return AttachmentInput{
		FileID:        fileID,
		FileName:      name,
		Kind:          "file",
		MimeType:      "text/plain",
		FileCategory:  "text",
		ExtractedText: text,
		ContextMode:   mode,
	}
}

// turnParts 把用户消息拆成「文件块文本」与「用户原文」，并校验文件块位于原文之前。
func turnParts(t *testing.T, message llm.Message) (string, string) {
	t.Helper()
	if len(message.Parts) == 0 {
		return "", message.Content
	}
	documents, question := "", ""
	for index, part := range message.Parts {
		switch {
		case part.Kind == llm.ContentPartFile:
			if question != "" {
				t.Fatalf("documents must precede the question, got %#v", message.Parts)
			}
			documents = part.Text
		case part.Kind == llm.ContentPartText && !part.Dynamic:
			if index != len(message.Parts)-1 {
				t.Fatalf("the question must be the last part, got %#v", message.Parts)
			}
			question = part.Text
		}
	}
	return documents, question
}

// 文件必须落在提交它的那一轮，位于用户原文之前；本轮文件以 Current 归属最后一条用户消息。
func TestPlaceTurnDocumentsPutsFilesInOwningTurns(t *testing.T) {
	domainMessages := []model.Message{
		{Role: "user", Content: "整理这份", Attachments: `[{"file_id":"file_old"}]`},
		{Role: "assistant", Content: "已整理"},
		{Role: "user", Content: "继续"},
	}
	history := historyMessagesFromDomain(domainMessages, historyMessageOptions{})
	attachments := []AttachmentInput{
		turnDocumentTestFile("file_old", "old.md", "旧文件全文", fileContextModeFull),
		func() AttachmentInput {
			item := turnDocumentTestFile("file_new", "pasted-text-1.txt", "新粘贴的全文", fileContextModeFull)
			item.Current = true
			return item
		}(),
	}

	placement := placeTurnDocuments(history, domainMessages, attachments, turnDocumentOptions{})

	documents, question := turnParts(t, placement.Messages[0])
	if documents != "<documents>\n<document source=\"old.md\">\n旧文件全文\n</document>\n</documents>" || question != "整理这份" {
		t.Fatalf("historical file must precede its own question, got %#v", placement.Messages[0])
	}
	if strings.Contains(documents, "新粘贴的全文") {
		t.Fatalf("current file must not leak into an earlier turn, got %q", documents)
	}
	if placement.Messages[1].Content != "已整理" {
		t.Fatalf("assistant turn must stay untouched, got %q", placement.Messages[1].Content)
	}
	documents, question = turnParts(t, placement.Messages[2])
	if documents != "<documents>\n<document source=\"pasted-text-1.txt\">\n新粘贴的全文\n</document>\n</documents>" || question != "继续" {
		t.Fatalf("current file must sit in the current turn before the question, got %#v", placement.Messages[2])
	}
	if len(placement.FullAttachments) != 2 || placement.TokenEstimate <= 0 {
		t.Fatalf("expected trace data for both full documents, got %#v", placement)
	}
}

// 前缀缓存要求历史轮次跨请求字节一致：本轮渲染结果必须等于它下一轮作为历史时的渲染结果。
func TestPlaceTurnDocumentsRendersCurrentTurnLikeFutureHistory(t *testing.T) {
	file := turnDocumentTestFile("file_new", "notes.md", "全文", fileContextModeFull)

	currentFile := file
	currentFile.Current = true
	currentDomain := []model.Message{{Role: "user", Content: "看看这个"}}
	now := placeTurnDocuments(historyMessagesFromDomain(currentDomain, historyMessageOptions{}), currentDomain, []AttachmentInput{currentFile}, turnDocumentOptions{})

	nextDomain := []model.Message{
		{Role: "user", Content: "看看这个", Attachments: `[{"file_id":"file_new","file_name":"notes.md"}]`},
		{Role: "assistant", Content: "好的"},
		{Role: "user", Content: "下一问"},
	}
	next := placeTurnDocuments(historyMessagesFromDomain(nextDomain, historyMessageOptions{}), nextDomain, []AttachmentInput{file}, turnDocumentOptions{})

	nowDocuments, nowQuestion := turnParts(t, now.Messages[0])
	nextDocuments, nextQuestion := turnParts(t, next.Messages[0])
	if nowDocuments == "" || nowDocuments != nextDocuments || nowQuestion != nextQuestion {
		t.Fatalf("turn rendering drifted between requests:\nnow:  %#v\nnext: %#v", now.Messages[0], next.Messages[0])
	}
	if next.Messages[2].Content != "下一问" {
		t.Fatalf("a turn without files must stay raw, got %q", next.Messages[2].Content)
	}
}

func TestPlaceTurnDocumentsRendersRetrievalAndUnavailableStubs(t *testing.T) {
	domainMessages := []model.Message{{Role: "user", Content: "总结"}}
	big := turnDocumentTestFile("file_big", "big.pdf", "很长", fileContextModeRAG)
	big.Current = true
	broken := turnDocumentTestFile("file_broken", "broken.docx", "", fileContextModeSkipped)
	broken.Current = true

	placement := placeTurnDocuments(historyMessagesFromDomain(domainMessages, historyMessageOptions{}), domainMessages, []AttachmentInput{big, broken}, turnDocumentOptions{})

	content, _ := turnParts(t, placement.Messages[0])
	for _, want := range []string{
		`<document source="big.pdf" access="retrieval">` + turnDocumentRetrievalNote + `</document>`,
		`<document source="broken.docx" access="unavailable">` + turnDocumentUnavailableNote + `</document>`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected %q in %q", want, content)
		}
	}
	if strings.Contains(content, "很长") {
		t.Fatalf("retrieval files must not inline their full text, got %q", content)
	}
	if len(placement.FullAttachments) != 0 {
		t.Fatalf("stubs must not be reported as full documents, got %#v", placement.FullAttachments)
	}
}

// 助手回复产生的文件（如生成图片的 OCR 文本）随其后的第一条用户消息提供，并标注来源。
func TestPlaceTurnDocumentsMovesAssistantFilesToFollowingUserTurn(t *testing.T) {
	domainMessages := []model.Message{
		{Role: "user", Content: "画一张图"},
		{Role: "assistant", Content: "已生成", Attachments: `[{"file_id":"img_1"}]`},
		{Role: "user", Content: "图上写了什么"},
	}
	ocr := AttachmentInput{
		FileID:        "img_1",
		FileName:      "generated.png",
		Kind:          "image",
		MimeType:      "image/png",
		ExtractedText: "图片 OCR 文字",
		ContextMode:   fileContextModeFull,
	}

	placement := placeTurnDocuments(historyMessagesFromDomain(domainMessages, historyMessageOptions{}), domainMessages, []AttachmentInput{ocr}, turnDocumentOptions{})

	if placement.Messages[1].Content != "已生成" {
		t.Fatalf("assistant message must stay untouched, got %q", placement.Messages[1].Content)
	}
	if documents, _ := turnParts(t, placement.Messages[2]); !strings.Contains(documents, `<document source="generated.png" origin="assistant">`) {
		t.Fatalf("expected assistant file on the following user turn, got %#v", placement.Messages[2])
	}
}

func TestPlaceTurnDocumentsSkipsDirectImagesAndEscapesXML(t *testing.T) {
	domainMessages := []model.Message{{Role: "user", Content: "看图并总结文件"}}
	image := AttachmentInput{FileID: "img", FileName: "photo.png", Kind: "image", MimeType: "image/png", ExtractedText: "OCR", ContextMode: fileContextModeDirectImage, Current: true}
	doc := turnDocumentTestFile("doc", `A&B "notes".md`, "Use <tag> & keep > value.\n\nNext line.", fileContextModeFull)
	doc.Current = true

	placement := placeTurnDocuments(historyMessagesFromDomain(domainMessages, historyMessageOptions{}), domainMessages, []AttachmentInput{image, doc}, turnDocumentOptions{})

	content, _ := turnParts(t, placement.Messages[0])
	if strings.Contains(content, "photo.png") {
		t.Fatalf("direct images are sent as image parts, not documents, got %q", content)
	}
	for _, want := range []string{`<document source="A&amp;B &#34;notes&#34;.md">`, "Use &lt;tag&gt; &amp; keep &gt; value.\n\nNext line."} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected escaped %q in %q", want, content)
		}
	}
}

func TestPlaceTurnDocumentsRefusesMisalignedHistory(t *testing.T) {
	domainMessages := []model.Message{{Role: "user", Content: "a"}, {Role: "user", Content: "b"}}
	history := []llm.Message{{Role: "user", Content: "a"}}
	file := turnDocumentTestFile("f", "f.md", "text", fileContextModeFull)
	file.Current = true

	placement := placeTurnDocuments(history, domainMessages, []AttachmentInput{file}, turnDocumentOptions{})

	if placement.Messages[0].Content != "a" || len(placement.FullAttachments) != 0 {
		t.Fatalf("misaligned history must be left untouched, got %#v", placement)
	}
}

// 端到端：经真实的路由提示词构建流程，相邻两轮请求中「上一轮及之前」的内容必须逐字节一致，
// 本轮缓存断点覆盖的前缀在下一轮可以整段命中；新增文件只影响最新一轮。
func TestBuildMessageRoutePromptKeepsCachedPrefixStableAcrossTurns(t *testing.T) {
	service := &Service{}
	route := &channel.ResolvedRoute{UpstreamModel: "test-model"}
	oldFile := turnDocumentTestFile("file_old", "old.md", "旧文件全文", fileContextModeFull)
	newFile := turnDocumentTestFile("file_new", "pasted-text-2.txt", "新粘贴全文", fileContextModeFull)
	ragNotice := userContextInput{RAGChunks: []model.RAGChunk{{FileName: "kb.md", Content: "本轮检索片段"}}}

	firstCurrent := oldFile
	firstCurrent.Current = true
	first, err := service.buildMessageRoutePrompt(t.Context(), route, messageRoutePromptInput{
		DomainMessages: []model.Message{
			{Role: "user", Content: "整理这份"},
		},
		ConversationFiles: []AttachmentInput{firstCurrent},
		DynamicContext:    ragNotice,
		PreferencePrompt:  "用户偏好：简洁",
	})
	if err != nil {
		t.Fatalf("first turn: %v", err)
	}

	secondCurrent := newFile
	secondCurrent.Current = true
	second, err := service.buildMessageRoutePrompt(t.Context(), route, messageRoutePromptInput{
		DomainMessages: []model.Message{
			{Role: "user", Content: "整理这份", Attachments: `[{"file_id":"file_old"}]`},
			{Role: "assistant", Content: "已整理"},
			{Role: "user", Content: "继续"},
		},
		ConversationFiles: []AttachmentInput{oldFile, secondCurrent},
		DynamicContext:    ragNotice,
		PreferencePrompt:  "用户偏好：简洁",
	})
	if err != nil {
		t.Fatalf("second turn: %v", err)
	}

	// 第一轮内容块顺序为 文件 → 动态上下文 → 原文；断点落在文件块上（动态上下文之前），
	// 去掉动态段后，整条消息必须与第二轮重建的历史一致。
	firstUser := first.Messages[len(first.Messages)-1]
	if len(firstUser.Parts) != 3 || firstUser.Parts[0].Kind != llm.ContentPartFile || firstUser.Parts[0].CacheControl == nil ||
		!firstUser.Parts[1].Dynamic || firstUser.Parts[1].CacheControl != nil || firstUser.Parts[2].Text != "整理这份" {
		t.Fatalf("expected file, dynamic context, question with the boundary on the file, got %#v", firstUser)
	}
	cachedPrefix := append(cloneLLMMessages(first.Messages[:len(first.Messages)-1]), stableUserMessage(firstUser))
	for index, want := range cachedPrefix {
		got := stableUserMessage(second.Messages[index])
		if !samePromptMessage(got, stableUserMessage(want)) {
			t.Fatalf("cached prefix drifted at message %d:\nfirst:  %#v\nsecond: %#v", index, want, got)
		}
	}

	secondUser := second.Messages[len(second.Messages)-1]
	last := secondUser.Parts[len(secondUser.Parts)-1]
	if !strings.HasPrefix(secondUser.Parts[0].Text, "<documents>\n<document source=\"pasted-text-2.txt\">") || last.Text != "继续" {
		t.Fatalf("new file must lead the current turn and the question must close it, got %#v", secondUser)
	}
	for _, message := range second.Messages[:len(second.Messages)-1] {
		for _, part := range message.Parts {
			if strings.Contains(part.Text, "新粘贴全文") {
				t.Fatalf("new file must not change earlier messages, got %#v", message)
			}
		}
	}
}

// samePromptMessage 比较两条消息发送给模型的内容（忽略缓存提示）。
func samePromptMessage(left llm.Message, right llm.Message) bool {
	if left.Role != right.Role || left.Content != right.Content || len(left.Parts) != len(right.Parts) {
		return false
	}
	for index := range left.Parts {
		a, b := left.Parts[index], right.Parts[index]
		if a.Kind != b.Kind || a.Text != b.Text || a.Dynamic != b.Dynamic || string(a.Data) != string(b.Data) {
			return false
		}
	}
	return true
}
