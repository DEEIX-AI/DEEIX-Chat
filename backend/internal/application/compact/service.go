package compact

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

// LLMSummarizerFunc 是由调用方注入的 LLM 摘要函数。
// platformModelName 用于路由选择，messages 是待压缩的消息列表，prompt 是摘要指令。
// 失败时返回非 nil error，调用方应降级到下一回退级别。
type LLMSummarizerFunc func(ctx context.Context, platformModelName string, messages []domainconversation.Message, prompt string) (string, error)

// Service 封装会话压缩能力。
type Service struct {
	cfg    *config.Runtime
	repo   repository.CompactRepository
	logger *zap.Logger

	// LLM 语义压缩（可选，由 conversation.Service 注入）
	mu                     sync.RWMutex
	llmSummarizer          LLMSummarizerFunc
	consecutiveLLMFailures int32 // atomic：连续 LLM 压缩失败次数，用于熔断
	lastLLMFailureAt       int64 // atomic：最近一次 LLM 失败的 Unix 纳秒时间，用于时间窗口自恢复
}

// NewService 创建上下文压缩服务。
func NewService(cfg config.Config, repo repository.CompactRepository, logger *zap.Logger) *Service {
	return NewServiceWithRuntime(config.NewRuntime(cfg), repo, logger)
}

// NewServiceWithRuntime 创建使用运行时配置容器的上下文压缩服务。
func NewServiceWithRuntime(cfg *config.Runtime, repo repository.CompactRepository, logger *zap.Logger) *Service {
	return &Service{
		cfg:    cfg,
		repo:   repo,
		logger: logger,
	}
}

// SetLLMSummarizer 注入 LLM 摘要函数（由 conversation.Service 在初始化后调用）。
// 线程安全，可在服务运行中替换。
func (s *Service) SetLLMSummarizer(fn LLMSummarizerFunc) {
	s.mu.Lock()
	s.llmSummarizer = fn
	s.mu.Unlock()
}

func (s *Service) getLLMSummarizer() LLMSummarizerFunc {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.llmSummarizer
}

// llmCircuitClosed 返回 true 表示 LLM 压缩熔断器未触发，可以尝试 LLM 压缩。
// 自动恢复：若距最后一次失败超过 5 分钟，重置计数器（冷却期结束）。
func (s *Service) llmCircuitClosed() bool {
	const recoveryWindow = 5 * time.Minute
	lastFailNs := atomic.LoadInt64(&s.lastLLMFailureAt)
	if lastFailNs > 0 && time.Since(time.Unix(0, lastFailNs)) > recoveryWindow {
		atomic.StoreInt32(&s.consecutiveLLMFailures, 0)
		atomic.StoreInt64(&s.lastLLMFailureAt, 0)
	}
	cfg := s.snapshot()
	maxFailures := cfg.CompactMaxFailures
	if maxFailures <= 0 {
		maxFailures = 3
	}
	return atomic.LoadInt32(&s.consecutiveLLMFailures) < int32(maxFailures)
}

// ResolveContextMessageLimit 返回祖先链查询上限。
func (s *Service) ResolveContextMessageLimit() int {
	limit := s.snapshot().MaxContextMessages * 2
	if limit <= 0 {
		return 40
	}
	if limit > 200 {
		return 200
	}
	return limit
}

// MaybeCompactConversation 根据配置判断是否压缩会话上下文。
// platformModelName 用于 Token 感知的阈值判断与 LLM 路由选择。
func (s *Service) MaybeCompactConversation(
	ctx context.Context,
	conversationID uint,
	runID string,
	messageCount int,
	platformModelName string,
) error {
	if s == nil || s.repo == nil {
		return nil
	}

	cfg := s.snapshot()
	maxTurns := cfg.ContextMaxTurns
	triggerTokens := cfg.ContextCompactTrigger
	if maxTurns <= 0 && triggerTokens <= 0 {
		return nil
	}

	turns := messageCount / 2
	totalTokens, err := s.repo.SumMessageTokens(ctx, conversationID)
	if err != nil {
		return err
	}

	strategy := ""
	switch {
	case maxTurns > 0 && turns > maxTurns:
		strategy = "turn_cap"
	case triggerTokens > 0 && totalTokens > int64(triggerTokens):
		strategy = "token_cap"
	default:
		return nil
	}

	preserveTurns := cfg.ContextCompactPreserve
	if preserveTurns <= 0 {
		preserveTurns = 8
	}
	if turns <= preserveTurns {
		return nil
	}

	// ── 复用已有 snapshot（若存在则跳过 LLM）──────────────────────
	// 已有的 snapshot 摘要已经覆盖了早期轮次，因此直接更新 LastCompactedAt 并返回，无需再次调用 LLM。
	if existing, existErr := s.repo.GetLatestContextSnapshot(ctx, conversationID); existErr == nil && existing != nil {
		if s.logger != nil {
			s.logger.Info("compact_reuse_snapshot",
				zap.Uint("conversation_id", conversationID),
				zap.Int("snapshot_to_turn", existing.ToTurn),
			)
		}
		_ = s.repo.UpdateConversationCompactedAt(ctx, conversationID, time.Now())
		return nil
	}

	// ── Level 1-3：LLM snapshot（现有流程）──────────────────────────
	fromTurn := 1
	toTurn := turns - preserveTurns
	if toTurn < fromTurn {
		return nil
	}

	summaryText := s.buildCompactionSummary(ctx, conversationID, strategy, fromTurn, toTurn, preserveTurns, platformModelName)
	summaryTokens := estimateTokens(summaryText)
	snapshotUserID, snapshotMessageID := s.resolveSnapshotOwner(ctx, conversationID, messageCount)
	if err = s.repo.CreateContextSnapshot(ctx, &domainconversation.ContextSnapshot{
		ConversationID: conversationID,
		MessageID:      snapshotMessageID,
		UserID:         snapshotUserID,
		RunID:          runID,
		FromTurn:       fromTurn,
		ToTurn:         toTurn,
		SourceTokens:   totalTokens,
		SummaryTokens:  summaryTokens,
		SummaryText:    summaryText,
		Strategy:       strategy,
	}); err != nil {
		return err
	}

	if err = s.repo.UpdateConversationCompactedAt(ctx, conversationID, time.Now()); err != nil {
		return err
	}

	return nil
}

func (s *Service) resolveSnapshotOwner(ctx context.Context, conversationID uint, messageCount int) (uint, uint) {
	offset := messageCount - 1
	if offset < 0 {
		offset = 0
	}
	messages, _, err := s.repo.ListMessages(ctx, conversationID, offset, 1)
	if (err != nil || len(messages) == 0) && offset > 0 {
		messages, _, err = s.repo.ListMessages(ctx, conversationID, 0, 1)
	}
	if err != nil || len(messages) == 0 {
		return 0, 0
	}
	return messages[0].UserID, messages[0].ID
}

// GetLatestSnapshot 返回最近一次上下文压缩快照。
func (s *Service) GetLatestSnapshot(ctx context.Context, conversationID uint) (*domainconversation.ContextSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.GetLatestContextSnapshot(ctx, conversationID)
}

// GetSnapshotByRunID 按运行 ID 返回压缩快照。
func (s *Service) GetSnapshotByRunID(ctx context.Context, runID string) (*domainconversation.ContextSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.GetContextSnapshotByRunID(ctx, runID)
}

// buildCompactionSummary 使用 4 级回退链生成压缩摘要：
//
//	Level 3 (LLM 全量)  → Level 2 (LLM 轻量) → Level 1 (增强模板) → Level 0 (空串，依赖截断)
func (s *Service) buildCompactionSummary(
	ctx context.Context,
	conversationID uint,
	strategy string,
	fromTurn int,
	toTurn int,
	preserveTurns int,
	platformModelName string,
) string {
	if s == nil || s.repo == nil {
		return fmt.Sprintf(
			"context compaction summary unavailable, strategy=%s compact_range=%d-%d preserve_recent=%d",
			strategy, fromTurn, toTurn, preserveTurns,
		)
	}

	maxMessages := toTurn * 2
	if maxMessages <= 0 {
		maxMessages = 20
	}
	if maxMessages > 120 {
		maxMessages = 120
	}

	messages, _, err := s.repo.ListMessages(ctx, conversationID, 0, maxMessages)
	if err != nil || len(messages) == 0 {
		return fmt.Sprintf(
			"context compaction summary unavailable, strategy=%s compact_range=%d-%d preserve_recent=%d",
			strategy, fromTurn, toTurn, preserveTurns,
		)
	}

	cfg := s.snapshot()

	// ── Level 3 & 2：LLM 语义压缩 ──────────────────────────────
	if cfg.CompactLLMEnabled {
		if summarizer := s.getLLMSummarizer(); summarizer != nil && s.llmCircuitClosed() {
			// Level 3：全量消息 + 完整摘要提示（优先使用可配置提示词）
			fullPrompt := resolveCompactPrompt(cfg.CompactSystemPrompt, fromTurn, toTurn, compactPromptFull)
			if result, llmErr := summarizer(ctx, platformModelName, messages, fullPrompt); llmErr == nil && strings.TrimSpace(result) != "" {
				atomic.StoreInt32(&s.consecutiveLLMFailures, 0)
				return result
			}

			// Level 2：近半消息 + 轻量提示
			liteStart := len(messages) / 2
			liteMessages := messages[liteStart:]
			if len(liteMessages) > 0 {
				litePrompt := resolveCompactPrompt(cfg.CompactLightPrompt, fromTurn, toTurn, compactPromptLite)
				if result, llmErr := summarizer(ctx, platformModelName, liteMessages, litePrompt); llmErr == nil && strings.TrimSpace(result) != "" {
					atomic.StoreInt32(&s.consecutiveLLMFailures, 0)
					return result
				}
			}

			// 两级 LLM 均失败，累计熔断计数，记录失败时间用于自恢复
			atomic.StoreInt64(&s.lastLLMFailureAt, time.Now().UnixNano())
			newCount := atomic.AddInt32(&s.consecutiveLLMFailures, 1)
			if s.logger != nil {
				s.logger.Warn("compact_llm_all_failed",
					zap.String("strategy", strategy),
					zap.Int32("consecutive_failures", newCount),
					zap.Int("max_failures", func() int {
						if cfg.CompactMaxFailures > 0 {
							return cfg.CompactMaxFailures
						}
						return 3
					}()),
				)
			}
		}
	}

	// ── Level 1：增强模板摘要 ────────────────────────────────────
	return s.buildTemplateCompactSummary(messages, strategy, fromTurn, toTurn, preserveTurns, cfg)
}

// buildTemplateCompactSummary 是 Level 1 的增强模板回退，结构清晰、无需 LLM。
func (s *Service) buildTemplateCompactSummary(
	messages []domainconversation.Message,
	strategy string,
	fromTurn int,
	toTurn int,
	preserveTurns int,
	cfg config.Config,
) string {
	highlightLimit := cfg.ContextCompactHighlightsPerRole
	if highlightLimit <= 0 {
		highlightLimit = 6
	}
	snippetChars := cfg.ContextCompactSnippetChars
	if snippetChars <= 0 {
		snippetChars = 140
	}

	userHighlights := collectRoleHighlights(messages, "user", highlightLimit, snippetChars)
	assistantHighlights := collectRoleHighlights(messages, "assistant", highlightLimit, snippetChars)

	lines := make([]string, 0, 6+len(userHighlights)+len(assistantHighlights))
	lines = append(lines, fmt.Sprintf("## Conversation Context Summary"))
	lines = append(lines, fmt.Sprintf("Compaction strategy: %s | Turns compressed: %d–%d | Recent %d turns preserved in full.", strategy, fromTurn, toTurn, preserveTurns))
	lines = append(lines, "")
	if len(userHighlights) > 0 {
		lines = append(lines, "**User intents & requests:**")
		for _, item := range userHighlights {
			lines = append(lines, "- "+item)
		}
		lines = append(lines, "")
	}
	if len(assistantHighlights) > 0 {
		lines = append(lines, "**Assistant key responses:**")
		for _, item := range assistantHighlights {
			lines = append(lines, "- "+item)
		}
	}
	return strings.Join(lines, "\n")
}

// compactPromptFull 返回 Level 3 全量摘要提示词（~9 个章节）。
func compactPromptFull(fromTurn, toTurn int) string {
	return fmt.Sprintf(`You are compressing conversation history (turns %d–%d) for context management.
Create a comprehensive but concise summary covering:

1. **Primary Topic & Goal**: What was the user trying to accomplish?
2. **Key Technical Details**: Important code snippets, configs, decisions, file names.
3. **Problems & Solutions**: Errors encountered and how they were resolved.
4. **Current State**: Where things stood at the end of this section.
5. **Open Items**: Unresolved questions or pending tasks.
6. **User Preferences**: Any stated preferences or constraints to remember.

Rules:
- Keep under 800 tokens total.
- Be specific with technical details (exact names, values, paths).
- Omit pleasantries and redundant exchanges.
- Output only the summary, no preamble.`, fromTurn, toTurn)
}

// compactPromptLite 返回 Level 2 轻量摘要提示词（快速、低成本）。
func compactPromptLite(fromTurn, toTurn int) string {
	return fmt.Sprintf(`Briefly summarize this conversation segment (turns %d–%d).
Cover: (1) main topic, (2) key outcomes or decisions, (3) current status.
Max 300 tokens. Output only the summary.`, fromTurn, toTurn)
}

func (s *Service) snapshot() config.Config {
	if s == nil || s.cfg == nil {
		return config.Config{}
	}
	return s.cfg.Snapshot()
}

func collectRoleHighlights(messages []domainconversation.Message, role string, limit int, snippetChars int) []string {
	if limit <= 0 {
		limit = 4
	}
	highlights := make([]string, 0, limit)
	for _, item := range messages {
		if item.Role != role {
			continue
		}
		snippet := compactSnippet(item.Content, snippetChars)
		if snippet == "" {
			continue
		}
		highlights = append(highlights, snippet)
		if len(highlights) >= limit {
			break
		}
	}
	return highlights
}

func compactSnippet(content string, maxLen int) string {
	value := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if value == "" {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 120
	}
	runes := []rune(value)
	if len(runes) <= maxLen {
		return value
	}
	return string(runes[:maxLen]) + "..."
}

func estimateTokens(content string) int64 {
	if len(content) == 0 {
		return 0
	}
	var cjk, other int64
	for _, r := range content {
		if isCJKRune(r) {
			cjk++
		} else {
			other++
		}
	}
	tokens := (cjk*2+2)/3 + (other+3)/4
	if tokens == 0 {
		return 1
	}
	return tokens
}

// resolveCompactPrompt 返回实际使用的压缩提示词。
// 若 customTpl 非空，替换占位符后使用；否则调用 defaultFn 生成内置提示词。
func resolveCompactPrompt(customTpl string, fromTurn, toTurn int, defaultFn func(int, int) string) string {
	tpl := strings.TrimSpace(customTpl)
	if tpl == "" {
		return defaultFn(fromTurn, toTurn)
	}
	tpl = strings.ReplaceAll(tpl, "{{FROM_TURN}}", fmt.Sprintf("%d", fromTurn))
	tpl = strings.ReplaceAll(tpl, "{{TO_TURN}}", fmt.Sprintf("%d", toTurn))
	return tpl
}

func isCJKRune(r rune) bool {
	return (r >= 0x2E80 && r <= 0x9FFF) ||
		(r >= 0xAC00 && r <= 0xD7AF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0x20000 && r <= 0x2A6DF)
}

// BuildSnapshotSystemPrompt 生成供模型消费的压缩快照系统提示。
func BuildSnapshotSystemPrompt(summary string, fromTurn int, toTurn int, strategy string) string {
	normalizedSummary := strings.TrimSpace(summary)
	if normalizedSummary == "" {
		return ""
	}
	return fmt.Sprintf(
		"Conversation history summary (strategy=%s, turns=%d-%d):\n%s",
		strings.TrimSpace(strategy),
		fromTurn,
		toTurn,
		normalizedSummary,
	)
}
