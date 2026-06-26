package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

const (
	conversationMetadataMessageMaxTokens    = int64(5000)
	conversationFallbackTitleMaxRunes       = 16
	conversationAutoGenerateTitleSettingKey = "chat.auto_generate_title"
	conversationMetadataRefreshPending      = "pending"
	conversationMetadataRefreshNotNeeded    = "not_needed"
	conversationMetadataRefreshNoContent    = "skipped_no_titleable_content"
	conversationMetadataTitlePrompt         = `Generate a concise title from the first conversation turn below. Return ONLY a valid JSON object.

## Constraints
1. **Content**: Reflect the primary topic, goal, or main subject.
2. **Language**: Use the language of the conversation turn.
3. **Length**: Max 15 Chinese characters or 8 English words.
4. **Format**: Strictly output valid JSON matching ` + "`" + `{ "title": "..." }` + "`" + ` without markdown code fences, extra quotes, or explanatory text.

## Conversation
{{MESSAGES}}`
	conversationManualTitlePrompt = `Generate a concise title from the conversation excerpt below. Return ONLY a valid JSON object.

## Constraints
1. **Content**: Reflect the latest primary topic, goal, or user intent.
2. **Language**: Use the language of the conversation.
3. **Length**: Max 15 Chinese characters or 8 English words.
4. **Format**: Strictly output valid JSON matching ` + "`" + `{ "title": "..." }` + "`" + ` without markdown code fences, extra quotes, or explanatory text.

## Conversation
{{MESSAGES}}`
	conversationMetadataLabelsPrompt = `Analyze the first turn of the conversation below and extract 1-3 concise topic labels. Return ONLY valid JSON.

## Constraints
1. **Language**: Use the language of the conversation turn.
2. **Taxonomy**: Prioritize broad domains (e.g., science, technology, philosophy, art, politics, business, health, sports, entertainment, education, culture, society, or nature.). Favor accuracy over specificity. Only include subdomains if they are the undeniable focus.
3. **Fallback**: If the input is too short, ambiguous, or lacks a clear primary topic, return: ` + "`" + `{ "labels": ["general"] }` + "`" + `.
4. **Strict Format**: Output pure JSON exactly matching the structure ` + "`" + `{ "labels": ["label1", "label2"] }` + "`" + `. Absolutely NO markdown formatting, code blocks , or explanatory text.

## Conversation
{{MESSAGES}}`
)

type conversationMetadataLLMResult struct {
	Text              string
	Usage             llm.Usage
	Messages          []llm.Message
	PlatformModelName string
	RoutedBindingCode string
	ProviderProtocol  string
	UpstreamName      string
	UpstreamModel     string
	LatencyMS         int64
}

func (s *Service) maybeGenerateConversationMetadataAsync(conversation model.Conversation, userMsg model.Message) {
	if !shouldGenerateConversationMetadata(conversation) {
		return
	}
	fallbackTitle := ""
	if shouldAutoReplaceConversationTitle(conversation.Title) {
		fallbackTitle = conversationTitleFromFirstUserMessage(userMsg.Content)
	}

	go func() {
		asyncCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		if fallbackTitle != "" {
			if _, err := s.repo.UpdateConversationMetadata(asyncCtx, conversation.ID, repository.ConversationMetadataPatch{Title: fallbackTitle}); err != nil && s.logger != nil {
				s.logger.Warn("conversation_fallback_title_update_failed",
					zap.Uint("conversation_id", conversation.ID),
					zap.String("model", conversation.Model),
					zap.Error(err),
				)
			}
		}

		if _, err := s.generateConversationMetadata(asyncCtx, conversation, userMsg); err != nil && s.logger != nil {
			if errors.Is(err, ErrInvalidConversationTitle) {
				s.logger.Info("conversation_metadata_skipped",
					zap.Uint("conversation_id", conversation.ID),
					zap.String("model", conversation.Model),
					zap.String("reason", "no_titleable_content"),
				)
				return
			}
			s.logger.Warn("conversation_metadata_generation_failed",
				zap.Uint("conversation_id", conversation.ID),
				zap.String("model", conversation.Model),
				zap.Error(err),
			)
		}
	}()
}

func (s *Service) generateConversationMetadata(ctx context.Context, conversation model.Conversation, userMsg model.Message) (*model.Conversation, error) {
	cfg := s.cfg.Snapshot()
	messages := buildConversationMetadataMessages(userMsg)
	hasTitleableMessages := strings.TrimSpace(messages) != ""

	var titleErr error
	var labelsErr error
	var updated *model.Conversation
	var mu sync.Mutex
	var wg sync.WaitGroup

	setTitleErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if titleErr == nil {
			titleErr = err
		}
	}
	setLabelsErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if labelsErr == nil {
			labelsErr = err
		}
	}
	setUpdated := func(item *model.Conversation) {
		if item == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		updated = item
	}

	shouldReplaceTitle := shouldAutoReplaceConversationTitle(conversation.Title)
	shouldGenerateTitle := shouldReplaceTitle && s.autoGenerateConversationTitleEnabled(ctx, conversation.UserID)
	fallbackTitle := conversationTitleFromFirstUserMessage(userMsg.Content)

	if shouldGenerateTitle && !hasTitleableMessages {
		setTitleErr(ErrInvalidConversationTitle)
	}
	if s.routeResolver != nil && s.llmClient != nil && shouldGenerateTitle && hasTitleableMessages {
		wg.Add(1)
		go func() {
			defer wg.Done()
			prompt := renderConversationMetadataPrompt(cfg.ConversationTitlePrompt, conversationMetadataTitlePrompt, messages)
			out, err := s.callConversationMetadataLLM(ctx, cfg.ConversationTaskModel, conversation.Model, conversation.UserID, conversation.ID, prompt)
			if err != nil {
				setTitleErr(err)
				return
			}
			s.recordBasicServiceUsage(ctx, conversation.UserID, conversation.ID, "title", "标题", out.PlatformModelName, out.RoutedBindingCode, out.ProviderProtocol, out.UpstreamName, out.UpstreamModel, "5m", out.Usage, out.Messages, out.Text, out.LatencyMS)
			title := resolveConversationMetadataTitle(shouldReplaceTitle, sanitizeGeneratedConversationTitle(parseGeneratedConversationTitle(out.Text)), userMsg.Content)
			if title == "" {
				return
			}
			updated, err := s.repo.UpdateConversationMetadata(ctx, conversation.ID, repository.ConversationMetadataPatch{
				Title:             title,
				ReplaceableTitles: []string{fallbackTitle},
			})
			if err != nil {
				setTitleErr(fmt.Errorf("update conversation title metadata: %w", err))
				return
			}
			setUpdated(updated)
			if s.logger != nil {
				s.logger.Info("conversation_metadata_updated",
					zap.Uint("conversation_id", conversation.ID),
					zap.String("conversation_model", conversation.Model),
					zap.Bool("title_updated", true),
				)
			}
		}()
	}

	shouldGenerateLabels := conversationLabelsEmpty(conversation.LabelsJSON)
	if shouldGenerateLabels && !hasTitleableMessages {
		setLabelsErr(ErrInvalidConversationTitle)
	}
	if s.routeResolver != nil && s.llmClient != nil && shouldGenerateLabels && hasTitleableMessages {
		wg.Add(1)
		go func() {
			defer wg.Done()
			labelsPrompt := renderConversationMetadataPrompt(cfg.ConversationLabelsPrompt, conversationMetadataLabelsPrompt, messages)
			labelsOut, err := s.callConversationMetadataLLM(ctx, cfg.ConversationTaskModel, conversation.Model, conversation.UserID, conversation.ID, labelsPrompt)
			if err != nil {
				setLabelsErr(err)
				return
			}
			s.recordBasicServiceUsage(ctx, conversation.UserID, conversation.ID, "labels", "标签", labelsOut.PlatformModelName, labelsOut.RoutedBindingCode, labelsOut.ProviderProtocol, labelsOut.UpstreamName, labelsOut.UpstreamModel, "5m", labelsOut.Usage, labelsOut.Messages, labelsOut.Text, labelsOut.LatencyMS)
			labels := sanitizeGeneratedConversationLabels(parseGeneratedConversationLabels(labelsOut.Text))
			if len(labels) == 0 {
				return
			}
			raw, marshalErr := json.Marshal(labels)
			if marshalErr != nil {
				setLabelsErr(marshalErr)
				return
			}
			updated, err := s.repo.UpdateConversationMetadata(ctx, conversation.ID, repository.ConversationMetadataPatch{LabelsJSON: string(raw)})
			if err != nil {
				setLabelsErr(fmt.Errorf("update conversation labels metadata: %w", err))
				return
			}
			setUpdated(updated)
			if s.logger != nil {
				s.logger.Info("conversation_metadata_updated",
					zap.Uint("conversation_id", conversation.ID),
					zap.String("conversation_model", conversation.Model),
					zap.Bool("labels_updated", true),
				)
			}
		}()
	}

	wg.Wait()
	mu.Lock()
	resolvedTitleErr := titleErr
	resolvedLabelsErr := labelsErr
	resolvedUpdated := updated
	mu.Unlock()

	if resolvedUpdated != nil {
		return resolvedUpdated, nil
	}
	if shouldReplaceTitle && !shouldGenerateTitle && fallbackTitle != "" {
		updated, err := s.repo.UpdateConversationMetadata(ctx, conversation.ID, repository.ConversationMetadataPatch{Title: fallbackTitle})
		if err != nil {
			return nil, fmt.Errorf("update conversation fallback title metadata: %w", err)
		}
		return updated, nil
	}
	return nil, resolveConversationMetadataError("", "", resolvedTitleErr, resolvedLabelsErr)
}

// RegenerateConversationTitle 根据已有会话正文强制重新生成标题。
func (s *Service) RegenerateConversationTitle(ctx context.Context, userID uint, publicID string) (*model.Conversation, error) {
	conversation, err := s.repo.GetConversationByPublicID(ctx, publicID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}

	messages, err := s.repo.ListAllMessages(ctx, conversation.ID)
	if err != nil {
		return nil, err
	}

	fallbackTitle := conversationTitleFromMessages(messages)
	metadataMessages := buildConversationTitleMessages(messages)
	if metadataMessages == "" && fallbackTitle == "" {
		return nil, ErrInvalidConversationTitle
	}

	cfg := s.cfg.Snapshot()
	title := ""
	if s.routeResolver != nil && s.llmClient != nil && metadataMessages != "" {
		prompt := renderConversationMetadataPrompt(cfg.ConversationTitlePrompt, conversationManualTitlePrompt, metadataMessages)
		out, generateErr := s.callConversationMetadataLLM(ctx, cfg.ConversationTaskModel, conversation.Model, conversation.UserID, conversation.ID, prompt)
		if generateErr != nil {
			if s.logger != nil {
				s.logger.Warn("conversation_title_regeneration_failed",
					zap.Uint("conversation_id", conversation.ID),
					zap.String("model", conversation.Model),
					zap.Error(generateErr),
				)
			}
		} else {
			s.recordBasicServiceUsage(ctx, conversation.UserID, conversation.ID, "title", "标题", out.PlatformModelName, out.RoutedBindingCode, out.ProviderProtocol, out.UpstreamName, out.UpstreamModel, "5m", out.Usage, out.Messages, out.Text, out.LatencyMS)
			title = sanitizeGeneratedConversationTitle(parseGeneratedConversationTitle(out.Text))
		}
	}

	if title == "" {
		title = fallbackTitle
	}
	if title == "" {
		return nil, ErrInvalidConversationTitle
	}

	updated, err := s.repo.UpdateConversationTitleByPublicID(ctx, userID, publicID, title)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	return updated, nil
}

func buildConversationMetadataMessages(userMsg model.Message) string {
	var sb strings.Builder
	if content := strings.TrimSpace(userMsg.Content); content != "" {
		sb.WriteString("user:\n")
		sb.WriteString(content)
	}
	return truncateByEstimatedTokens(strings.TrimSpace(sb.String()), conversationMetadataMessageMaxTokens)
}

func conversationMetadataRefreshHint(conversation model.Conversation, userMsg model.Message) string {
	if !shouldGenerateConversationMetadata(conversation) {
		return conversationMetadataRefreshNotNeeded
	}
	if strings.TrimSpace(buildConversationMetadataMessages(userMsg)) == "" {
		return conversationMetadataRefreshNoContent
	}
	return conversationMetadataRefreshPending
}

func buildConversationTitleMessages(messages []model.Message) string {
	blocks := make([]string, 0)
	remainingTokens := conversationMetadataMessageMaxTokens

	for index := len(messages) - 1; index >= 0; index-- {
		block := renderConversationTitleMessage(messages[index])
		if block == "" {
			continue
		}

		blockTokens := estimateTokens(block)
		if blockTokens > remainingTokens {
			if len(blocks) == 0 {
				blocks = append(blocks, truncateByEstimatedTokens(block, conversationMetadataMessageMaxTokens))
			}
			break
		}

		blocks = append(blocks, block)
		remainingTokens -= blockTokens
	}

	for left, right := 0, len(blocks)-1; left < right; left, right = left+1, right-1 {
		blocks[left], blocks[right] = blocks[right], blocks[left]
	}
	return truncateByEstimatedTokens(strings.TrimSpace(strings.Join(blocks, "\n\n")), conversationMetadataMessageMaxTokens)
}

func renderConversationTitleMessage(item model.Message) string {
	content := strings.TrimSpace(item.Content)
	if content == "" || item.Status == "pending" {
		return ""
	}
	switch item.Role {
	case "user", "assistant":
		return item.Role + ":\n" + content
	default:
		return ""
	}
}

func conversationTitleFromMessages(messages []model.Message) string {
	for index := len(messages) - 1; index >= 0; index-- {
		item := messages[index]
		if item.Role == "user" && item.Status != "pending" {
			if title := conversationTitleFromFirstUserMessage(item.Content); title != "" {
				return title
			}
		}
	}
	for index := len(messages) - 1; index >= 0; index-- {
		item := messages[index]
		if item.Status != "pending" {
			if title := conversationTitleFromFirstUserMessage(item.Content); title != "" {
				return title
			}
		}
	}
	return ""
}

func renderConversationMetadataPrompt(raw string, fallback string, messages string) string {
	prompt := strings.TrimSpace(raw)
	if prompt == "" {
		prompt = fallback
	}
	if strings.Contains(prompt, "{{MESSAGES}}") {
		return strings.ReplaceAll(prompt, "{{MESSAGES}}", messages)
	}
	return strings.TrimSpace(prompt) + "\n\n" + messages
}

// callConversationMetadataLLM 使用内部文本任务路由生成会话标题或标签。
// 即使会话当前模型是图片模型，也只会解析聊天路由。
func (s *Service) callConversationMetadataLLM(ctx context.Context, configuredModel string, conversationModel string, userID uint, conversationID uint, prompt string) (*conversationMetadataLLMResult, error) {
	routes, err := s.resolveTextTaskRouteCandidates(ctx, configuredModel, conversationModel, userID, conversationID, "")
	if err != nil {
		return nil, fmt.Errorf("metadata route resolve: %w", err)
	}
	if len(routes) == 0 {
		return nil, ErrModelRouteNotConfigured
	}
	attributionReferer, attributionTitle := s.llmAttribution()
	messages := []llm.Message{{Role: "user", Content: prompt}}
	var lastErr error
	for _, route := range routes {
		if route == nil || strings.TrimSpace(route.PlatformModelName) == "" {
			continue
		}
		routeConfig := llm.RouteConfig{
			Protocol:            route.Protocol,
			BaseURL:             route.BaseURL,
			APIKey:              route.APIKey,
			HeadersJSON:         route.HeadersJSON,
			ConnectTimeoutMS:    route.ConnectTimeoutMS,
			ReadTimeoutMS:       route.ReadTimeoutMS,
			StreamIdleTimeoutMS: route.StreamIdleTimeoutMS,
			Endpoint:            llm.DefaultEndpointForAdapter(route.Protocol),
			UpstreamModel:       route.UpstreamModel,
			AttributionReferer:  attributionReferer,
			AttributionTitle:    attributionTitle,
		}
		startedAt := time.Now()
		generateInput := buildTextTaskGenerateInput(route, s.cfg.Snapshot(), messages)
		out, generateErr := s.llmClient.Generate(ctx, routeConfig, generateInput)
		if generateErr != nil {
			lastErr = fmt.Errorf("metadata llm generate: %w", generateErr)
			continue
		}
		return &conversationMetadataLLMResult{
			Text:              strings.TrimSpace(out.Text),
			Usage:             out.Usage,
			Messages:          generateInput.Messages,
			PlatformModelName: route.PlatformModelName,
			RoutedBindingCode: route.BindingCode,
			ProviderProtocol:  route.Protocol,
			UpstreamName:      route.UpstreamName,
			UpstreamModel:     route.UpstreamModel,
			LatencyMS:         time.Since(startedAt).Milliseconds(),
		}, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrModelRouteNotConfigured
}

func parseGeneratedConversationTitle(raw string) string {
	object := extractMetadataJSONObject(raw, metadataTitleObjectPattern)
	if object == "" {
		return ""
	}
	var payload struct {
		Title string
	}
	if json.Unmarshal([]byte(object), &payload) == nil {
		return payload.Title
	}
	match := metadataLooseTitlePattern.FindStringSubmatch(object)
	if len(match) == 0 {
		return ""
	}
	return firstNonEmptyString(match[1], match[2], match[3])
}

func parseGeneratedConversationLabels(raw string) []string {
	object := extractMetadataJSONObject(raw, metadataLabelsObjectPattern)
	if object == "" {
		return nil
	}
	var payload struct {
		Labels []string
		Tags   []string
	}
	if json.Unmarshal([]byte(object), &payload) == nil {
		if len(payload.Labels) > 0 {
			return payload.Labels
		}
		return payload.Tags
	}
	match := metadataLooseLabelsPattern.FindStringSubmatch(object)
	if len(match) == 0 {
		return nil
	}
	return parseMetadataStringList(match[1])
}

var (
	metadataTitleObjectPattern  = regexp.MustCompile(`(?is)\{[^{}]*["']?title["']?\s*:\s*(?:"[^"]*"|'[^']*'|[^{}\[\]\r\n]+)[^{}]*\}`)
	metadataLabelsObjectPattern = regexp.MustCompile(`(?is)\{[^{}]*["']?(?:labels|tags)["']?\s*:\s*\[[^\]]*\][^{}]*\}`)
	metadataLooseTitlePattern   = regexp.MustCompile(`(?is)^\s*\{\s*["']?title["']?\s*:\s*(?:"([^"]*)"|'([^']*)'|([^,}\r\n]+))\s*\}\s*$`)
	metadataLooseLabelsPattern  = regexp.MustCompile(`(?is)^\s*\{\s*["']?(?:labels|tags)["']?\s*:\s*\[([^\]]*)\]\s*\}\s*$`)
	metadataQuotedStringPattern = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)
)

func stripMarkdownCodeFence(raw string) string {
	source := strings.TrimSpace(raw)
	if !strings.HasPrefix(source, "```") {
		return source
	}
	source = strings.TrimPrefix(source, "```")
	if index := strings.IndexAny(source, "\r\n"); index >= 0 {
		source = source[index+1:]
	}
	if index := strings.LastIndex(source, "```"); index >= 0 {
		source = source[:index]
	}
	return strings.TrimSpace(source)
}

func extractMetadataJSONObject(raw string, pattern *regexp.Regexp) string {
	source := strings.TrimSpace(stripMarkdownCodeFence(raw))
	if source == "" {
		return ""
	}
	if strings.HasPrefix(source, "{") && strings.HasSuffix(source, "}") && pattern.MatchString(source) {
		return source
	}
	return strings.TrimSpace(pattern.FindString(source))
}

func parseMetadataStringList(value string) []string {
	body := strings.TrimSpace(value)
	if body == "" {
		return nil
	}
	matches := metadataQuotedStringPattern.FindAllStringSubmatch(body, -1)
	if len(matches) > 0 {
		result := make([]string, 0, len(matches))
		for _, match := range matches {
			item := firstNonEmptyString(match[1], match[2])
			if item = strings.TrimSpace(item); item != "" {
				result = append(result, item)
			}
		}
		return result
	}
	parts := strings.Split(body, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(strings.Trim(part, " \t\r\n\"'`“”‘’"))
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func resolveConversationMetadataError(title string, labelsJSON string, titleErr error, labelsErr error) error {
	if strings.TrimSpace(title) != "" || strings.TrimSpace(labelsJSON) != "" {
		return nil
	}
	if titleErr != nil {
		return titleErr
	}
	return labelsErr
}

func sanitizeGeneratedConversationTitle(raw string) string {
	value := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	value = strings.Trim(value, " \t\r\n\"'`“”‘’")
	runes := []rune(value)
	if len(runes) > 80 {
		value = string(runes[:80])
	}
	return strings.TrimSpace(value)
}

func conversationTitleFromFirstUserMessage(content string) string {
	value := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	value = strings.Trim(value, " \t\r\n\"'`“”‘’")
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) > conversationFallbackTitleMaxRunes {
		value = string(runes[:conversationFallbackTitleMaxRunes])
	}
	return strings.TrimSpace(value)
}

func resolveConversationMetadataTitle(shouldReplaceTitle bool, generatedTitle string, firstUserMessage string) string {
	title := strings.TrimSpace(generatedTitle)
	if title != "" || !shouldReplaceTitle {
		return title
	}
	return conversationTitleFromFirstUserMessage(firstUserMessage)
}

func sanitizeGeneratedConversationLabels(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	labels := make([]string, 0, len(raw))
	for _, item := range raw {
		value := strings.Join(strings.Fields(strings.TrimSpace(item)), " ")
		value = strings.Trim(value, " \t\r\n#\"'`“”‘’")
		if value == "" {
			continue
		}
		runes := []rune(value)
		if len(runes) > 24 {
			value = string(runes[:24])
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		labels = append(labels, value)
		if len(labels) >= 6 {
			break
		}
	}
	return labels
}

func (s *Service) autoGenerateConversationTitleEnabled(ctx context.Context, userID uint) bool {
	value, err := s.repo.GetUserSettingValue(ctx, userID, conversationAutoGenerateTitleSettingKey)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("conversation_title_setting_load_failed", zap.Uint("user_id", userID), zap.Error(err))
		}
		return true
	}
	return strings.TrimSpace(strings.ToLower(value)) != "false"
}

func shouldAutoReplaceConversationTitle(title string) bool {
	value := strings.TrimSpace(strings.ToLower(title))
	switch value {
	case "new chat", "新会话":
		return true
	default:
		return false
	}
}

func shouldGenerateConversationMetadata(conversation model.Conversation) bool {
	return shouldAutoReplaceConversationTitle(conversation.Title) || conversationLabelsEmpty(conversation.LabelsJSON)
}

func conversationLabelsEmpty(labelsJSON string) bool {
	value := strings.TrimSpace(strings.ToLower(labelsJSON))
	return value == "" || value == "null" || value == "[]"
}

func truncateByEstimatedTokens(text string, maxTokens int64) string {
	if maxTokens <= 0 || estimateTokens(text) <= maxTokens {
		return text
	}
	suffix := "\n...[truncated]"
	runes := []rune(text)
	keep := int(float64(len(runes)) * float64(maxTokens) / float64(estimateTokens(text)))
	if keep < 1 {
		keep = 1
	}
	if keep > len(runes) {
		keep = len(runes)
	}
	for keep > 1 && estimateTokens(string(runes[:keep])+suffix) > maxTokens {
		keep -= 128
		if keep < 1 {
			keep = 1
		}
	}
	return strings.TrimSpace(string(runes[:keep])) + suffix
}
