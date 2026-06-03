package conversation

import (
	"context"
	"fmt"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
)

const textTaskFollowModel = "follow"

// resolveTextTaskRoute 解析标题、标签、压缩等内部文本任务使用的聊天路由。
// follow 优先复用当前会话模型；当前模型不是聊天模型时，回退到系统默认聊天路由。
func (s *Service) resolveTextTaskRoute(ctx context.Context, configured string, conversationModel string, userID uint, conversationID uint, requestID string) (*channel.ResolvedRoute, error) {
	routes, err := s.resolveTextTaskRouteCandidates(ctx, configured, conversationModel, userID, conversationID, requestID)
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
func (s *Service) resolveTextTaskRouteCandidates(ctx context.Context, configured string, conversationModel string, userID uint, conversationID uint, requestID string) ([]*channel.ResolvedRoute, error) {
	if s.routeResolver == nil {
		return nil, ErrModelRouteNotConfigured
	}
	value := strings.TrimSpace(configured)
	if value != "" && !strings.EqualFold(value, textTaskFollowModel) {
		route, err := s.routeResolver.ResolveRoute(ctx, channel.ResolveRouteInput{
			PlatformModelName: value,
			TaskType:          channel.TaskTypeChat,
			Scope:             channel.RouteScopeInternal,
			UserID:            userID,
			ConversationID:    conversationID,
			RequestID:         strings.TrimSpace(requestID),
		})
		if err != nil {
			return nil, fmt.Errorf("text task route resolve: %w", err)
		}
		return []*channel.ResolvedRoute{route}, nil
	}

	routes := make([]*channel.ResolvedRoute, 0, 2)
	var routeErr error

	// follow 只在当前会话模型本身具备聊天路由时直接复用；图片、视频等非文本模型不参与内部文本任务。
	if modelName := strings.TrimSpace(conversationModel); modelName != "" {
		route, err := s.routeResolver.ResolveRoute(ctx, channel.ResolveRouteInput{
			PlatformModelName: modelName,
			TaskType:          channel.TaskTypeChat,
			Scope:             channel.RouteScopeInternal,
			UserID:            userID,
			ConversationID:    conversationID,
			RequestID:         strings.TrimSpace(requestID),
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
			UserID:         userID,
			ConversationID: conversationID,
			RequestID:      strings.TrimSpace(requestID),
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
func (s *Service) resolveTextTaskModel(ctx context.Context, configured string, conversationModel string, userID uint, conversationID uint, requestID string) string {
	route, err := s.resolveTextTaskRoute(ctx, configured, conversationModel, userID, conversationID, requestID)
	if err != nil || route == nil {
		return ""
	}
	return strings.TrimSpace(route.PlatformModelName)
}
