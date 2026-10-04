package llm

import (
	"encoding/base64"
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func nativeDocumentTestMessage() portllm.Message {
	return portllm.Message{Role: "user", Parts: []portllm.ContentPart{
		{Kind: portllm.ContentPartFile, Text: `<documents><document source="report.pdf" access="native"/></documents>`},
		{Kind: portllm.ContentPartDocument, MimeType: "application/pdf", Data: []byte("%PDF-1.7"), FileName: "report.pdf", PageCount: 2,
			CacheControl: &portllm.CacheControl{Type: "ephemeral"}},
		{Kind: portllm.ContentPartText, Text: "总结这份报告"},
	}}
}

var nativeDocumentTestBase64 = base64.StdEncoding.EncodeToString([]byte("%PDF-1.7"))

func TestAnthropicContentEncodesNativeDocument(t *testing.T) {
	content, marked := buildAnthropicContent(nativeDocumentTestMessage(), func(*portllm.CacheControl) map[string]any {
		return map[string]any{"type": "ephemeral"}
	})
	blocks := content.([]map[string]any)
	if len(blocks) != 3 || !marked {
		t.Fatalf("expected text, document and text blocks, got %#v", blocks)
	}
	document := blocks[1]
	source, _ := document["source"].(map[string]any)
	if document["type"] != "document" || document["title"] != "report.pdf" || document["cache_control"] == nil ||
		source["type"] != "base64" || source["media_type"] != "application/pdf" || source["data"] != nativeDocumentTestBase64 {
		t.Fatalf("unexpected anthropic document block: %#v", document)
	}
}

func TestGeminiPartsEncodeNativeDocumentInline(t *testing.T) {
	parts := buildGeminiParts(nativeDocumentTestMessage())
	if len(parts) != 3 {
		t.Fatalf("expected three gemini parts, got %#v", parts)
	}
	inline, _ := parts[1]["inlineData"].(map[string]any)
	if inline["mimeType"] != "application/pdf" || inline["data"] != nativeDocumentTestBase64 {
		t.Fatalf("unexpected gemini inline document: %#v", parts[1])
	}
}

func TestOpenAIResponsesEncodesNativeDocumentAsInputFile(t *testing.T) {
	parts := buildResponsesAPIContent(nativeDocumentTestMessage(), &openAIPromptCacheConfig{Explicit: true})
	if len(parts) != 3 {
		t.Fatalf("expected three responses parts, got %#v", parts)
	}
	file := parts[1]
	if file["type"] != "input_file" || file["filename"] != "report.pdf" ||
		file["file_data"] != "data:application/pdf;base64,"+nativeDocumentTestBase64 || file["prompt_cache_breakpoint"] == nil {
		t.Fatalf("unexpected input_file block: %#v", file)
	}

	assistant := nativeDocumentTestMessage()
	assistant.Role = "assistant"
	for _, part := range buildResponsesAPIContent(assistant, nil) {
		if part["type"] == "input_file" {
			t.Fatalf("assistant messages must not carry input files: %#v", part)
		}
	}
}

func TestChatCompletionsEncodesNativeDocumentAsFilePart(t *testing.T) {
	parts, ok := buildChatCompletionsContent(nativeDocumentTestMessage(), nil).([]map[string]any)
	if !ok || len(parts) != 3 {
		t.Fatalf("documents must keep the content array form, got %#v", parts)
	}
	file, _ := parts[1]["file"].(map[string]any)
	if parts[1]["type"] != "file" || file["filename"] != "report.pdf" || file["file_data"] != "data:application/pdf;base64,"+nativeDocumentTestBase64 {
		t.Fatalf("unexpected chat completions file part: %#v", parts[1])
	}
}

func nativeMediaTestMessage() portllm.Message {
	return portllm.Message{Role: "user", Parts: []portllm.ContentPart{
		{Kind: portllm.ContentPartAudio, MimeType: "audio/mp3", Data: []byte("ID3"), FileName: "voice.mp3"},
		{Kind: portllm.ContentPartVideo, MimeType: "video/mp4", Data: []byte("ftyp"), FileName: "clip.mp4"},
		{Kind: portllm.ContentPartText, Text: "描述这些内容"},
	}}
}

func TestGeminiPartsEncodeNativeMediaInline(t *testing.T) {
	parts := buildGeminiParts(nativeMediaTestMessage())
	if len(parts) != 3 {
		t.Fatalf("expected three gemini parts, got %#v", parts)
	}
	audio, _ := parts[0]["inlineData"].(map[string]any)
	video, _ := parts[1]["inlineData"].(map[string]any)
	if audio["mimeType"] != "audio/mp3" || audio["data"] != base64.StdEncoding.EncodeToString([]byte("ID3")) ||
		video["mimeType"] != "video/mp4" || video["data"] != base64.StdEncoding.EncodeToString([]byte("ftyp")) {
		t.Fatalf("unexpected gemini inline media: %#v", parts)
	}
}

func TestChatCompletionsEncodesNativeMedia(t *testing.T) {
	parts, ok := buildChatCompletionsContent(nativeMediaTestMessage(), nil).([]map[string]any)
	if !ok || len(parts) != 3 {
		t.Fatalf("media must keep the content array form, got %#v", parts)
	}
	audio, _ := parts[0]["input_audio"].(map[string]any)
	if parts[0]["type"] != "input_audio" || audio["format"] != "mp3" || audio["data"] != base64.StdEncoding.EncodeToString([]byte("ID3")) {
		t.Fatalf("unexpected input_audio part: %#v", parts[0])
	}
	video, _ := parts[1]["video_url"].(map[string]any)
	if parts[1]["type"] != "video_url" || video["url"] != "data:video/mp4;base64,"+base64.StdEncoding.EncodeToString([]byte("ftyp")) {
		t.Fatalf("unexpected video_url part: %#v", parts[1])
	}
}
