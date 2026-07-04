package conversation

import (
	"context"
	"fmt"
	"strings"

	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/skill"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

// PromptBlockKind 标识 PromptPlan 中每一类上下文块。
type PromptBlockKind string

const (
	PromptBlockSystemPolicy       PromptBlockKind = "system_policy"
	PromptBlockStableContext      PromptBlockKind = "stable_context"
	PromptBlockHistoricalEvidence PromptBlockKind = "historical_evidence"
	PromptBlockDynamicContext     PromptBlockKind = "dynamic_context"
	PromptBlockSkillContext       PromptBlockKind = "skill_context"
	PromptBlockToolGuidance       PromptBlockKind = "tool_guidance"
	PromptBlockTranscript         PromptBlockKind = "transcript"
)

// PromptBlockTrace 描述一次请求中单个上下文块的规划结果。
type PromptBlockTrace struct {
	Kind          PromptBlockKind
	Title         string
	TokenEstimate int64
	Cacheable     bool
	SourceCount   int
	SourceRefs    []PromptSourceRef
}

// PromptSourceRef 描述 PromptPlan 块的来源引用。
type PromptSourceRef struct {
	SourceType string
	SourceID   string
	Title      string
	ArtifactID uint
}

// PromptTrace 汇总本轮发送给上游前的上下文形态。
type PromptTrace struct {
	Blocks             []PromptBlockTrace
	TotalTokenEstimate int64
}

// PromptPlan 是对话请求发送前的唯一上下文规划结果。
type PromptPlan struct {
	Messages []llm.Message
	Trace    PromptTrace
}

type promptPlanInput struct {
	BaseMessages      []llm.Message
	StableAttachments []AttachmentInput
	DynamicContext    userContextInput
	SkillPrompts      *skillPrompts
	ToolRuntime       selectedToolRuntime
	Config            config.Config
	StoreProvider     appstorage.Provider
}

// buildPromptPlan 按稳定上下文、动态上下文、工具规则的固定顺序生成最终上游消息。
func buildPromptPlan(ctx context.Context, input promptPlanInput) PromptPlan {
	messages := cloneLLMMessages(input.BaseMessages)
	trace := PromptTrace{}

	before := len(messages)
	messages = prependStableFileContext(messages, input.StableAttachments)
	if len(messages) > before {
		sourceRefs := stableAttachmentSourceRefs(input.StableAttachments, input.DynamicContext.CurrentArtifacts)
		trace.addBlock(PromptBlockTrace{
			Kind:          PromptBlockStableContext,
			Title:         "稳定文件上下文",
			TokenEstimate: estimateMessageTokens(messages[0]),
			Cacheable:     true,
			SourceCount:   len(sourceRefs),
			SourceRefs:    sourceRefs,
		})
	}
	systemPolicyCount := countMessagesByRole(input.BaseMessages, "system")
	if systemPolicyCount > 0 {
		trace.addBlock(PromptBlockTrace{
			Kind:          PromptBlockSystemPolicy,
			Title:         "系统策略",
			TokenEstimate: estimateMessagesByRole(input.BaseMessages, "system"),
			Cacheable:     true,
			SourceCount:   systemPolicyCount,
		})
	}
	trace.addBlock(PromptBlockTrace{
		Kind:          PromptBlockTranscript,
		Title:         "历史对话",
		TokenEstimate: estimateTranscriptTokens(input.BaseMessages),
		Cacheable:     false,
		SourceCount:   countMessagesByRole(input.BaseMessages, "user") + countMessagesByRole(input.BaseMessages, "assistant"),
	})

	beforeMessages := cloneLLMMessages(messages)
	messages = injectUserContext(ctx, messages, input.DynamicContext, input.Config, input.StoreProvider)
	if promptMessagesChanged(beforeMessages, messages) {
		historicalTokenEstimate := estimateContextArtifactsTokens(input.DynamicContext.HistoricalArtifacts)
		if len(input.DynamicContext.HistoricalArtifacts) > 0 {
			sourceRefs := historicalArtifactSourceRefs(input.DynamicContext.HistoricalArtifacts)
			trace.addBlock(PromptBlockTrace{
				Kind:          PromptBlockHistoricalEvidence,
				Title:         "历史证据",
				TokenEstimate: historicalTokenEstimate,
				Cacheable:     false,
				SourceCount:   len(sourceRefs),
				SourceRefs:    sourceRefs,
			})
		}
		dynamicTokenEstimate := estimatePromptTokens(messages) - estimatePromptTokens(beforeMessages) - historicalTokenEstimate
		dynamicSourceRefs := dynamicContextSourceRefs(input.DynamicContext)
		if len(dynamicSourceRefs) > 0 {
			trace.addBlock(PromptBlockTrace{
				Kind:          PromptBlockDynamicContext,
				Title:         "本轮动态上下文",
				TokenEstimate: dynamicTokenEstimate,
				Cacheable:     false,
				SourceCount:   len(dynamicSourceRefs),
				SourceRefs:    dynamicSourceRefs,
			})
		}
	}

	before = len(messages)
	messages = injectSkillPrompts(messages, input.SkillPrompts)
	if len(messages) > before && input.SkillPrompts != nil {
		inserted := findSkillPromptMessage(messages)
		tokenEstimate := int64(0)
		if inserted >= 0 {
			tokenEstimate = estimateMessageTokens(messages[inserted])
		}
		trace.addBlock(PromptBlockTrace{
			Kind:          PromptBlockSkillContext,
			Title:         "Skill 上下文",
			TokenEstimate: tokenEstimate,
			Cacheable:     true,
			SourceCount:   len(input.SkillPrompts.Skills),
			SourceRefs:    skillPromptSourceRefs(input.SkillPrompts.Skills),
		})
	}

	before = len(messages)
	messages = injectMCPToolGuidance(messages, input.ToolRuntime, input.Config.MCPToolPrompt)
	if len(messages) > before {
		inserted := findToolGuidanceMessage(messages)
		tokenEstimate := int64(0)
		if inserted >= 0 {
			tokenEstimate = estimateMessageTokens(messages[inserted])
		}
		trace.addBlock(PromptBlockTrace{
			Kind:          PromptBlockToolGuidance,
			Title:         "工具使用规则",
			TokenEstimate: tokenEstimate,
			Cacheable:     true,
			SourceCount:   len(input.ToolRuntime.definitions),
			SourceRefs:    toolDefinitionSourceRefs(input.ToolRuntime.definitions),
		})
	}
	messages = markLeadingSystemMessagesCacheable(messages)

	trace.TotalTokenEstimate = estimatePromptTokens(messages)
	return PromptPlan{Messages: messages, Trace: trace}
}

// addBlock 规范化并追加单个上下文块 trace。
func (t *PromptTrace) addBlock(block PromptBlockTrace) {
	if block.TokenEstimate < 0 {
		block.TokenEstimate = 0
	}
	if block.SourceCount < 0 {
		block.SourceCount = 0
	}
	t.Blocks = append(t.Blocks, block)
}

// cloneLLMMessages 复制消息切片，避免规划过程修改调用方持有的切片头。
func cloneLLMMessages(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return nil
	}
	result := make([]llm.Message, len(messages))
	copy(result, messages)
	return result
}

// countMessagesByRole 统计指定角色消息数量，用于 PromptTrace 来源计数。
func countMessagesByRole(messages []llm.Message, role string) int {
	count := 0
	for _, item := range messages {
		if item.Role == role {
			count++
		}
	}
	return count
}

// estimateMessagesByRole 估算指定角色消息的 token 数。
func estimateMessagesByRole(messages []llm.Message, role string) int64 {
	var total int64
	for _, item := range messages {
		if item.Role == role {
			total += estimateMessageTokens(item)
		}
	}
	return total
}

// estimateTranscriptTokens 只估算真实对话轮次，不把 system 策略混进历史对话。
func estimateTranscriptTokens(messages []llm.Message) int64 {
	var total int64
	for _, item := range messages {
		if item.Role == "user" || item.Role == "assistant" {
			total += estimateMessageTokens(item)
		}
	}
	return total
}

// countStableTextAttachments 统计可进入稳定文本上下文的附件数量。
func countStableTextAttachments(attachments []AttachmentInput) int {
	count := 0
	for _, att := range attachments {
		if !isStableTextAttachment(att) {
			continue
		}
		count++
	}
	return count
}

// stableAttachmentSourceRefs 提取稳定全文文件的来源引用。
func stableAttachmentSourceRefs(attachments []AttachmentInput, currentArtifacts []domainconversation.ContextArtifact) []PromptSourceRef {
	refs := make([]PromptSourceRef, 0, countStableTextAttachments(attachments))
	fallbackArtifacts := contextArtifactsByKindAndSourceID(currentArtifacts, domainconversation.ContextArtifactFileRAGFallback)
	for _, att := range attachments {
		if !isStableTextAttachment(att) {
			continue
		}
		sourceID := stableAttachmentSourceID(att)
		if artifact, ok := fallbackArtifacts[fallbackFileSourceID(att)]; ok {
			refs = appendPromptSourceRefWithArtifactID(refs, string(domainconversation.ContextArtifactFileRAGFallback), sourceID, firstNonEmptyString(att.FileName, artifact.SourceTitle), artifact.ID)
			continue
		}
		refs = appendPromptSourceRef(refs, "file_full", sourceID, att.FileName)
	}
	return refs
}

// dynamicContextSourceRefs 提取本轮动态上下文的来源引用。
func dynamicContextSourceRefs(input userContextInput) []PromptSourceRef {
	refs := make([]PromptSourceRef, 0, len(input.RAGChunks)+len(input.RecallChunks)+len(input.Memory)+len(input.Attachments)+1)
	ragArtifacts := contextArtifactsByKindAndSourceID(input.CurrentArtifacts, domainconversation.ContextArtifactFileRAGChunk)
	recallArtifacts := contextArtifactsByKindAndSourceID(input.CurrentArtifacts, domainconversation.ContextArtifactSemanticRecall)
	memoryArtifacts := contextArtifactsByKindAndSourceID(input.CurrentArtifacts, domainconversation.ContextArtifactUserMemory)
	for _, chunk := range input.RAGChunks {
		artifact := ragArtifacts[fileRAGChunkSourceID(chunk)]
		refs = appendPromptSourceRefWithArtifactID(refs, string(domainconversation.ContextArtifactFileRAGChunk), chunk.FileID, ragChunkSourceTitle(chunk), artifact.ID)
	}
	for _, chunk := range input.RecallChunks {
		sourceID := messageChunkSourceID(chunk)
		artifact := recallArtifacts[sourceID]
		refs = appendPromptSourceRefWithArtifactID(refs, string(domainconversation.ContextArtifactSemanticRecall), sourceID, chunk.Role, artifact.ID)
	}
	for _, memory := range input.Memory {
		sourceID := strings.TrimSpace(memory.MemoryKey)
		artifact := memoryArtifacts[sourceID]
		refs = appendPromptSourceRefWithArtifactID(refs, string(domainconversation.ContextArtifactUserMemory), sourceID, memory.Scope, artifact.ID)
	}
	if input.Snapshot != nil && strings.TrimSpace(input.Snapshot.Summary) != "" {
		refs = appendPromptSourceRef(refs, "summary", input.Snapshot.Strategy, "上下文摘要")
	}
	for _, att := range input.Attachments {
		refs = appendPromptSourceRef(refs, "image", stableAttachmentSourceID(att), att.FileName)
	}
	return refs
}

// contextArtifactsByKindAndSourceID 按类型和来源 ID 建立索引，用于把已落库证据回填到 PromptTrace 来源。
func contextArtifactsByKindAndSourceID(artifacts []domainconversation.ContextArtifact, kind domainconversation.ContextArtifactKind) map[string]domainconversation.ContextArtifact {
	result := make(map[string]domainconversation.ContextArtifact)
	for _, artifact := range artifacts {
		if artifact.Kind != kind {
			continue
		}
		sourceID := strings.TrimSpace(artifact.SourceID)
		if sourceID == "" || artifact.ID == 0 {
			continue
		}
		result[sourceID] = artifact
	}
	return result
}

func ragChunkSourceTitle(chunk domainconversation.RAGChunk) string {
	title := strings.TrimSpace(chunk.FileName)
	if title == "" {
		title = strings.TrimSpace(chunk.FileID)
	}
	if chunk.ChunkIndex >= 0 {
		return fmt.Sprintf("%s #%d", title, chunk.ChunkIndex+1)
	}
	return title
}

// historicalArtifactSourceRefs 提取历史证据 artifact 的来源引用。
func historicalArtifactSourceRefs(artifacts []domainconversation.ContextArtifact) []PromptSourceRef {
	refs := make([]PromptSourceRef, 0, len(artifacts))
	for _, artifact := range artifacts {
		refs = appendPromptSourceRefWithArtifactID(refs, string(artifact.Kind), artifact.SourceID, artifact.SourceTitle, artifact.ID)
	}
	return refs
}

// skillPromptSourceRefs 提取本轮可用 Skill 的来源引用。
func skillPromptSourceRefs(skills []domainskill.Skill) []PromptSourceRef {
	refs := make([]PromptSourceRef, 0, len(skills))
	for _, skill := range skills {
		refs = appendPromptSourceRef(refs, "skill", fmt.Sprintf("%d", skill.ID), skill.Title)
	}
	return refs
}

// toolDefinitionSourceRefs 提取本轮可用工具定义的来源引用。
func toolDefinitionSourceRefs(tools []llm.ToolDefinition) []PromptSourceRef {
	refs := make([]PromptSourceRef, 0, len(tools))
	for _, tool := range tools {
		refs = appendPromptSourceRef(refs, "tool", tool.Name, tool.Name)
	}
	return refs
}

// appendPromptSourceRef 追加非空来源引用，保持 trace payload 干净。
func appendPromptSourceRef(refs []PromptSourceRef, sourceType string, sourceID string, title string) []PromptSourceRef {
	return appendPromptSourceRefWithArtifactID(refs, sourceType, sourceID, title, 0)
}

// appendPromptSourceRefWithArtifactID 追加非空来源引用，并携带可追溯的 artifact 主键。
func appendPromptSourceRefWithArtifactID(refs []PromptSourceRef, sourceType string, sourceID string, title string, artifactID uint) []PromptSourceRef {
	sourceType = strings.TrimSpace(sourceType)
	sourceID = strings.TrimSpace(sourceID)
	title = strings.TrimSpace(title)
	if sourceType == "" && sourceID == "" && title == "" && artifactID == 0 {
		return refs
	}
	return append(refs, PromptSourceRef{
		SourceType: sourceType,
		SourceID:   sourceID,
		Title:      title,
		ArtifactID: artifactID,
	})
}

// stableAttachmentSourceID 选择文件来源的稳定标识，优先使用业务 fileID。
func stableAttachmentSourceID(att AttachmentInput) string {
	if id := strings.TrimSpace(att.FileID); id != "" {
		return id
	}
	if att.FileObjID > 0 {
		return fmt.Sprintf("%d", att.FileObjID)
	}
	if sha := strings.TrimSpace(att.SHA256); sha != "" {
		return sha
	}
	return strings.TrimSpace(att.FileName)
}

// messageChunkSourceID 生成消息语义分片的来源标识。
func messageChunkSourceID(chunk domainconversation.MessageChunk) string {
	if chunk.MessageID > 0 {
		return fmt.Sprintf("%d:%d", chunk.MessageID, chunk.ChunkIndex)
	}
	return fmt.Sprintf("%s:%d", strings.TrimSpace(chunk.Role), chunk.ChunkIndex)
}

// estimateContextArtifactsTokens 估算历史证据块的 token 消耗。
func estimateContextArtifactsTokens(artifacts []domainconversation.ContextArtifact) int64 {
	var total int64
	for _, artifact := range artifacts {
		if artifact.TokenEstimate > 0 {
			total += artifact.TokenEstimate
			continue
		}
		total += estimateTokens(artifact.Content)
	}
	return total
}

// promptMessagesChanged 判断规划步骤是否改变了最终上游消息形态。
func promptMessagesChanged(left []llm.Message, right []llm.Message) bool {
	if len(left) != len(right) {
		return true
	}
	for i := range left {
		if left[i].Role != right[i].Role || left[i].Content != right[i].Content || len(left[i].Parts) != len(right[i].Parts) {
			return true
		}
	}
	return false
}

// findToolGuidanceMessage 定位工具使用规则消息，供 trace 估算 token。
func findToolGuidanceMessage(messages []llm.Message) int {
	for i, item := range messages {
		if item.Role == "system" && strings.HasPrefix(strings.TrimSpace(item.Content), "# tool_use") {
			return i
		}
	}
	return -1
}

// markLeadingSystemMessagesCacheable 给稳定 system 前缀加块级缓存提示。
func markLeadingSystemMessagesCacheable(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return messages
	}
	result := cloneLLMMessages(messages)
	cacheableIndices := make([]int, 0, 4)
	for index := range result {
		if result[index].Role != "system" {
			break
		}
		if strings.TrimSpace(result[index].Content) == "" && len(result[index].Parts) == 0 {
			continue
		}
		cacheableIndices = append(cacheableIndices, index)
	}
	if len(cacheableIndices) > 4 {
		cacheableIndices = cacheableIndices[len(cacheableIndices)-4:]
	}
	for _, index := range cacheableIndices {
		result[index].CacheControl = &llm.CacheControl{Type: "ephemeral"}
	}
	return result
}
