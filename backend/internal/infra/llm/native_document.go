package llm

import (
	"encoding/base64"
	"strings"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// 原生文件内容块（portllm.ContentPartDocument / Audio / Video）在各协议中的公共字段。
// 只有会话层原生输入策略允许的协议会收到它们，内容一律 base64 内嵌在请求体中：
//   - PDF：Anthropic document、Gemini inlineData、OpenAI Responses input_file、Chat Completions file；
//   - 音频：Gemini inlineData、Chat Completions input_audio（OpenAI、OpenRouter）；
//   - 视频：Gemini inlineData、OpenRouter Chat Completions video_url。

const defaultNativeDocumentMIME = "application/pdf"

func nativeDocumentMIME(part portllm.ContentPart) string {
	if mime := strings.TrimSpace(part.MimeType); mime != "" {
		return mime
	}
	return defaultNativeDocumentMIME
}

// nativeDocumentFileName 返回 OpenAI 文件内容块要求的文件名；缺失时按 MIME 给出占位名。
func nativeDocumentFileName(part portllm.ContentPart) string {
	if name := strings.TrimSpace(part.FileName); name != "" {
		return name
	}
	return "document.pdf"
}

func nativeDocumentDataURL(part portllm.ContentPart) string {
	return "data:" + nativeDocumentMIME(part) + ";base64," + base64.StdEncoding.EncodeToString(part.Data)
}

// nativeAudioFormat 返回 input_audio 的格式名（MIME 子类型，如 audio/mp3 → mp3），缺失时按 wav 处理。
func nativeAudioFormat(part portllm.ContentPart) string {
	mime := strings.ToLower(strings.TrimSpace(part.MimeType))
	if format := strings.TrimPrefix(mime, "audio/"); format != "" && format != mime {
		return format
	}
	return "wav"
}
