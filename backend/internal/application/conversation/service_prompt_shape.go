package conversation

import (
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type promptShape struct {
	Mode               string
	MessageCount       int
	SystemCount        int
	UserCount          int
	AssistantCount     int
	ToolCount          int
	TotalTokens        int64
	LeadingSystem      int64
	LastUserTokens     int64
	HasUserContext     bool
	HasFiles           bool
	HasEvidence        bool
	HasRAG             bool
	HasSummary         bool
	HasMemory          bool
	HasRecall          bool
	PreviousResponse   bool
	FullMessageCount   int
	FullPromptTokens   int64
	StatefulSavedMsgs  int
	StatefulSavedToken int64
}

func summarizePromptShape(mode string, sent []llm.Message, full []llm.Message, previousResponseID string) promptShape {
	shape := promptShape{
		Mode:             strings.TrimSpace(mode),
		MessageCount:     len(sent),
		PreviousResponse: strings.TrimSpace(previousResponseID) != "",
		FullMessageCount: len(full),
		FullPromptTokens: estimatePromptTokens(full),
	}
	if shape.Mode == "" {
		if shape.PreviousResponse {
			shape.Mode = "stateful"
		} else {
			shape.Mode = "full"
		}
	}
	shape.TotalTokens = estimatePromptTokens(sent)

	leadingSystem := true
	for _, message := range sent {
		msgTokens := estimateMessageTokens(message)
		switch message.Role {
		case "system":
			shape.SystemCount++
			if leadingSystem {
				shape.LeadingSystem += msgTokens
			}
		case "user":
			shape.UserCount++
			shape.LastUserTokens = msgTokens
		case "assistant":
			shape.AssistantCount++
			leadingSystem = false
		case "tool":
			shape.ToolCount++
			leadingSystem = false
		default:
			leadingSystem = false
		}
		if message.Role != "system" {
			leadingSystem = false
		}
		if message.Role == "user" {
			content := message.Content
			if len(message.Parts) > 0 {
				var parts strings.Builder
				for _, part := range message.Parts {
					if part.Kind == llm.ContentPartText || part.Kind == llm.ContentPartFile {
						parts.WriteString(part.Text)
						parts.WriteString("\n")
					}
				}
				content = parts.String()
			}
			shape.HasUserContext = shape.HasUserContext || strings.Contains(content, "<ctx>")
			shape.HasFiles = shape.HasFiles || strings.Contains(content, "<files>")
			shape.HasEvidence = shape.HasEvidence || strings.Contains(content, "<evs>")
			shape.HasRAG = shape.HasRAG || strings.Contains(content, "<rag>")
			shape.HasSummary = shape.HasSummary || strings.Contains(content, "<sum")
			shape.HasMemory = shape.HasMemory || strings.Contains(content, "<mems>")
			shape.HasRecall = shape.HasRecall || strings.Contains(content, "<recall>")
		}
	}
	if len(full) > 0 {
		shape.StatefulSavedMsgs = len(full) - len(sent)
		shape.StatefulSavedToken = estimatePromptTokens(full) - shape.TotalTokens
		if shape.StatefulSavedMsgs < 0 {
			shape.StatefulSavedMsgs = 0
		}
		if shape.StatefulSavedToken < 0 {
			shape.StatefulSavedToken = 0
		}
	}
	return shape
}

func promptShapeTraceAttributes(prefix string, shape promptShape) []attribute.KeyValue {
	key := func(name string) string {
		if strings.TrimSpace(prefix) == "" {
			return name
		}
		return prefix + "." + name
	}
	return []attribute.KeyValue{
		attribute.String(key("mode"), shape.Mode),
		attribute.Bool(key("previous_response"), shape.PreviousResponse),
		attribute.Int(key("message_count"), shape.MessageCount),
		attribute.Int(key("full_message_count"), shape.FullMessageCount),
		attribute.Int64(key("tokens"), shape.TotalTokens),
		attribute.Int64(key("full_tokens"), shape.FullPromptTokens),
		attribute.Int64(key("leading_system_tokens"), shape.LeadingSystem),
		attribute.Int64(key("last_user_tokens"), shape.LastUserTokens),
		attribute.Int(key("system_count"), shape.SystemCount),
		attribute.Int(key("user_count"), shape.UserCount),
		attribute.Int(key("assistant_count"), shape.AssistantCount),
		attribute.Int(key("tool_count"), shape.ToolCount),
		attribute.Bool(key("has_ctx"), shape.HasUserContext),
		attribute.Bool(key("has_files"), shape.HasFiles),
		attribute.Bool(key("has_evidence"), shape.HasEvidence),
		attribute.Bool(key("has_rag"), shape.HasRAG),
		attribute.Bool(key("has_summary"), shape.HasSummary),
		attribute.Bool(key("has_memory"), shape.HasMemory),
		attribute.Bool(key("has_recall"), shape.HasRecall),
		attribute.Int(key("stateful_saved_messages"), shape.StatefulSavedMsgs),
		attribute.Int64(key("stateful_saved_tokens"), shape.StatefulSavedToken),
	}
}

func promptShapeLogFields(shape promptShape) []zap.Field {
	return []zap.Field{
		zap.String("prompt_mode", shape.Mode),
		zap.Bool("previous_response", shape.PreviousResponse),
		zap.Int("message_count", shape.MessageCount),
		zap.Int("full_message_count", shape.FullMessageCount),
		zap.Int64("prompt_tokens_estimated", shape.TotalTokens),
		zap.Int64("full_prompt_tokens_estimated", shape.FullPromptTokens),
		zap.Int64("last_user_tokens_estimated", shape.LastUserTokens),
		zap.Bool("has_ctx", shape.HasUserContext),
		zap.Bool("has_files", shape.HasFiles),
		zap.Bool("has_evidence", shape.HasEvidence),
		zap.Bool("has_rag", shape.HasRAG),
		zap.Bool("has_summary", shape.HasSummary),
		zap.Bool("has_memory", shape.HasMemory),
		zap.Bool("has_recall", shape.HasRecall),
		zap.Int("stateful_saved_messages", shape.StatefulSavedMsgs),
		zap.Int64("stateful_saved_tokens_estimated", shape.StatefulSavedToken),
	}
}
