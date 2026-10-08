package channel

import (
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

const (
	externalRouteConnectTimeoutMS    = 10_000
	externalRouteReadTimeoutMS       = 120_000
	externalRouteStreamIdleTimeoutMS = 60_000
)

// ExternalModel 描述一个不在平台路由表中的模型（用户自带 Key 的模型）。
type ExternalModel struct {
	// Ref 是对外使用的模型引用（personal:<id>/<model>），会话与计费都只认它。
	Ref string
	// Model 是上游真实模型名，用于识别厂商、图标与内置目录能力。
	Model        string
	ProviderName string
	Protocol     string
}

// ExternalRouteInput 描述外部模型的调用配置。APIKey 为明文，只在内存中传递。
type ExternalRouteInput struct {
	ExternalModel
	BaseURL string
	APIKey  string
}

// BuildExternalRoute 为外部模型构造调用路由：补齐厂商、图标、内置目录的推理能力与输入模态，
// 并标记 UntrustedEndpoint，使请求只走强制 SSRF 防护、不跟随重定向的客户端。
// 路由不关联任何平台上游（UpstreamID 为 0），因此不参与平台熔断、限流退避与故障切换。
func (s *Service) BuildExternalRoute(input ExternalRouteInput) *ResolvedRoute {
	model := strings.TrimSpace(input.Model)
	providerName := strings.TrimSpace(input.ProviderName)
	vendor := normalizeModelVendor("", model, providerName)
	protocol := llm.NormalizeAdapter(input.Protocol)
	route := &ResolvedRoute{
		PlatformModelName:               strings.TrimSpace(input.Ref),
		UpstreamName:                    providerName,
		Protocol:                        protocol,
		BaseURL:                         strings.TrimSpace(input.BaseURL),
		APIKey:                          input.APIKey,
		ConnectTimeoutMS:                externalRouteConnectTimeoutMS,
		ReadTimeoutMS:                   externalRouteReadTimeoutMS,
		StreamIdleTimeoutMS:             externalRouteStreamIdleTimeoutMS,
		ModelVendor:                     vendor,
		ModelIcon:                       normalizeModelIcon("", vendor, model),
		UpstreamModel:                   model,
		ReasoningContentPassback:        reasoningContentPassbackRequired(protocol, vendor, model, providerName),
		ReasoningPassbackRequestOptions: reasoningPassbackRequestOptions(protocol, vendor, model, providerName),
		UntrustedEndpoint:               true,
	}
	protocolKeys := []string{llm.OptionPolicyProtocolKey(protocol)}
	if catalog := s.activeModelCatalog(); catalog != nil {
		entry := catalog.Match(model, vendor, protocolKeys)
		route.CatalogReasoning, _ = domainchannel.CatalogReasoningCapability(entry, protocolKeys[0], vendor)
	}
	if catalog := s.activeModalityCatalog(); catalog != nil {
		if entry := catalog.MatchExternal(model, vendor, protocolKeys); entry != nil {
			route.CatalogInputModalities = append([]string(nil), entry.InputModalities...)
		}
	}
	return route
}

// ExternalModelView 构造外部模型在用户模型目录中的展示数据。能力按上游真实模型名解析，
// 返回的 PlatformModelName 仍是上游模型名，由调用方替换为对外引用。
func (s *Service) ExternalModelView(model ExternalModel) ModelView {
	name := strings.TrimSpace(model.Model)
	providerName := strings.TrimSpace(model.ProviderName)
	vendor := normalizeModelVendor("", name, providerName)
	protocol := llm.NormalizeAdapter(model.Protocol)
	return ModelView{
		PlatformModelName: name,
		Vendor:            vendor,
		VendorName:        vendor,
		VendorIcon:        resolveVendorIcon(vendor),
		KindsJSON:         `["chat"]`,
		Icon:              normalizeModelIcon("", vendor, name),
		CapabilitiesJSON:  "{}",
		AccessScope:       ModelAccessScopePublic,
		Status:            "active",
		ProtocolsJSON:     `["` + protocol + `"]`,
	}
}
