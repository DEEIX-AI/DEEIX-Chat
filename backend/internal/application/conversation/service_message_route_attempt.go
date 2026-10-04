package conversation

import (
	"context"
	domainuicomponent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/uicomponent"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

type messageRoutePromptInput struct {
	UserContent              string
	ProjectSystemPrompt      string
	HTMLVisualPromptEnabled  bool
	UIComponents             []domainuicomponent.Component
	ReasoningContentPassback bool
	DomainMessages           []model.Message
	// ConversationFiles 是本轮文件规划结果（含历史轮次与本轮，已带 ContextMode）。
	// 文本文件随所属用户轮次渲染，图片作为图片内容块随所属轮次发送。
	ConversationFiles []AttachmentInput
	// NativeInputs 缓存本次发送已读取的原生文件，路由之间共用；为空时按需读取。
	NativeInputs         *nativeInputCache
	DynamicContext       userContextInput
	PreferencePrompt     string
	SkillPrompts         *skillPrompts
	ToolRuntime          selectedToolRuntime
	SkipImageAttachments bool
	Config               config.Config
}

func withMessageRouteReasoningPassbackOptions(
	options map[string]any,
	inputOptions map[string]any,
	route *channel.ResolvedRoute,
	reasoningContentPassback bool,
	messages []llm.Message,
) map[string]any {
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

// planRoutePrompt 按路由决定推理内容是否回传后构建提示词；路由故障转移时对新路由重新规划。
// 返回值中的 bool 是该路由生效的推理回传开关。
func (s *Service) planRoutePrompt(
	ctx context.Context,
	userID uint,
	base messageRoutePromptInput,
	route *channel.ResolvedRoute,
) (PromptPlan, bool, error) {
	passbackEnabled := s.reasoningContentPassbackEnabled(ctx, userID, route)
	base.ReasoningContentPassback = passbackEnabled
	plan, err := s.buildMessageRoutePrompt(ctx, route, base)
	return plan, passbackEnabled, err
}

func (s *Service) buildMessageRoutePrompt(ctx context.Context, route *channel.ResolvedRoute, input messageRoutePromptInput) (PromptPlan, error) {
	// 模型上下文预算在最终 GenerateInput 完整组装后统一执行。这里保留完整活跃
	// 分支，避免先按历史消息耗尽预算，再遗漏文件、RAG、Skill 与工具定义开销。
	routeMessages := input.DomainMessages
	historyMessages := historyMessagesFromDomain(routeMessages, historyMessageOptions{
		ReasoningContentPassback: input.ReasoningContentPassback,
	})
	// 原生输入按路由判断：故障转移到另一条路由时重新计算，不把原生内容块发给不支持它的协议。
	native := s.resolveNativeInputs(ctx, route, input.ConversationFiles, input.NativeInputs)
	// 文件按所属轮次就位必须早于图片注入与同角色合并：二者都依赖未合并的历史下标。
	documents := placeTurnDocuments(historyMessages, routeMessages, input.ConversationFiles, turnDocumentOptions{
		Native:     native,
		SkipImages: input.SkipImageAttachments,
	})
	historyMessages = documents.Messages
	dynamicContext := input.DynamicContext
	if native.Policy.ImageUnsupported {
		// 模型不支持图片输入：本轮图片与检索命中的图片都不发送，所属轮次已留说明。
		dynamicContext.Attachments = nil
	}
	if !input.SkipImageAttachments && !native.Policy.ImageUnsupported {
		var err error
		historyMessages, err = s.injectConversationImageContext(ctx, historyMessages, routeMessages, input.ConversationFiles, input.Config)
		if err != nil {
			return PromptPlan{}, err
		}
	}
	// 图片注入按「过滤后的历史下标」对齐，必须在注入之后合并，避免下标错位。
	historyMessages = mergeConsecutiveSameRoleMessages(historyMessages)
	if len(historyMessages) == 0 {
		historyMessages = append(historyMessages, llm.Message{Role: "user", Content: input.UserContent})
	}

	// ContextAssembler 只负责稳定的槽位排序与去重；最终模型窗口由完整请求预算器
	// 统一约束，避免旧的固定 32K 上限提前丢弃偏好等系统上下文。
	assembler := NewContextAssembler(0)
	systemPrompt := resolveMessageSystemPromptInjection(input.Config, route, input.ProjectSystemPrompt, requestPromptOptions{HTMLVisual: input.HTMLVisualPromptEnabled, UIComponents: input.UIComponents})
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
		BaseMessages:   baseMessages,
		TurnDocuments:  documents,
		DynamicContext: dynamicContext,
		SkillPrompts:   input.SkillPrompts,
		ToolRuntime:    input.ToolRuntime,
		Config:         input.Config,
		StoreProvider:  s.storeProvider,
	}), nil
}
