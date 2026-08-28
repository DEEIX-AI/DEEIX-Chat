package conversation

import (
	"context"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
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
	// 模型上下文预算在最终 GenerateInput 完整组装后统一执行。这里保留完整活跃
	// 分支，避免先按历史消息耗尽预算，再遗漏文件、RAG、Skill 与工具定义开销。
	routeMessages := input.DomainMessages
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

	// ContextAssembler 只负责稳定的槽位排序与去重；最终模型窗口由完整请求预算器
	// 统一约束，避免旧的固定 32K 上限提前丢弃偏好等系统上下文。
	assembler := NewContextAssembler(0)
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
