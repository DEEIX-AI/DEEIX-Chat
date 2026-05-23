package conversation

import (
	"fmt"
	"strings"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

// ContextSlotKind 标识上下文槽位类型。
type ContextSlotKind string

const (
	SlotSystemPrompt   ContextSlotKind = "system_prompt"
	SlotPreference     ContextSlotKind = "preference"
	SlotMemory         ContextSlotKind = "memory"
	SlotSnapshot       ContextSlotKind = "snapshot"
	SlotRAG            ContextSlotKind = "rag"
	SlotSemanticRecall ContextSlotKind = "semantic_recall"
	SlotHistory        ContextSlotKind = "history"
	SlotInput          ContextSlotKind = "input"
)

// slotPriority 优先级硬编码：数值越大越优先保留。
var slotPriority = map[ContextSlotKind]int{
	SlotInput:          100,
	SlotSystemPrompt:   95,
	SlotPreference:     90,
	SlotSnapshot:       80,
	SlotRAG:            70,
	SlotSemanticRecall: 60,
	SlotMemory:         50,
	SlotHistory:        40, // 历史轮次基础优先级；逐轮递减
}

// ContextSlot 代表一个注入片段。
type ContextSlot struct {
	Kind       ContextSlotKind
	Content    string
	TokenCount int64
	Priority   int
	Required   bool // true = 预算不足时也不裁剪
}

// SlotTrimmed 记录被裁剪的槽位（用于调试和追踪）。
type SlotTrimmed struct {
	Kind   ContextSlotKind
	Reason string
}

// AssemblyTrace 装配过程的追踪信息。
type AssemblyTrace struct {
	TotalBudget  int64
	UsedTokens   int64
	TrimmedSlots []SlotTrimmed
}

// ContextAssembler 按优先级和 Token 预算将各槽位装配为 LLM 消息序列。
type ContextAssembler struct {
	budget       int64
	slots        []ContextSlot
	seenChunkIDs map[string]bool
}

// NewContextAssembler 创建装配器，budget 为最大允许 token 数（0 表示不限）。
func NewContextAssembler(budget int64) *ContextAssembler {
	return &ContextAssembler{
		budget:       budget,
		seenChunkIDs: make(map[string]bool),
	}
}

// DeduplicateRAGChunks 过滤掉已见过（fileID+chunkIndex 相同）的 chunk，并将新 chunk 标记为已见。
// 对 RAG 结果和语义召回结果分别调用，可在请求粒度内去重。
func (a *ContextAssembler) DeduplicateRAGChunks(chunks []model.RAGChunk) []model.RAGChunk {
	if len(chunks) == 0 {
		return chunks
	}
	result := make([]model.RAGChunk, 0, len(chunks))
	for _, c := range chunks {
		key := fmt.Sprintf("%s/%d", c.FileID, c.ChunkIndex)
		if a.seenChunkIDs[key] {
			continue
		}
		a.seenChunkIDs[key] = true
		result = append(result, c)
	}
	return result
}

// Add 添加一个槽位。Priority 为 0 时使用 slotPriority 默认值。
func (a *ContextAssembler) Add(slot ContextSlot) {
	if slot.Priority == 0 {
		slot.Priority = slotPriority[slot.Kind]
	}
	if slot.TokenCount == 0 && slot.Content != "" {
		slot.TokenCount = estimateTokens(slot.Content)
	}
	a.slots = append(a.slots, slot)
}

// Assemble 将所有槽位按优先级装配成 system 消息序列（由高到低排列）。
// history 槽位原样传入 historyMessages；返回最终合并的消息列表及追踪信息。
//
// 装配规则：
//  1. Required=true 的槽位先占预算
//  2. 剩余预算按 Priority 从高到低分配非 Required 槽位
//  3. 超预算的槽位记入 AssemblyTrace.TrimmedSlots
//
// 返回的 messages 中 system 消息按优先级从高到低排列，之后跟 historyMessages。
func (a *ContextAssembler) Assemble(historyMessages []llm.Message) ([]llm.Message, AssemblyTrace) {
	trace := AssemblyTrace{TotalBudget: a.budget}

	// 历史消息 token 估算（始终计入，后续可按优先级裁剪 — 目前简单保留全部）
	var historyTokens int64
	for _, m := range historyMessages {
		historyTokens += estimateMessageTokens(m)
	}

	// 按优先级排序槽位（Required 优先，同等 required 则按 Priority 降序）
	sorted := make([]ContextSlot, len(a.slots))
	copy(sorted, a.slots)
	sortSlots(sorted)

	// 预算分配：先处理 Required 槽位
	var usedByRequired int64
	for _, s := range sorted {
		if s.Required {
			usedByRequired += s.TokenCount
		}
	}

	remaining := a.budget - usedByRequired - historyTokens
	if a.budget <= 0 {
		remaining = 1<<62 - 1 // 无限制
	}

	// 收集本次要注入的系统消息（按 Priority 由高到低）
	var systemMsgs []llm.Message
	for _, s := range sorted {
		content := strings.TrimSpace(s.Content)
		if content == "" {
			continue
		}
		if s.Required {
			systemMsgs = append(systemMsgs, llm.Message{Role: "system", Content: content})
			trace.UsedTokens += s.TokenCount
			continue
		}
		if remaining >= s.TokenCount {
			systemMsgs = append(systemMsgs, llm.Message{Role: "system", Content: content})
			remaining -= s.TokenCount
			trace.UsedTokens += s.TokenCount
		} else {
			trace.TrimmedSlots = append(trace.TrimmedSlots, SlotTrimmed{
				Kind:   s.Kind,
				Reason: "budget_exceeded",
			})
		}
	}
	trace.UsedTokens += historyTokens

	// 最终消息：system 前缀 + 历史消息
	result := make([]llm.Message, 0, len(systemMsgs)+len(historyMessages))
	result = append(result, systemMsgs...)
	result = append(result, historyMessages...)
	return result, trace
}

// sortSlots 原地按（Required DESC, Priority DESC）排序。
func sortSlots(slots []ContextSlot) {
	for i := 1; i < len(slots); i++ {
		key := slots[i]
		j := i - 1
		for j >= 0 && less(slots[j], key) {
			slots[j+1] = slots[j]
			j--
		}
		slots[j+1] = key
	}
}

// less 返回 true 表示 b 应排在 a 前面（即 a < b 在排序中）。
func less(a, b ContextSlot) bool {
	if a.Required != b.Required {
		return b.Required // b 是 required → b 更优先 → a < b
	}
	return a.Priority < b.Priority
}
