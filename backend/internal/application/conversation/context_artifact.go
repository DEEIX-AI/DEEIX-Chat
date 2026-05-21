package conversation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

const (
	contextArtifactExcerptChars       = 2000
	historicalArtifactScanLimit       = 30
	historicalArtifactDefaultMaxItems = 5
	historicalArtifactDefaultMaxToken = 1200
)

type promptContextArtifactInput struct {
	ConversationID uint
	UserID         uint
	MessageID      uint
	RunID          string
	Query          string
	RAGChunks      []domainconversation.RAGChunk
	RAGFallbacks   []ragFallbackEvidence
	RecallChunks   []domainconversation.MessageChunk
	Memories       []domainmemory.UserMemory
}

type toolContextArtifactInput struct {
	ConversationID uint
	UserID         uint
	MessageID      uint
	RunID          string
	Rows           []domainconversation.ToolCall
}

type snapshotContextArtifactInput struct {
	ConversationID uint
	UserID         uint
	MessageID      uint
	RunID          string
	Snapshot       *domainconversation.ContextSnapshot
}

type historicalContextArtifactInput struct {
	CurrentMessageID uint
	Query            string
	Candidates       []domainconversation.ContextArtifact
	CurrentRAGChunks []domainconversation.RAGChunk
	CurrentFallbacks []AttachmentInput
	CurrentRecall    []domainconversation.MessageChunk
	MaxItems         int
	MaxTokens        int64
}

type historicalScoredArtifact struct {
	item  domainconversation.ContextArtifact
	score int
	index int
}

type ragFallbackEvidence struct {
	Attachment AttachmentInput
	Reason     string
	Error      string
}

// GetContextArtifact 查询当前用户可访问的上下文证据详情。
func (s *Service) GetContextArtifact(ctx context.Context, userID uint, artifactID uint) (*domainconversation.ContextArtifact, error) {
	if artifactID == 0 {
		return nil, ErrContextArtifactNotFound
	}
	item, err := s.repo.GetContextArtifactByIDForUser(ctx, userID, artifactID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrContextArtifactNotFound
		}
		return nil, err
	}
	return item, nil
}

// persistPromptContextArtifacts 保存本轮被 PromptPlan 消费的证据，并返回已回填主键的证据列表。
func (s *Service) persistPromptContextArtifacts(ctx context.Context, input promptContextArtifactInput) []domainconversation.ContextArtifact {
	items := buildPromptContextArtifacts(input)
	if len(items) == 0 {
		return nil
	}
	s.applyContextArtifactRetention(items)
	if err := s.repo.CreateContextArtifacts(ctx, items); err != nil {
		if s.logger != nil {
			s.logger.Warn("context_artifact_persist_failed",
				zap.String("trace_id", traceid.FromContext(ctx)),
				zap.Uint("conversation_id", input.ConversationID),
				zap.Uint("message_id", input.MessageID),
				zap.Error(err),
			)
		}
		return nil
	}
	return items
}

// persistToolContextArtifacts 保存工具执行结果证据，供后续轮次按 evidence 召回。
func (s *Service) persistToolContextArtifacts(ctx context.Context, input toolContextArtifactInput) {
	items := buildToolContextArtifacts(input)
	if len(items) == 0 {
		return
	}
	s.applyContextArtifactRetention(items)
	if err := s.repo.CreateContextArtifacts(ctx, items); err != nil && s.logger != nil {
		s.logger.Warn("tool_context_artifact_persist_failed",
			zap.String("trace_id", traceid.FromContext(ctx)),
			zap.Uint("conversation_id", input.ConversationID),
			zap.Uint("message_id", input.MessageID),
			zap.Error(err),
		)
	}
}

// persistSnapshotContextArtifact 保存压缩摘要证据，供未来 PromptPlan 解释和召回。
func (s *Service) persistSnapshotContextArtifact(ctx context.Context, input snapshotContextArtifactInput) {
	item := buildSnapshotContextArtifact(input)
	if item == nil {
		return
	}
	items := []domainconversation.ContextArtifact{*item}
	s.applyContextArtifactRetention(items)
	if err := s.repo.CreateContextArtifacts(ctx, items); err != nil && s.logger != nil {
		s.logger.Warn("snapshot_context_artifact_persist_failed",
			zap.String("trace_id", traceid.FromContext(ctx)),
			zap.Uint("conversation_id", input.ConversationID),
			zap.Uint("message_id", input.MessageID),
			zap.Error(err),
		)
	}
}

// applyContextArtifactRetention 给新证据写入过期时间；过期策略只影响证据表，不影响原始消息。
func (s *Service) applyContextArtifactRetention(items []domainconversation.ContextArtifact) {
	if len(items) == 0 || s == nil || s.cfg == nil {
		return
	}
	days := s.cfg.Snapshot().ContextArtifactRetentionDays
	if days <= 0 {
		return
	}
	expiresAt := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	for index := range items {
		items[index].ExpiresAt = &expiresAt
	}
}

// recallHistoricalContextArtifacts 读取近期上下文证据并按当前问题筛选。
func (s *Service) recallHistoricalContextArtifacts(
	ctx context.Context,
	conversationID uint,
	currentMessageID uint,
	query string,
	currentRAGChunks []domainconversation.RAGChunk,
	currentFallbacks []AttachmentInput,
	currentRecall []domainconversation.MessageChunk,
) []domainconversation.ContextArtifact {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	kinds := []domainconversation.ContextArtifactKind{
		domainconversation.ContextArtifactFileRAGChunk,
		domainconversation.ContextArtifactFileRAGFallback,
		domainconversation.ContextArtifactToolResult,
		domainconversation.ContextArtifactNativeTool,
		domainconversation.ContextArtifactSummary,
	}
	candidates, err := s.repo.ListRecentContextArtifacts(ctx, conversationID, kinds, historicalArtifactScanLimit)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("historical_context_artifact_recall_failed",
				zap.String("trace_id", traceid.FromContext(ctx)),
				zap.Uint("conversation_id", conversationID),
				zap.Error(err),
			)
		}
		return nil
	}
	return selectHistoricalContextArtifacts(historicalContextArtifactInput{
		CurrentMessageID: currentMessageID,
		Query:            query,
		Candidates:       candidates,
		CurrentRAGChunks: currentRAGChunks,
		CurrentFallbacks: currentFallbacks,
		CurrentRecall:    currentRecall,
	})
}

// buildPromptContextArtifacts 将 RAG、全文回退和语义召回统一转换为上下文证据。
func buildPromptContextArtifacts(input promptContextArtifactInput) []domainconversation.ContextArtifact {
	items := make([]domainconversation.ContextArtifact, 0, len(input.RAGChunks)+len(input.RAGFallbacks)+len(input.RecallChunks)+len(input.Memories))
	for _, chunk := range input.RAGChunks {
		content := strings.TrimSpace(chunk.Content)
		if content == "" {
			continue
		}
		sourceID := fileRAGChunkSourceID(chunk)
		items = append(items, domainconversation.ContextArtifact{
			ConversationID: input.ConversationID,
			MessageID:      input.MessageID,
			UserID:         input.UserID,
			RunID:          input.RunID,
			Kind:           domainconversation.ContextArtifactFileRAGChunk,
			SourceType:     "file_chunk",
			SourceID:       sourceID,
			SourceTitle:    strings.TrimSpace(chunk.FileName),
			Content:        contextArtifactExcerpt(content, contextArtifactExcerptChars),
			ContentHash:    contextArtifactHash(domainconversation.ContextArtifactFileRAGChunk, sourceID, content),
			TokenEstimate:  estimateTokens(content),
			Score:          float64(chunk.Score),
			MetadataJSON: contextArtifactMetadata(map[string]interface{}{
				"query":       strings.TrimSpace(input.Query),
				"file_id":     strings.TrimSpace(chunk.FileID),
				"chunk_index": chunk.ChunkIndex,
				"score":       chunk.Score,
			}),
		})
	}

	for _, fallback := range input.RAGFallbacks {
		file := fallback.Attachment
		content := strings.TrimSpace(file.ExtractedText)
		if content == "" {
			continue
		}
		sourceID := fallbackFileSourceID(file)
		items = append(items, domainconversation.ContextArtifact{
			ConversationID: input.ConversationID,
			MessageID:      input.MessageID,
			UserID:         input.UserID,
			RunID:          input.RunID,
			Kind:           domainconversation.ContextArtifactFileRAGFallback,
			SourceType:     "file",
			SourceID:       sourceID,
			SourceTitle:    strings.TrimSpace(file.FileName),
			Content:        contextArtifactExcerpt(content, contextArtifactExcerptChars),
			ContentHash:    contextArtifactHash(domainconversation.ContextArtifactFileRAGFallback, sourceID, content),
			TokenEstimate:  estimateTokens(content),
			MetadataJSON: contextArtifactMetadata(map[string]interface{}{
				"query":          strings.TrimSpace(input.Query),
				"reason":         strings.TrimSpace(fallback.Reason),
				"error":          strings.TrimSpace(fallback.Error),
				"file_id":        strings.TrimSpace(file.FileID),
				"file_obj_id":    file.FileObjID,
				"sha256":         strings.TrimSpace(file.SHA256),
				"chunk_count":    file.ChunkCount,
				"embed_status":   strings.TrimSpace(file.EmbedStatus),
				"extract_status": strings.TrimSpace(file.ExtractStatus),
			}),
		})
	}

	for _, memory := range input.Memories {
		key := strings.TrimSpace(memory.MemoryKey)
		content := strings.TrimSpace(memory.Value)
		if key == "" || content == "" {
			continue
		}
		scope := strings.TrimSpace(memory.Scope)
		items = append(items, domainconversation.ContextArtifact{
			ConversationID: input.ConversationID,
			MessageID:      input.MessageID,
			UserID:         input.UserID,
			RunID:          input.RunID,
			Kind:           domainconversation.ContextArtifactUserMemory,
			SourceType:     "user_memory",
			SourceID:       key,
			SourceTitle:    firstNonEmptyString(scope, key),
			Content:        contextArtifactExcerpt(content, contextArtifactExcerptChars),
			ContentHash:    contextArtifactHash(domainconversation.ContextArtifactUserMemory, key, content),
			TokenEstimate:  estimateTokens(content),
			Score:          1,
			MetadataJSON: contextArtifactMetadata(map[string]interface{}{
				"memory_key": strings.TrimSpace(memory.MemoryKey),
				"scope":      scope,
				"updated_by": strings.TrimSpace(memory.UpdatedBy),
			}),
		})
	}

	for _, chunk := range input.RecallChunks {
		content := strings.TrimSpace(chunk.Content)
		if content == "" {
			continue
		}
		sourceID := fmt.Sprintf("%d:%d", chunk.MessageID, chunk.ChunkIndex)
		items = append(items, domainconversation.ContextArtifact{
			ConversationID: input.ConversationID,
			MessageID:      input.MessageID,
			UserID:         input.UserID,
			RunID:          input.RunID,
			Kind:           domainconversation.ContextArtifactSemanticRecall,
			SourceType:     "message_chunk",
			SourceID:       sourceID,
			SourceTitle:    chunk.Role,
			Content:        contextArtifactExcerpt(content, contextArtifactExcerptChars),
			ContentHash:    contextArtifactHash(domainconversation.ContextArtifactSemanticRecall, sourceID, content),
			TokenEstimate:  estimateTokens(content),
			Score:          chunk.Similarity,
			MetadataJSON: contextArtifactMetadata(map[string]interface{}{
				"source_message_id": chunk.MessageID,
				"chunk_index":       chunk.ChunkIndex,
				"role":              strings.TrimSpace(chunk.Role),
				"similarity":        chunk.Similarity,
			}),
		})
	}
	return items
}

// buildToolContextArtifacts 将工具调用结果转换为可召回的上下文证据。
func buildToolContextArtifacts(input toolContextArtifactInput) []domainconversation.ContextArtifact {
	items := make([]domainconversation.ContextArtifact, 0, len(input.Rows))
	for _, row := range input.Rows {
		content := toolArtifactContent(row)
		if strings.TrimSpace(content) == "" {
			continue
		}
		sourceID := strings.TrimSpace(row.ToolCallID)
		if sourceID == "" {
			sourceID = strings.TrimSpace(row.ToolName)
		}
		kind := toolContextArtifactKind(row)
		items = append(items, domainconversation.ContextArtifact{
			ConversationID: input.ConversationID,
			MessageID:      input.MessageID,
			UserID:         input.UserID,
			RunID:          input.RunID,
			Kind:           kind,
			SourceType:     "tool_call",
			SourceID:       sourceID,
			SourceTitle:    strings.TrimSpace(row.ToolName),
			Content:        contextArtifactExcerpt(content, contextArtifactExcerptChars),
			ContentHash:    contextArtifactHash(kind, sourceID, content),
			TokenEstimate:  estimateTokens(content),
			Score:          1,
			MetadataJSON: contextArtifactMetadata(map[string]interface{}{
				"tool_call_id": strings.TrimSpace(row.ToolCallID),
				"tool_type":    strings.TrimSpace(row.ToolType),
				"tool_name":    strings.TrimSpace(row.ToolName),
				"status":       strings.TrimSpace(row.Status),
				"latency_ms":   row.LatencyMS,
				"input":        strings.TrimSpace(row.InputJSON),
			}),
		})
	}
	return items
}

// buildSnapshotContextArtifact 将压缩快照转换为历史 evidence。
func buildSnapshotContextArtifact(input snapshotContextArtifactInput) *domainconversation.ContextArtifact {
	if input.Snapshot == nil || strings.TrimSpace(input.Snapshot.SummaryText) == "" {
		return nil
	}
	sourceID := fmt.Sprintf("%d", input.Snapshot.ID)
	if input.Snapshot.ID == 0 {
		sourceID = strings.TrimSpace(input.Snapshot.RunID)
	}
	if sourceID == "" {
		sourceID = strings.TrimSpace(input.RunID)
	}
	title := fmt.Sprintf("上下文摘要 %d-%d", input.Snapshot.FromTurn, input.Snapshot.ToTurn)
	content := strings.TrimSpace(input.Snapshot.SummaryText)
	tokenEstimate := input.Snapshot.SummaryTokens
	if tokenEstimate <= 0 {
		tokenEstimate = estimateTokens(content)
	}
	return &domainconversation.ContextArtifact{
		ConversationID: input.ConversationID,
		MessageID:      input.MessageID,
		UserID:         input.UserID,
		RunID:          firstNonEmptyString(input.Snapshot.RunID, input.RunID),
		Kind:           domainconversation.ContextArtifactSummary,
		SourceType:     "context_snapshot",
		SourceID:       sourceID,
		SourceTitle:    title,
		Content:        contextArtifactExcerpt(content, contextArtifactExcerptChars),
		ContentHash:    contextArtifactHash(domainconversation.ContextArtifactSummary, sourceID, content),
		TokenEstimate:  tokenEstimate,
		Score:          1,
		MetadataJSON: contextArtifactMetadata(map[string]interface{}{
			"from_turn":      input.Snapshot.FromTurn,
			"to_turn":        input.Snapshot.ToTurn,
			"source_tokens":  input.Snapshot.SourceTokens,
			"summary_tokens": input.Snapshot.SummaryTokens,
			"strategy":       strings.TrimSpace(input.Snapshot.Strategy),
		}),
	}
}

// selectHistoricalContextArtifacts 从近期证据中选择与当前问题相关的少量历史证据。
func selectHistoricalContextArtifacts(input historicalContextArtifactInput) []domainconversation.ContextArtifact {
	if len(input.Candidates) == 0 {
		return nil
	}
	maxItems := input.MaxItems
	if maxItems <= 0 {
		maxItems = historicalArtifactDefaultMaxItems
	}
	maxTokens := input.MaxTokens
	if maxTokens <= 0 {
		maxTokens = historicalArtifactDefaultMaxToken
	}
	terms := artifactQueryTerms(input.Query)
	followUp := isFollowUpArtifactQuery(input.Query)
	seen := currentArtifactContentFingerprints(input)

	scored := make([]historicalScoredArtifact, 0, len(input.Candidates))
	for index, item := range input.Candidates {
		content := strings.TrimSpace(item.Content)
		if content == "" || item.MessageID == input.CurrentMessageID {
			continue
		}
		fingerprint := normalizedContentFingerprint(content)
		if fingerprint == "" {
			continue
		}
		if _, exists := seen[fingerprint]; exists {
			continue
		}
		seen[fingerprint] = struct{}{}
		score := scoreHistoricalArtifact(item, terms)
		if score == 0 && followUp {
			score = 1
		}
		if score <= 0 {
			continue
		}
		scored = append(scored, historicalScoredArtifact{item: item, score: score, index: index})
	}
	sortHistoricalArtifacts(scored)

	results := make([]domainconversation.ContextArtifact, 0, maxItems)
	var usedTokens int64
	for _, candidate := range scored {
		item := candidate.item
		content := compactSnippet(strings.TrimSpace(item.Content), 500)
		tokenEstimate := estimateTokens(content)
		if item.TokenEstimate > 0 && item.TokenEstimate < tokenEstimate {
			tokenEstimate = item.TokenEstimate
		}
		if tokenEstimate <= 0 {
			tokenEstimate = 1
		}
		if usedTokens+tokenEstimate > maxTokens {
			continue
		}
		item.Content = content
		item.TokenEstimate = tokenEstimate
		results = append(results, item)
		usedTokens += tokenEstimate
		if len(results) >= maxItems {
			break
		}
	}
	return results
}

func contextArtifactHash(kind domainconversation.ContextArtifactKind, sourceID string, content string) string {
	sum := sha256.Sum256([]byte(string(kind) + "\x00" + strings.TrimSpace(sourceID) + "\x00" + content))
	return hex.EncodeToString(sum[:])
}

func contextArtifactMetadata(value map[string]interface{}) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func contextArtifactExcerpt(content string, maxChars int) string {
	if maxChars <= 0 {
		return content
	}
	runes := []rune(content)
	if len(runes) <= maxChars {
		return content
	}
	return string(runes[:maxChars])
}

func fileRAGChunkSourceID(chunk domainconversation.RAGChunk) string {
	if id := strings.TrimSpace(chunk.FileID); id != "" {
		return fmt.Sprintf("%s:%d", id, chunk.ChunkIndex)
	}
	return fmt.Sprintf("%s:%d", strings.TrimSpace(chunk.FileName), chunk.ChunkIndex)
}

func fallbackFileSourceID(file AttachmentInput) string {
	if id := strings.TrimSpace(file.FileID); id != "" {
		return id
	}
	if file.FileObjID > 0 {
		return fmt.Sprintf("%d", file.FileObjID)
	}
	if sha := strings.TrimSpace(file.SHA256); sha != "" {
		return sha
	}
	return strings.TrimSpace(file.FileName)
}

func ragFallbackEvidencesFromAttachments(items []AttachmentInput, reason string, errMessage string) []ragFallbackEvidence {
	if len(items) == 0 {
		return nil
	}
	result := make([]ragFallbackEvidence, 0, len(items))
	for _, item := range items {
		result = append(result, ragFallbackEvidence{
			Attachment: item,
			Reason:     strings.TrimSpace(reason),
			Error:      strings.TrimSpace(errMessage),
		})
	}
	return result
}

func ragFallbackEvidenceAttachments(items []ragFallbackEvidence) []AttachmentInput {
	if len(items) == 0 {
		return nil
	}
	result := make([]AttachmentInput, 0, len(items))
	for _, item := range items {
		result = append(result, item.Attachment)
	}
	return result
}

func toolArtifactContent(row domainconversation.ToolCall) string {
	switch strings.TrimSpace(row.Status) {
	case "error", "failed":
		return firstNonEmptyString(row.ErrorJSON, row.OutputJSON)
	default:
		return firstNonEmptyString(row.OutputJSON, row.ErrorJSON)
	}
}

func toolContextArtifactKind(row domainconversation.ToolCall) domainconversation.ContextArtifactKind {
	toolType := strings.ToLower(strings.TrimSpace(row.ToolType))
	switch toolType {
	case "", "function", "mcp":
		return domainconversation.ContextArtifactToolResult
	default:
		return domainconversation.ContextArtifactNativeTool
	}
}

func currentArtifactContentFingerprints(input historicalContextArtifactInput) map[string]struct{} {
	seen := make(map[string]struct{})
	for _, chunk := range input.CurrentRAGChunks {
		if fingerprint := normalizedContentFingerprint(chunk.Content); fingerprint != "" {
			seen[fingerprint] = struct{}{}
		}
	}
	for _, file := range input.CurrentFallbacks {
		if fingerprint := normalizedContentFingerprint(file.ExtractedText); fingerprint != "" {
			seen[fingerprint] = struct{}{}
		}
	}
	for _, chunk := range input.CurrentRecall {
		if fingerprint := normalizedContentFingerprint(chunk.Content); fingerprint != "" {
			seen[fingerprint] = struct{}{}
		}
	}
	return seen
}

func normalizedContentFingerprint(content string) string {
	value := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(content)), " "))
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func artifactQueryTerms(query string) []string {
	value := strings.ToLower(strings.TrimSpace(query))
	if value == "" {
		return nil
	}
	seen := make(map[string]struct{})
	add := func(term string) {
		term = strings.TrimSpace(term)
		if len([]rune(term)) < 2 {
			return
		}
		seen[term] = struct{}{}
	}
	for _, field := range strings.Fields(value) {
		add(field)
	}
	runes := []rune(value)
	for i := 0; i+1 < len(runes); i++ {
		if isCJKRune(runes[i]) && isCJKRune(runes[i+1]) {
			add(string(runes[i : i+2]))
		}
	}
	terms := make([]string, 0, len(seen))
	for term := range seen {
		terms = append(terms, term)
	}
	return terms
}

func isFollowUpArtifactQuery(query string) bool {
	value := strings.ToLower(strings.TrimSpace(query))
	if value == "" {
		return false
	}
	markers := []string{"刚才", "上面", "上一", "上个", "之前", "前面", "那个", "这些", "这段", "这份", "这个文件", "继续", "修改", "改短", "展开", "引用", "总结"}
	for _, marker := range markers {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func scoreHistoricalArtifact(item domainconversation.ContextArtifact, terms []string) int {
	if len(terms) == 0 {
		return 0
	}
	content := strings.ToLower(strings.TrimSpace(item.Content))
	title := strings.ToLower(strings.TrimSpace(item.SourceTitle + " " + item.SourceID))
	score := 0
	for _, term := range terms {
		if strings.Contains(title, term) {
			score += 4
		}
		if strings.Contains(content, term) {
			score++
		}
	}
	if item.Score > 0 {
		score++
	}
	return score
}

func sortHistoricalArtifacts(items []historicalScoredArtifact) {
	for i := 1; i < len(items); i++ {
		key := items[i]
		j := i - 1
		for j >= 0 && historicalArtifactLess(items[j], key) {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = key
	}
}

func historicalArtifactLess(left historicalScoredArtifact, right historicalScoredArtifact) bool {
	if left.score != right.score {
		return left.score < right.score
	}
	return left.index > right.index
}
