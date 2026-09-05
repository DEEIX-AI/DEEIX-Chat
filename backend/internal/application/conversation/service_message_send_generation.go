package conversation

import (
	"context"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// routeGenerationMode 区分首个路由的提示词规划与路由故障转移后的重建。
type routeGenerationMode int

const (
	routeGenerationInitial routeGenerationMode = iota
	routeGenerationFailover
)

// routeGenerationContext 汇集与具体路由无关、整条消息共享的请求形状输入。
type routeGenerationContext struct {
	input                  SendMessageInput
	conversation           *model.Conversation
	cfg                    config.Config
	tools                  []llm.ToolDefinition
	promptCacheSessionKey  string
	statefulContextConfig  string
	statefulContextState   string
	normalizedBranchReason string
	attributionReferer     string
	attributionTitle       string
}

type routeGenerationPreparationInput struct {
	Generation               routeGenerationContext
	Route                    *channel.ResolvedRoute
	PromptPlan               PromptPlan
	ReasoningContentPassback bool
	Mode                     routeGenerationMode
	TraceRecorder            *messageTraceRecorder
}

// routeGenerationPlan 是一条路由上首次上游调用的完整请求形状。
type routeGenerationPlan struct {
	route                    *channel.ResolvedRoute
	routeConfig              llm.RouteConfig
	promptPlan               PromptPlan
	reasoningContentPassback bool
	filteredOptions          map[string]any
	// llmMessages 是预算裁剪后的完整上下文，工具回灌在其上追加；fullLLMMessages 是它在剥离
	// Responses instructions 与有状态续传裁剪之前的副本，用于续传指纹与账单输入估算。
	llmMessages               []llm.Message
	fullLLMMessages           []llm.Message
	generateInput             llm.GenerateInput
	estimatedPromptTokens     int64
	fullContextPromptTokens   int64
	statefulPrefixFingerprint string
	statefulDecision          statefulResponseDecision
	statefulContinuation      bool
	promptMode                string
	promptShape               promptShape
}

// prepareRouteGeneration 把路由级提示词规划固化为一次上游调用的请求形状：过滤模型参数、配置提示词
// 缓存、按模型上下文预算裁剪历史、剥离 Responses instructions，并计算有状态续传指纹。
// 首个路由额外决定是否沿用 previous_response_id；故障转移后的路由总是发送完整上下文。
func (s *Service) prepareRouteGeneration(ctx context.Context, input routeGenerationPreparationInput) routeGenerationPlan {
	gen := input.Generation
	route := input.Route
	promptPlan := input.PromptPlan
	reasoningContentPassback := input.ReasoningContentPassback
	mode := input.Mode
	traceRecorder := input.TraceRecorder
	cfg := gen.cfg
	messageInput := gen.input
	conversation := gen.conversation
	plan := routeGenerationPlan{
		route:                    route,
		routeConfig:              messageRouteConfig(route, gen.attributionReferer, gen.attributionTitle),
		promptPlan:               promptPlan,
		reasoningContentPassback: reasoningContentPassback,
	}

	llmMessages := promptPlan.Messages
	filteredOptions := filterModelOptions(messageInput.Options, route.Protocol, modelOptionPolicyConfig{
		Mode:                  cfg.ModelOptionPolicyMode,
		AllowedPathsJSON:      cfg.ModelOptionAllowedPaths,
		DeniedPathsJSON:       cfg.ModelOptionDeniedPaths,
		ModelCapabilitiesJSON: route.ModelCapabilitiesJSON,
	})
	filteredOptions = withMessageRouteReasoningPassbackOptions(
		filteredOptions,
		messageInput.Options,
		route,
		reasoningContentPassback,
		llmMessages,
	)
	var promptCacheKey string
	promptCacheKey, filteredOptions, llmMessages = configureOpenAIPromptCacheRequestForRoute(
		route,
		gen.promptCacheSessionKey,
		filteredOptions,
		llmMessages,
	)
	generateInput := llm.GenerateInput{
		RequestID:              strings.TrimSpace(messageInput.RequestID),
		ConversationID:         messageInput.ConversationID,
		ConversationPublicID:   strings.TrimSpace(conversation.PublicID),
		ConversationSessionKey: strings.TrimSpace(conversation.SessionKey),
		PromptCacheKey:         promptCacheKey,
		Messages:               cloneLLMMessages(llmMessages),
		Tools:                  gen.tools,
		Options:                filteredOptions,
	}
	generateInput, budgetFit := fitGenerateInputToModelBudget(
		generateInput,
		route.UpstreamModel,
		route.ModelCapabilitiesJSON,
		cfg.ContextWindowFallbackTokens,
		cfg.ContextTokenBudgetEnabled,
	)
	llmMessages = cloneLLMMessages(generateInput.Messages)
	if budgetFit.Trimmed {
		plan.promptPlan.applyMessages(llmMessages)
	}
	s.logPromptBudgetFit(ctx, route.UpstreamModel, budgetFit)
	if supportsOpenAIResponsesBackgroundMode(route) {
		generateInput.ResponsesBackground = true
	}
	fullLLMMessages := cloneLLMMessages(llmMessages)
	applyOpenAIResponsesInstructions(route, plan.routeConfig.Endpoint, &generateInput)
	plan.estimatedPromptTokens = estimateGenerateInputTokens(generateInput)
	// 有状态 Responses 续传只发送本轮增量，但压缩决策必须继续观察完整上下文；
	// 同时保留预算裁剪前的规模，让被裁掉的历史在回复后及时进入滚动摘要。
	plan.fullContextPromptTokens = maxPromptTokenEstimate(budgetFit.TokensBefore, plan.estimatedPromptTokens)
	plan.statefulPrefixFingerprint = buildPromptStateFingerprint(promptStateFingerprintInput{
		Protocol:          route.Protocol,
		Endpoint:          plan.routeConfig.Endpoint,
		UpstreamID:        route.UpstreamID,
		UpstreamModel:     route.UpstreamModel,
		PlatformModelName: conversation.Model,
		ContextConfig:     gen.statefulContextConfig,
		ContextState:      gen.statefulContextState,
		Messages:          promptStatePrefixMessages(fullLLMMessages),
		Tools:             gen.tools,
		Options:           filteredOptions,
	})

	switch mode {
	case routeGenerationInitial:
		plan.statefulDecision = resolveStatefulPreviousResponseID(
			route,
			gen.normalizedBranchReason,
			conversation.LastResponseID,
			conversation.LastPromptFingerprint,
			plan.statefulPrefixFingerprint,
			filteredOptions,
		)
		// 有状态续传只裁剪发送的消息，上游仍按完整上下文计输入，规划预估保持完整形状。
		plan.statefulContinuation = applyStatefulResponseContinuation(plan.routeConfig.Endpoint, plan.statefulDecision, &generateInput)
		plan.promptMode = "full"
		if strings.TrimSpace(generateInput.PreviousResponseID) != "" {
			plan.promptMode = "stateful"
		}
	case routeGenerationFailover:
		plan.statefulDecision = statefulResponseDecision{DisabledReason: "route_failover"}
		plan.promptMode = "route_failover"
	}
	plan.promptShape = summarizePromptShape(plan.promptMode, generateInput.Messages, fullLLMMessages, generateInput.PreviousResponseID)
	if traceRecorder != nil {
		traceRecorder.recordPromptTrace(buildMessagePromptTrace(messagePromptTraceInput{
			Plan:               plan.promptPlan.Trace,
			Mode:               plan.promptMode,
			PromptFingerprint:  plan.statefulPrefixFingerprint,
			StatefulDecision:   plan.statefulDecision,
			SentMessages:       generateInput.Messages,
			FullMessages:       fullLLMMessages,
			PreviousResponseID: generateInput.PreviousResponseID,
		}))
	}

	plan.filteredOptions = filteredOptions
	plan.llmMessages = llmMessages
	plan.fullLLMMessages = fullLLMMessages
	plan.generateInput = generateInput
	return plan
}

// messageGenerationRunner 执行一条消息内的每次上游 LLM 调用，并累计跨调用的运行观测：
// 请求计数、用量、可见输出、首字延迟与后台 Responses 恢复状态。中断收尾与最终结果都从这里读取。
type messageGenerationRunner struct {
	service       *Service
	input         SendMessageInput
	runID         string
	startedAt     time.Time
	preferStream  bool
	onDelta       func(string) error
	maxLLMCalls   int
	usage         *messageUsageAccumulator
	traceRecorder *messageTraceRecorder
	// routeConfig 是当前路由的调用配置，路由故障转移时随之更新。
	routeConfig llm.RouteConfig

	llmRequestCount       int
	completedLLMCallCount int
	upstreamCallStarted   bool
	// attemptHadSideEffect 表示当前路由上已有用量、推理或工具事件产生，此后不得再切换路由重试。
	attemptHadSideEffect        bool
	visibleDeltaCount           int
	firstVisibleDeltaLatencyMS  int64
	streamedText                strings.Builder
	responsesBackgroundRecovery openAIResponsesBackgroundRecoveryState
	lastAttemptObservation      *generationAttemptObservation
}

// emitVisibleDelta 把一段可见增量交给客户端回调，并记录首字延迟与累计可见文本。
func (r *messageGenerationRunner) emitVisibleDelta(delta string) error {
	if delta == "" {
		return nil
	}
	r.visibleDeltaCount++
	if r.firstVisibleDeltaLatencyMS == 0 {
		r.firstVisibleDeltaLatencyMS = time.Since(r.startedAt).Milliseconds()
		if r.firstVisibleDeltaLatencyMS < 0 {
			r.firstVisibleDeltaLatencyMS = 0
		}
	}
	if r.traceRecorder != nil {
		r.traceRecorder.completeProcess()
		r.traceRecorder.completeUpstreamThink()
	}
	if err := r.onDelta(delta); err != nil {
		return err
	}
	r.streamedText.WriteString(delta)
	return nil
}

// beginRouteFailover 切换到新路由：重置只属于上一路由的观测，已累计的用量与调用计数保留。
func (r *messageGenerationRunner) beginRouteFailover(routeConfig llm.RouteConfig) {
	r.routeConfig = routeConfig
	r.attemptHadSideEffect = false
	r.streamedText.Reset()
}

func (r *messageGenerationRunner) warn(ctx context.Context, event string, route *channel.ResolvedRoute, err error) {
	if r.service.logger == nil {
		return
	}
	r.service.logger.Warn(event,
		zap.String("trace_id", traceid.FromContext(ctx)),
		zap.Uint("conversation_id", r.input.ConversationID),
		zap.String("protocol", route.Protocol),
		zap.String("upstream_name", route.UpstreamName),
		zap.Error(err),
	)
}

// generationCall 是一次上游调用的局部观测：本次可见文本、流式用量与可重试性判断。
type generationCall struct {
	runner              *messageGenerationRunner
	observation         *generationAttemptObservation
	visibleText         strings.Builder
	thinkingRouter      *thinkingDeltaRouter
	streamUsage         llm.Usage
	observedServerTools map[string]string
}

func (c *generationCall) emitVisibleDelta(delta string) error {
	if delta != "" {
		c.observation.markObservable()
	}
	if err := c.runner.emitVisibleDelta(delta); err != nil {
		return err
	}
	c.visibleText.WriteString(delta)
	c.runner.usage.recordCallVisibleText(delta)
	return nil
}

// finalizeNonStreamingOutput 把非流式返回同步进 trace，并按需把正文作为一次性可见增量发出。
func (c *generationCall) finalizeNonStreamingOutput(output *llm.GenerateOutput, emitVisible bool) error {
	if output == nil {
		return nil
	}
	r := c.runner
	if r.traceRecorder != nil && r.traceRecorder.visible() && r.traceRecorder.onEvent != nil &&
		(output.Reasoning != nil || len(output.ServerToolCalls) > 0) {
		c.observation.markObservable()
	}
	r.usage.recordCallReasoningText(outputReasoningContent(output))
	cleanText, _ := syncUpstreamOutputTrace(r.traceRecorder, output, r.runID)
	output.Text = cleanText
	if emitVisible {
		return c.emitVisibleDelta(cleanText)
	}
	r.usage.recordCallVisibleText(cleanText)
	return nil
}

// handleStreamEvent 消费一条上游流式事件：换算用量增量、转发媒体与推理事件、同步服务端工具 trace，
// 并把正文增量路由到可见文本或思考轨迹。
func (c *generationCall) handleStreamEvent(generationCtx context.Context, currentInput llm.GenerateInput, event llm.GenerateStreamEvent) error {
	r := c.runner
	if currentInput.ResponsesBackground {
		if responseID := strings.TrimSpace(event.ResponseID); responseID != "" {
			r.responsesBackgroundRecovery.ResponseID = responseID
		}
	}
	if r.service.isMessageGenerationCanceled(generationCtx, r.runID) {
		return ErrMessageGenerationCanceled
	}
	if event.Usage != (llm.Usage{}) {
		r.attemptHadSideEffect = true
		// 上游流式 usage 通常是“本次 LLM 调用累计值”，但一条消息可能包含多轮 LLM 调用。
		// 这里先换算成本次调用内增量，再累加成本轮消息总量，保证实时展示和最终账单口径一致。
		usageDelta := diffLLMUsage(event.Usage, c.streamUsage)
		c.streamUsage = event.Usage
		if currentInput.ResponsesBackground {
			r.responsesBackgroundRecovery.ObservedUsage = c.streamUsage
		}
		currentUsage := r.usage.addObservedUsage(usageDelta)
		if r.input.OnEvent != nil {
			c.observation.markObservable()
			if err := emitLLMUsageEvent(r.input.OnEvent, currentUsage); err != nil {
				return err
			}
		}
	}
	if event.GeneratedImage != nil {
		r.attemptHadSideEffect = true
		if r.input.OnEvent != nil && strings.TrimSpace(event.GeneratedImage.B64JSON) != "" {
			c.observation.markObservable()
		}
		if err := emitMediaImageDelta(r.input.OnEvent, event); err != nil {
			return err
		}
	}
	if event.Reasoning != nil && event.Reasoning.Text != "" {
		r.attemptHadSideEffect = true
		if event.Reasoning.Kind != messageTraceThinkKindSignature {
			r.usage.recordCallReasoningText(event.Reasoning.Text)
		}
	}
	if r.traceRecorder != nil && event.Reasoning != nil && event.Reasoning.Text != "" {
		if r.traceRecorder.visible() && r.traceRecorder.onEvent != nil {
			c.observation.markObservable()
		}
		r.traceRecorder.appendUpstreamReasoning(event.Reasoning.Kind, event.Reasoning.Text, reasoningPayload(event.Reasoning))
		if strings.EqualFold(strings.TrimSpace(event.Reasoning.Status), "completed") {
			r.traceRecorder.completeUpstreamThink()
		}
	}
	if event.ServerToolCall != nil {
		r.attemptHadSideEffect = true
	}
	if r.traceRecorder != nil && event.ServerToolCall != nil {
		if r.traceRecorder.visible() && r.traceRecorder.onEvent != nil {
			c.observation.markObservable()
		}
		toolStatus := normalizeStreamServerToolStatus(event.ServerToolCall.Status)
		observeServerTool(c.observedServerTools, *event.ServerToolCall, toolStatus)
		summary, markdown, payload := buildToolTrace([]model.ToolCall{{
			RunID:      r.runID,
			ToolCallID: strings.TrimSpace(event.ServerToolCall.ToolCallID),
			ToolType:   strings.TrimSpace(event.ServerToolCall.ToolType),
			ToolName:   strings.TrimSpace(event.ServerToolCall.ToolName),
			Status:     toolStatus,
			InputJSON:  strings.TrimSpace(event.ServerToolCall.ArgumentsJSON),
			OutputJSON: strings.TrimSpace(event.ServerToolCall.OutputJSON),
			ErrorJSON:  strings.TrimSpace(event.ServerToolCall.ErrorJSON),
		}})
		r.traceRecorder.syncToolSection(summary, markdown, payload, traceStatusFromToolStatus(toolStatus))
	}
	if r.onDelta == nil || event.Delta == "" {
		return nil
	}
	visibleDelta, thinkDelta := c.thinkingRouter.consume(event.Delta)
	if thinkDelta != "" {
		r.attemptHadSideEffect = true
		r.usage.recordCallReasoningText(thinkDelta)
	}
	if r.traceRecorder != nil && thinkDelta != "" {
		if r.traceRecorder.visible() && r.traceRecorder.onEvent != nil {
			c.observation.markObservable()
		}
		r.traceRecorder.appendUpstreamReasoning(messageTraceThinkKindContent, thinkDelta, nil)
	}
	if visibleDelta == "" {
		return nil
	}
	return c.emitVisibleDelta(visibleDelta)
}

// generate 执行一次上游调用：优先流式，流式不受支持或在产生任何副作用前失败时回退到非流式。
// fullMessages 是本次调用对应的完整上下文，有状态续传时用于估算上游实际计费的输入规模。
func (r *messageGenerationRunner) generate(ctx context.Context, currentInput llm.GenerateInput, fullMessages []llm.Message) (*llm.GenerateOutput, error) {
	call := &generationCall{
		runner:              r,
		observation:         &generationAttemptObservation{},
		thinkingRouter:      &thinkingDeltaRouter{},
		observedServerTools: make(map[string]string),
	}
	r.lastAttemptObservation = call.observation
	callPromptMode := "full"
	if strings.TrimSpace(currentInput.PreviousResponseID) != "" {
		callPromptMode = "stateful"
	}
	streamRequested := r.preferStream && r.onDelta != nil
	streamSupported := llm.SupportsStreamingAdapter(r.routeConfig.Protocol)
	callPromptShape := summarizePromptShape(callPromptMode, currentInput.Messages, currentInput.Messages, currentInput.PreviousResponseID)
	r.usage.beginCall(estimateBillableInputTokens(currentInput, fullMessages))
	if currentInput.ResponsesBackground {
		r.responsesBackgroundRecovery = openAIResponsesBackgroundRecoveryState{Enabled: true}
	} else {
		r.responsesBackgroundRecovery = openAIResponsesBackgroundRecoveryState{}
	}
	generationCtx, generationSpan := platformtracing.Start(ctx, "conversation.llm.generate",
		trace.WithAttributes(append([]attribute.KeyValue{
			attribute.Int64("conversation.id", int64(r.input.ConversationID)),
			attribute.String("llm.model", r.routeConfig.UpstreamModel),
			attribute.String("llm.protocol", r.routeConfig.Protocol),
			attribute.String("llm.endpoint", r.routeConfig.Endpoint),
			attribute.Bool("llm.stream", streamRequested && streamSupported),
			attribute.Bool("llm.tools_disabled", currentInput.DisableTools),
			attribute.Bool("llm.responses_background", currentInput.ResponsesBackground),
			attribute.Int("llm.message_count", len(currentInput.Messages)),
			attribute.Int("llm.tool_count", len(currentInput.Tools)),
		}, promptShapeTraceAttributes("llm.prompt", callPromptShape)...)...),
	)
	var generateErr error
	defer func() {
		platformtracing.RecordError(generationSpan, generateErr)
		generationSpan.End()
	}()

	if !streamRequested || !streamSupported {
		r.upstreamCallStarted = true
		r.llmRequestCount++
		output, err := r.service.llmClient.Generate(generationCtx, r.routeConfig, currentInput)
		generateErr = err
		if err == nil {
			generateErr = call.finalizeNonStreamingOutput(output, streamRequested)
			if generateErr != nil {
				return output, generateErr
			}
		}
		if generateErr == nil {
			r.completedLLMCallCount++
			r.usage.finishCall(
				output != nil && output.Usage.HasObservedInput(),
				output != nil && output.Usage.HasObservedOutput(),
			)
		}
		return output, err
	}

	r.upstreamCallStarted = true
	r.llmRequestCount++
	output, streamErr := r.service.llmClient.GenerateStream(generationCtx, r.routeConfig, currentInput, func(event llm.GenerateStreamEvent) error {
		return call.handleStreamEvent(generationCtx, currentInput, event)
	})
	generateErr = streamErr
	if generateErr == nil {
		visibleTail, thinkTail := call.thinkingRouter.flush()
		r.usage.recordCallReasoningText(thinkTail)
		if r.traceRecorder != nil && thinkTail != "" {
			r.traceRecorder.appendUpstreamReasoning(messageTraceThinkKindContent, thinkTail, nil)
		}
		finalizeStreamingOutputTrace(r.traceRecorder, output, r.runID, call.observedServerTools)
		if visibleTail != "" {
			if tailErr := call.emitVisibleDelta(visibleTail); tailErr != nil {
				generateErr = tailErr
			}
		}
		if output != nil {
			output.Text = call.visibleText.String()
		}
	}
	if !r.attemptHadSideEffect && r.llmRequestCount < r.maxLLMCalls &&
		call.observation.canRetry(generateErr, shouldFallbackToNonStreaming) {
		r.llmRequestCount++
		output, generateErr = r.service.llmClient.Generate(generationCtx, r.routeConfig, currentInput)
		if generateErr == nil {
			generateErr = call.finalizeNonStreamingOutput(output, true)
		}
	}
	if generateErr == nil {
		r.completedLLMCallCount++
		r.usage.finishCall(
			call.streamUsage.HasObservedInput() || (output != nil && output.Usage.HasObservedInput()),
			call.streamUsage.HasObservedOutput() || (output != nil && output.Usage.HasObservedOutput()),
		)
	}
	return output, generateErr
}

// runRouteAttempt 在当前路由上发起首次调用，并按上游拒绝原因就地降级重试：
// 先放弃后台 Responses 模式，再放弃 previous_response_id 续传改发完整上下文。
// 降级会写回 plan，使后续工具回灌沿用降级后的请求形状。
func (r *messageGenerationRunner) runRouteAttempt(ctx context.Context, plan *routeGenerationPlan, sendSpan trace.Span) (*llm.GenerateOutput, error) {
	output, attemptErr := r.generate(ctx, plan.generateInput, plan.fullLLMMessages)
	if !r.attemptHadSideEffect && r.llmRequestCount < r.maxLLMCalls && plan.generateInput.ResponsesBackground &&
		r.lastAttemptObservation.canRetry(attemptErr, shouldRetryWithoutResponsesBackground) {
		r.warn(ctx, "openai_responses_background_rejected_retry_standard", plan.route, attemptErr)
		plan.generateInput.ResponsesBackground = false
		r.responsesBackgroundRecovery = openAIResponsesBackgroundRecoveryState{}
		output, attemptErr = r.generate(ctx, plan.generateInput, plan.fullLLMMessages)
	}
	if !r.attemptHadSideEffect && r.llmRequestCount < r.maxLLMCalls && strings.TrimSpace(plan.generateInput.PreviousResponseID) != "" &&
		r.lastAttemptObservation.canRetry(attemptErr, shouldRetryWithoutPreviousResponseID) {
		r.warn(ctx, "previous_response_id_rejected_retry_full_context", plan.route, attemptErr)
		_ = r.service.repo.UpdateConversationLastResponseID(ctx, r.input.ConversationID, "")
		plan.generateInput.PreviousResponseID = ""
		plan.generateInput.Messages = plan.fullLLMMessages
		applyOpenAIResponsesInstructions(plan.route, plan.routeConfig.Endpoint, &plan.generateInput)
		plan.estimatedPromptTokens = estimateGenerateInputTokens(plan.generateInput)
		plan.promptShape = summarizePromptShape("full_retry", plan.generateInput.Messages, plan.fullLLMMessages, "")
		if r.traceRecorder != nil {
			r.traceRecorder.recordPromptTrace(buildMessagePromptTrace(messagePromptTraceInput{
				Plan:              plan.promptPlan.Trace,
				Mode:              "full_retry",
				PromptFingerprint: plan.statefulPrefixFingerprint,
				StatefulDecision: statefulResponseDecision{
					DisabledReason: "previous_response_rejected",
				},
				SentMessages: plan.generateInput.Messages,
				FullMessages: plan.fullLLMMessages,
			}))
		}
		sendSpan.SetAttributes(promptShapeTraceAttributes("conversation.prompt_retry", plan.promptShape)...)
		output, attemptErr = r.generate(ctx, plan.generateInput, plan.fullLLMMessages)
	}
	return output, attemptErr
}

// mergeFollowUpUsage 把工具回灌后一次调用的用量并入本条消息累计：上游有观测值时以累计观测为准，
// 上游未上报时回退到运行期估算的累计口径。
func mergeFollowUpUsage(total llm.Usage, next *llm.GenerateOutput, usage *messageUsageAccumulator) llm.Usage {
	total = addLLMUsage(total, next.Usage)
	if next.Usage != (llm.Usage{}) {
		usage.setObservedUsage(total)
	} else if usage.usage() != (llm.Usage{}) {
		total = usage.usage()
	}
	return total
}

// generationCanceled 判断上游调用是否因请求取消或用户显式停止而终止。
func generationCanceled(ctx context.Context, generateErr error) bool {
	if generateErr == nil {
		return false
	}
	return ctx.Err() != nil || isMessageGenerationCanceledError(generateErr)
}
