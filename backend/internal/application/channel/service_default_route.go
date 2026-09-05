package channel

import (
	"context"
	"strings"
)

// ResolveDefaultRoute 返回指定任务类型的第一个可路由模型。
//
// 内部服务任务使用 follow 时，如果当前会话模型不支持该任务类型，会走这里兜底；
// 兜底仍必须经过任务类型过滤和真实路由解析，避免把图片模型误用于文本任务。
func (s *Service) ResolveDefaultRoute(ctx context.Context, input ResolveRouteInput) (*ResolvedRoute, error) {
	models, err := s.ListActiveModels(ctx, 0)
	if err != nil {
		return nil, err
	}
	for _, item := range models {
		name := strings.TrimSpace(item.PlatformModelName)
		if name == "" {
			continue
		}
		if !ModelSupportsTask(item.KindsJSON, input.TaskType) {
			continue
		}
		route, routeErr := s.ResolveRoute(ctx, ResolveRouteInput{
			PlatformModelName: name,
			TaskType:          input.TaskType,
			Scope:             input.Scope,
			UserID:            input.UserID,
			ConversationID:    input.ConversationID,
			RequestID:         strings.TrimSpace(input.RequestID),
		})
		if routeErr == nil {
			return route, nil
		}
	}
	return nil, ErrAllRoutesUnavailable
}

// ModelSupportsTask 判断模型声明的能力是否包含指定任务类型。
// 这里只校验静态目录能力，不读取路由健康或熔断状态。
func ModelSupportsTask(kindsJSON string, taskType string) bool {
	kinds := parseKinds(kindsJSON)
	if len(kinds) == 0 {
		return true
	}
	switch NormalizeTaskType(taskType) {
	case TaskTypeImageGeneration:
		return hasModelKind(kinds, modelKindImageGen)
	case TaskTypeImageEdit:
		return hasModelKind(kinds, modelKindImageEdit)
	case TaskTypeVideoGeneration:
		return hasModelKind(kinds, modelKindVideoGen)
	case TaskTypeVideoExtension:
		return hasModelKind(kinds, modelKindVideoExtension)
	default:
		return hasModelKind(kinds, modelKindChat)
	}
}
