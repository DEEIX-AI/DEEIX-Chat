package conversation

import (
	"context"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

type messageRoutePromptInput struct {
	UserContent              string
	ProjectSystemPrompt      string
	HTMLVisualPromptEnabled  bool
	ReasoningContentPassback bool
	DomainMessages           []model.Message
	StableAttachments        []AttachmentInput
	DynamicContext           userContextInput
	PreferencePrompt         string
	SkillPrompts             *skillPrompts
	ToolRuntime              selectedToolRuntime
	SkipImageAttachments     bool
	Config                   config.Config
}

func withMessageRouteReasoningPassbackOptions(
	options map[string]interface{},
	inputOptions map[string]interface{},
	route *channel.ResolvedRoute,
	reasoningContentPassback bool,
	messages []llm.Message,
) map[string]interface{} {
	if route == nil || !shouldApplyReasoningPassbackRequestOptions(
		reasoningContentPassback,
		route.ReasoningPassbackRequestOptions,
		messages,
	) {
		return options
	}
	return withReasoningPassbackRequestOptions(
		options,
		route.ReasoningPassbackRequestOptions,
		inputOptions,
		route.ModelCapabilitiesJSON,
	)
}

func (s *Service) buildMessageRoutePrompt(ctx context.Context, route *channel.ResolvedRoute, input messageRoutePromptInput) (PromptPlan, error) {
	routeMessages := s.applyContextTokenBudget(
		input.DomainMessages,
		route.UpstreamModel,
		route.ModelCapabilitiesJSON,
		input.ReasoningContentPassback,
	)
	historyMessages := historyMessagesFromDomain(routeMessages, historyMessageOptions{
		ReasoningContentPassback: input.ReasoningContentPassback,
	})
	if !input.SkipImageAttachments {
		var err error
		historyMessages, err = s.injectConversationImageContext(ctx, historyMessages, routeMessages, input.StableAttachments, input.Config)
		if err != nil {
			return PromptPlan{}, err
		}
	}
	if len(historyMessages) == 0 {
		historyMessages = append(historyMessages, llm.Message{Role: "user", Content: input.UserContent})
	}

	assembler := NewContextAssembler(int64(input.Config.ContextMaxInputTokens))
	systemPrompt := resolveMessageSystemPromptInjection(input.Config, route, input.ProjectSystemPrompt, input.HTMLVisualPromptEnabled)
	if systemPrompt.Content != "" {
		if systemPrompt.InlineToUser {
			historyMessages = inlineSystemPromptIntoLatestUserMessage(historyMessages, systemPrompt.Content)
		} else {
			assembler.Add(ContextSlot{Kind: SlotSystemPrompt, Content: systemPrompt.Content, Required: true})
		}
	}
	if input.PreferencePrompt != "" {
		assembler.Add(ContextSlot{Kind: SlotPreference, Content: input.PreferencePrompt})
	}
	baseMessages, _ := assembler.Assemble(historyMessages)
	return buildPromptPlan(ctx, promptPlanInput{
		BaseMessages:      baseMessages,
		StableAttachments: input.StableAttachments,
		DynamicContext:    input.DynamicContext,
		SkillPrompts:      input.SkillPrompts,
		ToolRuntime:       input.ToolRuntime,
		Config:            input.Config,
		StoreProvider:     s.storeProvider,
	}), nil
}
