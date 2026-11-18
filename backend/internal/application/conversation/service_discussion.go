package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/google/uuid"
)

const (
	discussionModelPrefix        = "discussion:"
	discussionDefaultStepPrompt  = "你是多模型讨论中的一员。请独立、具体地回答用户问题，指出关键假设与风险。"
	discussionFollowupPrompt     = "你是多模型讨论中的一员。请阅读已有讨论，补充遗漏、纠正错误、挑战薄弱推理，不要重复已经充分的内容。"
	discussionSummaryPrompt      = "你是多模型讨论的主持人。请只依据成功的讨论发言，给出面向用户的最终答案；失败的发言不是观点或证据。"
	discussionMaxTurnRunes       = 12000
	discussionMaxTranscriptRunes = 60000
)

// DiscussionSendInput 是讨论消息发送入参。
type DiscussionSendInput struct {
	UserID                uint
	ConversationID        uint
	RequestID             string
	ContentType           string
	Content               string
	TemplatePublicID      string
	ModelBindings         map[string]string
	ParentMessagePublicID string
	SourceMessagePublicID string
	BranchReason          string
	ClientRunID           string
	OnEvent               func(eventType string, payload map[string]any) error
}

// StreamDiscussionMessage 创建一轮讨论消息，并按模板编排模型调用。
func (s *Service) StreamDiscussionMessage(ctx context.Context, input DiscussionSendInput) (*SendMessageResult, error) {
	if s.discussionTemplates == nil {
		return nil, errors.New("discussion template provider is not configured")
	}
	if s.routeResolver == nil || s.llmClient == nil {
		return nil, ErrModelRouteNotConfigured
	}
	template, cfg, plan, err := s.discussionTemplates.ResolvePlan(ctx, input.TemplatePublicID, input.ModelBindings)
	if err != nil {
		return nil, err
	}
	conversation, err := s.repo.GetConversationByUser(ctx, input.ConversationID, input.UserID)
	if err != nil {
		return nil, ErrConversationNotFound
	}
	startedAt := time.Now()
	runID := normalizeRunID(input.ClientRunID)
	if runID == "" {
		runID = "run_" + normalizePublicID(uuid.NewString())
	}
	if input.ClientRunID != "" && s.generationStreams != nil {
		runCtx, cancel := context.WithCancel(ctx)
		ctx = runCtx
		if err = s.generationStreams.register(ctx, runID, input.UserID, conversation.PublicID, cancel); err != nil {
			return nil, err
		}
	}
	sendInput := SendMessageInput{
		UserID:                input.UserID,
		ConversationID:        input.ConversationID,
		RequestID:             input.RequestID,
		ContentType:           input.ContentType,
		Content:               input.Content,
		PlatformModelName:     discussionModelName(template.PublicID),
		ClientRunID:           input.ClientRunID,
		ParentMessagePublicID: input.ParentMessagePublicID,
		SourceMessagePublicID: input.SourceMessagePublicID,
		BranchReason:          input.BranchReason,
	}
	preparation, err := s.prepareMessageSendBranch(ctx, &sendInput)
	if err != nil {
		return nil, err
	}
	pair, err := s.createMessagePair(ctx, sendInput, runID, preparation, nil, nil)
	if err != nil {
		return nil, err
	}
	s.persistInitialConversationFallbackTitle(ctx, *conversation, *pair.user)
	if conversation.Model != sendInput.PlatformModelName {
		if err = s.repo.UpdateConversationModel(ctx, conversation.ID, sendInput.PlatformModelName, "deeix"); err == nil {
			conversation.Model = sendInput.PlatformModelName
			conversation.Provider = "deeix"
		}
	}

	state := domaindiscussion.State{
		Enabled:      true,
		Status:       "running",
		TemplateID:   template.PublicID,
		TemplateName: template.Name,
		Placeholders: cfg.Placeholders,
		Rounds:       plan.Rounds,
		Summary:      plan.Summary,
		Turns:        make([]domaindiscussion.Turn, 0, len(plan.Rounds)*2),
		CreatedAt:    startedAt.Unix(),
		UpdatedAt:    startedAt.Unix(),
	}
	emit := func(eventType string, payload map[string]any) {
		if input.OnEvent == nil {
			return
		}
		_ = input.OnEvent(eventType, payload)
	}
	var eventMu sync.Mutex
	turnInput := input
	if input.OnEvent != nil {
		turnInput.OnEvent = func(eventType string, payload map[string]any) error {
			eventMu.Lock()
			defer eventMu.Unlock()
			return input.OnEvent(eventType, payload)
		}
		emit = func(eventType string, payload map[string]any) {
			_ = turnInput.OnEvent(eventType, payload)
		}
	}
	emit("discussion_start", map[string]any{
		"template":     template.PublicID,
		"templateName": template.Name,
		"placeholders": discussionPlaceholdersPayloadOf(cfg.Placeholders),
		"rounds":       discussionRoundsPayloadOf(plan.Rounds),
		"summary":      discussionSummaryPayloadOf(plan.Summary),
	})

	// Every user turn keeps the branch's recent history as real user/assistant messages, so a
	// follow-up question reads exactly like a normal chat follow-up. Earlier discussions appear
	// as their final answer only; the per-model utterances live in discussion_json.
	history := s.discussionHistoryMessages(ctx, conversation.ID, pair.user.ID)

	var totalInput, totalOutput, totalReasoning, totalCacheRead, totalCacheWrite int64
	for _, round := range plan.Rounds {
		if err = ctx.Err(); err != nil {
			break
		}
		transcript := discussionTranscript(state.Turns)
		results := make([]domaindiscussion.Turn, len(round.Steps))
		var wg sync.WaitGroup
		for index, step := range round.Steps {
			wg.Add(1)
			go func(index int, step domaindiscussion.RuntimeStep) {
				defer wg.Done()
				turn := s.runDiscussionTurn(ctx, turnInput, conversation, round.Index, step, transcript, history)
				results[index] = turn
				emit("discussion_turn_done", map[string]any{"turn": discussionTurnPayloadOf(turn)})
			}(index, step)
		}
		wg.Wait()
		for _, turn := range results {
			state.Turns = append(state.Turns, turn)
			totalInput += turn.InputTokens
			totalOutput += turn.OutputTokens
			totalCacheRead += turn.CacheReadTokens
			totalCacheWrite += turn.CacheWriteTokens
			totalReasoning += turn.ReasoningTokens
		}
		state.UpdatedAt = time.Now().Unix()
		emit("discussion_round_done", map[string]any{
			"round": round.Index,
			"turns": discussionTurnsPayloadOf(results),
		})
	}

	finalText := ""
	summaryTurn := domaindiscussion.Turn{
		Round:       len(plan.Rounds) + 1,
		StepID:      "summary",
		Participant: plan.Summary.Participant,
		Model:       plan.Summary.Model,
		Status:      "running",
	}
	if ctx.Err() == nil {
		summaryTurn = s.runDiscussionSummary(ctx, turnInput, conversation, plan, state.Turns, history, emit)
	}
	if strings.TrimSpace(summaryTurn.Content) != "" {
		finalText = summaryTurn.Content
	} else if summaryTurn.Error != "" {
		finalText = discussionFallbackSummary(state.Turns, summaryTurn.Error)
	}
	state.SummaryPrompt = summaryTurn.Prompt
	state.SummaryPromptMessages = summaryTurn.PromptMessages
	totalInput += summaryTurn.InputTokens
	totalOutput += summaryTurn.OutputTokens
	totalCacheRead += summaryTurn.CacheReadTokens
	totalCacheWrite += summaryTurn.CacheWriteTokens
	totalReasoning += summaryTurn.ReasoningTokens
	state.UpdatedAt = time.Now().Unix()
	state.FinalContent = finalText
	state.Status = "completed"
	if ctx.Err() != nil {
		state.Status = "stopped"
	}
	if summaryTurn.Status == "error" && state.Status == "completed" {
		state.Status = "completed_with_errors"
	}
	stateJSON, _ := json.Marshal(discussionStatePayloadOf(state))

	assistantStatus := "success"
	errorCode := ""
	errorMessage := ""
	if state.Status == "stopped" {
		assistantStatus = "interrupted"
	} else if summaryTurn.Status == "error" && strings.TrimSpace(finalText) == "" {
		assistantStatus = "error"
		errorCode = "discussion_failed"
		errorMessage = summaryTurn.Error
	}
	latencyMS := time.Since(startedAt).Milliseconds()
	if latencyMS < 0 {
		latencyMS = 0
	}
	if err = s.repo.UpdateAssistantMessageCompletion(ctx, pair.assistant.ID, repository.AssistantMessageCompletionUpdate{
		Content:          finalText,
		InputTokens:      totalInput,
		OutputTokens:     totalOutput,
		CacheReadTokens:  totalCacheRead,
		CacheWriteTokens: totalCacheWrite,
		ReasoningTokens:  totalReasoning,
		LatencyMS:        latencyMS,
		Status:           assistantStatus,
		ErrorCode:        errorCode,
		ErrorMessage:     errorMessage,
		DiscussionJSON:   string(stateJSON),
	}); err != nil {
		return nil, err
	}
	pair.assistant.Content = finalText
	pair.assistant.InputTokens = totalInput
	pair.assistant.OutputTokens = totalOutput
	pair.assistant.CacheReadTokens = totalCacheRead
	pair.assistant.CacheWriteTokens = totalCacheWrite
	pair.assistant.ReasoningTokens = totalReasoning
	pair.assistant.TokenUsage = totalInput + totalCacheRead + totalCacheWrite + totalOutput + totalReasoning
	pair.assistant.DiscussionJSON = string(stateJSON)
	pair.assistant.LatencyMS = latencyMS
	pair.assistant.Status = assistantStatus
	pair.assistant.ErrorCode = errorCode
	pair.assistant.ErrorMessage = errorMessage
	_ = s.repo.UpdateMessageState(ctx, pair.user.ID, "success", "", "")

	if finalText != "" {
		// The summary already streamed as discussion_summary_delta; this replaces the preview.
		emit("delta", map[string]any{"delta": finalText, "replace": true})
	}
	emit("discussion_done", map[string]any{"state": discussionStatePayloadOf(state)})
	return &SendMessageResult{
		UserMessage:       *pair.user,
		AssistantMessage:  *pair.assistant,
		Billable:          false,
		PlatformModelName: sendInput.PlatformModelName,
		LatencyMS:         latencyMS,
		StartedAt:         startedAt,
	}, nil
}

func (s *Service) runDiscussionTurn(
	ctx context.Context,
	input DiscussionSendInput,
	conversation *model.Conversation,
	roundIndex int,
	step domaindiscussion.RuntimeStep,
	transcript string,
	history []llm.Message,
) domaindiscussion.Turn {
	turn := domaindiscussion.Turn{
		Round:       roundIndex,
		StepID:      step.ID,
		Participant: step.Participant,
		Model:       step.Model,
		Status:      "running",
	}
	startedAt := time.Now()
	prompt := strings.TrimSpace(step.Prompt)
	if prompt == "" {
		if roundIndex <= 1 {
			prompt = discussionDefaultStepPrompt
		} else {
			prompt = discussionFollowupPrompt
		}
	}
	messages := append([]llm.Message{{Role: "system", Content: prompt}}, history...)
	messages = append(messages, llm.Message{Role: "user", Content: discussionTurnUserContent(input.Content, transcript)})
	output, promptText, promptMessages, err := s.generateDiscussion(ctx, input, conversation, step.Model, messages, func(promptText string, promptMessages []domaindiscussion.PromptMessage) {
		if input.OnEvent != nil {
			_ = input.OnEvent("discussion_turn_start", map[string]any{
				"round":          roundIndex,
				"stepId":         step.ID,
				"participant":    step.Participant,
				"model":          step.Model,
				"prompt":         promptText,
				"promptMessages": discussionPromptMessagesPayloadOf(promptMessages),
			})
		}
	}, func(delta string) {
		if input.OnEvent != nil {
			_ = input.OnEvent("discussion_turn_delta", map[string]any{
				"round":       roundIndex,
				"stepId":      step.ID,
				"participant": step.Participant,
				"model":       step.Model,
				"delta":       delta,
			})
		}
	})
	turn.Prompt = promptText
	turn.PromptMessages = promptMessages
	if err != nil {
		turn.Status = "error"
		turn.Error = err.Error()
	} else {
		turn.Status = "completed"
		turn.Content = strings.TrimSpace(output.Text)
		turn.InputTokens = output.Usage.InputTokens
		turn.OutputTokens = output.Usage.OutputTokens
		turn.CacheReadTokens = output.Usage.CacheReadTokens
		turn.CacheWriteTokens = output.Usage.CacheWriteTokens
		turn.ReasoningTokens = output.Usage.ReasoningTokens
	}
	turn.LatencyMS = time.Since(startedAt).Milliseconds()
	return turn
}

func (s *Service) runDiscussionSummary(
	ctx context.Context,
	input DiscussionSendInput,
	conversation *model.Conversation,
	plan domaindiscussion.RuntimePlan,
	turns []domaindiscussion.Turn,
	history []llm.Message,
	emit func(string, map[string]any),
) domaindiscussion.Turn {
	turn := domaindiscussion.Turn{
		Round:       len(plan.Rounds) + 1,
		StepID:      "summary",
		Participant: plan.Summary.Participant,
		Model:       plan.Summary.Model,
		Status:      "running",
	}
	startedAt := time.Now()
	prompt := strings.TrimSpace(plan.Summary.Prompt)
	if prompt == "" {
		prompt = discussionSummaryPrompt
	}
	transcript := discussionTranscript(turns)
	if transcript == "" {
		transcript = "没有成功的讨论发言。"
	}
	messages := append([]llm.Message{{Role: "system", Content: prompt}}, history...)
	messages = append(messages, llm.Message{Role: "user", Content: discussionSummaryUserContent(input.Content, transcript)})
	output, promptText, promptMessages, err := s.generateDiscussion(ctx, input, conversation, plan.Summary.Model, messages, func(promptText string, promptMessages []domaindiscussion.PromptMessage) {
		emit("discussion_summary_start", map[string]any{
			"participant":    plan.Summary.Participant,
			"model":          plan.Summary.Model,
			"prompt":         promptText,
			"promptMessages": discussionPromptMessagesPayloadOf(promptMessages),
		})
	}, func(delta string) {
		emit("discussion_summary_delta", map[string]any{"delta": delta})
	})
	turn.Prompt = promptText
	turn.PromptMessages = promptMessages
	if err != nil {
		turn.Status = "error"
		turn.Error = err.Error()
	} else {
		turn.Status = "completed"
		turn.Content = strings.TrimSpace(output.Text)
		turn.InputTokens = output.Usage.InputTokens
		turn.OutputTokens = output.Usage.OutputTokens
		turn.CacheReadTokens = output.Usage.CacheReadTokens
		turn.CacheWriteTokens = output.Usage.CacheWriteTokens
		turn.ReasoningTokens = output.Usage.ReasoningTokens
	}
	turn.LatencyMS = time.Since(startedAt).Milliseconds()
	return turn
}

func (s *Service) generateDiscussion(
	ctx context.Context,
	input DiscussionSendInput,
	conversation *model.Conversation,
	platformModelName string,
	messages []llm.Message,
	onPrompt func(string, []domaindiscussion.PromptMessage),
	onDelta func(string),
) (*llm.GenerateOutput, string, []domaindiscussion.PromptMessage, error) {
	route, err := s.routeResolver.ResolveRoute(ctx, channel.ResolveRouteInput{
		PlatformModelName: platformModelName,
		TaskType:          channel.TaskTypeChat,
		Scope:             channel.RouteScopeUser,
		UserID:            input.UserID,
		ConversationID:    conversation.ID,
		RequestID:         input.RequestID,
	})
	if err != nil {
		return nil, "", nil, err
	}
	attributionReferer, attributionTitle := s.llmAttribution()
	routeConfig := routeConfigFromResolved(route, llm.DefaultEndpointForAdapter(route.Protocol), attributionReferer, attributionTitle)
	generateInput := buildTextTaskGenerateInput(route, s.cfg.Snapshot(), messages)
	generateInput.DisableTools = true
	// Same budget rule as a normal chat turn: trim the oldest history per model, never the
	// system prompt or the current question.
	generateInput, budgetFit := fitGenerateInputToModelBudget(
		generateInput,
		route.UpstreamModel,
		route.ModelCapabilitiesJSON,
		s.cfg.Snapshot().ContextWindowFallbackTokens,
		s.cfg.Snapshot().ContextTokenBudgetEnabled,
	)
	if budgetFit.Trimmed {
		s.logPromptBudgetFit(ctx, route.UpstreamModel, budgetFit)
	}
	promptText := renderDiscussionPrompt(generateInput.Messages)
	promptMessages := discussionPromptMessagesOf(generateInput.Messages)
	if onPrompt != nil {
		onPrompt(promptText, promptMessages)
	}
	output, err := s.llmClient.GenerateStream(ctx, routeConfig, generateInput, func(event llm.GenerateStreamEvent) error {
		if event.Delta != "" && onDelta != nil {
			onDelta(event.Delta)
		}
		return nil
	})
	if err != nil {
		return nil, promptText, promptMessages, err
	}
	return output, promptText, promptMessages, nil
}

func discussionPromptMessagesOf(messages []llm.Message) []domaindiscussion.PromptMessage {
	result := make([]domaindiscussion.PromptMessage, 0, len(messages))
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		result = append(result, domaindiscussion.PromptMessage{Role: message.Role, Content: content})
	}
	return result
}

// discussionTurnUserContent 组装一次轮次发言的当前用户消息：本次问题 + 本轮之前的讨论记录。
// 历史对话由 discussionHistoryMessages 以真实 user/assistant 消息的形式携带，不再拼进这一段。
func discussionTurnUserContent(question string, transcript string) string {
	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(question))
	if transcript != "" {
		// From the second round on, every model sees the previous round and all earlier rounds.
		builder.WriteString("\n\n上一轮及更早的讨论记录：\n")
		builder.WriteString(transcript)
	}
	return builder.String()
}

func discussionSummaryUserContent(question string, transcript string) string {
	return strings.TrimSpace(question) + "\n\n讨论记录：\n" + transcript
}

// renderDiscussionPrompt 把实际发送的消息拼成可读文本，仅用于调试展示。
func renderDiscussionPrompt(messages []llm.Message) string {
	var builder strings.Builder
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(message.Role)
		builder.WriteString(":\n")
		builder.WriteString(content)
	}
	return builder.String()
}

// discussionHistoryMessages 取当前分支上本次提问之前的对话历史，保留 user/assistant 原角色。
// 条数受 ContextMaxTurns 约束（一轮 = user+assistant 两条），token 预算在 generateDiscussion 里按模型裁剪。
func (s *Service) discussionHistoryMessages(ctx context.Context, conversationID uint, userMessageID uint) []llm.Message {
	if s.repo == nil || userMessageID == 0 {
		return nil
	}
	maxDepth := s.cfg.Snapshot().ContextMaxTurns*2 + 2
	if maxDepth < 8 {
		maxDepth = 8
	}
	ancestors, err := s.repo.ListMessageAncestors(ctx, conversationID, userMessageID, maxDepth)
	if err != nil {
		return nil
	}
	history := make([]llm.Message, 0, len(ancestors))
	for _, item := range ancestors {
		if item.ID == userMessageID {
			// 本次提问单独组装成最后一条 user，不重复进入历史。
			continue
		}
		if item.Role != "user" && item.Role != "assistant" {
			continue
		}
		if !isPromptHistoryMessage(item) {
			continue
		}
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		history = append(history, llm.Message{Role: item.Role, Content: content})
	}
	return history
}

func discussionTranscript(turns []domaindiscussion.Turn) string {
	var builder strings.Builder
	currentRound := 0
	for _, turn := range turns {
		if turn.Status != "completed" {
			continue
		}
		content := strings.TrimSpace(turn.Content)
		if content == "" {
			continue
		}
		runes := []rune(content)
		if len(runes) > discussionMaxTurnRunes {
			content = string(runes[:discussionMaxTurnRunes]) + "\n...[truncated]"
		}
		if turn.Round != currentRound {
			if builder.Len() > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(fmt.Sprintf("第 %d 轮\n", turn.Round))
			currentRound = turn.Round
		}
		// The speaker is named by its model: participants are internal placeholder keys and must
		// not leak into the prompt. A turn without a model falls back to the key it was planned with.
		speaker := strings.TrimSpace(turn.Model)
		if speaker == "" {
			speaker = turn.Participant
		}
		builder.WriteString(fmt.Sprintf("- %s: %s\n", speaker, content))
		if builder.Len() > discussionMaxTranscriptRunes {
			break
		}
	}
	text := builder.String()
	runes := []rune(text)
	if len(runes) > discussionMaxTranscriptRunes {
		text = string(runes[:discussionMaxTranscriptRunes]) + "\n...[transcript truncated]"
	}
	return strings.TrimSpace(text)
}

func discussionFallbackSummary(turns []domaindiscussion.Turn, failure string) string {
	transcript := discussionTranscript(turns)
	var builder strings.Builder
	builder.WriteString("## 讨论记录\n\n")
	if transcript == "" {
		builder.WriteString("没有成功的讨论发言。\n")
	} else {
		builder.WriteString(transcript)
	}
	if strings.TrimSpace(failure) != "" {
		builder.WriteString("\n\n> 总结模型调用失败：")
		builder.WriteString(failure)
	}
	return builder.String()
}

func discussionModelName(publicID string) string {
	return discussionModelPrefix + strings.TrimSpace(publicID)
}
