package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"go.uber.org/zap"
)

const (
	conversationMetadataMessageMaxTokens    = int64(5000)
	conversationFirstMessageTitleMaxRunes   = 20
	conversationAutoGenerateTitleSettingKey = "chat.auto_generate_title"
	conversationMetadataTitlePrompt         = `Generate a concise title from the first conversation turn below. Return ONLY a valid JSON object.

## Constraints
1. **Content**: Reflect the primary topic, goal, or main subject.
2. **Language**: Use the language of the conversation turn.
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

func (s *Service) maybeGenerateConversationMetadataAsync(conversation model.Conversation, userMsg model.Message, assistantMsg model.Message) {
	if !shouldGenerateConversationMetadata(conversation) {
		return
	}
	if strings.TrimSpace(userMsg.Content) == "" && strings.TrimSpace(assistantMsg.Content) == "" {
		return
	}

	go func() {
		asyncCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		if _, err := s.generateConversationMetadata(asyncCtx, conversation, userMsg, assistantMsg); err != nil && s.logger != nil {
			s.logger.Warn("conversation_metadata_generation_failed",
				zap.Uint("conversation_id", conversation.ID),
				zap.String("model", conversation.Model),
				zap.Error(err),
			)
		}
	}()
}

func (s *Service) generateConversationMetadata(ctx context.Context, conversation model.Conversation, userMsg model.Message, assistantMsg model.Message) (*model.Conversation, error) {
	cfg := s.cfg.Snapshot()
	messages := buildConversationMetadataMessages(userMsg, assistantMsg)

	title := ""
	labelsJSON := ""
	var generateErr error
	var mu sync.Mutex
	var wg sync.WaitGroup

	setGenerateErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if generateErr == nil {
			generateErr = err
		}
	}

	shouldReplaceTitle := shouldAutoReplaceConversationTitle(conversation.Title)
	shouldGenerateTitle := shouldReplaceTitle && s.autoGenerateConversationTitleEnabled(ctx, conversation.UserID)
	if shouldReplaceTitle && !shouldGenerateTitle {
		title = conversationTitleFromFirstUserMessage(userMsg.Content)
	}

	if s.routeResolver != nil && s.llmClient != nil && shouldGenerateTitle {
		wg.Add(1)
		go func() {
			defer wg.Done()
			prompt := renderConversationMetadataPrompt(cfg.ConversationTitlePrompt, conversationMetadataTitlePrompt, messages)
			out, err := s.callConversationMetadataLLM(ctx, cfg.ConversationTaskModel, conversation.Model, conversation.UserID, conversation.ID, prompt)
			if err != nil {
				setGenerateErr(err)
				return
			}
			s.recordBasicServiceUsage(ctx, conversation.UserID, conversation.ID, "title", "标题", out.PlatformModelName, out.RoutedBindingCode, out.ProviderProtocol, out.UpstreamName, out.UpstreamModel, "5m", out.Usage, out.Messages, out.Text, out.LatencyMS)
			mu.Lock()
			title = sanitizeGeneratedConversationTitle(parseGeneratedConversationTitle(out.Text))
			mu.Unlock()
		}()
	}

	if s.routeResolver != nil && s.llmClient != nil && conversationLabelsEmpty(conversation.LabelsJSON) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			labelsPrompt := renderConversationMetadataPrompt(cfg.ConversationLabelsPrompt, conversationMetadataLabelsPrompt, messages)
			labelsOut, err := s.callConversationMetadataLLM(ctx, cfg.ConversationTaskModel, conversation.Model, conversation.UserID, conversation.ID, labelsPrompt)
			if err != nil {
				setGenerateErr(err)
				return
			}
			s.recordBasicServiceUsage(ctx, conversation.UserID, conversation.ID, "labels", "标签", labelsOut.PlatformModelName, labelsOut.RoutedBindingCode, labelsOut.ProviderProtocol, labelsOut.UpstreamName, labelsOut.UpstreamModel, "5m", labelsOut.Usage, labelsOut.Messages, labelsOut.Text, labelsOut.LatencyMS)
			labels := sanitizeGeneratedConversationLabels(parseGeneratedConversationLabels(labelsOut.Text))
			if len(labels) == 0 {
				return
			}
			raw, marshalErr := json.Marshal(labels)
			if marshalErr != nil {
				setGenerateErr(marshalErr)
				return
			}
			mu.Lock()
			labelsJSON = string(raw)
			mu.Unlock()
		}()
	}

	wg.Wait()
	mu.Lock()
	resolvedTitle := strings.TrimSpace(title)
	resolvedLabelsJSON := strings.TrimSpace(labelsJSON)
	resolvedErr := generateErr
	mu.Unlock()

	if resolvedTitle == "" && resolvedLabelsJSON == "" {
		return nil, resolvedErr
	}
	updated, err := s.repo.UpdateConversationMetadata(ctx, conversation.ID, resolvedTitle, resolvedLabelsJSON)
	if err != nil {
		return nil, fmt.Errorf("update conversation metadata: %w", err)
	}
	if s.logger != nil {
		fields := []zap.Field{
			zap.Uint("conversation_id", conversation.ID),
			zap.String("conversation_model", conversation.Model),
		}
		if resolvedTitle != "" {
			fields = append(fields, zap.Bool("title_updated", true))
		}
		if resolvedLabelsJSON != "" {
			fields = append(fields, zap.Bool("labels_updated", true))
		}
		s.logger.Info("conversation_metadata_updated", fields...)
	}
	return updated, resolvedErr
}

func buildConversationMetadataMessages(userMsg model.Message, assistantMsg model.Message) string {
	var sb strings.Builder
	if content := strings.TrimSpace(userMsg.Content); content != "" {
		sb.WriteString("user:\n")
		sb.WriteString(content)
		sb.WriteString("\n\n")
	}
	if content := strings.TrimSpace(assistantMsg.Content); content != "" {
		sb.WriteString("assistant:\n")
		sb.WriteString(content)
	}
	return truncateByEstimatedTokens(strings.TrimSpace(sb.String()), conversationMetadataMessageMaxTokens)
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
	route, err := s.resolveTextTaskRoute(ctx, configuredModel, conversationModel, userID, conversationID, "")
	if err != nil {
		return nil, fmt.Errorf("metadata route resolve: %w", err)
	}
	if route == nil || strings.TrimSpace(route.PlatformModelName) == "" {
		return nil, ErrModelRouteNotConfigured
	}
	attributionReferer, attributionTitle := s.llmAttribution()
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
	messages := []llm.Message{{Role: "user", Content: prompt}}
	startedAt := time.Now()
	out, err := s.llmClient.Generate(ctx, routeConfig, llm.GenerateInput{Messages: messages})
	if err != nil {
		return nil, fmt.Errorf("metadata llm generate: %w", err)
	}
	return &conversationMetadataLLMResult{
		Text:              strings.TrimSpace(out.Text),
		Usage:             out.Usage,
		Messages:          messages,
		PlatformModelName: route.PlatformModelName,
		RoutedBindingCode: route.BindingCode,
		ProviderProtocol:  route.Protocol,
		UpstreamName:      route.UpstreamName,
		UpstreamModel:     route.UpstreamModel,
		LatencyMS:         time.Since(startedAt).Milliseconds(),
	}, nil
}

func parseGeneratedConversationTitle(raw string) string {
	var payload struct {
		Title string
	}
	if unmarshalStrictJSONObject(raw, &payload) == nil {
		return payload.Title
	}
	if title := extractLooseGeneratedConversationTitle(raw); title != "" {
		return title
	}
	return ""
}

func parseGeneratedConversationLabels(raw string) []string {
	var payload struct {
		Labels []string
		Tags   []string
	}
	if unmarshalJSONObject(raw, &payload) == nil {
		if len(payload.Labels) > 0 {
			return payload.Labels
		}
		return payload.Tags
	}
	return nil
}

func unmarshalJSONObject(raw string, dst interface{}) error {
	source := stripMarkdownCodeFence(raw)
	if err := json.Unmarshal([]byte(source), dst); err == nil {
		return nil
	}
	start := strings.Index(source, "{")
	end := strings.LastIndex(source, "}")
	if start >= 0 && end > start {
		return json.Unmarshal([]byte(source[start:end+1]), dst)
	}
	return fmt.Errorf("no json object")
}

func unmarshalStrictJSONObject(raw string, dst interface{}) error {
	return json.Unmarshal([]byte(stripMarkdownCodeFence(raw)), dst)
}

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

func extractLooseGeneratedConversationTitle(raw string) string {
	source := strings.TrimSpace(stripMarkdownCodeFence(raw))
	if !strings.HasPrefix(source, "{") || !strings.HasSuffix(source, "}") {
		return ""
	}
	value := looseObjectFieldValue(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(source, "{"), "}")), "title")
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
		return ""
	}
	if strings.HasPrefix(value, `"`) || strings.HasPrefix(value, `'`) {
		return extractQuotedLooseValue(value)
	}
	for index, char := range value {
		switch char {
		case ',', '\n', '\r':
			return strings.TrimSpace(value[:index])
		}
	}
	return strings.TrimSpace(value)
}

func looseObjectFieldValue(object string, key string) string {
	lower := strings.ToLower(object)
	needle := strings.ToLower(key)
	searchOffset := 0
	for {
		index := strings.Index(lower[searchOffset:], needle)
		if index < 0 {
			return ""
		}
		keyStart := searchOffset + index
		keyEnd := keyStart + len(needle)
		if value, ok := looseObjectFieldValueAt(object, keyStart, keyEnd); ok {
			return value
		}
		searchOffset = keyEnd
		if searchOffset >= len(object) {
			return ""
		}
	}
}

func looseObjectFieldValueAt(object string, keyStart int, keyEnd int) (string, bool) {
	beforeKey := strings.TrimSpace(object[:keyStart])
	afterKey := strings.TrimSpace(object[keyEnd:])
	if keyStart > 0 && keyEnd < len(object) && (object[keyStart-1] == '"' || object[keyStart-1] == '\'') && object[keyEnd] == object[keyStart-1] {
		beforeKey = strings.TrimSpace(object[:keyStart-1])
		afterKey = strings.TrimSpace(object[keyEnd+1:])
	}
	if beforeKey != "" && !strings.HasSuffix(beforeKey, ",") {
		return "", false
	}
	if !strings.HasPrefix(afterKey, ":") {
		return "", false
	}
	return strings.TrimSpace(afterKey[1:]), true
}

func extractQuotedLooseValue(value string) string {
	if value == "" {
		return ""
	}
	quote := value[0]
	value = value[1:]
	escaped := false
	for index := 0; index < len(value); index++ {
		current := value[index]
		if escaped {
			escaped = false
			continue
		}
		if current == '\\' {
			escaped = true
			continue
		}
		if current == quote {
			return strings.TrimSpace(value[:index])
		}
	}
	return ""
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
	if len(runes) > conversationFirstMessageTitleMaxRunes {
		value = string(runes[:conversationFirstMessageTitleMaxRunes])
	}
	return strings.TrimSpace(value)
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
	case "", "new conversation", "untitled", "新会话", "新对话", "新的对话":
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
