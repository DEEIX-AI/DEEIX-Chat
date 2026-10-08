package conversation

import (
	"context"
	"errors"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	apppersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/personalprovider"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// routeCapturingGateway 记录上游调用收到的路由配置。
type routeCapturingGateway struct {
	temporaryLLMGatewayStub
	routes []llm.RouteConfig
}

func (g *routeCapturingGateway) GenerateStream(
	ctx context.Context,
	route llm.RouteConfig,
	input llm.GenerateInput,
	onEvent func(llm.GenerateStreamEvent) error,
) (*llm.GenerateOutput, error) {
	g.routes = append(g.routes, route)
	return g.temporaryLLMGatewayStub.GenerateStream(ctx, route, input, onEvent)
}

// personalResolverStub 模拟组合路由：个人引用返回不受信端点路由，其它模型不可用。
type personalResolverStub struct {
	textTaskRouteResolverStub
	personal *channel.ResolvedRoute
	err      error
}

func (r *personalResolverStub) ResolveRoute(ctx context.Context, input channel.ResolveRouteInput) (*channel.ResolvedRoute, error) {
	if input.PlatformModelName == r.personal.PlatformModelName {
		if r.err != nil {
			return nil, r.err
		}
		return r.personal, nil
	}
	return r.textTaskRouteResolverStub.ResolveRoute(ctx, input)
}

func TestTemporaryChatOnPersonalModelUsesTheUntrustedClient(t *testing.T) {
	ref := "personal:abcdef1234567890/gpt-4o"
	gateway := &routeCapturingGateway{}
	service := &Service{
		cfg: config.NewRuntime(config.Config{
			ModelOptionPolicyMode:   modelOptionPolicyAllowlist,
			ModelOptionAllowedPaths: config.DefaultModelOptionAllowedPathsJSON(),
			ModelOptionDeniedPaths:  config.DefaultModelOptionDeniedPathsJSON(),
		}),
		routeResolver: &personalResolverStub{personal: &channel.ResolvedRoute{
			PlatformModelName: ref,
			UpstreamModel:     "gpt-4o",
			Protocol:          llm.AdapterOpenAIChatCompletions,
			BaseURL:           "https://api.example.com/v1",
			APIKey:            "sk-user-own",
			UntrustedEndpoint: true,
		}},
		llmClient: gateway,
	}
	if _, err := service.StreamTemporaryChat(t.Context(), TemporaryChatInput{
		UserID:      1,
		SessionID:   "temporary-session",
		ClientRunID: "temporary-run",
		Model:       ref,
		Messages:    []TemporaryChatMessage{{Role: "user", Content: "hi"}},
	}, nil); err != nil {
		t.Fatalf("stream temporary chat: %v", err)
	}
	if len(gateway.routes) != 1 {
		t.Fatalf("upstream calls = %d", len(gateway.routes))
	}
	route := gateway.routes[0]
	if !route.UntrustedEndpoint || route.APIKey != "sk-user-own" || route.UpstreamModel != "gpt-4o" {
		t.Fatalf("route config = %#v", route)
	}
}

func TestPersonalModelUnavailableSurfacesAsItsOwnError(t *testing.T) {
	ref := "personal:abcdef1234567890/gpt-4o"
	service := &Service{
		cfg: config.NewRuntime(config.Config{}),
		routeResolver: &personalResolverStub{
			personal: &channel.ResolvedRoute{PlatformModelName: ref},
			err:      apppersonalprovider.ErrModelUnavailable,
		},
		llmClient: &routeCapturingGateway{},
	}
	_, err := service.StreamTemporaryChat(t.Context(), TemporaryChatInput{
		UserID:      1,
		SessionID:   "temporary-session",
		ClientRunID: "temporary-run",
		Model:       ref,
		Messages:    []TemporaryChatMessage{{Role: "user", Content: "hi"}},
	}, nil)
	if !errors.Is(err, apppersonalprovider.ErrModelUnavailable) {
		t.Fatalf("err = %v, want ErrModelUnavailable", err)
	}
}
