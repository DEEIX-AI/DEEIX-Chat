package conversation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

type promptStateFingerprintInput struct {
	Protocol          string
	Endpoint          string
	UpstreamID        uint
	UpstreamModel     string
	PlatformModelName string
	ContextConfig     string
	ContextState      string
	Messages          []llm.Message
	Tools             []llm.ToolDefinition
	Options           map[string]interface{}
}

// buildPromptStateFingerprint 计算本地认为的上游会话状态指纹。
func buildPromptStateFingerprint(input promptStateFingerprintInput) string {
	hasher := sha256.New()
	writeFingerprintField(hasher, "protocol", input.Protocol)
	writeFingerprintField(hasher, "endpoint", input.Endpoint)
	writeFingerprintField(hasher, "upstream_id", fmt.Sprintf("%d", input.UpstreamID))
	writeFingerprintField(hasher, "upstream_model", input.UpstreamModel)
	writeFingerprintField(hasher, "platform_model_name", input.PlatformModelName)
	writeFingerprintField(hasher, "context_config", input.ContextConfig)
	writeFingerprintField(hasher, "context_state", input.ContextState)
	for _, message := range input.Messages {
		writeFingerprintField(hasher, "message_role", message.Role)
		writeFingerprintField(hasher, "message_content", message.Content)
		for _, part := range message.Parts {
			writeFingerprintField(hasher, "part_kind", part.Kind)
			writeFingerprintField(hasher, "part_text", part.Text)
			writeFingerprintField(hasher, "part_mime", part.MimeType)
			writeFingerprintField(hasher, "part_file", part.FileName)
			if len(part.Data) > 0 {
				dataHash := sha256.Sum256(part.Data)
				writeFingerprintField(hasher, "part_data", hex.EncodeToString(dataHash[:]))
			}
		}
		for _, call := range message.ToolCalls {
			writeFingerprintField(hasher, "tool_call_id", call.ToolCallID)
			writeFingerprintField(hasher, "tool_call_type", call.ToolType)
			writeFingerprintField(hasher, "tool_call_name", call.ToolName)
			writeFingerprintField(hasher, "tool_call_args", call.ArgumentsJSON)
			writeFingerprintField(hasher, "tool_call_status", call.Status)
			writeFingerprintField(hasher, "tool_call_output", call.OutputJSON)
			writeFingerprintField(hasher, "tool_call_error", call.ErrorJSON)
		}
		for _, result := range message.ToolResults {
			writeFingerprintField(hasher, "tool_result_id", result.ToolCallID)
			writeFingerprintField(hasher, "tool_result_name", result.ToolName)
			writeFingerprintField(hasher, "tool_result_output", result.OutputJSON)
			writeFingerprintField(hasher, "tool_result_status", result.Status)
			writeFingerprintField(hasher, "tool_result_error", result.Error)
		}
	}
	for _, tool := range normalizedToolDefinitions(input.Tools) {
		writeFingerprintField(hasher, "tool_name", tool.Name)
		writeFingerprintField(hasher, "tool_desc", tool.Description)
		writeFingerprintField(hasher, "tool_schema", string(tool.InputSchema))
	}
	if len(input.Options) > 0 {
		if payload, err := json.Marshal(input.Options); err == nil {
			writeFingerprintField(hasher, "options", string(payload))
		}
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// buildPromptContextStateSignature 显式记录稳定上下文来源版本，补足纯文本指纹无法表达的状态边界。
func buildPromptContextStateSignature(stableAttachments []AttachmentInput, preferenceMemories []domainmemory.UserMemory) string {
	payload := map[string]interface{}{
		"files":    promptContextFileState(stableAttachments),
		"memories": promptContextMemoryState(preferenceMemories),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func promptContextFileState(attachments []AttachmentInput) []map[string]interface{} {
	if len(attachments) == 0 {
		return nil
	}
	items := make([]AttachmentInput, 0, len(attachments))
	for _, item := range attachments {
		if normalizeAttachmentKind(item.Kind, item.MimeType) == "image" {
			continue
		}
		if strings.TrimSpace(item.ExtractedText) == "" {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return stableAttachmentSortKey(items[i]) < stableAttachmentSortKey(items[j])
	})
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]interface{}{
			"file_id":        strings.TrimSpace(item.FileID),
			"sha256":         strings.TrimSpace(item.SHA256),
			"extract_status": strings.TrimSpace(item.ExtractStatus),
			"embed_status":   strings.TrimSpace(item.EmbedStatus),
			"chunk_count":    item.ChunkCount,
			"context_mode":   strings.TrimSpace(item.ContextMode),
			"text_hash":      promptTextHash(item.ExtractedText),
		})
	}
	return result
}

func promptContextMemoryState(memories []domainmemory.UserMemory) []map[string]interface{} {
	if len(memories) == 0 {
		return nil
	}
	items := make([]domainmemory.UserMemory, len(memories))
	copy(items, memories)
	sort.SliceStable(items, func(i, j int) bool {
		left := strings.TrimSpace(items[i].Scope) + ":" + strings.TrimSpace(items[i].MemoryKey)
		right := strings.TrimSpace(items[j].Scope) + ":" + strings.TrimSpace(items[j].MemoryKey)
		return left < right
	})
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.MemoryKey)
		value := strings.TrimSpace(item.Value)
		if key == "" || value == "" {
			continue
		}
		result = append(result, map[string]interface{}{
			"key":        key,
			"scope":      strings.TrimSpace(item.Scope),
			"value_hash": promptTextHash(value),
			"updated_at": promptTimeUnixNano(item.UpdatedAt),
		})
	}
	return result
}

func promptTextHash(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:])
}

func promptTimeUnixNano(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixNano()
}

// buildPromptContextConfigSignature 提取会影响上下文规划和证据含义的稳定配置。
// 这些配置变化时，即使历史文本暂时相同，也应让 previous_response_id 失效并重建上游状态。
func buildPromptContextConfigSignature(cfg config.Config) string {
	payload := map[string]interface{}{
		"embedding_enabled":           cfg.EmbeddingEnabled,
		"embedding_output_dimensions": cfg.EmbeddingOutputDimensions,
		"embedding_normalize":         cfg.EmbeddingNormalize,
		"embedding_model_signature":   strings.TrimSpace(cfg.EmbeddingModelSignature),
		"rag_enabled":                 cfg.RAGEnabled,
		"rag_model":                   strings.TrimSpace(cfg.RAGModel),
		"rag_min_similarity":          cfg.RAGMinSimilarity,
		"rag_token_budget":            cfg.RAGTokenBudget,
		"rag_query_history_turns":     cfg.RAGQueryHistoryTurns,
		"context_compact_trigger":     cfg.ContextCompactTrigger,
		"context_compact_preserve":    cfg.ContextCompactPreserve,
		"message_embedding_enabled":   cfg.MessageEmbeddingEnabled,
		"semantic_context_enabled":    cfg.SemanticContextEnabled,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// promptStatePrefixMessages 返回当前用户输入之前、本地认为已经在上游状态中的消息。
func promptStatePrefixMessages(messages []llm.Message) []llm.Message {
	lastUserIndex := -1
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role == "user" {
			lastUserIndex = index
			break
		}
	}
	if lastUserIndex < 0 {
		return cloneLLMMessages(messages)
	}
	return cloneLLMMessages(messages[:lastUserIndex])
}

func appendAssistantStateMessage(messages []llm.Message, assistantText string) []llm.Message {
	result := cloneLLMMessages(messages)
	result = append(result, llm.Message{Role: "assistant", Content: assistantText})
	return result
}

// buildNextStatefulPrefixMessages 生成下一轮本地可重建的状态前缀。
// 当前轮真实发送给上游的 user 可能包含文件、RAG、图片等动态 XML；数据库历史只保存用户原文。
// 因此保存 previous_response_id 指纹时使用“稳定前缀 + 用户原文 + 助手回复”，避免下一轮因历史 XML 不可重建而误判失效。
func buildNextStatefulPrefixMessages(messages []llm.Message, currentUserContent string, assistantText string) []llm.Message {
	lastUserIndex := -1
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role == "user" {
			lastUserIndex = index
			break
		}
	}
	if lastUserIndex < 0 {
		return appendAssistantStateMessage(messages, assistantText)
	}
	result := cloneLLMMessages(messages[:lastUserIndex])
	result = append(result, llm.Message{Role: "user", Content: currentUserContent})
	result = append(result, llm.Message{Role: "assistant", Content: assistantText})
	return result
}

func writeFingerprintField(hasher interface{ Write([]byte) (int, error) }, key string, value string) {
	key = strings.TrimSpace(key)
	_, _ = hasher.Write([]byte(key))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write([]byte(fmt.Sprintf("%d", len(value))))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write([]byte(value))
	_, _ = hasher.Write([]byte{0})
}

func normalizedToolDefinitions(tools []llm.ToolDefinition) []llm.ToolDefinition {
	if len(tools) == 0 {
		return nil
	}
	result := make([]llm.ToolDefinition, len(tools))
	copy(result, tools)
	sort.SliceStable(result, func(i, j int) bool {
		left := strings.TrimSpace(result[i].Name)
		right := strings.TrimSpace(result[j].Name)
		if left == right {
			return strings.TrimSpace(result[i].Description) < strings.TrimSpace(result[j].Description)
		}
		return left < right
	})
	return result
}
