package personalprovider

import (
	"context"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainpersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/personalprovider"
)

// platformRouter 是平台路由的全部能力；组合解析器把非个人模型原样委托给它。
type platformRouter interface {
	ResolveRoute(ctx context.Context, input channel.ResolveRouteInput) (*channel.ResolvedRoute, error)
	ResolveDefaultRoute(ctx context.Context, input channel.ResolveRouteInput) (*channel.ResolvedRoute, error)
	MarkRouteFailure(ctx context.Context, route *channel.ResolvedRoute, cause error)
	MarkRouteSuccess(ctx context.Context, route *channel.ResolvedRoute)
	ListActiveModels(ctx context.Context, userID uint) ([]channel.ModelView, error)
	BuildExternalRoute(input channel.ExternalRouteInput) *channel.ResolvedRoute
}

// RouteResolver 在平台路由前加一层：personal: 引用由个人服务解析，其余全部交给平台。
// 个人模型解析失败时直接返回错误，绝不回退到平台上游，避免用平台的 Key 和余额替用户付费。
type RouteResolver struct {
	platform platformRouter
	personal *Service
}

// NewRouteResolver 创建组合路由解析器。
func NewRouteResolver(platform platformRouter, personal *Service) *RouteResolver {
	return &RouteResolver{platform: platform, personal: personal}
}

// ResolveRoute 解析模型路由。
func (r *RouteResolver) ResolveRoute(ctx context.Context, input channel.ResolveRouteInput) (*channel.ResolvedRoute, error) {
	if !domainpersonalprovider.IsModelRef(input.PlatformModelName) {
		return r.platform.ResolveRoute(ctx, input)
	}
	// 个人服务只开放对话协议；图片、视频等任务不能落到个人服务上。
	if input.TaskType != channel.TaskTypeChat {
		return nil, channel.ErrRouteNotFound
	}
	// 个人模型只有一条路由，故障后没有可切换的备选，也不能切到平台上游。
	if len(input.ExcludedRouteIDs) > 0 {
		return nil, channel.ErrAllRoutesUnavailable
	}
	if r.personal == nil {
		return nil, ErrModelUnavailable
	}
	resolved, err := r.personal.Resolve(ctx, input.UserID, input.PlatformModelName)
	if err != nil {
		return nil, err
	}
	return r.platform.BuildExternalRoute(channel.ExternalRouteInput{
		ExternalModel: channel.ExternalModel{
			Ref:          resolved.Ref,
			Model:        resolved.Model,
			ProviderName: resolved.ProviderName,
			Protocol:     resolved.Protocol,
		},
		BaseURL: resolved.BaseURL,
		APIKey:  resolved.APIKey,
	}), nil
}

// ResolveDefaultRoute 默认路由只来自平台。
func (r *RouteResolver) ResolveDefaultRoute(ctx context.Context, input channel.ResolveRouteInput) (*channel.ResolvedRoute, error) {
	return r.platform.ResolveDefaultRoute(ctx, input)
}

// MarkRouteFailure 只为平台路由记录熔断；个人路由不关联平台上游，交给平台实现也会被忽略。
func (r *RouteResolver) MarkRouteFailure(ctx context.Context, route *channel.ResolvedRoute, cause error) {
	if route != nil && route.UntrustedEndpoint {
		return
	}
	r.platform.MarkRouteFailure(ctx, route, cause)
}

// MarkRouteSuccess 只为平台路由清除失败计数。
func (r *RouteResolver) MarkRouteSuccess(ctx context.Context, route *channel.ResolvedRoute) {
	if route != nil && route.UntrustedEndpoint {
		return
	}
	r.platform.MarkRouteSuccess(ctx, route)
}

// ListActiveModels 返回平台模型目录。项目默认模型等长期配置只允许平台模型，个人模型不进入该列表。
func (r *RouteResolver) ListActiveModels(ctx context.Context, userID uint) ([]channel.ModelView, error) {
	return r.platform.ListActiveModels(ctx, userID)
}
