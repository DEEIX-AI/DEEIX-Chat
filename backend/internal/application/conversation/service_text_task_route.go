package conversation

import (
	"context"
	"fmt"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

const textTaskFollowModel = "follow"

type textTaskRouteInput struct {
	ConfiguredModel   string
	ConversationModel string
	UserID            uint
	ConversationID    uint
	RequestID         string
}

// resolveTextTaskRoute 解析标题、标签、压缩等内部文本任务使用的聊天路由。
// follow 优先复用当前会话模型；当前模型不是聊天模型时，回退到系统默认聊天路由。
func (s *Service) resolveTextTaskRoute(ctx context.Context, input textTaskRouteInput) (*channel.ResolvedRoute, error) {
	routes, err := s.resolveTextTaskRouteCandidates(ctx, input)
	if err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, ErrModelRouteNotConfigured
	}
	return routes[0], nil
}

// resolveTextTaskRouteCandidates 返回内部文本任务的候选路由。
// 指定模型只使用指定路由；follow 先使用当前会话模型，再使用默认聊天路由作为任务兜底。
func (s *Service) resolveTextTaskRouteCandidates(ctx context.Context, input textTaskRouteInput) ([]*channel.ResolvedRoute, error) {
	if s.routeResolver == nil {
		return nil, ErrModelRouteNotConfigured
	}
	value := strings.TrimSpace(input.ConfiguredModel)
	if value != "" && !strings.EqualFold(value, textTaskFollowModel) {
		route, err := s.routeResolver.ResolveRoute(ctx, channel.ResolveRouteInput{
			PlatformModelName: value,
			TaskType:          channel.TaskTypeChat,
			Scope:             channel.RouteScopeInternal,
			UserID:            input.UserID,
			ConversationID:    input.ConversationID,
			RequestID:         strings.TrimSpace(input.RequestID),
		})
		if err != nil {
			return nil, fmt.Errorf("text task route resolve: %w", err)
		}
		return []*channel.ResolvedRoute{route}, nil
	}

	routes := make([]*channel.ResolvedRoute, 0, 2)
	var routeErr error

	// follow 只在当前会话模型本身具备聊天路由时直接复用；图片、视频等非文本模型不参与内部文本任务。
	if modelName := strings.TrimSpace(input.ConversationModel); modelName != "" {
		route, err := s.routeResolver.ResolveRoute(ctx, channel.ResolveRouteInput{
			PlatformModelName: modelName,
			TaskType:          channel.TaskTypeChat,
			Scope:             channel.RouteScopeInternal,
			UserID:            input.UserID,
			ConversationID:    input.ConversationID,
			RequestID:         strings.TrimSpace(input.RequestID),
		})
		if err == nil {
			routes = append(routes, route)
		} else if routeErr == nil {
			routeErr = err
		}
	}

	if resolver, ok := s.routeResolver.(defaultRouteResolver); ok {
		route, err := resolver.ResolveDefaultRoute(ctx, channel.ResolveRouteInput{
			TaskType:       channel.TaskTypeChat,
			Scope:          channel.RouteScopeInternal,
			UserID:         input.UserID,
			ConversationID: input.ConversationID,
			RequestID:      strings.TrimSpace(input.RequestID),
		})
		if err != nil {
			if len(routes) == 0 {
				return nil, fmt.Errorf("default text task route resolve: %w", err)
			}
			return routes, nil
		}
		if !textTaskRouteExists(routes, route) {
			routes = append(routes, route)
		}
	}
	if len(routes) > 0 {
		return routes, nil
	}
	if routeErr != nil {
		return nil, fmt.Errorf("text task route resolve: %w", routeErr)
	}
	return nil, ErrModelRouteNotConfigured
}

func textTaskRouteExists(routes []*channel.ResolvedRoute, route *channel.ResolvedRoute) bool {
	if route == nil {
		return true
	}
	for _, item := range routes {
		if item == nil {
			continue
		}
		if strings.TrimSpace(item.BindingCode) != "" && strings.TrimSpace(item.BindingCode) == strings.TrimSpace(route.BindingCode) {
			return true
		}
		if strings.TrimSpace(item.PlatformModelName) == strings.TrimSpace(route.PlatformModelName) &&
			strings.TrimSpace(item.Protocol) == strings.TrimSpace(route.Protocol) &&
			strings.TrimSpace(item.UpstreamModel) == strings.TrimSpace(route.UpstreamModel) {
			return true
		}
	}
	return false
}

// resolveTextTaskModel 返回内部文本任务实际使用的平台模型名。
// 压缩服务拿到空模型时会走模板摘要回退，因此这里不把兜底失败升级为主流程错误。
func (s *Service) resolveTextTaskModel(ctx context.Context, input textTaskRouteInput) string {
	route, err := s.resolveTextTaskRoute(ctx, input)
	if err != nil || route == nil {
		return ""
	}
	return strings.TrimSpace(route.PlatformModelName)
}

// buildTextTaskGenerateInput 为标题、标签、压缩等内部文本任务统一应用模型能力策略。
func buildTextTaskGenerateInput(route *channel.ResolvedRoute, cfg config.Config, messages []llm.Message) llm.GenerateInput {
	if route == nil {
		return llm.GenerateInput{Messages: cloneLLMMessages(messages)}
	}
	input := llm.GenerateInput{
		Messages: normalizeTextTaskSystemMessages(route, messages),
		Options: filterModelOptions(nil, route.Protocol, modelOptionPolicyConfig{
			Mode:                  cfg.ModelOptionPolicyMode,
			AllowedPathsJSON:      cfg.ModelOptionAllowedPaths,
			DeniedPathsJSON:       cfg.ModelOptionDeniedPaths,
			ModelCapabilitiesJSON: route.ModelCapabilitiesJSON,
		}),
	}
	applyOpenAIResponsesInstructions(route, llm.DefaultEndpointForAdapter(route.Protocol), &input)
	return input
}

// normalizeTextTaskSystemMessages 按模型能力决定内部文本任务是否需要把 system 指令降级进 user 消息。
func normalizeTextTaskSystemMessages(route *channel.ResolvedRoute, messages []llm.Message) []llm.Message {
	if route == nil || !shouldInlineSystemPromptToUser(*route) {
		return cloneLLMMessages(messages)
	}
	var systemText strings.Builder
	remaining := make([]llm.Message, 0, len(messages))
	for _, message := range messages {
		if message.Role != "system" {
			remaining = append(remaining, message)
			continue
		}
		text := strings.TrimSpace(systemInstructionText(message))
		if text == "" {
			continue
		}
		if systemText.Len() > 0 {
			systemText.WriteString("\n\n")
		}
		systemText.WriteString(text)
	}
	if systemText.Len() == 0 {
		return remaining
	}
	return inlineSystemPromptIntoLatestUserMessage(remaining, systemText.String())
}
