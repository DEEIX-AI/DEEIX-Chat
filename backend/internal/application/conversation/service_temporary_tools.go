package conversation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

type temporaryGenerationResult struct {
	Output            *llm.GenerateOutput
	Usage             llm.Usage
	FirstTokenLatency int64
	// MCPToolUsage 聚合本次临时生成中成功的 MCP 调用；错误中断时也需带出已产生的上游费用。
	MCPToolUsage []MCPToolUsageItem
}

func (s *Service) runTemporaryGeneration(
	ctx context.Context,
	input TemporaryChatInput,
	route *channel.ResolvedRoute,
	routeConfig llm.RouteConfig,
	initialInput llm.GenerateInput,
	toolRuntime selectedToolRuntime,
	traceRecorder *messageTraceRecorder,
	startedAt time.Time,
	onDelta func(string) error,
) (temporaryGenerationResult, error) {
	cfg := s.cfg.Snapshot()
	totalUsage := llm.Usage{}
	totalServerToolUsage := map[string]int64(nil)
	var totalMCPToolUsage []MCPToolUsageItem
	firstTokenLatency := int64(0)
	llmCallCount := 0

	runGenerate := func(currentInput llm.GenerateInput, prepareInput bool) (*llm.GenerateOutput, error) {
		if prepareInput {
			currentInput, _ = fitGenerateInputToModelBudget(
				currentInput,
				route.UpstreamModel,
				route.ModelCapabilitiesJSON,
				cfg.ContextWindowFallbackTokens,
				cfg.ContextTokenBudgetEnabled,
			)
			applyOpenAIResponsesInstructions(route, routeConfig.Endpoint, &currentInput)
		}
		currentInput = enforceTemporaryGenerateInput(currentInput)

		var callText strings.Builder
		var observedUsage llm.Usage
		llmCallCount++
		output, err := s.llmClient.GenerateStream(ctx, routeConfig, currentInput, func(event llm.GenerateStreamEvent) error {
			if event.Usage != (llm.Usage{}) {
				observedUsage = event.Usage
				if emitErr := emitLLMUsageEvent(input.OnEvent, addLLMUsage(totalUsage, observedUsage)); emitErr != nil {
					return emitErr
				}
			}
			if traceRecorder != nil && event.Reasoning != nil && event.Reasoning.Text != "" {
				traceRecorder.appendUpstreamReasoning(event.Reasoning.Kind, event.Reasoning.Text, reasoningPayload(event.Reasoning))
				if strings.EqualFold(strings.TrimSpace(event.Reasoning.Status), "completed") {
					traceRecorder.completeUpstreamThink()
				}
			}
			if event.Delta == "" {
				return nil
			}
			if firstTokenLatency == 0 {
				firstTokenLatency = time.Since(startedAt).Milliseconds()
			}
			if traceRecorder != nil {
				traceRecorder.completeProcess()
				traceRecorder.completeUpstreamThink()
			}
			callText.WriteString(event.Delta)
			if onDelta != nil {
				return onDelta(event.Delta)
			}
			return nil
		})
		if output == nil {
			output = &llm.GenerateOutput{}
		}
		if output.Text == "" {
			output.Text = callText.String()
		}
		if output.Usage == (llm.Usage{}) {
			output.Usage = observedUsage
		}
		totalUsage = addLLMUsage(totalUsage, output.Usage)
		totalServerToolUsage = addServerSideToolUsage(totalServerToolUsage, output.ServerSideToolUsage)
		output.ServerSideToolUsage = totalServerToolUsage
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				s.routeResolver.MarkRouteFailure(ctx, route, err)
			}
			return output, err
		}
		s.routeResolver.MarkRouteSuccess(ctx, route)
		return output, nil
	}

	output, err := runGenerate(initialInput, false)
	if err != nil {
		return temporaryGenerationResult{Output: output, Usage: totalUsage, FirstTokenLatency: firstTokenLatency}, err
	}
	buildResult := func() temporaryGenerationResult {
		return temporaryGenerationResult{
			Output:            output,
			Usage:             totalUsage,
			FirstTokenLatency: firstTokenLatency,
			MCPToolUsage:      totalMCPToolUsage,
		}
	}

	messages := cloneLLMMessages(initialInput.Messages)
	remainingToolCalls := s.resolveMaxToolCallsPerRun()
	maxLLMCalls := s.resolveMaxLLMCallsPerRun()
	ledger := newToolExecutionLedger()
	for len(output.ToolCalls) > 0 && llmCallCount < maxLLMCalls && remainingToolCalls > 0 {
		pending := output.ToolCalls
		if len(pending) > remainingToolCalls {
			pending = pending[:remainingToolCalls]
		}
		assistantToolMessage := llm.Message{
			Role:      "assistant",
			Content:   output.Text,
			ToolCalls: pending,
		}
		resultBudget := resolveToolResultTokenBudget(
			initialInput,
			messages,
			assistantToolMessage,
			route.UpstreamModel,
			route.ModelCapabilitiesJSON,
			cfg.ContextWindowFallbackTokens,
		)
		toolResult := s.executeAssistantToolCalls(ctx, executeAssistantToolCallsInput{
			UserID:            input.UserID,
			ConversationID:    0,
			MessageID:         0,
			RequestID:         input.RequestID,
			RunID:             input.ClientRunID,
			ToolCalls:         pending,
			ToolCallLimit:     remainingToolCalls,
			TraceRecorder:     traceRecorder,
			ToolNameMap:       toolRuntime.nameMap,
			MCPBindings:       toolRuntime.mcpBindings,
			ToolSchemas:       toolRuntime.schemas,
			Ledger:            ledger,
			ResultTokenBudget: resultBudget,
			Ephemeral:         true,
		})
		totalMCPToolUsage = mergeMCPToolUsage(totalMCPToolUsage, toolResult.MCPToolUsage)
		remainingToolCalls -= len(toolResult.Rows)
		if toolResult.FatalErr != nil {
			return buildResult(), toolResult.FatalErr
		}
		if len(toolResult.ToolResults) == 0 {
			break
		}
		assistantToolMessage.ToolCalls = toolResult.ExecutedToolCalls
		messages = append(messages, assistantToolMessage, llm.Message{Role: "tool", ToolResults: toolResult.ToolResults})

		followUp := initialInput
		followUp.Messages = messages
		if llmCallCount+1 >= maxLLMCalls {
			followUp.Messages = buildFinalToolSynthesisMessages(messages, "The maximum number of LLM calls for this run has been reached. Stop calling tools and produce the final answer based on the tool results already available.")
			followUp.Tools = nil
			followUp.DisableTools = true
		}
		output, err = runGenerate(followUp, true)
		if err != nil {
			return buildResult(), err
		}
	}

	if len(output.ToolCalls) > 0 && remainingToolCalls <= 0 && llmCallCount < maxLLMCalls {
		finalInput := initialInput
		finalInput.Messages = buildFinalToolSynthesisMessages(messages, "The maximum number of tool calls for this run has been reached. Stop calling tools and produce the final answer based on the tool results already available.")
		finalInput.Tools = nil
		finalInput.DisableTools = true
		output, err = runGenerate(finalInput, true)
		if err != nil {
			return buildResult(), err
		}
	}

	return buildResult(), nil
}
